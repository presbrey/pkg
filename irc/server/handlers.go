package server

import (
	"fmt"
	"strings"
	"time"

	"github.com/presbrey/pkg/irc"
)

// handleNick handles the NICK command
func handleNick(params *HookParams) error {
	client := params.Client
	message := params.Message

	// Check if the client provided a nickname
	if len(message.Params) < 1 {
		client.SendError(irc.ERR_NONICKNAMEGIVEN, "No nickname given")
		return nil
	}

	newNick := message.Params[0]

	// Check if the nickname is already in use
	existingClient := client.Server.GetClient(newNick)
	if existingClient != nil && existingClient.ID != client.ID {
		client.SendError(irc.ERR_NICKNAMEINUSE, newNick, "Nickname is already in use")
		return nil
	}

	// Acquire the write lock before modifying client fields
	client.mu.Lock()

	// Store the old nickname for notifications
	oldNick := client.Nickname
	wasRegistered := client.Registered

	// Update the client's nickname
	client.Nickname = newNick

	// Release the lock
	client.mu.Unlock()

	// If the client wasn't registered before, check if they are now
	if !wasRegistered && client.Username != "" {
		client.mu.Lock()
		client.Registered = true
		client.mu.Unlock()
		client.SendWelcome()
	} else if wasRegistered {
		// Notify all channels the client is in about the nick change
		for _, channel := range client.Channels {
			channel.SendToAll(fmt.Sprintf(":%s!%s@%s NICK %s", oldNick, client.Username, client.Hostname, newNick), nil)
		}
	}

	return nil
}

// handleUser handles the USER command
func handleUser(params *HookParams) error {
	client := params.Client
	message := params.Message

	// Check if the client provided enough parameters
	if len(message.Params) < 4 {
		client.SendError(irc.ERR_NEEDMOREPARAMS, "USER", "Not enough parameters")
		return nil
	}

	// Check if the client is already registered
	if client.Registered {
		client.SendError(irc.ERR_ALREADYREGISTRED, "You may not reregister")
		return nil
	}

	// Update the client's user information
	client.Username = message.Params[0]
	client.Realname = message.Params[3]

	// Check if the client is now registered
	if client.Nickname != "" {
		// Check if server password is required but not provided
		serverPassword := client.Server.GetConfig().ListenIRC.Password
		if serverPassword != "" {
			client.mu.RLock()
			passwordProvided := client.PasswordProvided
			client.mu.RUnlock()

			if !passwordProvided {
				client.SendError(irc.ERR_PASSWDMISMATCH, "Password required")
				return nil
			}
		}

		client.mu.Lock()
		client.Registered = true
		client.mu.Unlock()
		client.SendWelcome()
	}

	return nil
}

// handleJoin handles the JOIN command
func handleJoin(params *HookParams) error {
	client := params.Client
	message := params.Message

	// Check if the client provided a channel
	if len(message.Params) < 1 {
		client.SendError(irc.ERR_NEEDMOREPARAMS, "JOIN", "Not enough parameters")
		return nil
	}

	// Split the channel list
	channels := strings.Split(message.Params[0], ",")
	keys := []string{}

	// If keys are provided, split them as well
	if len(message.Params) > 1 {
		keys = strings.Split(message.Params[1], ",")
	}

	// Join each channel
	for i, channelName := range channels {
		// Validate channel name
		if !strings.HasPrefix(channelName, "#") {
			client.SendError(irc.ERR_NOSUCHCHANNEL, channelName, "No such channel")
			continue
		}

		// Get the channel key, if any
		var key string
		if i < len(keys) {
			key = keys[i]
		}

		// Get or create the channel
		channel := client.Server.GetChannel(channelName)
		if channel == nil {
			channel = client.Server.CreateChannel(channelName)
			// First user to join a new channel becomes an operator and owner
			channel.mu.Lock()
			channel.Operators[client.Nickname] = true
			channel.Owners[client.Nickname] = true
			channel.mu.Unlock()
		}

		// Check if the channel has a key
		if channel.Modes.Key != "" && channel.Modes.Key != key {
			client.SendError(irc.ERR_BADCHANNELKEY, channelName, "Cannot join channel (+k) - bad key")
			continue
		}

		// Check if the channel is invite-only
		if channel.Modes.InviteOnly && !channel.IsInvited(client) {
			client.SendError(irc.ERR_INVITEONLYCHAN, channelName, "Cannot join channel (+i) - you must be invited")
			continue
		}

		// Check if the user is banned
		if channel.IsBanned(client) {
			client.SendError(irc.ERR_BANNEDFROMCHAN, channelName, "Cannot join channel (+b) - you are banned")
			continue
		}

		// Check if the channel is full
		if channel.Modes.UserLimit > 0 && channel.MemberCount() >= channel.Modes.UserLimit {
			client.SendError(irc.ERR_CHANNELISFULL, channelName, "Cannot join channel (+l) - channel is full")
			continue
		}

		// Join the channel
		client.JoinChannel(channelName)
	}

	return nil
}

