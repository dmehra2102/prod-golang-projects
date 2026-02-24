package main

import (
	"context"
	"encoding/json"
	"log"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/dmehra2102/prod-golang-projects/lambda-dummy-order-processor/internal/domain"
	"github.com/dmehra2102/prod-golang-projects/lambda-dummy-order-processor/internal/service"
)

func handler(ctx context.Context, event events.SQSEvent) error {
	for _, record := range event.Records {
		var order domain.Order

		if err := json.Unmarshal([]byte(record.Body), &order); err != nil {
			log.Printf("invalid message: %v", err)
			continue
		}

		if err := service.ProcessOrder(ctx, order); err != nil {
			log.Printf("failed order %s: %v", order.OrderID, err)
			return err
		}
	}

	return nil
}

func main() {
	lambda.Start(handler)
}