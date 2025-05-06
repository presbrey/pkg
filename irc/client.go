package irc

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"net/textproto"
	"strings"
	"sync"
	"time"
)

// Client represents a connected IRC client
type Client struct {
	sync.RWMutex
	conn        net.Conn
	server      *Server
	nickname    string
	username    string
	realname    string
	hostname    string
	email       string // Authenticated email address (if operator)
	password    string // Connection password from PASS command
	channels    map[string]bool
	registered  bool
	lastPong    time.Time
	writer      *bufio.Writer
	writeLock   sync.Mutex
	quitting    bool
	awayMessage string // Message displayed when user is away

	Modes UserMode // User modes

	// Peering support
	RemoteOrigin bool   // True if this client is from a remote server
	RemoteServer string // The name of the remote server this client is from

	capabilities *ClientCapabilities // Tracks client capability negotiation and enabled capabilities
}

// handleConnection handles a client connection
func (c *Client) handleConnection() {
	defer func() {
		c.quit("Connection closed")
	}()

	c.hostname = c.conn.RemoteAddr().String()
	log.Printf("[%s] *** New client connected", c.hostname)

	// Use textproto for reliable line-oriented protocol handling
	textReader := textproto.NewReader(bufio.NewReader(c.conn))

	// Set a read deadline for client registration
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))

	for {
		// ReadLine properly handles CRLF and other text protocol nuances
		line, err := textReader.ReadLine()
		if err != nil {
			if err != io.EOF {
				log.Printf("[%s] Error reading from client: %v", c.hostname, err)
			} else {
				log.Printf("[%s] Client disconnected", c.hostname)
			}
			break
		}

		// textproto.ReadLine already removes CRLF, just check for empty lines
		if line == "" {
			continue
		}

		// Handle the command
		c.handleCommand(line)
	}
}

// handleCommand handles an IRC command
func (c *Client) handleCommand(line string) {
	// Debug the exact command being processed
	log.Printf("[%s] <= %#v", c.hostname, line)

	// Parse the command
	var command string
	var params []string

	if len(line) > 0 && line[0] == ':' {
		parts := strings.SplitN(line[1:], " ", 2)
		_ = parts[0] // Ignore the prefix for now
		if len(parts) > 1 {
			line = parts[1]
		} else {
			return // No command after prefix
		}
	}

	parts := strings.SplitN(line, " ", 2)
	command = strings.ToUpper(parts[0])

	if len(parts) > 1 && len(parts[1]) > 0 {
		if parts[1][0] == ':' {
			params = append(params, parts[1][1:])
		} else {
			paramParts := strings.Split(parts[1], " ")
			for i, p := range paramParts {
				if len(p) > 0 && p[0] == ':' {
					trailing := strings.Join(paramParts[i:], " ")
					params = append(params, trailing[1:])
					break
				}
				params = append(params, p)
			}
		}
	}

	// Handle commands
	switch command {
	case "PASS":
		c.handlePass(params)
	case "PING":
		c.handlePing(params)
	case "PONG":
		c.handlePong(params)
	case "NICK":
		c.handleNick(params)
	case "USER":
		c.handleUser(params)
	case "CAP":
		c.handleCAP(params)
	case "JOIN":
		c.handleJoin(params)
	case "PART":
		c.handlePart(params)
	case "PRIVMSG":
		c.handlePrivmsg(params)
	case "NOTICE":
		c.handleNotice(params)
	case "QUIT":
		var reason string
		if len(params) > 0 {
			reason = params[0]
		} else {
			reason = "Quit"
		}
		c.quit(reason)
	case "MODE":
		c.handleMode(params)
	case "TOPIC":
		c.handleTopic(params)
	case "LIST":
		c.handleList(params)
	case "NAMES":
		c.handleNames(params)
	case "WHO":
		c.handleWho(params)
	case "WHOIS":
		c.handleWhois(params)
	case "OPER":
		c.handleOper(params)
	case "AWAY":
		c.handleAway(params)
	case "INVITE":
		c.handleInvite(params)
	case "KICK":
		c.handleKick(params)
	case "KLINE":
		c.handleKline(params)
	case "UNKLINE":
		c.handleUnkline(params)
	case "GLINE":
		c.handleGline(params)
	case "UNGLINE":
		c.handleUngline(params)
	case "KILL":
		c.handleKill(params)
	case "VERSION":
		c.handleVersion(params)
	case "ADMIN":
		c.handleAdmin(params)
	case "INFO":
		c.handleInfo(params)
	case "TIME":
		c.handleTime(params)
	case "MOTD":
		c.handleMotd(params)
	case "STATS":
		c.handleStats(params)
	case "WALLOPS":
		c.handleWallops(params)
	case "LUSERS":
		c.handleLusers(params)
	default:
		c.sendNumeric(ERR_UNKNOWNCOMMAND, fmt.Sprintf("%s :Unknown command", command))
	}

	// Update statistics
	c.server.stats.Lock()
	c.server.stats.MessagesReceived++
	c.server.stats.Unlock()
}

