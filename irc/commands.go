package irc

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
	"time"
)

// handleKick handles a KICK command
func (c *Client) handleKick(params []string) {
	if !c.registered {
		c.sendNumeric(451, ":You have not registered")
		return
	}

	if len(params) < 2 {
		c.sendNumeric(461, "KICK :Not enough parameters")
		return
	}

	channelName := params[0]
	targetNick := params[1]

	reason := "No reason"
	if len(params) > 2 {
		reason = params[2]
	}

	// Check if the channel exists
	c.server.RLock()
	channel, exists := c.server.channels[channelName]
	c.server.RUnlock()

	if !exists {
		c.sendNumeric(403, fmt.Sprintf("%s :No such channel", channelName))
		return
	}

	// Check if the client is on the channel
	channel.RLock()
	_, isOnChannel := channel.clients[c.nickname]
	nokicks := strings.Contains(channel.modes, "Q")
	channel.RUnlock()

	if !isOnChannel {
		c.sendNumeric(442, fmt.Sprintf("%s :You're not on that channel", channelName))
		return
	}

	// Check if the client has kick privileges
	channel.RLock()
	isOperator := channel.operators[c.nickname] || channel.owners[c.nickname] ||
		channel.admins[c.nickname] || channel.halfops[c.nickname] || c.Modes.Operator
	channel.RUnlock()

	if !isOperator {
		c.sendNumeric(482, fmt.Sprintf("%s :You're not a channel operator", channelName))
		return
	}

	// Check if the target exists and is on the channel
	channel.RLock()
	targetClient, targetOnChannel := channel.clients[targetNick]
	channel.RUnlock()

	if !targetOnChannel {
		c.sendNumeric(441, fmt.Sprintf("%s %s :They aren't on that channel",
			targetNick, channelName))
		return
	}

	// Check for NOKICK mode (Q)
	if nokicks && !c.Modes.Operator {
		c.sendNumeric(482, fmt.Sprintf("%s :Channel is +Q, can't kick users", channelName))
		return
	}

	// Check if the target has higher privileges than the kicker
	channel.RLock()
	targetIsOwner := channel.owners[targetNick]
	targetIsAdmin := channel.admins[targetNick]
	targetIsOp := channel.operators[targetNick]
	kickerIsOwner := channel.owners[c.nickname]
	kickerIsAdmin := channel.admins[c.nickname]
	kickerIsOp := channel.operators[c.nickname]
	channel.RUnlock()

	targetClient.RLock()
	targetIsServerOp := targetClient.Modes.Operator
	targetClient.RUnlock()

	// Privilege hierarchy: ServerOp > Owner > Admin > Op
	if (targetIsServerOp && !c.Modes.Operator) ||
		(targetIsOwner && !kickerIsOwner && !c.Modes.Operator) ||
		(targetIsAdmin && !kickerIsOwner && !kickerIsAdmin && !c.Modes.Operator) ||
		(targetIsOp && !kickerIsOwner && !kickerIsAdmin && !kickerIsOp && !c.Modes.Operator) {
		c.sendNumeric(482, fmt.Sprintf("%s :You cannot kick this user", channelName))
		return
	}

	// Perform the kick
	kickMsg := fmt.Sprintf(":%s!%s@%s KICK %s %s :%s",
		c.nickname, c.username, c.hostname, channelName, targetNick, reason)

	// Notify all clients in the channel
	channel.RLock()
	for _, client := range channel.clients {
		client.sendRaw(kickMsg)
	}
	channel.RUnlock()

	// Remove the target from the channel
	channel.Lock()
	delete(channel.clients, targetNick)
	delete(channel.operators, targetNick)
	delete(channel.voices, targetNick)
	delete(channel.halfops, targetNick)
	delete(channel.admins, targetNick)
	delete(channel.owners, targetNick)
	channel.Unlock()

	// Remove the channel from the target's list
	targetClient.Lock()
	delete(targetClient.channels, channelName)
	targetClient.Unlock()
}

// handleVersion handles a VERSION command
func (c *Client) handleVersion(_ []string) {
	if !c.registered {
		c.sendNumeric(451, ":You have not registered")
		return
	}

	serverName := c.server.config.ServerName
	version := "GoIRC-1.0"
	osInfo := fmt.Sprintf("%s %s [%s]", runtime.GOOS, runtime.GOARCH, os.Getenv("GOVERSION"))

	c.sendNumeric(351, fmt.Sprintf("%s %s :%s", version, serverName, osInfo))
}

// handleAdmin handles an ADMIN command
func (c *Client) handleAdmin(_ []string) {
	if !c.registered {
		c.sendNumeric(451, ":You have not registered")
		return
	}

	c.sendNumeric(256, fmt.Sprintf("%s :Administrative info", c.server.config.ServerName))
	c.sendNumeric(257, fmt.Sprintf(":%s", c.server.config.ServerDesc))
	c.sendNumeric(258, ":Administrative contact at admin@example.com")
	c.sendNumeric(259, fmt.Sprintf(":%s server", c.server.config.NetworkName))
}

