package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"product-api/models"
	"product-api/store"

	"github.com/go-chi/chi/v5"
)

type ProductHandler struct {
	Store *store.ProductStore
}

func NewProductHandler(s *store.ProductStore) *ProductHandler {
	return &ProductHandler{Store: s}
}

// GetProduct handles GET /products/{productId}
// Responses: 200 (found), 400 (bad input), 404 (not found), 500 (server error)
func (h *ProductHandler) GetProduct(w http.ResponseWriter, r *http.Request) {
	productID, err := parseProductID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}

	product, err := h.Store.GetProduct(productID)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, product)
}

// AddProductDetails handles POST /products/{productId}/details
// Responses: 204 (success), 400 (bad input), 404 (not found), 500 (server error)
func (h *ProductHandler) AddProductDetails(w http.ResponseWriter, r *http.Request) {
	productID, err := parseProductID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}

	var product models.Product
	if err := json.NewDecoder(r.Body).Decode(&product); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_INPUT", "Invalid JSON: "+err.Error())
		return
	}

	if err := validateProduct(&product); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}

	if err := h.Store.UpsertProduct(productID, &product); err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// --- Helpers ---

func parseProductID(r *http.Request) (int, error) {
	idStr := chi.URLParam(r, "productId")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return 0, fmt.Errorf("productId must be an integer")
	}
	if id < 1 {
		return 0, fmt.Errorf("productId must be >= 1")
	}
	return id, nil
}

func validateProduct(p *models.Product) error {
	if p.SKU == "" {
		return fmt.Errorf("sku is required")
	}
	if len(p.SKU) > 100 {
		return fmt.Errorf("sku must be at most 100 characters")
	}
	if p.Manufacturer == "" {
		return fmt.Errorf("manufacturer is required")
	}
	if len(p.Manufacturer) > 200 {
		return fmt.Errorf("manufacturer must be at most 200 characters")
	}
	if p.CategoryID < 1 {
		return fmt.Errorf("category_id must be >= 1")
	}
	if p.Weight < 0 {
		return fmt.Errorf("weight must be >= 0")
	}
	if p.SomeOtherID < 1 {
		return fmt.Errorf("some_other_id must be >= 1")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, errCode, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(models.Error{
		Error:   errCode,
		Message: message,
	})
}