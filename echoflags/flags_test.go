package echoflags

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockServer creates a test HTTP server that serves tenant JSON files
func mockServer(t *testing.T) *httptest.Server {
	mux := http.NewServeMux()

	// Tenant1 configuration
	tenant1Config := TenantConfig{
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

	// Tenant2 configuration
	tenant2Config := TenantConfig{
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

	mux.HandleFunc("/tenant1.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tenant1Config)
	})

	mux.HandleFunc("/tenant2.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tenant2Config)
	})

	mux.HandleFunc("/invalid.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("invalid json"))
	})

	return httptest.NewServer(mux)
}

func TestNew(t *testing.T) {
	t.Run("creates SDK with default config", func(t *testing.T) {
		config := Config{
			HTTPBaseURL: "https://example.com",
		}
		sdk := New(config)

		assert.NotNil(t, sdk)
		assert.NotNil(t, sdk.config.HTTPClient)
		assert.Equal(t, 5*time.Minute, sdk.config.CacheTTL)
		assert.NotNil(t, sdk.config.GetUserFromContext)
	})

	t.Run("creates SDK with custom config", func(t *testing.T) {
		client := &http.Client{Timeout: 10 * time.Second}
		customGetUser := func(c echo.Context) string { return "custom" }
		config := Config{
			HTTPBaseURL:        "https://example.com",
			DisableCache:       false,
			CacheTTL:           10 * time.Minute,
			HTTPClient:         client,
			GetUserFromContext: customGetUser,
		}
		sdk := New(config)

		assert.NotNil(t, sdk)
		assert.Equal(t, client, sdk.config.HTTPClient)
		assert.Equal(t, 10*time.Minute, sdk.config.CacheTTL)
		assert.False(t, sdk.config.DisableCache)
		assert.NotNil(t, sdk.config.GetUserFromContext)
	})
}

