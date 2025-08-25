package echoflags

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockServer creates a test HTTP server that serves host JSON files
func mockServer(*testing.T) *httptest.Server {
	mux := http.NewServeMux()

	// Host1 configuration
	host1Config := HostConfig{
		"*": {
			"feature1":       true,
			"feature2":       false,
			"maxItems":       100,
			"discount":       0.1,
			"allowedRegions": []string{"us-east", "us-west"},
			"metadata": map[string]interface{}{
				"version": "1.0",
				"tier":    "standard",
				"features": map[string]interface{}{
					"new_dashboard": true,
					"beta_access":   false,
				},
			},
		},
		"user@example.com": {
			"feature2":       true,
			"maxItems":       200,
			"discount":       0.2,
			"allowedRegions": []string{"us-east", "us-west", "eu-west"},
		},
	}

	// Host2 configuration
	host2Config := HostConfig{
		"*": {
			"feature1": false,
			"feature3": true,
			"limit":    50,
		},
		"admin@company.com": {
			"feature1": true,
			"limit":    1000,
		},
	}

	// Base config for merge testing
	baseForMerge := HostConfig{
		"*": {
			"fallbackKey":     true,
			"feature1":        false, // This should be overridden by tenant1
			"allowedRegions":  []string{"ap-south-1"},
			"metadata": map[string]interface{}{
				"source":  "base",
				"version": "0.5-base", // overridden by tenant1
			},
		},
		"user@example.com": {
			"maxItems": 50, // overridden by tenant1
		},
		"base-user@example.com": {
			"fromBase": true,
		},
	}

	mux.HandleFunc("/host1.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(host1Config)
	})

	mux.HandleFunc("/host2.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(host2Config)
	})

	mux.HandleFunc("/baseForMerge.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(baseForMerge)
	})

	// Serve actual files from examples
	mux.HandleFunc("/tenant1.json", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "examples/hosts/tenant1.json")
	})
	mux.HandleFunc("/default-host.json", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "examples/hosts/default-host.json")
	})
	mux.HandleFunc("/base-host.json", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "examples/hosts/fallback-host.json")
	})

	mux.HandleFunc("/invalid.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("invalid json"))
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Request: %s %s\n", r.Method, r.URL)
		w.WriteHeader(http.StatusNotFound)
	})

	return httptest.NewServer(mux)
}

func TestNewWithConfig(t *testing.T) {
	t.Run("creates SDK with default config", func(t *testing.T) {
		config := Config{
			FlagsBase: "https://example.com",
		}
		sdk := NewWithConfig(config)

		assert.NotNil(t, sdk)
		assert.NotNil(t, sdk.config.HTTPClient)
		assert.Equal(t, 5*time.Minute, sdk.config.CacheTTL)
		assert.NotNil(t, sdk.config.GetUserFunc)
	})

	t.Run("creates SDK with custom config", func(t *testing.T) {
		client := &http.Client{Timeout: 10 * time.Second}
		customGetUser := func(c echo.Context) string { return "custom" }
		config := Config{
			FlagsBase:    "https://example.com",
			DisableCache: false,
			CacheTTL:     10 * time.Minute,
			HTTPClient:   client,
			GetUserFunc:  customGetUser,
		}
		sdk := NewWithConfig(config)

		assert.NotNil(t, sdk)
		assert.Equal(t, client, sdk.config.HTTPClient)
		assert.Equal(t, 10*time.Minute, sdk.config.CacheTTL)
		assert.False(t, sdk.config.DisableCache)
		assert.NotNil(t, sdk.config.GetUserFunc)
	})
}