// handlePart handles the PART command
func handlePart(params *HookParams) error {
	client := params.Client
	message := params.Message

	// Check if the client provided a channel
	if len(message.Params) < 1 {
		client.SendError(irc.ERR_NEEDMOREPARAMS, "PART", "Not enough parameters")
		return nil
	}

	// Split the channel list
	channels := strings.Split(message.Params[0], ",")

	// Get the part message
	reason := "Leaving"
	if len(message.Params) > 1 {
		reason = message.Params[1]
	}

	// Part each channel
	for _, channelName := range channels {
		// Get the channel
		channel := client.Server.GetChannel(channelName)
		if channel == nil {
			client.SendError(irc.ERR_NOSUCHCHANNEL, channelName, "No such channel")
			continue
		}

		// Check if the client is on the channel
		if !channel.IsMember(client) {
			client.SendError(irc.ERR_NOTONCHANNEL, channelName, "You're not on that channel")
			continue
		}

		// Part the channel
		client.PartChannel(channelName, reason)
	}

	return nil
}

// handlePass handles the PASS command
func handlePass(params *HookParams) error {
	client := params.Client
	message := params.Message

	// Check if the client provided a password
	if len(message.Params) < 1 {
		client.SendError(irc.ERR_NEEDMOREPARAMS, "PASS", "Not enough parameters")
		return nil
	}

	password := message.Params[0]
	serverPassword := client.Server.GetConfig().ListenIRC.Password

	// If the server requires a password, validate it
	if serverPassword != "" && password != serverPassword {
		client.SendError(irc.ERR_PASSWDMISMATCH, "Password incorrect")

		// Note: In a real IRC server, we might disconnect the client here after an incorrect password
		// but we'll just mark the password as not provided
		client.mu.Lock()
		client.PasswordProvided = false
		client.mu.Unlock()
		return nil
	}

	// Mark the password as provided
	client.mu.Lock()
	client.PasswordProvided = true
	client.mu.Unlock()

	return nil
}

// handlePrivmsg handles the PRIVMSG command
func handlePrivmsg(params *HookParams) error {
	client := params.Client
	message := params.Message

	// Check if the client provided a target and a message
	if len(message.Params) < 2 {
		client.SendError(irc.ERR_NEEDMOREPARAMS, "PRIVMSG", "Not enough parameters")
		return nil
	}

	target := message.Params[0]
	text := message.Params[1]

	// Check if the target is a channel
	if strings.HasPrefix(target, "#") {
		// Get the channel
		channel := client.Server.GetChannel(target)
		if channel == nil {
			client.SendError(irc.ERR_NOSUCHNICK, target, "No such nick/channel")
			return nil
		}

		// Check if the client can send messages to the channel based on their permissions
		if !channel.CanSendToChannel(client) {
			if !channel.IsMember(client) && channel.Modes.NoExternalMsgs {
				client.SendError(irc.ERR_CANNOTSENDTOCHAN, target, "Cannot send to channel")
			} else if channel.Modes.Moderated {
				client.SendError(irc.ERR_CANNOTSENDTOCHAN, target, "Cannot send to channel (+m)")
			} else {
				client.SendError(404, target, "Cannot send to channel")
			}
			return nil
		}

		// Send the message to the channel
		channel.SendToAll(fmt.Sprintf(":%s!%s@%s PRIVMSG %s :%s", client.Nickname, client.Username, client.Hostname, target, text), client)
	} else {
		// Get the target client
		targetClient := client.Server.GetClient(target)
		if targetClient == nil {
			client.SendError(irc.ERR_NOSUCHNICK, target, "No such nick/channel")
			return nil
		}

		// Send the message to the target client
		targetClient.SendPrivmsg(client, text)
	}

	return nil
}

