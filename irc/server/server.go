package server

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/presbrey/pkg/irc"
	"github.com/presbrey/pkg/irc/config"
)

// Server represents the IRC server
type Server struct {
	config    *config.Config
	startTime time.Time
	clients   sync.Map // map[string]*Client
	channels  sync.Map // map[string]*Channel
	operators sync.Map // map[string]*Operator
	hooks     map[string][]Hook
	mu        sync.RWMutex // Still needed for hooks and other operations
	listener  net.Listener
	listeners []net.Listener
	botAPI    *BotAPI
	webPortal *WebPortal
	quit      chan struct{}
}

// Hook is a function that can be registered to handle various events
type Hook func(params *HookParams) error

// HookParams contains context information for hooks
type HookParams struct {
	Server   *Server
	Client   *Client
	Channel  *Channel
	Message  *irc.Message
	Command  string
	Target   string
	Text     string
	RawInput string
	Data     map[string]interface{}
}

// NewServer creates a new IRC server
func NewServer(cfg *config.Config) (*Server, error) {
	srv := &Server{
		config:    cfg,
		startTime: time.Now(),
		// sync.Map doesn't need initialization with make()
		hooks: make(map[string][]Hook),
		quit:  make(chan struct{}),
	}

	// Initialize the operator list
	for _, op := range cfg.Operators {
		srv.operators.Store(op.Username, &Operator{
			Username: op.Username,
			Password: op.Password,
			Email:    op.Email,
			Mask:     op.Mask,
		})
	}

	// Initialize the web portal if enabled
	if cfg.WebPortal.Enabled {
		portal, err := NewWebPortal(srv, cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize web portal: %v", err)
		}
		srv.webPortal = portal
	}

	// Initialize the bot API if enabled
	if cfg.Bots.Enabled {
		api, err := NewBotAPI(srv, cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize bot API: %v", err)
		}
		srv.botAPI = api
	}

	// Register default hooks
	srv.registerDefaultHooks()

	return srv, nil
}

// Start starts the IRC server with multiple possible listeners
func (s *Server) Start() error {
	var listeners []net.Listener

	// Start unencrypted IRC listener if enabled
	if s.config.ListenIRC.Enabled {
		// Create standard TCP listener
		fmt.Printf("Starting unencrypted IRC server on %s\n", s.config.GetIRCListenAddress())
		listener, err := net.Listen("tcp", s.config.GetIRCListenAddress())
		if err != nil {
			return fmt.Errorf("failed to listen on %s: %v", s.config.GetIRCListenAddress(), err)
		}
		listeners = append(listeners, listener)
	}

	// Start TLS encrypted IRC listener if enabled
	if s.config.ListenTLS.Enabled {
		// Create TLS config
		tlsConfig := &tls.Config{
			MinVersion: tls.VersionTLS12,
		}

		// Check if we need to generate certificates
		if s.config.ListenTLS.Generation {
			cert, key, err := s.generateSelfSignedCert()
			if err != nil {
				return fmt.Errorf("failed to generate self-signed certificate: %v", err)
			}

			// Print the certificates instead of saving to disk
			fmt.Println("========== GENERATED CERTIFICATE ==========")
			fmt.Println(cert)
			fmt.Println("========== GENERATED PRIVATE KEY ==========")
			fmt.Println(key)
			fmt.Println("===========================================")

			// Convert PEM strings to certificate
			certPair, err := tls.X509KeyPair([]byte(cert), []byte(key))
			if err != nil {
				return fmt.Errorf("failed to parse generated certificate: %v", err)
			}
			tlsConfig.Certificates = []tls.Certificate{certPair}
		} else if s.config.ListenTLS.Cert != "" && s.config.ListenTLS.Key != "" {
			// Load certificate and key from files
			cert, err := tls.LoadX509KeyPair(s.config.ListenTLS.Cert, s.config.ListenTLS.Key)
			if err != nil {
				return fmt.Errorf("failed to load TLS certificate: %v", err)
			}
			tlsConfig.Certificates = []tls.Certificate{cert}
		} else {
			return fmt.Errorf("TLS is enabled but no certificate/key provided and auto-generation is disabled")
		}

		// Create TLS listener
		tlsHost := s.config.ListenTLS.Host
		if tlsHost == "" {
			tlsHost = s.config.ListenIRC.Host // Use the same host as IRC if not specified
		}
		tlsAddress := fmt.Sprintf("%s:%d", tlsHost, s.config.ListenTLS.Port)
		fmt.Printf("Starting TLS encrypted IRC server on %s\n", tlsAddress)
		tlsListener, err := tls.Listen("tcp", tlsAddress, tlsConfig)
		if err != nil {
			// Close any previously created listeners
			for _, l := range listeners {
				l.Close()
			}
			return fmt.Errorf("failed to create TLS listener on %s: %v", tlsAddress, err)
		}
		listeners = append(listeners, tlsListener)
	}

	// Ensure at least one listener is active
	if len(listeners) == 0 {
		return fmt.Errorf("no listeners enabled, at least one of ListenIRC or ListenTLS must be enabled")
	}

	// Store all listeners
	s.listeners = listeners

	// Store the first listener as the primary for backward compatibility
	s.listener = listeners[0]

	// Start the web portal if enabled
	if s.webPortal != nil {
		go s.webPortal.Start()
	}

	// Start the bot API if enabled
	if s.botAPI != nil {
		go s.botAPI.Start()
	}

	// Accept and handle connections
	go s.acceptConnections()

	return nil
}

