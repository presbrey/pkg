// Package echo2gorilla provides conversion functions to transform
// Echo framework handlers and middleware to their Gorilla Mux equivalents.
package echo2gorilla

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"unicode"

	"github.com/gorilla/mux"
	"github.com/labstack/echo/v4"
	"github.com/presbrey/pkg/echovalidator"
)

// HandlerFunc converts an Echo handler function to a http.HandlerFunc that can be used with Gorilla Mux
func HandlerFunc(echoHandler echo.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Create a new Echo context
		echoCtx := &echoContext{
			request:        r,
			responseWriter: w,
			response:       &echo.Response{Writer: w},
			params:         make(map[string]string),
			store:          make(map[string]interface{}),
			binder:         &echo.DefaultBinder{},
		}

		// Extract path parameters from Gorilla context and add them to our echo context
		vars := mux.Vars(r)
		for k, v := range vars {
			echoCtx.params[k] = v
		}

		// Execute the Echo handler
		err := echoHandler(echoCtx)

		// Handle any errors returned from the Echo handler
		if err != nil {
			// Get the HTTP status code from the error if it's an Echo HTTPError
			if he, ok := err.(*echo.HTTPError); ok {
				w.WriteHeader(he.Code)
				// Write the error message to the response if it exists
				if he.Message != nil {
					w.Write([]byte(he.Error()))
				}
			} else {
				// Default to 500 Internal Server Error for non-Echo errors
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(err.Error()))
			}
		}
	}
}

// MiddlewareFunc converts an Echo middleware function to a Gorilla middleware function
func MiddlewareFunc(m echo.MiddlewareFunc) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Create a new Echo context
			c := &echoContext{
				request:        r,
				responseWriter: w,
				response:       &echo.Response{Writer: w},
				params:         make(map[string]string),
				store:          make(map[string]interface{}),
				binder:         &echo.DefaultBinder{},
			}

			// Extract path parameters from Gorilla mux
			vars := mux.Vars(r)
			if len(vars) > 0 {
				params := make([]string, 0, len(vars)*2)
				values := make([]string, 0, len(vars))

				for k, v := range vars {
					params = append(params, k)
					values = append(values, v)
				}

				c.paramNames = params
				c.paramValues = values
			}

			// Create a handler that will be called by the Echo middleware
			echoHandler := func(c echo.Context) error {
				// Pass control to the next handler in the chain
				next.ServeHTTP(w, r)
				return nil
			}

			// Execute the Echo middleware with our handler
			if err := m(echoHandler)(c); err != nil {
				// Handle any errors from the middleware
				if he, ok := err.(*echo.HTTPError); ok {
					w.WriteHeader(he.Code)
					if he.Message != nil {
						w.Write([]byte(he.Error()))
					}
				} else {
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte(err.Error()))
				}
			}
		})
	}
}

// echoContext is an implementation of echo.Context
type echoContext struct {
	request        *http.Request
	responseWriter http.ResponseWriter
	response       *echo.Response
	params         map[string]string
	path           string
	handler        echo.HandlerFunc
	store          map[string]interface{}
	paramNames     []string
	paramValues    []string
	binder         echo.Binder
	renderer       echo.Renderer
	logger         echo.Logger
}

// Request returns the http.Request object
func (c *echoContext) Request() *http.Request {
	return c.request
}

// SetRequest sets the http.Request object
func (c *echoContext) SetRequest(r *http.Request) {
	c.request = r
}

// Response returns the echo.Response object
func (c *echoContext) Response() *echo.Response {
	return c.response
}

// SetResponse sets the response
func (c *echoContext) SetResponse(r *echo.Response) {
	c.response = r
	c.responseWriter = r.Writer
}

// Param returns the path parameter value by name
func (c *echoContext) Param(name string) string {
	return c.params[name]
}

// ParamNames returns registered param names
func (c *echoContext) ParamNames() []string {
	// If paramNames is set, use it directly
	if len(c.paramNames) > 0 {
		return c.paramNames
	}

	// Otherwise, extract from the params map
	names := make([]string, 0, len(c.params))
	for name := range c.params {
		names = append(names, name)
	}
	return names
}

// ParamValues returns registered param values
func (c *echoContext) ParamValues() []string {
	// If paramValues is set, use it directly
	if len(c.paramValues) > 0 {
		return c.paramValues
	}

	// Otherwise, extract from the params map
	values := make([]string, 0, len(c.params))
	for _, value := range c.params {
		values = append(values, value)
	}
	return values
}

