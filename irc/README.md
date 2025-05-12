A flexible and extensible IRC server library written in Go. It provides a modern approach to building IRC servers with full support for UnrealIRCd compatible channel and user modes.

## Features

- **Flexible Configuration**: Configure via YAML, TOML, or JSON files, with support for remote configuration via HTTPS
- **Plugin System**: Extend functionality through a hook/callback system
- **Web Portal**: Built-in web interface for operator management
- **Bot API**: RESTful API for bots to interact with the IRC server
- **Mode Support**: Complete implementation of UnrealIRCd compatible channel and user modes
- **TLS Support**: Secure your IRC server with TLS
- **Hot Reload**: Rehash configuration without restarting the server

## Quick Start

### Running the Server

```bash
# Run with default configuration (config.yaml)
go run main.go

# Run with a specific configuration file
go run main.go -config /path/to/config.yaml

# Run with a remote configuration
go run main.go -config https://example.com/config.yaml
```

### Configuration

Copy the example configuration file to `config.yaml`:

```bash
cp config.yaml.example config.yaml
```

Edit the configuration to match your needs. See [Configuration](#configuration) for details.

## API Usage

### Core Server

```go
package main

import (
    "log"
    
    "github.com/yourusername/goircd/config"
    "github.com/yourusername/goircd/server"
)

func main() {
    // Load configuration
    cfg, err := config.Load("config.yaml")
    if err != nil {
        log.Fatalf("Failed to load configuration: %v", err)
    }
    
    // Create server
    srv, err := server.NewServer(cfg)
    if err != nil {
        log.Fatalf("Failed to create server: %v", err)
    }
    
    // Register custom hooks
    srv.RegisterHook("PRIVMSG", myCustomHandler)
    
    // Start server
    if err := srv.Start(); err != nil {
        log.Fatalf("Server failed: %v", err)
    }
}

func myCustomHandler(params *server.HookParams) error {
    // Custom handler for PRIVMSG
    client := params.Client
    message := params.Message
    
    // Log all private messages
    log.Printf("PRIVMSG from %s to %s: %s", 
        client.Nickname, 
        message.Params[0], 
        message.Params[1],
    )
    
    return nil
}
```

### Custom Channel Implementation

```go
package main

import (
    "fmt"
    
    "github.com/yourusername/goircd/server"
)

// CustomChannel extends the default Channel
type CustomChannel struct {
    *server.Channel
    CustomData map[string]interface{}
}

// NewCustomChannel creates a new custom channel
func NewCustomChannel(srv *server.Server, name string) *CustomChannel {
    return &CustomChannel{
        Channel:    server.NewChannel(srv, name),
        CustomData: make(map[string]interface{}),
    }
}

// CustomSendToAll sends a message to all members with a prefix
func (c *CustomChannel) CustomSendToAll(message string, except *server.Client) {
    c.SendToAll(fmt.Sprintf("[%s] %s", c.Name, message), except)
}

// Register a factory function to create custom channels
func registerCustomChannelFactory(srv *server.Server) {
    srv.ChannelFactory = func(srv *server.Server, name string) server.Channel {
        return NewCustomChannel(srv, name)
    }
}
```

## Bot API Usage

The Bot API allows bots to interact with the IRC server via a RESTful API. Here's an example of sending a message to a channel:

```bash
curl -X POST \
  http://localhost:8081/api/send \
  -H 'Authorization: Bearer your-secret-token-1' \
  -H 'Content-Type: application/json' \
  -d '{
    "nickname": "MyBot",
    "channel": "#mychannel",
    "message": "Hello, world!"
  }'
```

## Web Portal

The web portal provides a web interface for operator management. It allows operators to:

- View server statistics
- View and manage channels
- View and manage users
- Rehash the server configuration

Operators can log in using their operator credentials or via a magic link sent via IRC.

## Configuration

The configuration file can be in YAML, TOML, or JSON format. See `config.yaml.example` for a complete example.

Key configuration sections:

- `server`: Basic server settings
- `tls`: TLS configuration
- `web_portal`: Web portal configuration
- `bots`: Bot API configuration
- `operators`: Operator definitions
- `plugins`: Plugin configuration

## Environment Variables

All configuration options can be overridden with environment variables. The format is:

```
IRCD_SECTION_KEY=value
```

For example:

- `IRCD_SERVER_NAME=irc.example.com`
- `IRCD_SERVER_PORT=6667`
- `IRCD_TLS_ENABLED=true`

## IRC Commands

GoIRCd supports all standard IRC commands, including:

- `NICK`: Change nickname
- `USER`: Register user
- `JOIN`: Join channel
- `PART`: Leave channel
- `PRIVMSG`: Send message
- `QUIT`: Disconnect
- `MODE`: Change channel or user modes
- `TOPIC`: Change channel topic
- `KICK`: Kick user from channel
- `OPER`: Become an operator
- `KILL`: Forcibly disconnect a user
- `REHASH`: Reload configuration

## Supported Modes

### Channel Modes

- `p`: Private channel
- `s`: Secret channel
- `i`: Invite-only channel
- `n`: No messages from outside
- `m`: Moderated channel
- `t`: Topic settable by channel operators only
- `c`: No colors allowed
- `C`: No CTCPs allowed
- `D`: Delayed join
- `f`: Channel flood protection
- `P`: Permanent channel
- `R`: Only registered users can join
- `K`: No knock allowed
- `N`: No nickname changes while in channel
- `S`: Strip colors from channel messages
- `l`: User limit
- `k`: Channel key (password)

### User Modes

- `i`: Invisible
- `o`: IRC Operator
- `O`: Local Operator
- `s`: Receives server notices
- `w`: Receives wallops
- `z`: Connected via services
- `S`: User is protected
- `k`: User doesn't receive KNOCK notices
- `p`: Blocks details for whois requests
- `r`: Registered user
- `W`: WebIRC user
- `I`: Hides idle time in WHOIS
- `G`: Allow filter bypass
- `C`: No CTCPs

## License

This project is licensed under the MIT License - see the LICENSE file for details.