// handleQuit handles the QUIT command
func handleQuit(params *HookParams) error {
	client := params.Client
	message := params.Message

	// Get the quit message
	reason := "Client Quit"
	if len(message.Params) > 0 {
		reason = message.Params[0]
	}

	// Quit the client
	client.Quit(reason)

	return nil
}

// handleMode handles the MODE command
func handleMode(params *HookParams) error {
	client := params.Client
	message := params.Message

	// Check if the client provided a target
	if len(message.Params) < 1 {
		client.SendError(irc.ERR_NEEDMOREPARAMS, "MODE", "Not enough parameters")
		return nil
	}

	target := message.Params[0]

	// Check if the target is a channel
	if strings.HasPrefix(target, "#") {
		handleChannelMode(params)
	} else {
		handleUserMode(params)
	}

	return nil
}

// handleChannelMode handles channel MODE commands
func handleChannelMode(params *HookParams) error {
	client := params.Client
	message := params.Message
	channelName := message.Params[0]

	// Get the channel
	channel := client.Server.GetChannel(channelName)
	if channel == nil {
		client.SendError(irc.ERR_NOSUCHCHANNEL, channelName, "No such channel")
		return nil
	}

	// If no modes are specified, show the current modes
	if len(message.Params) < 2 {
		client.SendReply(irc.RPL_CHANNELMODEIS, channelName, channel.GetModeString())
		return nil
	}

	// Check if the client has permission to change channel modes
	if !channel.CanChangeChannelModes(client) {
		client.SendError(irc.ERR_CHANOPRIVSNEEDED, channelName, "You're not a channel operator")
		return nil
	}

	// Parse the mode string
	modeStr := message.Params[1]
	modeSet := true
	paramIndex := 2

	for _, mode := range modeStr {
		if mode == '+' {
			modeSet = true
			continue
		}
		if mode == '-' {
			modeSet = false
			continue
		}

		// Process the mode
		switch mode {
		case 'b': // Ban list
			if len(message.Params) <= paramIndex {
				// Show the ban list
				for _, ban := range channel.BanList {
					client.SendReply(irc.RPL_BANLIST, channelName, ban, "", "0")
				}
				client.SendReply(irc.RPL_ENDOFBANLIST, channelName, "End of channel ban list")
				continue
			}
			mask := message.Params[paramIndex]
			paramIndex++
			if modeSet {
				channel.AddBan(mask, client.Nickname)
			} else {
				channel.RemoveBan(mask)
			}
			channel.SendToAll(fmt.Sprintf(":%s!%s@%s MODE %s %c%c %s", client.Nickname, client.Username, client.Hostname, channelName, func() byte {
				if modeSet {
					return '+'
				} else {
					return '-'
				}
			}(), mode, mask), nil)
		case 'k': // Channel key
			if modeSet {
				if len(message.Params) <= paramIndex {
					client.SendError(irc.ERR_NEEDMOREPARAMS, "MODE", "Not enough parameters")
					continue
				}
				key := message.Params[paramIndex]
				paramIndex++
				channel.SetMode('k', true, key)
				channel.SendToAll(fmt.Sprintf(":%s!%s@%s MODE %s +k %s", client.Nickname, client.Username, client.Hostname, channelName, key), nil)
			} else {
				channel.SetMode('k', false, "")
				channel.SendToAll(fmt.Sprintf(":%s!%s@%s MODE %s -k", client.Nickname, client.Username, client.Hostname, channelName), nil)
			}
		case 'l': // User limit
			if modeSet {
				if len(message.Params) <= paramIndex {
					client.SendError(irc.ERR_NEEDMOREPARAMS, "MODE", "Not enough parameters")
					continue
				}
				limit := message.Params[paramIndex]
				paramIndex++
				channel.SetMode('l', true, limit)
				channel.SendToAll(fmt.Sprintf(":%s!%s@%s MODE %s +l %s", client.Nickname, client.Username, client.Hostname, channelName, limit), nil)
			} else {
				channel.SetMode('l', false, "")
				channel.SendToAll(fmt.Sprintf(":%s!%s@%s MODE %s -l", client.Nickname, client.Username, client.Hostname, channelName), nil)
			}
		default:
			// Handle other modes
			channel.SetMode(mode, modeSet, "")
			channel.SendToAll(fmt.Sprintf(":%s!%s@%s MODE %s %c%c", client.Nickname, client.Username, client.Hostname, channelName, func() byte {
				if modeSet {
					return '+'
				} else {
					return '-'
				}
			}(), mode), nil)
		}
	}

	return nil
}

