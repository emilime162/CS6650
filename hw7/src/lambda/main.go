package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
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

// handler is triggered by SNS — each SNS message = one Lambda invocation
func handler(ctx context.Context, snsEvent events.SNSEvent) error {
	for _, record := range snsEvent.Records {
		msg := record.SNS.Message
		log.Printf("[lambda] received SNS message: %s", msg)

		var order Order
		if err := json.Unmarshal([]byte(msg), &order); err != nil {
			log.Printf("[lambda] failed to parse order: %v", err)
			continue
		}

		log.Printf("[lambda] processing order %s for customer %d",
			order.OrderID, order.CustomerID)

		// Same 3s payment processing delay
		time.Sleep(3 * time.Second)

		log.Printf("[lambda] completed order %s", order.OrderID)
		fmt.Printf("[lambda] order %s status: completed\n", order.OrderID)
	}
	return nil
}

func main() {
	lambda.Start(handler)
}


