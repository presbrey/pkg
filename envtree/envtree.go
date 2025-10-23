// Package envtree provides utilities for loading environment variables from .env files.
// It automatically searches for .env files in the current directory and all parent directories.
package envtree

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

// Config holds the configuration for the environment loader
type Config struct {
	// EnvFileName is the name of the env file to search for (default: ".env")
	EnvFileName string
}

// DefaultConfig returns a Config with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		EnvFileName: ".env",
	}
}

// Loader handles environment file loading
type Loader struct {
	config *Config
}

// New creates a new Loader with the given configuration
func New(config *Config) *Loader {
	if config == nil {
		config = DefaultConfig()
	}
	return &Loader{config: config}
}

// Load searches for environment files and loads them
func (l *Loader) Load() error {
	// Get environment file paths
	envFiles, err := l.getEnvFilePaths()
	if err != nil {
		return fmt.Errorf("failed to get env file paths: %w", err)
	}

	// Load environment files if any were found
	if len(envFiles) > 0 {
		if err := godotenv.Load(envFiles...); err != nil {
			return fmt.Errorf("failed to load env files: %w", err)
		}
	}

	return nil
}

// MustLoad loads environment files and panics on error
func (l *Loader) MustLoad() {
	if err := l.Load(); err != nil {
		panic(err)
	}
}

// getEnvFilePaths searches for .env files from the current directory up to the root
func (l *Loader) getEnvFilePaths() ([]string, error) {
	var envFiles []string

	// Get the current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get working directory: %w", err)
	}

	// Start from the current directory and move up
	for {
		// Construct the path to the env file in the current directory
		envPath := filepath.Join(cwd, l.config.EnvFileName)

		// Check if the file exists
		if _, err := os.Stat(envPath); err == nil {
			// If it exists, add it to the list
			envFiles = append(envFiles, envPath)
		}

		// Move to the parent directory
		parent := filepath.Dir(cwd)

		// If we've reached the root (parent is the same as current), break the loop
		if parent == cwd {
			break
		}

		// Update current working directory to the parent
		cwd = parent
	}

	return envFiles, nil
}

// GetEnvFilePaths returns all environment file paths without loading them
func (l *Loader) GetEnvFilePaths() ([]string, error) {
	return l.getEnvFilePaths()
}

// LoadDefault loads environment files using default configuration
func LoadDefault() error {
	loader := New(nil)
	return loader.Load()
}

// MustLoadDefault loads environment files using default configuration and panics on error
func MustLoadDefault() {
	loader := New(nil)
	loader.MustLoad()
}

// AutoLoad is a convenience function for use in init() functions
// It loads environment files with default settings and logs any errors
func AutoLoad() {
	if err := LoadDefault(); err != nil {
		log.Printf("Warning: failed to auto-load environment files: %v", err)
	}
}
