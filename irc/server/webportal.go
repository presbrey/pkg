package server

import (
	"crypto/subtle"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"embed"

	"github.com/labstack/echo/v4"
	"github.com/presbrey/pkg/cdns"
	"github.com/presbrey/pkg/irc/config"
)

// WebPortal represents the web portal for IRC operators
type WebPortal struct {
	server   *Server
	config   *config.Config
	echo     *echo.Echo
	sessions map[string]*WebSession
}

// WebSession represents a web session
type WebSession struct {
	Username  string
	ExpiresAt time.Time
}

// Template is a renderer for Echo that uses html/template
type Template struct {
	templates *template.Template
}

//go:embed views
var templateFS embed.FS

// Render renders a template with data
func (t *Template) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.templates.ExecuteTemplate(w, name, data)
}

// NewWebPortal creates a new web portal
func NewWebPortal(server *Server, cfg *config.Config) (*WebPortal, error) {
	// Create new Echo instance
	e := echo.New()
	e.HideBanner = true

	// Apply default CDN middleware
	cdns.UseDefaults(e)

	// Configure renderer
	t := &Template{
		templates: template.Must(template.ParseFS(templateFS, "views/*.html")),
	}
	e.Renderer = t

	portal := &WebPortal{
		server:   server,
		config:   cfg,
		echo:     e,
		sessions: make(map[string]*WebSession),
	}

	// Setup routes
	portal.setupRoutes()

	return portal, nil
}

// Start starts the web portal
func (w *WebPortal) Start() error {
	// Start the Echo server
	if w.config.WebPortal.TLS {
		return w.echo.StartTLS(w.config.GetWebListenAddress(), w.config.ListenTLS.Cert, w.config.ListenTLS.Key)
	}
	return w.echo.Start(w.config.GetWebListenAddress())
}

// Stop stops the web portal
func (w *WebPortal) Stop() error {
	log.Println("Stopping web portal")
	return w.echo.Shutdown(nil)
}

// setupRoutes sets up the Echo routes
func (w *WebPortal) setupRoutes() {
	// Static files
	w.echo.Static("/static", "static")

	// Front-end routes
	w.echo.GET("/", w.handleIndex)
	w.echo.GET("/login", w.handleLogin)
	w.echo.GET("/logout", w.handleLogout)
	w.echo.GET("/dashboard", w.handleDashboard)
	w.echo.GET("/channels", w.handleChannels)
	w.echo.GET("/users", w.handleUsers)
	w.echo.GET("/rehash", w.handleRehash)

	// API routes
	api := w.echo.Group("/api")
	api.POST("/login", w.handleAPILogin)
	api.GET("/token", w.handleAPIToken)
	api.GET("/stats", w.handleAPIStats)
	api.GET("/channels", w.handleAPIChannels)
	api.GET("/users", w.handleAPIUsers)
	api.POST("/kick", w.handleAPIKick)
	api.POST("/kill", w.handleAPIKill)
	api.POST("/mode", w.handleAPIMode)
	api.POST("/rehash", w.handleAPIRehash)
}

// Note: Static files are now handled by Echo's Static middleware

// handleIndex handles the index page
func (w *WebPortal) handleIndex(c echo.Context) error {
	// Check if the user is logged in
	session, _ := w.getSession(c.Request())
	if session != nil {
		return c.Redirect(http.StatusFound, "/dashboard")
	}

	// Show the login page
	return c.File("templates/index.html")
}

// handleLogin handles the login page (GET)
func (w *WebPortal) handleLogin(c echo.Context) error {
	// Check if this is a token login
	token := c.QueryParam("token")
	username := c.QueryParam("username")

	if token != "" && username != "" {
		// Validate the token
		operator := w.server.GetOperator(username)
		if operator != nil && operator.ValidateMagicToken(token) {
			// Create a session
			session := &WebSession{
				Username:  username,
				ExpiresAt: time.Now().Add(24 * time.Hour),
			}

			// Generate a session ID
			sessionID := fmt.Sprintf("%s-%d", username, time.Now().UnixNano())
			w.sessions[sessionID] = session

			// Set a cookie
			c.SetCookie(&http.Cookie{
				Name:     "session",
				Value:    sessionID,
				Expires:  session.ExpiresAt,
				HttpOnly: true,
				Path:     "/",
			})

			// Update last login
			operator.UpdateLastLogin()

			// Redirect to dashboard
			return c.Redirect(http.StatusFound, "/dashboard")
		}
	}

	// Show the login page
	return c.File("templates/login.html")
}

