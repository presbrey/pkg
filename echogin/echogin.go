package echogin

import (
	"context"
	"errors"
	"io/fs"
	"net"
	"net/http"
	"sync"

	"github.com/labstack/echo/v4"
)

// Context is an alias for echo.Context for familiarity.
type Context = echo.Context

// HandlerFunc is an alias for echo.HandlerFunc for familiarity.
type HandlerFunc = echo.HandlerFunc

// MiddlewareFunc is an alias for echo.MiddlewareFunc.
type MiddlewareFunc = echo.MiddlewareFunc

var (
	// defaultEcho holds the single, package-wide Echo instance.
	defaultEcho = echo.New()

	// defaultGroup holds the root group associated with defaultEcho.
	// Since *echo.Echo implements the Group interface, we can use it directly.
	defaultGroup = defaultEcho.Group("")

	// hostGroups caches host groups to ensure the same group is returned for the same host.
	hostGroups sync.Map
)

func init() {
	// Add default middleware similar to gin.Default()
	defaultEcho.HideBanner = true
	defaultEcho.HidePort = true
}

// Default initializes and returns the package-wide default *echo.Group.
// It's safe to call Default multiple times; initialization happens only once.
func Default() *echo.Group {
	return defaultGroup
}

// --- Package-level Routing Methods ---
// These methods operate on the default group initialized by Default().

// GET registers a new GET route for the default group.
func GET(path string, h HandlerFunc, m ...MiddlewareFunc) *echo.Route {
	return defaultGroup.GET(path, h, m...)
}

// POST registers a new POST route for the default group.
func POST(path string, h HandlerFunc, m ...MiddlewareFunc) *echo.Route {
	return defaultGroup.POST(path, h, m...)
}

// PUT registers a new PUT route for the default group.
func PUT(path string, h HandlerFunc, m ...MiddlewareFunc) *echo.Route {
	return defaultGroup.PUT(path, h, m...)
}

// DELETE registers a new DELETE route for the default group.
func DELETE(path string, h HandlerFunc, m ...MiddlewareFunc) *echo.Route {
	return defaultGroup.DELETE(path, h, m...)
}

// PATCH registers a new PATCH route for the default group.
func PATCH(path string, h HandlerFunc, m ...MiddlewareFunc) *echo.Route {
	return defaultGroup.PATCH(path, h, m...)
}

// OPTIONS registers a new OPTIONS route for the default group.
func OPTIONS(path string, h HandlerFunc, m ...MiddlewareFunc) *echo.Route {
	return defaultGroup.OPTIONS(path, h, m...)
}

// HEAD registers a new HEAD route for the default group.
func HEAD(path string, h HandlerFunc, m ...MiddlewareFunc) *echo.Route {
	return defaultGroup.HEAD(path, h, m...)
}

// Any registers a route that matches all the HTTP methods.
// GET, POST, PUT, PATCH, HEAD, OPTIONS, DELETE, CONNECT, TRACE.
func Any(path string, h HandlerFunc, m ...MiddlewareFunc) []*echo.Route {
	return defaultGroup.Any(path, h, m...)
}

// Match registers a new route for multiple HTTP methods.
func Match(methods []string, path string, h echo.HandlerFunc, m ...MiddlewareFunc) []*echo.Route {
	return defaultGroup.Match(methods, path, h, m...)
}

// Static registers a new route with path prefix to serve static files from the
// provided file system directory.
func Static(prefix, root string) {
	defaultGroup.Static(prefix, root)
}

// StaticFS registers a new route with path prefix to serve static files from the
// provided file system.
func StaticFS(path string, filesystem fs.FS) {
	defaultGroup.StaticFS(path, filesystem)
}

// --- Package-level Grouping and Middleware ---

// Group creates a new sub-group from the default group.
func Group(prefix string, m ...MiddlewareFunc) *echo.Group {
	return defaultGroup.Group(prefix, m...)
}

