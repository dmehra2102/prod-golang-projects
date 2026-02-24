package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Collector struct {
	RequestsTotal   *prometheus.CounterVec
	RequestDuration *prometheus.HistogramVec
	InFlightGauge   prometheus.Gauge

	PatientsCreatedTotal prometheus.Counter
	AppointmentsTotal    *prometheus.CounterVec
	PrescriptionsIssued  prometheus.Counter

	DBQueryDuration *prometheus.HistogramVec
	DBConnections   prometheus.Gauge

	AuditEntriesTotal  prometheus.Counter
	AuditBufferDropped prometheus.Counter
}

func NewCollector(serviceName string) *Collector {
	return &Collector{
		RequestsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: serviceName,
			Subsystem: "http",
			Name:      "requests_total",
			Help:      "Total number of HTTP requests by method, path, and status code.",
		}, []string{"method", "path", "status"}),

		RequestDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: serviceName,
			Subsystem: "http",
			Name:      "request_duration_seconds",
			Help:      "HTTP request latency distribution.",
			Buckets:   []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0},
		}, []string{"method", "path", "status"}),

		InFlightGauge: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: serviceName,
			Subsystem: "http",
			Name:      "in_flight_requests",
			Help:      "Current number of in-flight HTTP requests.",
		}),

		PatientsCreatedTotal: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: serviceName,
			Subsystem: "clinical",
			Name:      "patients_created_total",
			Help:      "Total number of patient records created.",
		}),

		AppointmentsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: serviceName,
			Subsystem: "clinical",
			Name:      "appointments_total",
			Help:      "Total appointments by final status.",
		}, []string{"status"}),

		PrescriptionsIssued: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: serviceName,
			Subsystem: "clinical",
			Name:      "prescriptions_issued_total",
			Help:      "Total prescriptions issued.",
		}),

		DBQueryDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: serviceName,
			Subsystem: "db",
			Name:      "query_duration_seconds",
			Help:      "Database query latency distribution.",
			Buckets:   []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0},
		}, []string{"operation", "table"}),

		DBConnections: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: serviceName,
			Subsystem: "db",
			Name:      "open_connections",
			Help:      "Current number of open database connections.",
		}),

		AuditEntriesTotal: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: serviceName,
			Subsystem: "audit",
			Name:      "entries_total",
			Help:      "Total audit log entries written.",
		}),

		AuditBufferDropped: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: serviceName,
			Subsystem: "audit",
			Name:      "buffer_dropped_total",
			Help:      "Audit entries dropped due to full buffer. Alert if non-zero.",
		}),
	}
}

func MetricsHandler() http.Handler {
	return promhttp.Handler()
}