// Stop stops the IRC server
func (s *Server) Stop() error {
	close(s.quit)

	// Close all listeners
	for _, listener := range s.listeners {
		if listener != nil {
			listener.Close()
		}
	}

	// Stop the web portal
	if s.webPortal != nil {
		s.webPortal.Stop()
	}

	// Stop the bot API
	if s.botAPI != nil {
		s.botAPI.Stop()
	}

	// Create a list of clients to disconnect
	clientsToDisconnect := make([]*Client, 0)
	s.clients.Range(func(key, value interface{}) bool {
		client := value.(*Client)
		clientsToDisconnect = append(clientsToDisconnect, client)
		return true // Continue iteration
	})

	// Disconnect all clients from the copied list
	for _, client := range clientsToDisconnect {
		client.Quit("Server shutting down")
	}

	return nil
}

// acceptConnections accepts and handles new connections
func (s *Server) acceptConnections() {
	for i := range s.listeners {
		// Using a local copy of the index for the goroutine
		listenerIndex := i
		go func() {
			for {
				select {
				case <-s.quit:
					// Server is shutting down
					return
				default:
					// Accept new connection
					conn, err := s.listeners[listenerIndex].Accept()
					if err != nil {
						// Check if the server is shutting down
						if errors.Is(err, net.ErrClosed) {
							// Connection closed, exit this goroutine
							return
						}

						// Check if we need to exit
						select {
						case <-s.quit:
							return // Server is shutting down
						default:
							// Not shutting down, log the error
							fmt.Printf("Failed to accept connection on listener %d: %v\n", listenerIndex, err)
							// Add a small delay to avoid tight loops on errors
							time.Sleep(100 * time.Millisecond)
							continue
						}
					}

					// Handle the connection in a goroutine
					go s.handleConnection(conn)
				}
			}
		}()
	}
}

// handleConnection handles a new connection
func (s *Server) handleConnection(conn net.Conn) {
	client := NewClient(s, conn)

	// Register the client (temporary ID before nick registration)
	// No need for mutex with sync.Map
	s.clients.Store(client.ID, client)

	// Handle the client
	client.Handle()
}

// RegisterHook registers a hook for an event
func (s *Server) RegisterHook(event string, hook Hook) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hooks[event] = append(s.hooks[event], hook)
}

// RunHooks runs all hooks for an event
func (s *Server) RunHooks(event string, params *HookParams) error {
	s.mu.RLock()
	hooks := s.hooks[event]
	s.mu.RUnlock()

	for _, hook := range hooks {
		if err := hook(params); err != nil {
			return err
		}
	}
	return nil
}

