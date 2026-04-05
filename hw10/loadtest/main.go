// loadtest/main.go – load test client for the KV cluster.
//
// The client uses a SMALL key pool (default 20 keys) so that reads and
// writes frequently land on the same key within a short time window.
// This is what makes stale reads observable during load testing.
//
// For each request the client tracks the last-written version of every
// key. If a read returns a version lower than what was written, that is
// a stale read and is counted.
//
// Usage:
//
//	go run loadtest/main.go \
//	  -leader   http://localhost:8080 \
//	  -followers http://localhost:8081,http://localhost:8082,http://localhost:8083,http://localhost:8084 \
//	  -requests 2000 \
//	  -concurrency 20 \
//	  -write-pct 10 \
//	  -key-pool  20 \
//	  -out results.csv
//
// For leaderless clusters use -leaderless and pass all 5 node URLs as followers.
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

// -----------------------------------------------------------------------
// CLI flags
// -----------------------------------------------------------------------

var (
	leaderURL   = flag.String("leader", "http://localhost:8080", "Leader (or any node for leaderless) base URL")
	followersRaw = flag.String("followers", "http://localhost:8081,http://localhost:8082,http://localhost:8083,http://localhost:8084", "Comma-separated follower URLs (all nodes for leaderless)")
	requests    = flag.Int("requests", 2000, "Total number of requests")
	concurrency = flag.Int("concurrency", 20, "Number of concurrent workers")
	writePct    = flag.Int("write-pct", 10, "Percentage of requests that are writes (0-100)")
	keyPool     = flag.Int("key-pool", 20, "Number of distinct keys to use")
	outFile     = flag.String("out", "results.csv", "Output CSV file")
	leaderless  = flag.Bool("leaderless", false, "Leaderless mode: writes go to any node")
)

// -----------------------------------------------------------------------
// Result record
// -----------------------------------------------------------------------

type record struct {
	Op        string        // "read" or "write"
	Key       string
	LatencyMs float64
	Stale     bool   // true if read returned lower version than last written
	Version   int64
}

// -----------------------------------------------------------------------
// Shared version tracker
// -----------------------------------------------------------------------

type versionTracker struct {
	mu       sync.RWMutex
	versions map[string]int64 // key → last written version
}

func newVersionTracker() *versionTracker {
	return &versionTracker{versions: make(map[string]int64)}
}

func (vt *versionTracker) setWritten(key string, ver int64) {
	vt.mu.Lock()
	defer vt.mu.Unlock()
	if ver > vt.versions[key] {
		vt.versions[key] = ver
	}
}

func (vt *versionTracker) lastWritten(key string) int64 {
	vt.mu.RLock()
	defer vt.mu.RUnlock()
	return vt.versions[key]
}

// -----------------------------------------------------------------------
// HTTP helpers
// -----------------------------------------------------------------------

type entryResp struct {
	Value   string `json:"value"`
	Version int64  `json:"version"`
}

var client = &http.Client{Timeout: 10 * time.Second}

