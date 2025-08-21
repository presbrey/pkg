# Feature Flags SDK for Go

A powerful and flexible feature flags SDK for Go applications using Echo framework. This SDK fetches tenant-specific configuration from a GitHub repository and provides typed getters with user-level overrides, fallback tenants, and caching support.

## Features

- 🚀 **Typed Getters**: Support for string, bool, int, float64, string slice, and map types
- 👤 **User-Level Overrides**: Wildcard defaults with user-specific overrides
- 🔄 **Fallback Tenants**: Automatic fallback to another tenant when keys are not found
- 🌐 **Flexible Tenant Extraction**: Extract tenant from request host or custom logic
- 🗂️ **Nested Path Support**: Access nested configuration values with dot notation
- 💾 **Configurable Caching**: Built-in caching with configurable TTL
- 🔄 **Echo Integration**: Seamless integration with Echo web framework
- 🧪 **Well-Tested**: Comprehensive test coverage with 55+ test cases
- ⚡ **Thread-Safe**: Concurrent-safe operations

## Installation

```bash
go get github.com/presbrey/pkg/echoflags
```

## Configuration

### JSON File Format

Each tenant has a JSON file in your GitHub repository with the following structure:

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

- `"*"`: Default configuration for all users
- User-specific keys (e.g., `"user@example.com"`): Override values for specific users

## Usage

### Basic Setup

```go
package main

import (
    "time"
    "github.com/labstack/echo/v4"
    "github.com/presbrey/pkg/echoflags"
)

func main() {
    // Initialize the SDK
    sdk := echoflags.New(echoflags.Config{
        GitHubRepoURL:  "https://raw.githubusercontent.com/org/repo/main/tenants",
        CacheEnabled:   true,
        CacheTTL:       5 * time.Minute,
        DefaultTenant:  "default",
        FallbackTenant: "global", // Fallback to "global" tenant when keys not found
    })

    // Create Echo app
    e := echo.New()

    // Use SDK in your handlers
    e.GET("/data", func(c echo.Context) error {
        // Set user in context (typically from authentication middleware)
        c.Set("user", "user@example.com")

        // Check if feature is enabled
        if sdk.IsEnabled(c, "feature1") {
            // Feature is enabled
        }

        // Get typed values (will fallback to "global" tenant if not found)
        maxItems, _ := sdk.GetInt(c, "maxItems")
        discount, _ := sdk.GetFloat64(c, "discount")
        regions, _ := sdk.GetStringSlice(c, "allowedRegions")

        // Access nested configuration
        version, _ := sdk.GetBoolByPath(c, "metadata.features.newDashboard")

        return c.JSON(200, map[string]interface{}{
            "maxItems":     maxItems,
            "discount":     discount,
            "regions":      regions,
            "newDashboard": version,
        })
    })

    e.Start(":8080")
}
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

- `GetString(c echo.Context, key string) (string, error)` - Get string value (supports dot notation)
- `GetBool(c echo.Context, key string) (bool, error)` - Get boolean value (supports dot notation)
- `GetInt(c echo.Context, key string) (int, error)` - Get integer value (supports dot notation)
- `GetFloat64(c echo.Context, key string) (float64, error)` - Get float64 value (supports dot notation)
- `GetStringSlice(c echo.Context, key string) ([]string, error)` - Get string slice value (supports dot notation)
- `GetMap(c echo.Context, key string) (map[string]interface{}, error)` - Get map value (supports dot notation)
- `IsEnabled(c echo.Context, key string) bool` - Check if boolean feature is enabled (returns false on error, supports dot notation)

### Default Value Getters (Recommended)

These methods never return errors and provide fallback values:

- `GetStringWithDefault(c echo.Context, key string, defaultValue string) string`
- `GetBoolWithDefault(c echo.Context, key string, defaultValue bool) bool`
- `GetIntWithDefault(c echo.Context, key string, defaultValue int) int`
- `GetFloat64WithDefault(c echo.Context, key string, defaultValue float64) float64`
- `GetStringSliceWithDefault(c echo.Context, key string, defaultValue []string) []string`
- `GetMapWithDefault(c echo.Context, key string, defaultValue map[string]interface{}) map[string]interface{}`

### Usage Examples

```go
// Simple key access
enabled := sdk.GetBoolWithDefault(c, "feature1", false)

// Nested path access with dot notation
dashboardEnabled := sdk.GetBoolWithDefault(c, "metadata.features.dashboard", false)
maxUsers := sdk.GetIntWithDefault(c, "limits.maxUsers", 100)
```

#### Cache Management

```go
// Clear all cache entries
sdk.ClearCache()

