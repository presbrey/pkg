package echocdn

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
)

// FlyMiddleware holds configuration for the Fly.io middleware.
type FlyMiddleware struct {
	RedirectPort int // Port to use for HTTPS redirect (defaults to 443)
}

// NewFlyMiddleware creates a new FlyMiddleware config with default settings.
func NewFlyMiddleware() *FlyMiddleware {
	return &FlyMiddleware{
		RedirectPort: 443, // Default HTTPS port
	}
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
			if flyClientIP != "" {
				c.Set("RealIP", flyClientIP)
			} else {
				// Fallback to X-Forwarded-For header if Fly-Client-IP is not present
				xForwardedFor := c.Request().Header.Get("X-Forwarded-For")
				if xForwardedFor != "" {
					// X-Forwarded-For contains a comma-separated list
					// The first IP is typically the original client IP
					ips := strings.Split(xForwardedFor, ",")
					if len(ips) > 0 {
						clientIP := strings.TrimSpace(ips[0])
						c.Set("RealIP", clientIP)
					}
				}
			}

			// Check original protocol for HTTP to HTTPS redirect
			flyForwardedProto := c.Request().Header.Get("Fly-Forwarded-Proto")
			// Redirect if the protocol is explicitly http (or anything other than https)
			if flyForwardedProto != "" && flyForwardedProto != "https" {
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

			return next(c)
		}
	}
}

// FlyWithDefaults handles Fly.io-specific headers using default settings.
func FlyWithDefaults() echo.MiddlewareFunc {
	return NewFlyMiddleware().Build()
}
