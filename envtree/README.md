# envtree

A flexible Go library for automatically loading environment variables from `.env` files. It searches for `.env` files in the current directory and all parent directories, making it perfect for monorepos and nested project structures.

## Features

- 🔍 Automatically searches for `.env` files up the directory tree
- 🎛️ Configurable behavior (file names, logging, DNS resolver)
- 🔧 Simple API with sensible defaults
- 📦 Zero-config option for quick setup
- 🚀 Thread-safe and production-ready
- 🧪 Well-tested and documented

## Installation

```bash
go get github.com/presbrey/envtree
```

## Quick Start

### Option 1: Auto-load in init() (Simplest)

```go
package main

import (
    "github.com/presbrey/envtree"
)

func init() {
    envtree.AutoLoad()
}

func main() {
    // Your environment variables are now loaded
}
```

### Option 2: Load with Default Settings

```go
package main

import (
    "log"
    "github.com/presbrey/envtree"
)

func main() {
    if err := envtree.LoadDefault(); err != nil {
        log.Fatalf("Failed to load env: %v", err)
    }
    
    // Your environment variables are now loaded
}
```

### Option 3: Load with Custom Configuration

```go
package main

import (
    "log"
    "github.com/presbrey/envtree"
)

func main() {
    config := &envtree.Config{
        EnvFileName:      ".env.local",  // Custom file name
        Silent:           false,          // Enable logging
        PreferGoResolver: true,           // Use Go's DNS resolver
    }
    
    loader := envtree.New(config)
    if err := loader.Load(); err != nil {
        log.Fatalf("Failed to load env: %v", err)
    }
}
```

### Option 4: Must Load (Panics on Error)

```go
package main

import (
    "github.com/presbrey/envtree"
)

func main() {
    // This will panic if loading fails
    envtree.MustLoadDefault()
    
    // Your environment variables are now loaded
}
```

## Configuration Options

The `Config` struct provides fine-grained control over the loading behavior:

```go
type Config struct {
    // EnvFileName is the name of the env file to search for
    // Default: ".env"
    EnvFileName string
    
    // LogFlags sets the logging flags
    // Default: log.Lshortfile | log.LstdFlags
    LogFlags int
    
    // PreferGoResolver sets whether to prefer Go's built-in DNS resolver
    // If false, uses cgo resolver
    // Default: false
    PreferGoResolver bool
    
    // Silent suppresses all log output
    // Default: false
    Silent bool
    
    // StopAtRoot determines whether to stop searching at the filesystem root
    // Default: true
    StopAtRoot bool
}
```

## Usage Examples

### Loading Multiple Environment Files

The loader automatically finds all `.env` files from your current directory up to the root:

```
/
├── .env                    # Found (3rd)
└── projects/
    ├── .env                # Found (2nd)
    └── myapp/
        ├── .env            # Found (1st)
        └── cmd/
            └── main.go     # Your app runs here
```

All three `.env` files will be loaded, with files closer to your app taking precedence.

### Silent Mode

```go
config := &envtree.Config{
    Silent: true,  // No log output
}

loader := envtree.New(config)
loader.Load()
```

### Custom Environment File Name

```go
config := &envtree.Config{
    EnvFileName: ".env.production",
}

loader := envtree.New(config)
loader.Load()
```

### Getting File Paths Without Loading

```go
loader := envtree.New(nil)
paths, err := loader.GetEnvFilePaths()
if err != nil {
    log.Fatal(err)
}

for _, path := range paths {
    fmt.Println("Found .env file:", path)
}
```

### Integration with Existing Code

Replace your existing init function:

**Before:**
```go
func init() {
    log.SetFlags(log.Lshortfile | log.LstdFlags)
    net.DefaultResolver.PreferGo = false
    
    envFiles, err := getEnvFilePaths()
    if err != nil {
        panic(err)
    }
    
    if len(envFiles) > 0 {
        err = godotenv.Load(envFiles...)
        if err != nil {
            panic(err)
        }
    }
}
```

**After:**
```go
func init() {
    envtree.MustLoadDefault()
}
```

## How It Works

1. **Discovery**: Starting from your current working directory, the loader searches for `.env` files
2. **Traversal**: It walks up the directory tree, checking each parent directory
3. **Collection**: All found `.env` files are collected in order (closest to farthest)
4. **Loading**: All files are loaded using `godotenv`, with closer files taking precedence

## Best Practices

1. **Use AutoLoad() in init()** for simple applications
2. **Use LoadDefault()** when you need error handling
3. **Use MustLoadDefault()** when you want to fail fast on missing config
4. **Use custom Config** when you need fine-grained control
5. **Keep sensitive data** in `.env` files and add them to `.gitignore`

## Testing

When testing, you may want to disable environment loading:

```go
func TestSomething(t *testing.T) {
    // Option 1: Use Silent mode
    config := &envtree.Config{Silent: true}
    loader := envtree.New(config)
    loader.Load()
    
    // Option 2: Don't load at all
    // Just skip calling the loader in tests
}
```

## Comparison with Direct godotenv Usage

**Direct godotenv:**
```go
// Only loads from current directory
godotenv.Load()
```

**envtree:**
```go
// Finds and loads from current and all parent directories
envtree.LoadDefault()
```

## Dependencies

- [godotenv](https://github.com/joho/godotenv) - For parsing `.env` files

## License

MIT License - see LICENSE file for details

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## Related Projects

- [godotenv](https://github.com/joho/godotenv) - Go port of Ruby's dotenv library
- [viper](https://github.com/spf13/viper) - Complete configuration solution for Go applications