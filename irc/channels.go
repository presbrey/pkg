package irc

import (
	"fmt"
	"strings"
	"time"
)

// Channel mode flags (based on UnrealIRCd)
const (
	CMODE_PRIVATE    = 'p' // Private channel
	CMODE_SECRET     = 's' // Secret channel
	CMODE_MODERATED  = 'm' // Only voiced users can talk
	CMODE_INVITEONLY = 'i' // Invite-only channel
	CMODE_NOEXTMSG   = 'n' // No external messages
	CMODE_TOPICLIMIT = 't' // Only ops can change topic
	CMODE_BAN        = 'b' // Ban mask
	CMODE_KEY        = 'k' // Channel key/password
	CMODE_LIMIT      = 'l' // User limit
	CMODE_REGONLY    = 'r' // Only registered users can join
	CMODE_NOKICK     = 'Q' // No kicking users
	CMODE_OPERONLY   = 'O' // Operator only channel
)

// Updated handleJoin to support invite-only and other channel modes
func (c *Client) handleJoin(params []string) {
	if !c.registered {
		c.sendNumeric(451, ":You have not registered")
		return
	}

	if len(params) < 1 {
		c.sendNumeric(461, "JOIN :Not enough parameters")
		return
	}

	channelNames := strings.Split(params[0], ",")
	channelKeys := []string{}

	// Check for channel keys
	if len(params) > 1 {
		channelKeys = strings.Split(params[1], ",")
	}

	for i, channelName := range channelNames {
		if !isValidChannelName(channelName) {
			c.sendNumeric(403, fmt.Sprintf("%s :No such channel", channelName))
			continue
		}

		// Get channel key if provided
		var key string
		if i < len(channelKeys) {
			key = channelKeys[i]
		}

		c.server.RLock()
		channel, exists := c.server.channels[channelName]
		c.server.RUnlock()

		if exists {
			// Check for invite-only mode
			channel.RLock()
			inviteOnly := strings.Contains(channel.modes, string(CMODE_INVITEONLY))

			// Check if the user is invited
			var invited bool
			if inviteOnly {
				if invites, ok := c.server.invites[channelName]; ok {
					invited = invites[c.nickname]
				}
			}

			// Check for channel key
			hasKey := strings.Contains(channel.modes, string(CMODE_KEY))
			var correctKey bool
			if hasKey {
				correctKey = channel.modeArgs[CMODE_KEY] == key
			}
			channel.RUnlock()

			// Check bans
			if c.isChannelBanned(channel) {
				c.sendNumeric(474, fmt.Sprintf("%s :Cannot join channel (+b)", channelName))
				continue
			}

			// Enforce invite-only restriction
			if inviteOnly && !invited && !c.Modes.Operator {
				c.sendNumeric(473, fmt.Sprintf("%s :Cannot join channel (+i)", channelName))
				continue
			}

			// Enforce key restriction
			if hasKey && !correctKey && !c.Modes.Operator {
				c.sendNumeric(475, fmt.Sprintf("%s :Cannot join channel (+k)", channelName))
				continue
			}

			// Check user limit
			channel.RLock()
			hasLimit := strings.Contains(channel.modes, string(CMODE_LIMIT))
			var limitReached bool
			if hasLimit {
				limitStr := channel.modeArgs[CMODE_LIMIT]
				var limit int
				fmt.Sscanf(limitStr, "%d", &limit)
				limitReached = len(channel.clients) >= limit
			}
			channel.RUnlock()

			if hasLimit && limitReached && !c.Modes.Operator {
				c.sendNumeric(471, fmt.Sprintf("%s :Cannot join channel (+l)", channelName))
				continue
			}
		} else {
			// Create the channel
			c.server.Lock()
			channel = &Channel{
				name:      channelName,
				clients:   make(map[string]*Client),
				operators: make(map[string]bool),
				voices:    make(map[string]bool),
				halfops:   make(map[string]bool),
				owners:    make(map[string]bool),
				admins:    make(map[string]bool),
				bans:      make(map[string]*BanEntry),
				modeArgs:  make(map[rune]string),
				modes:     "nt", // Default modes: no external messages, ops control topic
			}
			c.server.channels[channelName] = channel
			c.server.Unlock()

			// First user gets operator status and channel owner
			channel.operators[c.nickname] = true
			channel.owners[c.nickname] = true
		}

		channel.Lock()
		channel.clients[c.nickname] = c
		channel.Unlock()

		c.Lock()
		c.channels[channelName] = true
		c.Unlock()

		// Clear invite if present
		if invites, ok := c.server.invites[channelName]; ok {
			delete(invites, c.nickname)
			if len(invites) == 0 {
				delete(c.server.invites, channelName)
			}
		}

		// Announce to all clients in the channel
		joinMsg := fmt.Sprintf(":%s!%s@%s JOIN %s",
			c.nickname, c.username, c.hostname, channelName)

		channel.RLock()
		for _, client := range channel.clients {
			client.sendRaw(joinMsg)
		}
		channel.RUnlock()

		// Send the channel topic
		channel.RLock()
		if channel.topic != "" {
			c.sendNumeric(332, fmt.Sprintf("%s :%s", channelName, channel.topic))
		} else {
			c.sendNumeric(331, fmt.Sprintf("%s :No topic is set", channelName))
		}
		channel.RUnlock()

		// Send the list of users in the channel
		c.sendNames(channelName)

		// Send channel modes
		channel.RLock()
		if channel.modes != "" {
			c.sendNumeric(324, fmt.Sprintf("%s +%s", channelName, channel.modes))
		}
		channel.RUnlock()
	}
}

