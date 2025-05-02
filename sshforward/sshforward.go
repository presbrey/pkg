package sshforward

import (
	"fmt"
	"io"
	"net"
	"os"

	"golang.org/x/crypto/ssh"
)

// Config holds the SSH connection configuration
type Config struct {
	User           string
	Password       string
	PrivateKeyPath string
	PrivateKey     string // New field for inline private key
	Server         string
	Port           int
	LocalPort      int
	RemotePort     int
}

// Forward establishes an SSH connection and sets up local port forwarding
func Forward(config *Config) error {
	// Create SSH client configuration
	sshConfig := &ssh.ClientConfig{
		User:            config.User,
		Auth:            make([]ssh.AuthMethod, 0),
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // Note: In production, use proper host key verification
	}

	// Add authentication methods
	if config.Password != "" {
		sshConfig.Auth = append(sshConfig.Auth, ssh.Password(config.Password))
	}

	// Try inline private key first
	if config.PrivateKey != "" {
		signer, err := ssh.ParsePrivateKey([]byte(config.PrivateKey))
		if err != nil {
			return fmt.Errorf("failed to parse inline private key: %v", err)
		}
		sshConfig.Auth = append(sshConfig.Auth, ssh.PublicKeys(signer))
	} else if config.PrivateKeyPath != "" {
		// Fall back to private key file if no inline key is provided
		key, err := loadPrivateKey(config.PrivateKeyPath)
		if err != nil {
			return fmt.Errorf("failed to load private key file: %v", err)
		}
		sshConfig.Auth = append(sshConfig.Auth, ssh.PublicKeys(key))
	}

	// Connect to SSH server
	serverAddr := fmt.Sprintf("%s:%d", config.Server, config.Port)
	client, err := ssh.Dial("tcp", serverAddr, sshConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to SSH server: %v", err)
	}

	// Start local listener
	listener, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", config.LocalPort))
	if err != nil {
		return fmt.Errorf("failed to start local listener: %v", err)
	}
	if config.LocalPort == 0 {
		if _, ok := listener.Addr().(*net.TCPAddr); ok {
			config.LocalPort = listener.Addr().(*net.TCPAddr).Port
		}
	}

	// Handle incoming connections
	go func() {
		for {
			local, err := listener.Accept()
			if err != nil {
				fmt.Printf("Failed to accept connection: %v\n", err)
				continue
			}

			go handleConnection(client, local, config.RemotePort)
		}
	}()

	return nil
}

// handleConnection manages the forwarding for a single connection
func handleConnection(client *ssh.Client, local net.Conn, remotePort int) {
	// Connect to remote server through SSH tunnel
	remote, err := client.Dial("tcp", fmt.Sprintf("localhost:%d", remotePort))
	if err != nil {
		fmt.Printf("Failed to connect to remote port: %v\n", err)
		local.Close()
		return
	}

	// Copy data bidirectionally
	go copyConn(local, remote)
	go copyConn(remote, local)
}

// copyConn copies data between connections
func copyConn(dst, src net.Conn) {
	defer dst.Close()
	defer src.Close()
	io.Copy(dst, src)
}

// loadPrivateKey reads and parses a private key file
func loadPrivateKey(path string) (ssh.Signer, error) {
	key, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key: %v", err)
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %v", err)
	}

	return signer, nil
}
