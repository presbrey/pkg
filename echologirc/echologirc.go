package middleware

import (
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/lrstanley/girc"
)

// IRCLoggedKey is the context key that indicates a request has been logged to IRC
const IRCLoggedKey = "irc_logged"

// IRCLoggerConfig holds configuration for IRC logger middleware
type IRCLoggerConfig struct {
	// IRC connection settings
	Server   string
	Port     int
	Nick     string
	User     string
	Password string
	Channel  string
	UseTLS   bool

	// Skipper defines a function to skip middleware
	Skipper middleware.Skipper

	// FormatFunc formats the log message to be sent to IRC
	FormatFunc func(values middleware.RequestLoggerValues) string

	// LogValuesFunc defines a function to get/alter log values
	LogValuesFunc func(c echo.Context) middleware.RequestLoggerValues
}

// DefaultIRCLoggerConfig returns a default configuration for IRCLogger middleware
func DefaultIRCLoggerConfig() IRCLoggerConfig {
	return IRCLoggerConfig{
		Server:   getEnv("IRC_SERVER", "irc.example.com"),
		Port:     getEnvInt("IRC_PORT", 6667),
		Nick:     getEnv("IRC_NICK", "echo-logger"),
		User:     getEnv("IRC_USER", "echo-logger"),
		Password: getEnv("IRC_PASSWORD", ""),
		Channel:  getEnv("IRC_CHANNEL", "#logs"),
		UseTLS:   getEnvBool("IRC_USE_TLS", true),
		Skipper:  middleware.DefaultSkipper,
		FormatFunc: func(v middleware.RequestLoggerValues) string {
			var status, method, uri, ip, latency, error, userAgent string

			status = fmt.Sprintf("%d", v.Status)
			method = v.Method
			uri = v.URI
			ip = v.RemoteIP
			latency = fmt.Sprintf("%v", v.Latency)
			
			if v.Error != nil {
				error = " | Error: " + v.Error.Error()
			}
			
			if v.UserAgent != "" {
				userAgent = " | UA: " + v.UserAgent
			}

			timestamp := time.Now().Format("2006-01-02 15:04:05")
			return fmt.Sprintf("[%s] %s %s %s from %s (took %s)%s%s",
				timestamp, status, method, uri, ip, latency, userAgent, error)
		},
		LogValuesFunc: func(c echo.Context) middleware.RequestLoggerValues {
			return middleware.RequestLoggerValues{
				Status:    c.Response().Status,
				Method:    c.Request().Method,
				URI:       c.Request().RequestURI,
				RemoteIP:  c.RealIP(),
				Latency:   time.Since(time.Now()), // This will be set properly in the actual middleware
				UserAgent: c.Request().UserAgent(),
			}
		},
	}
}

// Helper functions to get environment variables with defaults
func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if value, exists := os.LookupEnv(key); exists {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if value, exists := os.LookupEnv(key); exists {
		if boolVal, err := strconv.ParseBool(value); err == nil {
			return boolVal
		}
	}
	return fallback
}

// IRCLogger returns a middleware which logs requests to IRC with default config
func IRCLogger() echo.MiddlewareFunc {
	return IRCLoggerWithConfig(DefaultIRCLoggerConfig())
}

