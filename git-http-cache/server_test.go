package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewServer(t *testing.T) {
	keys := []string{"key1", "key2", "key3"}
	server := NewServer("https://github.com/test/repo.git", "./test-repo", keys, 1*time.Minute)

	if server.repoURL != "https://github.com/test/repo.git" {
		t.Errorf("Expected repoURL to be 'https://github.com/test/repo.git', got %s", server.repoURL)
	}

	if server.cloneDir != "./test-repo" {
		t.Errorf("Expected cloneDir to be './test-repo', got %s", server.cloneDir)
	}

	if len(server.bearerKeys) != 3 {
		t.Errorf("Expected 3 bearer keys, got %d", len(server.bearerKeys))
	}

	for _, key := range keys {
		if !server.bearerKeys[key] {
			t.Errorf("Expected key '%s' to be in bearerKeys", key)
		}
	}
}

func TestAuthMiddleware_NoKeys(t *testing.T) {
	// Test with no authentication keys (should allow all requests)
	server := NewServer("https://github.com/test/repo.git", "./test-repo", []string{}, 1*time.Minute)

	handler := server.AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if body != "success" {
		t.Errorf("Expected body 'success', got %s", body)
	}
}

func TestAuthMiddleware_WithKeys(t *testing.T) {
	server := NewServer("https://github.com/test/repo.git", "./test-repo", []string{"valid-key-1", "valid-key-2"}, 1*time.Minute)

	handler := server.AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("authenticated"))
	}))

	tests := []struct {
		name           string
		authHeader     string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "No auth header",
			authHeader:     "",
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "Authorization header required",
		},
		{
			name:           "Invalid format",
			authHeader:     "Basic dXNlcjpwYXNz",
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "Invalid authorization header format",
		},
		{
			name:           "Invalid token",
			authHeader:     "Bearer invalid-token",
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "Invalid bearer token",
		},
		{
			name:           "Valid token 1",
			authHeader:     "Bearer valid-key-1",
			expectedStatus: http.StatusOK,
			expectedBody:   "authenticated",
		},
		{
			name:           "Valid token 2",
			authHeader:     "Bearer valid-key-2",
			expectedStatus: http.StatusOK,
			expectedBody:   "authenticated",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			body := strings.TrimSpace(w.Body.String())
			if body != tt.expectedBody {
				t.Errorf("Expected body '%s', got '%s'", tt.expectedBody, body)
			}
		})
	}
}

func TestFileServing(t *testing.T) {
	// Create a temporary directory with test files
	tempDir := t.TempDir()

	// Create test files
	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create subdirectory with file
	subDir := filepath.Join(tempDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	subFile := filepath.Join(subDir, "sub.txt")
	if err := os.WriteFile(subFile, []byte("sub content"), 0644); err != nil {
		t.Fatalf("Failed to create sub file: %v", err)
	}

	// Create server without authentication for simplicity
	server := NewServer("https://github.com/test/repo.git", tempDir, []string{}, 1*time.Minute)
	handler := server.Handler()

	tests := []struct {
		name           string
		path           string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "Root directory",
			path:           "/",
			expectedStatus: http.StatusOK,
			expectedBody:   "", // Directory listing
		},
		{
			name:           "Existing file",
			path:           "/test.txt",
			expectedStatus: http.StatusOK,
			expectedBody:   "test content",
		},
		{
			name:           "File in subdirectory",
			path:           "/subdir/sub.txt",
			expectedStatus: http.StatusOK,
			expectedBody:   "sub content",
		},
		{
			name:           "Non-existent file",
			path:           "/notfound.txt",
			expectedStatus: http.StatusNotFound,
			expectedBody:   "404",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectedBody != "" {
				body := w.Body.String()
				if !strings.Contains(body, tt.expectedBody) {
					t.Errorf("Expected body to contain '%s', got '%s'", tt.expectedBody, body)
				}
			}
		})
	}
}

func TestAutoUpdate(t *testing.T) {
	// Create a temporary directory
	tempDir := t.TempDir()

	// Initialize git repo for testing
	gitDir := filepath.Join(tempDir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatalf("Failed to create .git directory: %v", err)
	}

	// Create server with short update interval for testing
	server := NewServer("https://github.com/test/repo.git", tempDir, []string{}, 100*time.Millisecond)

	// Track pull attempts
	pullCount := 0
	originalCloneOrPull := server.cloneOrPull
	server.cloneOrPull = func() error {
		pullCount++
		return originalCloneOrPull()
	}

	// Start auto-update
	server.StartAutoUpdate()

	// Wait for at least 2 updates
	time.Sleep(250 * time.Millisecond)

	// Stop the server
	server.Stop()

	// Check that pulls happened
	if pullCount < 2 {
		t.Errorf("Expected at least 2 pull attempts, got %d", pullCount)
	}
}

func TestGracefulShutdown(t *testing.T) {
	tempDir := t.TempDir()
	server := NewServer("https://github.com/test/repo.git", tempDir, []string{}, 1*time.Second)

	// Start auto-update
	server.StartAutoUpdate()

	// Give it a moment to start
	time.Sleep(50 * time.Millisecond)

	// Stop the server
	done := make(chan bool)
	go func() {
		server.Stop()
		done <- true
	}()

	// Wait for stop to complete (with timeout)
	select {
	case <-done:
		// Success - server stopped
	case <-time.After(2 * time.Second):
		t.Error("Server stop timed out")
	}
}

func TestIntegrationWithAuth(t *testing.T) {
	// Create temporary directory with test content
	tempDir := t.TempDir()

	indexFile := filepath.Join(tempDir, "index.html")
	if err := os.WriteFile(indexFile, []byte("<h1>Hello World</h1>"), 0644); err != nil {
		t.Fatalf("Failed to create index file: %v", err)
	}

	// Create server with authentication
	server := NewServer("https://github.com/test/repo.git", tempDir, []string{"secret-key"}, 1*time.Minute)

	// Create test server
	ts := httptest.NewServer(server.Handler())
	defer ts.Close()

	// Test without auth - should fail
	resp, err := http.Get(ts.URL + "/index.html")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected status 401 without auth, got %d", resp.StatusCode)
	}

	// Test with auth - should succeed
	req, err := http.NewRequest("GET", ts.URL+"/index.html", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer secret-key")

	client := &http.Client{}
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("Failed to make authenticated request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 with auth, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	if string(body) != "<h1>Hello World</h1>" {
		t.Errorf("Expected body '<h1>Hello World</h1>', got '%s'", body)
	}
}

// Benchmark for authentication middleware
func BenchmarkAuthMiddleware(b *testing.B) {
	server := NewServer("https://github.com/test/repo.git", "./test", []string{"key1", "key2"}, 1*time.Minute)

	handler := server.AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer key1")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}
}
