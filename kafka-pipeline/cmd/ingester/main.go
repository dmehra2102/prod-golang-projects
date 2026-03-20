package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"time"

	"github.com/dmehra2102/prod-golang-projects/kafka-pipeline/internal/config"
	"github.com/dmehra2102/prod-golang-projects/kafka-pipeline/internal/health"
	"github.com/dmehra2102/prod-golang-projects/kafka-pipeline/internal/kafka"
	"github.com/dmehra2102/prod-golang-projects/kafka-pipeline/internal/models"
	"github.com/dmehra2102/prod-golang-projects/kafka-pipeline/internal/runner"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Ingester simulates an API gateway that receives payment transactions and
// publishes them to Kafka.

func main() {
	configPath := flag.String("config", "configs/config.yaml", "path to config file")
	tps := flag.Int("tps", 100, "transactions per second to generate")
	flag.Parse()

	runner.Run(*configPath, func(ctx context.Context, cfg *config.Config, producer *kafka.Producer, logger *zap.Logger, healthSrv *health.Server) error {
		healthSrv.SetReady(true)
		logger = logger.Named("ingester")
		topic := cfg.Kafka.Topics.Transactions.Name

		logger.Info("starting transaction ingestion",
			zap.Int("target_tps", *tps),
			zap.String("topic", topic),
		)

		ticker := time.NewTicker(time.Second / time.Duration(*tps))
		defer ticker.Stop()

		currencies := []string{"INR", "USD", "EUR", "GBP", "RUB"}
		txnTypes := []models.TransactionType{models.TypeTransfer, models.TypePayment, models.TypeRefund}
		userPool := generateUserPool(1000) // creating 1000 dummy users

		var produced int64
		for {
			select {
			case <-ctx.Done():
				logger.Info("ingester shutting down", zap.Int64("total_produced", produced))
				return nil
			case <-ticker.C:
				txn := generateTransaction(userPool, currencies, txnTypes)

				_, _, err := producer.ProduceMessage(ctx, topic, txn.SenderID, txn, map[string]string{
					"idempotency_key": txn.IdempotencyKey,
					"schema_version":  "1",
				})

				if err != nil {
					logger.Error("failed to produce transaction", zap.Error(err))
					continue
				}

				produced++

				if produced%1000 == 0 {
					logger.Info("ingestion progress", zap.Int64("total_produced", produced))
				}
			}
		}
	})
}

func generateTransaction(users []string, currencies []string, types []models.TransactionType) models.Transaction {
	sender := users[rand.Intn(len(users))]
	receiver := users[rand.Intn(len(users))]
	for receiver == sender {
		receiver = users[rand.Intn(len(users))]
	}

	amount := generateRealisticAmount()

	return models.Transaction{
		ID:             uuid.New().String(),
		IdempotencyKey: uuid.New().String(),
		Amount:         amount,
		Currency:       currencies[rand.Intn(len(currencies))],
		SenderID:       sender,
		ReceiverID:     receiver,
		Type:           types[rand.Intn(len(types))],
		Status:         models.StatusPending,
		Metadata: map[string]string{
			"source":     "api_gateway",
			"ip_address": fmt.Sprintf("10.%d.%d.%d", rand.Intn(256), rand.Intn(256), rand.Intn(256)),
			"user_agent": "payment-sdk/2.1.0",
		},
		CreatedAt:     time.Now().UTC(),
		SchemaVersion: 1,
	}
}

func generateRealisticAmount() float64 {
	r := rand.Float64()
	switch {
	case r < 0.70:
		return float64(rand.Intn(9900)+100) / 100.0 // ₹1.00 - ₹99.99
	case r < 0.95:
		return float64(rand.Intn(90000)+10000) / 100.0 // ₹100.00 - ₹999.99
	default:
		return float64(rand.Intn(4900000)+100000) / 100.0 // ₹1000.00 - ₹49999.99
	}
}

func generateUserPool(size int) []string {
	users := make([]string, size)
	for i := 0; i < size; i++ {
		users[i] = fmt.Sprintf("user_%s", uuid.New().String()[:8])
	}
	return users
}
