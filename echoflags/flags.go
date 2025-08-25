package echoflags

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
)

// Config holds the SDK configuration
type Config struct {
	// FlagsURL is the URL for a single static configuration file (used in single file mode)
	FlagsURL string

	// MultihostBase is the base URL for the HTTP repository containing tenant JSON files
	// Example: "https://raw.githubusercontent.com/org/repo/main/tenants"
	MultihostBase string

	// DisableCache disables caching when set to true
	DisableCache bool

	// CacheTTL is the time-to-live for cached entries (default: 5 minutes)
	CacheTTL time.Duration

	// ErrorTTL is the time-to-live for cached errors (404s, network errors, etc.)
	ErrorTTL time.Duration

	// HTTPClient allows custom HTTP client configuration
	HTTPClient *http.Client

	// DefaultHost is used when no host is specified
	DefaultHost string

	// FallbackHost is used when a key is not found in the primary host
	FallbackHost string

	// GetHostFromContext allows custom logic to extract host from context
	GetHostFromContext func(c echo.Context) string

	// GetUserFromContext allows custom logic to extract user from context
	GetUserFromContext func(c echo.Context) string
}

// HostConfig represents the structure of a host's JSON configuration
type HostConfig map[string]map[string]interface{}

// SDK is the main feature flags SDK
type SDK struct {
	config Config
	cache  *cache
}

// cache represents an in-memory cache
type cache struct {
	mu      sync.RWMutex
	entries map[string]*cacheEntry
}

type cacheEntry struct {
	data      HostConfig
	err       error
	expiresAt time.Time
}

// NewWithConfig creates a new SDK instance with multi-tenant support based on request host
func NewWithConfig(config Config) *SDK {
	if config.HTTPClient == nil {
		config.HTTPClient = &http.Client{
			Timeout: 30 * time.Second,
		}
	}

	if config.CacheTTL == 0 {
		config.CacheTTL = 5 * time.Minute
	}

	if config.ErrorTTL == 0 {
		config.ErrorTTL = 1 * time.Minute
	}

	if config.GetHostFromContext == nil {
		config.GetHostFromContext = func(c echo.Context) string {
			if h := c.Request().Host; h != "" {
				return h
			}
			return c.Request().URL.Host
		}
	}

	if config.GetUserFromContext == nil {
		config.GetUserFromContext = func(c echo.Context) string {
			if user, ok := c.Get("user").(string); ok {
				return user
			}
			return ""
		}
	}

	return &SDK{
		config: config,
		cache: &cache{
			entries: make(map[string]*cacheEntry),
		},
	}
}

// New creates a new SDK instance that uses a single static configuration file
func New(flagsURL string) *SDK {
	return NewWithConfig(Config{
		FlagsURL: flagsURL,
		GetHostFromContext: func(c echo.Context) string {
			return "*"
		},
	})
}