// registerDefaultHooks registers the default hooks
func (s *Server) registerDefaultHooks() {
	// Register default command handlers
	s.RegisterHook("PASS", handlePass)
	s.RegisterHook("NICK", handleNick)
	s.RegisterHook("USER", handleUser)
	s.RegisterHook("JOIN", handleJoin)
	s.RegisterHook("PART", handlePart)
	s.RegisterHook("PRIVMSG", handlePrivmsg)
	s.RegisterHook("QUIT", handleQuit)
	s.RegisterHook("MODE", handleMode)
	s.RegisterHook("PING", handlePing)
	s.RegisterHook("PONG", handlePong)
	s.RegisterHook("WHO", handleWho)
	s.RegisterHook("WHOIS", handleWhois)
	s.RegisterHook("LIST", handleList)
	s.RegisterHook("NAMES", handleNames)
	s.RegisterHook("TOPIC", handleTopic)
	s.RegisterHook("KICK", handleKick)
	s.RegisterHook("INVITE", handleInvite)
	s.RegisterHook("OPER", handleOper)
	s.RegisterHook("KILL", handleKill)
	s.RegisterHook("REHASH", handleRehash)
}

// GetChannel gets a channel by name
func (s *Server) GetChannel(name string) *Channel {
	// No mutex needed with sync.Map
	value, exists := s.channels.Load(name)
	if !exists {
		return nil
	}
	return value.(*Channel)
}

// CreateChannel creates a new channel
func (s *Server) CreateChannel(name string) *Channel {
	// No mutex needed with sync.Map
	channel := NewChannel(s, name)
	s.channels.Store(name, channel)
	return channel
}

// RemoveChannel removes a channel
func (s *Server) RemoveChannel(name string) {
	// No mutex needed with sync.Map
	s.channels.Delete(name)
}

// GetClient gets a client by nickname
func (s *Server) GetClient(nickname string) *Client {
	// This requires iteration since we're looking up by nickname, not ID
	var result *Client

	// Use Range to iterate through all clients
	s.clients.Range(func(key, value interface{}) bool {
		client := value.(*Client)

		// Add locking when accessing the client's nickname
		client.mu.RLock()
		isMatch := client.Nickname == nickname
		client.mu.RUnlock()

		if isMatch {
			result = client
			return false // Stop iteration
		}
		return true // Continue iteration
	})

	return result
}

// RemoveClient removes a client
func (s *Server) RemoveClient(client *Client) {
	// Remove the client from all channels
	s.channels.Range(func(key, value interface{}) bool {
		channel := value.(*Channel)
		channel.RemoveMember(client)
		return true // Continue iteration
	})

	// Remove the client from the server
	s.clients.Delete(client.ID)
}

// GetOperator gets an operator by username
func (s *Server) GetOperator(username string) *Operator {
	// No mutex needed with sync.Map
	value, exists := s.operators.Load(username)
	if !exists {
		return nil
	}
	return value.(*Operator)
}

// Rehash reloads the server configuration
func (s *Server) Rehash(newSource string) error {
	// Reload the configuration
	err := s.config.Reload(newSource)
	if err != nil {
		return err
	}

	// Update operators
	s.operators = sync.Map{}
	for _, op := range s.config.Operators {
		s.operators.Store(op.Username, &Operator{
			Username: op.Username,
			Password: op.Password,
			Email:    op.Email,
			Mask:     op.Mask,
		})
	}

	// Restart the web portal if needed
	if s.config.WebPortal.Enabled {
		if s.webPortal != nil {
			s.webPortal.Stop()
		}
		portal, err := NewWebPortal(s, s.config)
		if err != nil {
			return fmt.Errorf("failed to reinitialize web portal: %v", err)
		}
		s.webPortal = portal
		go s.webPortal.Start()
	} else if s.webPortal != nil {
		s.webPortal.Stop()
		s.webPortal = nil
	}

	// Restart the bot API if needed
	if s.config.Bots.Enabled {
		if s.botAPI != nil {
			s.botAPI.Stop()
		}
		api, err := NewBotAPI(s, s.config)
		if err != nil {
			return fmt.Errorf("failed to reinitialize bot API: %v", err)
		}
		s.botAPI = api
		go s.botAPI.Start()
	} else if s.botAPI != nil {
		s.botAPI.Stop()
		s.botAPI = nil
	}

	return nil
}

