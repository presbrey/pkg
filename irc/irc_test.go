package irc_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/presbrey/pkg/irc"
	"github.com/presbrey/pkg/irc/config"
	"github.com/presbrey/pkg/irc/server"
	"github.com/stretchr/testify/assert"
)

type IRCClient struct {
	Conn   net.Conn
	Reader *bufio.Reader
}

// NewIRCClient creates a new IRC client
func NewIRCClient(t *testing.T, address string) *IRCClient {
	conn, err := net.Dial("tcp", address)
	assert.NoError(t, err, "Should connect to the server")

	return &IRCClient{
		Conn:   conn,
		Reader: bufio.NewReader(conn),
	}
}

// Send sends a message to the server
func (c *IRCClient) Send(message string) error {
	_, err := c.Conn.Write([]byte(message + "\r\n"))
	return err
}

// Expect waits for a message containing the expected string
func (c *IRCClient) Expect(t *testing.T, expected string, timeout time.Duration) (string, error) {
	// Set a deadline for reading
	c.Conn.SetReadDeadline(time.Now().Add(timeout))
	defer c.Conn.SetReadDeadline(time.Time{})

	// Read until we find the expected string
	for {
		line, err := c.Reader.ReadString('\n')
		if err != nil {
			return "", err
		}

		line = strings.TrimSpace(line)
		if strings.Contains(line, expected) {
			return line, nil
		}
	}
}

// ExpectMultiple waits for multiple messages
func (c *IRCClient) ExpectMultiple(t *testing.T, expected []string, timeout time.Duration) error {
	// Set a deadline for reading
	c.Conn.SetReadDeadline(time.Now().Add(timeout))
	defer c.Conn.SetReadDeadline(time.Time{})

	// Read until we find all expected strings
	remaining := make(map[string]bool)
	for _, exp := range expected {
		remaining[exp] = true
	}

	for len(remaining) > 0 {
		line, err := c.Reader.ReadString('\n')
		if err != nil {
			return err
		}

		line = strings.TrimSpace(line)
		for exp := range remaining {
			if strings.Contains(line, exp) {
				delete(remaining, exp)
			}
		}
	}

	return nil
}

// ReadUntil reads until a specific pattern is found
func (c *IRCClient) ReadUntil(t *testing.T, pattern string, timeout time.Duration) ([]string, error) {
	// Set a deadline for reading
	c.Conn.SetReadDeadline(time.Now().Add(timeout))
	defer c.Conn.SetReadDeadline(time.Time{})

	lines := []string{}
	for {
		line, err := c.Reader.ReadString('\n')
		if err != nil {
			return lines, err
		}

		line = strings.TrimSpace(line)
		lines = append(lines, line)

		if strings.Contains(line, pattern) {
			return lines, nil
		}
	}
}

// Close closes the connection
func (c *IRCClient) Close() error {
	return c.Conn.Close()
}

