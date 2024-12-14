package server

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (rec *statusRecorder) WriteHeader(code int) {
	rec.statusCode = code
	rec.ResponseWriter.WriteHeader(code)
}

// Define Prometheus metrics
var (
	requestCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Number of HTTP requests processed",
		},
		[]string{"method", "endpoint", "status"},
	)

	requestLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Histogram of latencies for HTTP requests",
			Buckets: prometheus.DefBuckets, // Default buckets: 0.005s, 0.01s, 0.025s, etc.
		},
		[]string{"method", "endpoint", "status"},
	)
)

func init() {
	// Register the metrics with Prometheus
	prometheus.MustRegister(requestCount)
	prometheus.MustRegister(requestLatency)
}

func hitCounterMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(recorder, r)
		requestCount.WithLabelValues(r.Method, r.URL.Path, http.StatusText(recorder.statusCode)).Inc()
	})
}

func latencyMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(recorder, r)
		duration := time.Since(start).Seconds()
		requestLatency.WithLabelValues(r.Method, r.URL.Path, http.StatusText(recorder.statusCode)).Observe(duration)
	})
}

func enableCors(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch origin := r.Header.Get("Origin"); origin {
		case "http://localhost":
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "PUT, DELETE, POST, GET, OPTIONS")
		}
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}
