package cdns

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

// Default middleware issues a redirect (301) if Cf-Visitor does not indicate HTTPS
func TestCloudflareMiddleware_PassThroughIfNoCfConnectingIp(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	// No Cf-Connecting-Ip header
	res := httptest.NewRecorder()
	c := e.NewContext(req, res)

	mw := CloudflareWithDefaults()
	h := mw(func(c echo.Context) error {
		// RealIP should not be set
		assert.Nil(t, c.Get("RealIP"))
		return c.String(http.StatusOK, "ok")
	})

	err := h(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, res.Code)
}

func TestCloudflareMiddleware_FallThroughIfSchemeIsHTTPS(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/secure", nil)
	req.Header.Set("Cf-Connecting-Ip", "2.3.4.5")
	req.Header.Set("Cf-Visitor", "{\"scheme\":\"https\"}")
	res := httptest.NewRecorder()
	c := e.NewContext(req, res)

	mw := CloudflareWithDefaults()
	h := mw(func(c echo.Context) error {
		// Should not redirect
		assert.Equal(t, "2.3.4.5", c.Get("RealIP"))
		return c.String(http.StatusOK, "ok")
	})

	err := h(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, res.Code)
}

func TestCloudflareWithDefaults_SetsRealIPFromCfConnectingIp(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Cf-Connecting-Ip", "2.3.4.5")
	res := httptest.NewRecorder()
	c := e.NewContext(req, res)

	mw := CloudflareWithDefaults()
	h := mw(func(c echo.Context) error {
		realIP := c.Get("RealIP")
		assert.Equal(t, "2.3.4.5", realIP)
		return c.String(http.StatusOK, "ok")
	})

	err := h(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusMovedPermanently, res.Code)
	assert.Contains(t, res.Header().Get("Location"), "https://")
}

func TestCloudflareMiddleware_WithoutRedirect(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Cf-Connecting-Ip", "2.3.4.5")
	res := httptest.NewRecorder()
	c := e.NewContext(req, res)

	mw := NewCloudflareMiddleware().WithoutRedirect().Build()
	h := mw(func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	err := h(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, res.Code)
}

// WithRedirectPort should also redirect if Cf-Visitor does not indicate HTTPS
func TestCloudflareMiddleware_WithRedirectPort(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Cf-Connecting-Ip", "2.3.4.5")
	req.Host = "example.com"
	res := httptest.NewRecorder()
	c := e.NewContext(req, res)

	mw := NewCloudflareMiddleware().WithRedirectPort(8443).Build()
	h := mw(func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	err := h(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusMovedPermanently, res.Code)
	assert.Equal(t, "https://example.com:8443/", res.Header().Get("Location"))
}

func TestCloudflareWithDefaults_HTTPSRedirect(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/some/path?foo=bar", nil)
	req.Header.Set("Cf-Connecting-Ip", "2.3.4.5")
	req.Header.Set("Cf-Visitor", "{\"scheme\":\"http\"}")
	req.Host = "example.com"
	res := httptest.NewRecorder()
	c := e.NewContext(req, res)

	mw := CloudflareWithDefaults()
	h := mw(func(c echo.Context) error {
		return c.String(http.StatusOK, "should not reach here")
	})

	err := h(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusMovedPermanently, res.Code)
	assert.Equal(t, "https://example.com/some/path?foo=bar", res.Header().Get("Location"))
}