// TestIntegration runs an integration test for the IRC server
func TestIntegration(t *testing.T) {
	// Create a temporary config file
	tempDir, err := os.MkdirTemp("", "goircd-test-*")
	assert.NoError(t, err, "Should create a temporary directory")
	defer os.RemoveAll(tempDir)

	configPath := filepath.Join(tempDir, "config.yaml")
	configContent := `
server:
  name: test.irc.local
  network: TestNet
  host: 127.0.0.1
  port: 6667
  password: ""

tls:
  enabled: false

web_portal:
  enabled: true
  host: 127.0.0.1
  port: 8080
  tls: false

bots:
  enabled: true
  host: 127.0.0.1
  port: 8081
  bearer_tokens:
    - test-token-1234

operators:
  - username: admin
    password: admin
    email: admin@example.com
    mask: "*@*"
`

	err = os.WriteFile(configPath, []byte(configContent), 0644)
	assert.NoError(t, err, "Should write the config file")

	// Load the configuration
	cfg, err := config.Load(configPath)
	assert.NoError(t, err, "Should load the configuration")

	// Create and start the server
	srv, err := server.NewServer(cfg)
	assert.NoError(t, err, "Should create the server")

	// Start the server in a goroutine
	go func() {
		err := srv.Start()
		if err != nil {
			t.Logf("Server error: %v", err)
		}
	}()

	// Wait for the server to start
	time.Sleep(1 * time.Second)

	// Run the integration test
	t.Run("ServerIntegrationTest", func(t *testing.T) {
		// Connect two clients
		client1 := NewIRCClient(t, "127.0.0.1:6667")
		defer client1.Close()

		client2 := NewIRCClient(t, "127.0.0.1:6667")
		defer client2.Close()

		// Register client 1
		client1.Send("NICK user1")
		client1.Send("USER user1 0 * :Test User 1")

		// Register client 2
		client2.Send("NICK user2")
		client2.Send("USER user2 0 * :Test User 2")

		// Wait for welcome messages
		_, err = client1.Expect(t, "Welcome to the TestNet IRC Network", 5*time.Second)
		assert.NoError(t, err, "Should receive welcome message")

		_, err = client2.Expect(t, "Welcome to the TestNet IRC Network", 5*time.Second)
		assert.NoError(t, err, "Should receive welcome message")

		// Join a channel with both clients
		client1.Send("JOIN #test")
		_, err = client1.Expect(t, "JOIN #test", 1*time.Second)
		assert.NoError(t, err, "Should join the channel")

		client2.Send("JOIN #test")
		_, err = client2.Expect(t, "JOIN #test", 1*time.Second)
		assert.NoError(t, err, "Should join the channel")

		// Send a message from client 1 to the channel
		client1.Send("PRIVMSG #test :Hello, world!")
		_, err = client2.Expect(t, "PRIVMSG #test :Hello, world!", 1*time.Second)
		assert.NoError(t, err, "Client 2 should receive the message")

		// Make a bot send a message to the channel
		sendBotMessage(t, "bot1", "#test", "Hello from the bot!")

		// Both clients should receive the bot message
		_, err = client1.Expect(t, "PRIVMSG #test :Hello from the bot!", 1*time.Second)
		assert.NoError(t, err, "Client 1 should receive the bot message")

		_, err = client2.Expect(t, "PRIVMSG #test :Hello from the bot!", 1*time.Second)
		assert.NoError(t, err, "Client 2 should receive the bot message")

		// Let client 1 become an operator
		client1.Send("OPER admin admin")
		_, err = client1.Expect(t, "MODE user1 +o", 1*time.Second)
		assert.NoError(t, err, "Client 1 should become an operator")

		// Let client 1 kick client 2
		client1.Send("KICK #test user2 :Testing kick")

		// Client 2 should receive the kick
		_, err = client2.Expect(t, "KICK #test user2 :Testing kick", 1*time.Second)
		assert.NoError(t, err, "Client 2 should receive the kick")

		// Let client 2 join the channel again
		client2.Send("JOIN #test")
		_, err = client2.Expect(t, "JOIN #test", 1*time.Second)
		assert.NoError(t, err, "Client 2 should join the channel again")

		// Drain any pending messages (like topic notifications) from client2's connection
		// Set a short timeout for reads so we don't block forever
		client2.Conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		for {
			_, err := client2.Reader.ReadString('\n')
			if err != nil {
				// Reset the deadline after we've drained messages
				client2.Conn.SetReadDeadline(time.Time{})
				break
			}
		}

		// Let client 1 kill client 2
		client1.Send("KILL user2 :Testing kill")

		// First, client2 should receive the KILL message
		killMsg, err := client2.Expect(t, "KILL user2", 1*time.Second)
		assert.NoError(t, err, "Client 2 should receive the KILL message")
		assert.Contains(t, killMsg, "Killed by user1: Testing kill", "Kill message should contain the reason")

		// AFTER receiving the kill message, the connection should be closed
		time.Sleep(100 * time.Millisecond) // Give some time for the server to close the connection
		s, err := client2.Reader.ReadString('\n')
		assert.Error(t, err, "Client 2's connection should be closed")
		assert.Equal(t, "", s)
	})

	// Stop the server
	err = srv.Stop()
	assert.NoError(t, err, "Should stop the server")
}

// sendBotMessage sends a message using the bot API
func sendBotMessage(t *testing.T, nickname string, channel string, message string) {
	// Create the request body
	body := map[string]interface{}{
		"nickname": nickname,
		"channel":  channel,
		"message":  message,
	}

	// Marshal the body to JSON
	bodyBytes, err := json.Marshal(body)
	assert.NoError(t, err, "Should marshal the request body")

	// Create the request
	req, err := http.NewRequest("POST", "http://127.0.0.1:8081/api/send", bytes.NewBuffer(bodyBytes))
	assert.NoError(t, err, "Should create the request")

	// Set the headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-token-1234")

	// Send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	assert.NoError(t, err, "Should send the request")
	defer resp.Body.Close()

	// Check the response
	assert.Equal(t, http.StatusOK, resp.StatusCode, "Should return status OK")

	// Read the response body
	responseBody, err := io.ReadAll(resp.Body)
	assert.NoError(t, err, "Should read the response body")

	// Parse the response
	var response map[string]interface{}
	err = json.Unmarshal(responseBody, &response)
	assert.NoError(t, err, "Should parse the response")

	// Check if the message was sent successfully
	assert.Equal(t, true, response["success"], "Should send the message successfully")
}

