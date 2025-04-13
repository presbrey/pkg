package t

import (
	"github.com/labstack/echo/v4"
)

// EchoMount defines an interface that captures the common routing methods
// between echo.Echo and echo.Group, allowing for more flexible route registration.
type EchoMount interface {
	// Use adds middleware to the router
	Use(middleware ...echo.MiddlewareFunc)

	// HTTP routing methods
	CONNECT(path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc) *echo.Route
	DELETE(path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc) *echo.Route
	GET(path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc) *echo.Route
	HEAD(path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc) *echo.Route
	OPTIONS(path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc) *echo.Route
	PATCH(path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc) *echo.Route
	POST(path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc) *echo.Route
	PUT(path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc) *echo.Route
	TRACE(path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc) *echo.Route

	// RouteNotFound handles not found routes
	RouteNotFound(path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc) *echo.Route

	// Any handles all HTTP methods
	Any(path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc) []*echo.Route

	// Match handles a single HTTP method
	Match(methods []string, path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc) []*echo.Route

	// Add registers a new route with a matcher for the URL path and method
	Add(method, path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc) *echo.Route

	// Group creates a new router group with prefix and optional middleware
	Group(prefix string, m ...echo.MiddlewareFunc) *echo.Group
}

// Ensure that both echo.Echo and echo.Group implement the EchoMount interface
var (
	_ EchoMount = (*echo.Echo)(nil)
	_ EchoMount = (*echo.Group)(nil)
)
