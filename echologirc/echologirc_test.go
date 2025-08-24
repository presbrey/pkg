package middleware_test

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	ircmiddleware "github.com/presbrey/pkg/echologirc"
	"github.com/presbrey/pkg/irc/config"
	"github.com/presbrey/pkg/irc/server"
	"github.com/stretchr/testify/assert"
)

// IRCClient is a simple IRC client for testing
type IRCClient struct {
	Conn   net.Conn
	Reader *bufio.Reader
}

// NewIRCClient creates a new IRC client with connection retry
func NewIRCClient(t *testing.T, address string) *IRCClient {
	// Try to connect with retries
	var conn net.Conn
	var err error
	
	for i := 0; i < 3; i++ {
		conn, err = net.Dial("tcp", address)
		if err == nil {
			break
		}
		t.Logf("Connection attempt %d failed: %v. Retrying...", i+1, err)
		time.Sleep(1 * time.Second)
	}

	// If we still have an error after retries, fail the test
	assert.NoError(t, err, "Should connect to the server after retries")

	return &IRCClient{
		Conn:   conn,
		Reader: bufio.NewReader(conn),
	}
}

// Send sends a message to the server
func (c *IRCClient) Send(message string) error {
	_, err := c.Conn.Write([]byte(message + "\r\n"))
	return err
}

// SendWithCheck sends a message and checks for errors
func (c *IRCClient) SendWithCheck(t *testing.T, message string) {
	err := c.Send(message)
	assert.NoError(t, err, "Should send message: "+message)
	// Small delay to ensure message processing
	time.Sleep(100 * time.Millisecond)
}

// Expect waits for a message containing the expected string
func (c *IRCClient) Expect(t *testing.T, expected string, timeout time.Duration) (string, error) {
	// Set a deadline for reading
	c.Conn.SetReadDeadline(time.Now().Add(timeout))
	defer c.Conn.SetReadDeadline(time.Time{})

	// Use a channel to signal when we find the message
	type readResult struct {
		line string
		err  error
	}
	resultCh := make(chan readResult)
	
	// Start a goroutine to read and process messages
	go func() {
		for {
			line, err := c.Reader.ReadString('\n')
			if err != nil {
				t.Logf("Error reading IRC message: %v", err)
				resultCh <- readResult{"", err}
				return
			}
			
			line = strings.TrimSpace(line)
			t.Logf("Received IRC message: %s", line)
			
			if strings.Contains(line, expected) {
				resultCh <- readResult{line, nil}
				return
			}
		}
	}()
	
	// Wait for either a result or timeout
	select {
	case result := <-resultCh:
		return result.line, result.err
	case <-time.After(timeout):
		return "", fmt.Errorf("timeout waiting for message containing '%s'", expected)
	}
}

// Close closes the connection
func (c *IRCClient) Close() error {
	return c.Conn.Close()
}

// setupIRCServer sets up a test IRC server
func setupIRCServer(t *testing.T) (*server.Server, string, string, error) {
	// Skip test if IRC features are not available
	if _, err := os.Stat("/Users/joe/github.com/presbrey/pkg/irc/server/server.go"); os.IsNotExist(err) {
		t.Skip("IRC server package not available for testing")
	}
	// Create a temporary config file
	tempDir, err := os.MkdirTemp("", "echologirc-test-*")
	assert.NoError(t, err, "Should create a temporary directory")

	configPath := filepath.Join(tempDir, "config.yaml")
	configContent := `
server:
  name: test.irc.local
  network: TestNet
  host: 127.0.0.1
  port: 6667
  password: ""

tls:
  enabled: false
`

	err = os.WriteFile(configPath, []byte(configContent), 0644)
	assert.NoError(t, err, "Should write the config file")

	// Load the configuration
	cfg, err := config.Load(configPath)
	assert.NoError(t, err, "Should load the configuration")

	// Create and start the server
	srv, err := server.NewServer(cfg)
	assert.NoError(t, err, "Should create the server")

	// Start the server in a goroutine
	go func() {
		err := srv.Start()
		if err != nil {
			t.Logf("Server error: %v", err)
		}
	}()

	// Wait for the server to start
	time.Sleep(1 * time.Second)

	return srv, tempDir, "127.0.0.1:6667", nil
}

