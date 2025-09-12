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
	gitToken     = flag.String("token", "", "Personal access token for private repository authentication")
	sshKeyPath   = flag.String("ssh-key", "", "Path to SSH private key for SSH-based authentication")
	cloneDir     = flag.String("dir", "./repo", "Directory to clone the repository into")
	pullInterval = flag.Duration("interval", 1*time.Minute, "Interval for pulling updates from git")
)

// Server represents the git file server
type Server struct {
	repoURL      string
	cloneDir     string
	bearerKeys   map[string]bool
	gitToken     string
	sshKeyPath   string
	pullInterval time.Duration
	mu           sync.RWMutex
	stopChan     chan struct{}
	wg           sync.WaitGroup
	cloneOrPull  func() error
}

// NewServer creates a new Server instance
func NewServer(repoURL, cloneDir string, keys []string, gitToken, sshKeyPath string, interval time.Duration) *Server {
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
		gitToken:     gitToken,
		sshKeyPath:   sshKeyPath,
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

	// Determine the authentication method based on repo URL and available credentials
	var cloneURL string
	var env []string

	// Check if it's an HTTPS URL and we have a token
	if strings.HasPrefix(s.repoURL, "https://") && s.gitToken != "" {
		// Modify the URL to include the token for authentication
		// Example: https://github.com/user/repo.git -> https://TOKEN@github.com/user/repo.git
		cloneURL = strings.Replace(s.repoURL, "https://", "https://"+s.gitToken+"@", 1)
		log.Printf("Using token authentication for HTTPS repository")
	} else if strings.HasPrefix(s.repoURL, "git@") && s.sshKeyPath != "" {
		// For SSH URLs, use the SSH key path
		cloneURL = s.repoURL
		env = append(os.Environ(), fmt.Sprintf("GIT_SSH_COMMAND=ssh -i %s -o StrictHostKeyChecking=no", s.sshKeyPath))
		log.Printf("Using SSH key authentication for SSH repository")
	} else {
		// No authentication or not needed
		cloneURL = s.repoURL
		env = os.Environ()
	}

	if _, err := os.Stat(filepath.Join(s.cloneDir, ".git")); os.IsNotExist(err) {
		// Clone the repository
		log.Printf("Cloning repository %s into %s", s.repoURL, s.cloneDir)
		cmd := exec.Command("git", "clone", cloneURL, s.cloneDir)
		cmd.Env = env
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
		cmd.Env = env
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

	// Get git token from environment variable if not provided via flag
	gitToken := *gitToken
	if gitToken == "" {
		gitToken = os.Getenv("GIT_TOKEN")
		if gitToken != "" {
			log.Println("Using git token from environment variable")
		}
	}

	// Get SSH key path from environment variable if not provided via flag
	sshKeyPath := *sshKeyPath
	if sshKeyPath == "" {
		sshKeyPath = os.Getenv("GIT_SSH_KEY")
		if sshKeyPath != "" {
			log.Println("Using SSH key path from environment variable")
		}
	}

	// Get port from environment or use default
	port := os.Getenv("PORT")
	if port == "" {
		port = "5000"
	}

	// Create server
	server := NewServer(*repoURL, *cloneDir, keys, gitToken, sshKeyPath, *pullInterval)

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
