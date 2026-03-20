package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"time"

	"github.com/dmehra2102/prod-golang-projects/kafka-pipeline/internal/config"
	"github.com/dmehra2102/prod-golang-projects/kafka-pipeline/internal/health"
	"github.com/dmehra2102/prod-golang-projects/kafka-pipeline/internal/kafka"
	kafkapkg "github.com/dmehra2102/prod-golang-projects/kafka-pipeline/internal/kafka"
	"github.com/dmehra2102/prod-golang-projects/kafka-pipeline/internal/middleware"
	"github.com/dmehra2102/prod-golang-projects/kafka-pipeline/internal/models"
	"github.com/dmehra2102/prod-golang-projects/kafka-pipeline/internal/runner"
	"github.com/dmehra2102/prod-golang-projects/kafka-pipeline/pkg/circuitbreaker"
	"go.uber.org/zap"
)

// Fraud Detector is a CONSUMER-PRODUCER: it reads from txn.raw.v1,
// evaluates fraud risk, and writes results to txn.fraud-results.v1.

// This is the most common Kafka pattern — a stream processor that transforms
// and enriches events.

func main() {
	configPath := flag.String("config", "configs/config.yaml", "path to string file")
	flag.Parse()

	runner.Run(*configPath, func(ctx context.Context, cfg *config.Config, producer *kafka.Producer, logger *zap.Logger, healthSrv *health.Server) error {
		logger = logger.Named("fraud-detector")

		cb := circuitbreaker.New(circuitbreaker.Config{
			Name:             "fraud_scoring_api",
			FailureThreshold: 5,
			SuccessThreshold: 2,
			Timeout:          30 * time.Second,
		})

		outputTopic := cfg.Kafka.Topics.FraudResults.Name

		handler := middleware.Chain(
			middleware.Recovery(logger),
			middleware.Logging(logger),
			middleware.Timeout(10*time.Second),
		)(func(ctx context.Context, key []byte, value []byte, headers map[string]string) error {
			var txn models.Transaction
			if err := json.Unmarshal(value, &txn); err != nil {
				return fmt.Errorf("deserializing transaction: %w", err)
			}

			// Score through circuit breaker
			var result models.FraudResult
			err := cb.Execute(func() error {
				var err error
				result, err = scoreFraud(ctx, txn)
				return err
			})
			if err != nil {
				return fmt.Errorf("fraud scoring failed: %w", err)
			}

			_, _, err = producer.ProduceMessage(ctx, outputTopic, txn.SenderID, result, map[string]string{
				"source_transaction_id": txn.ID,
				"fraud_decision":        result.Decision,
			})
			if err != nil {
				return fmt.Errorf("producing fraud result: %w", err)
			}

			logger.Debug("fraud evaluation complete",
				zap.String("txn_id", txn.ID),
				zap.Float64("risk_score", result.RiskScore),
				zap.String("decision", result.Decision),
			)
			return nil
		})

		// Create Consumer Group
		consumerCfg := cfg.Kafka
		consumerCfg.Consumer.GroupID = "fraud-detector-v1"

		cg, err := kafkapkg.NewConsumerGroup(
			&consumerCfg,
			[]string{cfg.Kafka.Topics.Transactions.Name},
			handler,
			producer, // DLQ Producer
			logger,
		)

		if err != nil {
			return fmt.Errorf("creating consumer group: %w", err)
		}

		// Register health checks.
		healthSrv.RegisterReadinessCheck("kafka_consumer", func(ctx context.Context) error {
			// In a real system, ping the broker or check consumer lag.
			return nil
		})
		healthSrv.SetReady(true)

		return cg.Run(ctx)
	})
}

func scoreFraud(_ context.Context, txn models.Transaction) (models.FraudResult, error) {
	var riskScore float64
	var factors []string

	// Factor 1 : Transaction Amount
	if txn.Amount > 10_000 {
		riskScore += 0.3
		factors = append(factors, "high_amount")
	} else if txn.Amount > 5_000 {
		riskScore += 0.15
		factors = append(factors, "elevated_amount")
	}

	// Factor 2: Refunds are higher risk
	if txn.Type == models.TypeRefund {
		riskScore += 0.2
		factors = append(factors, "refund_type")
	}

	// Factor 3: Velocity check
	// Randomly flag ~5% of transactions as velocity anomalies
	if txn.Amount > 1000 && math.Mod(float64(txn.CreatedAt.UnixNano()), 20) == 0 {
		riskScore += 0.25
		factors = append(factors, "velocity_anomaly")
	}

	// Factor 4: Currency risk
	highRiskCurrencies := map[string]float64{"EUR": 0.05, "USD": 0.03}
	if bonus, ok := highRiskCurrencies[txn.Currency]; ok {
		riskScore += bonus
		factors = append(factors, "high_risk_currency")
	}

	// Clamp to [0, 1]
	if riskScore > 1.0 {
		riskScore = 1.0
	}

	decision := "APPROVE"
	if riskScore >= 0.7 {
		decision = "REJECT"
	} else if riskScore >= 0.4 {
		decision = "REVIEW"
	}

	return models.FraudResult{
		TransactionID: txn.ID,
		RiskScore:     riskScore,
		RiskFactors:   factors,
		Decision:      decision,
		EvluatedAt:    time.Now().UTC(),
		ModelVersion:  "fraud-v2.3.1",
	}, nil
}