// handlePing handles a PING command
func (c *Client) handlePing(params []string) {
	if len(params) < 1 {
		c.sendNumeric(ERR_NEEDMOREPARAMS, "PING :Not enough parameters")
		return
	}

	c.sendMessage("PONG", params[0])
}

// handlePong handles a PONG command
func (c *Client) handlePong(_ []string) {
	c.lastPong = time.Now()
}

// handlePass handles a PASS command
func (c *Client) handlePass(params []string) {
	// If already registered, ignore PASS
	if c.registered {
		c.sendNumeric(ERR_ALREADYREGISTRED, ":You may not reregister")
		return
	}

	// Check for enough parameters
	if len(params) < 1 {
		c.sendNumeric(ERR_NEEDMOREPARAMS, "PASS :Not enough parameters")
		return
	}

	// Store the password
	c.password = params[0]
}

// handleNick handles a NICK command
func (c *Client) handleNick(params []string) {
	if len(params) < 1 {
		c.sendNumeric(ERR_NONICKNAMEGIVEN, ":No nickname given")
		return
	}

	newNick := params[0]

	// Send notice about nickname handling
	c.server.SendNickChangesNotice(fmt.Sprintf("Client %s attempting to change nickname to: %s", c.nickname, newNick))

	// Check if the nickname is valid
	if !isValidNickname(newNick) {
		c.sendNumeric(ERR_ERRONEUSNICKNAME, fmt.Sprintf("%s :Erroneous nickname", newNick))
		return
	}

	c.server.Lock()
	defer c.server.Unlock()

	// Check if the nickname is already in use
	if _, exists := c.server.clients[newNick]; exists {
		c.sendNumeric(ERR_NICKNAMEINUSE, fmt.Sprintf("%s :Nickname is already in use", newNick))
		return
	}

	// If this is a nickname change
	if c.nickname != "" {
		oldNick := c.nickname
		delete(c.server.clients, oldNick)

		// Announce the nickname change
		message := fmt.Sprintf(":%s NICK %s", oldNick, newNick)
		c.sendRaw(message)

		// Announce to all common channels
		for channelName := range c.channels {
			if channel, exists := c.server.channels[channelName]; exists {
				for _, memberClient := range channel.clients {
					if memberClient != c {
						memberClient.sendRaw(message)
					}
				}
			}
		}
	}

	c.nickname = newNick
	c.server.clients[newNick] = c

	// If this is the first time setting nick, check if we can complete registration
	if !c.registered && c.username != "" {
		c.tryCompleteRegistration()
	}
}

// handleUser handles a USER command
func (c *Client) handleUser(params []string) {
	if c.registered {
		c.sendNumeric(462, ":You may not reregister")
		return
	}

	if len(params) < 4 {
		c.sendNumeric(ERR_NEEDMOREPARAMS, "USER :Not enough parameters")
		return
	}

	c.username = params[0]
	c.realname = params[3]

	// If we already have a nickname, complete registration
	if c.nickname != "" {
		c.tryCompleteRegistration()
	}
}

