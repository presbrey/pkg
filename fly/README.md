# Fly Package

A Go package for interacting with [Fly.io](https://fly.io) services and applications. This package provides utilities for managing Fly.io machines, retrieving logs, and handling Fly.io-specific concerns in web applications.

## Components

### Core Library (`fly/fly.go`)

The core `fly` package provides Go functions for interacting with Fly.io resources:

- Machine management (listing, querying status)
- Log retrieval with support for both streaming and non-streaming modes
- Region configuration (US/EU regions)
- Colorized terminal output for better log readability
- Utilities for tracking flyctl CLI calls

### FlyMiddleware (`cdns/fly.go`)

A middleware for the [Echo](https://echo.labstack.com/) web framework that handles Fly.io-specific headers:

- Client IP detection via `Fly-Client-IP` header
- Automatic HTTPS redirection based on `Fly-Forwarded-Proto`
- Configurable redirect options (custom port, disable redirect)
- Sets `RealIP` and `IsTLS` context values for use in your handlers

### FlySuper Utility (`flysu`)

A command-line tool for Fly.io administrators and operators that provides:

- Aggregated machine management across multiple applications and regions
- Two main commands:
  - `list`: Display machine details across regions with filtering options
  - `logs`: Retrieve and display logs from machines with filtering options
- Region filtering (US-only, EU-only)
- Application-specific targeting
- Formatted, colorized output for better readability

## Installation

```bash
go get github.com/presbrey/pkg/fly
```

## Usage Examples

### Using the Core Library

```go
package main

import (
	"fmt"
	"log"

	"github.com/presbrey/pkg/fly"
)

func main() {
	// Get list of machines for an app
	machines, err := fly.GetMachineList("my-app")
	if err != nil {
		log.Fatalf("Error getting machines: %v", err)
	}
	
	// Print machine details
	for _, machine := range machines {
		fmt.Printf("Machine: %s, Region: %s, State: %s\n", 
			machine.Name, machine.Region, machine.State)
	}
	
	// Get logs for a specific machine (non-streaming mode)
	if len(machines) > 0 {
		logs, err := fly.GetMachineLogs("my-app", machines[0].ID, false)
		if err != nil {
			log.Fatalf("Error getting logs: %v", err)
		}
		fmt.Println(logs)
	}
}
```

### Using the Echo Middleware

```go
package main

import (
	"fmt"
	"github.com/labstack/echo/v4"
	"github.com/presbrey/pkg/cdns"
)

func main() {
	e := echo.New()
	
	// Add Fly.io middleware with default settings
	e.Use(cdns.FlyWithDefaults())
	
	// Or with custom settings
	flyMiddleware := cdns.NewFlyMiddleware().
		WithRedirectPort(8443).        // Custom HTTPS port
		WithoutRedirect()              // Disable auto-redirect
	e.Use(flyMiddleware.Build())
	
	e.GET("/", func(c echo.Context) error {
		// RealIP will be set from Fly-Client-IP if available
		clientIP, _ := c.Get("RealIP").(string)
		isTLS, _ := c.Get("IsTLS").(bool)
		
		return c.String(200, fmt.Sprintf("Hello from %s (TLS: %v)", clientIP, isTLS))
	})
	
	e.Start(":8080")
}
```

### Using the FlySuper Utility

```bash
# List all machines across all regions
flysu list

# View logs for all apps in US regions only
flysu logs --us

# List machines in EU regions only with minimal output
flysu list --eu --quiet

# Follow logs for a specific app
flysu logs -f -a us-east-1-portal

# View help information
flysu help
```

## Configuration

The package can be configured via environment variables:

- `US_REGIONS`: Comma-separated list of US regions (default: "us-east-1, us-east-2, us-east-3, us-east-4")
- `EU_REGIONS`: Comma-separated list of EU regions (default: "eu-west-1, eu-west-2, eu-west-3, eu-west-4")
- `APP_NAMES`: Comma-separated list of application types to monitor (default: "portal, websocket")

## Requirements

- Go 1.15 or higher
- [flyctl](https://fly.io/docs/hands-on/install-flyctl/) installed and configured
- Valid Fly.io authentication

## License

MIT