// handleUserMode handles user MODE commands
func handleUserMode(params *HookParams) error {
	client := params.Client
	message := params.Message
	target := message.Params[0]

	// Check if the target is the client
	if target != client.Nickname {
		client.SendError(irc.ERR_USERSDONTMATCH, "Can't change mode for other users")
		return nil
	}

	// If no modes are specified, show the current modes
	if len(message.Params) < 2 {
		client.SendReply(irc.RPL_UMODEIS, client.Modes.GetModeString())
		return nil
	}

	// Parse the mode string
	modeStr := message.Params[1]
	modeSet := true

	for _, mode := range modeStr {
		if mode == '+' {
			modeSet = true
			continue
		}
		if mode == '-' {
			modeSet = false
			continue
		}

		// Process the mode
		switch mode {
		case 'o', 'O': // Operator status
			// Only the server can set these modes
			if modeSet {
				continue
			}
			client.SetMode(string(mode), false)
		default:
			// Handle other modes
			client.SetMode(string(mode), modeSet)
		}
	}

	return nil
}

// handlePing handles the PING command
func handlePing(params *HookParams) error {
	client := params.Client
	message := params.Message

	// Check if the client provided a server
	if len(message.Params) < 1 {
		client.SendError(irc.ERR_NEEDMOREPARAMS, "PING", "Not enough parameters")
		return nil
	}

	// Send a PONG reply
	client.SendMessage(client.Server.GetConfig().Server.Name, "PONG", client.Server.GetConfig().Server.Name, message.Params[0])

	return nil
}

// handlePong handles the PONG command
func handlePong(params *HookParams) error {
	// Just update the client's last ping time
	params.Client.LastPing = time.Now()
	return nil
}

// handleWho handles the WHO command
func handleWho(params *HookParams) error {
	client := params.Client
	message := params.Message

	// Check if the client provided a mask
	if len(message.Params) < 1 {
		client.SendError(irc.ERR_NEEDMOREPARAMS, "WHO", "Not enough parameters")
		return nil
	}

	mask := message.Params[0]

	// Check if the mask is a channel
	if strings.HasPrefix(mask, "#") {
		channel := client.Server.GetChannel(mask)
		if channel != nil {
			for _, member := range channel.Members {
				flags := ""
				if member.IsOper {
					flags += "*"
				}
				client.SendReply(irc.RPL_WHOREPLY, mask, member.Username, member.Hostname, client.Server.GetConfig().Server.Name, member.Nickname, flags, fmt.Sprintf("0 %s", member.Realname))
			}
		}
	} else {
		// Check if the mask is a nickname
		target := client.Server.GetClient(mask)
		if target != nil {
			flags := ""
			if target.IsOper {
				flags += "*"
			}
			client.SendReply(irc.RPL_WHOREPLY, "*", target.Username, target.Hostname, client.Server.GetConfig().Server.Name, target.Nickname, flags, fmt.Sprintf("0 %s", target.Realname))
		}
	}

	client.SendReply(irc.RPL_ENDOFWHO, mask, "End of WHO list")

	return nil
}