// QueryParam returns the query param for the provided name
func (c *echoContext) QueryParam(name string) string {
	return c.request.URL.Query().Get(name)
}

// QueryParams returns the query parameters as url.Values
func (c *echoContext) QueryParams() url.Values {
	return c.request.URL.Query()
}

// QueryString returns the URL query string
func (c *echoContext) QueryString() string {
	return c.request.URL.RawQuery
}

// FormValue returns the form field value
func (c *echoContext) FormValue(name string) string {
	return c.request.FormValue(name)
}

// FormParams returns the form parameters as url.Values
func (c *echoContext) FormParams() (url.Values, error) {
	if err := c.request.ParseForm(); err != nil {
		return nil, err
	}
	return c.request.Form, nil
}

// MultipartForm returns the multipart form
func (c *echoContext) MultipartForm() (*multipart.Form, error) {
	if err := c.request.ParseMultipartForm(32 << 20); err != nil {
		return nil, err
	}
	return c.request.MultipartForm, nil
}

// FormFile returns the multipart form file for the provided name
func (c *echoContext) FormFile(name string) (*multipart.FileHeader, error) {
	if err := c.request.ParseMultipartForm(32 << 20); err != nil {
		return nil, err
	}
	_, fileHeader, err := c.request.FormFile(name)
	return fileHeader, err
}

// Cookie returns the named cookie
func (c *echoContext) Cookie(name string) (*http.Cookie, error) {
	return c.request.Cookie(name)
}

// SetCookie adds a Set-Cookie header to the ResponseWriter
func (c *echoContext) SetCookie(cookie *http.Cookie) {
	http.SetCookie(c.responseWriter, cookie)
}

// Cookies returns the HTTP cookies sent with the request
func (c *echoContext) Cookies() []*http.Cookie {
	return c.request.Cookies()
}

// Get retrieves data from the context
func (c *echoContext) Get(key string) interface{} {
	return c.store[key]
}

// Set saves data in the context
func (c *echoContext) Set(key string, val interface{}) {
	if c.store == nil {
		c.store = make(map[string]interface{})
	}
	c.store[key] = val
}

// Bind binds the request body into provided type
func (c *echoContext) Bind(i interface{}) error {
	req := c.Request()
	if req.ContentLength == 0 {
		return nil
	}

	// Get content type
	contentType := req.Header.Get(echo.HeaderContentType)
	base, _, _ := strings.Cut(contentType, ";")
	mediatype := strings.TrimSpace(base)

	switch mediatype {
	case echo.MIMEApplicationJSON:
		if req.Body == nil {
			return echo.NewHTTPError(http.StatusBadRequest, "Request body is empty")
		}
		defer req.Body.Close()
		if err := json.NewDecoder(req.Body).Decode(i); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}
	case echo.MIMEApplicationXML, echo.MIMETextXML:
		if req.Body == nil {
			return echo.NewHTTPError(http.StatusBadRequest, "Request body is empty")
		}
		defer req.Body.Close()
		if err := xml.NewDecoder(req.Body).Decode(i); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}
	case echo.MIMEApplicationForm:
		if err := req.ParseForm(); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}
		return bindData(i, req.Form, "form")
	case echo.MIMEMultipartForm:
		if err := req.ParseMultipartForm(32 << 20); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}
		return bindData(i, req.Form, "form")
	default:
		return echo.NewHTTPError(http.StatusUnsupportedMediaType, "Unsupported media type")
	}

	return nil
}

