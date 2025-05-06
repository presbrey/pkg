package irc

import (
	"bufio"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/caarlos0/env/v6"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Server represents an IRC server instance
type Server struct {
	sync.RWMutex
	config        *Config
	clients       map[string]*Client // nickname -> client
	channels      map[string]*Channel
	operators     map[string]bool            // email address -> operator status
	opCredentials map[string]string          // username -> password hash
	klines        map[string]*BanEntry       // hostmask -> ban entry
	glines        map[string]*BanEntry       // hostmask -> ban entry
	invites       map[string]map[string]bool // channel -> nickname -> invited
	peers         map[string]*grpc.ClientConn
	peerServer    *grpc.Server
	listener      net.Listener
	tlsListener   net.Listener
	tlsConfig     *tls.Config
	shutdown      chan struct{}
	stats         *ServerStats
}

// BanEntry represents a K-line or G-line ban
type BanEntry struct {
	Hostmask   string        // The hostmask pattern (nick!user@host)
	Reason     string        // Reason for the ban
	Setter     string        // Who set the ban
	SetTime    time.Time     // When the ban was set
	Duration   time.Duration // How long the ban lasts (0 = permanent)
	ExpiryTime time.Time     // When the ban expires (Zero time = permanent)
	IsGlobal   bool          // Whether this is a G-line (network-wide)
}

// Channel represents an IRC channel
type Channel struct {
	sync.RWMutex
	name       string
	topic      string
	clients    map[string]*Client
	modes      string               // Channel modes (i=invite-only, m=moderated, etc.)
	modeArgs   map[rune]string      // Arguments for modes that have them
	operators  map[string]bool      // Channel operators (@)
	voices     map[string]bool      // Voiced users (+)
	halfops    map[string]bool      // Half-operators (%)
	owners     map[string]bool      // Channel owners (~)
	admins     map[string]bool      // Channel admins (&)
	bans       map[string]*BanEntry // Ban list (mode +b)
	inviteOnly bool                 // Whether the channel is invite-only
}

// Config represents server configuration using environment variables
type Config struct {
	ServerName          string               `env:"SERVER_NAME" envDefault:"irc.example.com"`
	ServerDesc          string               `env:"SERVER_DESC" envDefault:"IRC Server"`
	NetworkName         string               `env:"NETWORK_NAME" envDefault:"IRCNet"`
	ConnectionPassword  string               `env:"CONNECTION_PASSWORD" envDefault:""`
	OperatorEmails      []string             `env:"OPERATOR_EMAILS" envSeparator:","`
	OperatorCredentials []OperatorCredential `env:"OPERATOR_CREDENTIALS" envSeparator:";"`
	PeerAddresses       []string             `env:"PEER_ADDRESSES" envSeparator:","`

	// Bind addresses from CLI flags, not environment
	IRCBindAddr   string
	AdminBindAddr string
	GRPCBindAddr  string
	TLSBindAddr   string

	// OAuth/OIDC configuration
	OIDCIssuer       string `env:"OIDC_ISSUER" envDefault:"https://accounts.google.com"`
	OIDCClientID     string `env:"OIDC_CLIENT_ID"`
	OIDCClientSecret string `env:"OIDC_CLIENT_SECRET"`
	OIDCRedirectURL  string `env:"OIDC_REDIRECT_URL" envDefault:"http://localhost:8080/callback"`

	// TLS configuration
	CertFile string `env:"CERT_FILE"`
	KeyFile  string `env:"KEY_FILE"`

	// Optionally save generated certificate and key
	SaveGeneratedCert bool   `env:"SAVE_GENERATED_CERT" envDefault:"false"`
	GeneratedCertPath string `env:"GENERATED_CERT_PATH" envDefault:"certs/server.crt"`
	GeneratedKeyPath  string `env:"GENERATED_KEY_PATH" envDefault:"certs/server.key"`

	// Protocol options
	EnableProxyProtocol bool `env:"ENABLE_PROXY_PROTOCOL" envDefault:"false"`
}

// OperatorCredential represents an IRC operator's credentials
type OperatorCredential struct {
	Username string
	Password string
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
// Format: username:password
func (o *OperatorCredential) UnmarshalText(text []byte) error {
	s := string(text)
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid operator credential format, expected username:password")
	}
	o.Username = parts[0]
	o.Password = parts[1]
	return nil
}

// ServerStats holds real-time server statistics
type ServerStats struct {
	sync.RWMutex
	StartTime        time.Time
	ConnectionCount  int
	MaxConnections   int
	MessagesSent     int64
	MessagesReceived int64
	ChannelCount     int
	PeakUserCount    int
	KlineHits        int
	GlineHits        int
}

// NewServer creates a new IRC server with the given bind addresses
func NewServer(ircBindAddr, tlsBindAddr, adminBindAddr, grpcBindAddr string) (*Server, error) {
	// Load configuration from environment variables
	config := &Config{}
	if err := env.Parse(config); err != nil {
		return nil, fmt.Errorf("failed to parse environment variables: %w", err)
	}

	// Set bind addresses from CLI arguments
	config.IRCBindAddr = ircBindAddr
	config.TLSBindAddr = tlsBindAddr
	config.GRPCBindAddr = grpcBindAddr

	s := &Server{
		config:        config,
		clients:       make(map[string]*Client),
		channels:      make(map[string]*Channel),
		operators:     make(map[string]bool),
		opCredentials: make(map[string]string),
		klines:        make(map[string]*BanEntry),
		glines:        make(map[string]*BanEntry),
		invites:       make(map[string]map[string]bool),
		peers:         make(map[string]*grpc.ClientConn),
		shutdown:      make(chan struct{}),
		stats:         &ServerStats{StartTime: time.Now()},
	}

	// Set up operator emails
	for _, email := range config.OperatorEmails {
		s.operators[email] = true
	}

	// Set up traditional operator credentials
	for _, cred := range config.OperatorCredentials {
		s.opCredentials[cred.Username] = cred.Password
	}

	// // Initialize OIDC provider
	// if err := s.initOIDC(); err != nil {
	// 	return nil, fmt.Errorf("failed to initialize OIDC: %w", err)
	// }

	return s, nil
}

// Start starts all components of the IRC server
func (s *Server) Start() error {
	// Start all server components in sequence
	// If any of the starts fail, we'll automatically stop any previously started components
	if err := s.StartIRCServer(); err != nil {
		return err
	}

	if err := s.StartGRPCServer(); err != nil {
		s.StopIRCServer()
		return err
	}

	if err := s.StartTLSServer(); err != nil {
		s.StopIRCServer()
		s.StopGRPCServer()
		return err
	}

	return nil
}

// StartIRCServer starts only the IRC listener component
func (s *Server) StartIRCServer() error {
	var err error

	// Only start if it's not already running
	if s.listener != nil {
		return nil
	}

	// Set up the IRC listener
	s.listener, err = net.Listen("tcp", s.config.IRCBindAddr)
	if err != nil {
		return fmt.Errorf("failed to start IRC listener: %w", err)
	}
	log.Printf("IRC Server started on %s", s.listener.Addr().String())

	// Start accepting connections
	go s.acceptConnections()

	return nil
}

// StopIRCServer stops only the IRC listener component
func (s *Server) StopIRCServer() error {
	if s.listener != nil {
		if err := s.listener.Close(); err != nil {
			return fmt.Errorf("error closing IRC listener: %w", err)
		}
		s.listener = nil
		log.Printf("IRC Server stopped")
	}
	return nil
}

// StartGRPCServer starts only the gRPC server component
func (s *Server) StartGRPCServer() error {
	// Only start if it's not already running
	if s.peerServer != nil {
		return nil
	}

	return s.startGRPCServer()
}

// StopGRPCServer stops only the gRPC server component
func (s *Server) StopGRPCServer() error {
	if s.peerServer != nil {
		s.peerServer.GracefulStop()
		s.peerServer = nil
		log.Printf("gRPC Server stopped")
	}
	return nil
}

// ConnectToPeers connects to peer servers after the server is already running
func (s *Server) ConnectToPeers() error {
	// Skip if no peer addresses are configured
	if len(s.config.PeerAddresses) == 0 {
		log.Println("No peer servers configured")
		return nil
	}

	log.Println("Connecting to peer servers...")
	for _, address := range s.config.PeerAddresses {
		conn, err := grpc.Dial(address,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithBlock(),
			grpc.WithTimeout(5*time.Second),
		)
		if err != nil {
			log.Printf("Warning: failed to connect to peer %s: %v", address, err)
			continue
		}

		s.peers[address] = conn
		log.Printf("Connected to peer server at %s", address)

		// Initial state sync
		go s.syncWithPeer(address, conn)
	}

	return nil
}

// proxyConn wraps a net.Conn and a bufio.Reader to handle PROXY protocol
// It ensures that data read into the buffer by the reader isn't lost.
type proxyConn struct {
	net.Conn
	reader *bufio.Reader
}

// Read reads data from the connection, prioritizing the bufio.Reader's buffer.
func (pc *proxyConn) Read(b []byte) (int, error) {
	n, err := pc.reader.Read(b)
	if err != nil && err != io.EOF {
		return n, err
	}
	if n < len(b) {
		n2, err2 := pc.Conn.Read(b[n:])
		return n + n2, err2
	}
	return n, err
}

// acceptConnections accepts incoming client connections
func (s *Server) acceptConnections() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.shutdown:
				return
			default:
				log.Printf("Error accepting connection: %v", err)
				continue
			}
		}

		// Track connection in stats
		s.stats.Lock()
		s.stats.ConnectionCount++
		if s.stats.ConnectionCount > s.stats.MaxConnections {
			s.stats.MaxConnections = s.stats.ConnectionCount
		}
		s.stats.Unlock()

		remoteAddr := conn.RemoteAddr().String()
		if s.config.EnableProxyProtocol {
			conn, remoteAddr = s.handleProxyProtocol(conn)
		}

		// Check for K-lines and G-lines before proceeding
		if banned, reason := s.checkBans(remoteAddr); banned {
			// Send an error message before disconnecting
			writer := bufio.NewWriter(conn)
			fmt.Fprintf(writer, "ERROR :Closing Link: %s [%s]\r\n", remoteAddr, reason)
			writer.Flush()
			conn.Close()

			// Increment ban hit stats
			s.stats.Lock()
			s.stats.KlineHits++
			s.stats.Unlock()

			log.Printf("Rejected connection from %s (banned: %s)", remoteAddr, reason)
			continue
		}

		client := &Client{
			conn:     conn,
			server:   s,
			channels: make(map[string]bool),
			lastPong: time.Now(),
			writer:   bufio.NewWriter(conn),
			hostname: remoteAddr,
		}

		go client.handleConnection()
	}
}

