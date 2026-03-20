package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"sync"
	"time"

	"github.com/dmehra2102/prod-golang-projects/kafka-pipeline/internal/config"
	"github.com/dmehra2102/prod-golang-projects/kafka-pipeline/internal/health"
	kafkapkg "github.com/dmehra2102/prod-golang-projects/kafka-pipeline/internal/kafka"
	"github.com/dmehra2102/prod-golang-projects/kafka-pipeline/internal/middleware"
	"github.com/dmehra2102/prod-golang-projects/kafka-pipeline/internal/models"
	"github.com/dmehra2102/prod-golang-projects/kafka-pipeline/internal/runner"
	"go.uber.org/zap"
)

// -------------------------------------------------------------------------------
// Enricher demonstrates the STREAM-TABLE JOIN pattern — the most powerful and
// most misunderstood pattern in Kafka stream processing.
//
// THE PROBLEM:
// We have two streams: raw transactions and fraud results. We need to combine
// them into a single enriched event. But they arrive on different topics at
// different times — the fraud result might arrive 50ms to 5s after the raw txn.
//
// THE SOLUTION: LOCAL STATE STORE
// We maintain an in-memory cache (simulating a RocksDB state store or Redis).
// When a raw transaction arrives, we store it. When a fraud result arrives,
// we look up the transaction, merge them, and emit the enriched event.
//
// REAL-WORLD CONSIDERATIONS:
//   - In Kafka Streams (Java), this is a KTable join — state is backed by a
//     changelog topic and survives restarts.
//   - In Go, you either use an external store (Redis, DynamoDB) or accept
//     that in-memory state is lost on restart (acceptable if you can replay).
//   - The timeout (TTL) prevents memory leaks from unmatched events.
//
// PARTITION CO-LOCATION:
// This join ONLY works because both topics use the same partition key (sender_id).
// If a transaction is on partition 3 of the raw topic, its fraud result will
// also be on partition 3 of the fraud results topic. This means a single
// consumer instance sees both halves of the join.
// -------------------------------------------------------------------------------
func main() {
	configPath := flag.String("config", "configs/config.yaml", "path to config file")
	flag.Parse()

	runner.Run(*configPath, func(ctx context.Context, cfg *config.Config, producer *kafkapkg.Producer, logger *zap.Logger, healthSrv *health.Server) error {
		logger = logger.Named("enricher")

		store := newJoinStore(5 * time.Minute)

		outputTopic := cfg.Kafka.Topics.EnrichedTransactions.Name

		handler := middleware.Chain(
			middleware.Recovery(logger),
			middleware.Logging(logger),
			middleware.Timeout(15*time.Second),
		)(func(ctx context.Context, key []byte, value []byte, headers map[string]string) error {
			// Determine which topic this message came from via header.
			sourceTopic := headers["source_topic"]
			if sourceTopic == "" {
				// Fallback: try to detect by deserializing
				return handleAutoDetect(ctx, value, store, producer, outputTopic, logger)
			}

			switch sourceTopic {
			case cfg.Kafka.Topics.Transactions.Name:
				return handleRawTransaction(value, store, logger)
			case cfg.Kafka.Topics.FraudResults.Name:
				return handleFraudResult(ctx, value, store, producer, outputTopic, logger)
			default:
				return fmt.Errorf("unexpected source topic: %s", sourceTopic)
			}
		})

		// consumer group subscribes to both topics.
		consumerCfg := cfg.Kafka
		consumerCfg.Consumer.GroupID = "enricher-v1"

		cg, err := kafkapkg.NewConsumerGroup(
			&consumerCfg,
			[]string{
				cfg.Kafka.Topics.Transactions.Name,
				cfg.Kafka.Topics.FraudResults.Name,
			},
			handler,
			producer,
			logger,
		)
		if err != nil {
			return fmt.Errorf("creating consumer group: %w", err)
		}

		healthSrv.SetReady(true)
		return cg.Run(ctx)
	})
}

// handleAutoDetect tries to determine the message type by attempting deserialization.
func handleAutoDetect(ctx context.Context, value []byte, store *joinStore, producer *kafkapkg.Producer, outputTopic string, logger *zap.Logger) error {
	// Try fraud result first (smaller, more specific structure).
	var fr models.FraudResult
	if err := json.Unmarshal(value, &fr); err == nil && fr.TransactionID != "" && fr.Decision != "" {
		return handleFraudResult(ctx, value, store, producer, outputTopic, logger)
	}

	// Must be a raw transaction.
	return handleRawTransaction(value, store, logger)
}