// completeRegistration completes the client registration process
func (c *Client) completeRegistration() {
	// Check if all required registration information has been provided
	if c.nickname == "" || c.username == "" {
		return
	}

	// Check if a connection password is required
	if c.server.Config.ConnectionPassword != "" {
		// If password is required but not provided, reject
		if c.password == "" {
			c.sendNumeric(ERR_PASSWDMISMATCH, ":Password required")
			return
		}

		// If password is incorrect, reject
		if c.password != c.server.Config.ConnectionPassword {
			c.sendNumeric(464, ":Password incorrect")
			return
		}

		// Password correct, continue with registration
	}

	// If client is in CAP negotiation, wait for CAP END
	if c.capabilities.Negotiating {
		return
	}

	// Send registration confirmation notice to operators
	c.server.SendStatsLinksNotice(fmt.Sprintf("Client registered: %s!%s@%s", c.nickname, c.username, c.hostname))

	c.registered = true

	// Clear the registration deadline
	c.conn.SetReadDeadline(time.Time{})

	// Send welcome messages
	c.sendNumeric(RPL_WELCOME, fmt.Sprintf(":Welcome to the %s IRC Network %s!%s@%s",
		c.server.Config.NetworkName, c.nickname, c.username, c.hostname))
	c.sendNumeric(RPL_YOURHOST, fmt.Sprintf(":Your host is %s, running version goirc-1.0",
		c.server.Config.ServerName))
	c.sendNumeric(RPL_CREATED, fmt.Sprintf(":This server was created %s", time.Now().Format(time.RFC1123)))
	c.sendNumeric(RPL_MYINFO, fmt.Sprintf("%s goirc-1.0 o o", c.server.Config.ServerName))

	// Send MOTD
	c.sendNumeric(RPL_MOTDSTART, fmt.Sprintf(":- %s Message of the Day -", c.server.Config.ServerName))
	c.sendNumeric(RPL_MOTD, fmt.Sprintf(":- Welcome to %s", c.server.Config.ServerDesc))
	c.sendNumeric(RPL_ENDOFMOTD, ":End of MOTD command")
}

// tryCompleteRegistration attempts to complete the registration process
// This is called after NICK/USER commands and after CAP END
func (c *Client) tryCompleteRegistration() {
	// Skip if already registered
	if c.registered {
		return
	}

	// Complete registration if we have all the required information
	c.completeRegistration()
}

// handlePart handles a PART command
func (c *Client) handlePart(params []string) {
	if !c.registered {
		c.sendNumeric(451, ":You have not registered")
		return
	}

	if len(params) < 1 {
		c.sendNumeric(461, "PART :Not enough parameters")
		return
	}

	channelNames := strings.Split(params[0], ",")
	reason := ""
	if len(params) > 1 {
		reason = params[1]
	}

	for _, channelName := range channelNames {
		c.server.RLock()
		channel, exists := c.server.channels[channelName]
		c.server.RUnlock()

		if !exists {
			c.sendNumeric(403, fmt.Sprintf("%s :No such channel", channelName))
			continue
		}

		c.Lock()
		if _, isMember := c.channels[channelName]; !isMember {
			c.sendNumeric(442, fmt.Sprintf("%s :You're not on that channel", channelName))
			c.Unlock()
			continue
		}
		delete(c.channels, channelName)
		c.Unlock()

		partMsg := fmt.Sprintf(":%s!%s@%s PART %s :%s",
			c.nickname, c.username, c.hostname, channelName, reason)

		channel.Lock()
		// Announce to all clients in the channel
		for _, client := range channel.clients {
			client.sendRaw(partMsg)
		}

		// Remove the client from the channel
		delete(channel.clients, c.nickname)
		delete(channel.operators, c.nickname)

		// If the channel is empty, remove it
		if len(channel.clients) == 0 {
			c.server.Lock()
			delete(c.server.channels, channelName)
			c.server.Unlock()
		}
		channel.Unlock()
	}
}