func TestNewSingleFile(t *testing.T) {
	// Serve the example flags.json file
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/flags.json" {
			http.ServeFile(w, r, "examples/flags.json")
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	t.Run("single file mode ignores request host", func(t *testing.T) {
		sdk := New(server.URL + "/flags.json")

		e := echo.New()

		// Different request hosts should all use the same static file
		req1 := httptest.NewRequest(http.MethodGet, "http://host1/", nil)
		rec1 := httptest.NewRecorder()
		c1 := e.NewContext(req1, rec1)

		req2 := httptest.NewRequest(http.MethodGet, "http://host2/", nil)
		rec2 := httptest.NewRecorder()
		c2 := e.NewContext(req2, rec2)

		// Both should get the same values from flags.json
		val1, err1 := sdk.GetBool(c1, "enableNewFeature")
		require.NoError(t, err1)
		assert.True(t, val1)

		val2, err2 := sdk.GetBool(c2, "enableNewFeature")
		require.NoError(t, err2)
		assert.True(t, val2)
	})

	t.Run("single file mode with user overrides", func(t *testing.T) {
		sdk := New(server.URL + "/flags.json")

		e := echo.New()

		// Test wildcard value
		req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		maxConn, err := sdk.GetInt(c, "maxConnections")
		require.NoError(t, err)
		assert.Equal(t, 100, maxConn)

		// Test admin user override
		c.Set("user", "admin@example.com")
		maxConn, err = sdk.GetInt(c, "maxConnections")
		require.NoError(t, err)
		assert.Equal(t, 500, maxConn)

		// Test beta user with nested config
		c.Set("user", "beta-user@example.com")
		version, err := sdk.GetString(c, "apiConfig.version")
		require.NoError(t, err)
		assert.Equal(t, "2.1-beta", version)
	})
}

func TestGetString(t *testing.T) {
	server := mockServer(t)
	defer server.Close()

	sdk := NewWithConfig(Config{
		FlagsBase:    server.URL,
		DisableCache: false,
	})

	e := echo.New()

	t.Run("gets wildcard value when no user", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://host1/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		value, err := sdk.GetString(c, "metadata")
		require.NoError(t, err)
		assert.Contains(t, value, "version")
	})

	t.Run("gets user override value", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://host1/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.Set("user", "user@example.com")

		value, err := sdk.GetString(c, "discount")
		require.NoError(t, err)
		assert.Equal(t, "0.2", value)
	})

	t.Run("returns error for missing key", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://host1/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		_, err := sdk.GetString(c, "nonexistent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestGetBool(t *testing.T) {
	server := mockServer(t)
	defer server.Close()

	sdk := NewWithConfig(Config{
		FlagsBase:    server.URL,
		DisableCache: false,
	})

	e := echo.New()

	t.Run("gets boolean value", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://host1/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		value, err := sdk.GetBool(c, "feature1")
		require.NoError(t, err)
		assert.True(t, value)

		value, err = sdk.GetBool(c, "feature2")
		require.NoError(t, err)
		assert.False(t, value)
	})

	t.Run("gets user override boolean", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://host1/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.Set("user", "user@example.com")

		value, err := sdk.GetBool(c, "feature2")
		require.NoError(t, err)
		assert.True(t, value) // Override from false to true
	})
}

func TestGetInt(t *testing.T) {
	server := mockServer(t)
	defer server.Close()

	sdk := NewWithConfig(Config{
		FlagsBase:    server.URL,
		DisableCache: false,
	})

	e := echo.New()

	t.Run("gets int value", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://host1/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		value, err := sdk.GetInt(c, "maxItems")
		require.NoError(t, err)
		assert.Equal(t, 100, value)
	})

	t.Run("gets user override int", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://host1/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.Set("user", "user@example.com")

		value, err := sdk.GetInt(c, "maxItems")
		require.NoError(t, err)
		assert.Equal(t, 200, value)
	})
}

func TestGetFloat64(t *testing.T) {
	server := mockServer(t)
	defer server.Close()

	sdk := NewWithConfig(Config{
		FlagsBase:    server.URL,
		DisableCache: false,
	})

	e := echo.New()

	t.Run("gets float value", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://host1/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		value, err := sdk.GetFloat64(c, "discount")
		require.NoError(t, err)
		assert.Equal(t, 0.1, value)
	})

	t.Run("gets user override float", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://host1/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.Set("user", "user@example.com")

		value, err := sdk.GetFloat64(c, "discount")
		require.NoError(t, err)
		assert.Equal(t, 0.2, value)
	})
}

func TestGetStringSlice(t *testing.T) {
	server := mockServer(t)
	defer server.Close()

	sdk := NewWithConfig(Config{
		FlagsBase:    server.URL,
		DisableCache: false,
	})

	e := echo.New()

	t.Run("gets string slice value", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://host1/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		value, err := sdk.GetStringSlice(c, "allowedRegions")
		require.NoError(t, err)
		assert.Equal(t, []string{"us-east", "us-west"}, value)
	})

	t.Run("gets user override string slice", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://host1/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.Set("user", "user@example.com")

		value, err := sdk.GetStringSlice(c, "allowedRegions")
		require.NoError(t, err)
		assert.Equal(t, []string{"us-east", "us-west", "eu-west"}, value)
	})
}

