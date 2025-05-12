package server

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/presbrey/pkg/irc"
)

// Client represents a connected IRC client
type Client struct {
	ID          string
	Nickname    string
	Username    string
	Realname    string
	Hostname    string
	IP          string
	Modes       UserModes
	Channels    map[string]*Channel
	Server      *Server
	Conn        net.Conn
	LastPing    time.Time
	Registered  bool
	Away        bool
	AwayMessage string
	IsOper      bool
	mu          sync.RWMutex
	quit        chan struct{}
}

// NewClient creates a new client
func NewClient(server *Server, conn net.Conn) *Client {
	// Extract the client's IP address
	ip, _, _ := net.SplitHostPort(conn.RemoteAddr().String())

	return &Client{
		ID:       uuid.New().String(),
		Server:   server,
		Conn:     conn,
		IP:       ip,
		Hostname: ip, // Initially set hostname to IP
		Channels: make(map[string]*Channel),
		LastPing: time.Now(),
		quit:     make(chan struct{}),
		Modes:    NewUserModes(),
	}
}

// Handle handles the client connection
func (c *Client) Handle() {
	defer c.cleanup()

	// Send welcome messages
	c.SendRaw(fmt.Sprintf(":%s NOTICE Auth :*** Looking up your hostname...", c.Server.GetConfig().Server.Name))
	c.SendRaw(fmt.Sprintf(":%s NOTICE Auth :*** Found your hostname", c.Server.GetConfig().Server.Name))

	// Start goroutines for reading from and writing to the client
	go c.pingLoop()

	reader := bufio.NewReader(c.Conn)
	for {
		// Read a line from the client
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}

		// Trim whitespace
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parse the message
		msg := irc.ParseMessage(line)
		if msg == nil {
			continue
		}

		// Handle the message
		if err := c.handleMessage(msg, line); err != nil {
			fmt.Printf("Error handling message: %v\n", err)
			break
		}
	}
}

// handleMessage handles an IRC message
func (c *Client) handleMessage(msg *irc.Message, raw string) error {
	// Update last activity time for ping/pong tracking
	c.LastPing = time.Now()

	// Create hook parameters
	params := &HookParams{
		Server:   c.Server,
		Client:   c,
		Message:  msg,
		RawInput: raw,
		Data:     make(map[string]interface{}),
	}

	// Set additional parameters based on the message
	if len(msg.Params) > 0 {
		params.Target = msg.Params[0]
		if len(msg.Params) > 1 {
			params.Text = msg.Params[1]
		}
	}

	// Run hooks for the command
	return c.Server.RunHooks(msg.Command, params)
}

// SendRaw sends a raw message to the client
func (c *Client) SendRaw(message string) {
	// Ensure the message ends with CRLF
	if !strings.HasSuffix(message, "\r\n") {
		message += "\r\n"
	}

	c.Conn.Write([]byte(message))
}

// SendMessage sends an IRC message to the client
func (c *Client) SendMessage(prefix, command string, params ...string) {
	msg := &irc.Message{
		Prefix:  prefix,
		Command: command,
		Params:  params,
	}
	c.SendRaw(msg.String())
}

// pingLoop sends pings to the client to check if they're still connected
func (c *Client) pingLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Check if the client hasn't responded to a ping for too long
			if time.Since(c.LastPing) > 2*time.Minute {
				c.Quit("Ping timeout")
				return
			}

			// Send a ping
			c.SendMessage(c.Server.GetConfig().Server.Name, "PING", c.Server.GetConfig().Server.Name)
		case <-c.quit:
			return
		}
	}
}

// Quit disconnects the client with a quit message
func (c *Client) Quit(message string) {
	// Make sure we only close the channel once
	c.mu.Lock()
	select {
	case <-c.quit:
		// Already closed
		c.mu.Unlock()
		return
	default:
		close(c.quit)
	}
	c.mu.Unlock()

	// Send a quit message to all channels the client is in
	for _, channel := range c.Channels {
		channel.SendToAll(fmt.Sprintf(":%s!%s@%s QUIT :%s", c.Nickname, c.Username, c.Hostname, message), c)
	}

	// Remove the client from the server
	c.Server.RemoveClient(c)

	// Ensure the connection is properly closed
	if c.Conn != nil {
		c.Conn.SetReadDeadline(time.Now()) // Force any pending reads to fail immediately
		c.Conn.Close()                      // Explicitly close the connection
	}
}

// cleanup cleans up resources when the client disconnects
func (c *Client) cleanup() {
	// Remove the client from the server
	c.Server.RemoveClient(c)

	// Close the connection
	c.Conn.Close()
}