// handlePrivmsg handles a PRIVMSG command
func (c *Client) handlePrivmsg(params []string) {
	if !c.registered {
		c.sendNumeric(451, ":You have not registered")
		return
	}

	if len(params) < 2 {
		c.sendNumeric(461, "PRIVMSG :Not enough parameters")
		return
	}

	target := params[0]
	message := params[1]

	if target[0] == '#' || target[0] == '&' {
		// Channel message
		c.server.RLock()
		channel, exists := c.server.channels[target]
		c.server.RUnlock()

		if !exists {
			c.sendNumeric(401, fmt.Sprintf("%s :No such nick/channel", target))
			return
		}

		msg := fmt.Sprintf(":%s!%s@%s PRIVMSG %s :%s",
			c.nickname, c.username, c.hostname, target, message)

		channel.RLock()
		for _, client := range channel.clients {
			if client != c {
				client.sendRaw(msg)
			}
		}
		channel.RUnlock()
	} else {
		// Private message
		c.server.RLock()
		targetClient, exists := c.server.clients[target]
		c.server.RUnlock()

		if !exists {
			c.sendNumeric(401, fmt.Sprintf("%s :No such nick/channel", target))
			return
		}

		msg := fmt.Sprintf(":%s!%s@%s PRIVMSG %s :%s",
			c.nickname, c.username, c.hostname, target, message)
		targetClient.sendRaw(msg)

		// Check if target user is away, and send automatic reply if so
		targetClient.RLock()
		isAway := targetClient.Modes.Away
		awayMessage := targetClient.awayMessage
		targetClient.RUnlock()

		if isAway && awayMessage != "" {
			// Send automatic away response - RPL_AWAY
			awayReply := fmt.Sprintf(":%s!%s@%s NOTICE %s :%s is away: %s",
				targetClient.nickname, targetClient.username, targetClient.hostname,
				c.nickname, targetClient.nickname, awayMessage)
			c.sendRaw(awayReply)
		}
	}
}

// handleNotice handles a NOTICE command
func (c *Client) handleNotice(params []string) {
	if !c.registered {
		c.sendNumeric(451, ":You have not registered")
		return
	}

	if len(params) < 2 {
		return // Notices should not generate errors
	}

	target := params[0]
	message := params[1]

	if target[0] == '#' || target[0] == '&' {
		// Channel notice
		c.server.RLock()
		channel, exists := c.server.channels[target]
		c.server.RUnlock()

		if !exists {
			return // Notices should not generate errors
		}

		msg := fmt.Sprintf(":%s!%s@%s NOTICE %s :%s",
			c.nickname, c.username, c.hostname, target, message)

		channel.RLock()
		for _, client := range channel.clients {
			if client != c {
				client.sendRaw(msg)
			}
		}
		channel.RUnlock()
	} else {
		// Private notice
		c.server.RLock()
		targetClient, exists := c.server.clients[target]
		c.server.RUnlock()

		if !exists {
			return // Notices should not generate errors
		}

		msg := fmt.Sprintf(":%s!%s@%s NOTICE %s :%s",
			c.nickname, c.username, c.hostname, target, message)
		targetClient.sendRaw(msg)
	}
}

// handleTopic handles a TOPIC command
func (c *Client) handleTopic(params []string) {
	if !c.registered {
		c.sendNumeric(ERR_NOTREGISTERED, ":You have not registered")
		return
	}

	if len(params) < 1 {
		c.sendNumeric(ERR_NEEDMOREPARAMS, "TOPIC :Not enough parameters")
		return
	}

	channelName := params[0]

	c.server.RLock()
	channel, exists := c.server.channels[channelName]
	c.server.RUnlock()

	if !exists {
		c.sendNumeric(403, fmt.Sprintf("%s :No such channel", channelName))
		return
	}

	channel.RLock()
	_, isMember := channel.clients[c.nickname]
	channel.RUnlock()

	if !isMember {
		c.sendNumeric(442, fmt.Sprintf("%s :You're not on that channel", channelName))
		return
	}

	// Get topic
	if len(params) == 1 {
		channel.RLock()
		if channel.topic != "" {
			c.sendNumeric(RPL_TOPIC, fmt.Sprintf("%s :%s", channelName, channel.topic))
		} else {
			c.sendNumeric(RPL_NOTOPIC, fmt.Sprintf("%s :No topic is set", channelName))
		}
		channel.RUnlock()
		return
	}

	// Set topic
	channel.RLock()
	isOperator := channel.operators[c.nickname]
	channel.RUnlock()

	if !isOperator && !c.Modes.Operator {
		c.sendNumeric(ERR_CHANOPRIVSNEEDED, fmt.Sprintf("%s :You're not a channel operator", channelName))
		return
	}

	newTopic := params[1]

	channel.Lock()
	channel.topic = newTopic
	channel.Unlock()

	// Announce the topic change
	topicMsg := fmt.Sprintf(":%s!%s@%s TOPIC %s :%s",
		c.nickname, c.username, c.hostname, channelName, newTopic)

	channel.RLock()
	for _, client := range channel.clients {
		client.sendRaw(topicMsg)
	}
	channel.RUnlock()
}