// handleLogout handles logging out
func (w *WebPortal) handleLogout(c echo.Context) error {
	// Get the session
	cookie, err := c.Cookie("session")
	if err == nil {
		// Delete the session
		delete(w.sessions, cookie.Value)

		// Clear the cookie
		c.SetCookie(&http.Cookie{
			Name:    "session",
			Value:   "",
			Expires: time.Unix(0, 0),
			Path:    "/",
		})
	}

	// Redirect to the login page
	return c.Redirect(http.StatusFound, "/")
}

// handleDashboard handles the dashboard page
func (w *WebPortal) handleDashboard(c echo.Context) error {
	// Check if the user is logged in
	session, _ := w.getSession(c.Request())
	if session == nil {
		return c.Redirect(http.StatusFound, "/login")
	}

	// Get stats
	stats := map[string]interface{}{
		"server":   w.server.GetConfig().Server.Name,
		"network":  w.server.GetConfig().Server.Network,
		"uptime":   w.server.GetUptime().String(),
		"clients":  w.server.ClientCount(),
		"channels": w.server.ChannelCount(),
		"username": session.Username,
	}

	// Show the dashboard
	return c.Render(http.StatusOK, "dashboard.html", stats)
}

// handleChannels handles the channels page
func (w *WebPortal) handleChannels(c echo.Context) error {
	// Check if the user is logged in
	session, _ := w.getSession(c.Request())
	if session == nil {
		return c.Redirect(http.StatusFound, "/login")
	}

	// Get channels
	w.server.mu.RLock()
	channels := make([]map[string]interface{}, 0)
	w.server.channels.Range(func(key, value interface{}) bool {
		name := key.(string)
		channel := value.(*Channel)
		channels = append(channels, map[string]interface{}{
			"name":  name,
			"topic": channel.Topic,
			"users": channel.MemberCount(),
			"modes": channel.GetModeString(),
		})
		return true
	})
	w.server.mu.RUnlock()

	// Show the channels page
	return c.Render(http.StatusOK, "channels.html", map[string]interface{}{
		"channels": channels,
		"username": session.Username,
	})
}

// handleUsers handles the users page
func (w *WebPortal) handleUsers(c echo.Context) error {
	// Check if the user is logged in
	session, _ := w.getSession(c.Request())
	if session == nil {
		return c.Redirect(http.StatusFound, "/login")
	}

	// Get users
	w.server.mu.RLock()
	users := make([]map[string]interface{}, 0)
	w.server.clients.Range(func(_, value interface{}) bool {
		client := value.(*Client)
		users = append(users, map[string]interface{}{
			"nickname":  client.Nickname,
			"username":  client.Username,
			"hostname":  client.Hostname,
			"ip":        client.IP,
			"modes":     client.Modes.GetModeString(),
			"channels":  len(client.Channels),
			"connected": time.Since(client.LastPing).String(),
		})
		return true
	})
	w.server.mu.RUnlock()

	// Show the users page
	return c.Render(http.StatusOK, "users.html", map[string]interface{}{
		"users":    users,
		"username": session.Username,
	})
}

// handleRehash handles the rehash page
func (w *WebPortal) handleRehash(c echo.Context) error {
	// Check if the user is logged in
	session, _ := w.getSession(c.Request())
	if session == nil {
		return c.Redirect(http.StatusFound, "/login")
	}

	// Show the rehash page
	return c.Render(http.StatusOK, "rehash.html", map[string]interface{}{
		"username": session.Username,
		"config":   w.server.GetConfig().Source,
	})
}

// API Handlers

// handleAPILogin handles the login API
func (w *WebPortal) handleAPILogin(c echo.Context) error {
	// Only allow POST
	if c.Request().Method != http.MethodPost {
		return echo.ErrMethodNotAllowed
	}

	// Parse the request
	err := c.Request().ParseForm()
	if err != nil {
		return echo.ErrBadRequest
	}

	username := c.FormValue("username")
	password := c.FormValue("password")

	// Validate the credentials
	operator := w.server.GetOperator(username)
	if operator == nil || !operator.CheckPassword(password) {
		return echo.ErrUnauthorized
	}

	// Create a session
	session := &WebSession{
		Username:  username,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	// Generate a session ID
	sessionID := fmt.Sprintf("%s-%d", username, time.Now().UnixNano())
	w.sessions[sessionID] = session

	// Set a cookie
	c.SetCookie(&http.Cookie{
		Name:     "session",
		Value:    sessionID,
		Expires:  session.ExpiresAt,
		HttpOnly: true,
		Path:     "/",
	})

	// Update last login
	operator.UpdateLastLogin()

	// Return success
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Login successful",
	})
}

