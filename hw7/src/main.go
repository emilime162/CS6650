package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/google/uuid"
)

type Item struct {
	ProductID string  `json:"product_id"`
	Quantity  int     `json:"quantity"`
	Price     float64 `json:"price"`
}

type Order struct {
	OrderID    string    `json:"order_id"`
	CustomerID int       `json:"customer_id"`
	Status     string    `json:"status"`
	Items      []Item    `json:"items"`
	CreatedAt  time.Time `json:"created_at"`
}

var (
	snsClient       *sns.Client
	sqsClient       *sqs.Client
	snsTopicARN     string
	sqsQueueURL     string
	workerSemaphore chan struct{}
)

func initAWS() {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("failed to load AWS config: %v", err)
	}
	snsClient = sns.NewFromConfig(cfg)
	sqsClient = sqs.NewFromConfig(cfg)
	snsTopicARN = os.Getenv("SNS_TOPIC_ARN")
	sqsQueueURL = os.Getenv("SQS_QUEUE_URL")
}

// Sync handler — 1 payment at a time, customers wait 3s+
var paymentSemaphore = make(chan struct{}, 1)

func verifyPayment(orderID string) {
	paymentSemaphore <- struct{}{}
	defer func() { <-paymentSemaphore }()
	time.Sleep(3 * time.Second)
	log.Printf("[payment] verified order %s", orderID)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func syncHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var order Order
	if err := json.NewDecoder(r.Body).Decode(&order); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	order.OrderID = uuid.New().String()
	order.Status = "processing"
	order.CreatedAt = time.Now()

	verifyPayment(order.OrderID)

	order.Status = "completed"
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(order)
}

func asyncHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var order Order
	if err := json.NewDecoder(r.Body).Decode(&order); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	order.OrderID = uuid.New().String()
	order.Status = "pending"
	order.CreatedAt = time.Now()

	payload, err := json.Marshal(order)
	if err != nil {
		http.Error(w, "failed to marshal order", http.StatusInternalServerError)
		return
	}
	msg := string(payload)
	_, err = snsClient.Publish(context.Background(), &sns.PublishInput{
		TopicArn: &snsTopicARN,
		Message:  &msg,
	})
	if err != nil {
		log.Printf("[async] failed to publish: %v", err)
		http.Error(w, "failed to queue order", http.StatusInternalServerError)
		return
	}
	log.Printf("[async] queued order %s", order.OrderID)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{
		"order_id": order.OrderID,
		"status":   "pending",
		"message":  "Order accepted, processing in background",
	})
}

// pollSQS acquires semaphore BEFORE pulling from SQS
// This keeps messages visible in queue until a worker is actually free
// → ApproximateNumberOfMessagesVisible will show real backlog in CloudWatch
func pollSQS(workerID int) {
	for {
		// Block here until a worker slot is free
		workerSemaphore <- struct{}{}

		result, err := sqsClient.ReceiveMessage(context.Background(), &sqs.ReceiveMessageInput{
			QueueUrl:            &sqsQueueURL,
			MaxNumberOfMessages: 1, // only grab 1 — slot already acquired
			WaitTimeSeconds:     20,
		})
		if err != nil {
			log.Printf("[worker %d] error receiving: %v", workerID, err)
			<-workerSemaphore // release on error
			time.Sleep(1 * time.Second)
			continue
		}

		if len(result.Messages) == 0 {
			<-workerSemaphore // release if nothing to do
			continue
		}

		msg := result.Messages[0]
		go func() {
			defer func() { <-workerSemaphore }() // release when done
			processMessage(workerID, msg.Body, msg.ReceiptHandle)
		}()
	}
}

// processMessage handles a single order — no semaphore here, handled by pollSQS
func processMessage(workerID int, body *string, receiptHandle *string) {
	var envelope map[string]interface{}
	if err := json.Unmarshal([]byte(*body), &envelope); err != nil {
		log.Printf("[worker %d] bad envelope: %v", workerID, err)
		return
	}
	innerMsg, ok := envelope["Message"].(string)
	if !ok {
		log.Printf("[worker %d] missing Message field", workerID)
		return
	}
	var order Order
	if err := json.Unmarshal([]byte(innerMsg), &order); err != nil {
		log.Printf("[worker %d] bad order: %v", workerID, err)
		return
	}
	log.Printf("[worker %d] processing order %s", workerID, order.OrderID)
	time.Sleep(3 * time.Second)
	log.Printf("[worker %d] completed order %s", workerID, order.OrderID)

	sqsClient.DeleteMessage(context.Background(), &sqs.DeleteMessageInput{
		QueueUrl:      &sqsQueueURL,
		ReceiptHandle: receiptHandle,
	})
}

func startWorkers(n int) {
	log.Printf("[workers] starting %d goroutines", n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			pollSQS(id)
		}(i)
	}
	wg.Wait()
}

func main() {
	initAWS()

	numWorkers := 1
	if n := os.Getenv("NUM_WORKERS"); n != "" {
		if parsed, err := strconv.Atoi(n); err == nil {
			numWorkers = parsed
		}
	}

	// semaphore size = num workers = true concurrency limit
	workerSemaphore = make(chan struct{}, numWorkers)

	mode := os.Getenv("SERVICE_MODE")
	switch mode {
	case "processor":
		fmt.Printf("Starting processor with %d workers\n", numWorkers)
		startWorkers(numWorkers)
	default:
		fmt.Println("Starting receiver on :8080")
		http.HandleFunc("/health", healthHandler)
		http.HandleFunc("/orders/sync", syncHandler)
		http.HandleFunc("/orders/async", asyncHandler)
		log.Fatal(http.ListenAndServe(":8080", nil))
	}
}