// handleProxyProtocol processes the PROXY protocol header if present
// Returns a net.Conn (potentially wrapped) and the client's real address string.
func (s *Server) handleProxyProtocol(conn net.Conn) (net.Conn, string) {
	// Set a short deadline for reading the potential PROXY header
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	defer conn.SetReadDeadline(time.Time{}) // Clear the deadline

	reader := bufio.NewReader(conn)

	// Try to peek at the beginning of the connection to see if it's a PROXY header
	header, err := reader.Peek(5)
	if err != nil {
		// Unable to peek or connection closed/timeout, assume no PROXY header
		// Return the original connection
		return conn, conn.RemoteAddr().String()
	}

	// Check for PROXY protocol signature
	if string(header) == "PROXY" {
		// Read the full PROXY line (this consumes data from the reader's buffer)
		line, err := reader.ReadString('\n')
		if err != nil {
			// Error reading the line after seeing "PROXY", return original conn
			log.Printf("Error reading PROXY line from %s: %v", conn.RemoteAddr(), err)
			return conn, conn.RemoteAddr().String()
		}

		// Parse the PROXY header
		parts := strings.Split(strings.TrimSpace(line), " ")
		if len(parts) >= 6 && parts[0] == "PROXY" {
			// Format: PROXY TCP4/TCP6 client_ip proxy_ip client_port proxy_port
			proto := parts[1]
			srcIP := parts[2]
			srcPort := parts[4]

			if proto == "TCP4" || proto == "TCP6" {
				clientAddr := fmt.Sprintf("%s:%s", srcIP, srcPort)
				log.Printf("PROXY protocol detected from %s -> %s", conn.RemoteAddr(), clientAddr)
				// Return the wrapped connection to preserve buffered data
				return &proxyConn{Conn: conn, reader: reader}, clientAddr
			}
		}
		// If parsing failed after seeing "PROXY", log it but return original conn for now.
		// Ideally, might close connection due to protocol error.
		log.Printf("Invalid PROXY line received from %s: %s", conn.RemoteAddr(), line)
	}

	// No valid PROXY header found or processed
	// Return the original connection
	return conn, conn.RemoteAddr().String()
}