func TestGetString(t *testing.T) {
	server := mockServer(t)
	defer server.Close()

	sdk := New(Config{
		HTTPBaseURL:  server.URL,
		DisableCache: false,
	})

	e := echo.New()

	t.Run("gets wildcard value when no user", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://tenant1/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		value, err := sdk.GetString(c, "metadata")
		require.NoError(t, err)
		assert.Contains(t, value, "version")
	})

	t.Run("gets user override value", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://tenant1/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.Set("user", "user@example.com")

		value, err := sdk.GetString(c, "discount")
		require.NoError(t, err)
		assert.Equal(t, "0.2", value)
	})

	t.Run("returns error for missing key", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://tenant1/", nil)
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

	sdk := New(Config{
		HTTPBaseURL:  server.URL,
		DisableCache: false,
	})

	e := echo.New()

	t.Run("gets boolean value", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://tenant1/", nil)
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
		req := httptest.NewRequest(http.MethodGet, "http://tenant1/", nil)
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

	sdk := New(Config{
		HTTPBaseURL:  server.URL,
		DisableCache: false,
	})

	e := echo.New()

	t.Run("gets int value", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://tenant1/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		value, err := sdk.GetInt(c, "maxItems")
		require.NoError(t, err)
		assert.Equal(t, 100, value)
	})

	t.Run("gets user override int", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://tenant1/", nil)
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

	sdk := New(Config{
		HTTPBaseURL:  server.URL,
		DisableCache: false,
	})

	e := echo.New()

	t.Run("gets float value", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://tenant1/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		value, err := sdk.GetFloat64(c, "discount")
		require.NoError(t, err)
		assert.Equal(t, 0.1, value)
	})

	t.Run("gets user override float", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://tenant1/", nil)
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

	sdk := New(Config{
		HTTPBaseURL:  server.URL,
		DisableCache: false,
	})

	e := echo.New()

	t.Run("gets string slice value", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://tenant1/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		value, err := sdk.GetStringSlice(c, "allowedRegions")
		require.NoError(t, err)
		assert.Equal(t, []string{"us-east", "us-west"}, value)
	})

	t.Run("gets user override string slice", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://tenant1/", nil)
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

	sdk := New(Config{
		HTTPBaseURL:  server.URL,
		DisableCache: false,
	})

	e := echo.New()

	t.Run("gets map value", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://tenant1/", nil)
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

	sdk := New(Config{
		HTTPBaseURL:  server.URL,
		DisableCache: false,
	})

	e := echo.New()

	t.Run("checks if feature is enabled", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://tenant1/", nil)
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

	sdk := New(Config{
		HTTPBaseURL:  server.URL,
		DisableCache: false,
		CacheTTL:     100 * time.Millisecond,
	})

	e := echo.New()

	t.Run("caches tenant config", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://tenant1/", nil)
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
		_, exists := sdk.cache.entries["tenant1"]
		sdk.cache.mu.RUnlock()
		assert.True(t, exists)
	})

	t.Run("cache expires", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://tenant2/", nil)
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
		req := httptest.NewRequest(http.MethodGet, "http://tenant1/", nil)
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

	t.Run("clear tenant cache", func(t *testing.T) {
		// Populate cache for multiple tenants
		req1 := httptest.NewRequest(http.MethodGet, "http://tenant1/", nil)
		rec1 := httptest.NewRecorder()
		c1 := e.NewContext(req1, rec1)
		_, err := sdk.GetBool(c1, "feature1")
		require.NoError(t, err)

		req2 := httptest.NewRequest(http.MethodGet, "http://tenant2/", nil)
		rec2 := httptest.NewRecorder()
		c2 := e.NewContext(req2, rec2)
		_, err = sdk.GetBool(c2, "feature3")
		require.NoError(t, err)

		// Clear only tenant1 cache
		sdk.ClearTenantCache("tenant1")

		// Verify tenant1 is cleared but tenant2 remains
		sdk.cache.mu.RLock()
		_, exists1 := sdk.cache.entries["tenant1"]
		_, exists2 := sdk.cache.entries["tenant2"]
		sdk.cache.mu.RUnlock()

		assert.False(t, exists1)
		assert.True(t, exists2)
	})
}

func TestDefaultTenant(t *testing.T) {
	server := mockServer(t)
	defer server.Close()

	sdk := New(Config{
		HTTPBaseURL:   server.URL,
		DefaultTenant: "tenant1",
		DisableCache:  false,
		GetTenantFromContext: func(c echo.Context) string {
			// Return empty string to test default tenant fallback
			return ""
		},
	})

	e := echo.New()

	t.Run("uses default tenant when not specified", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		// Custom GetTenantFromContext returns empty, should fall back to default tenant

		value, err := sdk.GetBool(c, "feature1")
		require.NoError(t, err)
		assert.True(t, value)
	})
}

func TestErrorHandling(t *testing.T) {
	server := mockServer(t)
	defer server.Close()

	sdk := New(Config{
		HTTPBaseURL:  server.URL,
		DisableCache: false,
	})

	e := echo.New()

	t.Run("handles missing tenant", func(t *testing.T) {
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

	t.Run("handles no tenant specified", func(t *testing.T) {
		sdkNoDefault := New(Config{
			HTTPBaseURL:  server.URL,
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

	sdk := New(Config{
		HTTPBaseURL: slowServer.URL,
		DisableCache:  false,
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

	sdk := New(Config{
		HTTPBaseURL: server.URL,
		DisableCache:  false,
		GetUserFromContext: func(c echo.Context) string {
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
		assert.Equal(t, 200, value)
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

	sdk := New(Config{
		HTTPBaseURL: server.URL,
		DefaultTenant: "tenant1",
	})

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "http://tenant1/", nil)
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

func TestFallbackTenant(t *testing.T) {
	server := mockServer(t)
	defer server.Close()

	sdk := New(Config{
		HTTPBaseURL:  server.URL,
		FallbackTenant: "tenant2",
		DisableCache:   false,
	})

	e := echo.New()

	t.Run("uses fallback tenant when key not found in primary tenant", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://tenant1/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		// tenant1 doesn't have "feature3", but tenant2 does
		value, err := sdk.GetBool(c, "feature3")
		require.NoError(t, err)
		assert.True(t, value)
	})

	t.Run("primary tenant value takes precedence over fallback", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://tenant1/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		// Both tenants have "feature1", but with different values
		// tenant1: true, tenant2: false
		value, err := sdk.GetBool(c, "feature1")
		require.NoError(t, err)
		assert.True(t, value) // Should use tenant1's value
	})

	t.Run("user override in primary tenant takes precedence", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://tenant1/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.Set("user", "user@example.com")

		// user@example.com has feature2=true in tenant1, feature2 doesn't exist in tenant2
		value, err := sdk.GetBool(c, "feature2")
		require.NoError(t, err)
		assert.True(t, value)
	})

	t.Run("fallback tenant user override when key not in primary", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://tenant1/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.Set("user", "admin@company.com")

		// admin@company.com exists in tenant2 but not tenant1
		// tenant1 doesn't have "limit", but tenant2 does with user override
		value, err := sdk.GetInt(c, "limit")
		require.NoError(t, err)
		assert.Equal(t, 1000, value) // Should use tenant2's user override
	})

	t.Run("no fallback when key exists in primary tenant", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://tenant2/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		// tenant2 has feature1=false, tenant1 has feature1=true
		// Should use tenant2's value, not fallback to tenant1
		value, err := sdk.GetBool(c, "feature1")
		require.NoError(t, err)
		assert.False(t, value) // tenant2's value
	})

	t.Run("error when key not found in either tenant", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://tenant1/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		_, err := sdk.GetBool(c, "nonexistent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("no fallback when fallback tenant same as primary", func(t *testing.T) {
		sdkSameFallback := New(Config{
			HTTPBaseURL:  server.URL,
			FallbackTenant: "tenant1", // Same as primary
			DisableCache:   false,
		})

		req := httptest.NewRequest(http.MethodGet, "http://tenant1/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		// Should not use fallback since it's the same as primary
		_, err := sdkSameFallback.GetBool(c, "feature3")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("fallback works with nested paths", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://tenant2/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		sdkWithFallback := New(Config{
			HTTPBaseURL:  server.URL,
			FallbackTenant: "tenant1",
			DisableCache:   false,
		})

		value, err := sdkWithFallback.GetBool(c, "metadata.features.new_dashboard")
		require.NoError(t, err)
		assert.True(t, value)
	})
}

func TestGetBoolWithNestedPaths(t *testing.T) {
	server := mockServer(t)
	defer server.Close()

	sdk := New(Config{
		HTTPBaseURL: server.URL,
		DisableCache:  false,
	})

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "http://tenant1/", nil)
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

	sdk := New(Config{
		HTTPBaseURL: server.URL,
		DisableCache:  false,
		ErrorTTL:      100 * time.Millisecond,
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