// TestChannelModes tests the channel modes
func TestChannelModes(t *testing.T) {
	// Create a channel
	channel := server.NewChannel(nil, "#test")

	// Set some modes
	channel.SetMode('i', true, "")        // +i
	channel.SetMode('s', true, "")        // +s
	channel.SetMode('k', true, "pppp456") // +k password
	channel.SetMode('l', true, "10")      // +l 10

	// Check if the modes are set
	modes := channel.GetModeString()
	assert.Contains(t, modes, "i", "Should set +i mode")
	assert.Contains(t, modes, "s", "Should set +s mode")
	assert.Contains(t, modes, "k", "Should set +k mode")
	assert.Contains(t, modes, "pppp456", "Should set the channel key")
	assert.Contains(t, modes, "l", "Should set +l mode")
	assert.Contains(t, modes, "10", "Should set the user limit")

	// Unset some modes
	channel.SetMode('i', false, "") // -i
	channel.SetMode('k', false, "") // -k

	// Check if the modes are unset
	modes = channel.GetModeString()
	assert.NotContains(t, modes, "i", "Should unset +i mode")
	assert.Contains(t, modes, "s", "Should still have +s mode")
	assert.NotContains(t, modes, "k", "Should unset +k mode")
	assert.NotContains(t, modes, "pppp456", "Should unset the channel key")
	assert.Contains(t, modes, "l", "Should still have +l mode")
	assert.Contains(t, modes, "10", "Should still have the user limit")
}

// TestUserModes tests the user modes
func TestUserModes(t *testing.T) {
	// Create a user
	userModes := server.NewUserModes()

	// Set some modes
	userModes.SetMode('i') // +i
	userModes.SetMode('o') // +o
	userModes.SetMode('w') // +w

	// Check if the modes are set
	modes := userModes.GetModeString()
	assert.Contains(t, modes, "i", "Should set +i mode")
	assert.Contains(t, modes, "o", "Should set +o mode")
	assert.Contains(t, modes, "w", "Should set +w mode")

	// Unset some modes
	userModes.UnsetMode('i') // -i
	userModes.UnsetMode('o') // -o

	// Check if the modes are unset
	modes = userModes.GetModeString()
	assert.NotContains(t, modes, "i", "Should unset +i mode")
	assert.NotContains(t, modes, "o", "Should unset +o mode")
	assert.Contains(t, modes, "w", "Should still have +w mode")
}

// TestMessageParsing tests message parsing
func TestMessageParsing(t *testing.T) {
	// Parse a simple message
	msg := irc.ParseMessage("PING :server1")
	assert.NotNil(t, msg, "Should parse the message")
	assert.Equal(t, "PING", msg.Command, "Should parse the command")
	assert.Equal(t, 1, len(msg.Params), "Should parse the parameters")
	assert.Equal(t, "server1", msg.Params[0], "Should parse the parameter value")

	// Parse a message with a prefix
	msg = irc.ParseMessage(":nick!user@host PRIVMSG #channel :Hello, world!")
	assert.NotNil(t, msg, "Should parse the message")
	assert.Equal(t, "nick!user@host", msg.Prefix, "Should parse the prefix")
	assert.Equal(t, "PRIVMSG", msg.Command, "Should parse the command")
	assert.Equal(t, 2, len(msg.Params), "Should parse the parameters")
	assert.Equal(t, "#channel", msg.Params[0], "Should parse the first parameter")
	assert.Equal(t, "Hello, world!", msg.Params[1], "Should parse the second parameter")

	// Parse a message with multiple parameters
	msg = irc.ParseMessage("MODE #channel +o-v user1 user2")
	assert.NotNil(t, msg, "Should parse the message")
	assert.Equal(t, "MODE", msg.Command, "Should parse the command")
	assert.Equal(t, 4, len(msg.Params), "Should parse the parameters")
	assert.Equal(t, "#channel", msg.Params[0], "Should parse the first parameter")
	assert.Equal(t, "+o-v", msg.Params[1], "Should parse the second parameter")
	assert.Equal(t, "user1", msg.Params[2], "Should parse the third parameter")
	assert.Equal(t, "user2", msg.Params[3], "Should parse the fourth parameter")
}
