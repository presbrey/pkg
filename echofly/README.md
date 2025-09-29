# echofly

`echofly` provides middleware for the Echo web framework to make sessions sticky on Fly.io. This middleware ensures that user sessions are consistently routed to the same machine instance, which is useful for applications that store session data in memory or need consistent state.

## Features

- **Automatic Session Stickiness**: Routes users to the same Fly.io machine instance
- **Cookie-based Routing**: Uses HTTP cookies to track which machine should handle requests
- **Configurable**: Customizable cookie name, max age, and skip conditions
- **Fly.io Integration**: Automatically detects Fly.io environment and uses `Fly-Replay` header
- **Echo v4 Compatible**: Works seamlessly with Echo v4 framework

## Installation

```bash
go get github.com/presbrey/pkg/echofly
```

## Quick Start

```go
package main

import (
    "github.com/labstack/echo/v4"
    "github.com/presbrey/pkg/echofly"
)

func main() {
    e := echo.New()
    
    // Add sticky sessions middleware
    e.Use(echofly.StickySessions())
    
    e.GET("/", func(c echo.Context) error {
        return c.String(200, "Hello from machine: " + os.Getenv("FLY_MACHINE_ID"))
    })
    
    e.Logger.Fatal(e.Start(":8080"))
}
```

## Configuration

You can customize the middleware behavior using `StickySessionsWithConfig`:

```go
config := echofly.StickySessionsConfig{
    CookieName: "my-machine-id",           // Custom cookie name
    MaxAge:     24 * time.Hour,           // 1 day instead of default 6 days
    Skipper: func(c echo.Context) bool {  // Skip for health checks
        return c.Path() == "/health"
    },
}

e.Use(echofly.StickySessionsWithConfig(config))
```

## How It Works

1. **First Request**: When a user makes their first request, the middleware checks if they have a machine ID cookie
2. **Set Cookie**: If no cookie exists, it sets a cookie with the current machine's ID (`FLY_MACHINE_ID`)
3. **Subsequent Requests**: On future requests, the middleware checks if the cookie's machine ID matches the current machine
4. **Replay**: If the IDs don't match, it sets the `Fly-Replay` header to route the request to the correct machine
5. **Continue**: If the IDs match, the request continues normally to your handlers

## Environment Requirements

This middleware only activates when running on Fly.io (when `FLY_MACHINE_ID` environment variable is present). In other environments, it acts as a no-op middleware.

## Configuration Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `CookieName` | `string` | `"fly-machine-id"` | Name of the cookie to store machine ID |
| `MaxAge` | `time.Duration` | `6 * 24 * time.Hour` | How long the cookie should last |
| `Skipper` | `func(echo.Context) bool` | `nil` | Function to skip middleware for certain requests |

## Cookie Properties

The middleware sets cookies with the following properties:
- **HttpOnly**: `true` (prevents JavaScript access)
- **SameSite**: `Lax` (allows cross-site navigation)
- **Path**: `/` (applies to entire site)
- **MaxAge**: Configurable (default: 6 days)

## Use Cases

- **In-memory Sessions**: Keep user sessions on the same machine
- **WebSocket Connections**: Ensure WebSocket upgrades happen on the same instance
- **Stateful Applications**: Maintain application state across requests
- **Caching**: Keep user-specific caches on the same machine

## Example with Custom Configuration

```go
package main

import (
    "os"
    "time"
    
    "github.com/labstack/echo/v4"
    "github.com/labstack/echo/v4/middleware"
    "github.com/presbrey/pkg/echofly"
)

func main() {
    e := echo.New()
    
    // Add logger middleware
    e.Use(middleware.Logger())
    
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
    
    e.Logger.Fatal(e.Start(":8080"))
}

func homeHandler(c echo.Context) error {
    machineID := os.Getenv("FLY_MACHINE_ID")
    return c.HTML(200, fmt.Sprintf(`
        <h1>Hello from Fly.io!</h1>
        <p>Machine ID: %s</p>
        <p>Refresh this page - you should always see the same machine ID.</p>
    `, machineID))
}

func healthHandler(c echo.Context) error {
    return c.JSON(200, map[string]string{"status": "ok"})
}

func sessionHandler(c echo.Context) error {
    cookie, err := c.Cookie("session-machine")
    if err != nil {
        return c.JSON(200, map[string]string{
            "machine_id": os.Getenv("FLY_MACHINE_ID"),
            "cookie": "not set",
        })
    }
    
    return c.JSON(200, map[string]string{
        "machine_id": os.Getenv("FLY_MACHINE_ID"),
        "cookie": cookie.Value,
        "sticky": fmt.Sprintf("%t", cookie.Value == os.Getenv("FLY_MACHINE_ID")),
    })
}
```

## License

MIT License
