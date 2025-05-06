package irc

import (
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"
)

// handleKline handles a KLINE command (operator only)
func (c *Client) handleKline(params []string) {
	if !c.registered {
		c.sendNumeric(451, ":You have not registered")
		return
	}

	if !c.Modes.Operator {
		c.sendNumeric(481, ":Permission Denied - You're not an IRC operator")
		return
	}

	if len(params) < 2 {
		c.sendNumeric(461, "KLINE :Not enough parameters")
		c.sendNotice("KLINE", "Usage: /KLINE <nick!user@host> [duration] :[reason]")
		return
	}

	maskPattern := params[0]

	// Ensure it's a valid mask pattern
	if !isValidHostmask(maskPattern) {
		c.sendNumeric(461, fmt.Sprintf("KLINE :Invalid hostmask format: %s", maskPattern))
		return
	}

	// Parse duration and reason
	var duration time.Duration
	var reason string
	var err error

	if len(params) == 2 {
		// Just the mask and reason
		reason = params[1]
	} else {
		// Try to parse a duration
		duration, err = parseDuration(params[1])
		if err != nil {
			// If it's not a valid duration, assume it's part of the reason
			reason = strings.Join(params[1:], " ")
		} else if len(params) > 2 {
			// We have a valid duration, the rest is the reason
			reason = strings.Join(params[2:], " ")
		} else {
			reason = "No reason"
		}
	}

	// Add the K-line
	ban := &BanEntry{
		Hostmask: maskPattern,
		Reason:   reason,
		Setter:   c.nickname,
		SetTime:  time.Now(),
		Duration: duration,
		IsGlobal: false,
	}

	// Set expiry time if applicable
	if duration > 0 {
		ban.ExpiryTime = time.Now().Add(duration)
	}

	c.server.Lock()
	c.server.klines[maskPattern] = ban
	c.server.Unlock()

	// Notify operators
	c.server.notifyOpers(fmt.Sprintf("K-Line added by %s for %s: %s",
		c.nickname, maskPattern, reason))

	// Disconnect matching clients
	c.server.disconnectBannedClients(ban)

	// Confirm to the operator
	var durationStr string
	if duration > 0 {
		durationStr = fmt.Sprintf(" for %s", duration)
	}
	c.sendNotice("KLINE", fmt.Sprintf("Added K-Line for %s%s: %s",
		maskPattern, durationStr, reason))
}

// handleGline handles a GLINE command (operator only)
func (c *Client) handleGline(params []string) {
	if !c.registered {
		c.sendNumeric(451, ":You have not registered")
		return
	}

	if !c.Modes.Operator {
		c.sendNumeric(481, ":Permission Denied - You're not an IRC operator")
		return
	}

	if len(params) < 2 {
		c.sendNumeric(461, "GLINE :Not enough parameters")
		c.sendNotice("GLINE", "Usage: /GLINE <nick!user@host> [duration] :[reason]")
		return
	}

	maskPattern := params[0]

	// Ensure it's a valid mask pattern
	if !isValidHostmask(maskPattern) {
		c.sendNumeric(461, fmt.Sprintf("GLINE :Invalid hostmask format: %s", maskPattern))
		return
	}

	// Parse duration and reason
	var duration time.Duration
	var reason string
	var err error

	if len(params) == 2 {
		// Just the mask and reason
		reason = params[1]
	} else {
		// Try to parse a duration
		duration, err = parseDuration(params[1])
		if err != nil {
			// If it's not a valid duration, assume it's part of the reason
			reason = strings.Join(params[1:], " ")
		} else if len(params) > 2 {
			// We have a valid duration, the rest is the reason
			reason = strings.Join(params[2:], " ")
		} else {
			reason = "No reason"
		}
	}

	// Add the G-line
	ban := &BanEntry{
		Hostmask: maskPattern,
		Reason:   reason,
		Setter:   c.nickname,
		SetTime:  time.Now(),
		Duration: duration,
		IsGlobal: true,
	}

	// Set expiry time if applicable
	if duration > 0 {
		ban.ExpiryTime = time.Now().Add(duration)
	}

	c.server.Lock()
	c.server.glines[maskPattern] = ban
	c.server.Unlock()

	// Notify operators
	c.server.notifyOpers(fmt.Sprintf("G-Line added by %s for %s: %s",
		c.nickname, maskPattern, reason))

	// Propagate to other servers in the network
	c.server.propagateGline(ban)

	// Disconnect matching clients
	c.server.disconnectBannedClients(ban)

	// Confirm to the operator
	var durationStr string
	if duration > 0 {
		durationStr = fmt.Sprintf(" for %s", duration)
	}
	c.sendNotice("GLINE", fmt.Sprintf("Added G-Line for %s%s: %s",
		maskPattern, durationStr, reason))
}

// handleUnkline handles an UNKLINE command (operator only)
func (c *Client) handleUnkline(params []string) {
	if !c.registered {
		c.sendNumeric(451, ":You have not registered")
		return
	}

	if !c.Modes.Operator {
		c.sendNumeric(481, ":Permission Denied - You're not an IRC operator")
		return
	}

	if len(params) < 1 {
		c.sendNumeric(461, "UNKLINE :Not enough parameters")
		c.sendNotice("UNKLINE", "Usage: /UNKLINE <hostmask>")
		return
	}

	hostmask := params[0]

	c.server.Lock()
	if _, exists := c.server.klines[hostmask]; exists {
		delete(c.server.klines, hostmask)
		c.server.Unlock()

		// Notify operators
		c.server.notifyOpers(fmt.Sprintf("K-Line for %s removed by %s",
			hostmask, c.nickname))

		c.sendNotice("UNKLINE", fmt.Sprintf("Removed K-Line for %s", hostmask))
	} else {
		c.server.Unlock()
		c.sendNotice("UNKLINE", fmt.Sprintf("No K-Line found for %s", hostmask))
	}
}