// handleList handles a LIST command
func (c *Client) handleList(_ []string) {
	if !c.registered {
		c.sendNumeric(ERR_NOTREGISTERED, ":You have not registered")
		return
	}

	c.sendNumeric(RPL_LISTSTART, "Channel :Users  Name")

	c.server.RLock()
	for name, channel := range c.server.channels {
		channel.RLock()
		c.sendNumeric(RPL_LIST, fmt.Sprintf("%s %d :%s",
			name, len(channel.clients), channel.topic))
		channel.RUnlock()
	}
	c.server.RUnlock()

	c.sendNumeric(RPL_LISTEND, ":End of LIST")
}

// handleNames handles a NAMES command
func (c *Client) handleNames(params []string) {
	if !c.registered {
		c.sendNumeric(ERR_NOTREGISTERED, ":You have not registered")
		return
	}

	if len(params) < 1 {
		// List all channels
		c.server.RLock()
		for channelName := range c.server.channels {
			c.sendNames(channelName)
		}
		c.server.RUnlock()

		c.sendNumeric(RPL_ENDOFNAMES, "* :End of NAMES list")
		return
	}

	channelNames := strings.Split(params[0], ",")
	for _, channelName := range channelNames {
		c.sendNames(channelName)
		c.sendNumeric(RPL_ENDOFNAMES, fmt.Sprintf("%s :End of NAMES list", channelName))
	}
}

// sendNames sends the NAMES list for a channel
func (c *Client) sendNames(channelName string) {
	c.server.RLock()
	channel, exists := c.server.channels[channelName]
	c.server.RUnlock()

	if !exists {
		return
	}

	channel.RLock()
	var namesList strings.Builder
	for nick := range channel.clients {
		if namesList.Len() > 0 {
			namesList.WriteString(" ")
		}

		if channel.operators[nick] {
			namesList.WriteString("@")
		}

		namesList.WriteString(nick)
	}
	channel.RUnlock()

	c.sendNumeric(RPL_NAMREPLY, fmt.Sprintf("= %s :%s", channelName, namesList.String()))
}

// handleWho handles a WHO command
func (c *Client) handleWho(params []string) {
	if !c.registered {
		c.sendNumeric(ERR_NOTREGISTERED, ":You have not registered")
		return
	}

	mask := "*"
	if len(params) > 0 {
		mask = params[0]
	}

	if mask[0] == '#' || mask[0] == '&' {
		// Channel WHO
		c.server.RLock()
		channel, exists := c.server.channels[mask]
		c.server.RUnlock()

		if exists {
			channel.RLock()
			for nick, client := range channel.clients {
				flags := "H" // Here
				if channel.operators[nick] {
					flags += "@"
				}
				if client.Modes.Operator {
					flags += "*"
				}

				c.sendNumeric(RPL_WHOREPLY, fmt.Sprintf("%s %s %s %s %s %s :0 %s",
					mask, client.username, client.hostname, c.server.Config.ServerName,
					nick, flags, client.realname))
			}
			channel.RUnlock()
		}
	} else {
		// User WHO with mask
		c.server.RLock()
		for nick, client := range c.server.clients {
			if wildcardMatch(nick, mask) || wildcardMatch(client.realname, mask) {
				flags := "H" // Here
				if client.Modes.Operator {
					flags += "*"
				}

				// Find a common channel
				var commonChannel string
				for chName := range client.channels {
					commonChannel = chName
					break
				}

				c.sendNumeric(RPL_WHOREPLY, fmt.Sprintf("%s %s %s %s %s %s :0 %s",
					commonChannel, client.username, client.hostname, c.server.Config.ServerName,
					nick, flags, client.realname))
			}
		}
		c.server.RUnlock()
	}

	c.sendNumeric(RPL_ENDOFWHO, fmt.Sprintf("%s :End of WHO list", mask))
}

