package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/presbrey/pkg/irc/config"
	"github.com/presbrey/pkg/irc/server"
)

func main() {
	// Parse command line flags
	configPath := flag.String("config", "https://raw.githubusercontent.com/presbrey/pkg/main/irc/config/example.yaml", "Path to configuration file or HTTP URL (http://, https://)")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize the server
	srv, err := server.NewServer(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize server: %v", err)
	}

	// Start the server
	go func() {
		if err := srv.Start(); err != nil {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	fmt.Println("IRC Server started successfully")
	
	// Print listener information
	if cfg.ListenIRC.Enabled {
		fmt.Printf("  - Listening for unencrypted connections on %s\n", cfg.GetIRCListenAddress())
	}
	
	if cfg.ListenTLS.Enabled {
		fmt.Printf("  - Listening for TLS encrypted connections on %s\n", cfg.GetTLSListenAddress())
	}

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	fmt.Println("Shutting down server...")
	if err := srv.Stop(); err != nil {
		log.Fatalf("Error shutting down server: %v", err)
	}
}
