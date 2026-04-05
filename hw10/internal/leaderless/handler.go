// Package leaderless implements the Leaderless distributed KV store.
//
// Any node can accept any request.  The node that receives a write becomes
// the Write Coordinator for that request and propagates it to all peers
// (W=N) before responding 201.  Reads are served locally (R=1), so a read
// that arrives at a not-yet-updated node returns a stale value – this is
// the inconsistency window the assignment asks you to demonstrate.
package leaderless

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"cs6650-a4/internal/store"
)

// SetRequest is the JSON body for client and internal write calls.
type SetRequest struct {
	Key     string `json:"key"`
	Value   string `json:"value"`
	Version int64  `json:"version,omitempty"`
}

// EntryResponse is returned for get / local_read calls.
type EntryResponse struct {
	Key     string `json:"key"`
	Value   string `json:"value"`
	Version int64  `json:"version"`
}

// Handler is a leaderless node. Every node is identical; there is no
// special leader.
type Handler struct {
	store *store.Store
	peers []string // base URLs of the other nodes in the cluster
}

// New constructs a Handler.
// peers must NOT include the current node's own URL.
func New(s *store.Store, peers []string) *Handler {
	return &Handler{store: s, peers: peers}
}

// RegisterRoutes attaches all routes to mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/set", h.handleSet)
	mux.HandleFunc("/get", h.handleGet)
	mux.HandleFunc("/local_read", h.handleLocalRead)
	mux.HandleFunc("/internal/replicate", h.handleReplicate)
}

// handleSet makes this node the Write Coordinator.
// It writes locally first to obtain a version, then propagates to every
// peer and waits for all of them before returning 201 (W=N).
//
// Delay model:
//   - Coordinator sleeps 200 ms after dispatching each peer message.
//   - Each peer sleeps 100 ms before writing (enforced in handleReplicate).
func (h *Handler) handleSet(w http.ResponseWriter, r *http.Request) {
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

	// Write locally first; this establishes the canonical version.
	req.Version = h.store.Set(req.Key, req.Value, 0)

	// W=N: propagate to every other node and wait for all acks.
	var wg sync.WaitGroup
	for _, peer := range h.peers {
		wg.Add(1)
		go func(peer string) {
			defer wg.Done()
			h.sendReplicate(peer, req)
		}(peer)
		// Assignment: coordinator sleeps 200 ms after each message sent.
		time.Sleep(200 * time.Millisecond)
	}
	wg.Wait()

	w.WriteHeader(http.StatusCreated)
}

// sendReplicate POSTs a replication message to one peer.
func (h *Handler) sendReplicate(peer string, req SetRequest) {
	body, _ := json.Marshal(req)
	resp, err := http.Post(peer+"/internal/replicate", "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("leaderless: replication to %s failed: %v", peer, err)
		return
	}
	resp.Body.Close()
}

// handleGet returns this node's local value for the key (R=1).
// No coordination – reads are served from local state only.
// If this node hasn't received the replication yet, the caller gets a
// stale value; that is the inconsistency window.
func (h *Handler) handleGet(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "key required", http.StatusBadRequest)
		return
	}

	e, ok := h.store.Get(key)
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(EntryResponse{
		Key:     key,
		Value:   e.Value,
		Version: e.Version,
	})
}

// handleLocalRead is identical to handleGet but exists as a named
// endpoint for tests that want to be explicit about inspecting raw
// local state (no coordination, no delay).
func (h *Handler) handleLocalRead(w http.ResponseWriter, r *http.Request) {
	h.handleGet(w, r)
}

// handleReplicate receives a write from the Write Coordinator.
// Sleeps 100 ms to simulate storage write latency before committing.
func (h *Handler) handleReplicate(w http.ResponseWriter, r *http.Request) {
	var req SetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad JSON", http.StatusBadRequest)
		return
	}

	// Assignment requirement: peer sleeps 100 ms on receiving a write.
	time.Sleep(100 * time.Millisecond)
	h.store.Set(req.Key, req.Value, req.Version)

	w.WriteHeader(http.StatusCreated)
	fmt.Fprint(w, "ok")
}
