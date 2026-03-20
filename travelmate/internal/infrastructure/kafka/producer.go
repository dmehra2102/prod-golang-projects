package kafka

import (
	"sync"
	"time"

	"github.com/IBM/sarama"
	"github.com/dmehra2102/prod-golang-projects/travelmate/internal/config"
	"github.com/dmehra2102/prod-golang-projects/travelmate/pkg/logger"
)

type Producer struct {
	syncProduucer sarama.SyncProducer
	asyncProducer sarama.AsyncProducer
	log *logger.Logger
	cfg config.KafkaConfig
	metrics *ProducerMetrics
	wg sync.WaitGroup
	closed chan struct{}
}

type ProducerMetrics struct {
	mu             sync.Mutex
	MessagesSent   int64
	MessagesErr    int64
	BytesSent      int64
	LastError      error
	LastErrorAt    time.Time
	AvgLatencyMs   float64
	latencySum     float64
	latencyCount   int64
}