// SendWelcome sends the welcome messages to the client
func (c *Client) SendWelcome() {
	serverName := c.Server.GetConfig().Server.Name
	networkName := c.Server.GetConfig().Server.Network

	// Send the initial welcome messages
	c.SendMessage(serverName, "001", c.Nickname, fmt.Sprintf("Welcome to the %s IRC Network %s!%s@%s", networkName, c.Nickname, c.Username, c.Hostname))
	c.SendMessage(serverName, "002", c.Nickname, fmt.Sprintf("Your host is %s, running version GoIRCd-1.0", serverName))
	c.SendMessage(serverName, "003", c.Nickname, fmt.Sprintf("This server was created %s", c.Server.startTime.Format(time.RFC1123)))
	c.SendMessage(serverName, "004", c.Nickname, serverName, "GoIRCd-1.0", "iwosxz", "biklmnopstv")

	// Send MOTD
	c.SendMessage(serverName, "375", c.Nickname, fmt.Sprintf("- %s Message of the Day -", serverName))
	c.SendMessage(serverName, "372", c.Nickname, "- Welcome to GoIRCd!")
	c.SendMessage(serverName, "372", c.Nickname, "- This server is running GoIRCd, a Go IRC Server")
	c.SendMessage(serverName, "376", c.Nickname, "End of /MOTD command")
}

// JoinChannel makes the client join a channel
func (c *Client) JoinChannel(channelName string) {
	// Check if the channel exists, create it if not
	channel := c.Server.GetChannel(channelName)
	if channel == nil {
		channel = c.Server.CreateChannel(channelName)
	}

	// Add the client to the channel
	channel.AddMember(c)

	// Add the channel to the client's channel list
	c.mu.Lock()
	c.Channels[channelName] = channel
	c.mu.Unlock()

	// Send join message to all members
	channel.SendToAll(fmt.Sprintf(":%s!%s@%s JOIN %s", c.Nickname, c.Username, c.Hostname, channelName), nil)

	// Send the channel topic
	if channel.Topic != "" {
		c.SendMessage(c.Server.GetConfig().Server.Name, "332", c.Nickname, channelName, channel.Topic)
	} else {
		c.SendMessage(c.Server.GetConfig().Server.Name, "331", c.Nickname, channelName, "No topic is set")
	}

	// Send the list of users in the channel
	channel.SendNames(c)
}

// PartChannel makes the client leave a channel
func (c *Client) PartChannel(channelName, reason string) {
	// Check if the client is in the channel
	c.mu.RLock()
	channel, ok := c.Channels[channelName]
	c.mu.RUnlock()

	if !ok {
		return
	}

	// Send part message to all members
	channel.SendToAll(fmt.Sprintf(":%s!%s@%s PART %s :%s", c.Nickname, c.Username, c.Hostname, channelName, reason), nil)

	// Remove the client from the channel
	channel.RemoveMember(c)

	// Remove the channel from the client's channel list
	c.mu.Lock()
	delete(c.Channels, channelName)
	c.mu.Unlock()

	// If the channel is now empty, remove it
	if channel.MemberCount() == 0 {
		c.Server.RemoveChannel(channelName)
	}
}

// SendPrivmsg sends a private message to the client
func (c *Client) SendPrivmsg(sender *Client, message string) {
	c.SendRaw(fmt.Sprintf(":%s!%s@%s PRIVMSG %s :%s", sender.Nickname, sender.Username, sender.Hostname, c.Nickname, message))
}

// SetMode sets a mode for the client
func (c *Client) SetMode(mode string, enable bool) {
	// Parse the mode string
	for _, m := range mode {
		if enable {
			c.Modes.SetMode(m)
		} else {
			c.Modes.UnsetMode(m)
		}
	}

	// Notify the client about the mode change
	modeStr := "+"
	if !enable {
		modeStr = "-"
	}
	modeStr += mode
	c.SendMessage(c.Server.GetConfig().Server.Name, "MODE", c.Nickname, modeStr)
}

// SetAway sets the client's away status
func (c *Client) SetAway(away bool, message string) {
	c.mu.Lock()
	c.Away = away
	c.AwayMessage = message
	c.mu.Unlock()

	if away {
		c.SendMessage(c.Server.GetConfig().Server.Name, "306", c.Nickname, "You have been marked as being away")
	} else {
		c.SendMessage(c.Server.GetConfig().Server.Name, "305", c.Nickname, "You are no longer marked as being away")
	}
}

// SetOper sets the client's operator status
func (c *Client) SetOper(isOper bool) {
	c.mu.Lock()
	c.IsOper = isOper
	c.mu.Unlock()

	if isOper {
		c.SendMessage(c.Server.GetConfig().Server.Name, "381", c.Nickname, "You are now an IRC operator")
		c.SetMode("o", true)
	} else {
		c.SetMode("o", false)
	}
}