// bindData binds form data to a struct
func bindData(ptr interface{}, data map[string][]string, tag string) error {
	typ := reflect.TypeOf(ptr).Elem()
	val := reflect.ValueOf(ptr).Elem()

	if typ.Kind() != reflect.Struct {
		return echo.NewHTTPError(http.StatusBadRequest, "binding element must be a struct")
	}

	for i := 0; i < typ.NumField(); i++ {
		typeField := typ.Field(i)
		structField := val.Field(i)
		if !structField.CanSet() {
			continue
		}

		structFieldKind := structField.Kind()
		inputFieldName := typeField.Tag.Get(tag)
		if inputFieldName == "" {
			inputFieldName = typeField.Name
			// If tag is nil, we lowercase the first letter of the field name
			runes := []rune(inputFieldName)
			runes[0] = unicode.ToLower(runes[0])
			inputFieldName = string(runes)
		}

		inputValue, exists := data[inputFieldName]
		if !exists {
			continue
		}

		numElems := len(inputValue)
		if structFieldKind == reflect.Slice && numElems > 0 {
			sliceOf := structField.Type().Elem().Kind()
			slice := reflect.MakeSlice(structField.Type(), numElems, numElems)
			for j := 0; j < numElems; j++ {
				if err := setWithProperType(sliceOf, inputValue[j], slice.Index(j)); err != nil {
					return echo.NewHTTPError(http.StatusBadRequest, err.Error())
				}
			}
			val.Field(i).Set(slice)
		} else if numElems > 0 {
			if err := setWithProperType(structFieldKind, inputValue[0], structField); err != nil {
				return echo.NewHTTPError(http.StatusBadRequest, err.Error())
			}
		}
	}
	return nil
}

// setWithProperType sets the value with the proper type
func setWithProperType(valueKind reflect.Kind, val string, structField reflect.Value) error {
	switch valueKind {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return setIntField(val, structField)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return setUintField(val, structField)
	case reflect.Bool:
		return setBoolField(val, structField)
	case reflect.Float32, reflect.Float64:
		return setFloatField(val, structField)
	case reflect.String:
		structField.SetString(val)
	}
	return nil
}

// setIntField sets an int field
func setIntField(val string, field reflect.Value) error {
	if val == "" {
		val = "0"
	}
	intVal, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return err
	}
	field.SetInt(intVal)
	return nil
}

// setUintField sets a uint field
func setUintField(val string, field reflect.Value) error {
	if val == "" {
		val = "0"
	}
	uintVal, err := strconv.ParseUint(val, 10, 64)
	if err != nil {
		return err
	}
	field.SetUint(uintVal)
	return nil
}

// setBoolField sets a bool field
func setBoolField(val string, field reflect.Value) error {
	if val == "" {
		val = "false"
	}
	boolVal, err := strconv.ParseBool(val)
	if err != nil {
		return err
	}
	field.SetBool(boolVal)
	return nil
}

// setFloatField sets a float field
func setFloatField(val string, field reflect.Value) error {
	if val == "" {
		val = "0.0"
	}
	floatVal, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return err
	}
	field.SetFloat(floatVal)
	return nil
}

// Stream sends a streaming response with status code and content type
func (c *echoContext) Stream(code int, contentType string, r io.Reader) error {
	c.responseWriter.Header().Set(echo.HeaderContentType, contentType)
	c.responseWriter.WriteHeader(code)
	_, err := io.Copy(c.responseWriter, r)
	return err
}

// HTML sends an HTTP response with status code
func (c *echoContext) HTML(code int, html string) error {
	c.responseWriter.Header().Set(echo.HeaderContentType, echo.MIMETextHTMLCharsetUTF8)
	c.responseWriter.WriteHeader(code)
	_, err := c.responseWriter.Write([]byte(html))
	return err
}

// HTMLBlob sends an HTTP blob response with status code
func (c *echoContext) HTMLBlob(code int, b []byte) error {
	c.responseWriter.Header().Set(echo.HeaderContentType, echo.MIMETextHTMLCharsetUTF8)
	c.responseWriter.WriteHeader(code)
	_, err := c.responseWriter.Write(b)
	return err
}

// JSON sends a JSON response with status code
func (c *echoContext) JSON(code int, i interface{}) error {
	c.responseWriter.Header().Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	c.responseWriter.WriteHeader(code)
	return json.NewEncoder(c.responseWriter).Encode(i)
}

// JSONPretty sends a pretty-print JSON with status code
func (c *echoContext) JSONPretty(code int, i interface{}, indent string) error {
	c.responseWriter.Header().Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	c.responseWriter.WriteHeader(code)
	enc := json.NewEncoder(c.responseWriter)
	enc.SetIndent("", indent)
	return enc.Encode(i)
}

// JSONBlob sends a JSON blob response with status code
func (c *echoContext) JSONBlob(code int, b []byte) error {
	c.responseWriter.Header().Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	c.responseWriter.WriteHeader(code)
	_, err := c.responseWriter.Write(b)
	return err
}