func TestGetMap(t *testing.T) {
	server := mockServer(t)
	defer server.Close()

	sdk := NewWithConfig(Config{
		FlagsBase:    server.URL,
		DisableCache: false,
	})

	e := echo.New()

	t.Run("gets map value", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://host1/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		value, err := sdk.GetMap(c, "metadata")
		require.NoError(t, err)
		assert.Equal(t, "1.0", value["version"])
		assert.Equal(t, "standard", value["tier"])
	})
}

func TestIsEnabled(t *testing.T) {
	server := mockServer(t)
	defer server.Close()

	sdk := NewWithConfig(Config{
		FlagsBase:    server.URL,
		DisableCache: false,
	})

	e := echo.New()

	t.Run("checks if feature is enabled", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://host1/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		assert.True(t, sdk.IsEnabled(c, "feature1"))
		assert.False(t, sdk.IsEnabled(c, "feature2"))
		assert.False(t, sdk.IsEnabled(c, "nonexistent"))
	})
}

func TestCaching(t *testing.T) {
	server := mockServer(t)
	defer server.Close()

	sdk := NewWithConfig(Config{
		FlagsBase:    server.URL,
		DisableCache: false,
		CacheTTL:     100 * time.Millisecond,
	})

	e := echo.New()

	t.Run("caches host config", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://host1/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		// First call - fetches from server
		value1, err := sdk.GetBool(c, "feature1")
		require.NoError(t, err)
		assert.True(t, value1)

		// Second call - should use cache
		value2, err := sdk.GetBool(c, "feature1")
		require.NoError(t, err)
		assert.True(t, value2)

		// Verify cache has entry
		sdk.cache.mu.RLock()
		assert.NotEmpty(t, sdk.cache.entries)
		sdk.cache.mu.RUnlock()
	})

	t.Run("cache expires", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://host2/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		// First call
		value1, err := sdk.GetBool(c, "feature3")
		require.NoError(t, err)
		assert.True(t, value1)

		// Wait for cache to expire
		time.Sleep(150 * time.Millisecond)

		// Should fetch again
		value2, err := sdk.GetBool(c, "feature3")
		require.NoError(t, err)
		assert.True(t, value2)
	})

	t.Run("clear cache", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://host1/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		// Populate cache
		_, err := sdk.GetBool(c, "feature1")
		require.NoError(t, err)

		// Clear all cache
		sdk.ClearCache()

		// Verify cache is empty
		sdk.cache.mu.RLock()
		count := len(sdk.cache.entries)
		sdk.cache.mu.RUnlock()
		assert.Equal(t, 0, count)
	})

	t.Run("clear host cache", func(t *testing.T) {
		// Populate cache for multiple tenants
		req1 := httptest.NewRequest(http.MethodGet, "http://host1/", nil)
		rec1 := httptest.NewRecorder()
		c1 := e.NewContext(req1, rec1)
		_, err := sdk.GetBool(c1, "feature1")
		require.NoError(t, err)

		req2 := httptest.NewRequest(http.MethodGet, "http://host2/", nil)
		rec2 := httptest.NewRecorder()
		c2 := e.NewContext(req2, rec2)
		_, err = sdk.GetBool(c2, "feature3")
		require.NoError(t, err)

		assert.NotEmpty(t, sdk.cache.entries)

		// Clear only tenant1 cache
		sdk.ClearCache()

		assert.Empty(t, sdk.cache.entries)
	})
}

func TestNoHost(t *testing.T) {
	server := mockServer(t)
	defer server.Close()

	e := echo.New()

	t.Run("uses base host when no host specified", func(t *testing.T) {
		sdk := NewWithConfig(Config{
			FlagsBase:    server.URL,
			BaseHost:     "host1",
			DisableCache: false,
		})
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Host = ""
		c := e.NewContext(req, httptest.NewRecorder())

		value, err := sdk.GetBool(c, "feature1") // feature1 is in host1
		require.NoError(t, err)
		assert.True(t, value)
	})

	t.Run("fails when no host and no base host", func(t *testing.T) {
		sdk := NewWithConfig(Config{
			FlagsBase:    server.URL,
			DisableCache: true,
		})
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Host = ""
		c := e.NewContext(req, httptest.NewRecorder())

		_, err := sdk.GetBool(c, "feature1")
		assert.Error(t, err)
	})
}

