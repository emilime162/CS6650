package store

import (
	"fmt"
	"sync"

	"product-api/models"
)

// ProductStore provides thread-safe in-memory storage for products.
// Uses sync.RWMutex so concurrent GETs don't block each other.
type ProductStore struct {
	mu       sync.RWMutex
	products map[int]*models.Product
}

func NewProductStore() *ProductStore {
	return &ProductStore{
		products: make(map[int]*models.Product),
	}
}

func (s *ProductStore) GetProduct(id int) (*models.Product, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	product, exists := s.products[id]
	if !exists {
		return nil, fmt.Errorf("product with ID %d not found", id)
	}
	return product, nil
}

func (s *ProductStore) UpsertProduct(id int, product *models.Product) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	product.ProductID = id
	s.products[id] = product
	return nil
}