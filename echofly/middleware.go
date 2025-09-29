// Package echofly provides middleware for Echo framework to make sessions sticky on Fly.io
package echofly

import (
	"net/http"
	"os"
	"time"

	"github.com/labstack/echo/v4"
)

const (
	// CookieName is the name of the cookie used to store the machine ID
	CookieName = "fly-machine-id"
	// DefaultMaxAge is the default max age for the cookie (6 days)
	DefaultMaxAge = 6 * 24 * time.Hour
	// FlyReplayHeader is the header used to replay requests to specific instances
	FlyReplayHeader = "Fly-Replay"
)

// StickySessionsConfig holds configuration for the sticky sessions middleware
type StickySessionsConfig struct {
	// CookieName is the name of the cookie to use (default: "fly-machine-id")
	CookieName string
	// MaxAge is the max age for the cookie (default: 6 days)
	MaxAge time.Duration
	// Skipper defines a function to skip middleware
	Skipper func(c echo.Context) bool
}

// DefaultStickySessionsConfig returns the default configuration
func DefaultStickySessionsConfig() StickySessionsConfig {
	return StickySessionsConfig{
		CookieName: CookieName,
		MaxAge:     DefaultMaxAge,
		Skipper:    nil,
	}
}

// StickySessionsWithConfig returns a middleware function with custom configuration
func StickySessionsWithConfig(config StickySessionsConfig) echo.MiddlewareFunc {
	// Set defaults
	if config.CookieName == "" {
		config.CookieName = CookieName
	}
	if config.MaxAge == 0 {
		config.MaxAge = DefaultMaxAge
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Skip middleware if skipper function returns true
			if config.Skipper != nil && config.Skipper(c) {
				return next(c)
			}

			// Get the current machine ID from environment
			machineID := os.Getenv("FLY_MACHINE_ID")

			// If not running on Fly.io, skip the middleware
			if machineID == "" {
				return next(c)
			}

			// Get the cookie from the request
			cookie, err := c.Cookie(config.CookieName)

			if err != nil || cookie.Value == "" {
				// No cookie found, set it with current machine ID
				newCookie := &http.Cookie{
					Name:     config.CookieName,
					Value:    machineID,
					MaxAge:   int(config.MaxAge.Seconds()),
					Path:     "/",
					HttpOnly: true,
					SameSite: http.SameSiteLaxMode,
				}
				c.SetCookie(newCookie)
				return next(c)
			}

			// Cookie exists, check if it matches current machine ID
			if cookie.Value != machineID {
				// Cookie has different machine ID, replay to that instance
				c.Response().Header().Set(FlyReplayHeader, "instance="+cookie.Value)
				return c.NoContent(http.StatusTemporaryRedirect)
			}

			// Cookie matches current machine, continue normally
			return next(c)
		}
	}
}

// StickySessions returns a middleware function with default configuration
func StickySessions() echo.MiddlewareFunc {
	return StickySessionsWithConfig(DefaultStickySessionsConfig())
}
