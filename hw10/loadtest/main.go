// loadtest/main.go – load test client for the KV cluster.
//
// Uses a small key pool (default 20 keys) so reads and writes cluster
// on the same key within short time windows – maximising stale read exposure.
//
// Every record written to the CSV includes a unix-ms timestamp so that
// plot.py can compute the time interval between a write and the next read
// on the same key (required graph 3 in the assignment report).
//
// Usage (leader-follower):
//
//	go run loadtest/main.go \
//	  -leader   http://localhost:8080 \
//	  -followers http://localhost:8081,http://localhost:8082,http://localhost:8083,http://localhost:8084 \
//	  -requests 500 -concurrency 10 -write-pct 10 -key-pool 20 \
//	  -out results.csv
//
// Usage (leaderless):
//
//	go run loadtest/main.go -leaderless \
//	  -leader   http://localhost:9080 \
//	  -followers http://localhost:9081,http://localhost:9082,http://localhost:9083,http://localhost:9084 \
//	  -requests 500 -concurrency 10 -write-pct 10 -key-pool 20 \
//	  -out results.csv
package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	leaderURL    = flag.String("leader", "http://localhost:8080", "Leader (or any node for leaderless) base URL")
	followersRaw = flag.String("followers", "http://localhost:8081,http://localhost:8082,http://localhost:8083,http://localhost:8084", "Comma-separated follower URLs")
	requests     = flag.Int("requests", 500, "Total number of requests")
	concurrency  = flag.Int("concurrency", 10, "Number of concurrent workers")
	writePct     = flag.Int("write-pct", 10, "Percentage of requests that are writes (0-100)")
	keyPool      = flag.Int("key-pool", 20, "Number of distinct keys to use")
	outFile      = flag.String("out", "results.csv", "Output CSV file")
	leaderless   = flag.Bool("leaderless", false, "Leaderless mode: writes go to any node")
)

// record is one completed request.
type record struct {
	Op          string  // "read" or "write"
	Key         string
	LatencyMs   float64
	Stale       bool  // read returned version < last written version
	Version     int64
	TimestampMs int64 // unix ms when request completed — used for interval graph
}

// versionTracker remembers the highest version written per key so reads
// can detect staleness.
type versionTracker struct {
	mu       sync.RWMutex
	versions map[string]int64
}

func newVersionTracker() *versionTracker {
	return &versionTracker{versions: make(map[string]int64)}
}
func (vt *versionTracker) setWritten(key string, ver int64) {
	vt.mu.Lock(); defer vt.mu.Unlock()
	if ver > vt.versions[key] {
		vt.versions[key] = ver
	}
}
func (vt *versionTracker) lastWritten(key string) int64 {
	vt.mu.RLock(); defer vt.mu.RUnlock()
	return vt.versions[key]
}

// writeTimeTracker records the wall-clock completion time of the last
// write per key. plot.py uses this to compute the read-write interval:
// how many ms after a write does the next read on the same key happen?
type writeTimeTracker struct {
	mu    sync.RWMutex
	times map[string]int64 // key → unix ms of last write completion
}

func newWriteTimeTracker() *writeTimeTracker {
	return &writeTimeTracker{times: make(map[string]int64)}
}
func (wt *writeTimeTracker) set(key string, ms int64) {
	wt.mu.Lock(); defer wt.mu.Unlock()
	wt.times[key] = ms
}
func (wt *writeTimeTracker) get(key string) int64 {
	wt.mu.RLock(); defer wt.mu.RUnlock()
	t, ok := wt.times[key]
	if !ok { return -1 }
	return t
}

type entryResp struct {
	Value   string `json:"value"`
	Version int64  `json:"version"`
}

var httpClient = &http.Client{Timeout: 10 * time.Second}

func doWrite(baseURL, key, value string) error {
	body, _ := json.Marshal(map[string]string{"key": key, "value": value})
	resp, err := httpClient.Post(baseURL+"/set", "application/json", bytes.NewReader(body))
	if err != nil { return err }
	resp.Body.Close()
	return nil
}

func doRead(baseURL, key string) (*entryResp, error) {
	resp, err := httpClient.Get(fmt.Sprintf("%s/get?key=%s", baseURL, key))
	if err != nil { return nil, err }
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound { return nil, nil }
	var e entryResp
	json.NewDecoder(resp.Body).Decode(&e)
	return &e, nil
}

