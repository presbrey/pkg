package admind

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"
)

// Admin HTTP server handlers

// authMiddleware is a middleware that enforces OIDC authentication
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip auth for login and callback endpoints
		if r.URL.Path == "/login" || r.URL.Path == "/callback" {
			next.ServeHTTP(w, r)
			return
		}

		// Skip auth for API endpoints with valid token
		if strings.HasPrefix(r.URL.Path, "/api/") {
			token := r.Header.Get("Authorization")
			if token != "" && strings.HasPrefix(token, "Bearer ") {
				// Verify the token
				idToken, err := s.oidcVerifier.Verify(r.Context(), token[7:])
				if err == nil {
					var claims struct {
						Email         string `json:"email"`
						EmailVerified bool   `json:"email_verified"`
					}
					if err := idToken.Claims(&claims); err == nil && claims.EmailVerified {
						s.RLock()
						isOperator := s.GetOperators()[claims.Email]
						s.RUnlock()

						if isOperator {
							// Valid operator, proceed
							next.ServeHTTP(w, r)
							return
						}
					}
				}
			}

			// Invalid token for API
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
			return
		}

		// Check for session cookie
		cookie, err := r.Cookie("irc_session")
		if err != nil || cookie.Value == "" {
			// No session, redirect to login
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		// Verify the session token
		idToken, err := s.oidcVerifier.Verify(r.Context(), cookie.Value)
		if err != nil {
			// Invalid session, redirect to login
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		// Extract and verify claims
		var claims struct {
			Email         string `json:"email"`
			EmailVerified bool   `json:"email_verified"`
			Name          string `json:"name"`
		}
		if err := idToken.Claims(&claims); err != nil || !claims.EmailVerified {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		// Check if the user is an operator
		s.RLock()
		isOperator := s.GetOperators()[claims.Email]
		s.RUnlock()

		if !isOperator {
			http.Error(w, "Unauthorized: not an operator", http.StatusUnauthorized)
			return
		}

		// Add user info to request context
		ctx := context.WithValue(r.Context(), "user", map[string]string{
			"email": claims.Email,
			"name":  claims.Name,
		})

		// User authenticated, proceed
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// handleAdminHome renders the admin dashboard homepage
func (s *Server) handleAdminHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	user := r.Context().Value("user").(map[string]string)

	// Get server stats
	serverStats := s.GetStats()
	serverConfig := s.GetConfig()
	
	serverStats.RLock()
	stats := struct {
		ServerName      string
		ServerDesc      string
		Uptime          string
		ConnectionCount int
		MaxConnections  int
		ChannelCount    int
		ClientCount     int
		UserName        string
		UserEmail       string
	}{
		ServerName:      serverConfig.ServerName,
		ServerDesc:      serverConfig.ServerDesc,
		Uptime:          time.Since(serverStats.StartTime).Round(time.Second).String(),
		ConnectionCount: serverStats.ConnectionCount,
		MaxConnections:  serverStats.MaxConnections,
		ChannelCount:    len(s.GetChannels()),
		ClientCount:     len(s.GetClients()),
		UserName:        user["name"],
		UserEmail:       user["email"],
	}
	serverStats.RUnlock()

	// Render the template
	tmpl, err := template.New("home").Parse(`
<!DOCTYPE html>
<html>
<head>
    <title>IRC Server Administration</title>
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
        body { font-family: Arial, sans-serif; margin: 0; padding: 20px; color: #333; }
        .header { background: #2c3e50; color: white; padding: 10px 20px; margin-bottom: 20px; }
        .container { max-width: 1200px; margin: 0 auto; }
        .card { background: white; border-radius: 5px; box-shadow: 0 2px 5px rgba(0,0,0,0.1); padding: 20px; margin-bottom: 20px; }
        .stats { display: flex; flex-wrap: wrap; }
        .stat-box { flex: 1; min-width: 150px; margin: 10px; padding: 15px; background: #f8f9fa; border-radius: 5px; text-align: center; }
        .stat-value { font-size: 24px; font-weight: bold; margin: 10px 0; }
        .stat-label { color: #666; }
        .navbar { display: flex; background: #34495e; margin-bottom: 20px; }
        .navbar a { color: white; text-decoration: none; padding: 10px 15px; }
        .navbar a:hover { background: #2c3e50; }
        .navbar a.active { background: #2c3e50; }
        .user-info { margin-left: auto; padding: 10px 15px; color: white; }
    </style>
</head>
<body>
    <div class="header">
        <h1>IRC Server Administration</h1>
    </div>
    
    <div class="container">
        <div class="navbar">
            <a href="/" class="active">Dashboard</a>
            <a href="/channels">Channels</a>
            <a href="/clients">Clients</a>
            <div class="user-info">Logged in as: {{.UserName}}</div>
        </div>
        
        <div class="card">
            <h2>Server Information</h2>
            <p><strong>Server Name:</strong> {{.ServerName}}</p>
            <p><strong>Description:</strong> {{.ServerDesc}}</p>
            <p><strong>Uptime:</strong> {{.Uptime}}</p>
        </div>
        
        <div class="card">
            <h2>Statistics</h2>
            <div class="stats">
                <div class="stat-box">
                    <div class="stat-label">Connected Clients</div>
                    <div class="stat-value">{{.ClientCount}}</div>
                </div>
                <div class="stat-box">
                    <div class="stat-label">Active Channels</div>
                    <div class="stat-value">{{.ChannelCount}}</div>
                </div>
                <div class="stat-box">
                    <div class="stat-label">Current Connections</div>
                    <div class="stat-value">{{.ConnectionCount}}</div>
                </div>
                <div class="stat-box">
                    <div class="stat-label">Peak Connections</div>
                    <div class="stat-value">{{.MaxConnections}}</div>
                </div>
            </div>
        </div>
    </div>
    
    <script>
        // Auto-refresh the page every 60 seconds
        setTimeout(function() {
            window.location.reload();
        }, 60000);
    </script>
</body>
</html>
`)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		log.Printf("Template error: %v", err)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	tmpl.Execute(w, stats)
}

// handleAdminStats renders the server statistics page
func (s *Server) handleAdminStats(w http.ResponseWriter, r *http.Request) {
	// Similar to home but with more detailed statistics
	// Implementation similar to handleAdminHome
	fmt.Fprintf(w, "Detailed server statistics page")
}

// handleAdminChannels renders the channels overview page
func (s *Server) handleAdminChannels(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value("user").(map[string]string)

	// Gather channel data
	type ChannelData struct {
		Name      string
		Topic     string
		UserCount int
		Operators []string
	}

	var channels []ChannelData

	s.RLock()
	for name, channel := range s.GetChannels() {
		channel.RLock()

		// Extract operator nicknames
		operators := make([]string, 0, len(channel.GetOperators()))
		for nickname := range channel.GetOperators() {
			operators = append(operators, nickname)
		}
		sort.Strings(operators)

		channels = append(channels, ChannelData{
			Name:      name,
			Topic:     channel.GetTopic(),
			UserCount: len(channel.GetClients()),
			Operators: operators,
		})
		channel.RUnlock()
	}
	s.RUnlock()

	// Sort channels by name
	sort.Slice(channels, func(i, j int) bool {
		return channels[i].Name < channels[j].Name
	})

	// Template data
	data := struct {
		Channels  []ChannelData
		UserName  string
		UserEmail string
	}{
		Channels:  channels,
		UserName:  user["name"],
		UserEmail: user["email"],
	}

	// Render the template
	tmpl, err := template.New("channels").Parse(`
<!DOCTYPE html>
<html>
<head>
    <title>IRC Channels - IRC Server Administration</title>
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
        body { font-family: Arial, sans-serif; margin: 0; padding: 20px; color: #333; }
        .header { background: #2c3e50; color: white; padding: 10px 20px; margin-bottom: 20px; }
        .container { max-width: 1200px; margin: 0 auto; }
        .card { background: white; border-radius: 5px; box-shadow: 0 2px 5px rgba(0,0,0,0.1); padding: 20px; margin-bottom: 20px; }
        .navbar { display: flex; background: #34495e; margin-bottom: 20px; }
        .navbar a { color: white; text-decoration: none; padding: 10px 15px; }
        .navbar a:hover { background: #2c3e50; }
        .navbar a.active { background: #2c3e50; }
        .user-info { margin-left: auto; padding: 10px 15px; color: white; }
        table { width: 100%; border-collapse: collapse; }
        th, td { text-align: left; padding: 12px; border-bottom: 1px solid #ddd; }
        th { background-color: #f2f2f2; }
        tr:hover { background-color: #f5f5f5; }
        .empty-message { text-align: center; padding: 30px; color: #666; }
    </style>
</head>
<body>
    <div class="header">
        <h1>IRC Server Administration</h1>
    </div>
    
    <div class="container">
        <div class="navbar">
            <a href="/">Dashboard</a>
            <a href="/channels" class="active">Channels</a>
            <a href="/clients">Clients</a>
            <div class="user-info">Logged in as: {{.UserName}}</div>
        </div>
        
        <div class="card">
            <h2>Active Channels</h2>
            
            {{if .Channels}}
            <table>
                <thead>
                    <tr>
                        <th>Channel</th>
                        <th>Users</th>
                        <th>Topic</th>
                        <th>Operators</th>
                    </tr>
                </thead>
                <tbody>
                    {{range .Channels}}
                    <tr>
                        <td>{{.Name}}</td>
                        <td>{{.UserCount}}</td>
                        <td>{{.Topic}}</td>
                        <td>{{range .Operators}}{{.}} {{end}}</td>
                    </tr>
                    {{end}}
                </tbody>
            </table>
            {{else}}
            <div class="empty-message">No active channels</div>
            {{end}}
        </div>
    </div>
    
    <script>
        // Auto-refresh the page every 30 seconds
        setTimeout(function() {
            window.location.reload();
        }, 30000);
    </script>
</body>
</html>
`)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		log.Printf("Template error: %v", err)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	tmpl.Execute(w, data)
}

// handleAdminClients renders the clients overview page
func (s *Server) handleAdminClients(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value("user").(map[string]string)

	// Gather client data
	type ClientData struct {
		Nickname    string
		Username    string
		Hostname    string
		Realname    string
		IsOperator  bool
		Connected   time.Time
		ChannelList string
	}

	var clients []ClientData

	s.RLock()
	for _, client := range s.GetClients() {
		client.RLock()

		// Build channel list
		channels := make([]string, 0, len(client.GetChannels()))
		for channelName := range client.GetChannels() {
			channels = append(channels, channelName)
		}
		sort.Strings(channels)

		clients = append(clients, ClientData{
			Nickname:    client.GetNickname(),
			Username:    client.GetUsername(),
			Hostname:    client.GetHostname(),
			Realname:    client.GetRealname(),
			IsOperator:  false, // TODO: Check operator status
			Connected:   client.GetLastPong(),
			ChannelList: strings.Join(channels, ", "),
		})
		client.RUnlock()
	}
	s.RUnlock()

	// Sort clients by nickname
	sort.Slice(clients, func(i, j int) bool {
		return clients[i].Nickname < clients[j].Nickname
	})

	// Template data
	data := struct {
		Clients   []ClientData
		UserName  string
		UserEmail string
	}{
		Clients:   clients,
		UserName:  user["name"],
		UserEmail: user["email"],
	}

	// Render the template
	tmpl, err := template.New("clients").Parse(`
<!DOCTYPE html>
<html>
<head>
    <title>IRC Clients - IRC Server Administration</title>
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
        body { font-family: Arial, sans-serif; margin: 0; padding: 20px; color: #333; }
        .header { background: #2c3e50; color: white; padding: 10px 20px; margin-bottom: 20px; }
        .container { max-width: 1200px; margin: 0 auto; }
        .card { background: white; border-radius: 5px; box-shadow: 0 2px 5px rgba(0,0,0,0.1); padding: 20px; margin-bottom: 20px; }
        .navbar { display: flex; background: #34495e; margin-bottom: 20px; }
        .navbar a { color: white; text-decoration: none; padding: 10px 15px; }
        .navbar a:hover { background: #2c3e50; }
        .navbar a.active { background: #2c3e50; }
        .user-info { margin-left: auto; padding: 10px 15px; color: white; }
        table { width: 100%; border-collapse: collapse; }
        th, td { text-align: left; padding: 12px; border-bottom: 1px solid #ddd; }
        th { background-color: #f2f2f2; }
        tr:hover { background-color: #f5f5f5; }
        .empty-message { text-align: center; padding: 30px; color: #666; }
        .operator-badge { display: inline-block; padding: 2px 6px; background: #e74c3c; color: white; border-radius: 3px; font-size: 12px; }
    </style>
</head>
<body>
    <div class="header">
        <h1>IRC Server Administration</h1>
    </div>
    
    <div class="container">
        <div class="navbar">
            <a href="/">Dashboard</a>
            <a href="/channels">Channels</a>
            <a href="/clients" class="active">Clients</a>
            <div class="user-info">Logged in as: {{.UserName}}</div>
        </div>
        
        <div class="card">
            <h2>Connected Clients</h2>
            
            {{if .Clients}}
            <table>
                <thead>
                    <tr>
                        <th>Nickname</th>
                        <th>Username</th>
                        <th>Host</th>
                        <th>Real Name</th>
                        <th>Channels</th>
                        <th>Connected Since</th>
                    </tr>
                </thead>
                <tbody>
                    {{range .Clients}}
                    <tr>
                        <td>
                            {{.Nickname}}
                            {{if .IsOperator}}
                            <span class="operator-badge">OPER</span>
                            {{end}}
                        </td>
                        <td>{{.Username}}</td>
                        <td>{{.Hostname}}</td>
                        <td>{{.Realname}}</td>
                        <td>{{.ChannelList}}</td>
                        <td>{{.Connected.Format "2006-01-02 15:04:05"}}</td>
                    </tr>
                    {{end}}
                </tbody>
            </table>
            {{else}}
            <div class="empty-message">No connected clients</div>
            {{end}}
        </div>
    </div>
    
    <script>
        // Auto-refresh the page every 30 seconds
        setTimeout(function() {
            window.location.reload();
        }, 30000);
    </script>
</body>
</html>
`)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		log.Printf("Template error: %v", err)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	tmpl.Execute(w, data)
}

// handleAdminLogin handles the login page and OIDC flow initiation
func (s *Server) handleAdminLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		// Start OIDC flow
		state := generateRandomString(32)

		// Store the state in a cookie
		http.SetCookie(w, &http.Cookie{
			Name:     "oidc_state",
			Value:    state,
			MaxAge:   int(time.Hour.Seconds()),
			HttpOnly: true,
			Path:     "/",
			SameSite: http.SameSiteLaxMode,
		})

		// Redirect to OIDC provider
		authURL := s.oauth2Config.AuthCodeURL(state)
		http.Redirect(w, r, authURL, http.StatusFound)
		return
	}

	// Render login page
	tmpl, err := template.New("login").Parse(`
<!DOCTYPE html>
<html>
<head>
    <title>Login - IRC Server Administration</title>
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
        body { font-family: Arial, sans-serif; margin: 0; padding: 0; background-color: #f5f5f5; }
        .container { max-width: 400px; margin: 100px auto; padding: 20px; background: white; border-radius: 5px; box-shadow: 0 2px 10px rgba(0,0,0,0.1); }
        h1 { text-align: center; color: #2c3e50; margin-bottom: 30px; }
        .btn { display: block; width: 100%; padding: 10px; background: #3498db; color: white; border: none; border-radius: 3px; cursor: pointer; font-size: 16px; text-align: center; text-decoration: none; }
        .btn:hover { background: #2980b9; }
        .info { margin-top: 20px; padding: 15px; background: #f8f9fa; border-radius: 3px; font-size: 14px; color: #666; }
    </style>
</head>
<body>
    <div class="container">
        <h1>IRC Server Administration</h1>
        
        <form method="post" action="/login">
            <button type="submit" class="btn">Sign in with Google</button>
        </form>
        
        <div class="info">
            <p>You must be an authorized operator to access the admin dashboard. Authentication is handled through Google Accounts.</p>
        </div>
    </div>
</body>
</html>
`)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		log.Printf("Template error: %v", err)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	tmpl.Execute(w, nil)
}

