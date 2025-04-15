package cdns

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

func UseDefaults(e *echo.Echo) {
	e.Use(CloudflareWithDefaults())
	e.Use(FlyWithDefaults())
	e.IPExtractor = func(r *http.Request) string {
		// check Cloudflare first because its sometimes in front of Fly
		if ip := r.Header.Get("Cf-Connecting-Ip"); ip != "" {
			return ip
		}
		// Fly doesn't go in front of Cloudflare
		if ip := r.Header.Get("Fly-Client-IP"); ip != "" {
			return ip
		}
		return ""
	}
}
