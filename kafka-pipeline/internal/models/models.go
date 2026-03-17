package models

import "time"

type Transaction struct {
	ID             string            `json:"id"`
	IdempotencyKey string            `json:"idempotency_key"` // Client-generated. Critical for exactly-once on the producer side.
	Amount         float64           `json:"amount"`
	Currency       string            `json:"currency"`
	SenderID       string            `json:"sender_id"`
	ReceiverID     string            `json:"receiver_id"`
	Type           TransactionType   `json:"type"`
	Status         TransactionStatus `json:"status"`
	Metadata       map[string]string `json:"metadata,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
	ProcessedAt    *time.Time        `json:"processed_at,omitempty"`
	SchemaVersion  int               `json:"schema_version"`
}

type TransactionType string

const (
	TypeTransfer TransactionType = "TRANSFER"
	TypePayment  TransactionType = "PAYMENT"
	TypeRefund   TransactionType = "REFUND"
)

type TransactionStatus string

const (
	StatusPending   TransactionStatus = "PENDING"
	StatusApproved  TransactionStatus = "APPROVED"
	StatusRejected  TransactionStatus = "REJECTED"
	StatusFlagged   TransactionStatus = "FLAGGED"
	StatusEnriched  TransactionStatus = "ENRICHED"
	StatusCompleted TransactionStatus = "COMPLETED"
	StatusFailed    TransactionStatus = "FAILED"
)

// FraudResult is produced by the fraud-detector and written to its own topic.
type FraudResult struct {
	TransactionID string    `json:"transaction_id"`
	RiskScore     float64   `json:"risk_score"` // 0.0 = clean, 1.0 = certain fraud
	RiskFactors   []string  `json:"risk_factors"`
	Decision      string    `json:"decision"` // APPROVE / REJECT / REVIEW
	EvluatedAt    time.Time `json:"evaluated_at"`
	ModelVersion  string    `json:"model_version"`
}

type EnrichedTransaction struct {
	Transaction
	SenderRiskTier   string    `json:"sender_risk_tier"`
	ReceiverRiskTier string    `json:"receiver_risk_tier"`
	GeoLocation      string    `json:"geo_location"`
	MerchantCategory string    `json:"merchant_category,omitempty"`
	EnrichedAt       time.Time `json:"enriched_at"`
}

// Notification is the terminal event — what actually reaches the end user.
type Notification struct {
	TransactionID string            `json:"transaction_id"`
	UserID        string            `json:"user_id"`
	Channel       string            `json:"channel"` // email / sms / push
	TemplateID    string            `json:"template_id"`
	Params        map[string]string `json:"params"`
	SentAt        time.Time         `json:"sent_at"`
}

// DeadLetterEnvelope wraps any failed message with enough context to replay it.
type DeadLetterEnvelope struct {
	OriginalTopic     string            `json:"original_topic"`
	OriginalPartition int32             `json:"original_partition"`
	OriginalOffset    int64             `json:"original_offset"`
	OriginalKey       string            `json:"original_key"`
	OriginalValue     []byte            `json:"original_value"`
	OriginalHeaders   map[string]string `json:"original_headers"`
	ErrorMessage      string            `json:"error_message"`
	ErrorType         string            `json:"error_type"` // DESERIALIZATION / PROCESSING / TRANSIENT / POISON
	RetryCount        int               `json:"retry_count"`
	MaxRetries        int               `json:"max_retries"`
	FirstFailedAt     time.Time         `json:"first_failed_at"`
	LastFailedAt      time.Time         `json:"last_failed_at"`
	ServiceName       string            `json:"service_name"`
	ServiceVersion    string            `json:"service_version"`
}
