package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"time"

	"github.com/dmehra2102/prod-golang-projects/kafka-pipeline/internal/config"
	"github.com/dmehra2102/prod-golang-projects/kafka-pipeline/internal/health"
	"github.com/dmehra2102/prod-golang-projects/kafka-pipeline/internal/kafka"
	kafkapkg "github.com/dmehra2102/prod-golang-projects/kafka-pipeline/internal/kafka"
	"github.com/dmehra2102/prod-golang-projects/kafka-pipeline/internal/middleware"
	"github.com/dmehra2102/prod-golang-projects/kafka-pipeline/internal/models"
	"github.com/dmehra2102/prod-golang-projects/kafka-pipeline/internal/runner"
	"go.uber.org/zap"
)

func main() {
	configPath := flag.String("config", "configs/config.yaml", "path to config file")
	flag.Parse()

	runner.Run(*configPath, func(ctx context.Context, cfg *config.Config, producer *kafka.Producer, logger *zap.Logger, healthSrv *health.Server) error {
		logger = logger.Named("notifier")

		handler := middleware.Chain(
			middleware.Recovery(logger),
			middleware.Logging(logger),
			middleware.Timeout(10*time.Second),
			middleware.Dedupilcation(logger, 1*time.Hour),
		)(func(ctx context.Context, key, value []byte, headers map[string]string) error {
			var enriched models.EnrichedTransaction
			if err := json.Unmarshal(value, &enriched); err != nil {
				return fmt.Errorf("deserializing enriched transaction: %w", err)
			}

			notifications := buildNotification(&enriched)

			for _, notif := range notifications {
				if err := sendNotification(ctx, notif, logger); err != nil {
					return fmt.Errorf("sending notification to %s via %s: %w",
						notif.UserID, notif.Channel, err)
				}
			}

			logger.Info("notifications sent",
				zap.String("txn_id", enriched.ID),
				zap.String("status", string(enriched.Status)),
				zap.Int("notification_count", len(notifications)),
			)
			return nil
		})

		consumeCfg := cfg.Kafka
		consumeCfg.Consumer.GroupID = "notifier-v1"

		cg, err := kafkapkg.NewConsumerGroup(
			&consumeCfg,
			[]string{cfg.Kafka.Topics.EnrichedTransactions.Name},
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

func buildNotification(txn *models.EnrichedTransaction) []models.Notification {
	var notifications []models.Notification
	now := time.Now().UTC()

	switch txn.Status {
	case models.StatusApproved:
		notifications = append(notifications, models.Notification{
			TransactionID: txn.ID,
			UserID:        txn.SenderID,
			Channel:       "push",
			TemplateID:    "txn_sent_success",
			Params: map[string]string{
				"amount":   fmt.Sprintf("%.2f", txn.Amount),
				"currency": txn.Currency,
				"receiver": txn.ReceiverID,
			},
			SentAt: now,
		},
			models.Notification{
				TransactionID: txn.ID,
				UserID:        txn.ReceiverID,
				Channel:       "push",
				TemplateID:    "txn_received",
				Params: map[string]string{
					"amount":   fmt.Sprintf("%.2f", txn.Amount),
					"currency": txn.Currency,
					"sender":   txn.SenderID,
				},
				SentAt: now,
			},
		)

	case models.StatusFailed:
		notifications = append(notifications, models.Notification{
			TransactionID: txn.ID,
			UserID:        txn.SenderID,
			Channel:       "email",
			TemplateID:    "txn_rejected",
			Params: map[string]string{
				"amount":   fmt.Sprintf("%.2f", txn.Amount),
				"currency": txn.Currency,
				"reason":   "Transaction flagged by security review",
			},
			SentAt: now,
		})

	case models.StatusFlagged:
		// Notify sender + internal fraud team.
		notifications = append(notifications,
			models.Notification{
				TransactionID: txn.ID,
				UserID:        txn.SenderID,
				Channel:       "sms",
				TemplateID:    "txn_under_review",
				Params: map[string]string{
					"amount":   fmt.Sprintf("%.2f", txn.Amount),
					"currency": txn.Currency,
				},
				SentAt: now,
			},
			models.Notification{
				TransactionID: txn.ID,
				UserID:        "fraud-team",
				Channel:       "email",
				TemplateID:    "fraud_review_needed",
				Params: map[string]string{
					"txn_id":    txn.ID,
					"amount":    fmt.Sprintf("%.2f", txn.Amount),
					"risk_tier": txn.SenderRiskTier,
					"sender":    txn.SenderID,
				},
				SentAt: now,
			},
		)
	}

	return notifications
}

func sendNotification(ctx context.Context, notif models.Notification, logger *zap.Logger) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(5 * time.Millisecond):
	}

	logger.Debug("notification dispatched",
		zap.String("user", notif.UserID),
		zap.String("channel", notif.Channel),
		zap.String("template", notif.TemplateID),
	)
	return nil
}
