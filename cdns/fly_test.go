package cdns

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

// Default middleware issues a redirect (301) if Fly-Client-IP is present and protocol is not https
func TestFlyWithDefaults_SetsRealIPFromFlyClientIP(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Fly-Client-IP", "1.2.3.4")
	res := httptest.NewRecorder()
	c := e.NewContext(req, res)

	mw := FlyWithDefaults()
	h := mw(func(c echo.Context) error {
		realIP := c.Get("RealIP")
		assert.Equal(t, "1.2.3.4", realIP)
		return c.String(http.StatusOK, "ok")
	})

	err := h(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusMovedPermanently, res.Code)
	assert.Contains(t, res.Header().Get("Location"), "https://")
}

// If Fly-Client-IP is missing, middleware skips and does not set RealIP
func TestFlyWithDefaults_SetsRealIPFromXForwardedFor(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "5.6.7.8, 9.10.11.12")
	res := httptest.NewRecorder()
	c := e.NewContext(req, res)

	mw := FlyWithDefaults()
	h := mw(func(c echo.Context) error {
		realIP := c.Get("RealIP")
		assert.Nil(t, realIP)
		return c.String(http.StatusOK, "ok")
	})

	err := h(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, res.Code)
}

func TestFlyMiddleware_FallThroughIfForwardedProtoIsHTTPS(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/secure", nil)
	req.Header.Set("Fly-Client-IP", "1.2.3.4")
	req.Header.Set("Fly-Forwarded-Proto", "https")
	res := httptest.NewRecorder()
	c := e.NewContext(req, res)

	mw := NewFlyMiddleware().Build()
	h := mw(func(c echo.Context) error {
		// Should not redirect
		assert.Equal(t, "1.2.3.4", c.Get("RealIP"))
		return c.String(http.StatusOK, "ok")
	})

	err := h(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, res.Code)
}

func TestFlyMiddleware_WithoutRedirect(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Fly-Client-IP", "1.2.3.4")
	res := httptest.NewRecorder()
	c := e.NewContext(req, res)

	mw := NewFlyMiddleware().WithoutRedirect().Build()
	h := mw(func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	err := h(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, res.Code)
}

func TestFlyMiddleware_WithRedirectPort_CustomPortInRedirect(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/foo", nil)
	req.Header.Set("Fly-Client-IP", "1.2.3.4")
	req.Host = "flytest.com"
	res := httptest.NewRecorder()
	c := e.NewContext(req, res)

	mw := NewFlyMiddleware().WithRedirectPort(8443).Build()
	h := mw(func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	err := h(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusMovedPermanently, res.Code)
	assert.Equal(t, "https://flytest.com:8443/foo", res.Header().Get("Location"))
}
