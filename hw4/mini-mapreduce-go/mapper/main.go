package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

//
// =============================
// Request / Response Structures
// =============================
//

// MapReq is the JSON input to /map.
// It contains the S3 URL of one chunk file produced by the splitter.
type MapReq struct {
	ChunkURL string `json:"chunk_url"`
}

// MapResp is the JSON output returned by the mapper.
// It contains the S3 URL where this mapper wrote its intermediate result.
type MapResp struct {
	MapURL string `json:"map_url"`
}

//
// =============================
// Utility Functions
// =============================
//

// writeJSON writes a JSON response with a given HTTP status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// parseS3URL parses a string like: s3://bucket-name/path/to/object
// and returns (bucket, key, ok).
func parseS3URL(url string) (bucket, key string, ok bool) {
	if !strings.HasPrefix(url, "s3://") {
		return "", "", false
	}
	rest := strings.TrimPrefix(url, "s3://")
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// readAll reads an entire io.Reader into a string.
// Here it's used to read the full text content of the S3 chunk file.
func readAll(r io.Reader) (string, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

//
// =============================
// Tokenization (word splitting)
// =============================
//

// wordRe matches words: letters + apostrophes (e.g., "don't").
// This is a simple tokenizer for word count.
var wordRe = regexp.MustCompile(`[A-Za-z']+`)

// tokenize converts the text to lowercase and extracts all words using wordRe.
// Output is a list of tokens like ["hello", "world", "don't", ...]
func tokenize(text string) []string {
	raw := wordRe.FindAllString(strings.ToLower(text), -1)
	return raw
}

//
// =============================
// Main Application
// =============================
//

func main() {
	// Read port from environment (useful when running in ECS/container).
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

	// Create an S3 client
	s3c := s3.NewFromConfig(cfg)

	//
	// -----------------------------
	// /map HTTP endpoint
	// -----------------------------
	//
	http.HandleFunc("/map", func(w http.ResponseWriter, r *http.Request) {

		// Only allow POST requests
		if r.Method != http.MethodPost {
			writeJSON(w, 405, map[string]any{"error": "method not allowed"})
			return
		}

		// Parse JSON body: {"chunk_url": "..."}
		var req MapReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, 400, map[string]any{"error": "invalid json", "detail": err.Error()})
			return
		}

		// Extract bucket + key from the S3 chunk URL
		bucket, key, ok := parseS3URL(req.ChunkURL)
		if !ok {
			writeJSON(w, 400, map[string]any{"error": "invalid chunk_url"})
			return
		}

		// Download the chunk file content from S3
		obj, err := s3c.GetObject(ctx, &s3.GetObjectInput{
			Bucket: &bucket,
			Key:    &key,
		})
		if err != nil {
			writeJSON(w, 500, map[string]any{"error": "s3 get_object failed", "detail": err.Error()})
			return
		}
		defer obj.Body.Close()

		// Read the chunk file into a string (the chunk is plain text)
		text, err := readAll(obj.Body)
		if err != nil {
			writeJSON(w, 500, map[string]any{"error": "read body failed", "detail": err.Error()})
			return
		}

		// Perform "map" logic: count words in this chunk
		// counts will look like: {"the": 12, "and": 4, ...}
		counts := map[string]int{}
		for _, w2 := range tokenize(text) {
			counts[w2]++
		}

		//
		// Build the output S3 key for the mapper result.
		//
		// The input chunk key format is expected to be:
		//   chunks/run-XXXX/chunk-0.txt
		//
		// This mapper will write output as:
		//   maps/run-XXXX/map-0.json
		//
		parts := strings.Split(key, "/")
		if len(parts) < 3 {
			writeJSON(w, 500, map[string]any{"error": "unexpected chunk key format", "key": key})
			return
		}

		// Example: parts[1] = "run-XXXX"
		runPart := parts[1]
		runID := strings.TrimPrefix(runPart, "run-")

		// Example: last part = "chunk-0.txt"
		chunkFile := parts[len(parts)-1]
		idxStr := strings.TrimSuffix(strings.TrimPrefix(chunkFile, "chunk-"), ".txt")

		// If idxStr is not a valid integer, fallback to "0"
		if _, err := strconv.Atoi(idxStr); err != nil {
			idxStr = "0"
		}

		// Construct output key for this mapper result
		outKey := fmt.Sprintf("maps/run-%s/map-%s.json", runID, idxStr)

		// Convert word count map to JSON bytes
		body, _ := json.Marshal(counts)

		// Upload mapper output JSON to S3
		_, err = s3c.PutObject(ctx, &s3.PutObjectInput{
			Bucket: &bucket,
			Key:    &outKey,
			Body:   strings.NewReader(string(body)),
		})
		if err != nil {
			writeJSON(w, 500, map[string]any{"error": "s3 put_object failed", "detail": err.Error()})
			return
		}

		// Return the S3 location of the mapper output
		writeJSON(w, 200, MapResp{
			MapURL: fmt.Sprintf("s3://%s/%s", bucket, outKey),
		})
	})

	//
	// -----------------------------
	// Health Check Endpoint
	// -----------------------------
	//
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"ok": true})
	})

	log.Printf("mapper listening on :%s", port)
	log.Fatal(http.ListenAndServe("0.0.0.0:"+port, nil))
}
