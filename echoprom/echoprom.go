// Package echoprom provides Echo middleware for Prometheus metrics.
// It records request latency percentiles (75th, 90th, 95th) and status code counts
// per URL path, and exposes them via a /metrics endpoint on port 7070.
package echoprom

import (
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/presbrey/pkg/t"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// Registry is the Prometheus registry used by this package
	Registry = prometheus.NewRegistry()

	// RequestDuration measures request latency
	RequestDuration = promauto.With(Registry).NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "method_code_seconds",
			Help:    "HTTP request latency in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "code"},
	)

	// RequestsTotal counts total requests by status code and path
	RequestsTotal = promauto.With(Registry).NewCounterVec(
		prometheus.CounterOpts{
			Name: "method_code_total",
			Help: "Total number of HTTP requests by status code",
		},
		[]string{"method", "code"},
	)
)

func init() {
	// Register the Go collector (collects metrics about the Go runtime)
	Registry.MustRegister(collectors.NewGoCollector())

	// Register the Process collector (collects metrics about the process)
	Registry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
}

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

// DefaultMiddleware returns Echo middleware which records Prometheus metrics
func DefaultMiddleware() echo.MiddlewareFunc {
	return MiddlewareWithConfig(DefaultConfig())
}

// MiddlewareWithConfig returns Echo middleware with config
func MiddlewareWithConfig(config Config) echo.MiddlewareFunc {
	// Use default config if necessary
	if config.Skipper == nil {
		config.Skipper = DefaultConfig().Skipper
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if config.Skipper(c) {
				return next(c)
			}

			// Start timer
			start := time.Now()
			method := c.Request().Method

			// Execute the request
			err := next(c)

			// Record metrics
			duration := time.Since(start).Seconds()
			status := c.Response().Status
			RequestDuration.WithLabelValues(method, strconv.Itoa(status)).Observe(duration)
			RequestsTotal.WithLabelValues(method, strconv.Itoa(status)).Inc()

			return err
		}
	}
}

// NewMetricsHandler returns a handler for Prometheus metrics
func NewMetricsHandler(config Config) http.Handler {
	return promhttp.HandlerFor(
		Registry,
		promhttp.HandlerOpts{
			EnableOpenMetrics: true,
		},
	)
}

// DefaultRoute registers the metrics endpoint
func DefaultRoute(r t.EchoRouter) {
	r.GET("/metrics", echo.WrapHandler(NewMetricsHandler(DefaultConfig())))
}

// DefaultStart creates a new Echo server with Prometheus metrics
func DefaultStart(address string) error {
	e := echo.New()
	e.HideBanner = true
	DefaultRoute(e)
	return e.Start(address)
}
