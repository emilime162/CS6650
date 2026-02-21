package main

import (
	"encoding/json"
	"fmt"
	"log"
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
	Products    []Product `json:"products"`
	TotalFound  int       `json:"total_found"`
	SearchTime  string    `json:"search_time"`
	ItemsChecked int      `json:"items_checked"` // for verification
}

// Global product store
var (
	productStore sync.Map
	totalProducts = 100_000
)

// Sample data arrays for generation
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

	elapsed := time.Since(start)
	log.Printf("Generated %d products in %v", totalProducts, elapsed)
}

// searchProducts checks exactly 100 products then stops.
// This simulates fixed-cost computation (e.g. running an AI model).
func searchProducts(query string) SearchResponse {
	start := time.Now()
	query = strings.ToLower(strings.TrimSpace(query))

	const maxCheck = 100 // CRITICAL: always check exactly this many
	const maxResults = 20

	var results []Product
	checked := 0
	found := 0

	// Iterate product IDs 1..totalProducts, stopping after maxCheck
	for i := 1; i <= totalProducts && checked < maxCheck; i++ {
		val, ok := productStore.Load(i)
		if !ok {
			continue
		}
		p := val.(Product)
		checked++ // count EVERY product checked, not just matches

		nameLower := strings.ToLower(p.Name)
		catLower := strings.ToLower(p.Category)

		if strings.Contains(nameLower, query) || strings.Contains(catLower, query) {
			found++
			if len(results) < maxResults {
				results = append(results, p)
			}
		}
	}

	elapsed := time.Since(start)

	return SearchResponse{
		Products:     results,
		TotalFound:   found,
		SearchTime:   elapsed.String(),
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
	log.Printf("Search endpoint: GET /products/search?q={query}")
	log.Printf("Health endpoint: GET /health")

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}