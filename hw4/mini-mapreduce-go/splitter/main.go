package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

//
// =============================
// Request / Response Structures
// =============================
//

// SplitReq is the JSON input sent to /split.
// It tells the splitter which S3 object to split, and into how many chunks.
type SplitReq struct {
	Bucket    string `json:"bucket"`
	Key       string `json:"key"`
	NumChunks int    `json:"num_chunks"`
}

// SplitResp is the JSON output returned by the splitter.
// It returns a run ID and a list of S3 URLs for all generated chunk files.
type SplitResp struct {
	RunID     string   `json:"run_id"`
	ChunkURLs []string `json:"chunk_urls"`
}

//
// =============================
// Utility Functions
// =============================
//

// writeJSON writes a JSON response with a specific status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// readAll reads the entire content from an io.Reader into a string.
// Here, it is used to read the whole input file content from S3 into memory.
func readAll(r io.Reader) (string, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

//
// =============================
// Splitting Helpers
// =============================
//

// safeCut tries to avoid splitting in the middle of a word.
// It adjusts the start and end positions to land near whitespace boundaries.
func safeCut(text string, start, end int) (int, int) {
	// Clamp to valid bounds
	if start < 0 {
		start = 0
	}
	if end > len(text) {
		end = len(text)
	}
	if start >= end {
		return start, end
	}

	// If start is in the middle of a word, move it forward until whitespace.
	// This reduces the chance that chunk starts with a half-word.
	if start > 0 && start < len(text) && !isSpace(text[start]) && !isSpace(text[start-1]) {
		for start < end && !isSpace(text[start]) {
			start++
		}
	}

	// If end is in the middle of a word, move it backward until whitespace.
	// This reduces the chance that chunk ends with a half-word.
	if end < len(text) && end > 0 && !isSpace(text[end-1]) && !isSpace(text[end]) {
		for end > start && !isSpace(text[end-1]) {
			end--
		}
	}

	return start, end
}

// isSpace checks whether a byte is whitespace (space/newline/tab/carriage return).
func isSpace(b byte) bool {
	return b == ' ' || b == '\n' || b == '\t' || b == '\r'
}

//
// =============================
// Main Application
// =============================
//

func main() {
	// Read port from environment (useful in ECS/container).
	// Default to 8080 if not set.
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	ctx := context.Background()

	// Load AWS SDK config (region, credentials, etc.)
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("load aws config: %v", err)
	}

	// Create S3 client
	s3c := s3.NewFromConfig(cfg)

	//
	// -----------------------------
	// /split HTTP endpoint
	// -----------------------------
	//
	http.HandleFunc("/split", func(w http.ResponseWriter, r *http.Request) {

		// Only allow POST requests
		if r.Method != http.MethodPost {
			writeJSON(w, 405, map[string]any{"error": "method not allowed"})
			return
		}

		// Parse JSON body into SplitReq
		var req SplitReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, 400, map[string]any{"error": "invalid json", "detail": err.Error()})
			return
		}

		// Validate required fields
		if req.Bucket == "" || req.Key == "" {
			writeJSON(w, 400, map[string]any{"error": "bucket and key are required"})
			return
		}

		// Default number of chunks if user passes <= 0
		if req.NumChunks <= 0 {
			req.NumChunks = 3
		}

		// Download the original input text file from S3
		obj, err := s3c.GetObject(ctx, &s3.GetObjectInput{
			Bucket: &req.Bucket,
			Key:    &req.Key,
		})
		if err != nil {
			writeJSON(w, 500, map[string]any{"error": "s3 get_object failed", "detail": err.Error()})
			return
		}
		defer obj.Body.Close()

		// Read full file content into memory (string)
		text, err := readAll(obj.Body)
		if err != nil {
			writeJSON(w, 500, map[string]any{"error": "read body failed", "detail": err.Error()})
			return
		}

		// Create a run ID so all outputs for this run are grouped together.
		// Example: chunks/run-a1b2c3d4/chunk-0.txt
		runID := randomHex8()

		// Compute chunk size using ceiling division
		// Example: if total=1000 chars and n=3 -> chunkSize=334
		n := req.NumChunks
		total := len(text)
		chunkSize := int(math.Ceil(float64(total) / float64(n)))

		// urls will store all chunk output S3 URLs
		var urls []string

		// Create each chunk and upload it to S3
		for i := 0; i < n; i++ {
			// Compute initial start/end boundaries for this chunk
			start := i * chunkSize
			end := (i + 1) * chunkSize

			// Clamp bounds so we don't go outside the text length
			if start > total {
				start = total
			}
			if end > total {
				end = total
			}

			// Adjust boundaries to avoid cutting a word in half
			start2, end2 := safeCut(text, start, end)
			chunkText := text[start2:end2]

			// Output key naming convention
			outKey := fmt.Sprintf("chunks/run-%s/chunk-%d.txt", runID, i)

			// Upload the chunk text to S3
			body := strings.NewReader(chunkText)
			_, err = s3c.PutObject(ctx, &s3.PutObjectInput{
				Bucket: &req.Bucket,
				Key:    &outKey,
				Body:   body,
			})
			if err != nil {
				writeJSON(w, 500, map[string]any{"error": "s3 put_object failed", "detail": err.Error()})
				return
			}

			// Record chunk URL for returning to the driver
			urls = append(urls, fmt.Sprintf("s3://%s/%s", req.Bucket, outKey))
		}

		// Return run ID + list of chunk URLs
		writeJSON(w, 200, SplitResp{RunID: runID, ChunkURLs: urls})
	})

	//
	// -----------------------------
	// Health check endpoint
	// -----------------------------
	//
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"ok": true})
	})

	log.Printf("splitter listening on :%s", port)
	log.Fatal(http.ListenAndServe("0.0.0.0:"+port, nil))
}

//
// =============================
// Run ID generator
// =============================
//

// randomHex8 creates a simple 8-char hex string used as a run ID.
// Not cryptographically secure; fine for a class project.
func randomHex8() string {
	const alphabet = "0123456789abcdef"

	var b [8]byte

	// Create a deterministic-ish seed from environment + process info
	seed := int64(0)
	for _, c := range []byte(os.Getenv("AWS_REGION") + os.Getenv("HOSTNAME")) {
		seed += int64(c)
	}
	seed += int64(os.Getpid())

	// Simple pseudo-random sequence from the seed
	x := uint64(seed*1103515245 + 12345)
	for i := 0; i < 8; i++ {
		x = x*2862933555777941757 + 3037000493
		b[i] = alphabet[int(x%16)]
	}
	return string(b[:])
}
