# Feature Flags SDK for Go

A powerful and flexible feature flags SDK for Go applications using the Echo framework. This SDK fetches host-specific configuration from a remote URL (like a GitHub repository) and provides typed getters with user-level overrides, a base configuration for defaults, and caching support.

## Features

- 🚀 **Typed Getters**: Support for string, bool, int, float64, string slice, and map types.
- 👤 **User-Level Overrides**: Wildcard defaults with user-specific overrides.
- 🏠 **Base Host Configuration**: Use a base configuration file that is merged with the host-specific configuration for shared defaults.
- 🌐 **Fluent API**: A convenient fluent API for accessing flags within a request context.
- 🗂️ **Nested Path Support**: Access nested configuration values with dot notation.
- 💾 **Configurable Caching**: Built-in caching with configurable TTL for both successful fetches and errors.
- 🔄 **Echo Integration**: Seamless integration with the Echo web framework.
- 🧪 **Well-Tested**: Comprehensive test coverage.
- ⚡ **Thread-Safe**: Concurrent-safe operations.

## Installation

```bash
go get github.com/presbrey/pkg/echoflags
```

## Configuration

### JSON File Format

Each host has a JSON file with the following structure:

```json
{
  "*": {
    "feature1": true,
    "feature2": false,
    "maxItems": 100,
    "discount": 0.1,
    "allowedRegions": ["us-east", "us-west"],
    "metadata": {
      "version": "1.0",
      "tier": "standard"
    }
  },
  "user@example.com": {
    "feature2": true,
    "maxItems": 200,
    "discount": 0.2,
    "allowedRegions": ["us-east", "us-west", "eu-west"]
  }
}
```

- `"*"`: Default configuration for all users.
- User-specific keys (e.g., `"user@example.com"`): Override values for specific users.

## Usage

### Basic Setup - Single File Mode

This mode is ideal when all your flags are in a single file.

```go
package main

import (
    "github.com/labstack/echo/v4"
    "github.com/presbrey/pkg/echoflags"
)

func main() {
    // Simple setup with a single configuration file
    sdk := echoflags.New("https://raw.githubusercontent.com/org/repo/main/config.json")

    e := echo.New()

    e.GET("/data", func(c echo.Context) error {
        // Set user in context (typically from authentication middleware)
        c.Set("user", "user@example.com")

        // Check if a feature is enabled
        if sdk.IsEnabled(c, "feature1") {
            // Feature is enabled
        }

        // Get typed values
        maxItems := sdk.GetIntWithDefault(c, "maxItems", 0)
        discount := sdk.GetFloat64WithDefault(c, "discount", 0.0)
        regions := sdk.GetStringSliceWithDefault(c, "allowedRegions", nil)

        return c.JSON(200, map[string]interface{}{
            "maxItems": maxItems,
            "discount": discount,
            "regions":  regions,
        })
    })

    e.Start(":8080")
}
```

### Advanced Setup - Multi-Host Mode

This mode loads configuration based on the request's host and merges it with a base configuration file.

```go
package main

import (
    "time"
    "github.com/labstack/echo/v4"
    "github.com/presbrey/pkg/echoflags"
)

func main() {
    // Advanced setup with multi-host support
    sdk := echoflags.NewWithConfig(echoflags.Config{
        FlagsBase: "https://raw.githubusercontent.com/org/repo/main/hosts",
        BaseHost:  "base-host", // A base config to merge with the host's config
        CacheTTL:  5 * time.Minute,
    })

    e := echo.New()

    e.GET("/data", func(c echo.Context) error {
        // Set user in context (typically from authentication middleware)
        c.Set("user", "user@example.com")

        // Check if a feature is enabled. This will check the host-specific config
        // first, then the base-host config.
        if sdk.IsEnabled(c, "feature1") {
            // Feature is enabled
        }

        // Access nested configuration
        // e.g., metadata.features.new_dashboard
        newDashboard, _ := sdk.GetBool(c, "metadata.features.new_dashboard")

        return c.JSON(200, map[string]interface{}{
            "newDashboard": newDashboard,
        })
    })

    e.Start(":8080")
}
```

### Fluent API

For more concise code inside your handlers, you can use the fluent API.

```go
e.GET("/fluent", func(c echo.Context) error {
    flags := sdk.WithContext(c)

    // No need to pass the context `c` to every call
    maxItems := flags.GetIntWithDefault("maxItems", 0)
    isEnabled := flags.IsEnabled("feature1")

    return c.JSON(200, map[string]interface{}{
        "maxItems": maxItems,
        "enabled": isEnabled,
    })
})
```

