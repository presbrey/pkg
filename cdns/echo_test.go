package cdns

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestUseDefaults_IPExtractorPrefersCloudflareHeader(t *testing.T) {
	e := echo.New()
	UseDefaults(e)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Cf-Connecting-Ip", "2.2.2.2")
	req.Header.Set("Fly-Client-IP", "1.1.1.1")
	res := httptest.NewRecorder()
	c := e.NewContext(req, res)

	// Simulate Echo's IP extraction logic
	ip := e.IPExtractor(req)
	assert.Equal(t, "2.2.2.2", ip)

	// RealIP() uses IPExtractor
	assert.Equal(t, "2.2.2.2", c.RealIP())
}

func TestUseDefaults_IPExtractorOnlyFly(t *testing.T) {
	e := echo.New()
	UseDefaults(e)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Fly-Client-IP", "1.1.1.1")
	res := httptest.NewRecorder()
	c := e.NewContext(req, res)

	// Simulate Echo's IP extraction logic
	ip := e.IPExtractor(req)
	assert.Equal(t, "1.1.1.1", ip)

	// RealIP() uses IPExtractor
	assert.Equal(t, "1.1.1.1", c.RealIP())
}

func TestUseDefaults_IPExtractorNoHeaders(t *testing.T) {
	e := echo.New()
	UseDefaults(e)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	res := httptest.NewRecorder()
	c := e.NewContext(req, res)

	// Simulate Echo's IP extraction logic
	ip := e.IPExtractor(req)
	assert.Equal(t, "", ip)

	// RealIP() uses IPExtractor
	assert.Equal(t, "", c.RealIP())
}
