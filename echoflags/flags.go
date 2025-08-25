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

var ContextHost = func(c echo.Context) string {
	if h := c.Request().Host; h != "" {
		return h
	} else if h := c.Request().URL.Host; h != "" {
		return h
	}
	return ""
}

// Config holds the SDK configuration
type Config struct {
	// FlagsURL is the URL for a single static configuration file (used in single file mode)
	// When set, FlagsBase is ignored and the SDK operates in single-file mode
	FlagsURL string

	// FlagsBase is the base URL for the HTTP repository containing host JSON files
	// Example: "https://raw.githubusercontent.com/org/repo/main/hosts"
	// Only used when FlagsURL is empty
	FlagsBase string

	// DisableCache disables caching when set to true
	DisableCache bool

	// CacheTTL is the time-to-live for cached entries (default: 5 minutes)
	CacheTTL time.Duration

	// ErrorTTL is the time-to-live for cached errors (404s, network errors, etc.)
	ErrorTTL time.Duration

	// HTTPClient allows custom HTTP client configuration
	HTTPClient *http.Client

	

	// BaseHost is used as a base configuration when using FlagsBase.
	// The host-specific configuration is merged on top of the BaseHost configuration.
	BaseHost string

	// DefaultUser is used when no user is specified
	DefaultUser string

	// GetFlagsURL allows custom logic to extract flag path from context
	GetFlagsURL func(c echo.Context, host string) string

	// GetUserFunc allows custom logic to extract user from context
	GetUserFunc func(c echo.Context) string
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

	if config.GetFlagsURL == nil {
		config.GetFlagsURL = func(c echo.Context, host string) string {
			if config.FlagsURL != "" {
				// Single file mode - always use the same file
				return config.FlagsURL
			}

			if host == "" {
				host = ContextHost(c)
			}
			return fmt.Sprintf("%s/%s.json", config.FlagsBase, host)
		}
	}

	if config.GetUserFunc == nil {
		config.GetUserFunc = func(c echo.Context) string {
			if user, ok := c.Get("user").(string); ok {
				return user
			}
			return config.DefaultUser
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
	})
}

// fetchHostConfig fetches the host configuration from HTTP
func (s *SDK) fetchHostConfig(ctx context.Context, url string) (HostConfig, error) {
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
func (s *SDK) getHostConfig(c echo.Context, host string) (HostConfig, error) {
	flagsURL := s.config.GetFlagsURL(c, host)
	if s.config.DisableCache {
		return s.fetchHostConfig(c.Request().Context(), flagsURL)
	}

	// Check cache
	s.cache.mu.RLock()
	if entry, exists := s.cache.entries[flagsURL]; exists {
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
	config, err := s.fetchHostConfig(c.Request().Context(), flagsURL)

	// Update cache with either success or error
	s.cache.mu.Lock()
	if err != nil {
		// Cache the error for ErrorTTL duration
		s.cache.entries[flagsURL] = &cacheEntry{
			err:       err,
			expiresAt: time.Now().Add(s.config.ErrorTTL),
		}
		s.cache.mu.Unlock()
		return nil, err
	}

	// Cache successful response for CacheTTL duration
	s.cache.entries[flagsURL] = &cacheEntry{
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

	host := ContextHost(c)

	if s.config.FlagsURL != "" {
		// Single file mode
		config, err := s.getHostConfig(c, host) // host is ignored here
		if err != nil {
			return nil, err
		}
		return lookupValueInConfig(config, key, s.config.GetUserFunc(c))
	}

	// Multi-host mode
	var baseConfig HostConfig
	if s.config.BaseHost != "" {
		baseConfig, _ = s.getHostConfig(c, s.config.BaseHost)
	}

	if host == "" {
		if baseConfig == nil {
			return nil, fmt.Errorf("no flag configuration could be loaded")
		}
		return lookupValueInConfig(baseConfig, key, s.config.GetUserFunc(c))
	}

	if host == s.config.BaseHost {
		if baseConfig == nil {
			return nil, fmt.Errorf("no flag configuration could be loaded for host: %s", host)
		}
		return lookupValueInConfig(baseConfig, key, s.config.GetUserFunc(c))
	}

	hostConfig, err := s.getHostConfig(c, host)
	if err != nil {
		if baseConfig != nil {
			return lookupValueInConfig(baseConfig, key, s.config.GetUserFunc(c))
		}
		return nil, err
	}

	finalConfig := mergeHostConfig(baseConfig, hostConfig)
	return lookupValueInConfig(finalConfig, key, s.config.GetUserFunc(c))
}

func lookupValueInConfig(config HostConfig, key, user string) (interface{}, error) {
	if config == nil {
		return nil, fmt.Errorf("key %s not found", key)
	}
	parts := strings.Split(key, ".")
	rootKey := parts[0]

	var value interface{}
	if wildcardConfig, exists := config["*"]; exists {
		if v, ok := wildcardConfig[rootKey]; ok {
			value = v
		}
	}

	if user != "" {
		if userConfig, exists := config[user]; exists {
			if v, ok := userConfig[rootKey]; ok {
				value = v
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

// mergeMaps recursively merges two maps. Values in override map take precedence.
func mergeMaps(base, override map[string]interface{}) map[string]interface{} {
	if base == nil {
		return override
	}
	if override == nil {
		return base
	}
	merged := make(map[string]interface{})
	for k, v := range base {
		merged[k] = v
	}
	for k, v := range override {
		if baseV, ok := base[k]; ok {
			if baseMap, ok1 := baseV.(map[string]interface{}); ok1 {
				if overrideMap, ok2 := v.(map[string]interface{}); ok2 {
					v = mergeMaps(baseMap, overrideMap)
				}
			}
		}
		merged[k] = v
	}
	return merged
}

// mergeHostConfig merges two HostConfig maps. The override config takes precedence.
func mergeHostConfig(base, override HostConfig) HostConfig {
	if base == nil {
		return override
	}
	if override == nil {
		return base
	}
	merged := make(HostConfig)
	for user, flags := range base {
		merged[user] = flags
	}
	for user, overrideFlags := range override {
		if baseFlags, ok := merged[user]; ok {
			merged[user] = mergeMaps(baseFlags, overrideFlags)
		} else {
			merged[user] = overrideFlags
		}
	}
	return merged
}


// ClearCache clears all cached entries
func (s *SDK) ClearCache() {
	s.cache.mu.Lock()
	defer s.cache.mu.Unlock()
	s.cache.entries = make(map[string]*cacheEntry)
}

// ClearCacheKey clears cache for a specific key
func (s *SDK) ClearCacheKey(key string) {
	s.cache.mu.Lock()
	defer s.cache.mu.Unlock()
	delete(s.cache.entries, key)
}

// EnsureLoaded ensures that at least one successful fetch has occurred for the host.
// In single-file mode (FlagsURL set), it performs one fetch for the static file.
// In multihost mode, it performs a synchronous fetch for the primary host.
// Returns error if fetch fails, nil if it succeeds.
func (s *SDK) EnsureLoaded(c echo.Context) error {
	_, err := s.getHostConfig(c, "")
	return err
}