// Updated handleMode to support new channel modes
func (c *Client) handleMode(params []string) {
	if !c.registered {
		c.sendNumeric(451, ":You have not registered")
		return
	}

	if len(params) < 1 {
		c.sendNumeric(461, "MODE :Not enough parameters")
		return
	}

	target := params[0]

	// Channel mode
	if target[0] == '#' || target[0] == '&' {
		c.handleChanMode(params)
	}
	// User mode
	c.handleUserMode(params)
}

// handleChanMode handles channel mode changes
func (c *Client) handleChanMode(params []string) {
	target := params[0]

	c.server.RLock()
	channel, exists := c.server.channels[target]
	c.server.RUnlock()

	if !exists {
		c.sendNumeric(403, fmt.Sprintf("%s :No such channel", target))
		return
	}

	// If no mode specified, send the current mode
	if len(params) == 1 {
		channel.RLock()
		modeStr := channel.modes

		// Add mode arguments
		modeArgs := []string{}
		for _, mode := range modeStr {
			if arg, ok := channel.modeArgs[rune(mode)]; ok {
				modeArgs = append(modeArgs, arg)
			}
		}

		response := fmt.Sprintf("%s +%s", target, modeStr)
		if len(modeArgs) > 0 {
			response += " " + strings.Join(modeArgs, " ")
		}
		channel.RUnlock()

		c.sendNumeric(324, response)
		return
	}

	// Check if the client is an operator in the channel
	channel.RLock()
	isOperator := channel.operators[c.nickname] || channel.owners[c.nickname] ||
		channel.admins[c.nickname] || c.Modes.Operator
	channel.RUnlock()

	if !isOperator {
		c.sendNumeric(482, fmt.Sprintf("%s :You're not a channel operator", target))
		return
	}

	// Process mode changes
	modeStr := params[1]
	modeArgs := params[2:]
	modeIndex := 0
	adding := true

	// List of modes that require arguments when adding
	modesWithArgsAdd := "bklovL"

	// List of modes that require arguments when removing
	modesWithArgsRemove := "b"

	channel.Lock()

	var appliedModes string
	var appliedArgs []string

	for _, ch := range modeStr {
		switch ch {
		case '+':
			adding = true
		case '-':
			adding = false
		default:
			// Check if this mode requires an argument
			requiresArg := (adding && strings.ContainsRune(modesWithArgsAdd, ch)) ||
				(!adding && strings.ContainsRune(modesWithArgsRemove, ch))

			// Skip if argument required but not provided
			if requiresArg && modeIndex >= len(modeArgs) {
				continue
			}

			// Process the mode
			switch ch {
			case 'i': // Invite-only
				handleChanModeChange(channel, ch, adding, &appliedModes, nil, &appliedArgs)
				if adding {
					channel.inviteOnly = true
				} else {
					channel.inviteOnly = false
				}
			case 'k': // Channel key
				if adding {
					if modeIndex < len(modeArgs) {
						key := modeArgs[modeIndex]
						modeIndex++
						channel.modeArgs[rune(ch)] = key
						handleChanModeChange(channel, ch, adding, &appliedModes, &key, &appliedArgs)
					}
				} else {
					delete(channel.modeArgs, rune(ch))
					handleChanModeChange(channel, ch, adding, &appliedModes, nil, &appliedArgs)
				}
			case 'l': // User limit
				if adding {
					if modeIndex < len(modeArgs) {
						limit := modeArgs[modeIndex]
						modeIndex++
						channel.modeArgs[rune(ch)] = limit
						handleChanModeChange(channel, ch, adding, &appliedModes, &limit, &appliedArgs)
					}
				} else {
					delete(channel.modeArgs, rune(ch))
					handleChanModeChange(channel, ch, adding, &appliedModes, nil, &appliedArgs)
				}
			case 'b': // Ban
				if modeIndex < len(modeArgs) {
					mask := modeArgs[modeIndex]
					modeIndex++

					if adding {
						if isValidHostmask(mask) {
							// Create a ban entry
							ban := &BanEntry{
								Hostmask: mask,
								Setter:   c.nickname,
								SetTime:  time.Now(),
							}
							channel.bans[mask] = ban
							handleChanModeChange(channel, ch, adding, &appliedModes, &mask, &appliedArgs)
						}
					} else {
						// Remove a ban
						if _, exists := channel.bans[mask]; exists {
							delete(channel.bans, mask)
							handleChanModeChange(channel, ch, adding, &appliedModes, &mask, &appliedArgs)
						}
					}
				} else if adding {
					// List bans
					channel.Unlock()
					c.listChannelBans(target)
					return
				}
			case 'o': // Channel operator
				if modeIndex < len(modeArgs) {
					targetNick := modeArgs[modeIndex]
					modeIndex++

					if _, exists := channel.clients[targetNick]; exists {
						if adding {
							channel.operators[targetNick] = true
						} else {
							delete(channel.operators, targetNick)
						}
						handleChanModeChange(channel, ch, adding, &appliedModes, &targetNick, &appliedArgs)
					}
				}
			case 'v': // Voice
				if modeIndex < len(modeArgs) {
					targetNick := modeArgs[modeIndex]
					modeIndex++

					if _, exists := channel.clients[targetNick]; exists {
						if adding {
							channel.voices[targetNick] = true
						} else {
							delete(channel.voices, targetNick)
						}
						handleChanModeChange(channel, ch, adding, &appliedModes, &targetNick, &appliedArgs)
					}
				}
			case 'h': // Halfop
				if modeIndex < len(modeArgs) {
					targetNick := modeArgs[modeIndex]
					modeIndex++

					if _, exists := channel.clients[targetNick]; exists {
						if adding {
							channel.halfops[targetNick] = true
						} else {
							delete(channel.halfops, targetNick)
						}
						handleChanModeChange(channel, ch, adding, &appliedModes, &targetNick, &appliedArgs)
					}
				}
			case 'a': // Admin
				if modeIndex < len(modeArgs) {
					targetNick := modeArgs[modeIndex]
					modeIndex++

					if _, exists := channel.clients[targetNick]; exists {
						if adding {
							channel.admins[targetNick] = true
						} else {
							delete(channel.admins, targetNick)
						}
						handleChanModeChange(channel, ch, adding, &appliedModes, &targetNick, &appliedArgs)
					}
				}
			case 'q': // Owner
				if modeIndex < len(modeArgs) {
					targetNick := modeArgs[modeIndex]
					modeIndex++

					if _, exists := channel.clients[targetNick]; exists {
						if adding {
							channel.owners[targetNick] = true
						} else {
							delete(channel.owners, targetNick)
						}
						handleChanModeChange(channel, ch, adding, &appliedModes, &targetNick, &appliedArgs)
					}
				}
			default:
				// Simple mode (no argument)
				handleChanModeChange(channel, ch, adding, &appliedModes, nil, &appliedArgs)
			}

			// Update the channel's mode string
			updateChannelModes(channel, ch, adding)
		}
	}

	// Only announce if modes were actually changed
	if appliedModes != "" {
		// Build the mode change announcement
		modeChangeMsg := fmt.Sprintf(":%s!%s@%s MODE %s %s%s",
			c.nickname, c.username, c.hostname, target, appliedModes,
			func() string {
				if len(appliedArgs) > 0 {
					return " " + strings.Join(appliedArgs, " ")
				}
				return ""
			}())

		// Announce to all clients in the channel
		for _, client := range channel.clients {
			client.sendRaw(modeChangeMsg)
		}
	}

	channel.Unlock()
}