// handleWhois handles a WHOIS command
func (c *Client) handleWhois(params []string) {
	if !c.registered {
		c.sendNumeric(451, ":You have not registered")
		return
	}

	if len(params) < 1 {
		c.sendNumeric(461, "WHOIS :Not enough parameters")
		return
	}

	target := params[0]

	c.server.RLock()
	targetClient, exists := c.server.clients[target]
	c.server.RUnlock()

	if !exists {
		c.sendNumeric(401, fmt.Sprintf("%s :No such nick/channel", target))
		c.sendNumeric(318, fmt.Sprintf("%s :End of WHOIS list", target))
		return
	}

	targetClient.RLock()
	isAway := targetClient.Modes.Away
	awayMessage := targetClient.awayMessage
	modeString := targetClient.Modes.String()
	targetClient.RUnlock()

	if isAway && awayMessage != "" {
		c.sendNumeric(301, fmt.Sprintf("%s :%s", targetClient.nickname, awayMessage))
	}

	// Show user modes in WHOIS response
	if modeString != "" {
		c.sendNumeric(379, fmt.Sprintf("%s :is using modes %s", targetClient.nickname, modeString))
	}

	c.sendNumeric(311, fmt.Sprintf("%s %s %s * :%s",
		targetClient.nickname, targetClient.username, targetClient.hostname, targetClient.realname))

	// List channels the user is in
	targetClient.RLock()
	if len(targetClient.channels) > 0 {
		var channels strings.Builder
		for channelName := range targetClient.channels {
			if channels.Len() > 0 {
				channels.WriteString(" ")
			}

			// Check if the user is an operator in this channel
			c.server.RLock()
			channel, channelExists := c.server.channels[channelName]
			c.server.RUnlock()

			if channelExists {
				channel.RLock()
				if channel.operators[targetClient.nickname] {
					channels.WriteString("@")
				} else if channel.voices[targetClient.nickname] {
					channels.WriteString("+")
				}
				channel.RUnlock()
			}

			channels.WriteString(channelName)
		}
		c.sendNumeric(319, fmt.Sprintf("%s :%s", targetClient.nickname, channels.String()))
	}

	c.sendNumeric(312, fmt.Sprintf("%s %s :%s",
		targetClient.nickname, c.server.Config.ServerName, c.server.Config.ServerDesc))

	if targetClient.Modes.Operator {
		c.sendNumeric(313, fmt.Sprintf("%s :is an IRC operator", targetClient.nickname))
	}

	c.sendNumeric(317, fmt.Sprintf("%s %d %d :seconds idle, signon time",
		targetClient.nickname, int(time.Since(targetClient.lastPong).Seconds()),
		targetClient.lastPong.Unix()))

	c.sendNumeric(318, fmt.Sprintf("%s :End of WHOIS list", targetClient.nickname))
}