// checkBans checks if an address is banned by K-lines or G-lines
func (s *Server) checkBans(hostAddr string) (banned bool, reason string) {
	s.RLock()
	defer s.RUnlock()

	// Format for checking - we don't have nick/user yet, so use wildcards
	checkMask := fmt.Sprintf("*!*@%s", hostAddr)

	// First check K-lines (local bans)
	for mask, entry := range s.klines {
		// Skip expired bans
		if !entry.ExpiryTime.IsZero() && time.Now().After(entry.ExpiryTime) {
			continue
		}

		// Check if the hostmask matches
		if wildcardMatch(checkMask, mask) {
			return true, fmt.Sprintf("K-lined: %s", entry.Reason)
		}
	}

	// Then check G-lines (network bans)
	for mask, entry := range s.glines {
		// Skip expired bans
		if !entry.ExpiryTime.IsZero() && time.Now().After(entry.ExpiryTime) {
			continue
		}

		// Check if the hostmask matches
		if wildcardMatch(checkMask, mask) {
			return true, fmt.Sprintf("G-lined: %s", entry.Reason)
		}
	}

	return false, ""
}

// Stop stops the IRC server
func (s *Server) Stop() error {
	log.Printf("Stopping IRC server...")

	// Signal shutdown
	close(s.shutdown)

	// Disconnect clients
	s.Lock()
	for _, client := range s.clients {
		client.sendMessage("ERROR", "Server shutting down")
		client.conn.Close()
	}
	s.clients = make(map[string]*Client)
	s.Unlock()

	// Disconnect peers
	for name, conn := range s.peers {
		conn.Close()
		delete(s.peers, name)
	}

	// Stop all server components
	var errMsgs []string

	// Stop IRC listener
	if err := s.StopIRCServer(); err != nil {
		errMsgs = append(errMsgs, fmt.Sprintf("error stopping IRC server: %v", err))
	}

	// Stop TLS listener
	if err := s.StopTLSServer(); err != nil {
		errMsgs = append(errMsgs, fmt.Sprintf("error stopping TLS server: %v", err))
	}

	// Stop gRPC server
	if err := s.StopGRPCServer(); err != nil {
		errMsgs = append(errMsgs, fmt.Sprintf("error stopping gRPC server: %v", err))
	}

	// Handle errors if any
	if len(errMsgs) > 0 {
		return fmt.Errorf("errors during shutdown: %s", strings.Join(errMsgs, "; "))
	}

	log.Printf("IRC server completely stopped")
	return nil
}

