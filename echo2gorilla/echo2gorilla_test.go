package echo2gorilla

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

// Test data structures
type User struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Echo handler that returns JSON
func echoJSONHandler(c echo.Context) error {
	user := &User{
		ID:   1,
		Name: "John Doe",
	}
	return c.JSON(http.StatusOK, user)
}

// Echo handler that uses path parameters
func echoParamsHandler(c echo.Context) error {
	id := c.Param("id")
	name := c.Param("name")
	return c.JSON(http.StatusOK, map[string]string{
		"id":   id,
		"name": name,
	})
}

// Echo handler that binds JSON
func echoBindHandler(c echo.Context) error {
	user := new(User)
	if err := c.Bind(user); err != nil {
		return err
	}
	user.Name = "Modified: " + user.Name
	return c.JSON(http.StatusOK, user)
}

// Echo middleware that adds a header
func echoHeaderMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		// Directly set the header on the response writer to avoid nil pointer issues
		c.Response().Writer.Header().Set("X-Custom-Header", "EchoMiddleware")
		return next(c)
	}
}

// Echo middleware that checks authorization
func echoAuthMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		auth := c.Request().Header.Get("Authorization")
		if auth != "Bearer valid-token" {
			return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
		}
		return next(c)
	}
}

func TestEchoHandlerInGorilla(t *testing.T) {
	// Create a new Gorilla router
	r := mux.NewRouter()

	// Mount Echo handlers into Gorilla routes
	r.HandleFunc("/users", HandlerFunc(echoJSONHandler)).Methods("GET")
	r.HandleFunc("/users/{id}/{name}", HandlerFunc(echoParamsHandler)).Methods("GET")
	r.HandleFunc("/users", HandlerFunc(echoBindHandler)).Methods("POST")

	// Test JSON handler
	t.Run("JSON Handler", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/users", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var user User
		err := json.Unmarshal(w.Body.Bytes(), &user)
		assert.NoError(t, err)
		assert.Equal(t, 1, user.ID)
		assert.Equal(t, "John Doe", user.Name)
	})

	// Test params handler
	t.Run("Params Handler", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/users/123/alice", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var params map[string]string
		err := json.Unmarshal(w.Body.Bytes(), &params)
		assert.NoError(t, err)
		assert.Equal(t, "123", params["id"])
		assert.Equal(t, "alice", params["name"])
	})

	// Test bind handler
	t.Run("Bind Handler", func(t *testing.T) {
		reqBody := `{"id": 2, "name": "Jane Smith"}`
		req := httptest.NewRequest("POST", "/users", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var user User
		err := json.Unmarshal(w.Body.Bytes(), &user)
		assert.NoError(t, err)
		assert.Equal(t, 2, user.ID)
		assert.Equal(t, "Modified: Jane Smith", user.Name)
	})
}

func TestEchoMiddlewareInGorilla(t *testing.T) {
	// Create a new Gorilla router
	r := mux.NewRouter()

	// Apply Echo middleware to a Gorilla route
	r.HandleFunc("/protected", HandlerFunc(echoHeaderMiddleware(echoJSONHandler))).Methods("GET")

	// Apply multiple Echo middleware to a Gorilla route
	r.HandleFunc("/auth", HandlerFunc(echoHeaderMiddleware(echoAuthMiddleware(echoJSONHandler)))).Methods("GET")

	// Apply Echo middleware to a Gorilla subrouter - use a different approach
	sub := r.PathPrefix("/api").Subrouter()
	// Instead of using sub.Use, apply the middleware directly to the handler
	sub.HandleFunc("/users", HandlerFunc(echoHeaderMiddleware(echoJSONHandler))).Methods("GET")

	// Test middleware adding header
	t.Run("Middleware Header", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/protected", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "EchoMiddleware", w.Header().Get("X-Custom-Header"))
	})

	// Test auth middleware - unauthorized
	t.Run("Auth Middleware - Unauthorized", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/auth", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	// Test auth middleware - authorized
	t.Run("Auth Middleware - Authorized", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/auth", nil)
		req.Header.Set("Authorization", "Bearer valid-token")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "EchoMiddleware", w.Header().Get("X-Custom-Header"))
	})

	// Test middleware on subrouter
	t.Run("Middleware on Subrouter", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/users", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "EchoMiddleware", w.Header().Get("X-Custom-Header"))
		
		// Also verify the JSON response
		var user User
		err := json.Unmarshal(w.Body.Bytes(), &user)
		assert.NoError(t, err)
		assert.Equal(t, 1, user.ID)
		assert.Equal(t, "John Doe", user.Name)
	})
}