func worker(
	id int,
	jobs <-chan struct{},
	results chan<- record,
	keys []string,
	allNodes []string,
	vt *versionTracker,
	wt *writeTimeTracker,
) {
	for range jobs {
		key := keys[rand.Intn(len(keys))]
		isWrite := rand.Intn(100) < *writePct

		if isWrite {
			writeURL := *leaderURL
			if *leaderless {
				writeURL = allNodes[rand.Intn(len(allNodes))]
			}

			value := fmt.Sprintf("v-%d-%d", id, time.Now().UnixNano())
			start := time.Now()
			err := doWrite(writeURL, key, value)
			completedAt := time.Now()

			if err != nil {
				log.Printf("write error: %v", err)
				continue
			}

			// Read back version immediately after write completes.
			re, _ := doRead(writeURL, key)
			var ver int64
			if re != nil {
				ver = re.Version
				vt.setWritten(key, ver)
			}

			completedMs := completedAt.UnixMilli()
			wt.set(key, completedMs) // record when this write finished

			results <- record{
				Op:          "write",
				Key:         key,
				LatencyMs:   float64(completedAt.Sub(start).Milliseconds()),
				Version:     ver,
				TimestampMs: completedMs,
			}

		} else {
			// For leader-follower: ALL reads go to the leader.
			// The leader handles R internally (R=1 local, R=5 fan-out, R=3 quorum).
			// Sending reads to followers bypasses quorum logic entirely.
			// For leaderless: reads go to any random node (R=1 local read per spec).
			var readURL string
			if *leaderless {
				readURL = allNodes[rand.Intn(len(allNodes))]
			} else {
				readURL = *leaderURL
			}

			start := time.Now()
			e, err := doRead(readURL, key)
			completedAt := time.Now()

			if err != nil {
				log.Printf("read error: %v", err)
				continue
			}

			var ver int64
			stale := false
			if e != nil {
				ver = e.Version
				if last := vt.lastWritten(key); last > 0 && ver < last {
					stale = true
				}
			}

			results <- record{
				Op:          "read",
				Key:         key,
				LatencyMs:   float64(completedAt.Sub(start).Milliseconds()),
				Stale:       stale,
				Version:     ver,
				TimestampMs: completedAt.UnixMilli(),
			}
		}
	}
}

func main() {
	flag.Parse()

	followers := strings.Split(*followersRaw, ",")
	allNodes := append([]string{*leaderURL}, followers...)

	keys := make([]string, *keyPool)
	for i := range keys { keys[i] = fmt.Sprintf("key%04d", i) }

	jobs := make(chan struct{}, *requests)
	results := make(chan record, *requests)
	vt := newVersionTracker()
	wt := newWriteTimeTracker()

	var wg sync.WaitGroup
	for i := 0; i < *concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			worker(id, jobs, results, keys, allNodes, vt, wt)
		}(i)
	}

	for i := 0; i < *requests; i++ { jobs <- struct{}{} }
	close(jobs)
	go func() { wg.Wait(); close(results) }()

	var records []record
	for r := range results { records = append(records, r) }

	// CSV columns:
	//   op, key, latency_ms, stale, version, timestamp_ms
	// timestamp_ms is unix milliseconds when the request completed.
	// plot.py uses it to compute: for each read, how many ms since the
	// last write on that key? That produces the interval distribution graph.
	f, err := os.Create(*outFile)
	if err != nil { log.Fatalf("cannot create output file: %v", err) }
	defer f.Close()

	w := csv.NewWriter(f)
	w.Write([]string{"op", "key", "latency_ms", "stale", "version", "timestamp_ms"})
	for _, r := range records {
		w.Write([]string{
			r.Op, r.Key,
			strconv.FormatFloat(r.LatencyMs, 'f', 2, 64),
			strconv.FormatBool(r.Stale),
			strconv.FormatInt(r.Version, 10),
			strconv.FormatInt(r.TimestampMs, 10),
		})
	}
	w.Flush()

	var reads, writes, stale int
	var totalR, totalW float64
	for _, r := range records {
		if r.Op == "read" {
			reads++; totalR += r.LatencyMs
			if r.Stale { stale++ }
		} else {
			writes++; totalW += r.LatencyMs
		}
	}
	fmt.Printf("\n===== Load Test Summary =====\n")
	fmt.Printf("Total requests:   %d\n", len(records))
	fmt.Printf("Writes:           %d  avg latency: %.1f ms\n", writes, safeDiv(totalW, float64(writes)))
	fmt.Printf("Reads:            %d  avg latency: %.1f ms\n", reads, safeDiv(totalR, float64(reads)))
	fmt.Printf("Stale reads:      %d (%.1f%%)\n", stale, 100*safeDiv(float64(stale), float64(reads)))
	fmt.Printf("Results saved to: %s\n", *outFile)
}

func safeDiv(a, b float64) float64 {
	if b == 0 { return 0 }
	return a / b
}