// generateSelfSignedCert generates a self-signed certificate and key
func (s *Server) generateSelfSignedCert() (*tls.Certificate, error) {
	// Generate a new private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	// Prepare certificate template
	notBefore := time.Now()
	notAfter := notBefore.Add(365 * 24 * time.Hour) // Valid for 1 year

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, fmt.Errorf("failed to generate serial number: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{s.config.NetworkName},
			CommonName:   s.config.ServerName,
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{s.config.ServerName},
	}

	// Add IP addresses to the certificate
	if host, _, err := net.SplitHostPort(s.config.TLSBindAddr); err == nil {
		if ip := net.ParseIP(host); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		}
	}

	// Self-sign the certificate
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate: %w", err)
	}

	// Create a certificate object
	cert := &tls.Certificate{
		Certificate: [][]byte{derBytes},
		PrivateKey:  privateKey,
	}

	return cert, nil
}

// StartTLSServer starts the TLS IRC listener component
func (s *Server) StartTLSServer() error {
	var err error

	// Only start if it's not already running
	if s.tlsListener != nil {
		return nil
	}

	// If no TLS bind address is specified, don't start TLS
	if s.config.TLSBindAddr == "" {
		log.Println("TLS IRC Server not started (no bind address specified)")
		return nil
	}

	// Set up TLS configuration
	var tlsConfig *tls.Config

	// Check if certificate and key files are provided
	if s.config.CertFile != "" && s.config.KeyFile != "" {
		// Load the certificate and key from files
		cert, err := tls.LoadX509KeyPair(s.config.CertFile, s.config.KeyFile)
		if err != nil {
			return fmt.Errorf("failed to load TLS certificate: %w", err)
		}
		tlsConfig = &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		}
		log.Printf("Using TLS certificate from %s and key from %s", s.config.CertFile, s.config.KeyFile)
	} else {
		// Generate a self-signed certificate
		log.Println("No TLS certificate provided, generating a self-signed certificate")
		cert, err := s.generateSelfSignedCert()
		if err != nil {
			return fmt.Errorf("failed to generate self-signed certificate: %w", err)
		}
		tlsConfig = &tls.Config{
			Certificates: []tls.Certificate{*cert},
			MinVersion:   tls.VersionTLS12,
		}

		// Optionally save the generated certificate and key to files
		if s.config.SaveGeneratedCert {
			certPath := s.config.GeneratedCertPath
			keyPath := s.config.GeneratedKeyPath
			certDir := filepath.Dir(certPath)
			if err := os.MkdirAll(certDir, 0755); err == nil {
				// Save certificate
				certOut, err := os.Create(certPath)
				if err == nil {
					pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: cert.Certificate[0]})
					certOut.Close()
					log.Printf("Self-signed certificate saved to %s", certPath)
				}

				// Save private key
				keyOut, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
				if err == nil {
					privateKey := cert.PrivateKey.(*rsa.PrivateKey)
					pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})
					keyOut.Close()
					log.Printf("Private key saved to %s", keyPath)
				}
			}
		}
	}

	s.tlsConfig = tlsConfig

	// Set up the TLS listener
	s.tlsListener, err = tls.Listen("tcp", s.config.TLSBindAddr, tlsConfig)
	if err != nil {
		return fmt.Errorf("failed to start TLS IRC listener: %w", err)
	}
	log.Printf("TLS IRC Server started on %s", s.tlsListener.Addr().String())

	// Start accepting connections
	go s.acceptTLSConnections()

	return nil
}