// handleAPIToken handles the token generation API
func (w *WebPortal) handleAPIToken(c echo.Context) error {
	// Only allow POST
	if c.Request().Method != http.MethodPost {
		return echo.ErrMethodNotAllowed
	}

	// Check if the user is logged in
	session, _ := w.getSession(c.Request())
	if session == nil {
		return echo.ErrUnauthorized
	}

	// Parse the request
	err := c.Request().ParseForm()
	if err != nil {
		return echo.ErrBadRequest
	}

	nickname := c.FormValue("nickname")

	// Find the client
	client := w.server.GetClient(nickname)
	if client == nil {
		return echo.ErrNotFound
	}

	// Get the operator
	operator := w.server.GetOperator(session.Username)
	if operator == nil {
		return echo.ErrNotFound
	}

	// Create and send a magic link
	webPortalURL := fmt.Sprintf("http://%s", w.config.GetWebListenAddress())
	if w.config.WebPortal.TLS {
		webPortalURL = fmt.Sprintf("https://%s", w.config.GetWebListenAddress())
	}

	token, err := operator.SendMagicLink(client, webPortalURL)
	if err != nil {
		return err
	}

	// Return success
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Magic link sent",
		"token":   token,
	})
}

// handleAPIStats handles the stats API
func (w *WebPortal) handleAPIStats(c echo.Context) error {
	// Check if the user is logged in
	session, _ := w.getSession(c.Request())
	if session == nil {
		return echo.ErrUnauthorized
	}

	// Get stats
	stats := map[string]interface{}{
		"server":   w.server.GetConfig().Server.Name,
		"network":  w.server.GetConfig().Server.Network,
		"uptime":   w.server.GetUptime().String(),
		"clients":  w.server.ClientCount(),
		"channels": w.server.ChannelCount(),
	}

	// Return the stats
	return c.JSON(http.StatusOK, stats)
}

// handleAPIChannels handles the channels API
func (w *WebPortal) handleAPIChannels(c echo.Context) error {
	// Check if the user is logged in
	session, _ := w.getSession(c.Request())
	if session == nil {
		return echo.ErrUnauthorized
	}

	// Get channels
	w.server.mu.RLock()
	channels := make([]map[string]interface{}, 0)
	w.server.channels.Range(func(key, value interface{}) bool {
		name := key.(string)
		channel := value.(*Channel)
		channels = append(channels, map[string]interface{}{
			"name":  name,
			"topic": channel.Topic,
			"users": channel.MemberCount(),
			"modes": channel.GetModeString(),
		})
		return true
	})
	w.server.mu.RUnlock()

	// Return the channels
	return c.JSON(http.StatusOK, channels)
}

// handleAPIUsers handles the users API
func (w *WebPortal) handleAPIUsers(c echo.Context) error {
	// Check if the user is logged in
	session, _ := w.getSession(c.Request())
	if session == nil {
		return echo.ErrUnauthorized
	}

	// Get users
	w.server.mu.RLock()
	users := make([]map[string]interface{}, 0)
	w.server.clients.Range(func(_, value interface{}) bool {
		client := value.(*Client)
		users = append(users, map[string]interface{}{
			"nickname":  client.Nickname,
			"username":  client.Username,
			"hostname":  client.Hostname,
			"ip":        client.IP,
			"modes":     client.Modes.GetModeString(),
			"channels":  len(client.Channels),
			"connected": time.Since(client.LastPing).String(),
		})
		return true
	})
	w.server.mu.RUnlock()

	// Return the users
	return c.JSON(http.StatusOK, users)
}

// handleAPIKick handles the kick API
func (w *WebPortal) handleAPIKick(c echo.Context) error {
	// Only allow POST
	if c.Request().Method != http.MethodPost {
		return echo.NewHTTPError(http.StatusMethodNotAllowed, "Method not allowed")
	}

	// Check if the user is logged in
	session, _ := w.getSessionFromEcho(c)
	if session == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}

	nickname := c.FormValue("nickname")
	channel := c.FormValue("channel")
	reason := c.FormValue("reason")

	if reason == "" {
		reason = "Kicked by operator"
	}

	// Get the client and channel
	targetClient := w.server.GetClient(nickname)
	targetChannel := w.server.GetChannel(channel)

	if targetClient == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Client not found")
	}

	if targetChannel == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Channel not found")
	}

	// Create a virtual operator client
	operClient := &Client{
		ID:       "oper-" + session.Username,
		Nickname: session.Username,
		Username: "oper",
		Hostname: w.server.GetConfig().Server.Name,
		IsOper:   true,
		Server:   w.server,
	}

	// Kick the client
	targetChannel.Kick(operClient, targetClient, reason)

	// Return success
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Kicked %s from %s: %s", nickname, channel, reason),
	})
}