### Authentication Middleware

```go
func AuthMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
    return func(c echo.Context) error {
        // Extract user from JWT, session, etc.
        user := getUserFromToken(c.Request().Header.Get("Authorization"))
        
        // Set user in context for feature flags SDK
        c.Set("user", user)
        
        return next(c)
    }
}
```

## Available Methods

All getter methods support **dot notation** for accessing nested values (e.g., `"metadata.features.dashboard"`).

### Basic Getters

These methods return a value and an error if the key is not found or the type is wrong.

- `GetString(c echo.Context, key string) (string, error)`
- `GetBool(c echo.Context, key string) (bool, error)`
- `GetInt(c echo.Context, key string) (int, error)`
- `GetFloat64(c echo.Context, key string) (float64, error)`
- `GetStringSlice(c echo.Context, key string) ([]string, error)`
- `GetMap(c echo.Context, key string) (map[string]interface{}, error)`
- `IsEnabled(c echo.Context, key string) bool` - A convenient helper that returns `false` on error.

### Default Value Getters (Recommended)

These methods never return errors and provide a fallback default value.

- `GetStringWithDefault(c echo.Context, key string, defaultValue string) string`
- `GetBoolWithDefault(c echo.Context, key string, defaultValue bool) bool`
- `GetIntWithDefault(c echo.Context, key string, defaultValue int) int`
- `GetFloat64WithDefault(c echo.Context, key string, defaultValue float64) float64`
- `GetStringSliceWithDefault(c echo.Context, key string, defaultValue []string) []string`
- `GetMapWithDefault(c echo.Context, key string, defaultValue map[string]interface{}) map[string]interface{}`

### Fluent API Getters

The same methods are available on the `FlagSet` object returned by `sdk.WithContext(c)`.

- `fs.GetString(key string) (string, error)`
- `fs.GetStringWithDefault(key string, defaultValue string) string`
- `fs.IsEnabled(key string) bool`
- ...and so on for all other types.

### Cache Management

```go
// Pre-load configuration for the given context to warm the cache
// or ensure flags are available before first use.
err := sdk.EnsureLoaded(c)
if err != nil {
    // Handle error, e.g., log it or use fallback values
}

// Clear all cache entries
sdk.ClearCache()

// Clear cache for a specific configuration URL
// This is useful if you've updated a single flag file.
flagsURL := "https://raw.githubusercontent.com/org/repo/main/hosts/tenant1.json"
sdk.ClearCacheKey(flagsURL)
```

### Configuration Options

```go
type Config struct {
    // FlagsURL is the URL for a single static configuration file.
    // If set, the SDK operates in single-file mode.
    FlagsURL string

    // FlagsBase is the base URL for the directory containing host JSON files.
    // Example: "https://raw.githubusercontent.com/org/repo/main/hosts"
    FlagsBase string

    // BaseHost is the name of the base configuration file (e.g., "base-config")
    // to be merged with the host-specific configuration.
    BaseHost string

    // DisableCache disables caching when set to true.
    DisableCache bool

    // CacheTTL is the time-to-live for cached entries.
    // Default: 5 minutes.
    CacheTTL time.Duration

    // ErrorTTL is the time-to-live for cached errors (e.g., 404s).
    // Default: 1 minute.
    ErrorTTL time.Duration

    // HTTPClient allows providing a custom HTTP client.
    HTTPClient *http.Client

    // DefaultUser is used as the user identifier when GetUserFunc returns an empty string.
    DefaultUser string

    // GetFlagsURL allows custom logic to build the flag file URL from the context.
    GetFlagsURL func(c echo.Context, host string) string

    // GetUserFunc allows custom logic to extract a user identifier from the context.
    // Default: gets the "user" value from the context.
    GetUserFunc func(c echo.Context) string
}
```

## Advanced Usage

### Base Host Configuration

The SDK supports merging a `BaseHost` configuration with a host-specific configuration. This is useful for providing global defaults while allowing specific hosts to override them.

The merging logic is as follows:
1.  The `BaseHost` configuration is loaded.
2.  The host-specific configuration is loaded.
3.  The two configurations are merged. For any given key, the host-specific value takes precedence. For nested maps, values are merged recursively. For arrays and other types, the host value replaces the base value entirely.