// StopTLSServer stops the TLS IRC listener component
func (s *Server) StopTLSServer() error {
	if s.tlsListener != nil {
		err := s.tlsListener.Close()
		s.tlsListener = nil
		if err != nil {
			return fmt.Errorf("failed to stop TLS IRC listener: %w", err)
		}
		log.Println("TLS IRC Server stopped")
	}
	return nil
}

// acceptTLSConnections accepts incoming TLS client connections
func (s *Server) acceptTLSConnections() {
	for {
		conn, err := s.tlsListener.Accept()
		if err != nil {
			select {
			case <-s.shutdown:
				return
			default:
				log.Printf("Error accepting TLS connection: %v", err)
				continue
			}
		}

		// Update connection statistics
		s.stats.Lock()
		s.stats.ConnectionCount++
		if s.stats.ConnectionCount > s.stats.MaxConnections {
			s.stats.MaxConnections = s.stats.ConnectionCount
		}
		s.stats.Unlock()

		// Handle PROXY protocol if configured
		remoteAddr := conn.RemoteAddr().String()
		if s.config.EnableProxyProtocol {
			conn, remoteAddr = s.handleProxyProtocol(conn)
		}

		// Check if the connection is banned
		banned, reason := s.checkBans(remoteAddr)
		if banned {
			conn.Write([]byte(fmt.Sprintf("ERROR :Closing Link: %s (%s)\r\n", remoteAddr, reason)))
			conn.Close()
			continue
		}

		// Create a new client and handle the connection
		client := &Client{
			conn:     conn,
			server:   s,
			channels: make(map[string]bool),
			writer:   bufio.NewWriter(conn),
			lastPong: time.Now(),
		}

		go client.handleConnection()
	}
}
