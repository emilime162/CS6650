// Package tests runs black-box consistency tests against live clusters.
//
// These tests are integration tests – they require the Docker Compose
// cluster to be running before you execute them.
//
// Leader-Follower (ports 8080-8084):
//
//	docker compose -f docker/compose-lf-w5r1.yml up --build -d
//	go test ./tests/... -v -run TestLeader
//
// Leaderless (ports 9080-9084):
//
//	docker compose -f docker/compose-leaderless.yml up --build -d
//	go test ./tests/... -v -run TestLeaderless
package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"testing"
	"time"
)

// -----------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------

type entryResp struct {
	Key     string `json:"key"`
	Value   string `json:"value"`
	Version int64  `json:"version"`
}

func doSet(t *testing.T, baseURL, key, value string) int {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"key": key, "value": value})
	resp, err := http.Post(baseURL+"/set", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("set request failed: %v", err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

func doGet(t *testing.T, baseURL, key string) (*entryResp, int) {
	t.Helper()
	resp, err := http.Get(fmt.Sprintf("%s/get?key=%s", baseURL, key))
	if err != nil {
		t.Fatalf("get request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, resp.StatusCode
	}
	var e entryResp
	json.NewDecoder(resp.Body).Decode(&e)
	return &e, resp.StatusCode
}

func doLocalRead(t *testing.T, baseURL, key string) (*entryResp, int) {
	t.Helper()
	resp, err := http.Get(fmt.Sprintf("%s/local_read?key=%s", baseURL, key))
	if err != nil {
		t.Fatalf("local_read request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, resp.StatusCode
	}
	var e entryResp
	json.NewDecoder(resp.Body).Decode(&e)
	return &e, resp.StatusCode
}

func uniqueKey() string {
	return fmt.Sprintf("testkey-%d", rand.Int63())
}

// -----------------------------------------------------------------------
// Leader-Follower tests
// -----------------------------------------------------------------------

const (
	leaderURL    = "http://localhost:8080"
	follower1URL = "http://localhost:8081"
	follower2URL = "http://localhost:8082"
	follower3URL = "http://localhost:8083"
	follower4URL = "http://localhost:8084"
)

var followerURLs = []string{follower1URL, follower2URL, follower3URL, follower4URL}

// TestLeaderWriteThenReadFromLeader writes to the leader and immediately
// reads back from the leader. Must always return the written value because
// the leader writes locally before replicating.
func TestLeaderWriteThenReadFromLeader(t *testing.T) {
	key := uniqueKey()
	value := "consistent-leader-value"

	status := doSet(t, leaderURL, key, value)
	if status != http.StatusCreated {
		t.Fatalf("expected 201, got %d", status)
	}

	e, code := doGet(t, leaderURL, key)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if e.Value != value {
		t.Errorf("leader read stale: got %q, want %q", e.Value, value)
	}
	t.Logf("Leader read: key=%s value=%s version=%d", e.Key, e.Value, e.Version)
}

// TestLeaderWriteThenReadFromFollower writes to the leader and reads from
// each follower after the leader has acknowledged the write. In W=5 mode
// the followers must all be updated before the 201 was sent, so reads
// here should always be consistent.
func TestLeaderWriteThenReadFromFollower(t *testing.T) {
	key := uniqueKey()
	value := "follower-consistency-check"

	status := doSet(t, leaderURL, key, value)
	if status != http.StatusCreated {
		t.Fatalf("expected 201, got %d", status)
	}

	for i, fu := range followerURLs {
		e, code := doGet(t, fu, key)
		if code != http.StatusOK {
			t.Errorf("follower%d: expected 200, got %d", i+1, code)
			continue
		}
		if e.Value != value {
			t.Errorf("follower%d returned stale value %q, want %q", i+1, e.Value, value)
		} else {
			t.Logf("follower%d: consistent – version=%d", i+1, e.Version)
		}
	}
}

// TestLocalReadDuringReplicationWindow fires set to the leader and then
// immediately local_reads every follower BEFORE the write window closes.
// In W=5 mode with 200 ms + 100 ms delays per follower this test has a
// good chance of catching at least one inconsistent read. Results are
// logged rather than failing the test because inconsistency is expected
// and is the whole point.
func TestLocalReadDuringReplicationWindow(t *testing.T) {
	key := uniqueKey()
	value := "mid-window-read"

	// Fire the set in a goroutine so we can immediately start local_reads.
	done := make(chan struct{})
	go func() {
		body, _ := json.Marshal(map[string]string{"key": key, "value": value})
		http.Post(leaderURL+"/set", "application/json", bytes.NewReader(body))
		close(done)
	}()

	// Give the leader a moment to write locally and start replication.
	time.Sleep(50 * time.Millisecond)

	staleCount := 0
	for i, fu := range followerURLs {
		e, code := doLocalRead(t, fu, key)
		if code == http.StatusNotFound {
			t.Logf("follower%d: key not yet present (stale)", i+1)
			staleCount++
		} else if e != nil && e.Value != value {
			t.Logf("follower%d: stale value %q", i+1, e.Value)
			staleCount++
		} else if e != nil {
			t.Logf("follower%d: already consistent version=%d", i+1, e.Version)
		}
	}
	<-done
	t.Logf("Stale reads during window: %d / %d", staleCount, len(followerURLs))
}

// TestRepeatedLocalReadFindsInconsistency repeats the window test many
// times at high concurrency to maximise the chance of catching stale reads.
func TestRepeatedLocalReadFindsInconsistency(t *testing.T) {
	totalStale := 0
	trials := 20
	for i := 0; i < trials; i++ {
		key := uniqueKey()
		value := fmt.Sprintf("val-%d", i)

		go func(k, v string) {
			body, _ := json.Marshal(map[string]string{"key": k, "value": v})
			http.Post(leaderURL+"/set", "application/json", bytes.NewReader(body))
		}(key, value)
		time.Sleep(30 * time.Millisecond)

		for _, fu := range followerURLs {
			e, code := doLocalRead(t, fu, key)
			if code == http.StatusNotFound || (e != nil && e.Value != value) {
				totalStale++
			}
		}
	}
	t.Logf("Total stale local_reads across %d trials: %d", trials, totalStale)
	if totalStale == 0 {
		t.Log("No stale reads observed – try increasing concurrency or sleep durations")
	}
}

// -----------------------------------------------------------------------
// Leaderless tests
// -----------------------------------------------------------------------

var leaderlessURLs = []string{
	"http://localhost:9080",
	"http://localhost:9081",
	"http://localhost:9082",
	"http://localhost:9083",
	"http://localhost:9084",
}

// TestLeaderlessInconsistencyWindow writes to a random node (Write
// Coordinator) and immediately reads from the other nodes before the
// coordinator has finished propagating. Stale reads are expected and
// are the point of this test.
func TestLeaderlessInconsistencyWindow(t *testing.T) {
	coordinatorURL := leaderlessURLs[rand.Intn(len(leaderlessURLs))]
	key := uniqueKey()
	value := "leaderless-write"

	done := make(chan struct{})
	go func() {
		body, _ := json.Marshal(map[string]string{"key": key, "value": value})
		http.Post(coordinatorURL+"/set", "application/json", bytes.NewReader(body))
		close(done)
	}()

	// Read from other nodes during the propagation window.
	time.Sleep(60 * time.Millisecond)

	staleCount := 0
	for _, url := range leaderlessURLs {
		if url == coordinatorURL {
			continue // coordinator already has the value
		}
		e, code := doGet(t, url, key)
		if code == http.StatusNotFound {
			t.Logf("node %s: key not yet present (stale)", url)
			staleCount++
		} else if e != nil && e.Value != value {
			t.Logf("node %s: stale value %q", url, e.Value)
			staleCount++
		} else if e != nil {
			t.Logf("node %s: consistent version=%d", url, e.Version)
		}
	}
	<-done
	t.Logf("Stale reads during leaderless window: %d / %d", staleCount, len(leaderlessURLs)-1)
}

// TestLeaderlessReadAfterWriteCoordinator waits for the coordinator to
// acknowledge the write (W=N, so all nodes are updated) then reads from
// the coordinator. Must be consistent.
func TestLeaderlessReadAfterWriteCoordinator(t *testing.T) {
	coordinatorURL := leaderlessURLs[0]
	key := uniqueKey()
	value := "after-coordinator-ack"

	status := doSet(t, coordinatorURL, key, value)
	if status != http.StatusCreated {
		t.Fatalf("expected 201, got %d", status)
	}

	e, code := doGet(t, coordinatorURL, key)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if e.Value != value {
		t.Errorf("coordinator stale: got %q, want %q", e.Value, value)
	}
	t.Logf("Coordinator read after ack: version=%d", e.Version)
}

// TestLeaderlessReadAfterAckFromAnyNode after W=N ack, all nodes must
// have the value.
func TestLeaderlessReadAfterAckFromAnyNode(t *testing.T) {
	coordinatorURL := leaderlessURLs[rand.Intn(len(leaderlessURLs))]
	key := uniqueKey()
	value := "all-nodes-consistent"

	status := doSet(t, coordinatorURL, key, value)
	if status != http.StatusCreated {
		t.Fatalf("expected 201, got %d", status)
	}

	for _, url := range leaderlessURLs {
		e, code := doGet(t, url, key)
		if code != http.StatusOK {
			t.Errorf("node %s: expected 200, got %d", url, code)
			continue
		}
		if e.Value != value {
			t.Errorf("node %s: got %q, want %q", url, e.Value, value)
		} else {
			t.Logf("node %s: consistent version=%d", url, e.Version)
		}
	}
}
