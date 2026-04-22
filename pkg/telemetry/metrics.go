package telemetry

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	HTTPRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total number of HTTP requests",
	}, []string{"method", "path", "status"})

	HTTPRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request duration in seconds",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path"})

	RabbitMQPublishTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "rabbitmq_publish_total",
		Help: "Total number of RabbitMQ publish attempts",
	}, []string{"routing_key", "status"})

	RabbitMQConsumeTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "rabbitmq_consume_total",
		Help: "Total number of RabbitMQ consumed messages",
	}, []string{"queue", "status"})
)