// Broadcast sends a message to all clients
func (s *Server) Broadcast(message string) {
	s.clients.Range(func(key, value interface{}) bool {
		client := value.(*Client)
		client.SendRaw(message)
		return true // Continue iteration
	})
}

// GetConfig returns the server configuration
func (s *Server) GetConfig() *config.Config {
	return s.config
}

// GetUptime returns the server uptime
func (s *Server) GetUptime() time.Duration {
	return time.Since(s.startTime)
}

// GetUserList returns a list of all users
func (s *Server) GetUserList() []string {
	// No mutex needed with sync.Map
	users := make([]string, 0)
	s.clients.Range(func(key, value interface{}) bool {
		client := value.(*Client)
		if client.Nickname != "" {
			users = append(users, client.Nickname)
		}
		return true // Continue iteration
	})
	return users
}

// GetStats returns server statistics
func (s *Server) GetStats() map[string]int {
	// No mutex needed with sync.Map
	stats := make(map[string]int)
	s.clients.Range(func(key, value interface{}) bool {
		client := value.(*Client)
		if client.Nickname != "" {
			stats["users"]++
		}
		return true // Continue iteration
	})
	s.channels.Range(func(key, value interface{}) bool {
		stats["channels"]++
		return true // Continue iteration
	})
	return stats
}

// ClientCount returns the number of connected clients
func (s *Server) ClientCount() int {
	count := 0
	s.clients.Range(func(key, value interface{}) bool {
		count++
		return true // Continue iteration
	})
	return count
}

// ChannelCount returns the number of active channels
func (s *Server) ChannelCount() int {
	count := 0
	s.channels.Range(func(key, value interface{}) bool {
		count++
		return true // Continue iteration
	})
	return count
}

// generateSelfSignedCert generates a self-signed certificate and private key
func (s *Server) generateSelfSignedCert() (string, string, error) {
	// Generate private key
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate private key: %v", err)
	}

	// Define certificate template
	serverName := s.config.Server.Name
	if serverName == "" {
		serverName = "goircd.local"
	}

	// Create a unique serial number
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return "", "", fmt.Errorf("failed to generate serial number: %v", err)
	}

	// Generate a certificate that is valid for 1 year
	notBefore := time.Now()
	notAfter := notBefore.Add(365 * 24 * time.Hour)

	// Create the certificate template
	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"GoIRCd Self-Signed Certificate"},
			CommonName:   serverName,
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	// Add Subject Alternative Names for the server
	host := s.config.ListenIRC.Host
	// If host is 0.0.0.0 or ::, use localhost instead for the certificate
	if host == "0.0.0.0" || host == "::" {
		template.DNSNames = []string{serverName, "localhost"}
		template.IPAddresses = []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")}
	} else {
		// Check if the host is an IP address
		if ip := net.ParseIP(host); ip != nil {
			template.IPAddresses = []net.IP{ip}
		} else {
			template.DNSNames = []string{serverName, host}
		}
	}

	// Create the certificate
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return "", "", fmt.Errorf("failed to create certificate: %v", err)
	}

	// Encode certificate to PEM
	certBuffer := strings.Builder{}
	pem.Encode(&certBuffer, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: derBytes,
	})

	// Encode private key to PEM
	keyBuffer := strings.Builder{}
	keyBytes, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal private key: %v", err)
	}

	pem.Encode(&keyBuffer, &pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: keyBytes,
	})

	return certBuffer.String(), keyBuffer.String(), nil
}
