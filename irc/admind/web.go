package admind

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"sort"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/presbrey/pkg/t"
)

// handleAdminBans renders the ban management page
func (s *Server) handleAdminBans(c echo.Context) error {
	user := c.Get("user").(map[string]string)

	// Handle form submissions for adding/removing bans
	if c.Request().Method == http.MethodPost {
		if err := c.Request().ParseForm(); err == nil {
			action := c.Request().Form.Get("action")

			switch action {
			case "add_kline":
				mask := c.Request().Form.Get("mask")
				reason := c.Request().Form.Get("reason")
				duration := c.Request().Form.Get("duration")

				if mask != "" && reason != "" && isValidHostmask(mask) {
					var banDuration time.Duration
					var err error

					if duration != "" {
						banDuration, err = parseDuration(duration)
						if err != nil {
							banDuration = 0 // Permanent if invalid
						}
					}

					// Create and add K-line
					ban := &BanEntry{
						Hostmask: mask,
						Reason:   reason,
						Setter:   user["email"],
						SetTime:  time.Now(),
						Duration: banDuration,
						IsGlobal: false,
					}

					// Set expiry time if applicable
					if banDuration > 0 {
						ban.ExpiryTime = time.Now().Add(banDuration)
					}

					s.Lock()
					s.GetKlines()[mask] = ban
					s.Unlock()

					// Disconnect matching clients
					s.DisconnectBannedClients(ban)
				}

			case "add_gline":
				mask := c.Request().Form.Get("mask")
				reason := c.Request().Form.Get("reason")
				duration := c.Request().Form.Get("duration")

				if mask != "" && reason != "" && isValidHostmask(mask) {
					var banDuration time.Duration
					var err error

					if duration != "" {
						banDuration, err = parseDuration(duration)
						if err != nil {
							banDuration = 0 // Permanent if invalid
						}
					}

					// Create and add G-line
					ban := &BanEntry{
						Hostmask: mask,
						Reason:   reason,
						Setter:   user["email"],
						SetTime:  time.Now(),
						Duration: banDuration,
						IsGlobal: true,
					}

					// Set expiry time if applicable
					if banDuration > 0 {
						ban.ExpiryTime = time.Now().Add(banDuration)
					}

					s.Lock()
					s.GetGlines()[mask] = ban
					s.Unlock()

					// Disconnect matching clients
					s.DisconnectBannedClients(ban)

					// Propagate to other servers
					s.PropagateGline(ban)
				}

			case "remove_kline":
				mask := c.Request().Form.Get("mask")
				if mask != "" {
					s.Lock()
					delete(s.GetKlines(), mask)
					s.Unlock()
				}

			case "remove_gline":
				mask := c.Request().Form.Get("mask")
				if mask != "" {
					s.Lock()
					delete(s.GetGlines(), mask)
					s.Unlock()

					// Propagate removal to other servers
					s.PropagateUngline(mask)
				}
			}

			// Redirect to avoid form resubmission
			return c.Redirect(http.StatusSeeOther, "/bans")
		}
	}

	// Gather K-line data
	type BanData struct {
		Mask      string
		Reason    string
		Setter    string
		SetTime   time.Time
		Expiry    string
		IsExpired bool
	}

	var klines []BanData
	var glines []BanData

	s.RLock()

	// Process K-lines
	for mask, ban := range s.GetKlines() {
		expiryStr := "Permanent"
		isExpired := false

		if !ban.ExpiryTime.IsZero() {
			if time.Now().After(ban.ExpiryTime) {
				expiryStr = "Expired"
				isExpired = true
			} else {
				remaining := time.Until(ban.ExpiryTime).Round(time.Second)
				expiryStr = remaining.String()
			}
		}

		klines = append(klines, BanData{
			Mask:      mask,
			Reason:    ban.Reason,
			Setter:    ban.Setter,
			SetTime:   ban.SetTime,
			Expiry:    expiryStr,
			IsExpired: isExpired,
		})
	}

	// Process G-lines
	for mask, ban := range s.GetGlines() {
		expiryStr := "Permanent"
		isExpired := false

		if !ban.ExpiryTime.IsZero() {
			if time.Now().After(ban.ExpiryTime) {
				expiryStr = "Expired"
				isExpired = true
			} else {
				remaining := time.Until(ban.ExpiryTime).Round(time.Second)
				expiryStr = remaining.String()
			}
		}

		glines = append(glines, BanData{
			Mask:      mask,
			Reason:    ban.Reason,
			Setter:    ban.Setter,
			SetTime:   ban.SetTime,
			Expiry:    expiryStr,
			IsExpired: isExpired,
		})
	}
	s.RUnlock()

	// Sort bans by set time (newest first)
	sort.Slice(klines, func(i, j int) bool {
		return klines[i].SetTime.After(klines[j].SetTime)
	})

	sort.Slice(glines, func(i, j int) bool {
		return glines[i].SetTime.After(glines[j].SetTime)
	})

	// Template data
	data := struct {
		KLines    []BanData
		GLines    []BanData
		UserName  string
		UserEmail string
	}{
		KLines:    klines,
		GLines:    glines,
		UserName:  user["name"],
		UserEmail: user["email"],
	}

	// Render the template
	tmpl, err := template.New("bans").Parse(`
<!DOCTYPE html>
<html>
<head>
    <title>Ban Management - IRC Server Administration</title>
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
        .tab-content { padding: 20px 0; }
        .tabs { display: flex; border-bottom: 1px solid #ddd; }
        .tab { padding: 10px 15px; cursor: pointer; margin-right: 5px; }
        .tab.active { border: 1px solid #ddd; border-bottom: none; background: white; border-radius: 5px 5px 0 0; }
        .expired { color: #888; }
        .form-group { margin-bottom: 15px; }
        label { display: block; margin-bottom: 5px; font-weight: bold; }
        input[type="text"], textarea, select { width: 100%; padding: 8px; border: 1px solid #ddd; border-radius: 4px; box-sizing: border-box; }
        .btn { display: inline-block; background: #3498db; color: white; border: none; padding: 8px 15px; border-radius: 4px; cursor: pointer; }
        .btn-danger { background: #e74c3c; }
        .form-row { display: flex; margin: 0 -10px; }
        .form-col { flex: 1; padding: 0 10px; }
        .remove-form { display: inline-block; }
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
            <a href="/clients">Clients</a>
            <a href="/bans" class="active">Bans</a>
            <div class="user-info">Logged in as: {{.UserName}}</div>
        </div>
        
        <div class="card">
            <div class="tabs">
                <div class="tab active" id="tab-klines" onclick="showTab('klines')">K-Lines (Server Bans)</div>
                <div class="tab" id="tab-glines" onclick="showTab('glines')">G-Lines (Network Bans)</div>
            </div>
            
            <div id="klines-content" class="tab-content">
                <h2>K-Lines (Server Bans)</h2>
                
                <div class="card">
                    <h3>Add New K-Line</h3>
                    <form method="post" action="/bans">
                        <input type="hidden" name="action" value="add_kline">
                        <div class="form-row">
                            <div class="form-col">
                                <div class="form-group">
                                    <label for="mask">Host Mask (e.g., *!*@example.com):</label>
                                    <input type="text" id="mask" name="mask" required>
                                </div>
                            </div>
                            <div class="form-col">
                                <div class="form-group">
                                    <label for="duration">Duration (e.g., 1d6h, empty for permanent):</label>
                                    <input type="text" id="duration" name="duration">
                                </div>
                            </div>
                        </div>
                        <div class="form-group">
                            <label for="reason">Reason:</label>
                            <input type="text" id="reason" name="reason" required>
                        </div>
                        <button type="submit" class="btn">Add K-Line</button>
                    </form>
                </div>
                
                <h3>Active K-Lines</h3>
                {{if .KLines}}
                <table>
                    <thead>
                        <tr>
                            <th>Host Mask</th>
                            <th>Reason</th>
                            <th>Set By</th>
                            <th>Set Time</th>
                            <th>Expiry</th>
                            <th>Actions</th>
                        </tr>
                    </thead>
                    <tbody>
                        {{range .KLines}}
                        <tr {{if .IsExpired}}class="expired"{{end}}>
                            <td>{{.Mask}}</td>
                            <td>{{.Reason}}</td>
                            <td>{{.Setter}}</td>
                            <td>{{.SetTime.Format "2006-01-02 15:04:05"}}</td>
                            <td>{{.Expiry}}</td>
                            <td>
                                <form method="post" action="/bans" class="remove-form">
                                    <input type="hidden" name="action" value="remove_kline">
                                    <input type="hidden" name="mask" value="{{.Mask}}">
                                    <button type="submit" class="btn btn-danger">Remove</button>
                                </form>
                            </td>
                        </tr>
                        {{end}}
                    </tbody>
                </table>
                {{else}}
                <div class="empty-message">No K-Lines defined</div>
                {{end}}
            </div>
            
            <div id="glines-content" class="tab-content" style="display: none;">
                <h2>G-Lines (Network Bans)</h2>
                
                <div class="card">
                    <h3>Add New G-Line</h3>
                    <form method="post" action="/bans">
                        <input type="hidden" name="action" value="add_gline">
                        <div class="form-row">
                            <div class="form-col">
                                <div class="form-group">
                                    <label for="gmask">Host Mask (e.g., *!*@example.com):</label>
                                    <input type="text" id="gmask" name="mask" required>
                                </div>
                            </div>
                            <div class="form-col">
                                <div class="form-group">
                                    <label for="gduration">Duration (e.g., 1d6h, empty for permanent):</label>
                                    <input type="text" id="gduration" name="duration">
                                </div>
                            </div>
                        </div>
                        <div class="form-group">
                            <label for="greason">Reason:</label>
                            <input type="text" id="greason" name="reason" required>
                        </div>
                        <button type="submit" class="btn">Add G-Line</button>
                    </form>
                </div>
                
                <h3>Active G-Lines</h3>
                {{if .GLines}}
                <table>
                    <thead>
                        <tr>
                            <th>Host Mask</th>
                            <th>Reason</th>
                            <th>Set By</th>
                            <th>Set Time</th>
                            <th>Expiry</th>
                            <th>Actions</th>
                        </tr>
                    </thead>
                    <tbody>
                        {{range .GLines}}
                        <tr {{if .IsExpired}}class="expired"{{end}}>
                            <td>{{.Mask}}</td>
                            <td>{{.Reason}}</td>
                            <td>{{.Setter}}</td>
                            <td>{{.SetTime.Format "2006-01-02 15:04:05"}}</td>
                            <td>{{.Expiry}}</td>
                            <td>
                                <form method="post" action="/bans" class="remove-form">
                                    <input type="hidden" name="action" value="remove_gline">
                                    <input type="hidden" name="mask" value="{{.Mask}}">
                                    <button type="submit" class="btn btn-danger">Remove</button>
                                </form>
                            </td>
                        </tr>
                        {{end}}
                    </tbody>
                </table>
                {{else}}
                <div class="empty-message">No G-Lines defined</div>
                {{end}}
            </div>
        </div>
    </div>
    
    <script>
        function showTab(tabId) {
            document.getElementById('klines-content').style.display = 'none';
            document.getElementById('glines-content').style.display = 'none';
            document.getElementById('tab-klines').classList.remove('active');
            document.getElementById('tab-glines').classList.remove('active');
            
            document.getElementById(tabId + '-content').style.display = 'block';
            document.getElementById('tab-' + tabId).classList.add('active');
        }
    </script>
</body>
</html>
`)
	if err != nil {
		return err
	}
	err = tmpl.ExecuteTemplate(c.Response().Writer, "admin", data)
	if err != nil {
		return err
	}

	return nil
}