func doWrite(baseURL, key, value string) (int64, error) {
	body, _ := json.Marshal(map[string]string{"key": key, "value": value})
	resp, err := client.Post(baseURL+"/set", "application/json", bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	// The version is only returned by GET; on a write we use the tracker.
	return 0, nil
}

func doRead(baseURL, key string) (*entryResp, error) {
	resp, err := client.Get(fmt.Sprintf("%s/get?key=%s", baseURL, key))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	var e entryResp
	json.NewDecoder(resp.Body).Decode(&e)
	return &e, nil
}

// -----------------------------------------------------------------------
// Worker
// -----------------------------------------------------------------------

func worker(
	id int,
	jobs <-chan struct{},
	results chan<- record,
	keys []string,
	followers []string,
	vt *versionTracker,
) {
	for range jobs {
		key := keys[rand.Intn(len(keys))]
		isWrite := rand.Intn(100) < *writePct

		if isWrite {
			// Choose write target: leader (or random node if leaderless).
			var writeURL string
			if *leaderless {
				all := append([]string{*leaderURL}, followers...)
				writeURL = all[rand.Intn(len(all))]
			} else {
				writeURL = *leaderURL
			}

			value := fmt.Sprintf("v-%d-%d", id, time.Now().UnixNano())
			start := time.Now()
			_, err := doWrite(writeURL, key, value)
			latency := time.Since(start)

			if err != nil {
				log.Printf("write error: %v", err)
				continue
			}

			// We can't easily get the version from a set response, so we do
			// a local_read immediately after to capture the written version.
			re, _ := doRead(writeURL, key)
			var ver int64
			if re != nil {
				ver = re.Version
				vt.setWritten(key, ver)
			}

			results <- record{
				Op:        "write",
				Key:       key,
				LatencyMs: float64(latency.Milliseconds()),
				Version:   ver,
			}

		} else {
			// Read from a random follower (or random node if leaderless).
			var readURL string
			if *leaderless {
				all := append([]string{*leaderURL}, followers...)
				readURL = all[rand.Intn(len(all))]
			} else {
				all := append([]string{*leaderURL}, followers...)
				readURL = all[rand.Intn(len(all))]
			}

			start := time.Now()
			e, err := doRead(readURL, key)
			latency := time.Since(start)

			if err != nil {
				log.Printf("read error: %v", err)
				continue
			}

			var ver int64
			stale := false
			if e != nil {
				ver = e.Version
				last := vt.lastWritten(key)
				if last > 0 && ver < last {
					stale = true
				}
			}

			results <- record{
				Op:        "read",
				Key:       key,
				LatencyMs: float64(latency.Milliseconds()),
				Stale:     stale,
				Version:   ver,
			}
		}
	}
}

// -----------------------------------------------------------------------
// Main
// -----------------------------------------------------------------------

func main() {
	flag.Parse()

	followers := strings.Split(*followersRaw, ",")

	// Build the key pool.
	keys := make([]string, *keyPool)
	for i := range keys {
		keys[i] = fmt.Sprintf("key%04d", i)
	}

	jobs := make(chan struct{}, *requests)
	results := make(chan record, *requests)
	vt := newVersionTracker()

	// Launch workers.
	var wg sync.WaitGroup
	for i := 0; i < *concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			worker(id, jobs, results, keys, followers, vt)
		}(i)
	}

	// Feed jobs.
	for i := 0; i < *requests; i++ {
		jobs <- struct{}{}
	}
	close(jobs)

	// Wait then close results.
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results.
	var records []record
	for r := range results {
		records = append(records, r)
	}

	// Write CSV.
	f, err := os.Create(*outFile)
	if err != nil {
		log.Fatalf("cannot create output file: %v", err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	w.Write([]string{"op", "key", "latency_ms", "stale", "version"})
	for _, r := range records {
		w.Write([]string{
			r.Op,
			r.Key,
			strconv.FormatFloat(r.LatencyMs, 'f', 2, 64),
			strconv.FormatBool(r.Stale),
			strconv.FormatInt(r.Version, 10),
		})
	}
	w.Flush()

	// Print summary.
	var reads, writes, stale int
	var totalReadLatency, totalWriteLatency float64
	for _, r := range records {
		if r.Op == "read" {
			reads++
			totalReadLatency += r.LatencyMs
			if r.Stale {
				stale++
			}
		} else {
			writes++
			totalWriteLatency += r.LatencyMs
		}
	}

	fmt.Printf("\n===== Load Test Summary =====\n")
	fmt.Printf("Total requests:   %d\n", len(records))
	fmt.Printf("Writes:           %d  avg latency: %.1f ms\n", writes, safeDiv(totalWriteLatency, float64(writes)))
	fmt.Printf("Reads:            %d  avg latency: %.1f ms\n", reads, safeDiv(totalReadLatency, float64(reads)))
	fmt.Printf("Stale reads:      %d (%.1f%%)\n", stale, 100*safeDiv(float64(stale), float64(reads)))
	fmt.Printf("Results saved to: %s\n", *outFile)
}

func safeDiv(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return a / b
}
