// Package leader implements the Leader-Follower distributed KV store.
//
// A single binary serves as either a Leader or a Follower depending on
// the ROLE environment variable. Three replication modes are supported,
// selected by the MODE env var:
//
//	MODE=w5r1   W=5, R=1  – synchronous writes to all nodes, cheap reads
//	MODE=w1r5   W=1, R=5  – fast writes, read from all nodes
//	MODE=quorum W=3, R=3  – balanced quorum
//
// Simulated delays (as required by the assignment):
//   - Leader sleeps 200 ms after sending each replication message.
//   - Follower sleeps 100 ms before acknowledging a write.
//   - Follower sleeps  50 ms before responding to an internal read.
package leader

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"sync"
	"time"

	"cs6650-a4/internal/store"
)

// -----------------------------------------------------------------------
// Shared message types used for both leader→follower and client→leader
// -----------------------------------------------------------------------

// SetRequest is the JSON body for a client set call or an internal
// replication call.
type SetRequest struct {
	Key     string `json:"key"`
	Value   string `json:"value"`
	Version int64  `json:"version,omitempty"` // set by leader during replication
}

// EntryResponse is returned for get / local_read / internal read calls.
type EntryResponse struct {
	Key     string `json:"key"`
	Value   string `json:"value"`
	Version int64  `json:"version"`
}

// -----------------------------------------------------------------------
// LeaderHandler – handles external client requests on the Leader node
// -----------------------------------------------------------------------

// LeaderHandler holds the leader's configuration and coordinates reads
// and writes across the cluster.
type LeaderHandler struct {
	store    *store.Store
	peers    []string // follower base URLs, e.g. ["http://follower1:8080"]
	mode     string   // "w5r1" | "w1r5" | "quorum"
}

// NewLeaderHandler constructs a LeaderHandler.
// peers is the list of follower base URLs.
func NewLeaderHandler(s *store.Store, peers []string, mode string) *LeaderHandler {
	return &LeaderHandler{store: s, peers: peers, mode: mode}
}

// RegisterRoutes attaches routes to the given mux.
func (h *LeaderHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/set", h.handleSet)
	mux.HandleFunc("/get", h.handleGet)
	mux.HandleFunc("/local_read", h.handleLocalRead)
	// Internal endpoint so the leader can also be read by the leader
	// during quorum reads.
	mux.HandleFunc("/internal/read", h.handleInternalRead)
}

// handleSet processes a write from the client.
// Behaviour differs by mode:
//
//	w5r1   – replicate to ALL followers synchronously before responding.
//	w1r5   – write locally only, fire-and-forget replication.
//	quorum – replicate to W-1=2 followers (plus self = 3 total) before responding.
func (h *LeaderHandler) handleSet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}

	var req SetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad JSON", http.StatusBadRequest)
		return
	}
	if req.Key == "" {
		http.Error(w, "key cannot be empty", http.StatusBadRequest)
		return
	}

	// Write locally first and obtain the version for this write.
	version := h.store.Set(req.Key, req.Value, 0)
	req.Version = version

	switch h.mode {
	case "w5r1":
		// Synchronous: replicate to every follower before responding.
		h.replicateSync(req, len(h.peers))
	case "w1r5":
		// Asynchronous: respond immediately, replicate in background.
		go h.replicateSync(req, len(h.peers))
	case "quorum":
		// Quorum: wait for W-1 = 2 additional nodes (self already written).
		// We send to all but only wait for 2 confirmations.
		h.replicateSync(req, 2)
		// Remaining followers updated in background.
		remaining := h.peers[2:]
		if len(remaining) > 0 {
			go h.replicateToPeers(req, remaining)
		}
	}

	w.WriteHeader(http.StatusCreated)
}

// replicateSync sends req to the first n peers and waits for all of them.
// The leader sleeps 200 ms after sending to each peer (assignment requirement).
func (h *LeaderHandler) replicateSync(req SetRequest, n int) {
	targets := h.peers
	if n < len(h.peers) {
		targets = h.peers[:n]
	}

	var wg sync.WaitGroup
	for _, peer := range targets {
		wg.Add(1)
		go func(peer string) {
			defer wg.Done()
			h.sendReplicate(peer, req)
		}(peer)
		// Assignment: sleep 200 ms after each message sent to a follower.
		time.Sleep(200 * time.Millisecond)
	}
	wg.Wait()
}

// replicateToPeers sends req to a specific list of peers (fire-and-forget
// goroutines, but the function itself blocks until all are launched).
func (h *LeaderHandler) replicateToPeers(req SetRequest, peers []string) {
	for _, peer := range peers {
		go func(p string) {
			h.sendReplicate(p, req)
		}(peer)
		time.Sleep(200 * time.Millisecond)
	}
}

// sendReplicate does the actual HTTP POST to a follower's /internal/replicate.
func (h *LeaderHandler) sendReplicate(peer string, req SetRequest) {
	body, _ := json.Marshal(req)
	resp, err := http.Post(peer+"/internal/replicate", "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("replication to %s failed: %v", peer, err)
		return
	}
	resp.Body.Close()
}

