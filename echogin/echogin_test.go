package echogin

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock Validator for testing
type MockValidator struct{}

func (mv *MockValidator) Validate(i interface{}) error {
	// Minimal implementation for testing setters
	return nil
}

// Mock Renderer for testing
type MockRenderer struct{}

func (mr *MockRenderer) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	// Minimal implementation for testing setters
	return nil
}

// Helper to reset global state between tests
var resetMu sync.Mutex

func resetGlobals() {
	resetMu.Lock()
	defer resetMu.Unlock()

	// Reset the core echogin globals by creating new instances
	defaultEcho = echo.New()
	defaultTarget = defaultEcho.Group("")

	// Re-apply necessary initial configurations similar to the init() block in echogin.go
	// (Consider if the init() logic needs duplication or if resetting is sufficient)
	defaultEcho.HideBanner = true // Assuming these are desired defaults for tests too
	defaultEcho.HidePort = true

	// Any other package-level state in echogin.go that needs resetting for tests
	// should be added here.

	hostGroups = sync.Map{}
}

func performRequest(method, path string, body io.Reader) *httptest.ResponseRecorder {
	resetMu.Lock()
	defer resetMu.Unlock()
	req := httptest.NewRequest(method, path, body)
	rec := httptest.NewRecorder()
	defaultEcho.ServeHTTP(rec, req)
	return rec
}

func TestRouting(t *testing.T) {
	resetGlobals() // Ensure clean state

	handler := func(c echo.Context) error {
		// Use a simpler body to ensure the handler itself is reached
		return c.String(http.StatusOK, "match handler reached")
	}

	// Register ONLY the Match route
	Match([]string{http.MethodGet, http.MethodPost}, "/match", handler)

	t.Run("Match_GET", func(t *testing.T) {
		rec := performRequest(http.MethodGet, "/match", nil)
		assert.Equal(t, http.StatusOK, rec.Code, "GET /match should return 200")
		assert.Contains(t, rec.Body.String(), "match handler reached", "GET /match response body mismatch")
	})

	t.Run("Match_POST", func(t *testing.T) {
		rec := performRequest(http.MethodPost, "/match", nil)
		assert.Equal(t, http.StatusOK, rec.Code, "POST /match should return 200")
		assert.Contains(t, rec.Body.String(), "match handler reached", "POST /match response body mismatch")
	})

	// Add a test for a non-matched method to ensure it still 405s
	t.Run("Match_PUT_NotFound", func(t *testing.T) {
		rec := performRequest(http.MethodPut, "/match", nil)
		// Expect 405 Method Not Allowed when path exists but method doesn't
		assert.Equal(t, http.StatusMethodNotAllowed, rec.Code, "PUT /match should return 405")
	})

	// Test a different path to ensure router isn't completely broken
	t.Run("OtherPath_NotFound", func(t *testing.T) {
		rec := performRequest(http.MethodGet, "/not-match", nil)
		assert.Equal(t, http.StatusNotFound, rec.Code, "GET /not-match should return 404")
	})
}

func TestMiddleware(t *testing.T) {
	resetGlobals()

	Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Response().Header().Set("X-Global-Use", "used")
			return next(c)
		}
	})

	Pre(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Response().Header().Set("X-Global-Pre", "pre-used")
			return next(c)
		}
	})

	GET("/mw", func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	}, func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Response().Header().Set("X-Route-MW", "route-used")
			return next(c)
		}
	})

	rec := performRequest(http.MethodGet, "/mw", nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "used", rec.Header().Get("X-Global-Use"))
	assert.Equal(t, "pre-used", rec.Header().Get("X-Global-Pre"))
	assert.Equal(t, "route-used", rec.Header().Get("X-Route-MW"))
}