// handleUserMode handles user mode changes
func (c *Client) handleUserMode(params []string) {
	// User mode
	if params[0] != c.nickname {
		c.sendNumeric(502, ":Can't change mode for other users")
		return
	}

	if len(params) == 1 {
		// Send current user modes
		c.RLock()
		// Use the new String() method that creates the mode string automatically
		modes := c.Modes.String()
		c.RUnlock()

		if modes == "" {
			modes = "+"
		}
		c.sendNumeric(221, modes)
		return
	}

	// Parse mode changes
	modeChanges := params[1]

	// Use ParseModeString to process the changes
	c.Lock()
	err := c.Modes.ParseModeString(modeChanges)
	c.Unlock()

	if err != nil {
		c.sendNumeric(501, ":Unknown MODE flag")
		return
	}

	// Send the mode change message
	c.sendMessage("MODE", c.nickname, modeChanges)
}

// handleInvite handles an INVITE command
func (c *Client) handleInvite(params []string) {
	if !c.registered {
		c.sendNumeric(451, ":You have not registered")
		return
	}

	if len(params) < 2 {
		c.sendNumeric(461, "INVITE :Not enough parameters")
		return
	}

	targetNick := params[0]
	channelName := params[1]

	// Check if the channel exists
	c.server.RLock()
	channel, channelExists := c.server.channels[channelName]
	c.server.RUnlock()

	if !channelExists {
		c.sendNumeric(403, fmt.Sprintf("%s :No such channel", channelName))
		return
	}

	// Check if the client is on the channel
	c.RLock()
	_, isOnChannel := c.channels[channelName]
	c.RUnlock()

	if !isOnChannel {
		c.sendNumeric(442, fmt.Sprintf("%s :You're not on that channel", channelName))
		return
	}

	// Check if the channel is invite-only and the user is not an operator
	channel.RLock()
	isOperator := channel.operators[c.nickname] || channel.owners[c.nickname] ||
		channel.admins[c.nickname] || c.Modes.Operator
	inviteOnly := strings.Contains(channel.modes, string(CMODE_INVITEONLY))
	channel.RUnlock()

	if inviteOnly && !isOperator {
		c.sendNumeric(482, fmt.Sprintf("%s :You're not a channel operator", channelName))
		return
	}

	// Check if the target user exists
	c.server.RLock()
	targetClient, targetExists := c.server.clients[targetNick]
	c.server.RUnlock()

	if !targetExists {
		c.sendNumeric(401, fmt.Sprintf("%s :No such nick/channel", targetNick))
		return
	}

	// Check if the target user is already on the channel
	targetClient.RLock()
	_, alreadyOnChannel := targetClient.channels[channelName]
	targetClient.RUnlock()

	if alreadyOnChannel {
		c.sendNumeric(443, fmt.Sprintf("%s %s :is already on channel",
			targetNick, channelName))
		return
	}

	// Add the user to the invite list
	c.server.Lock()
	if _, ok := c.server.invites[channelName]; !ok {
		c.server.invites[channelName] = make(map[string]bool)
	}
	c.server.invites[channelName][targetNick] = true
	c.server.Unlock()

	// Send acknowledgment to the inviter
	c.sendNumeric(341, fmt.Sprintf("%s %s", targetNick, channelName))

	// Send invite notification to the target
	targetClient.sendRaw(fmt.Sprintf(":%s!%s@%s INVITE %s :%s",
		c.nickname, c.username, c.hostname, targetNick, channelName))
}