// handleWhois handles the WHOIS command
func handleWhois(params *HookParams) error {
	client := params.Client
	message := params.Message

	// Check if the client provided a nickname
	if len(message.Params) < 1 {
		client.SendError(irc.ERR_NEEDMOREPARAMS, "WHOIS", "Not enough parameters")
		return nil
	}

	target := message.Params[0]
	targetClient := client.Server.GetClient(target)

	if targetClient == nil {
		client.SendError(irc.ERR_NOSUCHNICK, target, "No such nick/channel")
		return nil
	}

	serverName := client.Server.GetConfig().Server.Name
	networkName := client.Server.GetConfig().Server.Network

	// Send WHOIS information
	client.SendReply(irc.RPL_WHOISUSER, targetClient.Nickname, targetClient.Username, targetClient.Hostname, "*", targetClient.Realname)
	client.SendReply(irc.RPL_WHOISSERVER, targetClient.Nickname, serverName, fmt.Sprintf("%s Server", networkName))

	// Send channel list
	var channels string
	for channelName := range targetClient.Channels {
		channels += channelName + " "
	}
	if channels != "" {
		client.SendReply(irc.RPL_WHOISCHANNELS, targetClient.Nickname, channels)
	}

	// Send operator status
	if targetClient.IsOper {
		client.SendReply(irc.RPL_WHOISOPERATOR, targetClient.Nickname, "is an IRC Operator")
	}

	// Send idle time
	client.SendReply(irc.RPL_WHOISIDLE, targetClient.Nickname, fmt.Sprintf("%d", int(time.Since(targetClient.LastPing).Seconds())), "seconds idle")

	// End of WHOIS
	client.SendReply(irc.RPL_ENDOFWHOIS, targetClient.Nickname, "End of WHOIS list")

	return nil
}

// handleList handles the LIST command
func handleList(params *HookParams) error {
	client := params.Client
	message := params.Message

	// Start the list
	client.SendReply(irc.RPL_LISTSTART, "Channel", "Users Name")

	// If a specific channel is requested
	if len(message.Params) > 0 {
		channels := strings.Split(message.Params[0], ",")
		for _, channelName := range channels {
			channel := client.Server.GetChannel(channelName)
			if channel != nil {
				client.SendReply(irc.RPL_LIST, channelName, fmt.Sprintf("%d", channel.MemberCount()), channel.Topic)
			}
		}
	} else {
		// List all channels
		client.Server.channels.Range(func(key, value interface{}) bool {
			channelName := key.(string)
			channel := value.(*Channel)
			client.SendReply(irc.RPL_LIST, channelName, fmt.Sprintf("%d", channel.MemberCount()), channel.Topic)
			return true // Continue iteration
		})
	}

	// End the list
	client.SendReply(irc.RPL_LISTEND, "End of LIST")

	return nil
}

// handleNames handles the NAMES command
func handleNames(params *HookParams) error {
	client := params.Client
	message := params.Message

	// If a specific channel is requested
	if len(message.Params) > 0 {
		channels := strings.Split(message.Params[0], ",")
		for _, channelName := range channels {
			channel := client.Server.GetChannel(channelName)
			if channel != nil {
				channel.SendNames(client)
			}
		}
	} else {
		// List all channels
		client.Server.channels.Range(func(_, value interface{}) bool {
			channel := value.(*Channel)
			channel.SendNames(client)
			return true // Continue iteration
		})
	}

	return nil
}

