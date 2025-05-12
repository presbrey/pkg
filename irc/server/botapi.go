package server

import (
	"crypto/subtle"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/presbrey/pkg/irc/config"
)

// BotAPI represents the REST API for bots
type BotAPI struct {
	server *Server
	config *config.Config
	echo   *echo.Echo
}

// NewBotAPI creates a new bot API
func NewBotAPI(server *Server, cfg *config.Config) (*BotAPI, error) {
	api := &BotAPI{
		server: server,
		config: cfg,
		echo:   echo.New(),
	}
	api.echo.HideBanner = true

	// Set up the HTTP routes
	api.echo.POST("/api/send", api.handleSend)
	api.echo.POST("/api/join", api.handleJoin)
	api.echo.POST("/api/part", api.handlePart)
	api.echo.POST("/api/nick", api.handleNick)
	api.echo.POST("/api/mode", api.handleMode)
	api.echo.POST("/api/topic", api.handleTopic)
	api.echo.GET("/api/who", api.handleWho)
	api.echo.GET("/api/list", api.handleList)

	return api, nil
}

// Start starts the bot API
func (b *BotAPI) Start() error {
	// Start the HTTP server
	return b.echo.Start(b.config.GetBotAPIListenAddress())
}

// Stop stops the bot API
func (b *BotAPI) Stop() error {
	log.Println("Stopping bot API")
	return b.echo.Close()
}

// BotMessage represents a message sent by a bot
type BotMessage struct {
	Type     string   `json:"type"`
	Channel  string   `json:"channel,omitempty"`
	Target   string   `json:"target,omitempty"`
	Message  string   `json:"message,omitempty"`
	Nickname string   `json:"nickname,omitempty"`
	Username string   `json:"username,omitempty"`
	Realname string   `json:"realname,omitempty"`
	Mode     string   `json:"mode,omitempty"`
	Topic    string   `json:"topic,omitempty"`
	Channels []string `json:"channels,omitempty"`
	Reason   string   `json:"reason,omitempty"`
}

// handleSend handles sending a message
func (b *BotAPI) handleSend(c echo.Context) error {
	// Authenticate the request
	if !b.authenticateRequest(c.Request()) {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}

	// Parse the request body
	var message BotMessage
	err := c.Bind(&message)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Bad request")
	}

	// Check if the required fields are present
	if message.Nickname == "" {
		message.Nickname = "BotAPI"
	}
	if message.Username == "" {
		message.Username = "botapi"
	}
	if message.Realname == "" {
		message.Realname = "Bot API"
	}

	// Create a virtual client for the bot
	botClient := &Client{
		ID:         fmt.Sprintf("bot-%s-%d", message.Nickname, time.Now().UnixNano()),
		Nickname:   message.Nickname,
		Username:   message.Username,
		Realname:   message.Realname,
		Hostname:   b.config.Server.Name,
		Server:     b.server,
		Registered: true,
	}

	// Send the message
	switch message.Type {
	case "privmsg", "":
		// Check if the target is specified
		if message.Target == "" && message.Channel == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "Target or channel is required")
		}

		// Check if the message is specified
		if message.Message == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "Message is required")
		}

		// Send to a channel
		if message.Channel != "" {
			channel := b.server.GetChannel(message.Channel)
			if channel == nil {
				return echo.NewHTTPError(http.StatusNotFound, "Channel not found")
			}

			// Send the message
			channel.SendToAll(fmt.Sprintf(":%s!%s@%s PRIVMSG %s :%s", botClient.Nickname, botClient.Username, botClient.Hostname, message.Channel, message.Message), nil)
		} else {
			// Send to a user
			target := b.server.GetClient(message.Target)
			if target == nil {
				return echo.NewHTTPError(http.StatusNotFound, "Target not found")
			}

			// Send the message
			target.SendPrivmsg(botClient, message.Message)
		}
	case "notice":
		// Check if the target is specified
		if message.Target == "" && message.Channel == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "Target or channel is required")
		}

		// Check if the message is specified
		if message.Message == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "Message is required")
		}

		// Send to a channel
		if message.Channel != "" {
			channel := b.server.GetChannel(message.Channel)
			if channel == nil {
				return echo.NewHTTPError(http.StatusNotFound, "Channel not found")
			}

			// Send the message
			channel.SendToAll(fmt.Sprintf(":%s!%s@%s NOTICE %s :%s", botClient.Nickname, botClient.Username, botClient.Hostname, message.Channel, message.Message), nil)
		} else {
			// Send to a user
			target := b.server.GetClient(message.Target)
			if target == nil {
				return echo.NewHTTPError(http.StatusNotFound, "Target not found")
			}

			// Send the message
			target.SendRaw(fmt.Sprintf(":%s!%s@%s NOTICE %s :%s", botClient.Nickname, botClient.Username, botClient.Hostname, target.Nickname, message.Message))
		}
	default:
		return echo.NewHTTPError(http.StatusBadRequest, "Unknown message type")
	}

	// Return success
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Message sent",
	})
}