// isChannelBanned checks if a client is banned from a channel
func (c *Client) isChannelBanned(channel *Channel) bool {
	if c.Modes.Operator {
		return false // Operators bypass bans
	}

	hostmask := fmt.Sprintf("%s!%s@%s", c.nickname, c.username, c.hostname)

	channel.RLock()
	defer channel.RUnlock()

	for banMask := range channel.bans {
		if wildcardMatch(hostmask, banMask) {
			return true
		}
	}

	return false
}

// listChannelBans lists the bans for a channel
func (c *Client) listChannelBans(channelName string) {
	c.server.RLock()
	channel, exists := c.server.channels[channelName]
	c.server.RUnlock()

	if !exists {
		c.sendNumeric(403, fmt.Sprintf("%s :No such channel", channelName))
		return
	}

	channel.RLock()
	for mask, ban := range channel.bans {
		c.sendNumeric(367, fmt.Sprintf("%s %s %s %d",
			channelName, mask, ban.Setter, ban.SetTime.Unix()))
	}
	channel.RUnlock()

	c.sendNumeric(368, fmt.Sprintf("%s :End of channel ban list", channelName))
}

// handleChanModeChange tracks the applied modes for announcement
func handleChanModeChange(_ *Channel, mode rune, adding bool, appliedModes *string, arg *string, appliedArgs *[]string) {
	if adding {
		*appliedModes += "+"
	} else {
		*appliedModes += "-"
	}
	*appliedModes += string(mode)

	if arg != nil {
		*appliedArgs = append(*appliedArgs, *arg)
	}
}

// updateChannelModes updates the channel mode string
func updateChannelModes(channel *Channel, mode rune, adding bool) {
	if adding {
		if !strings.ContainsRune(channel.modes, mode) {
			channel.modes += string(mode)
		}
	} else {
		channel.modes = strings.ReplaceAll(channel.modes, string(mode), "")
	}
}