// JSONP sends a JSONP response with status code
func (c *echoContext) JSONP(code int, callback string, i interface{}) error {
	c.responseWriter.Header().Set(echo.HeaderContentType, echo.MIMEApplicationJavaScriptCharsetUTF8)
	c.responseWriter.WriteHeader(code)

	// Write the callback function name
	if _, err := c.responseWriter.Write([]byte(callback + "(")); err != nil {
		return err
	}

	// Encode the data as JSON
	if err := json.NewEncoder(c.responseWriter).Encode(i); err != nil {
		return err
	}

	// The encoder adds a newline, so we need to remove it and add the closing parenthesis
	if _, err := c.responseWriter.Write([]byte(");")); err != nil {
		return err
	}

	return nil
}

// JSONPBlob sends a JSONP blob response with status code
func (c *echoContext) JSONPBlob(code int, callback string, b []byte) error {
	c.responseWriter.Header().Set(echo.HeaderContentType, echo.MIMEApplicationJavaScriptCharsetUTF8)
	c.responseWriter.WriteHeader(code)

	// Write the callback function name and the JSON data (which is already encoded)
	_, err := fmt.Fprintf(c.responseWriter, "%s(%s);", callback, b)
	return err
}

// XML sends an XML response with status code
func (c *echoContext) XML(code int, i interface{}) error {
	c.responseWriter.Header().Set(echo.HeaderContentType, echo.MIMEApplicationXMLCharsetUTF8)
	c.responseWriter.WriteHeader(code)

	// Add the XML header
	if _, err := c.responseWriter.Write([]byte(xml.Header)); err != nil {
		return err
	}

	return xml.NewEncoder(c.responseWriter).Encode(i)
}

// XMLPretty sends a pretty-print XML with status code
func (c *echoContext) XMLPretty(code int, i interface{}, indent string) error {
	c.responseWriter.Header().Set(echo.HeaderContentType, echo.MIMEApplicationXMLCharsetUTF8)
	c.responseWriter.WriteHeader(code)

	// Add the XML header
	if _, err := c.responseWriter.Write([]byte(xml.Header)); err != nil {
		return err
	}

	// Marshal the data with indentation
	output, err := xml.MarshalIndent(i, "", indent)
	if err != nil {
		return err
	}

	// Write the indented XML
	_, err = c.responseWriter.Write(output)
	return err
}

// XMLBlob sends an XML blob response with status code
func (c *echoContext) XMLBlob(code int, b []byte) error {
	c.responseWriter.Header().Set(echo.HeaderContentType, echo.MIMEApplicationXMLCharsetUTF8)
	c.responseWriter.WriteHeader(code)
	_, err := c.responseWriter.Write(b)
	return err
}

// Validate validates provided value using the echovalidator package
func (c *echoContext) Validate(i interface{}) error {
	// Use the singleton validator from echovalidator package
	return echovalidator.Default().Validate(i)
}

// Path returns the registered path for the handler
func (c *echoContext) Path() string {
	return c.path
}

// SetPath sets the registered path for the handler
func (c *echoContext) SetPath(p string) {
	c.path = p
}

// Handler returns the matched handler by router
func (c *echoContext) Handler() echo.HandlerFunc {
	return c.handler
}

// SetHandler sets the matched handler by router
func (c *echoContext) SetHandler(h echo.HandlerFunc) {
	c.handler = h
}

// SetParamNames sets path parameter names
func (c *echoContext) SetParamNames(names ...string) {
	c.paramNames = names
}

// SetParamValues sets path parameter values
func (c *echoContext) SetParamValues(values ...string) {
	c.paramValues = values

	// Update the params map to keep it in sync
	c.params = make(map[string]string)
	for i, name := range c.paramNames {
		if i < len(values) {
			c.params[name] = values[i]
		}
	}
}

// IsTLS returns true if HTTP connection is TLS otherwise false
func (c *echoContext) IsTLS() bool {
	return c.request.TLS != nil
}

// IsWebSocket returns true if HTTP connection is WebSocket otherwise false
func (c *echoContext) IsWebSocket() bool {
	upgrade := c.request.Header.Get(echo.HeaderUpgrade)
	return upgrade == "websocket" || upgrade == "Websocket"
}

// Scheme returns the HTTP protocol scheme, `http` or `https`
func (c *echoContext) Scheme() string {
	// Can't use `r.URL.Scheme` as it's not set by the Go HTTP server
	if c.IsTLS() {
		return "https"
	}
	if scheme := c.request.Header.Get(echo.HeaderXForwardedProto); scheme != "" {
		return scheme
	}
	if scheme := c.request.Header.Get(echo.HeaderXForwardedProtocol); scheme != "" {
		return scheme
	}
	if ssl := c.request.Header.Get(echo.HeaderXForwardedSsl); ssl == "on" {
		return "https"
	}
	if scheme := c.request.Header.Get(echo.HeaderXUrlScheme); scheme != "" {
		return scheme
	}
	return "http"
}

