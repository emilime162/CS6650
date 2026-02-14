package main

import (
	"fmt"
	"log"
	"net/http"

	"product-api/handlers"
	"product-api/store"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	productStore := store.NewProductStore()
	productHandler := handlers.NewProductHandler(productStore)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/products/{productId}", productHandler.GetProduct)
	r.Post("/products/{productId}/details", productHandler.AddProductDetails)

	port := 8080
	fmt.Printf("Product API server starting on :%d\n", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), r))
}