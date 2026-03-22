package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	_ "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
)

// ── Domain Types ──────────────────────────────────────────────────────────────

type CartItem struct {
	ProductID int `json:"product_id"`
	Quantity  int `json:"quantity"`
}

type Cart struct {
	CartID     string     `json:"cart_id"`
	CustomerID int        `json:"customer_id"`
	Items      []CartItem `json:"items"`
	CreatedAt  time.Time  `json:"created_at"`
}

// ── Global Clients ────────────────────────────────────────────────────────────

var (
	mysqlDB       *sql.DB
	dynamoClient  *dynamodb.Client
	dynamoTable   string
)

// ── MySQL Setup ───────────────────────────────────────────────────────────────

func initMySQL() {
	endpoint := os.Getenv("MYSQL_ENDPOINT")
	user     := os.Getenv("MYSQL_USER")
	password := os.Getenv("MYSQL_PASSWORD")
	dbname   := os.Getenv("MYSQL_DB")

	// Remove port from endpoint if present — RDS includes it
	host := strings.Split(endpoint, ":")[0]
	dsn  := fmt.Sprintf("%s:%s@tcp(%s:3306)/%s?parseTime=true",
		user, password, host, dbname)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("failed to open MySQL: %v", err)
	}

	// Connection pool settings
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Wait for RDS to be ready (retry up to 30s)
	for i := 0; i < 10; i++ {
		if err = db.Ping(); err == nil {
			break
		}
		log.Printf("waiting for MySQL... (%d/10)", i+1)
		time.Sleep(3 * time.Second)
	}
	if err != nil {
		log.Fatalf("MySQL not reachable: %v", err)
	}

	mysqlDB = db
	log.Println("MySQL connected!")
	initSchema()
}

func initSchema() {
	// carts table
	_, err := mysqlDB.Exec(`
		CREATE TABLE IF NOT EXISTS carts (
			cart_id     VARCHAR(36)  PRIMARY KEY,
			customer_id INT          NOT NULL,
			created_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_customer (customer_id)
		)
	`)
	if err != nil {
		log.Fatalf("failed to create carts table: %v", err)
	}

	// cart_items table
	_, err = mysqlDB.Exec(`
		CREATE TABLE IF NOT EXISTS cart_items (
			id         INT AUTO_INCREMENT PRIMARY KEY,
			cart_id    VARCHAR(36) NOT NULL,
			product_id INT         NOT NULL,
			quantity   INT         NOT NULL DEFAULT 1,
			FOREIGN KEY (cart_id) REFERENCES carts(cart_id) ON DELETE CASCADE,
			UNIQUE KEY uq_cart_product (cart_id, product_id),
			INDEX idx_cart (cart_id)
		)
	`)
	if err != nil {
		log.Fatalf("failed to create cart_items table: %v", err)
	}
	log.Println("MySQL schema ready!")
}

// ── DynamoDB Setup ────────────────────────────────────────────────────────────

func initDynamo() {
	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(os.Getenv("AWS_REGION")),
	)
	if err != nil {
		log.Fatalf("failed to load AWS config: %v", err)
	}
	dynamoClient = dynamodb.NewFromConfig(cfg)
	dynamoTable  = os.Getenv("DYNAMODB_TABLE")
	log.Println("DynamoDB client ready!")
}

// ── Health ────────────────────────────────────────────────────────────────────

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

// ── MySQL Handlers ────────────────────────────────────────────────────────────

