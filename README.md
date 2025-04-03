# Go Remote Map Synchronization Libraries

This repository contains two Go packages for synchronizing local maps with remote JSON endpoints:

## Packages

### [syncmap](./syncmap)

`syncmap` is a package that extends `sync.Map` to synchronize its values from a remote JSON URL. It provides a thread-safe way to keep a local cache of remote JSON data that is periodically refreshed.

#### Key Features

- Extends the standard Go `sync.Map` with all its methods
- Periodically fetches and synchronizes data from a remote JSON endpoint
- Configurable refresh period, timeout, and TLS verification
- Custom HTTP headers support
- Error handling callback
- Data transformation capability
- Type-specific getters for common types (string, int, int64, float, bool, map)
- Keys() method to retrieve all keys as a string slice

### [syncthing](./syncthing)

`syncthing` is a more advanced package that builds on the concepts of `syncmap` but uses Go's generics feature to provide type-safe access to map values of any type. It implements a Fluent Interface pattern for a more elegant API.

#### Key Features

- Type-safe access to map values using Go generics
- Fluent Interface with method chaining for configuration
- Periodic synchronization with remote JSON endpoints
- Support for custom HTTP headers
- Separate callbacks for updates and deletions
- Proper type conversion for numeric types and nested maps
- Error handling and update notifications
- TLS configuration options

## Comparison

| Feature | syncmap | syncthing |
|---------|---------|-----------|
| API Style | Traditional | Fluent Interface |
| Type Safety | Runtime type checking | Compile-time generics |
| Callbacks | Single error handler | Multiple (error, update, delete, refresh) |
| Change Tracking | No | Yes (added/changed/deleted keys) |
| Nested Maps | Basic support | Enhanced support with type conversion |

## Installation

```bash
# Install syncmap
go get github.com/presbrey/pkg/syncmap

# Install syncthing
go get github.com/presbrey/pkg/syncthing
```

## Quick Start

### syncmap Example

```go
import (
    "fmt"
    "time"
    "github.com/presbrey/pkg/syncmap"
)

func main() {
    rm := syncmap.NewRemoteMap("https://api.example.com/data", &syncmap.Options{
        RefreshPeriod: 30 * time.Second,
    })
    
    rm.Start()
    defer rm.Stop()
    
    if name, ok := rm.GetString("name"); ok {
        fmt.Printf("Name: %s\n", name)
    }
    
    // Get all keys
    keys := rm.Keys()
    fmt.Println("All keys:", keys)
}
```

### syncthing Example

```go
import (
    "fmt"
    "time"
    "github.com/presbrey/pkg/syncthing"
)

func main() {
    rm := syncthing.NewMapString[string]("https://api.example.com/data").
        WithRefreshPeriod(30 * time.Second).
        WithUpdateCallback(func(updated []string) {
            fmt.Printf("Updated keys: %v\n", updated)
        }).
        Start()
    
    defer rm.Stop()
    
    if value, ok := rm.Get("name"); ok {
        fmt.Printf("Name: %s\n", value)
    }
    
    // Get all keys
    keys := rm.Keys()
    fmt.Println("All keys:", keys)
}
```

## When to Use Which Package

- Use **syncmap** when:
  - You need a simpler API that extends the familiar `sync.Map`
  - You don't need compile-time type safety
  - You're working with Go versions prior to Go 1.18 (which introduced generics)

- Use **syncthing** when:
  - You prefer a fluent, chainable API
  - You want compile-time type safety with generics
  - You need detailed change tracking (added/changed/deleted keys)
  - You're working with complex nested data structures
  - You're using Go 1.18 or later

## License

[License information]
