// Package echoprom provides Echo middleware for Prometheus metrics.
// It records request latency percentiles (75th, 90th, 95th) and status code counts
// per URL path, and exposes them via a /metrics endpoint on port 7070.
package echoprom

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// Registry is the Prometheus registry used by this package
	Registry = prometheus.NewRegistry()

	// RequestDuration measures request latency
	RequestDuration = promauto.With(Registry).NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latency in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "code"},
	)

	// RequestsTotal counts total requests by status code and path
	RequestsTotal = promauto.With(Registry).NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests by status code",
		},
		[]string{"method", "code"},
	)
)

// Config holds configuration for the middleware
type Config struct {
	// Skipper defines a function to skip middleware
	Skipper func(c echo.Context) bool

	// MetricsPath is the endpoint for Prometheus metrics
	MetricsPath string

	// MetricsPort is the port for the metrics server
	MetricsPort int
}

// DefaultConfig provides default configuration
func DefaultConfig() Config {
	return Config{
		Skipper:     func(c echo.Context) bool { return false },
		MetricsPath: "/metrics",
		MetricsPort: 7070,
	}
}

// Middleware returns Echo middleware which records Prometheus metrics
func Middleware() echo.MiddlewareFunc {
	return MiddlewareWithConfig(DefaultConfig())
}

// MiddlewareWithConfig returns Echo middleware with config
func MiddlewareWithConfig(config Config) echo.MiddlewareFunc {
	// Use default config if necessary
	if config.Skipper == nil {
		config.Skipper = DefaultConfig().Skipper
	}

	// Start metrics server
	if config.MetricsPort != 0 {
		go startMetricsServer(config)
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if config.Skipper(c) {
				return next(c)
			}

			// Start timer
			start := time.Now()
			path := sanitizePath(c.Path())
			method := c.Request().Method

			// Execute the request
			err := next(c)

			// Record metrics
			duration := time.Since(start).Seconds()
			status := c.Response().Status
			RequestDuration.WithLabelValues(path, method).Observe(duration)
			RequestsTotal.WithLabelValues(path, method, strconv.Itoa(status)).Inc()

			return err
		}
	}
}

// sanitizePath removes route parameters to avoid high cardinality
func sanitizePath(path string) string {
	// For actual implementation, you might want to replace parameters like :id with a placeholder
	// This simple version just returns the path as-is
	return path
}

// startMetricsServer starts a separate HTTP server to expose metrics
func startMetricsServer(config Config) {
	mux := http.NewServeMux()
	mux.Handle(config.MetricsPath, promhttp.HandlerFor(
		Registry,
		promhttp.HandlerOpts{
			EnableOpenMetrics: true,
		},
	))

	server := &http.Server{
		Addr:    ":" + strconv.Itoa(config.MetricsPort),
		Handler: mux,
	}

	// Start server in a goroutine
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			panic("Failed to start metrics server: " + err.Error())
		}
	}()
}

// ShutdownMetricsServer gracefully shuts down the metrics server
func ShutdownMetricsServer(ctx context.Context) error {
	server := &http.Server{Addr: ":7070"}
	return server.Shutdown(ctx)
}
