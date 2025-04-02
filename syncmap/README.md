# RemoteMap

RemoteMap is a Go package that extends `sync.Map` to synchronize its values from a remote JSON URL. It provides a thread-safe way to keep a local cache of remote JSON data that is periodically refreshed.

## Features

- Extends the standard Go `sync.Map` with all its methods
- Periodically fetches and synchronizes data from a remote JSON endpoint
- Configurable refresh period, timeout, and TLS verification
- Custom HTTP headers support
- Error handling callback
- Data transformation capability
- Type-specific getters for common types (string, int, int64, float, bool, map)

## Installation

```bash
go get github.com/user/syncmap
```

## Usage

```go
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/user/syncmap"
)

func main() {
	// Create with default options
	rm := syncmap.NewRemoteMap("https://api.example.com/data", nil)
	
	// Or with custom options
	options := &syncmap.Options{
		RefreshPeriod:   30 * time.Second,
		Timeout:         10 * time.Second,
		IgnoreTLSVerify: false,
		Headers: map[string]string{
			"User-Agent": "MyApp/1.0",
		},
		ErrorHandler: func(err error) {
			log.Printf("Error refreshing map: %v", err)
		},
		TransformFunc: func(data map[string]interface{}) map[string]interface{} {
			// Transform the data if needed
			return data
		},
	}
	rm = syncmap.NewRemoteMap("https://api.example.com/data", options)

	// Start automatic refresh
	rm.Start()
	defer rm.Stop()

	// Access values using type-specific getters
	if name, ok := rm.GetString("name"); ok {
		fmt.Printf("Name: %s\n", name)
	}

	if count, ok := rm.GetInt("count"); ok {
		fmt.Printf("Count: %d\n", count)
	}

	// Or use standard sync.Map methods
	value, ok := rm.Load("key")
	
	// Iterate over all values
	rm.Range(func(key, value interface{}) bool {
		fmt.Printf("%v: %v\n", key, value)
		return true
	})
}
```

## Default Values

- Default refresh period: 5 minutes
- Default HTTP timeout: 30 seconds
- Default TLS verification: enabled (not ignored)

## License

MIT