// fetchHostConfig fetches the host configuration from HTTP
func (s *SDK) fetchHostConfig(ctx context.Context, host string) (HostConfig, error) {
	var url string
	if s.config.FlagsURL != "" {
		// Single file mode - always use the same file
		url = s.config.FlagsURL
	} else {
		// Multihost mode - construct URL based on host
		url = fmt.Sprintf("%s/%s.json", s.config.MultihostBase, host)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := s.config.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching config: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	var config HostConfig
	if err := json.Unmarshal(body, &config); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	return config, nil
}

// getHostConfig gets the host configuration with caching support
func (s *SDK) getHostConfig(ctx context.Context, host string) (HostConfig, error) {
	if s.config.DisableCache {
		return s.fetchHostConfig(ctx, host)
	}

	// Check cache
	s.cache.mu.RLock()
	if entry, exists := s.cache.entries[host]; exists {
		if time.Now().Before(entry.expiresAt) {
			s.cache.mu.RUnlock()
			// Return cached error or data
			if entry.err != nil {
				return nil, entry.err
			}
			return entry.data, nil
		}
	}
	s.cache.mu.RUnlock()

	// Fetch from source
	config, err := s.fetchHostConfig(ctx, host)

	// Update cache with either success or error
	s.cache.mu.Lock()
	if err != nil {
		// Cache the error for ErrorTTL duration
		s.cache.entries[host] = &cacheEntry{
			err:       err,
			expiresAt: time.Now().Add(s.config.ErrorTTL),
		}
		s.cache.mu.Unlock()
		return nil, err
	}

	// Cache successful response for CacheTTL duration
	s.cache.entries[host] = &cacheEntry{
		data:      config,
		expiresAt: time.Now().Add(s.config.CacheTTL),
	}
	s.cache.mu.Unlock()

	return config, nil
}

// getValue retrieves a value for a key (supporting dot notation paths) with wildcard and user-specific overrides.
func (s *SDK) getValue(c echo.Context, key string) (interface{}, error) {
	if key == "" {
		return nil, fmt.Errorf("key cannot be empty")
	}

	host := s.config.GetHostFromContext(c)
	if host == "" {
		host = s.config.DefaultHost
	}
	if host == "" {
		return nil, fmt.Errorf("no host specified")
	}

	// Split the key by dots to handle nested paths
	parts := strings.Split(key, ".")
	rootKey := parts[0]

	// Get host config
	config, err := s.getHostConfig(c.Request().Context(), host)
	if err != nil {
		return nil, err
	}

	user := s.config.GetUserFromContext(c)

	// Start with wildcard value for root key
	var value interface{}
	if wildcardConfig, exists := config["*"]; exists {
		if v, ok := wildcardConfig[rootKey]; ok {
			value = v
		}
	}

	// Override with user-specific value if available for root key
	if user != "" {
		if userConfig, exists := config[user]; exists {
			if v, ok := userConfig[rootKey]; ok {
				value = v
			}
		}
	}

	// Try fallback host if root key not found in primary host
	if value == nil && s.config.FallbackHost != "" && s.config.FallbackHost != host {
		fallbackConfig, err := s.getHostConfig(c.Request().Context(), s.config.FallbackHost)
		if err == nil {
			// Start with wildcard value from fallback host
			if wildcardConfig, exists := fallbackConfig["*"]; exists {
				if v, ok := wildcardConfig[rootKey]; ok {
					value = v
				}
			}

			// Override with user-specific value from fallback host if available
			if user != "" {
				if userConfig, exists := fallbackConfig[user]; exists {
					if v, ok := userConfig[rootKey]; ok {
						value = v
					}
				}
			}
		}
	}

	if value == nil {
		return nil, fmt.Errorf("key %s not found", key)
	}

	// If we have nested path (more than one part), traverse the nested structure
	if len(parts) > 1 {
		currentValue := value
		for i := 1; i < len(parts); i++ {
			pathKey := parts[i]
			currentMap, ok := currentValue.(map[string]interface{})
			if !ok {
				traversedPath := strings.Join(parts[:i], ".")
				return nil, fmt.Errorf("value at path '%s' is not a map, cannot resolve '%s'", traversedPath, pathKey)
			}

			nestedValue, found := currentMap[pathKey]
			if !found {
				traversedPath := strings.Join(parts[:i+1], ".")
				return nil, fmt.Errorf("key not found at path '%s'", traversedPath)
			}
			currentValue = nestedValue
		}
		return currentValue, nil
	}

	return value, nil
}

// ClearCache clears all cached entries
func (s *SDK) ClearCache() {
	s.cache.mu.Lock()
	defer s.cache.mu.Unlock()
	s.cache.entries = make(map[string]*cacheEntry)
}

// ClearHostCache clears cache for a specific host
func (s *SDK) ClearHostCache(host string) {
	s.cache.mu.Lock()
	defer s.cache.mu.Unlock()
	delete(s.cache.entries, host)
}

// EnsureLoaded ensures that at least one successful fetch has occurred for the host.
// In single-file mode (FlagsURL set), it performs one fetch for the static file.
// In multihost mode, it performs a synchronous fetch for the primary host and fallback host (if configured).
// Returns error if fetches fail, nil if at least one succeeds.
func (s *SDK) EnsureLoaded(c echo.Context) error {
	ctx := c.Request().Context()

	// Single-file mode - just fetch the static file once
	if s.config.FlagsURL != "" {
		_, err := s.getHostConfig(ctx, "*")
		return err
	}

	// Multihost mode
	host := s.config.GetHostFromContext(c)
	if host == "" {
		host = s.config.DefaultHost
	}
	if host == "" {
		return fmt.Errorf("no host specified")
	}

	// Try to load primary host
	_, primaryErr := s.getHostConfig(ctx, host)

	// If fallback host is configured and different from primary, try it too
	if s.config.FallbackHost != "" && s.config.FallbackHost != host {
		_, fallbackErr := s.getHostConfig(ctx, s.config.FallbackHost)

		// Success if either host loaded successfully
		if primaryErr == nil || fallbackErr == nil {
			return nil
		}

		// Both failed - return the primary error as it's more relevant
		return fmt.Errorf("failed to load tenant configs - primary: %w", primaryErr)
	}

	// Only primary tenant, return its result
	return primaryErr
}
