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

type SplitReq struct {
	Bucket    string `json:"bucket"`
	Key       string `json:"key"`
	NumChunks int    `json:"num_chunks"`
}

type SplitResp struct {
	RunID     string   `json:"run_id"`
	ChunkURLs []string `json:"chunk_urls"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func readAll(r io.Reader) (string, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Avoid splitting in the middle of a word: try to cut near whitespace.
func safeCut(text string, start, end int) (int, int) {
	if start < 0 {
		start = 0
	}
	if end > len(text) {
		end = len(text)
	}
	if start >= end {
		return start, end
	}

	// Move start forward to next whitespace boundary (optional)
	if start > 0 && start < len(text) && !isSpace(text[start]) && !isSpace(text[start-1]) {
		for start < end && !isSpace(text[start]) {
			start++
		}
	}

	// Move end backward to previous whitespace boundary
	if end < len(text) && end > 0 && !isSpace(text[end-1]) && !isSpace(text[end]) {
		for end > start && !isSpace(text[end-1]) {
			end--
		}
	}

	return start, end
}

func isSpace(b byte) bool {
	return b == ' ' || b == '\n' || b == '\t' || b == '\r'
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("load aws config: %v", err)
	}
	s3c := s3.NewFromConfig(cfg)

	http.HandleFunc("/split", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, 405, map[string]any{"error": "method not allowed"})
			return
		}

		var req SplitReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, 400, map[string]any{"error": "invalid json", "detail": err.Error()})
			return
		}
		if req.Bucket == "" || req.Key == "" {
			writeJSON(w, 400, map[string]any{"error": "bucket and key are required"})
			return
		}
		if req.NumChunks <= 0 {
			req.NumChunks = 3
		}

		obj, err := s3c.GetObject(ctx, &s3.GetObjectInput{
			Bucket: &req.Bucket,
			Key:    &req.Key,
		})
		if err != nil {
			writeJSON(w, 500, map[string]any{"error": "s3 get_object failed", "detail": err.Error()})
			return
		}
		defer obj.Body.Close()

		text, err := readAll(obj.Body)
		if err != nil {
			writeJSON(w, 500, map[string]any{"error": "read body failed", "detail": err.Error()})
			return
		}

		runID := randomHex8()
		n := req.NumChunks
		total := len(text)
		chunkSize := int(math.Ceil(float64(total) / float64(n)))

		var urls []string
		for i := 0; i < n; i++ {
			start := i * chunkSize
			end := (i + 1) * chunkSize
			if start > total {
				start = total
			}
			if end > total {
				end = total
			}

			start2, end2 := safeCut(text, start, end)
			chunkText := text[start2:end2]

			outKey := fmt.Sprintf("chunks/run-%s/chunk-%d.txt", runID, i)
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
			urls = append(urls, fmt.Sprintf("s3://%s/%s", req.Bucket, outKey))
		}

		writeJSON(w, 200, SplitResp{RunID: runID, ChunkURLs: urls})
	})

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"ok": true})
	})

	log.Printf("splitter listening on :%s", port)
	log.Fatal(http.ListenAndServe("0.0.0.0:"+port, nil))
}

// simple run id
func randomHex8() string {
	// Not crypto, good enough for a run id in a class project.
	const alphabet = "0123456789abcdef"
	var b [8]byte
	seed := int64(0)
	for _, c := range []byte(os.Getenv("AWS_REGION") + os.Getenv("HOSTNAME")) {
		seed += int64(c)
	}
	seed += int64(os.Getpid())
	x := uint64(seed*1103515245 + 12345)
	for i := 0; i < 8; i++ {
		x = x*2862933555777941757 + 3037000493
		b[i] = alphabet[int(x%16)]
	}
	return string(b[:])
}
