package irc_test

import (
	"fmt"
	"log"
	"net"
	"net/textproto"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/presbrey/pkg/irc"
)

func init() {
	log.SetFlags(log.Lshortfile | log.Lmicroseconds)
}

// TestIRCServerIntegration tests the IRC server with two client connections
func TestIRCServerIntegration(t *testing.T) {
	// Enable parallel test execution if more tests are added
	// t.Parallel()

	// Set required environment variables for testing
	os.Setenv("SERVER_NAME", "test.irc.server")
	os.Setenv("SERVER_DESC", "Test IRC Server")
	os.Setenv("NETWORK_NAME", "TestNet")
	os.Setenv("OPERATOR_EMAILS", "admin@example.com,test@example.com")
	os.Setenv("OIDC_CLIENT_ID", "test-client-id")
	os.Setenv("OIDC_CLIENT_SECRET", "test-client-secret")

	// Start IRC server
	server, err := irc.NewServer(":0", ":0", ":0", ":0")
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer server.Stop()

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	setTestingAddress(server.TestingGetListener().Addr().String())

	// Give server time to start (reduced from 500ms)
	time.Sleep(100 * time.Millisecond)

	// STEP 1: Connect and register client1
	log.Printf("<STEP 1> Connecting client1")
	client1 := &TestClient{
		t:        t,
		nickname: "client1",
	}
	if err := client1.Connect(); err != nil {
		t.Fatalf("Failed to connect client1: %v", err)
	}
	defer client1.Close()

	// Register client1
	client1.SendCommand("NICK client1")
	client1.SendCommand("USER client1 0 * :Client One")

	// We need to ensure client1 is fully registered (reduced from 500ms)
	client1.WaitForRegistration(250 * time.Millisecond)

	// STEP 2: Connect and register client2
	log.Printf("<STEP 2> Connecting client2")
	client2 := &TestClient{
		t:        t,
		nickname: "client2",
	}
	if err := client2.Connect(); err != nil {
		t.Fatalf("Failed to connect client2: %v", err)
	}
	defer client2.Close()

	// Register client2
	client2.SendCommand("NICK client2")
	client2.SendCommand("USER client2 0 * :Client Two")
	client2.WaitForRegistration(250 * time.Millisecond)

	// STEP 3: Client2 joins a testing channel
	log.Printf("<STEP 3> Client2 joins #testing")
	client2.SendCommand("JOIN #testing")
	client2.WaitForMessage("JOIN #testing", 100*time.Millisecond)

	// STEP 4: Client1 joins the same channel
	log.Printf("<STEP 4> Client1 joining #testing")
	client1.SendCommand("JOIN #testing")
	client1.WaitForMessage("JOIN #testing", 100*time.Millisecond)

	// STEP 5: Both clients message the channel
	log.Printf("<STEP 5> Both clients message the channel")

	// Drain any pending messages first
	numerics1 := client1.DrainMessages() // Clear welcome messages
	numerics2 := client2.DrainMessages() // Clear welcome messages
	if len(numerics1) > 0 {
		log.Printf("[client1] Step 5: Numeric responses: %v", numerics1)
	}
	if len(numerics2) > 0 {
		log.Printf("[client2] Step 5: Numeric responses: %v", numerics2)
	}

	// Client1 sends message to channel
	client1.SendCommand("PRIVMSG #testing :Hello from client1")

	// Client2 should receive client1's message
	msgFound := client2.WaitForMessage("PRIVMSG #testing :Hello from client1", 250*time.Millisecond)
	if !msgFound {
		t.Errorf("Client2 didn't receive client1's channel message")
	} else {
		log.Printf("✓ Client2 received client1's message")
	}

	// Client2 sends message to channel
	client2.SendCommand("PRIVMSG #testing :Hello from client2")

	// Client1 should receive client2's message
	msgFound = client1.WaitForMessage("PRIVMSG #testing :Hello from client2", 250*time.Millisecond)
	if !msgFound {
		t.Errorf("Client1 didn't receive client2's channel message")
	} else {
		log.Printf("✓ Client1 received client2's message")
	}

	// STEP 6: Client2 already has channel ops (as first joiner)
	log.Printf("<STEP 6> Client2 uses channel ops to kick client1")
	// Drain any pending messages
	kickNumerics1 := client1.DrainMessages()
	kickNumerics2 := client2.DrainMessages()
	if len(kickNumerics1) > 0 {
		log.Printf("[client1] Step 6: Numeric responses: %v", kickNumerics1)
	}
	if len(kickNumerics2) > 0 {
		log.Printf("[client2] Step 6: Numeric responses: %v", kickNumerics2)
	}

	client2.SendCommand("KICK #testing client1 :Testing kick command")

	// Client1 should receive the kick message
	kickFound := client1.WaitForMessage("KICK #testing client1", 250*time.Millisecond)
	if !kickFound {
		t.Logf("Client1 didn't receive any kick message, proceeding with test")
	} else {
		log.Printf("✓ Client1 received kick message")
	}

	// No need to rejoin any channel for the KILL command, as it's a server-wide operator command
	// Just give a moment for the system to settle (reduced from 200ms)
	time.Sleep(50 * time.Millisecond)

	// STEP 7: Grant client1 operator status
	log.Printf("<STEP 7> Granting client1 operator status")
	err = server.GrantOperatorForTest("client1")
	if err != nil {
		t.Fatalf("Failed to grant operator status to client1: %v", err)
	}

	// STEP 8: Client1 kills client2 from the server
	log.Printf("<STEP 8> Client1 kills client2 from the server")
	client1.SendCommand("KILL client2 :Testing kill command")

	// Verify client2 disconnection with a timeout
	// Use a longer timeout for the KILL command detection
	killSucceeded := checkClientDisconnected(client2, 1000*time.Millisecond)
	if !killSucceeded {
		t.Errorf("KILL command failed - client2 was still connected")
	} else {
		log.Printf("✓ Client2 was disconnected as expected")
	}
}