// handleOper handles an OPER command with both OIDC and traditional authentication
func (c *Client) handleOper(params []string) {
	if !c.registered {
		c.sendNumeric(451, ":You have not registered")
		return
	}

	if len(params) < 1 {
		c.sendNumeric(461, "OPER :Not enough parameters")
		return
	}

	// If two parameters are provided, attempt traditional authentication
	if len(params) >= 2 {
		username := params[0]
		password := params[1]

		c.server.RLock()
		correctPassword, exists := c.server.opCredentials[username]
		c.server.RUnlock()

		if exists && password == correctPassword {
			c.Modes.Operator = true
			c.sendNumeric(381, ":You are now an IRC operator")

			// Send new user mode
			c.sendMessage("MODE", c.nickname, "+o")

			log.Printf("User %s authenticated as operator with traditional credentials", c.nickname)
			return
		}

		c.sendNumeric(464, ":Password incorrect")
		return
	}

	// // Single parameter - assume OIDC token
	// token := params[0]

	// // Verify the ID token
	// idToken, err := c.server.oidcVerifier.Verify(context.Background(), token)
	// if err != nil {
	// 	c.sendNumeric(464, ":Authentication failed")
	// 	log.Printf("OIDC token verification failed: %v", err)
	// 	return
	// }

	// Extract claims from the token
	var claims struct {
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
	}
	// if err := idToken.Claims(&claims); err != nil {
	// 	c.sendNumeric(464, ":Authentication failed")
	// 	log.Printf("Failed to parse OIDC claims: %v", err)
	// 	return
	// }

	// Check if the email is in the operators list
	c.server.RLock()
	isOperator := c.server.operators[claims.Email]
	c.server.RUnlock()

	if !isOperator || !claims.EmailVerified {
		c.sendNumeric(464, ":Not authorized as an operator")
		return
	}

	c.Modes.Operator = true
	c.email = claims.Email // Store the authenticated email
	c.sendNumeric(381, ":You are now an IRC operator")

	// Send new user mode
	c.sendMessage("MODE", c.nickname, "+o")

	log.Printf("User %s authenticated as operator with email %s", c.nickname, claims.Email)
}

// handleKill handles a KILL command (operator only)
func (c *Client) handleKill(params []string) {
	// Verify client is an operator
	if !c.Modes.Operator {
		c.sendNumeric(481, ":Permission Denied - You're not an IRC operator")
		return
	}

	// Check for sufficient parameters
	if len(params) < 1 {
		c.sendNumeric(461, "KILL :Not enough parameters")
		return
	}

	// Get target nickname and optional reason
	targetNick := params[0]
	reason := "No reason"
	if len(params) > 1 {
		reason = params[1]
	}

	// Find target client
	c.server.RLock()
	targetClient, exists := c.server.clients[targetNick]
	c.server.RUnlock()

	if !exists {
		c.sendNumeric(401, fmt.Sprintf("%s :No such nick/channel", targetNick))
		return
	}

	// Don't allow killing other operators (protection mechanism)
	targetClient.RLock()
	isTargetOperator := targetClient.Modes.Operator
	targetClient.RUnlock()

	if isTargetOperator {
		c.sendNumeric(483, ":You can't KILL an operator")
		return
	}

	// Notify the killed client
	killMessage := fmt.Sprintf(":Killed by %s: %s", c.nickname, reason)
	targetClient.sendNumeric(999, killMessage)

	// Force the client to quit
	targetClient.quit(fmt.Sprintf("Killed by %s: %s", c.nickname, reason))

	// Log the kill action and send notices to operators
	c.server.SendGlobopsNotice(fmt.Sprintf("Client %s disconnected by operator %s: %s",
		targetNick, c.nickname, reason))
	c.server.SendLocopsNotice(fmt.Sprintf("Client %s disconnected by operator %s: %s",
		targetNick, c.nickname, reason))
}

// quit handles client disconnection
func (c *Client) quit(reason string) {
	if c.quitting {
		return
	}

	c.quitting = true

	// If the client is registered, send a quit message to all relevant clients
	if c.registered {
		quitMsg := fmt.Sprintf(":%s!%s@%s QUIT :%s",
			c.nickname, c.username, c.hostname, reason)

		// List of clients to notify (all clients in shared channels)
		notifySet := make(map[*Client]bool)

		// Add clients from shared channels
		c.RLock()
		for channelName := range c.channels {
			c.server.RLock()
			channel, exists := c.server.channels[channelName]
			c.server.RUnlock()

			if exists {
				channel.RLock()
				for _, client := range channel.clients {
					if client != c {
						notifySet[client] = true
					}
				}
				channel.RUnlock()
			}
		}
		c.RUnlock()

		// Notify clients
		for client := range notifySet {
			client.sendRaw(quitMsg)
		}

		// Remove from channels
		c.RLock()
		for channelName := range c.channels {
			c.server.RLock()
			channel, exists := c.server.channels[channelName]
			c.server.RUnlock()

			if exists {
				channel.Lock()
				delete(channel.clients, c.nickname)
				delete(channel.operators, c.nickname)

				// If the channel is empty, remove it
				if len(channel.clients) == 0 {
					c.server.Lock()
					delete(c.server.channels, channelName)
					c.server.Unlock()
				}
				channel.Unlock()
			}
		}
		c.RUnlock()

		// Remove from server's clients list
		c.server.Lock()
		delete(c.server.clients, c.nickname)
		c.server.Unlock()
	}

	// Close the connection
	c.conn.Close()
}