// handleInfo handles an INFO command
func (c *Client) handleInfo(_ []string) {
	if !c.registered {
		c.sendNumeric(451, ":You have not registered")
		return
	}

	c.sendNumeric(371, ":GoIRC Server Information:")
	c.sendNumeric(371, ":This IRC server is powered by GoIRC.")
	c.sendNumeric(371, ":Built with Go and configured with environment variables.")
	c.sendNumeric(371, fmt.Sprintf(":Server started at %s", c.server.stats.StartTime.Format(time.RFC1123)))
	c.sendNumeric(374, ":End of INFO list")
}

// handleTime handles a TIME command
func (c *Client) handleTime(_ []string) {
	if !c.registered {
		c.sendNumeric(451, ":You have not registered")
		return
	}

	now := time.Now()
	c.sendNumeric(391, fmt.Sprintf("%s :%s",
		c.server.config.ServerName, now.Format(time.RFC1123)))
}

// handleMotd handles a MOTD command
func (c *Client) handleMotd(_ []string) {
	if !c.registered {
		c.sendNumeric(451, ":You have not registered")
		return
	}

	// In a real implementation, this would load from a file or database
	c.sendNumeric(375, fmt.Sprintf(":- %s Message of the Day -", c.server.config.ServerName))
	c.sendNumeric(372, fmt.Sprintf(":- Welcome to %s", c.server.config.ServerDesc))
	c.sendNumeric(372, ":- This server is running GoIRC server software")
	c.sendNumeric(372, ":- Enjoy your stay!")
	c.sendNumeric(376, ":End of MOTD command")
}

// handleStats handles a STATS command for server statistics (operator only)
func (c *Client) handleStats(params []string) {
	if !c.registered {
		c.sendNumeric(451, ":You have not registered")
		return
	}

	if len(params) < 1 {
		c.sendNumeric(461, "STATS :Not enough parameters")
		return
	}

	flag := params[0]

	// Most stats commands require operator status
	if !c.Modes.Operator && flag != "u" {
		c.sendNumeric(481, ":Permission Denied - You're not an IRC operator")
		return
	}

	switch flag {
	case "u": // Uptime
		uptime := time.Since(c.server.stats.StartTime)
		c.sendNumeric(242, fmt.Sprintf(":Server Up %d days, %.2f hours",
			int(uptime.Hours()/24), uptime.Hours()))

	case "o": // Oper list
		c.server.RLock()
		for username := range c.server.opCredentials {
			c.sendNumeric(243, fmt.Sprintf("O %s * %s", username, c.server.config.ServerName))
		}

		for email := range c.server.operators {
			c.sendNumeric(243, fmt.Sprintf("O %s * %s (OIDC)", email, c.server.config.ServerName))
		}
		c.server.RUnlock()

	case "k": // K-line list
		c.server.RLock()
		for mask, ban := range c.server.klines {
			expiryInfo := "permanent"
			if !ban.ExpiryTime.IsZero() {
				if time.Now().After(ban.ExpiryTime) {
					expiryInfo = "expired"
				} else {
					remaining := time.Until(ban.ExpiryTime).Round(time.Second)
					expiryInfo = fmt.Sprintf("expires in %s", remaining)
				}
			}

			c.sendNumeric(216, fmt.Sprintf("k %s * %s (%s) %s",
				mask, ban.SetTime.Format("2006-01-02 15:04:05"), ban.Setter, expiryInfo))
		}
		c.server.RUnlock()

	case "g": // G-line list
		c.server.RLock()
		for mask, ban := range c.server.glines {
			expiryInfo := "permanent"
			if !ban.ExpiryTime.IsZero() {
				if time.Now().After(ban.ExpiryTime) {
					expiryInfo = "expired"
				} else {
					remaining := time.Until(ban.ExpiryTime).Round(time.Second)
					expiryInfo = fmt.Sprintf("expires in %s", remaining)
				}
			}

			c.sendNumeric(216, fmt.Sprintf("g %s * %s (%s) %s",
				mask, ban.SetTime.Format("2006-01-02 15:04:05"), ban.Setter, expiryInfo))
		}
		c.server.RUnlock()

	case "m": // Command statistics
		c.sendNumeric(212, fmt.Sprintf("M %s %d :Messages Received",
			c.server.config.ServerName, c.server.stats.MessagesReceived))
		c.sendNumeric(212, fmt.Sprintf("M %s %d :Messages Sent",
			c.server.config.ServerName, c.server.stats.MessagesSent))

	case "c": // Connection statistics
		c.sendNumeric(213, fmt.Sprintf("C %s %d :Current Connections",
			c.server.config.ServerName, len(c.server.clients)))
		c.sendNumeric(213, fmt.Sprintf("C %s %d :Peak Connections",
			c.server.config.ServerName, c.server.stats.MaxConnections))
		c.sendNumeric(213, fmt.Sprintf("C %s %d :Maximum Connection Limit",
			c.server.config.ServerName, c.server.config.MaxConnections))
		c.sendNumeric(213, fmt.Sprintf("C %s %d :K-line Hits",
			c.server.config.ServerName, c.server.stats.KlineHits))
		c.sendNumeric(213, fmt.Sprintf("C %s %d :G-line Hits",
			c.server.config.ServerName, c.server.stats.GlineHits))

	default:
		c.sendNumeric(219, fmt.Sprintf("%s :End of STATS report", flag))
		return
	}

	c.sendNumeric(219, fmt.Sprintf("%s :End of STATS report", flag))
}

