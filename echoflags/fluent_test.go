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
		MultihostBase: server.URL,
		DefaultHost:   "host1",
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
	})

	t.Run("GetStringWithDefault", func(t *testing.T) {
		val := fs.GetStringWithDefault("nonexistent", "default")
		assert.Equal(t, "default", val)
	})

	t.Run("GetBool", func(t *testing.T) {
		val, err := fs.GetBool("feature1")
		require.NoError(t, err)
		assert.True(t, val)
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
	})

	t.Run("IsEnabled", func(t *testing.T) {
		assert.True(t, fs.IsEnabled("feature1"))
		assert.False(t, fs.IsEnabled("feature2"))
		assert.True(t, userFs.IsEnabled("feature2"))
	})
}
