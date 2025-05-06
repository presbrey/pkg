package irc

import (
	"fmt"
	"strings"
)

// handleCAP handles capability negotiation commands (CAP LS, CAP REQ, CAP END, etc.)
func (c *Client) handleCAP(params []string) {
	if len(params) < 1 {
		c.sendNumeric(461, "CAP :Not enough parameters")
		return
	}

	subCommand := strings.ToUpper(params[0])

	switch subCommand {
	case "LS":
		c.handleCapLS(params)
	case "LIST":
		c.handleCapLIST()
	case "REQ":
		c.handleCapREQ(params)
	case "END":
		c.handleCapEND()
	case "ACK":
		// Client shouldn't send this, ignore
	case "NAK":
		// Client shouldn't send this, ignore
	default:
		c.sendRaw(fmt.Sprintf(":%s CAP %s %s :Unknown subcommand",
			c.server.Config.ServerName, c.nickname, subCommand))
	}
}

// handleCapLS handles the CAP LS command
func (c *Client) handleCapLS(params []string) {
	version := "302"
	if len(params) > 1 {
		version = params[1]
	}

	c.capabilities.Negotiating = true

	// Build capability list string
	var capList strings.Builder

	for _, cap := range ServerCapabilities {
		if capList.Len() > 0 {
			capList.WriteString(" ")
		}
		capList.WriteString(cap.GetCapabilityString())
	}

	// Output in multiline format if the client specified a version >= 302
	if version == "302" {
		// Modern clients can handle this format (multiline)
		c.sendRaw(fmt.Sprintf(":%s CAP %s LS * :%s",
			c.server.Config.ServerName, "*", capList.String()))
		c.sendRaw(fmt.Sprintf(":%s CAP %s LS :",
			c.server.Config.ServerName, "*"))
	} else {
		// Legacy format
		c.sendRaw(fmt.Sprintf(":%s CAP %s LS :%s",
			c.server.Config.ServerName, "*", capList.String()))
	}
}

// handleCapLIST lists the currently enabled capabilities for this client
func (c *Client) handleCapLIST() {
	var enabledList strings.Builder

	for capName := range c.capabilities.Enabled {
		if enabledList.Len() > 0 {
			enabledList.WriteString(" ")
		}

		cap, exists := ServerCapabilities[capName]
		if exists {
			enabledList.WriteString(cap.GetCapabilityString())
		} else {
			// This shouldn't happen, but just in case
			enabledList.WriteString(capName)
		}
	}

	c.sendRaw(fmt.Sprintf(":%s CAP %s LIST :%s",
		c.server.Config.ServerName, "*", enabledList.String()))
}

// handleCapREQ handles capability requests from clients
func (c *Client) handleCapREQ(params []string) {
	if len(params) < 2 {
		c.sendRaw(fmt.Sprintf(":%s CAP %s NAK :No capabilities specified",
			c.server.Config.ServerName, "*"))
		return
	}

	// Handle capabilities list
	capList := strings.TrimSpace(params[1])
	requestedCaps := strings.Split(capList, " ")

	// Track if we can acknowledge all requested capabilities
	validRequest := true
	c.capabilities.RequestedCaps = nil

	// Check if all requested capabilities are supported
	for _, capName := range requestedCaps {
		if capName == "" {
			continue
		}

		// Handle capability removal requests (prefixed with -)
		isRemoval := false
		if capName[0] == '-' {
			isRemoval = true
			capName = capName[1:]
		}

		// Check if the capability exists
		if _, exists := ServerCapabilities[capName]; !exists && !isRemoval {
			validRequest = false
			break
		}

		c.capabilities.RequestedCaps = append(c.capabilities.RequestedCaps, capName)
	}

	if validRequest {
		// ACK the capabilities
		c.sendRaw(fmt.Sprintf(":%s CAP %s ACK :%s",
			c.server.Config.ServerName, "*", capList))

		// Actually enable/disable the capabilities
		for _, capName := range requestedCaps {
			if capName == "" {
				continue
			}

			isRemoval := false
			if capName[0] == '-' {
				isRemoval = true
				capName = capName[1:]
			}

			if isRemoval {
				c.capabilities.DisableCapability(capName)
			} else {
				c.capabilities.EnableCapability(capName)
			}
		}
	} else {
		// NAK the capabilities
		c.sendRaw(fmt.Sprintf(":%s CAP %s NAK :%s",
			c.server.Config.ServerName, "*", capList))
	}
}

// handleCapEND ends the capability negotiation
func (c *Client) handleCapEND() {
	c.capabilities.Negotiating = false
	c.capabilities.RequestedCaps = nil

	// If the client is already registered, we don't need to do anything
	// If not registered, check if registration can now proceed
	if !c.registered {
		c.tryCompleteRegistration()
	}
}
