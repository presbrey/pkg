package echofly

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStickySessions_NoFlyMachineID(t *testing.T) {
	// Ensure FLY_MACHINE_ID is not set
	originalID := os.Getenv("FLY_MACHINE_ID")
	os.Unsetenv("FLY_MACHINE_ID")
	defer func() {
		if originalID != "" {
			os.Setenv("FLY_MACHINE_ID", originalID)
		}
	}()

	e := echo.New()
	e.Use(StickySessions())
	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "test")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "test", rec.Body.String())
	// Should not set any cookies when not on Fly.io
	assert.Empty(t, rec.Header().Get("Set-Cookie"))
}

func TestStickySessions_FirstRequest(t *testing.T) {
	// Set up Fly.io environment
	testMachineID := "test-machine-123"
	os.Setenv("FLY_MACHINE_ID", testMachineID)
	defer os.Unsetenv("FLY_MACHINE_ID")

	e := echo.New()
	e.Use(StickySessions())
	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "test")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "test", rec.Body.String())

	// Should set cookie with machine ID
	cookies := rec.Header().Get("Set-Cookie")
	assert.Contains(t, cookies, "fly-machine-id="+testMachineID)
	assert.Contains(t, cookies, "Max-Age=518400") // 6 days in seconds
	assert.Contains(t, cookies, "Path=/")
	assert.Contains(t, cookies, "HttpOnly")
}

func TestStickySessions_SameMachine(t *testing.T) {
	testMachineID := "test-machine-123"
	os.Setenv("FLY_MACHINE_ID", testMachineID)
	defer os.Unsetenv("FLY_MACHINE_ID")

	e := echo.New()
	e.Use(StickySessions())
	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "test")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{
		Name:  "fly-machine-id",
		Value: testMachineID,
	})
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "test", rec.Body.String())
	// Should not set new cookie since machine matches
	assert.Empty(t, rec.Header().Get("Set-Cookie"))
	// Should not set Fly-Replay header
	assert.Empty(t, rec.Header().Get("Fly-Replay"))
}

func TestStickySessions_DifferentMachine(t *testing.T) {
	testMachineID := "test-machine-123"
	cookieMachineID := "different-machine-456"
	os.Setenv("FLY_MACHINE_ID", testMachineID)
	defer os.Unsetenv("FLY_MACHINE_ID")

	e := echo.New()
	e.Use(StickySessions())
	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "test")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{
		Name:  "fly-machine-id",
		Value: cookieMachineID,
	})
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusTemporaryRedirect, rec.Code)
	assert.Empty(t, rec.Body.String())
	// Should set Fly-Replay header to redirect to correct machine
	assert.Equal(t, "instance="+cookieMachineID, rec.Header().Get("Fly-Replay"))
}

func TestStickySessionsWithConfig_CustomCookieName(t *testing.T) {
	testMachineID := "test-machine-123"
	os.Setenv("FLY_MACHINE_ID", testMachineID)
	defer os.Unsetenv("FLY_MACHINE_ID")

	config := StickySessionsConfig{
		CookieName: "custom-machine-id",
		MaxAge:     24 * time.Hour,
	}

	e := echo.New()
	e.Use(StickySessionsWithConfig(config))
	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "test")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	cookies := rec.Header().Get("Set-Cookie")
	assert.Contains(t, cookies, "custom-machine-id="+testMachineID)
	assert.Contains(t, cookies, "Max-Age=86400") // 24 hours in seconds
}

func TestStickySessionsWithConfig_Skipper(t *testing.T) {
	testMachineID := "test-machine-123"
	os.Setenv("FLY_MACHINE_ID", testMachineID)
	defer os.Unsetenv("FLY_MACHINE_ID")

	config := StickySessionsConfig{
		Skipper: func(c echo.Context) bool {
			return c.Path() == "/health"
		},
	}

	e := echo.New()
	e.Use(StickySessionsWithConfig(config))
	e.GET("/health", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})
	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "test")
	})

	// Test skipped route
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "ok", rec.Body.String())
	assert.Empty(t, rec.Header().Get("Set-Cookie"))

	// Test non-skipped route
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	cookies := rec.Header().Get("Set-Cookie")
	assert.Contains(t, cookies, "fly-machine-id="+testMachineID)
}