// handleUngline handles an UNGLINE command (operator only)
func (c *Client) handleUngline(params []string) {
	if !c.registered {
		c.sendNumeric(451, ":You have not registered")
		return
	}

	if !c.Modes.Operator {
		c.sendNumeric(481, ":Permission Denied - You're not an IRC operator")
		return
	}

	if len(params) < 1 {
		c.sendNumeric(461, "UNGLINE :Not enough parameters")
		c.sendNotice("UNGLINE", "Usage: /UNGLINE <hostmask>")
		return
	}

	hostmask := params[0]

	c.server.Lock()
	if _, exists := c.server.glines[hostmask]; exists {
		delete(c.server.glines, hostmask)
		c.server.Unlock()

		// Notify operators
		c.server.notifyOpers(fmt.Sprintf("G-Line for %s removed by %s",
			hostmask, c.nickname))

		// Propagate to other servers
		c.server.propagateUngline(hostmask)

		c.sendNotice("UNGLINE", fmt.Sprintf("Removed G-Line for %s", hostmask))
	} else {
		c.server.Unlock()
		c.sendNotice("UNGLINE", fmt.Sprintf("No G-Line found for %s", hostmask))
	}
}

// disconnectBannedClients disconnects clients matching a ban entry
func (s *Server) disconnectBannedClients(ban *BanEntry) {
	s.RLock()
	defer s.RUnlock()

	// Find matching clients
	for _, client := range s.clients {
		client.RLock()
		hostmask := fmt.Sprintf("%s!%s@%s", client.nickname, client.username, client.hostname)
		client.RUnlock()

		if wildcardMatch(hostmask, ban.Hostmask) {
			// Notify the client they're banned
			var banType string
			if ban.IsGlobal {
				banType = "G-lined"
			} else {
				banType = "K-lined"
			}

			client.sendError(fmt.Sprintf("Closing Link: %s [%s: %s]",
				client.hostname, banType, ban.Reason))

			// Disconnect the client
			client.quit(fmt.Sprintf("%s: %s", banType, ban.Reason))
		}
	}
}

// notifyOpers sends a notice to all operators
func (s *Server) notifyOpers(message string) {
	s.RLock()
	defer s.RUnlock()

	for _, client := range s.clients {
		client.RLock()
		isOper := client.Modes.Operator
		client.RUnlock()

		if isOper {
			client.sendNotice("NOTICE", message)
		}
	}
}

// propagateGline propagates a G-line to all connected servers
func (s *Server) propagateGline(ban *BanEntry) {
	// Implementation will be done in the gRPC peer communication
	// We need to extend the protobuf definition to include G-line propagation
	log.Printf("G-Line propagation: %s (%s)", ban.Hostmask, ban.Reason)
}

// propagateUngline propagates a G-line removal to all connected servers
func (s *Server) propagateUngline(hostmask string) {
	// Implementation will be done in the gRPC peer communication
	log.Printf("G-Line removal propagation: %s", hostmask)
}

// sendNotice sends a notice to a client
func (c *Client) sendNotice(sender, message string) {
	c.sendRaw(fmt.Sprintf(":%s NOTICE %s :%s",
		sender, c.nickname, message))
}

// isValidHostmask checks if a hostmask is valid
func isValidHostmask(hostmask string) bool {
	// Simple pattern check: nick!user@host with wildcards allowed
	pattern := `^[a-zA-Z0-9_\*\[\]\\\^\{\}~\|]+![a-zA-Z0-9_\*\[\]\\\^\{\}~\|]+@[a-zA-Z0-9_\*\[\]\\\^\{\}~\|\.-]+$`
	matched, _ := regexp.MatchString(pattern, hostmask)
	return matched
}

// parseDuration parses an IRC-style duration string (1d2h3m)
func parseDuration(s string) (time.Duration, error) {
	// Extract parts with regex
	re := regexp.MustCompile(`(\d+)([smhdwy])`)
	matches := re.FindAllStringSubmatch(s, -1)

	if len(matches) == 0 {
		return 0, fmt.Errorf("invalid duration format")
	}

	var duration time.Duration
	for _, match := range matches {
		num := match[1]
		unit := match[2]

		var unitDuration time.Duration
		switch unit {
		case "s":
			unitDuration = time.Second
		case "m":
			unitDuration = time.Minute
		case "h":
			unitDuration = time.Hour
		case "d":
			unitDuration = 24 * time.Hour
		case "w":
			unitDuration = 7 * 24 * time.Hour
		case "y":
			unitDuration = 365 * 24 * time.Hour
		default:
			return 0, fmt.Errorf("unknown duration unit: %s", unit)
		}

		var intValue int
		fmt.Sscanf(num, "%d", &intValue)
		duration += time.Duration(intValue) * unitDuration
	}

	return duration, nil
}
