package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

var (
	repoURL      = flag.String("repo", "", "Git repository URL to clone and serve")
	bearerKeys   = flag.String("keys", "", "Comma-separated list of bearer tokens for authentication")
	cloneDir     = flag.String("dir", "./repo", "Directory to clone the repository into")
	pullInterval = flag.Duration("interval", 1*time.Minute, "Interval for pulling updates from git")
)

// Server represents the git file server
type Server struct {
	repoURL      string
	cloneDir     string
	bearerKeys   map[string]bool
	pullInterval time.Duration
	mu           sync.RWMutex
	stopChan     chan struct{}
	wg           sync.WaitGroup
	cloneOrPull  func() error
}

// NewServer creates a new Server instance
func NewServer(repoURL, cloneDir string, keys []string, interval time.Duration) *Server {
	keyMap := make(map[string]bool)
	for _, key := range keys {
		if key != "" {
			keyMap[key] = true
		}
	}

	server := &Server{
		repoURL:      repoURL,
		cloneDir:     cloneDir,
		bearerKeys:   keyMap,
		pullInterval: interval,
		stopChan:     make(chan struct{}),
	}

	// Initialize the cloneOrPull function with the default implementation
	server.cloneOrPull = server.defaultCloneOrPull

	return server
}

// defaultCloneOrPull clones the repository if it doesn't exist, or pulls if it does
func (s *Server) defaultCloneOrPull() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := os.Stat(filepath.Join(s.cloneDir, ".git")); os.IsNotExist(err) {
		// Clone the repository
		log.Printf("Cloning repository %s into %s", s.repoURL, s.cloneDir)
		cmd := exec.Command("git", "clone", s.repoURL, s.cloneDir)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to clone repository: %v\nOutput: %s", err, output)
		}
		log.Printf("Successfully cloned repository")
	} else {
		// Pull latest changes
		log.Printf("Pulling latest changes from repository")
		cmd := exec.Command("git", "pull")
		cmd.Dir = s.cloneDir
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to pull repository: %v\nOutput: %s", err, output)
		}
		log.Printf("Successfully pulled latest changes: %s", strings.TrimSpace(string(output)))
	}

	return nil
}

// CloneOrPull calls the configured cloneOrPull function
func (s *Server) CloneOrPull() error {
	return s.cloneOrPull()
}

// StartAutoUpdate starts the automatic git pull routine
func (s *Server) StartAutoUpdate() {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(s.pullInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := s.CloneOrPull(); err != nil {
					log.Printf("Error updating repository: %v", err)
				}
			case <-s.stopChan:
				log.Println("Stopping auto-update routine")
				return
			}
		}
	}()
}

// Stop gracefully stops the server
func (s *Server) Stop() {
	close(s.stopChan)
	s.wg.Wait()
}

// AuthMiddleware provides bearer token authentication
func (s *Server) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// If no keys are configured, allow all requests
		if len(s.bearerKeys) == 0 {
			next.ServeHTTP(w, r)
			return
		}

		// Check for Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authorization header required", http.StatusUnauthorized)
			return
		}

		// Parse bearer token
		const bearerPrefix = "Bearer "
		if !strings.HasPrefix(authHeader, bearerPrefix) {
			http.Error(w, "Invalid authorization header format", http.StatusUnauthorized)
			return
		}

		token := strings.TrimPrefix(authHeader, bearerPrefix)

		// Validate token
		s.mu.RLock()
		valid := s.bearerKeys[token]
		s.mu.RUnlock()

		if !valid {
			http.Error(w, "Invalid bearer token", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Handler returns the HTTP handler for serving files
func (s *Server) Handler() http.Handler {
	fileServer := http.FileServer(http.Dir(s.cloneDir))
	return s.AuthMiddleware(fileServer)
}

func main() {
	flag.Parse()

	if *repoURL == "" {
		log.Fatal("Repository URL is required. Use -repo flag")
	}

	// Parse bearer keys
	var keys []string
	if *bearerKeys != "" {
		keys = strings.Split(*bearerKeys, ",")
		for i := range keys {
			keys[i] = strings.TrimSpace(keys[i])
		}
		log.Printf("Configured with %d bearer key(s)", len(keys))
	} else {
		log.Println("Warning: No bearer keys configured, server will accept all requests")
	}

	// Get port from environment or use default
	port := os.Getenv("PORT")
	if port == "" {
		port = "5000"
	}

	// Create server
	server := NewServer(*repoURL, *cloneDir, keys, *pullInterval)

	// Initial clone/pull
	if err := server.CloneOrPull(); err != nil {
		log.Fatalf("Failed to initialize repository: %v", err)
	}

	// Start auto-update routine
	server.StartAutoUpdate()

	// Setup HTTP server
	httpServer := &http.Server{
		Addr:    ":" + port,
		Handler: server.Handler(),
	}

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Shutting down server...")

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		server.Stop()

		if err := httpServer.Shutdown(ctx); err != nil {
			log.Printf("Error shutting down HTTP server: %v", err)
		}
	}()

	// Start server
	log.Printf("Starting server on port %s", port)
	log.Printf("Serving files from: %s", *cloneDir)
	log.Printf("Auto-pulling every %v", *pullInterval)

	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Failed to start server: %v", err)
	}
}
