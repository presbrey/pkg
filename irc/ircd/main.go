package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/joho/godotenv/autoload"
	"github.com/presbrey/pkg/irc"
)

func main() {
	// Define command-line flags
	ircAddr := flag.String("irc", ":6667", "IRC server bind address")
	tlsAddr := flag.String("tls", ":6697", "TLS IRC server bind address")
	adminAddr := flag.String("admin", "127.0.0.1:8080", "Admin HTTP server bind address")
	grpcAddr := flag.String("grpc", ":6668", "gRPC peering server bind address")
	connectPeers := flag.Bool("connect-peers", false, "Connect to peers after startup")
	debug := flag.Bool("debug", false, "Enable debug logging")
	flag.Parse()

	// Log startup configuration
	log.Printf("Starting IRC server with the following configuration:")
	log.Printf("IRC bind address: %s", *ircAddr)
	log.Printf("TLS IRC bind address: %s", *tlsAddr)
	log.Printf("Admin bind address: %s", *adminAddr)
	log.Printf("gRPC bind address: %s", *grpcAddr)
	log.Printf("Connect to peers: %v", *connectPeers)
	log.Printf("Debug logging: %v", *debug)

	// Create a new IRC server with CLI flags
	server, err := irc.NewServer(*ircAddr, *tlsAddr, *adminAddr, *grpcAddr)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}
	server.Config.Debug = *debug

	// Start the server
	log.Println("Starting IRC server...")
	err = server.Start()
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
	log.Println("IRC server started successfully!")

	// Connect to peers if requested
	if *connectPeers {
		log.Println("Connecting to peer servers...")
		// Wait a moment for the server to fully initialize
		time.Sleep(2 * time.Second)

		// // In server.go or main.go
		// peeringManager := peering.NewManager(server)
		// peeringManager.Register() // Registers all the hooks
		// peeringManager.StartGRPCServer(server.config.GRPCBindAddr)
		// peeringManager.ConnectToPeers(server.config.PeerAddresses)

		err := server.ConnectToPeers()
		if err != nil {
			log.Printf("Warning: error connecting to peers: %v", err)
		}
	} else {
		log.Println("Peer connections disabled. Use --connect-peers to enable.")
	}

	// Wait for termination signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Println("Server is running. Press Ctrl+C to stop.")
	<-sigChan
	log.Println("Shutdown signal received, stopping server...")

	// Stop the server
	err = server.Stop()
	if err != nil {
		log.Printf("Error stopping server: %v", err)
	}

	log.Println("Server stopped. Goodbye!")
}
