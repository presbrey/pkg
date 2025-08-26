package echoflags

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFluentAPI(t *testing.T) {
	server := mockServer(t)
	defer server.Close()

	sdk := NewWithConfig(Config{
		FlagsBase: server.URL,
		BaseHost:   "host1",
	})

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "http://host1/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// Create a second context with a user
	userReq := httptest.NewRequest(http.MethodGet, "http://host1/", nil)
	userRec := httptest.NewRecorder()
	userC := e.NewContext(userReq, userRec)
	userC.Set("user", "user@example.com")

	fs := sdk.WithContext(c)
	userFs := sdk.WithContext(userC)

	t.Run("GetString", func(t *testing.T) {
		val, err := fs.GetString("maxItems")
		require.NoError(t, err)
		assert.Equal(t, "100", val)

		userVal, err := userFs.GetString("maxItems")
		require.NoError(t, err)
		assert.Equal(t, "200", userVal)

		_, err = fs.GetString("nonexistent")
		assert.Error(t, err)
	})

	t.Run("GetStringWithDefault", func(t *testing.T) {
		val := fs.GetStringWithDefault("maxItems", "default")
		assert.Equal(t, "100", val)

		val = fs.GetStringWithDefault("nonexistent", "default")
		assert.Equal(t, "default", val)
	})

	t.Run("GetBool", func(t *testing.T) {
		val, err := fs.GetBool("feature1")
		require.NoError(t, err)
		assert.True(t, val)

		val, err = fs.GetBool("feature2")
		require.NoError(t, err)
		assert.False(t, val)
	})

	t.Run("GetBoolWithNestedPath", func(t *testing.T) {
		val, err := fs.GetBool("metadata.features.new_dashboard")
		require.NoError(t, err)
		assert.True(t, val)
	})

	t.Run("GetInt", func(t *testing.T) {
		val, err := fs.GetInt("maxItems")
		require.NoError(t, err)
		assert.Equal(t, 100, val)
	})

	t.Run("GetFloat64", func(t *testing.T) {
		val, err := fs.GetFloat64("discount")
		require.NoError(t, err)
		assert.Equal(t, 0.1, val)
	})

	t.Run("GetStringSlice", func(t *testing.T) {
		val, err := fs.GetStringSlice("allowedRegions")
		require.NoError(t, err)
		assert.Equal(t, []string{"us-east", "us-west"}, val)
	})

	t.Run("GetMap", func(t *testing.T) {
		val, err := fs.GetMap("metadata")
		require.NoError(t, err)
		assert.Equal(t, "1.0", val["version"])
		assert.Equal(t, "standard", val["tier"])
	})

	t.Run("IsEnabled", func(t *testing.T) {
		assert.True(t, fs.IsEnabled("feature1"))
		assert.False(t, fs.IsEnabled("feature2"))
		assert.True(t, userFs.IsEnabled("feature2"))
	})
}

func TestFluentAPIWithCustomUserKey(t *testing.T) {
	server := mockServer(t)
	defer server.Close()

	sdk := NewWithConfig(Config{
		FlagsBase:      server.URL,
		BaseHost:       "host1",
		UserContextKey: "custom_user",
	})

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "http://host1/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("custom_user", "user@example.com")

	fs := sdk.WithContext(c)

	// feature2 is false for wildcard, true for user@example.com
	assert.True(t, fs.IsEnabled("feature2"))
}

func TestFluentAPIWithDefault(t *testing.T) {
	server := mockServer(t)
	defer server.Close()

	sdk := NewWithConfig(Config{
		FlagsBase: server.URL,
		BaseHost:   "host1",
	})

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "http://host1/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	fs := sdk.WithContext(c)

	t.Run("GetBoolWithDefault", func(t *testing.T) {
		assert.True(t, fs.GetBoolWithDefault("feature1", false))
		assert.False(t, fs.GetBoolWithDefault("feature2", true))
		assert.False(t, fs.GetBoolWithDefault("nonexistent", false))
		assert.True(t, fs.GetBoolWithDefault("nonexistent", true))
	})

	t.Run("GetIntWithDefault", func(t *testing.T) {
		assert.Equal(t, 100, fs.GetIntWithDefault("maxItems", 0))
		assert.Equal(t, 123, fs.GetIntWithDefault("nonexistent", 123))
	})

	t.Run("GetFloat64WithDefault", func(t *testing.T) {
		assert.Equal(t, 0.1, fs.GetFloat64WithDefault("discount", 0.0))
		assert.Equal(t, 3.14, fs.GetFloat64WithDefault("nonexistent", 3.14))
	})

	t.Run("GetStringSliceWithDefault", func(t *testing.T) {
		assert.Equal(t, []string{"us-east", "us-west"}, fs.GetStringSliceWithDefault("allowedRegions", []string{}))
		assert.Equal(t, []string{"default"}, fs.GetStringSliceWithDefault("nonexistent", []string{"default"}))
	})

	t.Run("GetMapWithDefault", func(t *testing.T) {
		val := fs.GetMapWithDefault("metadata", map[string]interface{}{"default": true})
		assert.Equal(t, "1.0", val["version"])

		val = fs.GetMapWithDefault("nonexistent", map[string]interface{}{"default": true})
		assert.Equal(t, true, val["default"])
	})
}