// Integration test that combines everything
func TestCompleteIntegration(t *testing.T) {
	// Create a new Gorilla router
	r := mux.NewRouter()

	// Create a middleware function
	apiVersionMiddleware := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Response().Writer.Header().Set("X-API-Version", "1.0")
			return next(c)
		}
	}
	
	// Create API subrouter
	api := r.PathPrefix("/api").Subrouter()

	// Add routes with various combinations of handlers and middleware
	api.HandleFunc("/users", HandlerFunc(apiVersionMiddleware(echoJSONHandler))).Methods("GET")
	api.HandleFunc("/users", HandlerFunc(apiVersionMiddleware(echoBindHandler))).Methods("POST")
	api.HandleFunc("/users/{id}/{name}", HandlerFunc(apiVersionMiddleware(echoParamsHandler))).Methods("GET")
	
	// Protected routes with auth middleware
	protected := api.PathPrefix("/protected").Subrouter()
	// Combine middleware functions
	protectedHandler := apiVersionMiddleware(echoAuthMiddleware(func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{
			"message": "This is a protected resource",
		})
	}))
	protected.HandleFunc("/profile", HandlerFunc(protectedHandler)).Methods("GET")

	// Create a test server
	server := httptest.NewServer(r)
	defer server.Close()

	// Helper function for making requests
	makeRequest := func(method, path string, body io.Reader, headers map[string]string) (*http.Response, error) {
		req, err := http.NewRequest(method, server.URL+path, body)
		if err != nil {
			return nil, err
		}
		
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		
		return http.DefaultClient.Do(req)
	}

	// Test GET /api/users
	t.Run("GET /api/users", func(t *testing.T) {
		resp, err := makeRequest("GET", "/api/users", nil, nil)
		assert.NoError(t, err)
		defer resp.Body.Close()
		
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "1.0", resp.Header.Get("X-API-Version"))
		
		var user User
		err = json.NewDecoder(resp.Body).Decode(&user)
		assert.NoError(t, err)
		assert.Equal(t, 1, user.ID)
		assert.Equal(t, "John Doe", user.Name)
	})

	// Test POST /api/users
	t.Run("POST /api/users", func(t *testing.T) {
		reqBody := `{"id": 3, "name": "Bob Johnson"}`
		resp, err := makeRequest("POST", "/api/users", strings.NewReader(reqBody), map[string]string{
			"Content-Type": "application/json",
		})
		assert.NoError(t, err)
		defer resp.Body.Close()
		
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "1.0", resp.Header.Get("X-API-Version"))
		
		var user User
		err = json.NewDecoder(resp.Body).Decode(&user)
		assert.NoError(t, err)
		assert.Equal(t, 3, user.ID)
		assert.Equal(t, "Modified: Bob Johnson", user.Name)
	})

	// Test GET /api/users/{id}/{name}
	t.Run("GET /api/users/{id}/{name}", func(t *testing.T) {
		resp, err := makeRequest("GET", "/api/users/456/charlie", nil, nil)
		assert.NoError(t, err)
		defer resp.Body.Close()
		
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "1.0", resp.Header.Get("X-API-Version"))
		
		var params map[string]string
		err = json.NewDecoder(resp.Body).Decode(&params)
		assert.NoError(t, err)
		assert.Equal(t, "456", params["id"])
		assert.Equal(t, "charlie", params["name"])
	})

	// Test protected route - unauthorized
	t.Run("Protected Route - Unauthorized", func(t *testing.T) {
		resp, err := makeRequest("GET", "/api/protected/profile", nil, nil)
		assert.NoError(t, err)
		defer resp.Body.Close()
		
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	// Test protected route - authorized
	t.Run("Protected Route - Authorized", func(t *testing.T) {
		resp, err := makeRequest("GET", "/api/protected/profile", nil, map[string]string{
			"Authorization": "Bearer valid-token",
		})
		assert.NoError(t, err)
		defer resp.Body.Close()
		
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "1.0", resp.Header.Get("X-API-Version"))
		
		var result map[string]string
		err = json.NewDecoder(resp.Body).Decode(&result)
		assert.NoError(t, err)
		assert.Equal(t, "This is a protected resource", result["message"])
	})
}
