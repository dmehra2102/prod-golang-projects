package service

import (
	"context"
	"log"

	"github.com/dmehra2102/prod-golang-projects/lambda-dummy-order-processor/internal/domain"
)

func ProcessOrder(ctx context.Context, order domain.Order) error {
	log.Printf("charging payment for order %s", order.OrderID)
	log.Printf("updating inventory for order %s", order.OrderID)
	return nil
}