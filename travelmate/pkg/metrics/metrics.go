package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Collector struct {
	HTTPRequestsTotal    *prometheus.CounterVec
	HTTPRequestDuration  *prometheus.HistogramVec
	HTTPActiveRequests   prometheus.Gauge
	KafkaMessagesTotal   *prometheus.CounterVec
	KafkaPublishLatency  *prometheus.HistogramVec
	KafkaConsumerLag     *prometheus.GaugeVec
	RedisOperationsTotal *prometheus.CounterVec
	RedisLatency         *prometheus.HistogramVec
	CacheHitsTotal       prometheus.Counter
	CacheMissesTotal     prometheus.Counter
	DBQueryDuration      *prometheus.HistogramVec
	DBActiveConns        prometheus.Gauge
	S3OperationsTotal    *prometheus.CounterVec
	S3OperationLatency   *prometheus.HistogramVec
	ActiveUsersGauge     prometheus.Gauge
}

func New() *Collector {
	return &Collector{
		HTTPRequestsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "travelmate",
			Subsystem: "http",
			Name:      "request_total",
			Help:      "Total number of HTTP requests.",
		}, []string{"method", "path", "status"}),

		HTTPRequestDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "travelmate",
			Subsystem: "http",
			Name:      "request_duration_seconds",
			Help:      "HTTP request latency in seconds",
			Buckets:   []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		}, []string{"method", "path"}),

		HTTPActiveRequests: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: "travelmate",
			Subsystem: "http",
			Name:      "active_requests",
			Help:      "Number of in-flight HTTP requests.",
		}),

		KafkaMessagesTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "travelmate",
			Subsystem: "kafka",
			Name:      "messages_total",
			Help:      "Total Kafka messages produced/consumed.",
		}, []string{"topic", "direction", "status"}),

		KafkaPublishLatency: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "travelmate",
			Subsystem: "kafka",
			Name:      "publish_latency_seconds",
			Help:      "Kafka publish latency in seconds.",
			Buckets:   prometheus.DefBuckets,
		}, []string{"topic"}),

		KafkaConsumerLag: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "travelmate",
			Subsystem: "kafka",
			Name:      "consumer_lag",
			Help:      "Kafka consumer lag by partition.",
		}, []string{"topic", "partition", "group"}),

		RedisOperationsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "travelmate",
			Subsystem: "redis",
			Name:      "operations_total",
			Help:      "Total Redis operations.",
		}, []string{"operation", "status"}),

		RedisLatency: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "travelmate",
			Subsystem: "redis",
			Name:      "operation_latency_seconds",
			Help:      "Redis operation latency.",
			Buckets:   []float64{.001, .005, .01, .025, .05, .1, .25},
		}, []string{"operation"}),

		CacheHitsTotal: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: "travelmate",
			Subsystem: "cache",
			Name:      "hits_total",
			Help:      "Total cache hits.",
		}),

		CacheMissesTotal: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: "travelmate",
			Subsystem: "cache",
			Name:      "misses_total",
			Help:      "Total cache misses.",
		}),

		DBQueryDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "travelmate",
			Subsystem: "db",
			Name:      "query_duration_seconds",
			Help:      "Database query duration.",
			Buckets:   []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 5},
		}, []string{"query_type"}),

		DBActiveConns: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: "travelmate",
			Subsystem: "db",
			Name:      "active_connections",
			Help:      "Number of active database connections.",
		}),

		S3OperationsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "travelmate",
			Subsystem: "s3",
			Name:      "operations_total",
			Help:      "Total S3 operations.",
		}, []string{"operation", "status"}),

		S3OperationLatency: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "travelmate",
			Subsystem: "s3",
			Name:      "operation_latency_seconds",
			Help:      "S3 operation latency.",
			Buckets:   prometheus.DefBuckets,
		}, []string{"operation"}),

		ActiveUsersGauge: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: "travelmate",
			Name:      "active_travelers",
			Help:      "Number of currently tracked active travelers.",
		}),
	}
}

// Handler returns the Prometheus metrics HTTP handler.
func Handler() http.Handler {
	return promhttp.Handler()
}

// HTTPMiddleware instruments all HTTP requests with Prometheus metrics.
func (c *Collector) HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/metrics" || r.URL.Path == "/health/live" {
			next.ServeHTTP(w, r)
			return
		}

		c.HTTPActiveRequests.Inc()
		defer c.HTTPActiveRequests.Dec()

		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rec, r)

		duration := time.Since(start).Seconds()
		path := normalizePath(r.URL.Path)

		c.HTTPRequestsTotal.WithLabelValues(r.Method, path, strconv.Itoa(rec.status)).Inc()
		c.HTTPRequestDuration.WithLabelValues(r.Method, path).Observe(duration)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func normalizePath(path string) string {
	parts := splitPath(path)
	for i, p := range parts {
		if isUUID(p) {
			parts[i] = ":id"
		}
	}
	return joinPath(parts)
}

func splitPath(p string) []string {
	var parts []string
	current := ""
	for _, c := range p {
		if c == '/' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

func joinPath(parts []string) string {
	if len(parts) == 0 {
		return "/"
	}
	result := ""
	for _, p := range parts {
		result += "/" + p
	}
	return result
}

func isUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
			continue
		}
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}