func TestErrorHandling(t *testing.T) {
	server := mockServer(t)
	defer server.Close()

	sdk := NewWithConfig(Config{
		FlagsBase:    server.URL,
		DisableCache: false,
	})

	e := echo.New()

	t.Run("handles missing host", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://nonexistent/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		_, err := sdk.GetBool(c, "feature1")
		assert.Error(t, err)
	})

	t.Run("handles invalid JSON", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://invalid/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		_, err := sdk.GetBool(c, "feature1")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unmarshaling")
	})

	t.Run("handles no host specified", func(t *testing.T) {
		sdkNoDefault := NewWithConfig(Config{
			FlagsBase:    server.URL,
			DisableCache: true,
		})

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		_, err := sdkNoDefault.GetBool(c, "feature1")
		assert.Error(t, err)
		// Since we're now extracting tenant from host and there's no host,
		// it will try to fetch from an empty tenant name, resulting in a 404
		assert.Contains(t, err.Error(), "404")
	})
}

func TestContextCancellation(t *testing.T) {
	// Create a slow server
	slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(1 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer slowServer.Close()

	sdk := NewWithConfig(Config{
		FlagsBase:    slowServer.URL,
		DisableCache: false,
	})

	e := echo.New()

	t.Run("respects context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		req := httptest.NewRequest(http.MethodGet, "http://tenant1/", nil).WithContext(ctx)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		_, err := sdk.GetBool(c, "feature1")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "context")
	})
}

func TestCustomGetUserFromContext(t *testing.T) {
	server := mockServer(t)
	defer server.Close()

	sdk := NewWithConfig(Config{
		FlagsBase:    server.URL,
		DisableCache: false,
		GetUserFunc: func(c echo.Context) string {
			// Custom logic: get user from header
			return c.Request().Header.Get("X-User")
		},
	})

	e := echo.New()

	t.Run("gets user from custom function", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://tenant1/", nil)
		req.Header.Set("X-User", "user@example.com")
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		// This should use the user override for user@example.com
		value, err := sdk.GetInt(c, "maxItems")
		require.NoError(t, err)
		assert.Equal(t, 150, value)
	})

	t.Run("no user from custom function", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://tenant1/", nil)
		// No X-User header
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		// This should use the wildcard value
		value, err := sdk.GetInt(c, "maxItems")
		require.NoError(t, err)
		assert.Equal(t, 100, value)
	})
}

func TestGettersWithDefault(t *testing.T) {
	server := mockServer(t)
	defer server.Close()

	sdk := NewWithConfig(Config{
		FlagsBase:   server.URL,
		BaseHost: "host1",
	})

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "http://host1/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	t.Run("GetStringWithDefault", func(t *testing.T) {
		// Existing key
		value := sdk.GetStringWithDefault(c, "maxItems", "default")
		assert.Equal(t, "100", value)

		// Non-existing key
		value = sdk.GetStringWithDefault(c, "nonexistent", "default-value")
		assert.Equal(t, "default-value", value)
	})

	t.Run("GetBoolWithDefault", func(t *testing.T) {
		// Existing key
		value := sdk.GetBoolWithDefault(c, "feature1", false)
		assert.True(t, value)

		// Non-existing key
		value = sdk.GetBoolWithDefault(c, "nonexistent", true)
		assert.True(t, value)
		value = sdk.GetBoolWithDefault(c, "nonexistent", false)
		assert.False(t, value)
	})

	t.Run("GetIntWithDefault", func(t *testing.T) {
		// Existing key
		value := sdk.GetIntWithDefault(c, "maxItems", 500)
		assert.Equal(t, 100, value)

		// Non-existing key
		value = sdk.GetIntWithDefault(c, "nonexistent", 999)
		assert.Equal(t, 999, value)
	})

	t.Run("GetFloat64WithDefault", func(t *testing.T) {
		// Existing key
		value := sdk.GetFloat64WithDefault(c, "discount", 0.5)
		assert.Equal(t, 0.1, value)

		// Non-existing key
		value = sdk.GetFloat64WithDefault(c, "nonexistent", 0.99)
		assert.Equal(t, 0.99, value)
	})

	t.Run("GetStringSliceWithDefault", func(t *testing.T) {
		// Existing key
		value := sdk.GetStringSliceWithDefault(c, "allowedRegions", []string{"default"})
		assert.Equal(t, []string{"us-east", "us-west"}, value)

		// Non-existing key
		value = sdk.GetStringSliceWithDefault(c, "nonexistent", []string{"default-slice"})
		assert.Equal(t, []string{"default-slice"}, value)
	})

	t.Run("GetMapWithDefault", func(t *testing.T) {
		// Existing key
		value := sdk.GetMapWithDefault(c, "metadata", map[string]interface{}{"a": "b"})
		assert.Equal(t, "1.0", value["version"])

		// Non-existing key
		value = sdk.GetMapWithDefault(c, "nonexistent", map[string]interface{}{"default": "map"})
		assert.Equal(t, map[string]interface{}{"default": "map"}, value)
	})
}