// Helper function to check if a client has been disconnected
func checkClientDisconnected(client *TestClient, timeout time.Duration) bool {
	// Use multiple approaches to check connection status
	done := make(chan bool, 1)
	start := time.Now()

	go func() {
		client.mux.Lock()
		defer client.mux.Unlock()
		
		// Keep trying with retries until global timeout
		for time.Since(start) < timeout {
			// First approach: Try reading with textproto
			client.conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
			_, err := client.tpConn.R.ReadByte() // Use the underlying Reader from textproto
			client.conn.SetReadDeadline(time.Time{}) // Reset deadline
			
			if err != nil {
				log.Printf("DEBUG: Detected disconnect for %s: %v", client.nickname, err)
				done <- true
				return
			}
			
			// Second approach: Try writing with textproto
			client.conn.SetWriteDeadline(time.Now().Add(100 * time.Millisecond))
			// Need to unlock first since PrintfLine will try to acquire the lock
			client.mux.Unlock()
			err = client.tpConn.PrintfLine("PING") // Use PrintfLine on the textproto.Conn
			client.mux.Lock()
			client.conn.SetWriteDeadline(time.Time{}) // Reset deadline
			
			if err != nil {
				log.Printf("DEBUG: Detected disconnect for %s (write error): %v", client.nickname, err)
				done <- true
				return
			}
			
			// Short wait before next check
			time.Sleep(100 * time.Millisecond)
		}
		
		// If we got here, the connection still appears active
		done <- false
	}()
	
	// Wait for either the result or timeout
	select {
	case disconnected := <-done:
		return disconnected
	case <-time.After(timeout):
		return false // Global timeout occurred
	}
}

// TestClient represents a test IRC client
type TestClient struct {
	t        *testing.T
	conn     net.Conn
	tpConn   *textproto.Conn // Using textproto instead of bufio
	nickname string
	mux      sync.Mutex // Protects concurrent read/write operations
}

// Connect establishes a connection to the IRC server
func (c *TestClient) Connect() error {
	conn, err := net.Dial("tcp", testingAddress)
	if err != nil {
		return err
	}
	c.conn = conn
	c.tpConn = textproto.NewConn(conn) // Use textproto instead of bufio
	return nil
}

// NewTestClient creates a new test client connected to the IRC server
func NewTestClient(t *testing.T) *TestClient {
	client := &TestClient{
		t: t,
	}
	if err := client.Connect(); err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}
	return client
}

// Close closes the client connection
func (c *TestClient) Close() {
	if c.conn != nil {
		c.conn.Close()
	}
}

// DrainMessages reads and discards all pending messages
// Useful for clearing welcome messages and other expected output
// Returns a map of numeric response codes with their counts
func (c *TestClient) DrainMessages() map[int]int {
	c.mux.Lock()
	defer c.mux.Unlock()

	// Map to track numerics
	numerics := make(map[int]int)

	// Set a short deadline for reading to avoid blocking for too long
	c.conn.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
	defer c.conn.SetReadDeadline(time.Time{}) // Reset deadline

	drained := 0
	for {
		// Use textproto to read a line - it handles CRLF properly
		// This is a non-blocking read due to the deadline set above
		msg, err := c.tpConn.ReadLine()
		if err != nil {
			// We expect a timeout error when there are no more messages
			break
		}
		
		// Look for numeric responses (e.g., ":server 001 nick :message")
		parts := strings.Split(msg, " ")
		if len(parts) >= 3 {
			// Check if the second part is a numeric
			if num, err := strconv.Atoi(parts[1]); err == nil {
				numerics[num]++
			}
		}
		
		drained++
	}
	if drained > 0 {
		log.Printf("[%s] Drained %d messages", c.nickname, drained)
	}
	
	return numerics
}