// handleJoin handles joining a channel
func (b *BotAPI) handleJoin(c echo.Context) error {
	// Authenticate the request
	if !b.authenticateRequest(c.Request()) {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}

	// Parse the request body
	var message BotMessage
	err := c.Bind(&message)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Bad request")
	}

	// Check if the required fields are present
	if message.Nickname == "" {
		message.Nickname = "BotAPI"
	}
	if message.Username == "" {
		message.Username = "botapi"
	}
	if message.Realname == "" {
		message.Realname = "Bot API"
	}
	if len(message.Channels) == 0 && message.Channel == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Channel is required")
	}

	// Create a virtual client for the bot
	botClient := &Client{
		ID:         fmt.Sprintf("bot-%s-%d", message.Nickname, time.Now().UnixNano()),
		Nickname:   message.Nickname,
		Username:   message.Username,
		Realname:   message.Realname,
		Hostname:   b.config.Server.Name,
		Server:     b.server,
		Registered: true,
		Channels:   make(map[string]*Channel),
	}

	// Add the client to the server
	// No lock needed with sync.Map
	b.server.clients.Store(botClient.ID, botClient)

	// Join the channels
	channels := message.Channels
	if message.Channel != "" {
		channels = append(channels, message.Channel)
	}

	for _, channelName := range channels {
		botClient.JoinChannel(channelName)
	}

	// Return success
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Joined channels",
		"id":      botClient.ID,
	})
}

// handlePart handles leaving a channel
func (b *BotAPI) handlePart(c echo.Context) error {
	// Authenticate the request
	if !b.authenticateRequest(c.Request()) {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}

	// Parse the request body
	var message BotMessage
	err := c.Bind(&message)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Bad request")
	}

	// Check if the required fields are present
	if message.Nickname == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Nickname is required")
	}
	if len(message.Channels) == 0 && message.Channel == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Channel is required")
	}

	// Find the client
	botClient := b.server.GetClient(message.Nickname)
	if botClient == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Bot not found")
	}

	// Leave the channels
	channels := message.Channels
	if message.Channel != "" {
		channels = append(channels, message.Channel)
	}

	reason := "Leaving"
	if message.Reason != "" {
		reason = message.Reason
	}

	for _, channelName := range channels {
		botClient.PartChannel(channelName, reason)
	}

	// Return success
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Left channels",
	})
}

// handleNick handles changing a nickname
func (b *BotAPI) handleNick(c echo.Context) error {
	// Authenticate the request
	if !b.authenticateRequest(c.Request()) {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}

	// Parse the request body
	var message BotMessage
	err := c.Bind(&message)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Bad request")
	}

	// Check if the required fields are present
	if message.Nickname == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Nickname is required")
	}
	if message.Target == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Target nickname is required")
	}

	// Find the client
	botClient := b.server.GetClient(message.Nickname)
	if botClient == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Bot not found")
	}

	// Check if the new nickname is available
	if b.server.GetClient(message.Target) != nil {
		return echo.NewHTTPError(http.StatusConflict, "Nickname already in use")
	}

	// Change the nickname
	oldNick := botClient.Nickname
	botClient.Nickname = message.Target

	// Notify all channels
	for _, channel := range botClient.Channels {
		channel.SendToAll(fmt.Sprintf(":%s!%s@%s NICK %s", oldNick, botClient.Username, botClient.Hostname, message.Target), nil)
	}

	// Return success
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Nickname changed",
	})
}

// handleMode handles changing a mode
func (b *BotAPI) handleMode(c echo.Context) error {
	// Authenticate the request
	if !b.authenticateRequest(c.Request()) {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}

	// Parse the request body
	var message BotMessage
	err := c.Bind(&message)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Bad request")
	}

	// Check if the required fields are present
	if message.Nickname == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Nickname is required")
	}
	if message.Target == "" && message.Channel == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Target or channel is required")
	}
	if message.Mode == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Mode is required")
	}

	// Find the client
	botClient := b.server.GetClient(message.Nickname)
	if botClient == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Bot not found")
	}

	// Change the mode
	if message.Channel != "" {
		// Channel mode
		channel := b.server.GetChannel(message.Channel)
		if channel == nil {
			return echo.NewHTTPError(http.StatusNotFound, "Channel not found")
		}

		// Parse the mode string
		modeSet := true
		for _, m := range message.Mode {
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
		channel.SendToAll(fmt.Sprintf(":%s!%s@%s MODE %s %s", botClient.Nickname, botClient.Username, botClient.Hostname, message.Channel, message.Mode), nil)
	} else {
		// User mode
		target := b.server.GetClient(message.Target)
		if target == nil {
			return echo.NewHTTPError(http.StatusNotFound, "Target not found")
		}

		// Parse the mode string
		modeSet := true
		for _, m := range message.Mode {
			if m == '+' {
				modeSet = true
				continue
			}
			if m == '-' {
				modeSet = false
				continue
			}

			// Set the mode
			target.SetMode(string(m), modeSet)
		}

		// Notify the client
		target.SendMessage(botClient.Nickname, "MODE", target.Nickname, message.Mode)
	}

	// Return success
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Mode changed",
	})
}

