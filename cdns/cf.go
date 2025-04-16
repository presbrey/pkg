package cdns

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
)

// CloudflareMiddleware holds configuration for the Cloudflare middleware.
type CloudflareMiddleware struct {
	DisableRedirect bool // If true, disables the automatic HTTPS redirect
	RedirectPort    int  // Port to use for HTTPS redirect (defaults to 443)
}

// CloudflareWithDefaults handles Cloudflare-specific headers
func CloudflareWithDefaults() echo.MiddlewareFunc {
	return NewCloudflareMiddleware().Build()
}

// NewCloudflareMiddleware creates a new CloudflareConfig with default settings.
func NewCloudflareMiddleware() *CloudflareMiddleware {
	return &CloudflareMiddleware{
		RedirectPort: 443, // Default HTTPS port
	}
}

// WithoutRedirect disables the automatic HTTPS redirect.
func (cfg *CloudflareMiddleware) WithoutRedirect() *CloudflareMiddleware {
	cfg.DisableRedirect = true
	return cfg
}

// WithRedirectPort sets a custom port for the HTTPS redirect.
func (cfg *CloudflareMiddleware) WithRedirectPort(port int) *CloudflareMiddleware {
	if port > 0 && port <= 65535 {
		cfg.RedirectPort = port
	}
	return cfg
}

// Build creates the echo.MiddlewareFunc using the configured options.
func (cfg *CloudflareMiddleware) Build() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Check for Cloudflare IP header
			cfIP := c.Request().Header.Get("Cf-Connecting-Ip")
			if cfIP == "" {
				// this isn't Cloudflare
				return next(c)
			}

			// Set RealIP if not already set
			if c.Get("RealIP") == nil {
				c.Set("RealIP", cfIP)
			}

			// If redirect is disabled, pass through
			if cfg.DisableRedirect {
				return next(c)
			}

			// Check if already TLS
			if isTLS, _ := c.Get("IsTLS").(bool); isTLS {
				return next(c)
			}

			// Check for Cf-Visitor header for HTTPS redirect
			cfVisitor := c.Request().Header.Get("Cf-Visitor")

			// Set IsTLS if CDN says it is
			if strings.Contains(cfVisitor, "\"scheme\":\"https\"") {
				c.Set("IsTLS", true)
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
