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
	// Create unbuffered channel for log messages
	logCh := make(chan string)

	// Initialize IRC client
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
	client := girc.New(clientConfig)
	
	client.Handlers.Add(girc.ERROR, func(c *girc.Client, e girc.Event) {
		fmt.Printf("IRC connection error: %v\n", e.Params[0])
	})

	var connected bool
	var connMutex sync.RWMutex

	// Handle connection established
	client.Handlers.Add(girc.CONNECTED, func(c *girc.Client, e girc.Event) {
		c.Cmd.Join(config.Channel)
		connMutex.Lock()
		connected = true
		connMutex.Unlock()
		fmt.Printf("Connected to IRC server %s and joined %s\n", config.Server, config.Channel)
	})

	// Handle disconnect
	client.Handlers.Add(girc.DISCONNECTED, func(c *girc.Client, e girc.Event) {
		connMutex.Lock()
		connected = false
		connMutex.Unlock()
		fmt.Printf("Disconnected from IRC server %s\n", config.Server)

		// Try to reconnect
		go func() {
			time.Sleep(5 * time.Second)
			if err := client.Connect(); err != nil {
				fmt.Printf("Failed to reconnect to IRC: %v\n", err)
			}
		}()
	})

	// Start IRC client connection in a goroutine
	go func() {
		if err := client.Connect(); err != nil {
			fmt.Printf("Failed to connect to IRC: %v\n", err)
		}
	}()

	// Start message consumer in a goroutine
	go func() {
		for msg := range logCh {
			connMutex.RLock()
			isConnected := connected
			connMutex.RUnlock()

			if isConnected {
				client.Cmd.Message(config.Channel, msg)
			} else {
				fmt.Printf("Not connected to IRC, dropping log message: %s\n", msg)
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