// handleAPIKill handles the kill API
func (w *WebPortal) handleAPIKill(c echo.Context) error {
	// Only allow POST
	if c.Request().Method != http.MethodPost {
		return echo.ErrMethodNotAllowed
	}

	// Check if the user is logged in
	session, _ := w.getSession(c.Request())
	if session == nil {
		return echo.ErrUnauthorized
	}

	// Parse the request
	err := c.Request().ParseForm()
	if err != nil {
		return echo.ErrBadRequest
	}

	nickname := c.FormValue("nickname")
	reason := c.FormValue("reason")

	if reason == "" {
		reason = "Killed by operator"
	}

	// Get the client
	targetClient := w.server.GetClient(nickname)

	if targetClient == nil {
		return echo.ErrNotFound
	}

	// Kill the client
	targetClient.Quit(fmt.Sprintf("Killed by %s: %s", session.Username, reason))

	// Return success
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Killed %s: %s", nickname, reason),
	})
}

// handleAPIMode handles the mode API
func (w *WebPortal) handleAPIMode(c echo.Context) error {
	// Only allow POST
	if c.Request().Method != http.MethodPost {
		return echo.ErrMethodNotAllowed
	}

	// Check if the user is logged in
	session, _ := w.getSession(c.Request())
	if session == nil {
		return echo.ErrUnauthorized
	}

	// Parse the request
	err := c.Request().ParseForm()
	if err != nil {
		return echo.ErrBadRequest
	}

	target := c.Request().Form.Get("target")
	mode := c.Request().Form.Get("mode")

	// Check if the target is a channel or a user
	if strings.HasPrefix(target, "#") {
		// Channel mode
		channel := w.server.GetChannel(target)
		if channel == nil {
			return echo.ErrNotFound
		}

		// Parse the mode string
		modeSet := true
		for _, m := range mode {
			if m == '+' {
				modeSet = true
				continue
			}
			if m == '-' {
				modeSet = false
				continue
			}

			// Set the mode
			channel.SetMode(m, modeSet, "")
		}

		// Notify all members
		channel.SendToAll(fmt.Sprintf(":%s!oper@%s MODE %s %s", session.Username, w.server.GetConfig().Server.Name, target, mode), nil)
	} else {
		// User mode
		client := w.server.GetClient(target)
		if client == nil {
			return echo.ErrNotFound
		}

		// Parse the mode string
		modeSet := true
		for _, m := range mode {
			if m == '+' {
				modeSet = true
				continue
			}
			if m == '-' {
				modeSet = false
				continue
			}

			// Set the mode
			client.SetMode(string(m), modeSet)
		}

		// Notify the client
		client.SendMessage(w.server.GetConfig().Server.Name, "MODE", client.Nickname, mode)
	}

	// Return success
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Set mode %s on %s", mode, target),
	})
}

// handleAPIRehash handles the rehash API
func (w *WebPortal) handleAPIRehash(c echo.Context) error {
	// Only allow POST
	if c.Request().Method != http.MethodPost {
		return echo.ErrMethodNotAllowed
	}

	// Check if the user is logged in
	session, _ := w.getSession(c.Request())
	if session == nil {
		return echo.ErrUnauthorized
	}

	// Parse the request
	err := c.Request().ParseForm()
	if err != nil {
		return echo.ErrBadRequest
	}

	configURL := c.Request().Form.Get("url")

	// Rehash the server
	err = w.server.Rehash(configURL)
	if err != nil {
		return echo.ErrInternalServerError
	}

	// Return success
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Rehash successful",
	})
}

// Helper methods

// getSession gets the session from the request
func (w *WebPortal) getSession(req *http.Request) (*WebSession, error) {
	// Get the session cookie
	cookie, err := req.Cookie("session")
	if err != nil {
		return nil, err
	}

	// Get the session
	session, exists := w.sessions[cookie.Value]
	if !exists {
		return nil, fmt.Errorf("session not found")
	}

	// Check if the session is expired
	if time.Now().After(session.ExpiresAt) {
		delete(w.sessions, cookie.Value)
		return nil, fmt.Errorf("session expired")
	}

	return session, nil
}

// getSessionFromEcho gets the session from an Echo context
func (w *WebPortal) getSessionFromEcho(c echo.Context) (*WebSession, error) {
	// Get the session cookie
	cookie, err := c.Cookie("session")
	if err != nil {
		return nil, err
	}

	// Get the session
	session, exists := w.sessions[cookie.Value]
	if !exists {
		return nil, fmt.Errorf("session not found")
	}

	// Check if the session is expired
	if time.Now().After(session.ExpiresAt) {
		delete(w.sessions, cookie.Value)
		return nil, fmt.Errorf("session expired")
	}

	return session, nil
}

// checkPassword checks if a password is correct using constant-time comparison
func checkPassword(actual, expected string) bool {
	return subtle.ConstantTimeCompare([]byte(actual), []byte(expected)) == 1
}
