# cdns

This package provides Echo (`v4`) middleware for interacting with various Content Delivery Network (CDN) providers, such as Cloudflare and Fly.io.

Key features include:
- Automatic client IP extraction from CDN-specific headers (e.g., `Cf-Connecting-Ip`, `Fly-Client-IP`, fallback to `X-Forwarded-For`). The real IP is stored in `c.Get("RealIP")`.
- Automatic HTTP to HTTPS redirection based on CDN headers (`Cf-Visitor`, `Fly-Forwarded-Proto`). This is enabled by default.
- Fluent configuration API to customize behavior (e.g., disable redirect, set custom redirect port).

## Installation

```bash
go get github.com/presbrey/pkg/cdns
```

## Usage

Import the package:

```go
import "github.com/presbrey/pkg/cdns"
```

### Cloudflare

```go
package main

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/presbrey/pkg/cdns"
)

func main() {
	e := echo.New()

	// Use Cloudflare middleware with defaults (HTTPS redirect enabled on port 443)
	e.Use(echocdn.CloudflareWithDefaults())

	// Or, configure manually:
	// - Disable HTTPS redirect
	// e.Use(echocdn.NewCloudflareMiddleware().WithoutRedirect().Build())
	// - Use a custom redirect port (e.g., 8443)
	// e.Use(echocdn.NewCloudflareMiddleware().WithRedirectPort(8443).Build())

	e.GET("/", func(c echo.Context) error {
		// Access real client IP if needed
		realIP := c.Get("RealIP")
		return c.String(http.StatusOK, "Hello from behind Cloudflare! Your IP: %v", realIP)
	})

	e.Logger.Fatal(e.Start(":8080"))
}
```

### Fly.io

```go
package main

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/presbrey/pkg/cdns"
)

func main() {
	e := echo.New()

	// Use Fly middleware with defaults (HTTPS redirect enabled on port 443)
	e.Use(echocdn.FlyWithDefaults())

	// Or, configure manually:
	// - Disable HTTPS redirect
	// e.Use(echocdn.NewFlyMiddleware().WithoutRedirect().Build())
	// - Use a custom redirect port (e.g., 8443)
	// e.Use(echocdn.NewFlyMiddleware().WithRedirectPort(8443).Build())

	e.GET("/", func(c echo.Context) error {
		// Access real client IP if needed
		realIP := c.Get("RealIP")
		return c.String(http.StatusOK, "Hello from behind Fly.io! Your IP: %v", realIP)
	})

	e.Logger.Fatal(e.Start(":8080"))
}
