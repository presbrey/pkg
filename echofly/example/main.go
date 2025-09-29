package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/presbrey/pkg/echofly"
)

func main() {
	e := echo.New()

	// Add logger middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// Add sticky sessions with custom config
	e.Use(echofly.StickySessionsWithConfig(echofly.StickySessionsConfig{
		CookieName: "session-machine",
		MaxAge:     48 * time.Hour, // 2 days
		Skipper: func(c echo.Context) bool {
			// Skip for API endpoints and health checks
			path := c.Path()
			return path == "/health" || path == "/metrics" ||
				strings.HasPrefix(path, "/api/")
		},
	}))

	// Routes
	e.GET("/", homeHandler)
	e.GET("/health", healthHandler)
	e.GET("/session", sessionHandler)
	e.GET("/api/info", apiInfoHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	e.Logger.Fatal(e.Start(":" + port))
}

func homeHandler(c echo.Context) error {
	machineID := os.Getenv("FLY_MACHINE_ID")
	if machineID == "" {
		machineID = "local-development"
	}

	html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <title>Fly.io Sticky Sessions Demo</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; }
        .machine-id { background: #f0f0f0; padding: 10px; border-radius: 5px; }
        .info { margin: 20px 0; }
    </style>
</head>
<body>
    <h1>🪰 Fly.io Sticky Sessions Demo</h1>
    <div class="machine-id">
        <strong>Machine ID:</strong> %s
    </div>
    <div class="info">
        <p>Refresh this page multiple times - you should always see the same machine ID thanks to sticky sessions!</p>
        <p>Try these endpoints:</p>
        <ul>
            <li><a href="/session">/session</a> - View session information</li>
            <li><a href="/health">/health</a> - Health check (skips sticky middleware)</li>
            <li><a href="/api/info">/api/info</a> - API endpoint (skips sticky middleware)</li>
        </ul>
    </div>
</body>
</html>`, machineID)

	return c.HTML(200, html)
}

func healthHandler(c echo.Context) error {
	return c.JSON(200, map[string]string{
		"status":     "ok",
		"machine_id": os.Getenv("FLY_MACHINE_ID"),
		"note":       "This endpoint skips sticky sessions middleware",
	})
}

func sessionHandler(c echo.Context) error {
	machineID := os.Getenv("FLY_MACHINE_ID")
	if machineID == "" {
		machineID = "local-development"
	}

	cookie, err := c.Cookie("session-machine")
	response := map[string]interface{}{
		"current_machine_id": machineID,
		"fly_environment":    machineID != "local-development",
	}

	if err != nil {
		response["cookie"] = "not set"
		response["sticky"] = false
	} else {
		response["cookie_machine_id"] = cookie.Value
		response["sticky"] = cookie.Value == machineID
	}

	return c.JSON(200, response)
}

func apiInfoHandler(c echo.Context) error {
	return c.JSON(200, map[string]string{
		"service":    "echofly-demo",
		"version":    "1.0.0",
		"machine_id": os.Getenv("FLY_MACHINE_ID"),
		"note":       "API endpoints skip sticky sessions middleware",
	})
}
