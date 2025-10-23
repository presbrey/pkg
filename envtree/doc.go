/*
Package envtree provides utilities for loading environment variables from .env files.

It automatically searches for .env files in the current directory and all parent
directories, making it perfect for monorepos and nested project structures.

# Quick Start

The simplest way to use envtree is with AutoLoad in your init function:

	package main

	import "github.com/presbrey/envtree"

	func init() {
		envtree.AutoLoad()
	}

	func main() {
		// Your environment variables are now loaded
	}

# Loading Strategies

envtree provides several ways to load environment files:

AutoLoad - For use in init(), loads with default settings and logs errors:

	envtree.AutoLoad()

LoadDefault - Returns error for explicit handling:

	if err := envtree.LoadDefault(); err != nil {
		log.Fatal(err)
	}

MustLoadDefault - Panics on error:

	envtree.MustLoadDefault()

Custom Configuration - Fine-grained control:

	config := &envtree.Config{
		EnvFileName:      ".env.production",
		Silent:           true,
		PreferGoResolver: true,
	}
	loader := envtree.New(config)
	loader.Load()

# How It Works

The loader walks up the directory tree from the current working directory,
collecting all .env files found along the way. All files are then loaded,
with files closer to the current directory taking precedence over those
higher in the directory tree.

For example, given this directory structure:

	/
	├── .env                    # Loaded (3rd priority)
	└── projects/
	    ├── .env                # Loaded (2nd priority)
	    └── myapp/
	        ├── .env            # Loaded (1st priority)
	        └── cmd/
	            └── main.go     # Your app runs here

All three .env files will be loaded, with variables in myapp/.env
taking precedence over those in projects/.env, which in turn take
precedence over those in /.env.

# Configuration Options

The Config struct provides fine-grained control:

	type Config struct {
		// EnvFileName is the name of the env file to search for
		// Default: ".env"
		EnvFileName string

		// LogFlags sets the logging flags
		// Default: log.Lshortfile | log.LstdFlags
		LogFlags int

		// PreferGoResolver sets whether to prefer Go's built-in DNS resolver
		// Default: false (use cgo resolver)
		PreferGoResolver bool

		// Silent suppresses all log output
		// Default: false
		Silent bool

		// StopAtRoot determines whether to stop searching at filesystem root
		// Default: true
		StopAtRoot bool
	}

# Thread Safety

All functions and methods in this package are safe for concurrent use.
The underlying godotenv library handles concurrent access to environment
variables safely.

# Dependencies

This package depends on github.com/joho/godotenv for parsing .env files.
*/
package envtree