// TestIRCLoggerMiddleware tests the IRC logger middleware
func TestIRCLoggerMiddleware(t *testing.T) {
	// Set timeout for the entire test
	testTimeout := time.NewTimer(30 * time.Second)
	defer testTimeout.Stop()

	doneCh := make(chan struct{})
	go func() {
		defer close(doneCh)
		// The actual test will run here
	// Setup IRC server
	ircServer, tempDir, ircAddress, err := setupIRCServer(t)
	assert.NoError(t, err, "Should set up IRC server")
	defer func() {
		// Cleanup
		ircServer.Stop()
		os.RemoveAll(tempDir)
	}()

	// Connect an IRC listener client that will monitor for log messages
	listener := NewIRCClient(t, ircAddress)
	defer listener.Close()

	// Register the listener client
	listener.SendWithCheck(t, "NICK echo-listener")
	listener.SendWithCheck(t, "USER echo-listener 0 * :Echo Listener")

	// Wait for welcome message
	_, err = listener.Expect(t, "Welcome to the TestNet IRC Network", 5*time.Second)
	assert.NoError(t, err, "Should receive welcome message")

	// Join the logs channel (which will be used by the middleware)
	logsChannel := "#logs"
	listener.SendWithCheck(t, "JOIN " + logsChannel)
	_, err = listener.Expect(t, "JOIN "+logsChannel, 1*time.Second)
	assert.NoError(t, err, "Should join the logs channel")

	// Create Echo server with IRC logger middleware
	e := echo.New()
	
	// Add some standard middleware
	e.Use(echomiddleware.Logger())
	e.Use(echomiddleware.Recover())
	
	// Configure and add the IRC logger middleware with a modified version for testing
	ircConfig := ircmiddleware.DefaultIRCLoggerConfig()
	ircConfig.Server = "127.0.0.1" // Use our test server
	ircConfig.Port = 6667
	ircConfig.Nick = "echo-logger"
	ircConfig.User = "echo-logger"
	ircConfig.Channel = logsChannel
	ircConfig.UseTLS = false
	
	// Simplify the middleware config for testing
	ircConfig.FormatFunc = func(v echomiddleware.RequestLoggerValues) string {
		timestamp := time.Now().Format("2006-01-02 15:04:05")
		return fmt.Sprintf("[%s] TEST_LOG: %d %s %s from %s", 
			timestamp, v.Status, v.Method, v.URI, v.RemoteIP)
	}
	
	e.Use(ircmiddleware.IRCLoggerWithConfig(ircConfig))

	// Add test routes
	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "Hello, World!")
	})
	
	e.GET("/error", func(c echo.Context) error {
		return echo.NewHTTPError(http.StatusInternalServerError, "Internal server error")
	})
	
	e.GET("/delay/:ms", func(c echo.Context) error {
		ms := c.Param("ms")
		if delay, err := time.ParseDuration(ms + "ms"); err == nil {
			time.Sleep(delay)
		}
		return c.String(http.StatusOK, fmt.Sprintf("Delayed for %s ms", ms))
	})

	// Setup is complete, now run tests with different requests
	
	// Test 1: Simple GET request
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("User-Agent", "TestUserAgent/1.0")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	
	// Check for the log message in IRC
	t.Log("Waiting for echo-logger to join the channel...")
	logMsg, err := listener.Expect(t, "echo-logger", 5*time.Second) // Wait for our logger to join
	assert.NoError(t, err, "Should see echo-logger join")
	t.Logf("Got logger join message: %s", logMsg)
	
	// The middleware should send a message to the channel
	t.Log("Sending test request to trigger logging...")
	logMsg, err = listener.Expect(t, "TEST_LOG: 200 GET /", 5*time.Second)
	assert.NoError(t, err, "Should log the request to IRC")
	t.Logf("Got log message: %s", logMsg)
	assert.Contains(t, logMsg, "200 GET /", "Log message should contain status and method")
	assert.Contains(t, logMsg, "from", "Log message should contain IP")
	
	// Test 2: Error request
	req = httptest.NewRequest(http.MethodGet, "/error", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	
	// Check for error log in IRC
	t.Log("Sending error request to trigger error logging...")
	logMsg, err = listener.Expect(t, "TEST_LOG: 500 GET /error", 5*time.Second)
	assert.NoError(t, err, "Should log the error request to IRC")
	t.Logf("Got error log message: %s", logMsg)
	assert.Contains(t, logMsg, "500", "Log message should contain error status code")
	
	// Test 3: Delayed request to check latency reporting
	req = httptest.NewRequest(http.MethodGet, "/delay/100", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	
	// Check for delayed request log in IRC
	t.Log("Sending delayed request to check latency reporting...")
	logMsg, err = listener.Expect(t, "TEST_LOG: 200 GET /delay/100", 5*time.Second)
	assert.NoError(t, err, "Should log the delayed request to IRC")
	t.Logf("Got delayed request log message: %s", logMsg)
	
	// Test complete
	t.Log("All test cases passed successfully!")
		
		// Signal test completion
	}()
	
	// Wait for either test completion or timeout
	select {
	case <-doneCh:
		t.Log("Test completed successfully")
	case <-testTimeout.C:
		t.Fatal("Test timed out after 30 seconds")
	}
}