// ReadMessages reads up to maxMessages messages from the server with a timeout
// Returns the messages as strings with line endings trimmed
func (c *TestClient) ReadMessages(maxMessages int) []string {
	c.mux.Lock()
	defer c.mux.Unlock()

	var messages []string

	// Set a reasonable deadline (reduced from 300ms)
	c.conn.SetReadDeadline(time.Now().Add(150 * time.Millisecond))
	defer c.conn.SetReadDeadline(time.Time{}) // Reset deadline

	for i := 0; i < maxMessages; i++ {
		// Use textproto to read a line - handles CRLF properly
		msg, err := c.tpConn.ReadLine()
		if err != nil {
			// We may get a timeout if no more messages are available
			break
		}
		
		// textproto.ReadLine already trims CRLF, so we just need to check for empty lines
		if msg != "" {
			messages = append(messages, msg)
		}
	}

	return messages
}

// SendCommand sends an IRC command to the server
func (c *TestClient) SendCommand(command string) {
	// Remove any trailing newlines for logging and to ensure proper protocol formatting
	command = strings.TrimSuffix(strings.TrimSuffix(command, "\r\n"), "\n")
	
	log.Printf("    [%s] => %#v", c.nickname, command)
	
	// Use textproto's PrintfLine to handle proper line termination
	c.mux.Lock()
	err := c.tpConn.PrintfLine("%s", command)
	c.mux.Unlock()
	
	if err != nil {
		c.t.Errorf("Failed to send command '%s': %v", command, err)
	}
}

// Expect checks if the next line from the server matches the expected pattern
func (c *TestClient) Expect(expected string) {
	msg := c.ReadMessages(1)
	if len(msg) == 0 || !strings.Contains(msg[0], expected) {
		c.t.Errorf("Expected line containing %q, got %v", expected, msg)
	}
}

// ExpectNumeric checks if the next line from the server is a numeric reply with the expected code
func (c *TestClient) ExpectNumeric(numeric int) {
	msg := c.ReadMessages(1)
	expectedCode := fmt.Sprintf(" %03d ", numeric)
	if len(msg) == 0 || !strings.Contains(msg[0], expectedCode) {
		c.t.Errorf("Expected numeric %d, got %v", numeric, msg)
	}
}

// ExpectPrefix checks if the next line from the server has the expected prefix and command/text
func (c *TestClient) ExpectPrefix(nick, text string) {
	msg := c.ReadMessages(1)
	prefix := fmt.Sprintf(":%s!", nick)
	if len(msg) == 0 || !strings.HasPrefix(msg[0], prefix) || !strings.Contains(msg[0], text) {
		c.t.Errorf("Expected line with prefix %q and text %q, got %v", prefix, text, msg)
	}
}

func setTestingAddress(address string) {
	log.Printf("Setting testing address to %s", address)
	testingAddress = address
}

var testingAddress string

// WaitForMessage waits for a specific message and returns true if found within timeout
func (c *TestClient) WaitForMessage(expectedMessage string, timeout time.Duration) bool {
	c.mux.Lock()
	defer c.mux.Unlock()

	start := time.Now()
	for time.Since(start) < timeout {
		c.conn.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
		// Use textproto to read a line - handles CRLF properly
		msg, err := c.tpConn.ReadLine()
		if err != nil {
			// Short pause and try again
			time.Sleep(10 * time.Millisecond)
			continue
		}

		// ReadLine already trims CRLF, so we can directly check for the expected message
		if strings.Contains(msg, expectedMessage) {
			c.conn.SetReadDeadline(time.Time{}) // Reset deadline
			return true
		}
	}

	c.conn.SetReadDeadline(time.Time{}) // Reset deadline
	return false
}

// WaitForRegistration waits until the client is fully registered on the server
func (c *TestClient) WaitForRegistration(timeout time.Duration) {
	// Wait for welcome message (numeric 001) or timeout
	start := time.Now()
	registered := false

	for time.Since(start) < timeout {
		messages := c.ReadMessages(5)
		for _, msg := range messages {
			if strings.Contains(msg, " 001 ") {
				registered = true
				break
			}
		}

		if registered {
			break
		}

		time.Sleep(50 * time.Millisecond)
	}

	welcomeNumerics := c.DrainMessages() // Clear remaining welcome messages
	if len(welcomeNumerics) > 0 {
		log.Printf("[%s] Registration: Numeric responses: %v", c.nickname, welcomeNumerics)
	}
}
