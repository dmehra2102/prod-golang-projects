package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	MessagesProduced = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "kafka_pipeline",
		Subsystem: "producer",
		Name:      "messages_produced_total",
		Help:      "Total messages produced, partitioned by topic and outcome.",
	}, []string{"topic", "status"})

	ProduceLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "kafka_pipeline",
		Subsystem: "producer",
		Name:      "produce_latency_seconds",
		Help:      "Time from producer call to broker acknowledgement.",
		Buckets:   []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0},
	}, []string{"topic"})

	MessagesConsumed = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "kafka_pipeline",
		Subsystem: "consumer",
		Name:      "messages_consumed_total",
		Help:      "Total messages consumed, partitioned by topic, group, an outcome.",
	}, []string{"topic", "group", "status"})

	ConsumeLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "kafka_pipeline",
		Subsystem: "consumer",
		Name:      "consume_latency_seconds",
		Help:      "Time to process a single consumed message.",
		Buckets:   []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0},
	}, []string{"topic", "group"})

	ConsumerLag = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "kafka_pipeline",
		Subsystem: "consumer",
		Name:      "lag",
		Help:      "Consumer lag (high-water mark - committed offset) per topic-partition.",
	}, []string{"topic", "group", "partition"})

	ConsumerRebalances = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "kafka_pipeline",
		Subsystem: "consumer",
		Name:      "rebalances_total",
		Help:      "Number of consumer group rebalance events.",
	}, []string{"group", "type"})

	DLQMessages = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "kafka_pipeline",
		Subsystem: "dlq",
		Name:      "messages_total",
		Help:      "Messages sent to dead letter queue.",
	}, []string{"source_topic", "error_type"})

	CircuitBreakerState = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "kafka_pipeline",
		Subsystem: "circuit_breaker",
		Name:      "state",
		Help:      "Circuit breaker state: 0=closed, 1=half-open, 2=open.",
	}, []string{"name"})

	BuildInfo = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "kafka_pipeline",
		Name:      "build_info",
		Help:      "Build information as labels.",
	}, []string{"service", "version", "env"})
)