// handleOIDCCallback handles the OIDC callback after authentication
func (s *Server) handleOIDCCallback(w http.ResponseWriter, r *http.Request) {
	// Verify state
	stateCookie, err := r.Cookie("oidc_state")
	if err != nil {
		http.Error(w, "Invalid state", http.StatusBadRequest)
		return
	}

	state := r.URL.Query().Get("state")
	if state != stateCookie.Value {
		http.Error(w, "Invalid state", http.StatusBadRequest)
		return
	}

	// Clear state cookie
	http.SetCookie(w, &http.Cookie{
		Name:   "oidc_state",
		Value:  "",
		MaxAge: -1,
		Path:   "/",
	})

	// Exchange authorization code for token
	code := r.URL.Query().Get("code")
	oauth2Token, err := s.oauth2Config.Exchange(r.Context(), code)
	if err != nil {
		http.Error(w, "Failed to exchange token", http.StatusInternalServerError)
		log.Printf("Token exchange error: %v", err)
		return
	}

	// Extract ID token
	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok {
		http.Error(w, "No ID token in response", http.StatusInternalServerError)
		return
	}

	// Verify ID token
	idToken, err := s.oidcVerifier.Verify(r.Context(), rawIDToken)
	if err != nil {
		http.Error(w, "Invalid ID token", http.StatusInternalServerError)
		log.Printf("ID token verification error: %v", err)
		return
	}

	// Extract claims
	var claims struct {
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
	}
	if err := idToken.Claims(&claims); err != nil {
		http.Error(w, "Failed to parse claims", http.StatusInternalServerError)
		log.Printf("Claims parsing error: %v", err)
		return
	}

	// Check if user is an operator
	s.RLock()
	isOperator := s.GetOperators()[claims.Email]
	s.RUnlock()

	if !isOperator || !claims.EmailVerified {
		http.Error(w, "Unauthorized: You are not a registered operator", http.StatusUnauthorized)
		return
	}

	// Set session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "irc_session",
		Value:    rawIDToken,
		MaxAge:   int(time.Hour.Seconds()),
		HttpOnly: true,
		Path:     "/",
		SameSite: http.SameSiteLaxMode,
	})

	// Redirect to dashboard
	http.Redirect(w, r, "/", http.StatusFound)
}