// handleTopic handles the TOPIC command
func handleTopic(params *HookParams) error {
	client := params.Client
	message := params.Message

	// Check if the client provided a channel
	if len(message.Params) < 1 {
		client.SendError(irc.ERR_NEEDMOREPARAMS, "TOPIC", "Not enough parameters")
		return nil
	}

	channelName := message.Params[0]
	channel := client.Server.GetChannel(channelName)

	if channel == nil {
		client.SendError(irc.ERR_NOSUCHCHANNEL, channelName, "No such channel")
		return nil
	}

	// Check if the client is on the channel
	if !channel.IsMember(client) {
		client.SendError(irc.ERR_NOTONCHANNEL, channelName, "You're not on that channel")
		return nil
	}

	// If no topic is provided, show the current topic
	if len(message.Params) < 2 {
		topic, setBy, setAt := channel.GetTopic()
		if topic != "" {
			client.SendReply(irc.RPL_TOPIC, channelName, topic)
			client.SendNumeric(333, channelName, setBy, fmt.Sprintf("%d", setAt.Unix()))
		} else {
			client.SendReply(irc.RPL_NOTOPIC, channelName, "No topic is set")
		}
		return nil
	}

	// Check if the client can set the topic
	if channel.Modes.TopicSettableByOpsOnly && !client.IsOper {
		client.SendError(irc.ERR_CHANOPRIVSNEEDED, channelName, "You're not a channel operator")
		return nil
	}

	// Set the topic
	topic := message.Params[1]
	channel.SetTopic(topic, client.Nickname)

	// Notify all members
	channel.SendToAll(fmt.Sprintf(":%s!%s@%s TOPIC %s :%s", client.Nickname, client.Username, client.Hostname, channelName, topic), nil)

	return nil
}

// handleKick handles the KICK command
func handleKick(params *HookParams) error {
	client := params.Client
	message := params.Message

	// Check if the client provided a channel and a target
	if len(message.Params) < 2 {
		client.SendError(irc.ERR_NEEDMOREPARAMS, "KICK", "Not enough parameters")
		return nil
	}

	channelName := message.Params[0]
	target := message.Params[1]

	reason := "No reason given"
	if len(message.Params) > 2 {
		reason = message.Params[2]
	}

	// Get the channel
	channel := client.Server.GetChannel(channelName)
	if channel == nil {
		client.SendError(irc.ERR_NOSUCHCHANNEL, channelName, "No such channel")
		return nil
	}

	// Check if the client is on the channel
	if !channel.IsMember(client) {
		client.SendError(irc.ERR_NOTONCHANNEL, channelName, "You're not on that channel")
		return nil
	}

	// Check if the client has permission to kick the target
	targetClient := client.Server.GetClient(target)
	if targetClient == nil {
		client.SendError(irc.ERR_NOSUCHNICK, target, "No such nick/channel")
		return nil
	}

	// Check if the target is on the channel
	if !channel.IsMember(targetClient) {
		client.SendError(irc.ERR_USERNOTINCHANNEL, target, channelName, "They aren't on that channel")
		return nil
	}

	// Check if the client has permission to kick the target
	if !channel.CanKickUsers(client, targetClient) {
		if !channel.IsHalfop(client) && !client.IsOper {
			client.SendNumeric(482, channelName, "You're not a channel operator")
		} else {
			client.SendNumeric(482, channelName, "You don't have sufficient privileges to kick this user")
		}
		return nil
	}

	// Kick the target
	channel.Kick(client, targetClient, reason)

	return nil
}

// handleInvite handles the INVITE command
func handleInvite(params *HookParams) error {
	client := params.Client
	message := params.Message

	// Check if the client provided a target and a channel
	if len(message.Params) < 2 {
		client.SendError(irc.ERR_NEEDMOREPARAMS, "INVITE", "Not enough parameters")
		return nil
	}

	target := message.Params[0]
	channelName := message.Params[1]

	// Get the channel
	channel := client.Server.GetChannel(channelName)
	if channel == nil {
		client.SendError(irc.ERR_NOSUCHCHANNEL, channelName, "No such channel")
		return nil
	}

	// Check if the client is on the channel
	if !channel.IsMember(client) {
		client.SendError(irc.ERR_NOTONCHANNEL, channelName, "You're not on that channel")
		return nil
	}

	// Check if the channel is invite-only and the client is not an operator
	if channel.Modes.InviteOnly && !client.IsOper {
		client.SendError(irc.ERR_CHANOPRIVSNEEDED, channelName, "You're not a channel operator")
		return nil
	}

	// Get the target client
	targetClient := client.Server.GetClient(target)
	if targetClient == nil {
		client.SendError(irc.ERR_NOSUCHNICK, target, "No such nick/channel")
		return nil
	}

	// Check if the target is already on the channel
	if channel.IsMember(targetClient) {
		client.SendError(irc.ERR_USERONCHANNEL, target, channelName, "is already on channel")
		return nil
	}

	// Add the target to the invite list
	channel.AddInvite(targetClient.Nickname)

	// Notify the client
	client.SendReply(irc.RPL_INVITING, target, channelName)

	// Notify the target
	targetClient.SendMessage(client.Nickname, "INVITE", targetClient.Nickname, channelName)

	return nil
}