// handleSendMessageToChannel handles requests to send a message to an IRC channel.
func (s *Server) handleSendMessageToChannel(c echo.Context) error {
	if c.Request().Method != http.MethodPost {
		return echo.ErrMethodNotAllowed
	}

	var req struct {
		Channel string `json:"channel"`
		Message string `json:"message"`
	}

	decoder := json.NewDecoder(c.Request().Body)
	if err := decoder.Decode(&req); err != nil {
		return echo.ErrBadRequest
	}
	// Ensure r.Body is closed after handling the request, not immediately after decoding.
	// defer r.Body.Close() // This should be at the end of the handler if we need to read more, or if other middleware might read it.

	if req.Channel == "" || req.Message == "" {
		return echo.ErrBadRequest
	}

	// Placeholder: s.RelayPrivmsgToChannel will be implemented on admind.Server
	// It will be responsible for sending the message to the actual IRC server.
	err := s.RelayPrivmsgToChannel(req.Channel, req.Message)
	if err != nil {
		log.Printf("Error sending message to channel %s: %v", req.Channel, err)
		return err
	}

	return c.String(http.StatusOK, "Message sent successfully")
}

// setupAdminServer configures the admin server routes
func (s *Server) route(r t.EchoRouter) error {
	// Add admin routes
	r.GET("/", s.handleAdminHome)
	r.GET("/stats", s.handleAdminStats)
	r.GET("/channels", s.handleAdminChannels)
	r.GET("/clients", s.handleAdminClients)
	r.GET("/bans", s.handleAdminBans)
	r.GET("/login", s.handleAdminLogin)
	r.GET("/callback", s.handleOIDCCallback)
	r.GET("/api/stats", s.handleAPIStats)
	r.POST("/api/send_message", s.handleSendMessageToChannel)
	r.Use(s.authMiddleware)
	return nil
}
