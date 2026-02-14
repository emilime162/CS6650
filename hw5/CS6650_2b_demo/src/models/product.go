package models

// Product represents the Product schema from the OpenAPI spec.
type Product struct {
	ProductID    int    `json:"product_id"`
	SKU          string `json:"sku"`
	Manufacturer string `json:"manufacturer"`
	CategoryID   int    `json:"category_id"`
	Weight       int    `json:"weight"`
	SomeOtherID  int    `json:"some_other_id"`
}

// Error matches the Error schema from the OpenAPI spec.
type Error struct {
	Error   string  `json:"error"`
	Message string  `json:"message"`
	Details *string `json:"details,omitempty"`
}