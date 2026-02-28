package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ============================================================
// DOMAIN TYPES (unchanged from original)
// ============================================================

type Product struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	Description string `json:"description"`
	Brand       string `json:"brand"`
}

type SearchResponse struct {
	Products     []Product `json:"products"`
	TotalFound   int       `json:"total_found"`
	SearchTime   string    `json:"search_time"`
	ItemsChecked int       `json:"items_checked"`
	RankerStatus string    `json:"ranker_status"` // NEW: shows which path was taken
}

var (
	productStore  sync.Map
	totalProducts = 100_000
)

var (
	brands = []string{
		"Alpha", "Beta", "Gamma", "Delta", "Epsilon",
		"Zeta", "Eta", "Theta", "Iota", "Kappa",
	}
	categories = []string{
		"Electronics", "Books", "Home", "Sports", "Clothing",
		"Toys", "Garden", "Automotive", "Health", "Beauty",
	}
	descriptions = []string{
		"High quality product with excellent durability.",
		"Perfect for everyday use and professional settings.",
		"Designed for maximum performance and reliability.",
		"Eco-friendly materials with modern design.",
		"Best-in-class features at an affordable price.",
	}
)

func generateProducts() {
	log.Printf("Generating %d products...", totalProducts)
	start := time.Now()
	for i := 1; i <= totalProducts; i++ {
		brand := brands[i%len(brands)]
		productStore.Store(i, Product{
			ID:          i,
			Name:        fmt.Sprintf("Product %s %d", brand, i),
			Category:    categories[i%len(categories)],
			Description: descriptions[i%len(descriptions)],
			Brand:       brand,
		})
	}
	log.Printf("Generated %d products in %v", totalProducts, time.Since(start))
}

// ============================================================
// FIX 1: FAIL FAST — context timeout on the ranker call
// If aiRanker doesn't respond in 500ms, we abort and
// return results without ranking rather than hanging forever.
// ============================================================

const rankerTimeout = 500 * time.Millisecond

func aiRankerWithTimeout(ctx context.Context, products []Product) ([]Product, error) {
	resultCh := make(chan []Product, 1)

	go func() {
		// Simulate the slow/hung dependency (same 30% hang as broken version)
		if rand.Float32() < 0.30 {
			log.Println("[aiRanker] SLOW: ranking service is hanging...")
			time.Sleep(30 * time.Second)
		}
		resultCh <- products
	}()

	select {
	case result := <-resultCh:
		return result, nil
	case <-ctx.Done():
		// FAIL FAST: context expired, don't wait any longer
		return products, errors.New("ranker timeout: returning unranked results")
	}
}

// ============================================================
// FIX 2: CIRCUIT BREAKER
// After 5 consecutive failures (timeouts), the breaker OPENS.
// Open breaker: skip the ranker entirely for 10 seconds.
// After 10s it goes HALF-OPEN: try one request to probe recovery.
// ============================================================

type CircuitState int

const (
	StateClosed   CircuitState = iota // normal operation
	StateOpen                         // failing fast, not calling ranker
	StateHalfOpen                     // probing: allow one request through
)

type CircuitBreaker struct {
	mu           sync.Mutex
	state        CircuitState
	failures     int
	lastFailure  time.Time
	threshold    int           // failures before opening
	resetTimeout time.Duration // how long to stay open
}

func NewCircuitBreaker(threshold int, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:        StateClosed,
		threshold:    threshold,
		resetTimeout: resetTimeout,
	}
}

func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		// Check if enough time has passed to try again
		if time.Since(cb.lastFailure) > cb.resetTimeout {
			cb.state = StateHalfOpen
			log.Println("[circuit breaker] HALF-OPEN: probing ranker service")
			return true
		}
		return false // still open, fail fast
	case StateHalfOpen:
		return true // let the probe through
	}
	return false
}

func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures = 0
	cb.state = StateClosed
	log.Println("[circuit breaker] CLOSED: ranker service recovered")
}

func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures++
	cb.lastFailure = time.Now()
	if cb.failures >= cb.threshold || cb.state == StateHalfOpen {
		cb.state = StateOpen
		log.Printf("[circuit breaker] OPEN after %d failures — skipping ranker for %v",
			cb.failures, cb.resetTimeout)
	}
}

