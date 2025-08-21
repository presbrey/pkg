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
	// HTTPBaseURL is the base URL for the HTTP repository containing tenant JSON files
	// Example: "https://raw.githubusercontent.com/org/repo/main/tenants"
	HTTPBaseURL string

	// DisableCache disables caching when set to true
	DisableCache bool

	// CacheTTL is the time-to-live for cached entries (default: 5 minutes)
	CacheTTL time.Duration

	// ErrorTTL is the time-to-live for cached errors (404s, network errors, etc.)
	ErrorTTL time.Duration

	// HTTPClient allows custom HTTP client configuration
	HTTPClient *http.Client

	// DefaultTenant is used when no tenant is specified
	DefaultTenant string

	// FallbackTenant is used when a key is not found in the primary tenant
	FallbackTenant string

	// GetTenantFromContext allows custom logic to extract tenant from context
	GetTenantFromContext func(c echo.Context) string

	// GetUserFromContext allows custom logic to extract user from context
	GetUserFromContext func(c echo.Context) string
}

// TenantConfig represents the structure of a tenant's JSON configuration
type TenantConfig map[string]map[string]interface{}

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
	data      TenantConfig
	err       error
	expiresAt time.Time
}

// New creates a new SDK instance with the given configuration
func New(config Config) *SDK {
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

	if config.GetTenantFromContext == nil {
		config.GetTenantFromContext = func(c echo.Context) string {
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

// fetchTenantConfig fetches the tenant configuration from HTTP
func (s *SDK) fetchTenantConfig(ctx context.Context, tenant string) (TenantConfig, error) {
	url := fmt.Sprintf("%s/%s.json", s.config.HTTPBaseURL, tenant)

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

	var config TenantConfig
	if err := json.Unmarshal(body, &config); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	return config, nil
}

// getTenantConfig gets the tenant configuration with caching support
func (s *SDK) getTenantConfig(ctx context.Context, tenant string) (TenantConfig, error) {
	if s.config.DisableCache {
		return s.fetchTenantConfig(ctx, tenant)
	}

	// Check cache
	s.cache.mu.RLock()
	if entry, exists := s.cache.entries[tenant]; exists {
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
	config, err := s.fetchTenantConfig(ctx, tenant)

	// Update cache with either success or error
	s.cache.mu.Lock()
	if err != nil {
		// Cache the error for ErrorTTL duration
		s.cache.entries[tenant] = &cacheEntry{
			err:       err,
			expiresAt: time.Now().Add(s.config.ErrorTTL),
		}
		s.cache.mu.Unlock()
		return nil, err
	}

	// Cache successful response for CacheTTL duration
	s.cache.entries[tenant] = &cacheEntry{
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

	tenant := s.config.GetTenantFromContext(c)
	if tenant == "" {
		tenant = s.config.DefaultTenant
	}
	if tenant == "" {
		return nil, fmt.Errorf("no tenant specified")
	}

	// Split the key by dots to handle nested paths
	parts := strings.Split(key, ".")
	rootKey := parts[0]

	// Get tenant config
	config, err := s.getTenantConfig(c.Request().Context(), tenant)
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

	// Try fallback tenant if root key not found in primary tenant
	if value == nil && s.config.FallbackTenant != "" && s.config.FallbackTenant != tenant {
		fallbackConfig, err := s.getTenantConfig(c.Request().Context(), s.config.FallbackTenant)
		if err == nil {
			// Start with wildcard value from fallback tenant
			if wildcardConfig, exists := fallbackConfig["*"]; exists {
				if v, ok := wildcardConfig[rootKey]; ok {
					value = v
				}
			}

			// Override with user-specific value from fallback tenant if available
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

// ClearTenantCache clears cache for a specific tenant
func (s *SDK) ClearTenantCache(tenant string) {
	s.cache.mu.Lock()
	defer s.cache.mu.Unlock()
	delete(s.cache.entries, tenant)
}