func handleRawTransaction(value []byte, store *joinStore, logger *zap.Logger) error {
	var txn models.Transaction
	if err := json.Unmarshal(value, &txn); err != nil {
		return fmt.Errorf("deserializing transaction: %w", err)
	}
	store.StoreTransaction(txn.ID, &txn)
	logger.Debug("stored transaction for join", zap.String("txn_id", txn.ID))
	return nil
}

func handleFraudResult(ctx context.Context, value []byte, store *joinStore, producer *kafkapkg.Producer, outputTopic string, logger *zap.Logger) error {
	var result models.FraudResult
	if err := json.Unmarshal(value, &result); err != nil {
		return fmt.Errorf("deserializing fraud result: %w", err)
	}

	txn, ok := store.GetTransaction(result.TransactionID)
	if !ok {
		logger.Warn("transaction not found for fraud result — join miss",
			zap.String("txn_id", result.TransactionID),
		)

		store.StoreFraudResult(result.TransactionID, &result)
		return nil
	}

	enriched := enrich(txn, &result)

	_, _, err := producer.ProduceMessage(ctx, outputTopic, txn.SenderID, enriched, map[string]string{
		"fraud_decision": result.Decision,
		"risk_score":     fmt.Sprintf("%.2f", result.RiskScore),
	})
	if err != nil {
		return fmt.Errorf("producing enriched transaction: %w", err)
	}

	// Clean up the state store.
	store.Delete(result.TransactionID)

	logger.Debug("enriched transaction produced",
		zap.String("txn_id", txn.ID),
		zap.String("decision", result.Decision),
	)
	return nil
}

func enrich(txn *models.Transaction, fraud *models.FraudResult) models.EnrichedTransaction {
	riskTier := "LOW"
	if fraud.RiskScore > 0.4 {
		riskTier = "MEDIUM"
	}
	if fraud.RiskScore > 0.7 {
		riskTier = "HIGH"
	}

	status := models.StatusApproved
	if fraud.Decision == "REJECT" {
		status = models.StatusRejected
	} else if fraud.Decision == "REVIEW" {
		status = models.StatusFlagged
	}

	txn.Status = status
	now := time.Now().UTC()
	txn.ProcessedAt = &now

	return models.EnrichedTransaction{
		Transaction:      *txn,
		SenderRiskTier:   riskTier,
		ReceiverRiskTier: "LOW",     // Simulated lookup
		GeoLocation:      "IND-DEL", // Simulated geo lookup
		MerchantCategory: "5411",    // Simulated MCC lookup
		EnrichedAt:       now,
	}
}

// -------------------------------------------------------------------------------
// joinStore is a simple in-memory state store with TTL.
// In production, replace with Redis or a compacted Kafka topic.
// -------------------------------------------------------------------------------

type joinStore struct {
	mu           sync.RWMutex
	transactions map[string]*txnEntry
	fraudResults map[string]*frEntry
	ttl          time.Duration
}

type txnEntry struct {
	txn      *models.Transaction
	storedAt time.Time
}

type frEntry struct {
	result   *models.FraudResult
	storedAt time.Time
}

func newJoinStore(ttl time.Duration) *joinStore {
	s := &joinStore{
		transactions: make(map[string]*txnEntry),
		fraudResults: make(map[string]*frEntry),
		ttl:          ttl,
	}
	go s.evictLoop()
	return s
}

func (s *joinStore) StoreTransaction(id string, txn *models.Transaction) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.transactions[id] = &txnEntry{txn: txn, storedAt: time.Now()}
}

func (s *joinStore) StoreFraudResult(id string, result *models.FraudResult) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fraudResults[id] = &frEntry{result: result, storedAt: time.Now()}
}

func (s *joinStore) GetTransaction(id string) (*models.Transaction, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.transactions[id]
	if !ok || time.Since(entry.storedAt) > s.ttl {
		return nil, false
	}
	return entry.txn, true
}

func (s *joinStore) GetFraudResult(id string) (*models.FraudResult, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.fraudResults[id]
	if !ok || time.Since(entry.storedAt) > s.ttl {
		return nil, false
	}
	return entry.result, true
}

func (s *joinStore) Delete(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.transactions, id)
	delete(s.fraudResults, id)
}

func (s *joinStore) evictLoop() {
	ticker := time.NewTicker(30 * time.Second)
	for range ticker.C {
		s.mu.Lock()
		now := time.Now()
		for id, entry := range s.transactions {
			if now.Sub(entry.storedAt) > s.ttl {
				delete(s.transactions, id)
			}
		}

		for id, entry := range s.fraudResults {
			if now.Sub(entry.storedAt) > s.ttl {
				delete(s.fraudResults, id)
			}
		}
		s.mu.Unlock()
	}
}