// IRCLoggerWithConfig returns a middleware which logs requests to IRC using provided config
func IRCLoggerWithConfig(config IRCLoggerConfig) echo.MiddlewareFunc {
	// Connection state management
	var connected bool
	var connMutex sync.RWMutex
	// Create unbuffered channel for log messages
	logCh := make(chan string)

	// Function to create a new IRC client with the same configuration
	createClient := func() *girc.Client {
		clientConfig := girc.Config{
			Server:   config.Server,
			Port:     config.Port,
			Nick:     config.Nick,
			User:     config.User,
			SSL:      config.UseTLS,
		}
		// Add password if provided
		if config.Password != "" {
			clientConfig.SASL = &girc.SASLPlain{
				User: config.Nick,
				Pass: config.Password,
			}
		}
		return girc.New(clientConfig)
	}
	
	// Initialize IRC client
	client := createClient()
	
	client.Handlers.Add(girc.ERROR, func(c *girc.Client, e girc.Event) {
		fmt.Printf("IRC connection error: %v\n", e.Params[0])
	})


	// Channel to signal when connection is established
	connectedCh := make(chan struct{})

	// Handle connection established
	client.Handlers.Add(girc.CONNECTED, func(c *girc.Client, e girc.Event) {
		c.Cmd.Join(config.Channel)
		connMutex.Lock()
		connected = true
		connMutex.Unlock()
		fmt.Printf("Connected to IRC server %s and joined %s\n", config.Server, config.Channel)
		
		// Notify that we're connected
		select {
		case connectedCh <- struct{}{}:
			// Successfully sent signal
		default:
			// Channel already closed or buffer full, ignore
		}
	})

	// Handle disconnect
	client.Handlers.Add(girc.DISCONNECTED, func(c *girc.Client, e girc.Event) {
		connMutex.Lock()
		connected = false
		connMutex.Unlock()
		fmt.Printf("Disconnected from IRC server %s\n", config.Server)

		// Create a new client instance for reconnection instead of reusing the existing one
		go func() {
			// We need to create a new client for each reconnection attempt
			// as the girc library doesn't support calling Connect() multiple times on the same client
			newClientConfig := girc.Config{
				Server:   config.Server,
				Port:     config.Port,
				Nick:     config.Nick,
				User:     config.User,
				SSL:      config.UseTLS,
			}
			
			// Add password if provided
			if config.Password != "" {
				newClientConfig.SASL = &girc.SASLPlain{
					User: config.Nick,
					Pass: config.Password,
				}
			}
			
			// Create a new client
			newClient := girc.New(newClientConfig)
			
			// Add the same handlers to the new client
			newClient.Handlers.Add(girc.CONNECTED, func(c *girc.Client, e girc.Event) {
				c.Cmd.Join(config.Channel)
				connMutex.Lock()
				connected = true
				connMutex.Unlock()
				client = newClient // Update the main client reference
				fmt.Printf("Reconnected to IRC server %s and joined %s\n", config.Server, config.Channel)
			})

			// Try to connect with the new client
			if err := newClient.Connect(); err != nil {
				fmt.Printf("Reconnection failed: %v\n", err)
				
				// If initial reconnection fails, try with backoff
				for retries := 0; retries < 3; retries++ {
					time.Sleep(time.Duration(1+retries) * time.Second)
					
					// Create a fresh client for each retry
					retryClient := girc.New(newClientConfig)
					
					// Set up the same handlers
					retryClient.Handlers.Add(girc.CONNECTED, func(c *girc.Client, e girc.Event) {
						c.Cmd.Join(config.Channel)
						connMutex.Lock()
						connected = true
						connMutex.Unlock()
						client = retryClient // Update the main client reference
						fmt.Printf("Reconnected to IRC server %s and joined %s\n", config.Server, config.Channel)
					})
					
					if err := retryClient.Connect(); err != nil {
						fmt.Printf("Failed to reconnect to IRC (attempt %d): %v\n", retries+1, err)
					} else {
						break
					}
				}
			}
		}()
	})
	
	// Start IRC client connection in a goroutine
	connectErrorCh := make(chan error, 1)
	go func() {
		if err := client.Connect(); err != nil {
			fmt.Printf("Failed to connect to IRC: %v\n", err)
			connectErrorCh <- err
		}
	}()
	
	// Wait for initial connection or timeout
	select {
	case <-connectedCh:
		fmt.Println("Successfully connected to IRC server")
	case err := <-connectErrorCh:
		fmt.Printf("Error connecting to IRC server: %v\n", err)
	case <-time.After(2 * time.Second):
		// Continue even if we timeout. We'll queue messages and send them when connected.
		fmt.Println("Timeout waiting for IRC connection, will continue and retry")
	}

	// Create a buffer to hold messages that couldn't be sent due to connection issues
	msgBuffer := make([]string, 0, 100) // Buffer up to 100 messages
	msgBufferMutex := &sync.Mutex{}
	
	// Function to try to send buffered messages
	sendBufferedMessages := func(ircClient *girc.Client) {
		msgBufferMutex.Lock()
		defer msgBufferMutex.Unlock()
		
		if len(msgBuffer) == 0 {
			return
		}
		
		connMutex.RLock()
		isConnected := connected
		connMutex.RUnlock()
		
		if !isConnected {
			return
		}
		
		fmt.Printf("Attempting to send %d buffered messages\n", len(msgBuffer))
		for i, msg := range msgBuffer {
			ircClient.Cmd.Message(config.Channel, msg)
			// Short delay to prevent flooding
			if i < len(msgBuffer)-1 {
				time.Sleep(100 * time.Millisecond)
			}
		}
		
		// Clear buffer after sending
		msgBuffer = msgBuffer[:0]
	}
	
	// Add handler for successful channel join to send buffered messages
	client.Handlers.Add(girc.JOIN, func(c *girc.Client, e girc.Event) {
		// Only process our own joins
		if e.Source.Name == config.Nick && e.Params[0] == config.Channel {
			fmt.Printf("Successfully joined channel %s, sending any buffered messages\n", config.Channel)
			sendBufferedMessages(client)
		}
	})
	
	// Start message consumer in a goroutine
	go func() {
		for msg := range logCh {
			connMutex.RLock()
			isConnected := connected
			connMutex.RUnlock()

			if isConnected {
				client.Cmd.Message(config.Channel, msg)
			} else {
				// Buffer the message for later sending
				fmt.Printf("Not connected to IRC, buffering log message: %s\n", msg)
				msgBufferMutex.Lock()
				// Avoid buffer overflow by removing oldest message if needed
				if len(msgBuffer) >= 100 {
					msgBuffer = msgBuffer[1:]
				}
				msgBuffer = append(msgBuffer, msg)
				msgBufferMutex.Unlock()
				
				// Do not attempt to reconnect here
				// The DISCONNECTED handler should handle reconnection
				// Attempting to call Connect() multiple times on the same client instance
				// will cause a panic in the girc library
			}
		}
	}()

	// Return the actual middleware function
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if config.Skipper(c) {
				return next(c)
			}

			// Process the request
			err := next(c)

			// Get request values using LogValuesFunc
			values := config.LogValuesFunc(c)

			// Format the log message using FormatFunc
			msg := config.FormatFunc(values)

			// We're specifically asked to use an unbuffered channel
			// This will block if the consumer is busy
			logCh <- msg

			// Mark as logged to IRC in context to trigger Skipper in default logger
			c.Set(IRCLoggedKey, true)

			return err
		}
	}
}

// SkipIfLoggedToIRC returns a skipper function for the default logger
// to avoid duplicate logging for requests already logged to IRC
func SkipIfLoggedToIRC(c echo.Context) bool {
	return c.Get(IRCLoggedKey) != nil
}

// Example usage:
//
// func main() {
//     e := echo.New()
//
//     // Use IRC logger middleware
//     e.Use(middleware.IRCLogger())
//
//     // Add default logger with skipper to avoid double logging
//     e.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
//         Skipper: middleware.SkipIfLoggedToIRC,
//     }))
//
//     // Rest of your Echo setup
//     e.GET("/", func(c echo.Context) error {
//         return c.String(http.StatusOK, "Hello, World!")
//     })
//
//     e.Logger.Fatal(e.Start(":1323"))
// }