// handleGet processes a read from the client.
//
//	w5r1   – read from local store only (R=1).
//	w1r5   – fetch from all 5 nodes, return highest version (R=5).
//	quorum – fetch from 3 nodes (self + 2 followers), return highest version (R=3).
func (h *LeaderHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "key required", http.StatusBadRequest)
		return
	}

	var entry *store.Entry

	switch h.mode {
	case "w5r1":
		// R=1: just read locally.
		e, ok := h.store.Get(key)
		if !ok {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		entry = e

	case "w1r5":
		// R=5: collect from all 5 nodes (self + 4 followers).
		entry = h.collectQuorum(key, h.peers)

	case "quorum":
		// R=3: collect from self + 2 followers.
		entry = h.collectQuorum(key, h.peers[:2])
	}

	if entry == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(EntryResponse{
		Key:     key,
		Value:   entry.Value,
		Version: entry.Version,
	})
}

// collectQuorum reads from the local store plus the given peers and returns
// the entry with the highest version. Returns nil if no node has the key.
func (h *LeaderHandler) collectQuorum(key string, peers []string) *store.Entry {
	type result struct {
		entry *store.Entry
	}

	results := make([]result, 0, len(peers)+1)
	var mu sync.Mutex

	// Include local store result.
	if e, ok := h.store.Get(key); ok {
		mu.Lock()
		results = append(results, result{e})
		mu.Unlock()
	}

	var wg sync.WaitGroup
	for _, peer := range peers {
		wg.Add(1)
		go func(peer string) {
			defer wg.Done()
			e := h.fetchFromPeer(peer, key)
			if e != nil {
				mu.Lock()
				results = append(results, result{e})
				mu.Unlock()
			}
		}(peer)
	}
	wg.Wait()

	if len(results) == 0 {
		return nil
	}

	// Return the entry with the highest version.
	sort.Slice(results, func(i, j int) bool {
		return results[i].entry.Version > results[j].entry.Version
	})
	return results[0].entry
}

// fetchFromPeer calls a follower's /internal/read endpoint.
func (h *LeaderHandler) fetchFromPeer(peer, key string) *store.Entry {
	url := fmt.Sprintf("%s/internal/read?key=%s", peer, key)
	resp, err := http.Get(url)
	if err != nil || resp.StatusCode == http.StatusNotFound {
		return nil
	}
	defer resp.Body.Close()

	var er EntryResponse
	if err := json.NewDecoder(resp.Body).Decode(&er); err != nil {
		return nil
	}
	return &store.Entry{Value: er.Value, Version: er.Version}
}

// handleLocalRead returns the raw local value without coordination.
// Used only during testing to inspect internal state mid-replication.
func (h *LeaderHandler) handleLocalRead(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	e, ok := h.store.Get(key)
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(EntryResponse{Key: key, Value: e.Value, Version: e.Version})
}

// handleInternalRead serves the leader's own data to quorum collectors.
// Sleeps 50 ms to simulate real read latency.
func (h *LeaderHandler) handleInternalRead(w http.ResponseWriter, r *http.Request) {
	time.Sleep(50 * time.Millisecond)
	h.handleLocalRead(w, r)
}

// -----------------------------------------------------------------------
// FollowerHandler – handles internal replication calls from the Leader
// -----------------------------------------------------------------------

// FollowerHandler exposes the follower endpoints.
type FollowerHandler struct {
	store *store.Store
}

// NewFollowerHandler constructs a FollowerHandler.
func NewFollowerHandler(s *store.Store) *FollowerHandler {
	return &FollowerHandler{store: s}
}

// RegisterRoutes attaches routes to the given mux.
func (h *FollowerHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/set", h.handleClientSet) // client reads allowed from followers
	mux.HandleFunc("/get", h.handleClientGet)
	mux.HandleFunc("/local_read", h.handleLocalRead)
	mux.HandleFunc("/internal/replicate", h.handleReplicate)
	mux.HandleFunc("/internal/read", h.handleInternalRead)
}

// handleClientSet on a follower: reject writes from clients with 403.
// All writes must go to the leader.
func (h *FollowerHandler) handleClientSet(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "writes must go to the leader", http.StatusForbidden)
}

// handleClientGet returns the follower's local value for the key.
func (h *FollowerHandler) handleClientGet(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	e, ok := h.store.Get(key)
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(EntryResponse{Key: key, Value: e.Value, Version: e.Version})
}

// handleReplicate receives a write from the leader, sleeps 100 ms, then
// stores the value.
func (h *FollowerHandler) handleReplicate(w http.ResponseWriter, r *http.Request) {
	var req SetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad JSON", http.StatusBadRequest)
		return
	}

	// Assignment requirement: follower sleeps 100 ms on write.
	time.Sleep(100 * time.Millisecond)
	h.store.Set(req.Key, req.Value, req.Version)
	w.WriteHeader(http.StatusCreated)
}

// handleInternalRead responds to leader's quorum read. Sleeps 50 ms first.
func (h *FollowerHandler) handleInternalRead(w http.ResponseWriter, r *http.Request) {
	time.Sleep(50 * time.Millisecond)
	h.handleLocalRead(w, r)
}

// handleLocalRead returns raw local value, no delay.
func (h *FollowerHandler) handleLocalRead(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	e, ok := h.store.Get(key)
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(EntryResponse{Key: key, Value: e.Value, Version: e.Version})
}