func TestGetBoolWithNestedPaths(t *testing.T) {
	server := mockServer(t)
	defer server.Close()

	sdk := NewWithConfig(Config{
		FlagsBase:    server.URL,
		DisableCache: false,
	})

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "http://host1/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	t.Run("gets bool from nested map", func(t *testing.T) {
		value, err := sdk.GetBool(c, "metadata.features.new_dashboard")
		require.NoError(t, err)
		assert.True(t, value)

		value, err = sdk.GetBool(c, "metadata.features.beta_access")
		require.NoError(t, err)
		assert.False(t, value)
	})

	t.Run("gets bool from top level", func(t *testing.T) {
		value, err := sdk.GetBool(c, "feature1")
		require.NoError(t, err)
		assert.True(t, value)
	})

	t.Run("path not found", func(t *testing.T) {
		_, err := sdk.GetBool(c, "metadata.features.nonexistent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "key not found at path 'metadata.features.nonexistent'")
	})

	t.Run("intermediate path not a map", func(t *testing.T) {
		_, err := sdk.GetBool(c, "feature1.subfeature")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "value at path 'feature1' is not a map")
	})

	t.Run("empty path", func(t *testing.T) {
		_, err := sdk.GetBool(c, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "key cannot be empty")
	})
}

func TestErrorCaching(t *testing.T) {
	// Create a server that returns 404 for unknown tenants
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "nonexistent") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Server error"))
	}))
	defer server.Close()

	sdk := NewWithConfig(Config{
		FlagsBase:    server.URL,
		DisableCache: false,
		ErrorTTL:     100 * time.Millisecond,
	})

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "http://nonexistent/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	t.Run("caches 404 errors", func(t *testing.T) {
		start := time.Now()

		// First call should hit the server and cache the error
		_, err := sdk.GetString(c, "somekey")
		assert.Error(t, err)
		firstCallDuration := time.Since(start)

		start = time.Now()

		// Second call should return cached error (much faster)
		_, err = sdk.GetString(c, "somekey")
		assert.Error(t, err)
		secondCallDuration := time.Since(start)

		// Second call should be significantly faster (cached)
		assert.True(t, secondCallDuration < firstCallDuration/2,
			"Second call should be much faster due to caching")
	})

	t.Run("error cache expires", func(t *testing.T) {
		// Make a call to cache the error
		_, err := sdk.GetString(c, "somekey")
		assert.Error(t, err)

		// Wait for error cache to expire
		time.Sleep(150 * time.Millisecond)

		// This should hit the server again
		_, err = sdk.GetString(c, "somekey")
		assert.Error(t, err)
	})

	t.Run("caches server errors", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://servererror/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		start := time.Now()

		// First call should hit the server and cache the error
		_, err := sdk.GetString(c, "somekey")
		assert.Error(t, err)
		firstCallDuration := time.Since(start)

		start = time.Now()

		// Second call should return cached error
		_, err = sdk.GetString(c, "somekey")
		assert.Error(t, err)
		secondCallDuration := time.Since(start)

		// Second call should be faster (cached)
		assert.True(t, secondCallDuration < firstCallDuration/2)
	})
}

