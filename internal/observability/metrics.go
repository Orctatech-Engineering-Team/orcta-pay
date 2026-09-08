package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	HTTPRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total number of HTTP requests.",
	}, []string{"method", "route", "status"})

	HTTPRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "Duration of HTTP requests in seconds.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "route", "status"})

	GatewayRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "gateway_requests_total",
		Help: "Total number of gateway requests.",
	}, []string{"gateway", "result"})

	GatewayRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "gateway_request_duration_seconds",
		Help:    "Duration of gateway requests in seconds.",
		Buckets: prometheus.DefBuckets,
	}, []string{"gateway", "result"})
)

func init() {
	for _, gw := range []string{"hubtel", "paystack", "moolre"} {
		for _, result := range []string{"success", "failure"} {
			GatewayRequestsTotal.WithLabelValues(gw, result).Add(0)
		}
	}
}

// ObserveGatewayResult records a gateway outcome for Prometheus.
func ObserveGatewayResult(gateway string, success bool) {
	result := "success"
	if !success {
		result = "failure"
	}
	GatewayRequestsTotal.WithLabelValues(gateway, result).Inc()
}

// ObserveGatewayDuration records gateway latency.
func ObserveGatewayDuration(gateway string, success bool, seconds float64) {
	result := "success"
	if !success {
		result = "failure"
	}
	GatewayRequestDuration.WithLabelValues(gateway, result).Observe(seconds)
}