// Host creates a new sub-group from the default Echo instance with the specified host.
// It uses a sync.Map to cache host groups, ensuring that different callers get the
// same group when they pass the same host.
func Host(host string, m ...MiddlewareFunc) *echo.Group {
	// Check if we already have a group for this host
	if group, ok := hostGroups.Load(host); ok {
		// If middleware is provided, apply it to the existing group
		if len(m) > 0 {
			g := group.(*echo.Group)
			g.Use(m...)
		}
		return group.(*echo.Group)
	}

	// Create a new group for this host
	group := defaultEcho.Host(host)
	
	// Apply middleware if provided
	if len(m) > 0 {
		group.Use(m...)
	}
	
	// Store the group in the cache
	hostGroups.Store(host, group)
	
	return group
}

// Use applies middleware to the default group.
func Use(middleware ...MiddlewareFunc) {
	defaultGroup.Use(middleware...)
}

// --- Package-level Server Execution ---

// Start attaches the router to a http.Server and starts listening and serving HTTP requests.
//
// If no address is provided, it defaults to ":8080".
// If multiple addresses are provided, only the first one is used.
func Start(address ...string) error {
	if len(address) == 0 {
		return defaultEcho.Start(":8080")
	}
	if len(address) > 1 {
		return errors.New("multiple addresses not supported")
	}
	return defaultEcho.Start(address[0])
}

// Shutdown gracefully shuts down the server.
func Shutdown(ctx context.Context) error {
	return defaultEcho.Shutdown(ctx)
}

// Pre applies middleware before routing
func Pre(middleware ...MiddlewareFunc) {
	defaultEcho.Pre(middleware...)
}

// Close immediately shuts down the server.
func Close() error {
	return defaultEcho.Close()
}

// --- Convenience access to underlying Echo instance ---

// Echo returns the underlying *echo.Echo instance managed by echogin.
func Echo() *echo.Echo {
	return defaultEcho
}

// --- Package-wide configuration setters ---

// SetServer sets the server for the underlying Echo instance.
func SetServer(server *http.Server) {
	defaultEcho.Server = server
}

// SetTLSServer sets the TLS server for the underlying Echo instance.
func SetTLSServer(server *http.Server) {
	defaultEcho.TLSServer = server
}

// SetListener sets the listener for the underlying Echo instance.
func SetListener(listener net.Listener) {
	defaultEcho.Listener = listener
}

// SetTLSListener sets the TLS listener for the underlying Echo instance.
func SetTLSListener(listener net.Listener) {
	defaultEcho.TLSListener = listener
}

// SetListenerNetwork sets the listener network for the underlying Echo instance.
func SetListenerNetwork(network string) {
	defaultEcho.ListenerNetwork = network
}

// SetRenderer sets the renderer for the underlying Echo instance.
func SetRenderer(renderer echo.Renderer) {
	defaultEcho.Renderer = renderer
}

// SetIPExtractor sets the IP extractor for the underlying Echo instance.
func SetIPExtractor(extractor echo.IPExtractor) {
	defaultEcho.IPExtractor = extractor
}

// SetLogger sets the logger for the underlying Echo instance.
func SetLogger(logger echo.Logger) {
	defaultEcho.Logger = logger
}

// SetHTTPErrorHandler sets the HTTP error handler for the underlying Echo instance.
func SetHTTPErrorHandler(handler echo.HTTPErrorHandler) {
	defaultEcho.HTTPErrorHandler = handler
}

// SetBinder sets the binder for the underlying Echo instance.
func SetBinder(binder echo.Binder) {
	defaultEcho.Binder = binder
}

// SetJSONSerializer sets the JSON serializer for the underlying Echo instance.
func SetJSONSerializer(serializer echo.JSONSerializer) {
	defaultEcho.JSONSerializer = serializer
}

// SetValidator sets the validator for the underlying Echo instance.
func SetValidator(validator echo.Validator) {
	defaultEcho.Validator = validator
}

// SetDebug sets the debug mode for the underlying Echo instance.
func SetDebug(debug bool) {
	defaultEcho.Debug = debug
}

// SetDisableHTTP2 sets the disable HTTP/2 mode for the underlying Echo instance.
func SetDisableHTTP2(disable bool) {
	defaultEcho.DisableHTTP2 = disable
}

// SetHideBanner sets the hide banner mode for the underlying Echo instance.
func SetHideBanner(hide bool) {
	defaultEcho.HideBanner = hide
}

// SetHidePort sets the hide port mode for the underlying Echo instance.
func SetHidePort(hide bool) {
	defaultEcho.HidePort = hide
}