// handleOper handles the OPER command
func handleOper(params *HookParams) error {
	client := params.Client
	message := params.Message

	// Check if the client provided a username and password
	if len(message.Params) < 2 {
		client.SendError(irc.ERR_NEEDMOREPARAMS, "OPER", "Not enough parameters")
		return nil
	}

	username := message.Params[0]
	password := message.Params[1]

	// Get the operator
	operator := client.Server.GetOperator(username)
	if operator == nil || operator.Password != password {
		client.SendError(irc.ERR_PASSWDMISMATCH, "Password incorrect")
		return nil
	}

	// Set the client as an operator
	client.SetOper(true)

	return nil
}

// handleKill handles the KILL command
func handleKill(params *HookParams) error {
	client := params.Client
	message := params.Message

	// Check if the client provided a target and a reason
	if len(message.Params) < 2 {
		client.SendError(irc.ERR_NEEDMOREPARAMS, "KILL", "Not enough parameters")
		return nil
	}

	// Check if the client is an operator
	if !client.IsOper {
		client.SendNumeric(481, "Permission Denied- You're not an IRC operator")
		return nil
	}

	target := message.Params[0]
	reason := message.Params[1]

	// Get the target client
	targetClient := client.Server.GetClient(target)
	if targetClient == nil {
		client.SendError(irc.ERR_NOSUCHNICK, target, "No such nick/channel")
		return nil
	}

	// Kill the target
	// First send the kill message to the target
	killMessage := fmt.Sprintf("Killed by %s: %s", client.Nickname, reason)
	targetClient.SendMessage(client.Server.GetConfig().Server.Name, "KILL", targetClient.Nickname, killMessage)

	// Add a small delay to ensure the message is delivered
	time.Sleep(50 * time.Millisecond)

	// Directly manipulate the connection to ensure it gets closed properly
	if targetClient.Conn != nil {
		// Set read deadline to immediately expire to force any pending reads to fail
		targetClient.Conn.SetReadDeadline(time.Now())

		// Explicitly close the connection
		targetClient.Conn.Close()
	}

	// *After* closing the connection, clean up the client resources
	targetClient.Server.RemoveClient(targetClient)

	// Remove the client from all channels
	for _, channel := range targetClient.Channels {
		channel.RemoveMember(targetClient)

		// Notify members of the channel that the client has quit
		channel.SendToAll(fmt.Sprintf(":%s!%s@%s QUIT :%s", targetClient.Nickname, targetClient.Username, targetClient.Hostname, killMessage), targetClient)
	}

	// We don't call Quit() because we've manually handled its functionality to ensure proper order

	return nil
}

// handleRehash handles the REHASH command
func handleRehash(params *HookParams) error {
	client := params.Client
	message := params.Message

	// Check if the client is an operator
	if !client.IsOper {
		client.SendNumeric(481, "Permission Denied- You're not an IRC operator")
		return nil
	}

	// Get the new config source, if any
	var newSource string
	if len(message.Params) > 0 {
		newSource = message.Params[0]
	}

	// Rehash the server
	err := client.Server.Rehash(newSource)
	if err != nil {
		client.SendReply(irc.RPL_REHASHING, client.Server.GetConfig().Server.Name, fmt.Sprintf("Rehash failed: %v", err))
		return nil
	}

	client.SendReply(irc.RPL_REHASHING, client.Server.GetConfig().Server.Name, "Rehash successful")

	return nil
}
