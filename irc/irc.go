/*
Package irc implements a full-featured Internet Relay Chat (IRC) server
compliant with RFC 2812 (Internet Relay Chat: Client Protocol) and related RFCs.

# Features

## Connection and Authentication

- Full IRC connection lifecycle, including registration sequence (PASS, NICK, USER)
- Connection password protection with the PASS command
- Client-to-server ping/pong keep-alive mechanism
- Operator authentication with traditional username/password and modern OIDC
- Hostname and TLS certificate validation
- Support for PROXY protocol for real IP address preservation

## Channel Operations

- Channel creation and management (JOIN, PART)
- Channel modes supporting:
  - i (invite-only)
  - k (channel key/password)
  - l (user limit)
  - b (ban mask)
  - o (operator)
  - v (voice)
  - h (half-operator)
  - t (topic restriction)
  - m (moderated)
  - n (no external messages)
  - p (private)
  - s (secret)
  - r (registered users only)
  - Q (no kicking users)
  - O (operator only channel)

- Channel topic management with the TOPIC command
- Listing channels with the LIST command
- User listing with the NAMES command
- Channel invitation with the INVITE command
- User removal with the KICK command
- Advanced channel roles support (operators, voice, half-operators, owners, admins)

## Messaging

- Private messages between users with PRIVMSG
- Notices between users with NOTICE
- Channel messages (both PRIVMSG and NOTICE)
- Support for CTCP (Client-To-Client Protocol) messages
- WALLOPS messages to operators and users with +w mode
- Support for AWAY status messages

## User Information and Queries

- User information querying with WHO
- Detailed user information with WHOIS
- Server statistics with STATS
- Network statistics with LUSERS
- AWAY status setting and querying

## User Modes

- Comprehensive user mode system with over 30 supported modes, including:
  - i (invisible)
  - w (receive wallops)
  - o (operator)
  - a (away)
  - r (restricted)
  - Advanced modes for services, bot detection, privacy settings, and more

## Server Administration

- Operator status granting with OPER
- User disconnection with KILL
- Local user banning with KLINE/UNKLINE
- Global user banning with GLINE/UNGLINE
- Server information with VERSION, ADMIN, INFO, TIME, MOTD
- WALLOPS command for operator announcements

## Server Federation

- Server-to-server communication via gRPC
- Distributed ban list propagation
- Synchronized channel and user state

## Security Features

- Multiple authentication methods
  - Traditional username/password for operators
  - OIDC (OpenID Connect) for secure operator authentication

- Ban management:
  - K-lines (local server bans)
  - G-lines (global network bans)

- Hostname-based access control
- Configurable connection limits
- Encrypted communications with TLS support
- Self-signed certificate generation

## RFC Compliance

This implementation adheres to the following RFCs:
- RFC 1459: Internet Relay Chat Protocol
- RFC 2810: Internet Relay Chat: Architecture
- RFC 2811: Internet Relay Chat: Channel Management
- RFC 2812: Internet Relay Chat: Client Protocol
- RFC 2813: Internet Relay Chat: Server Protocol
- RFC 7194: Default Port for Internet Relay Chat (IRC) via TLS/SSL

## Implementation Details

The server is implemented in Go with a focus on:
- High concurrency using goroutines and channels
- Thread-safety with proper synchronization
- Clean separation of concerns (client handling, channel management, server operations)
- Extensibility for future protocol enhancements
- Comprehensive error handling and logging
- Environment-based configuration

# TODO: Missing Commands

The following IRC commands defined in RFC 2812 have not yet been implemented:

## User Commands
- USERHOST: Return a list of information about the specified nicknames
- ISON: Check if the specified nicknames are currently on IRC

## Server Queries and Commands
- LINKS: List all server links in the network
- CONNECT: Request a server connection (operator only)
- TRACE: Find the route to a specific server or user
- SQUIT: Disconnect a server from the network (operator only)

## Server Administration
- REHASH: Reload the server configuration file (operator only)
- RESTART: Restart the server (operator only)
- DIE: Shut down the server (operator only)

## Service Query and Commands
- SERVLIST: List services connected to the network
- SQUERY: Send a message to a specific service

# Usage

To start an IRC server with default configuration:

	server, err := irc.NewServer(":6667", ":6697", "127.0.0.1:8080", ":6668")
	if err != nil {
	    log.Fatalf("Failed to create server: %v", err)
	}

	err = server.Start()
	if err != nil {
	    log.Fatalf("Failed to start server: %v", err)
	}

For more advanced configuration, see the Config struct and load settings from environment variables.
*/
package irc