func TestGroups(t *testing.T) {
	resetGlobals()

	admin := Group("/admin")
	admin.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if c.Request().Header.Get("Authorization") != "secret" {
				return c.String(http.StatusUnauthorized, "Unauthorized")
			}
			return next(c)
		}
	})

	admin.GET("/users", func(c echo.Context) error {
		return c.String(http.StatusOK, "Admin Users")
	})

	GET("/public", func(c echo.Context) error {
		return c.String(http.StatusOK, "Public Access")
	})

	// Test public route (no auth needed)
	recPublic := performRequest(http.MethodGet, "/public", nil)
	assert.Equal(t, http.StatusOK, recPublic.Code)
	assert.Equal(t, "Public Access", recPublic.Body.String())

	// Test admin route without auth
	recAdminNoAuth := performRequest(http.MethodGet, "/admin/users", nil)
	assert.Equal(t, http.StatusUnauthorized, recAdminNoAuth.Code)
	assert.Equal(t, "Unauthorized", recAdminNoAuth.Body.String())

	// Test admin route with auth
	resetMu.Lock()
	reqAdminAuth := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	reqAdminAuth.Header.Set("Authorization", "secret")
	recAdminAuth := httptest.NewRecorder()
	defaultEcho.ServeHTTP(recAdminAuth, reqAdminAuth)
	resetMu.Unlock()
	assert.Equal(t, http.StatusOK, recAdminAuth.Code)
	assert.Equal(t, "Admin Users", recAdminAuth.Body.String())
}

// Note: Testing Host requires more setup, potentially involving custom listeners or hosts file modifications.
// This basic test checks if the Host function correctly configures the router.
func TestHost(t *testing.T) {
	resetGlobals()

	exampleHost := Host("example.com")
	exampleHost.GET("/test", func(c echo.Context) error {
		return c.String(http.StatusOK, "Example Host")
	})

	apiHost := Host("api.example.com")
	apiHost.GET("/test", func(c echo.Context) error {
		return c.String(http.StatusOK, "API Host")
	})

	// Test request to example.com
	resetMu.Lock()
	reqExample := httptest.NewRequest(http.MethodGet, "/test", nil)
	reqExample.Host = "example.com"
	recExample := httptest.NewRecorder()
	defaultEcho.ServeHTTP(recExample, reqExample)
	resetMu.Unlock()
	assert.Equal(t, http.StatusOK, recExample.Code)
	assert.Equal(t, "Example Host", recExample.Body.String())

	// Test request to api.example.com
	resetMu.Lock()
	reqAPI := httptest.NewRequest(http.MethodGet, "/test", nil)
	reqAPI.Host = "api.example.com"
	recAPI := httptest.NewRecorder()
	defaultEcho.ServeHTTP(recAPI, reqAPI)
	resetMu.Unlock()
	assert.Equal(t, http.StatusOK, recAPI.Code)
	assert.Equal(t, "API Host", recAPI.Body.String())

	// Test request to unknown host (should 404)
	reqUnknownHost := httptest.NewRequest(http.MethodGet, "/test", nil)
	reqUnknownHost.Host = "unknown.com"
	recUnknownHost := httptest.NewRecorder()
	defaultEcho.ServeHTTP(recUnknownHost, reqUnknownHost)
	assert.Equal(t, http.StatusNotFound, recUnknownHost.Code)
}

func TestConfigSetters(t *testing.T) {
	resetGlobals()

	e := Echo() // Get the instance reset by resetGlobals

	// Test HideBanner
	assert.False(t, e.Debug)
	SetDebug(true)
	assert.True(t, e.Debug)
	SetDebug(false)
	assert.False(t, e.Debug)

	// Test HideBanner
	assert.True(t, e.HideBanner) // resetGlobals sets this to true
	SetHideBanner(true)          // Set it to true again (shouldn't change)
	assert.True(t, e.HideBanner)
	SetHideBanner(false)          // Set it to false
	assert.False(t, e.HideBanner) // Line 292: Should be false

	// Test HidePort
	assert.True(t, e.HidePort) // resetGlobals sets this to true
	SetHidePort(true)          // Set it to true again (shouldn't change)
	assert.True(t, e.HidePort)
	SetHidePort(false)          // Set it to false
	assert.False(t, e.HidePort) // Line 296: Should be false

	// Test other setters (just basic assignment checks)
	assert.Nil(t, e.Validator)
	newValidator := &MockValidator{}
	SetValidator(newValidator)
	assert.Same(t, newValidator, e.Validator)

	assert.NotNil(t, e.Binder) // Echo has a default binder
	newBinder := &echo.DefaultBinder{}
	SetBinder(newBinder)
	assert.Same(t, newBinder, e.Binder)

	assert.NotNil(t, e.JSONSerializer) // Echo has a default serializer
	newSerializer := &echo.DefaultJSONSerializer{}
	SetJSONSerializer(newSerializer)
	assert.Same(t, newSerializer, e.JSONSerializer)

	assert.Nil(t, e.Renderer) // Default is nil
	newRenderer := &MockRenderer{}
	SetRenderer(newRenderer)
	assert.Same(t, newRenderer, e.Renderer)

	// Test server/listener setters (just check assignment)
	newServer := &http.Server{}
	SetServer(newServer)
	assert.Same(t, newServer, e.Server)

	newTLSServer := &http.Server{}
	SetTLSServer(newTLSServer)
	assert.Same(t, newTLSServer, e.TLSServer)
}

