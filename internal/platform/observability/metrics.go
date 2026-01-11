package observability

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// HTTP metrics
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		},
		[]string{"method", "path", "status"},
	)

	httpRequestSize = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_size_bytes",
			Help:    "HTTP request size in bytes",
			Buckets: prometheus.ExponentialBuckets(100, 10, 8),
		},
		[]string{"method", "path"},
	)

	httpResponseSize = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_response_size_bytes",
			Help:    "HTTP response size in bytes",
			Buckets: prometheus.ExponentialBuckets(100, 10, 8),
		},
		[]string{"method", "path"},
	)

	// Application metrics
	activeConnections = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "http_active_connections",
			Help: "Number of active HTTP connections",
		},
	)

	// Database metrics
	dbQueryTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "db_queries_total",
			Help: "Total number of database queries",
		},
		[]string{"operation", "table"},
	)

	dbQueryDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "db_query_duration_seconds",
			Help:    "Database query duration in seconds",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
		},
		[]string{"operation", "table"},
	)

	// Cache metrics
	cacheHitsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_hits_total",
			Help: "Total number of cache hits",
		},
		[]string{"cache"},
	)

	cacheMissesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_misses_total",
			Help: "Total number of cache misses",
		},
		[]string{"cache"},
	)
)

// PrometheusMiddleware returns a Fiber middleware that collects Prometheus metrics
func PrometheusMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()

		// Track active connections
		activeConnections.Inc()
		defer activeConnections.Dec()

		// Get request size
		reqSize := float64(len(c.Request().Body()))

		// Process request
		err := c.Next()

		// Calculate duration
		duration := time.Since(start).Seconds()

		// Get response info
		status := strconv.Itoa(c.Response().StatusCode())
		method := c.Method()
		path := c.Route().Path // Use route path to avoid cardinality explosion
		if path == "" {
			path = c.Path()
		}
		respSize := float64(len(c.Response().Body()))

		// Record metrics
		httpRequestsTotal.WithLabelValues(method, path, status).Inc()
		httpRequestDuration.WithLabelValues(method, path, status).Observe(duration)
		httpRequestSize.WithLabelValues(method, path).Observe(reqSize)
		httpResponseSize.WithLabelValues(method, path).Observe(respSize)

		return err
	}
}

// MetricsHandler returns a Fiber handler for the Prometheus metrics endpoint
func MetricsHandler() fiber.Handler {
	return adaptor.HTTPHandler(promhttp.Handler())
}

// RecordDBQuery records a database query metric
func RecordDBQuery(operation, table string, duration time.Duration) {
	dbQueryTotal.WithLabelValues(operation, table).Inc()
	dbQueryDuration.WithLabelValues(operation, table).Observe(duration.Seconds())
}

// RecordCacheHit records a cache hit
func RecordCacheHit(cache string) {
	cacheHitsTotal.WithLabelValues(cache).Inc()
}

// RecordCacheMiss records a cache miss
func RecordCacheMiss(cache string) {
	cacheMissesTotal.WithLabelValues(cache).Inc()
}

// Custom business metrics

var (
	usersRegisteredTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "users_registered_total",
			Help: "Total number of registered users",
		},
	)

	loginAttemptsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "login_attempts_total",
			Help: "Total number of login attempts",
		},
		[]string{"status"}, // success, failed
	)
)

// RecordUserRegistration records a new user registration
func RecordUserRegistration() {
	usersRegisteredTotal.Inc()
}

// RecordLoginAttempt records a login attempt
func RecordLoginAttempt(success bool) {
	status := "failed"
	if success {
		status = "success"
	}
	loginAttemptsTotal.WithLabelValues(status).Inc()
}