// handleTopic handles changing a topic
// handleTopic handles changing a topic
//
// The request body should contain the following fields:
//
// - Nickname: the nickname of the bot (optional)
// - Channel: the name of the channel to set the topic on (required)
// - Topic: the new topic text (required)
//
// # The bot will be able to set the topic even if it's restricted
//
// Returns a JSON object with a "success" field set to true and a "message" field set to "Topic changed" if the topic was set successfully
func (b *BotAPI) handleTopic(c echo.Context) error {
	// Authenticate the request
	if !b.authenticateRequest(c.Request()) {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}

	// Parse the request body
	var message BotMessage
	err := c.Bind(&message)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Bad request")
	}

	// Check if the required fields are present
	if message.Nickname == "" {
		message.Nickname = "BotAPI"
	}
	if message.Channel == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Channel is required")
	}
	if message.Topic == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Topic is required")
	}

	// Find the channel
	channel := b.server.GetChannel(message.Channel)
	if channel == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Channel not found")
	}

	// Create a virtual client for the bot
	botClient := &Client{
		ID:         fmt.Sprintf("bot-%s-%d", message.Nickname, time.Now().UnixNano()),
		Nickname:   message.Nickname,
		Username:   "botapi",
		Realname:   "Bot API",
		Hostname:   b.config.Server.Name,
		Server:     b.server,
		Registered: true,
		IsOper:     true, // Bot can set topic even if it's restricted
	}

	// Set the topic
	channel.SetTopic(message.Topic, botClient.Nickname)

	// Notify all members
	channel.SendToAll(fmt.Sprintf(":%s!%s@%s TOPIC %s :%s", botClient.Nickname, botClient.Username, botClient.Hostname, message.Channel, message.Topic), nil)

	// Return success
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Topic changed",
	})
}

// handleWho handles the WHO command
func (b *BotAPI) handleWho(c echo.Context) error {
	// Authenticate the request
	if !b.authenticateRequest(c.Request()) {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}

	// Parse the query parameters
	query := c.Request().URL.Query()
	mask := query.Get("mask")

	// Check if the mask is specified
	if mask == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Mask is required")
	}

	// Check if the mask is a channel
	if strings.HasPrefix(mask, "#") {
		channel := b.server.GetChannel(mask)
		if channel == nil {
			return echo.NewHTTPError(http.StatusNotFound, "Channel not found")
		}

		// Get the members
		channel.mu.RLock()
		members := make([]map[string]interface{}, 0, len(channel.Members))
		for _, member := range channel.Members {
			members = append(members, map[string]interface{}{
				"nickname": member.Nickname,
				"username": member.Username,
				"hostname": member.Hostname,
				"realname": member.Realname,
				"operator": member.IsOper,
			})
		}
		channel.mu.RUnlock()

		// Return the members
		return c.JSON(http.StatusOK, members)
	} else {
		// Check if the mask is a nickname
		clientVal, exists := b.server.clients.Load(mask)
		if !exists {
			return echo.NewHTTPError(http.StatusNotFound, "Client not found")
		}
		client := clientVal.(*Client)

		// Return the client
		return c.JSON(http.StatusOK, []map[string]interface{}{
			{
				"nickname": client.Nickname,
				"username": client.Username,
				"hostname": client.Hostname,
				"realname": client.Realname,
				"operator": client.IsOper,
			},
		})
	}
}

// handleList handles the LIST command
func (b *BotAPI) handleList(c echo.Context) error {
	// Authenticate the request
	if !b.authenticateRequest(c.Request()) {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}

	// Parse the query parameters
	query := c.Request().URL.Query()
	mask := query.Get("mask")

	// Get the channels
	channels := make([]map[string]interface{}, 0)
	b.server.channels.Range(func(key, channelVal interface{}) bool {
		channel := channelVal.(*Channel)
		name := key.(string)
		// If a mask is specified, filter the channels
		if mask != "" && !strings.Contains(name, mask) {
			return true
		}

		channels = append(channels, map[string]interface{}{
			"name":  name,
			"topic": channel.Topic,
			"users": channel.MemberCount(),
			"modes": channel.GetModeString(),
			"setby": channel.TopicSetBy,
			"setat": channel.TopicSetAt,
		})
		return true // Continue iteration
	}) // Close the Range callback

	// Return the channels
	return c.JSON(http.StatusOK, channels)
}

// authenticateRequest authenticates a request using the bearer token
func (b *BotAPI) authenticateRequest(req *http.Request) bool {
	// Get the authorization header
	authHeader := req.Header.Get("Authorization")
	if authHeader == "" {
		return false
	}

	// Check if it's a bearer token
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return false
	}

	// Extract the token
	token := strings.TrimPrefix(authHeader, "Bearer ")

	// Check if the token is valid
	for _, validToken := range b.config.Bots.BearerTokens {
		if subtle.ConstantTimeCompare([]byte(token), []byte(validToken)) == 1 {
			return true
		}
	}

	return false
}