// Clear cache for specific tenant
sdk.ClearTenantCache("tenant1")
```

### Configuration Options

```go
type Config struct {
    // Base URL for HTTP repository containing tenant JSON files
    // Example: "https://raw.githubusercontent.com/org/repo/main/tenants"
    HTTPBaseURL string

    // DisableCache disables caching when set to true
    DisableCache bool

    // Time-to-live for cached entries
    // Default: 5 minutes
    CacheTTL time.Duration

    // ErrorTTL is the time-to-live for cached errors (404s, network errors, etc.)
    // Default: 1 minute
    ErrorTTL time.Duration

    // Custom HTTP client (optional)
    HTTPClient *http.Client

    // Default tenant to use when none specified (optional)
    DefaultTenant string

    // Fallback tenant to use when keys are not found in primary tenant (optional)
    FallbackTenant string

    // Custom function to extract tenant from Echo context (optional)
    // Default: extracts from request host
    GetTenantFromContext func(c echo.Context) string

    // Custom function to extract user from Echo context (optional)
    // Default: gets "user" from context
    GetUserFromContext func(c echo.Context) string
}
```

## Advanced Usage

### Fallback Tenant Configuration

The SDK supports automatic fallback to another tenant when a key is not found in the primary tenant. This is useful for providing default configurations or gradual feature rollouts.

```go
sdk := echoflags.New(echoflags.Config{
    HTTPBaseURL:  "https://raw.githubusercontent.com/org/repo/main/tenants",
    FallbackTenant: "global", // Fallback to "global" tenant
})

// If "newFeature" doesn't exist in the primary tenant (e.g., "tenant1"),
// it will automatically check the "global" tenant
enabled := sdk.IsEnabled(c, "newFeature")
```

**Precedence Order:**
1. Primary tenant user-specific override
2. Primary tenant wildcard (`"*"`) value
3. Fallback tenant user-specific override
4. Fallback tenant wildcard (`"*"`) value
5. Error if not found in either tenant

### Custom Tenant and User Extraction

```go
sdk := echoflags.New(echoflags.Config{
    HTTPBaseURL: "https://raw.githubusercontent.com/org/repo/main/tenants",
    GetTenantFromContext: func(c echo.Context) string {
        // Extract tenant from custom header
        return c.Request().Header.Get("X-Tenant-ID")
    },
    GetUserFromContext: func(c echo.Context) string {
        // Extract user from JWT claims
        token := c.Get("jwt-claims").(jwt.MapClaims)
        return token["sub"].(string)
    },
})
```

### Custom HTTP Client

```go
sdk := echoflags.New(echoflags.Config{
    HTTPBaseURL: "https://raw.githubusercontent.com/org/repo/main/tenants",
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

### Multi-Tenant Configuration with Fallback

```go
// Example: tenant-specific configuration with global defaults
func getTenantLimits(c echo.Context) (int, int) {
    // These will check tenant-specific config first, then fallback to "global" tenant
    maxUsers := sdk.GetIntWithDefault(c, "maxUsers", 100)
    maxStorage := sdk.GetIntWithDefault(c, "maxStorageGB", 10)
    
    return maxUsers, maxStorage
}

// Nested configuration access
func getFeatureConfig(c echo.Context) map[string]bool {
    features := make(map[string]bool)
    
    // Access nested configuration with fallback using dot notation
    features["dashboard"] = sdk.GetBoolWithDefault(c, "features.dashboard", false)
    features["analytics"] = sdk.GetBoolWithDefault(c, "features.analytics", false)
    features["api"] = sdk.GetBoolWithDefault(c, "features.api", false)
    
    return features
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

Your GitHub repository should have the following structure:

```
repo/
├── tenants/
│   ├── tenant1.json      # Tenant-specific configuration
│   ├── tenant2.json      # Another tenant
│   ├── tenant3.json      # Yet another tenant
│   ├── global.json       # Global fallback configuration
│   └── default.json      # Default tenant configuration
```

## Best Practices

1. **Use Caching**: Enable caching in production to reduce GitHub API calls
2. **Set Default Tenant**: Configure a default tenant for fallback when no tenant is specified
3. **Configure Fallback Tenant**: Use a fallback tenant for global defaults and gradual rollouts
4. **Handle Errors**: Always handle errors from getter methods or use `*WithDefault` methods
5. **User Context**: Set user in Echo context through authentication middleware
6. **Cache Invalidation**: Clear cache when updating configuration files
7. **Tenant Extraction**: By default, tenant is extracted from request host - customize if needed
8. **Nested Configuration**: All getters support dot notation for nested values (e.g., `"features.dashboard"`)

## Error Handling

```go
// Method 1: Handle errors explicitly
value, err := sdk.GetString(c, "key")
if err != nil {
    if strings.Contains(err.Error(), "not found") {
        // Key doesn't exist in primary or fallback tenant
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

- **Caching**: Reduces network latency and GitHub API rate limits
- **Error Caching**: Failed requests (404s, network errors) are cached for `ErrorTTL` duration to prevent repeated failures
- **Fallback Tenants**: May result in additional HTTP requests when keys are not found
- **Concurrent Access**: SDK is thread-safe for concurrent requests
- **Memory Usage**: Cache stores parsed JSON for both primary and fallback tenants, plus cached errors
- **Network Timeouts**: Configure appropriate HTTP client timeouts
- **Request Batching**: Consider batching multiple flag checks in a single request

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