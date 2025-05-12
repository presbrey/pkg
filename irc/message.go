package irc

import (
	"fmt"
	"strings"
)

// Message represents an IRC message
type Message struct {
	Prefix  string
	Command string
	Params  []string
}

// ParseMessage parses an IRC message
func ParseMessage(line string) *Message {
	if line == "" {
		return nil
	}

	msg := &Message{
		Params: make([]string, 0),
	}

	// Check if the message has a prefix
	if line[0] == ':' {
		parts := strings.SplitN(line[1:], " ", 2)
		if len(parts) < 2 {
			return nil
		}
		msg.Prefix = parts[0]
		line = parts[1]
	}

	// Split the rest of the line by spaces
	parts := strings.SplitN(line, " ", 2)
	if len(parts) == 0 {
		return nil
	}

	msg.Command = strings.ToUpper(parts[0])
	if len(parts) > 1 {
		paramPart := parts[1]

		// Parse parameters
		for paramPart != "" {
			// Check if this is the last parameter (starts with a colon)
			if paramPart[0] == ':' {
				msg.Params = append(msg.Params, paramPart[1:])
				break
			}

			// Otherwise, split by space
			parts := strings.SplitN(paramPart, " ", 2)
			msg.Params = append(msg.Params, parts[0])
			if len(parts) > 1 {
				paramPart = parts[1]
			} else {
				break
			}
		}
	}

	return msg
}

// String returns the string representation of the message
func (m *Message) String() string {
	var builder strings.Builder

	// Add prefix if present
	if m.Prefix != "" {
		builder.WriteString(":")
		builder.WriteString(m.Prefix)
		builder.WriteString(" ")
	}

	// Add command
	builder.WriteString(m.Command)

	// Add parameters
	for i, param := range m.Params {
		builder.WriteString(" ")

		// If this is the last parameter and it contains spaces or starts with a colon, add a colon
		if i == len(m.Params)-1 && (strings.Contains(param, " ") || strings.HasPrefix(param, ":")) {
			builder.WriteString(":")
			builder.WriteString(param)
		} else {
			builder.WriteString(param)
		}
	}

	return builder.String()
}

// Reply creates a new message as a reply to this message
func (m *Message) Reply(prefix, command string, params ...string) *Message {
	return &Message{
		Prefix:  prefix,
		Command: command,
		Params:  params,
	}
}

// ErrorReply creates an error reply to this message
func (m *Message) ErrorReply(prefix, errorCode, target, message string) *Message {
	return &Message{
		Prefix:  prefix,
		Command: errorCode,
		Params:  []string{target, message},
	}
}

// ParseHostmask parses a hostmask (nick!user@host)
func ParseHostmask(hostmask string) (nick, user, host string) {
	nickParts := strings.SplitN(hostmask, "!", 2)
	if len(nickParts) < 2 {
		nick = hostmask
		return
	}
	nick = nickParts[0]

	userHostParts := strings.SplitN(nickParts[1], "@", 2)
	if len(userHostParts) < 2 {
		user = nickParts[1]
		return
	}
	user = userHostParts[0]
	host = userHostParts[1]

	return
}

// FormatHostmask formats a hostmask
func FormatHostmask(nick, user, host string) string {
	return fmt.Sprintf("%s!%s@%s", nick, user, host)
}
