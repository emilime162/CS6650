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

type MapReq struct {
	ChunkURL string `json:"chunk_url"`
}

type MapResp struct {
	MapURL string `json:"map_url"`
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

func readAll(r io.Reader) (string, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

var wordRe = regexp.MustCompile(`[A-Za-z']+`)

func tokenize(text string) []string {
	raw := wordRe.FindAllString(strings.ToLower(text), -1)
	return raw
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

	http.HandleFunc("/map", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, 405, map[string]any{"error": "method not allowed"})
			return
		}

		var req MapReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, 400, map[string]any{"error": "invalid json", "detail": err.Error()})
			return
		}
		bucket, key, ok := parseS3URL(req.ChunkURL)
		if !ok {
			writeJSON(w, 400, map[string]any{"error": "invalid chunk_url"})
			return
		}

		obj, err := s3c.GetObject(ctx, &s3.GetObjectInput{
			Bucket: &bucket,
			Key:    &key,
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

		counts := map[string]int{}
		for _, w2 := range tokenize(text) {
			counts[w2]++
		}

		// chunks/run-XXXX/chunk-0.txt
		parts := strings.Split(key, "/")
		if len(parts) < 3 {
			writeJSON(w, 500, map[string]any{"error": "unexpected chunk key format", "key": key})
			return
		}
		runPart := parts[1] // run-XXXX
		runID := strings.TrimPrefix(runPart, "run-")
		chunkFile := parts[len(parts)-1] // chunk-0.txt
		idxStr := strings.TrimSuffix(strings.TrimPrefix(chunkFile, "chunk-"), ".txt")
		if _, err := strconv.Atoi(idxStr); err != nil {
			idxStr = "0"
		}

		outKey := fmt.Sprintf("maps/run-%s/map-%s.json", runID, idxStr)
		body, _ := json.Marshal(counts)

		_, err = s3c.PutObject(ctx, &s3.PutObjectInput{
			Bucket: &bucket,
			Key:    &outKey,
			Body:   strings.NewReader(string(body)),
		})
		if err != nil {
			writeJSON(w, 500, map[string]any{"error": "s3 put_object failed", "detail": err.Error()})
			return
		}

		writeJSON(w, 200, MapResp{MapURL: fmt.Sprintf("s3://%s/%s", bucket, outKey)})
	})

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"ok": true})
	})

	log.Printf("mapper listening on :%s", port)
	log.Fatal(http.ListenAndServe("0.0.0.0:"+port, nil))
}
