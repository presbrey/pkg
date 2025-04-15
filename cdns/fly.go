package cdns

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
)

// FlyMiddleware holds configuration for the Fly.io middleware.
type FlyMiddleware struct {
	DisableRedirect bool // If true, disables the automatic HTTPS redirect
	RedirectPort    int  // Port to use for HTTPS redirect (defaults to 443)
}

// FlyWithDefaults handles Fly.io-specific headers using default settings.
func FlyWithDefaults() echo.MiddlewareFunc {
	return NewFlyMiddleware().Build()
}

// NewFlyMiddleware creates a new FlyMiddleware config with default settings.
func NewFlyMiddleware() *FlyMiddleware {
	return &FlyMiddleware{
		RedirectPort: 443, // Default HTTPS port
	}
}

// WithoutRedirect disables the automatic HTTPS redirect.
func (cfg *FlyMiddleware) WithoutRedirect() *FlyMiddleware {
	cfg.DisableRedirect = true
	return cfg
}

// WithRedirectPort sets a custom port for the HTTPS redirect.
func (cfg *FlyMiddleware) WithRedirectPort(port int) *FlyMiddleware {
	if port > 0 && port <= 65535 {
		cfg.RedirectPort = port
	}
	return cfg
}

// Build creates the echo.MiddlewareFunc using the configured options.
func (cfg *FlyMiddleware) Build() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Check for Fly-Client-IP header first (preferred method for client IP)
			flyClientIP := c.Request().Header.Get("Fly-Client-IP")
			if flyClientIP == "" {
				// this isn't Fly.io
				return next(c)
			}

			// Set RealIP if not already set
			if c.Get("RealIP") == nil {
				c.Set("RealIP", flyClientIP)
			}

			if cfg.DisableRedirect {
				return next(c)
			}

			// Check original protocol for HTTP to HTTPS redirect
			flyForwardedProto := c.Request().Header.Get("Fly-Forwarded-Proto")

			// Redirect if the protocol is not https
			if flyForwardedProto == "https" {
				return next(c)
			}

			// If the protocol is not HTTPS, redirect to HTTPS
			host := c.Request().Host
			uri := c.Request().RequestURI

			redirectURL := "https://" + host
			// Append port only if it's not the default 443
			if cfg.RedirectPort != 443 {
				redirectURL += ":" + strconv.Itoa(cfg.RedirectPort)
			}
			redirectURL += uri

			// Redirect to HTTPS using Permanent Redirect
			return c.Redirect(http.StatusMovedPermanently, redirectURL)
		}
	}
}