func TestStickySessionsWithConfig_EmptyCookie(t *testing.T) {
	testMachineID := "test-machine-123"
	os.Setenv("FLY_MACHINE_ID", testMachineID)
	defer os.Unsetenv("FLY_MACHINE_ID")

	e := echo.New()
	e.Use(StickySessions())
	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "test")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{
		Name:  "fly-machine-id",
		Value: "", // Empty cookie value
	})
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	// Should set new cookie since existing one is empty
	cookies := rec.Header().Get("Set-Cookie")
	assert.Contains(t, cookies, "fly-machine-id="+testMachineID)
}

func TestStickySessionsWithConfig_DefaultValues(t *testing.T) {
	testMachineID := "test-machine-123"
	os.Setenv("FLY_MACHINE_ID", testMachineID)
	defer os.Unsetenv("FLY_MACHINE_ID")

	// Test with empty config - should use defaults
	config := StickySessionsConfig{}

	e := echo.New()
	e.Use(StickySessionsWithConfig(config))
	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "test")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	cookies := rec.Header().Get("Set-Cookie")
	assert.Contains(t, cookies, "fly-machine-id="+testMachineID)
	assert.Contains(t, cookies, "Max-Age=518400") // Default 6 days
}

func TestDefaultStickySessionsConfig(t *testing.T) {
	config := DefaultStickySessionsConfig()
	
	assert.Equal(t, CookieName, config.CookieName)
	assert.Equal(t, DefaultMaxAge, config.MaxAge)
	assert.Nil(t, config.Skipper)
}

func TestConstants(t *testing.T) {
	assert.Equal(t, "fly-machine-id", CookieName)
	assert.Equal(t, 6*24*time.Hour, DefaultMaxAge)
	assert.Equal(t, "Fly-Replay", FlyReplayHeader)
}

// Integration test simulating multiple requests with different scenarios
func TestStickySessionsIntegration(t *testing.T) {
	testMachineID := "integration-test-machine"
	os.Setenv("FLY_MACHINE_ID", testMachineID)
	defer os.Unsetenv("FLY_MACHINE_ID")

	e := echo.New()
	e.Use(StickySessionsWithConfig(StickySessionsConfig{
		CookieName: "session-id",
		MaxAge:     1 * time.Hour,
		Skipper: func(c echo.Context) bool {
			return strings.HasPrefix(c.Path(), "/api/")
		},
	}))

	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "home")
	})
	e.GET("/api/health", func(c echo.Context) error {
		return c.String(http.StatusOK, "healthy")
	})

	// Test 1: First request should set cookie
	req1 := httptest.NewRequest(http.MethodGet, "/", nil)
	rec1 := httptest.NewRecorder()
	e.ServeHTTP(rec1, req1)

	require.Equal(t, http.StatusOK, rec1.Code)
	cookies1 := rec1.Header().Get("Set-Cookie")
	require.Contains(t, cookies1, "session-id="+testMachineID)

	// Extract cookie for next request
	var sessionCookie *http.Cookie
	for _, cookie := range rec1.Result().Cookies() {
		if cookie.Name == "session-id" {
			sessionCookie = cookie
			break
		}
	}
	require.NotNil(t, sessionCookie)

	// Test 2: Subsequent request with same machine should not redirect
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.AddCookie(sessionCookie)
	rec2 := httptest.NewRecorder()
	e.ServeHTTP(rec2, req2)

	assert.Equal(t, http.StatusOK, rec2.Code)
	assert.Equal(t, "home", rec2.Body.String())
	assert.Empty(t, rec2.Header().Get("Set-Cookie"))
	assert.Empty(t, rec2.Header().Get("Fly-Replay"))

	// Test 3: API endpoint should be skipped
	req3 := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec3 := httptest.NewRecorder()
	e.ServeHTTP(rec3, req3)

	assert.Equal(t, http.StatusOK, rec3.Code)
	assert.Equal(t, "healthy", rec3.Body.String())
	assert.Empty(t, rec3.Header().Get("Set-Cookie"))

	// Test 4: Request with different machine ID should redirect
	req4 := httptest.NewRequest(http.MethodGet, "/", nil)
	req4.AddCookie(&http.Cookie{
		Name:  "session-id",
		Value: "different-machine-id",
	})
	rec4 := httptest.NewRecorder()
	e.ServeHTTP(rec4, req4)

	assert.Equal(t, http.StatusTemporaryRedirect, rec4.Code)
	assert.Equal(t, "instance=different-machine-id", rec4.Header().Get("Fly-Replay"))
}