// sendRaw sends a raw message to the client
func (c *Client) sendRaw(message string) {
	if c.server.Config.Debug {
		log.Printf("[%s] => %s", c.nickname, message)
	}

	c.writeLock.Lock()
	defer c.writeLock.Unlock()

	_, err := c.writer.WriteString(message + "\r\n")
	if err == nil {
		c.writer.Flush()
	}
}

// sendMessage sends an IRC message to the client
func (c *Client) sendMessage(command string, params ...string) {
	var sb strings.Builder

	sb.WriteString(":")
	sb.WriteString(c.server.Config.ServerName)
	sb.WriteString(" ")
	sb.WriteString(command)

	for i, param := range params {
		sb.WriteString(" ")

		// Last parameter gets a colon if it contains spaces
		if i == len(params)-1 && (strings.Contains(param, " ") || param == "") {
			sb.WriteString(":")
		}

		sb.WriteString(param)
	}

	c.sendRaw(sb.String())
}

// sendNumeric sends a numeric reply to the client
func (c *Client) sendNumeric(numeric int, message string) {
	var sb strings.Builder

	sb.WriteString(":")
	sb.WriteString(c.server.Config.ServerName)
	sb.WriteString(" ")
	sb.WriteString(fmt.Sprintf("%03d", numeric))
	sb.WriteString(" ")

	// Add the nickname as the first parameter if registered
	if c.registered {
		sb.WriteString(c.nickname)
		sb.WriteString(" ")
	} else if c.nickname != "" {
		sb.WriteString(c.nickname)
		sb.WriteString(" ")
	} else {
		sb.WriteString("* ")
	}

	// Add the message
	sb.WriteString(message)

	c.sendRaw(sb.String())
}

// Helper functions

// isValidNickname checks if a nickname is valid
func isValidNickname(nick string) bool {
	if len(nick) < 1 || len(nick) > 30 {
		return false
	}

	for i, ch := range nick {
		// First character can't be a number
		if i == 0 && ch >= '0' && ch <= '9' {
			return false
		}

		// Valid characters: A-Z, a-z, 0-9, and special chars like -_[]{}|\
		if !((ch >= 'A' && ch <= 'Z') ||
			(ch >= 'a' && ch <= 'z') ||
			(ch >= '0' && ch <= '9') ||
			strings.ContainsRune("-_[]{}|\\", ch)) {
			return false
		}
	}

	return true
}

// isValidChannelName checks if a channel name is valid
func isValidChannelName(name string) bool {
	if len(name) < 2 {
		return false
	}

	// Must start with # or &
	if name[0] != '#' && name[0] != '&' {
		return false
	}

	// Can't contain spaces, ASCII 7 (bell), commas, colons, or NULL bytes
	if strings.ContainsAny(name, " ,:\x00\x07") {
		return false
	}

	return true
}

// wildcardMatch performs simple wildcard matching
func wildcardMatch(s, pattern string) bool {
	if pattern == "*" {
		return true
	}

	if !strings.Contains(pattern, "*") {
		return s == pattern
	}

	// Split by *
	parts := strings.Split(pattern, "*")

	// Check if start matches
	if parts[0] != "" && !strings.HasPrefix(s, parts[0]) {
		return false
	}

	// Check if end matches
	if parts[len(parts)-1] != "" && !strings.HasSuffix(s, parts[len(parts)-1]) {
		return false
	}

	// Check middle parts
	pos := 0
	for _, part := range parts {
		if part == "" {
			continue
		}

		newPos := strings.Index(s[pos:], part)
		if newPos == -1 {
			return false
		}

		pos += newPos + len(part)
	}

	return true
}