func TestEnsureLoaded(t *testing.T) {
	server := mockServer(t)
	defer server.Close()

	e := echo.New()

	t.Run("single-file mode ensures static file loaded", func(t *testing.T) {
		// Create a server that serves the example flags.json
		exampleServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/flags.json" {
				http.ServeFile(w, r, "examples/flags.json")
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer exampleServer.Close()

		sdk := New(exampleServer.URL + "/flags.json")

		req := httptest.NewRequest(http.MethodGet, "http://anydomain/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := sdk.EnsureLoaded(c)
		require.NoError(t, err)

		// Verify cache has entry for static file
		sdk.cache.mu.RLock()
		_, exists := sdk.cache.entries[exampleServer.URL+"/flags.json"]
		sdk.cache.mu.RUnlock()
		assert.True(t, exists)

		// Verify we can now get values from the loaded config
		enabled, err := sdk.GetBool(c, "enableNewFeature")
		require.NoError(t, err)
		assert.True(t, enabled)
	})

	t.Run("single-file mode handles error", func(t *testing.T) {
		sdk := New(server.URL + "/nonexistent.json")

		req := httptest.NewRequest(http.MethodGet, "http://anydomain/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := sdk.EnsureLoaded(c)
		assert.Error(t, err)
	})

	t.Run("multihost mode loads primary tenant", func(t *testing.T) {
		sdk := NewWithConfig(Config{
			FlagsBase:    server.URL,
			DisableCache: false,
		})

		req := httptest.NewRequest(http.MethodGet, "http://tenant1/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := sdk.EnsureLoaded(c)
		require.NoError(t, err)

		// Verify cache has entry for tenant1
		sdk.cache.mu.RLock()
		_, exists := sdk.cache.entries[server.URL+"/tenant1.json"]
		sdk.cache.mu.RUnlock()
		assert.True(t, exists)
	})

	t.Run("multihost mode retrieves feature3 from tenant1", func(t *testing.T) {
		sdk := NewWithConfig(Config{
			FlagsBase:    server.URL,
			DisableCache: false,
		})

		req := httptest.NewRequest(http.MethodGet, "http://tenant1/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		// Test that feature3 is true as defined in tenant1.json
		value, err := sdk.GetBool(c, "feature3")
		require.NoError(t, err)
		assert.True(t, value)

		// Verify cache has entry for tenant1
		sdk.cache.mu.RLock()
		_, exists := sdk.cache.entries[server.URL+"/tenant1.json"]
		sdk.cache.mu.RUnlock()
		assert.True(t, exists)
	})

	t.Run("multihost mode - primary fails", func(t *testing.T) {
		sdk := NewWithConfig(Config{
			FlagsBase:    server.URL,
			DisableCache: false,
		})

		req := httptest.NewRequest(http.MethodGet, "http://nonexistent/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := sdk.EnsureLoaded(c)
		assert.Error(t, err)
	})

	

	t.Run("multihost mode no host specified", func(t *testing.T) {
		sdk := NewWithConfig(Config{
			FlagsBase:    server.URL,
			DisableCache: false,
		})

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := sdk.EnsureLoaded(c)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected status code")
	})
}

func TestMergingLogic(t *testing.T) {
	server := mockServer(t)
	defer server.Close()

	e := echo.New()

	t.Run("no base, no problem", func(t *testing.T) {
		sdk := NewWithConfig(Config{
			FlagsBase:    server.URL,
			DisableCache: true,
		})
		req := httptest.NewRequest(http.MethodGet, "http://tenant1/", nil)
		c := e.NewContext(req, httptest.NewRecorder())

		val, err := sdk.GetBool(c, "feature1")
		require.NoError(t, err)
		assert.True(t, val)
	})

	t.Run("base only when host is missing", func(t *testing.T) {
		sdk := NewWithConfig(Config{
			FlagsBase:    server.URL,
			BaseHost:     "base-host",
			DisableCache: true,
		})
		req := httptest.NewRequest(http.MethodGet, "http://nonexistent/", nil)
		c := e.NewContext(req, httptest.NewRecorder())

		val, err := sdk.GetBool(c, "fallbackHost")
		require.NoError(t, err)
		assert.True(t, val)

		_, err = sdk.GetBool(c, "feature1")
		assert.Error(t, err, "feature1 should not exist in base")
	})

	t.Run("host values override base values", func(t *testing.T) {
		sdk := NewWithConfig(Config{
			FlagsBase:    server.URL,
			BaseHost:     "baseForMerge",
			DisableCache: true,
		})
		req := httptest.NewRequest(http.MethodGet, "http://tenant1/", nil)
		c := e.NewContext(req, httptest.NewRecorder())

		// This value is true in tenant1, but false in baseForMerge
		val, err := sdk.GetBool(c, "feature1")
		require.NoError(t, err)
		assert.True(t, val, "tenant1 value for feature1 should take precedence")
	})

	t.Run("values are merged from host and base", func(t *testing.T) {
		sdk := NewWithConfig(Config{
			FlagsBase:    server.URL,
			BaseHost:     "baseForMerge",
			DisableCache: true,
		})
		req := httptest.NewRequest(http.MethodGet, "http://tenant1/", nil)
		c := e.NewContext(req, httptest.NewRecorder())

		// This value only exists in the base
		fbVal, err := sdk.GetBool(c, "fallbackKey")
		require.NoError(t, err)
		assert.True(t, fbVal)

		// This value only exists in the host config
		feature2, err := sdk.GetBool(c, "feature2")
		require.NoError(t, err)
		assert.False(t, feature2)
	})

	t.Run("nested maps are merged", func(t *testing.T) {
		sdk := NewWithConfig(Config{
			FlagsBase:    server.URL,
			BaseHost:     "baseForMerge",
			DisableCache: true,
		})
		req := httptest.NewRequest(http.MethodGet, "http://tenant1/", nil)
		c := e.NewContext(req, httptest.NewRecorder())

		meta, err := sdk.GetMap(c, "metadata")
		require.NoError(t, err)

		// From base
		assert.Equal(t, "base", meta["source"])
		// From tenant1 (override)
		assert.Equal(t, "1.0.0", meta["version"])
		// From tenant1
		assert.Equal(t, "standard", meta["tier"])
	})

	t.Run("arrays are replaced, not merged", func(t *testing.T) {
		sdk := NewWithConfig(Config{
			FlagsBase:    server.URL,
			BaseHost:     "baseForMerge",
			DisableCache: true,
		})
		req := httptest.NewRequest(http.MethodGet, "http://tenant1/", nil)
		c := e.NewContext(req, httptest.NewRecorder())

		// baseForMerge has ["ap-south-1"], tenant1 has ["us-east-1", "us-west-2"]
		regions, err := sdk.GetStringSlice(c, "allowedRegions")
		require.NoError(t, err)
		assert.Equal(t, []string{"us-east-1", "us-west-2"}, regions, "host array should replace base array")
	})

	t.Run("user-specific values are merged correctly", func(t *testing.T) {
		sdk := NewWithConfig(Config{
			FlagsBase:    server.URL,
			BaseHost:     "baseForMerge",
			DisableCache: true,
		})
		req := httptest.NewRequest(http.MethodGet, "http://tenant1/", nil)
		c := e.NewContext(req, httptest.NewRecorder())
		c.Set("user", "user@example.com")

		// From tenant1 user-specific (150) overrides base user-specific (50)
		maxItems, err := sdk.GetInt(c, "maxItems")
		require.NoError(t, err)
		assert.Equal(t, 150, maxItems)

		// This is a wildcard value from tenant1, not defined for the user
		feature2, err := sdk.GetBool(c, "feature2")
		require.NoError(t, err)
		assert.True(t, feature2)
	})

	t.Run("user exists only in base", func(t *testing.T) {
		sdk := NewWithConfig(Config{
			FlagsBase:    server.URL,
			BaseHost:     "baseForMerge",
			DisableCache: true,
		})
		req := httptest.NewRequest(http.MethodGet, "http://tenant1/", nil)
		c := e.NewContext(req, httptest.NewRecorder())
		c.Set("user", "base-user@example.com")

		// This value comes from the user-specific block in the base
		fromFb, err := sdk.GetBool(c, "fromBase")
		require.NoError(t, err)
		assert.True(t, fromFb)

		// This value comes from the wildcard block in tenant1
		feature1, err := sdk.GetBool(c, "feature1")
		require.NoError(t, err)
		assert.True(t, feature1)
	})
}