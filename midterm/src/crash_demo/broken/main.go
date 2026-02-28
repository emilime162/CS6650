package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Product represents a single product in the catalog
type Product struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	Description string `json:"description"`
	Brand       string `json:"brand"`
}

// SearchResponse is the API response format
type SearchResponse struct {
	Products     []Product `json:"products"`
	TotalFound   int       `json:"total_found"`
	SearchTime   string    `json:"search_time"`
	ItemsChecked int       `json:"items_checked"`
}

// Global product store
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
		product := Product{
			ID:          i,
			Name:        fmt.Sprintf("Product %s %d", brand, i),
			Category:    categories[i%len(categories)],
			Description: descriptions[i%len(descriptions)],
			Brand:       brand,
		}
		productStore.Store(i, product)
	}
	log.Printf("Generated %d products in %v", totalProducts, time.Since(start))
}

// aiRanker simulates calling an external AI ranking service.
// BUG: no timeout — 30% of calls hang for 30 seconds.
// Under load, goroutines pile up waiting here forever.
func aiRanker(products []Product) []Product {
	if rand.Float32() < 0.30 { // 30% chance of a slow/hung call
		log.Println("[aiRanker] SLOW: ranking service is hanging...")
		time.Sleep(30 * time.Second) // simulates a hung downstream service
	}
	// Normally returns instantly (fast path)
	return products
}

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

	// BUG: aiRanker is called with no timeout or protection.
	// If it hangs, this goroutine is blocked forever.
	results = aiRanker(results)

	return SearchResponse{
		Products:     results,
		TotalFound:   len(results),
		SearchTime:   time.Since(start).String(),
		ItemsChecked: checked,
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
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func main() {
	generateProducts()
	http.HandleFunc("/products/search", searchHandler)
	http.HandleFunc("/health", healthHandler)

	port := "8080"
	log.Printf("Server starting on port %s", port)
	log.Printf("*** BROKEN VERSION: aiRanker has no timeout or circuit protection ***")
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}