func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}

// ============================================================
// FIX 3: BULKHEAD
// Limit concurrent calls to the ranker to 10 at a time.
// If 10 goroutines are already waiting, reject immediately
// rather than queuing up and exhausting all goroutines.
// ============================================================

type Bulkhead struct {
	sem     chan struct{}
	rejected atomic.Int64
}

func NewBulkhead(maxConcurrent int) *Bulkhead {
	return &Bulkhead{
		sem: make(chan struct{}, maxConcurrent),
	}
}

func (b *Bulkhead) Acquire() bool {
	select {
	case b.sem <- struct{}{}:
		return true
	default:
		b.rejected.Add(1)
		return false // bulkhead full, reject immediately
	}
}

func (b *Bulkhead) Release() {
	<-b.sem
}

func (b *Bulkhead) RejectedCount() int64 {
	return b.rejected.Load()
}

// ============================================================
// WIRING: compose all three patterns around aiRanker
// ============================================================

var (
	breaker  = NewCircuitBreaker(5, 10*time.Second)
	bulkhead = NewBulkhead(10) // max 10 concurrent ranker calls
)

// callRankerSafely applies bulkhead → circuit breaker → fail fast, in order.
// Returns (ranked results, status string for the response body).
func callRankerSafely(products []Product) ([]Product, string) {
	// Layer 3: Bulkhead — don't let too many goroutines pile up
	if !bulkhead.Acquire() {
		log.Printf("[bulkhead] REJECTED (total rejected: %d)", bulkhead.RejectedCount())
		return products, "bulkhead_rejected"
	}
	defer bulkhead.Release()

	// Layer 2: Circuit breaker — if ranker is known-bad, skip immediately
	if !breaker.Allow() {
		return products, "circuit_open"
	}

	// Layer 1: Fail fast — enforce a hard timeout on the ranker call
	ctx, cancel := context.WithTimeout(context.Background(), rankerTimeout)
	defer cancel()

	result, err := aiRankerWithTimeout(ctx, products)
	if err != nil {
		breaker.RecordFailure()
		log.Printf("[fail fast] ranker timed out: %v", err)
		return result, "timeout_fallback"
	}

	breaker.RecordSuccess()
	return result, "ranked"
}

// ============================================================
// HTTP HANDLERS (same structure as original)
// ============================================================

func searchProducts(query string) SearchResponse {
	start := time.Now()
	query = strings.ToLower(strings.TrimSpace(query))

	const maxCheck = 100
	const maxResults = 20

	var results []Product
	checked := 0

	for i := 1; i <= totalProducts && checked < maxCheck; i++ {
		val, ok := productStore.Load(i)
		if !ok {
			continue
		}
		p := val.(Product)
		checked++
		nameLower := strings.ToLower(p.Name)
		catLower := strings.ToLower(p.Category)
		if strings.Contains(nameLower, query) || strings.Contains(catLower, query) {
			if len(results) < maxResults {
				results = append(results, p)
			}
		}
	}

	// FIXED: all three Newman patterns protect this call
	results, rankerStatus := callRankerSafely(results)

	return SearchResponse{
		Products:     results,
		TotalFound:   len(results),
		SearchTime:   time.Since(start).String(),
		ItemsChecked: checked,
		RankerStatus: rankerStatus, // visible in response for demo
	}
}

func searchHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		http.Error(w, `{"error":"missing query parameter q"}`, http.StatusBadRequest)
		return
	}
	response := searchProducts(query)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":           "ok",
		"circuit_state":    fmt.Sprintf("%d", breaker.State()), // 0=closed, 1=open, 2=half-open
		"bulkhead_rejected": fmt.Sprintf("%d", bulkhead.RejectedCount()),
	})
}

func main() {
	generateProducts()
	http.HandleFunc("/products/search", searchHandler)
	http.HandleFunc("/health", healthHandler)

	port := "8080"
	log.Printf("Server starting on port %s", port)
	log.Printf("*** FIXED VERSION: fail fast + circuit breaker + bulkhead active ***")
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}