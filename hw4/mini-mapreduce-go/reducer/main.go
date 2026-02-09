package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type ReduceReq struct {
	MapURLs []string `json:"map_urls"`
}

type ReduceResp struct {
	ResultURL string `json:"result_url"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

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

func readAll(r io.Reader) ([]byte, error) {
	return io.ReadAll(r)
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

	http.HandleFunc("/reduce", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, 405, map[string]any{"error": "method not allowed"})
			return
		}

		var req ReduceReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, 400, map[string]any{"error": "invalid json", "detail": err.Error()})
			return
		}
		if len(req.MapURLs) == 0 {
			writeJSON(w, 400, map[string]any{"error": "map_urls required"})
			return
		}

		merged := map[string]int{}
		var bucket string
		var runID string

		for _, url := range req.MapURLs {
			b, k, ok := parseS3URL(url)
			if !ok {
				writeJSON(w, 400, map[string]any{"error": "invalid map_url", "url": url})
				return
			}
			bucket = b

			// maps/run-XXXX/map-0.json
			parts := strings.Split(k, "/")
			if len(parts) >= 2 {
				runID = strings.TrimPrefix(parts[1], "run-")
			}

			obj, err := s3c.GetObject(ctx, &s3.GetObjectInput{
				Bucket: &b,
				Key:    &k,
			})
			if err != nil {
				writeJSON(w, 500, map[string]any{"error": "s3 get_object failed", "detail": err.Error()})
				return
			}
			data, err := readAll(obj.Body)
			obj.Body.Close()
			if err != nil {
				writeJSON(w, 500, map[string]any{"error": "read body failed", "detail": err.Error()})
				return
			}

			var counts map[string]int
			if err := json.Unmarshal(data, &counts); err != nil {
				writeJSON(w, 500, map[string]any{"error": "invalid json in map output", "detail": err.Error()})
				return
			}

			for word, c := range counts {
				merged[word] += c
			}
		}

		outKey := fmt.Sprintf("reduce/run-%s/result.json", runID)
		body, _ := json.Marshal(merged)

		_, err := s3c.PutObject(ctx, &s3.PutObjectInput{
			Bucket: &bucket,
			Key:    &outKey,
			Body:   strings.NewReader(string(body)),
		})
		if err != nil {
			writeJSON(w, 500, map[string]any{"error": "s3 put_object failed", "detail": err.Error()})
			return
		}

		writeJSON(w, 200, ReduceResp{ResultURL: fmt.Sprintf("s3://%s/%s", bucket, outKey)})
	})

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"ok": true})
	})

	log.Printf("reducer listening on :%s", port)
	log.Fatal(http.ListenAndServe("0.0.0.0:"+port, nil))
}