// POST /mysql/shopping-carts
func mysqlCreateCart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		CustomerID int `json:"customer_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.CustomerID == 0 {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	cartID := uuid.New().String()
	_, err := mysqlDB.Exec(
		"INSERT INTO carts (cart_id, customer_id) VALUES (?, ?)",
		cartID, req.CustomerID,
	)
	if err != nil {
		log.Printf("MySQL createCart error: %v", err)
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"shopping_cart_id": cartID})
}

// GET /mysql/shopping-carts/{id}
func mysqlGetCart(w http.ResponseWriter, r *http.Request) {
	cartID := strings.TrimPrefix(r.URL.Path, "/mysql/shopping-carts/")
	if cartID == "" {
		http.Error(w, "missing cart id", http.StatusBadRequest)
		return
	}

	// Get cart
	var cart Cart
	err := mysqlDB.QueryRow(
		"SELECT cart_id, customer_id, created_at FROM carts WHERE cart_id = ?",
		cartID,
	).Scan(&cart.CartID, &cart.CustomerID, &cart.CreatedAt)
	if err == sql.ErrNoRows {
		http.Error(w, "cart not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	// Get items with JOIN
	rows, err := mysqlDB.Query(
		"SELECT product_id, quantity FROM cart_items WHERE cart_id = ?",
		cartID,
	)
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	cart.Items = []CartItem{}
	for rows.Next() {
		var item CartItem
		rows.Scan(&item.ProductID, &item.Quantity)
		cart.Items = append(cart.Items, item)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cart)
}

// POST /mysql/shopping-carts/{id}/items
func mysqlAddItems(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract cart ID from path
	path   := strings.TrimPrefix(r.URL.Path, "/mysql/shopping-carts/")
	cartID := strings.TrimSuffix(path, "/items")

	var req CartItem
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Verify cart exists
	var exists int
	mysqlDB.QueryRow("SELECT COUNT(*) FROM carts WHERE cart_id = ?", cartID).Scan(&exists)
	if exists == 0 {
		http.Error(w, "cart not found", http.StatusNotFound)
		return
	}

	// Insert or update quantity if product already in cart
	_, err := mysqlDB.Exec(`
		INSERT INTO cart_items (cart_id, product_id, quantity)
		VALUES (?, ?, ?)
		ON DUPLICATE KEY UPDATE quantity = quantity + VALUES(quantity)
	`, cartID, req.ProductID, req.Quantity)
	if err != nil {
		log.Printf("MySQL addItems error: %v", err)
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ── DynamoDB Handlers ─────────────────────────────────────────────────────────

// POST /dynamo/shopping-carts
func dynamoCreateCart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		CustomerID int `json:"customer_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.CustomerID == 0 {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	cartID := uuid.New().String()
	_, err := dynamoClient.PutItem(context.Background(), &dynamodb.PutItemInput{
		TableName: aws.String(dynamoTable),
		Item: map[string]types.AttributeValue{
			"cart_id":     &types.AttributeValueMemberS{Value: cartID},
			"customer_id": &types.AttributeValueMemberN{Value: strconv.Itoa(req.CustomerID)},
			"items":       &types.AttributeValueMemberL{Value: []types.AttributeValue{}},
			"created_at":  &types.AttributeValueMemberS{Value: time.Now().UTC().Format(time.RFC3339)},
		},
	})
	if err != nil {
		log.Printf("DynamoDB createCart error: %v", err)
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"shopping_cart_id": cartID})
}

// GET /dynamo/shopping-carts/{id}
func dynamoGetCart(w http.ResponseWriter, r *http.Request) {
	cartID := strings.TrimPrefix(r.URL.Path, "/dynamo/shopping-carts/")
	if cartID == "" {
		http.Error(w, "missing cart id", http.StatusBadRequest)
		return
	}

	result, err := dynamoClient.GetItem(context.Background(), &dynamodb.GetItemInput{
		TableName: aws.String(dynamoTable),
		Key: map[string]types.AttributeValue{
			"cart_id": &types.AttributeValueMemberS{Value: cartID},
		},
	})
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	if result.Item == nil {
		http.Error(w, "cart not found", http.StatusNotFound)
		return
	}

	var cart Cart
	attributevalue.UnmarshalMap(result.Item, &cart)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cart)
}

// POST /dynamo/shopping-carts/{id}/items
func dynamoAddItems(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path   := strings.TrimPrefix(r.URL.Path, "/dynamo/shopping-carts/")
	cartID := strings.TrimSuffix(path, "/items")

	var req CartItem
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Append item to items list using UpdateItem expression
	_, err := dynamoClient.UpdateItem(context.Background(), &dynamodb.UpdateItemInput{
		TableName: aws.String(dynamoTable),
		Key: map[string]types.AttributeValue{
			"cart_id": &types.AttributeValueMemberS{Value: cartID},
		},
		UpdateExpression: aws.String("SET #items = list_append(#items, :new_item)"),
		ExpressionAttributeNames: map[string]string{
			"#items": "items",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":new_item": &types.AttributeValueMemberL{
				Value: []types.AttributeValue{
					&types.AttributeValueMemberM{
						Value: map[string]types.AttributeValue{
							"product_id": &types.AttributeValueMemberN{
								Value: strconv.Itoa(req.ProductID),
							},
							"quantity": &types.AttributeValueMemberN{
								Value: strconv.Itoa(req.Quantity),
							},
						},
					},
				},
			},
		},
		ConditionExpression: aws.String("attribute_exists(cart_id)"),
	})
	if err != nil {
		log.Printf("DynamoDB addItems error: %v", err)
		http.Error(w, "cart not found or database error", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ── Router ────────────────────────────────────────────────────────────────────

func router(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	switch {
	// Health
	case path == "/health":
		healthHandler(w, r)

	// MySQL routes
	case path == "/mysql/shopping-carts" && r.Method == http.MethodPost:
		mysqlCreateCart(w, r)
	case strings.HasPrefix(path, "/mysql/shopping-carts/") && strings.HasSuffix(path, "/items"):
		mysqlAddItems(w, r)
	case strings.HasPrefix(path, "/mysql/shopping-carts/"):
		mysqlGetCart(w, r)

	// DynamoDB routes
	case path == "/dynamo/shopping-carts" && r.Method == http.MethodPost:
		dynamoCreateCart(w, r)
	case strings.HasPrefix(path, "/dynamo/shopping-carts/") && strings.HasSuffix(path, "/items"):
		dynamoAddItems(w, r)
	case strings.HasPrefix(path, "/dynamo/shopping-carts/"):
		dynamoGetCart(w, r)

	default:
		http.Error(w, "not found", http.StatusNotFound)
	}
}

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	initMySQL()
	initDynamo()

	fmt.Println("Shopping Cart Service starting on :8080")
	http.HandleFunc("/", router)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