**Precedence Order for a resolved key:**
1.  User-specific value from the host's file.
2.  Wildcard (`"*"`) value from the host's file.
3.  User-specific value from the `BaseHost` file (if not present in the host's file).
4.  Wildcard (`"*"`) value from the `BaseHost` file (if not present in the host's file).

### Custom URL and User Extraction

You can provide custom functions to control how the configuration URL is determined and how the user is identified.

```go
sdk := echoflags.NewWithConfig(echoflags.Config{
    FlagsBase: "https://raw.githubusercontent.com/org/repo/main/hosts",
    GetFlagsURL: func(c echo.Context, host string) string {
        // Extract tenant from custom header to build the URL
        tenant := c.Request().Header.Get("X-Tenant-ID")
        if tenant == "" {
            tenant = "default"
        }
        return fmt.Sprintf("https://raw.githubusercontent.com/org/repo/main/hosts/%s.json", tenant)
    },
    GetUserFunc: func(c echo.Context) string {
        // Extract user from JWT claims
        if token, ok := c.Get("jwt-claims").(jwt.MapClaims); ok {
            return token["sub"].(string)
        }
        return "" // No user
    },
})
```

### Custom HTTP Client

```go
sdk := echoflags.NewWithConfig(echoflags.Config{
    FlagsBase: "https://raw.githubusercontent.com/org/repo/main/hosts",
    HTTPClient: &http.Client{
        Timeout: 10 * time.Second,
        Transport: &http.Transport{
            MaxIdleConns:    100,
            IdleConnTimeout: 90 * time.Second,
        },
    },
})
```

### Feature Flags with A/B Testing

```go
func handleRequest(c echo.Context) error {
    // Get user-specific configuration with fallback support
    variant := sdk.GetStringWithDefault(c, "experimentVariant", "control")
    
    switch variant {
    case "A":
        return handleVariantA(c)
    case "B":
        return handleVariantB(c)
    default:
        return handleDefault(c)
    }
}
```

### Progressive Rollouts

```go
func isFeatureEnabledForUser(c echo.Context) bool {
    // Check if user is in rollout percentage (with fallback to 0%)
    rolloutPercentage := sdk.GetFloat64WithDefault(c, "rolloutPercentage", 0.0)
    userHash := hashUser(c.Get("user").(string))
    
    return float64(userHash%100) < rolloutPercentage
}
```

## Testing

Run the test suite:

```bash
go test -v ./...
```

Run with coverage:

```bash
go test -cover -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## Repository Structure

When using multi-host mode, your repository can be structured like this:

```
repo/
└── hosts/
    ├── tenant1.json      # Host-specific configuration
    ├── tenant2.json      # Another host's configuration
    └── base-host.json    # Base configuration for all hosts
```

## Best Practices

1.  **Use Caching**: Enable caching in production to reduce latency and API calls.
2.  **Use a Base Host**: Configure a `BaseHost` for global defaults and shared configuration.
3.  **Use `*WithDefault` Getters**: Prefer the `Get...WithDefault` methods to avoid panics and make your code more resilient to missing flags.
4.  **Set User Context**: Set the user identifier in the Echo context, typically via an authentication middleware.
5.  **Cache Invalidation**: Use `ClearCache()` or `ClearCacheKey()` if you need to force-reload configuration.
6.  **Dot Notation**: Use dot notation (`"a.b.c"`) to access nested configuration values.

## Error Handling

```go
// Method 1: Handle errors explicitly
value, err := sdk.GetString(c, "key")
if err != nil {
    if strings.Contains(err.Error(), "not found") {
        // Key doesn't exist in the merged configuration
        return defaultValue
    }
    // Handle other errors (network, JSON parsing, etc.)
    log.Printf("Error getting feature flag: %v", err)
    return defaultValue
}

// Method 2: Use methods with default values (recommended)
value := sdk.GetStringWithDefault(c, "key", "defaultValue")
enabled := sdk.GetBoolWithDefault(c, "feature", false)
```

## Performance Considerations

- **Caching**: Reduces network latency and GitHub API rate limits.
- **Error Caching**: Failed requests (404s, network errors) are cached for `ErrorTTL` duration to prevent repeated failures.
- **Configuration Merging**: In multi-host mode, fetching and merging a base configuration may result in an additional HTTP request.
- **Concurrent Access**: The SDK is thread-safe for concurrent requests.
- **Memory Usage**: The cache stores parsed JSON for each configuration URL, plus any cached errors.
- **Network Timeouts**: Configure appropriate HTTP client timeouts.

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Support

For issues, questions, or contributions, please open an issue on GitHub.