// RealIP returns the client's network address based on `X-Forwarded-For` or `X-Real-IP` request header
func (c *echoContext) RealIP() string {
	ra := c.request.RemoteAddr
	if ip := c.request.Header.Get(echo.HeaderXForwardedFor); ip != "" {
		ra = ip
	} else if ip := c.request.Header.Get(echo.HeaderXRealIP); ip != "" {
		ra = ip
	} else {
		ra, _, _ = net.SplitHostPort(ra)
	}
	return ra
}

// Logger returns the `Logger` instance
func (c *echoContext) Logger() echo.Logger {
	// If we have a logger set directly on this context, return it
	if c.logger != nil {
		return c.logger
	}

	// Otherwise, return a no-op logger
	return echo.New().Logger
}

// SetLogger sets the logger
func (c *echoContext) SetLogger(l echo.Logger) {
	c.logger = l
}

// Echo returns the `Echo` instance
func (c *echoContext) Echo() *echo.Echo {
	// We don't have an Echo instance in this adapter
	return nil
}

// Reset resets the context after request completes
func (c *echoContext) Reset(r *http.Request, w http.ResponseWriter) {
	c.request = r
	c.responseWriter = w
	c.response = &echo.Response{Writer: w}
	c.params = make(map[string]string)
	c.store = make(map[string]interface{})
	c.path = ""
	c.handler = nil
	c.paramNames = make([]string, 0)
	c.paramValues = make([]string, 0)
}

// Render renders a template with data and sends a text/html response with status code
func (c *echoContext) Render(code int, name string, data interface{}) error {
	if c.renderer == nil {
		return echo.ErrRendererNotRegistered
	}

	buf := new(bytes.Buffer)
	if err := c.renderer.Render(buf, name, data, c); err != nil {
		return err
	}

	return c.HTMLBlob(code, buf.Bytes())
}

// Error invokes the registered HTTP error handler
func (c *echoContext) Error(err error) {
	// Default error handling
	if he, ok := err.(*echo.HTTPError); ok {
		if he.Internal != nil {
			if herr, ok := he.Internal.(*echo.HTTPError); ok {
				he = herr
			}
		}
		// Send HTTP error status and message
		c.JSON(he.Code, map[string]interface{}{
			"error": he.Message,
		})
	} else {
		// For non-HTTPError types, return a 500 internal server error
		c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": err.Error(),
		})
	}
}

// NoContent sends a response with no body and a status code
func (c *echoContext) NoContent(code int) error {
	c.responseWriter.WriteHeader(code)
	return nil
}

// Blob sends a blob response with content type
func (c *echoContext) Blob(code int, contentType string, b []byte) error {
	c.responseWriter.Header().Set(echo.HeaderContentType, contentType)
	c.responseWriter.WriteHeader(code)
	_, err := c.responseWriter.Write(b)
	return err
}

// File sends a response with the content of the file
func (c *echoContext) File(file string) error {
	http.ServeFile(c.responseWriter, c.request, file)
	return nil
}

// Attachment sends a response as attachment, prompting client to save the file
func (c *echoContext) Attachment(file string, name string) error {
	c.responseWriter.Header().Set(echo.HeaderContentDisposition, fmt.Sprintf("attachment; filename=%q", name))
	return c.File(file)
}

// Inline sends a response as inline, opening the file in the browser
func (c *echoContext) Inline(file string, name string) error {
	c.responseWriter.Header().Set(echo.HeaderContentDisposition, fmt.Sprintf("inline; filename=%q", name))
	return c.File(file)
}

// String sends a string response
func (c *echoContext) String(code int, s string) error {
	c.responseWriter.Header().Set(echo.HeaderContentType, echo.MIMETextPlainCharsetUTF8)
	c.responseWriter.WriteHeader(code)
	_, err := c.responseWriter.Write([]byte(s))
	return err
}

// Redirect redirects the request to a provided URL with status code
func (c *echoContext) Redirect(code int, url string) error {
	http.Redirect(c.responseWriter, c.request, url, code)
	return nil
}