// handleWallops handles a WALLOPS command (operator only)
func (c *Client) handleWallops(params []string) {
	if !c.registered {
		c.sendNumeric(451, ":You have not registered")
		return
	}

	// Verify operator status
	if !c.Modes.Operator {
		c.sendNumeric(481, ":Permission Denied - You're not an IRC operator")
		return
	}

	// Check for message content
	if len(params) < 1 {
		c.sendNumeric(461, "WALLOPS :Not enough parameters")
		return
	}

	message := params[0]
	log.Printf("[WALLOPS] %s: %s", c.nickname, message)

	// Format the wallops message
	wallopsMsg := fmt.Sprintf(":%s!%s@%s WALLOPS :%s",
		c.nickname, c.username, c.hostname, message)

	// Send to all users with +w mode
	c.server.RLock()
	for _, client := range c.server.clients {
		client.RLock()
		if client.Modes.Wallops || client.Modes.Operator {
			client.sendRaw(wallopsMsg)
		}
		client.RUnlock()
	}
	c.server.RUnlock()
}

// handleLusers handles a LUSERS command for network statistics
func (c *Client) handleLusers(_ []string) {
	if !c.registered {
		c.sendNumeric(451, ":You have not registered")
		return
	}

	// Count users, operators, and channels
	totalUsers := 0
	invisibleUsers := 0
	operators := 0
	channels := 0

	c.server.RLock()
	totalUsers = len(c.server.clients)
	channels = len(c.server.channels)

	for _, client := range c.server.clients {
		client.RLock()
		if client.Modes.Invisible {
			invisibleUsers++
		}
		if client.Modes.Operator {
			operators++
		}
		client.RUnlock()
	}
	c.server.RUnlock()

	// Calculate visible users
	visibleUsers := totalUsers - invisibleUsers

	// Send the LUSERS information using standard numeric replies
	c.sendNumeric(251, fmt.Sprintf(":There are %d users and %d invisible on 1 server",
		visibleUsers, invisibleUsers))
	c.sendNumeric(252, fmt.Sprintf("%d :IRC Operators online", operators))
	c.sendNumeric(254, fmt.Sprintf("%d :channels formed", channels))
	c.sendNumeric(255, fmt.Sprintf(":I have %d clients and 1 server", totalUsers))

	// Send local and global user information
	c.sendNumeric(265, fmt.Sprintf("%d %d :Current local users %d, max %d",
		totalUsers, c.server.stats.PeakUserCount, totalUsers, c.server.stats.PeakUserCount))
	c.sendNumeric(266, fmt.Sprintf("%d %d :Current global users %d, max %d",
		totalUsers, c.server.stats.PeakUserCount, totalUsers, c.server.stats.PeakUserCount))
}

// handleAway handles the AWAY command, which marks a user as away or returns them from away status
func (c *Client) handleAway(params []string) {
	if !c.registered {
		c.sendNumeric(451, ":You have not registered")
		return
	}

	// If no parameters provided, turn off away status
	if len(params) == 0 {
		c.Lock()
		c.awayMessage = ""
		c.Modes.Away = false
		c.Unlock()

		// RPL_UNAWAY
		c.sendNumeric(305, ":You are no longer marked as being away")
		return
	}

	// Set away message and enable away mode
	awayMessage := params[0]

	c.Lock()
	c.awayMessage = awayMessage
	c.Modes.Away = true
	c.Unlock()

	// RPL_NOWAWAY
	c.sendNumeric(306, ":You have been marked as being away")

	// Log for debugging
	log.Printf("[%s] User marked as away: %s", c.hostname, awayMessage)
}

// sendError sends an ERROR message to the client
func (c *Client) sendError(message string) {
	c.sendRaw(fmt.Sprintf("ERROR :%s", message))
}