func TestServerLifecycle(t *testing.T) {
	// Direct testing of Start, Shutdown, Close is complex in unit tests.
	// We'll perform some basic checks.

	// Test Start with default address (doesn't actually start, just checks setup)
	t.Run("StartDefaultAddress", func(t *testing.T) {
		resetGlobals()
		// We expect Start to block or return an error if port is taken.
		// We can test the error case for invalid address.
		err := Start("invalid-address")
		assert.Error(t, err) // Echo's Start usually wraps net.Listen errors
	})

	t.Run("StartSpecificAddress", func(t *testing.T) {
		resetGlobals()
		addr := "127.0.0.1:0" // Use 127.0.0.1 and let OS pick port
		e := Echo()

		started := make(chan struct{})  // Channel to signal startup
		startErr := make(chan error, 1) // Channel to capture Start error

		go func() {
			// Set listener explicitly *before* starting in goroutine
			l, err := net.Listen("tcp", addr)
			if err != nil {
				startErr <- fmt.Errorf("failed to create listener: %w", err)
				close(started) // Signal completion (with error)
				return
			}
			SetListener(l)           // Set the listener for echogin to use
			addr = l.Addr().String() // Get the actual address with the assigned port

			// Signal that the listener is ready and address is updated
			close(started)

			// Call Start (now Start address is ignored as Listener is set)
			if err := Start(""); err != nil && err != http.ErrServerClosed {
				// Report error only if it's not the expected shutdown error
				// Use non-blocking send in case main goroutine already exited
				select {
				case startErr <- fmt.Errorf("server start failed: %w", err):
				default:
				}
			}
		}()

		// Wait for the goroutine to signal that the listener is ready
		<-started

		// Check for listener creation errors
		select {
		case err := <-startErr:
			t.Fatalf("Setup failed: %v", err)
		default:
			// No setup error
		}

		// Now it's safer to check the listener address
		require.NotNil(t, e.Listener, "Listener should be set")
		assert.Equal(t, addr, e.Listener.Addr().String())
		t.Logf("Server started on %s, attempting close.", addr)

		// Shutdown the server
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err := Shutdown(ctx)
		assert.NoError(t, err, "Shutdown should not return an error")

		// Check for any startup errors that might have occurred after signaling
		select {
		case err := <-startErr:
			t.Errorf("Server Start() returned an unexpected error: %v", err)
		default:
			// No error from Start()
		}
	})

	// Test starting on multiple addresses (conceptual, Echo stdlib doesn't support multiple directly)
	t.Run("StartMultipleAddresses", func(t *testing.T) {
		resetGlobals()
		err := Start(":8081", ":8082")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "multiple addresses not supported")
	})

	// Test Shutdown/Close (hard to test without a running server)
	t.Run("ShutdownWithoutStart", func(t *testing.T) {
		resetGlobals()
		// Shutdown requires a running server, should likely error or do nothing if not started.
		ctx := context.Background()
		err := Shutdown(ctx)
		assert.NoError(t, err, "Shutdown before start should return nil, not an error")
	})

	t.Run("CloseWithoutStart", func(t *testing.T) {
		resetGlobals()
		// Close might return nil or an error depending on the state of the internal listener.
		err := Close()
		assert.NoError(t, err) // Echo's Close() checks for nil listener/server
	})

}

func TestEchoGetter(t *testing.T) {
	resetGlobals()

	e := Echo()
	assert.NotNil(t, e)
	assert.Same(t, defaultEcho, e)
}