// handleAPIStats returns server statistics in JSON format
func (s *Server) handleAPIStats(w http.ResponseWriter, r *http.Request) {
	// Get server stats
	serverStats := s.GetStats()
	serverConfig := s.GetConfig()
	
	serverStats.RLock()
	statsData := struct {
		ServerName       string    `json:"server_name"`
		ServerDesc       string    `json:"server_desc"`
		StartTime        time.Time `json:"start_time"`
		Uptime           string    `json:"uptime"`
		ConnectionCount  int       `json:"connection_count"`
		MaxConnections   int       `json:"max_connections"`
		MessagesSent     int64     `json:"messages_sent"`
		MessagesReceived int64     `json:"messages_received"`
	}{
		ServerName:       serverConfig.ServerName,
		ServerDesc:       serverConfig.ServerDesc,
		StartTime:        serverStats.StartTime,
		Uptime:           time.Since(serverStats.StartTime).String(),
		ConnectionCount:  serverStats.ConnectionCount,
		MaxConnections:   serverStats.MaxConnections,
		MessagesSent:     serverStats.MessagesSent,
		MessagesReceived: serverStats.MessagesReceived,
	}
	serverStats.RUnlock()

	s.RLock()
	statsData.ConnectionCount = len(s.GetClients())
	s.RUnlock()

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(statsData)
}

// Helper function to generate a random string for state parameter
func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
		time.Sleep(1 * time.Nanosecond)
	}
	return string(b)
}
