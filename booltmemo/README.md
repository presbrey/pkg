# booltmemo

`booltmemo` is a Go package that provides function memoization for boolean-returning functions with different Time-To-Live (TTL) values for true and false results.

## Features

- Memoize any function that returns a boolean result
- Set different expiration times for true and false results
- Thread-safe implementation using sync.RWMutex
- Automatic cleanup of expired cache entries
- Manual cache invalidation methods

## Installation

```bash
go get github.com/yourusername/booltmemo
```

## Usage

Here's a simple example of how to use the package:

```go
package main

import (
    "fmt"
    "time"
    "github.com/yourusername/booltmemo"
)

// A function that might be expensive to compute
func isEven(val interface{}) bool {
    // Simulate expensive computation
    time.Sleep(100 * time.Millisecond)
    
    num, ok := val.(int)
    if !ok {
        return false
    }
    return num%2 == 0
}

func main() {
    // Create a memoizer:
    // - Cache "true" results for 1 minute
    // - Cache "false" results for 30 seconds
    memo := booltmemo.New(isEven, 1*time.Minute, 30*time.Second)
    defer memo.Stop() // Stop the cleanup timer when done
    
    // First call (computes the result)
    start := time.Now()
    result := memo.Get(42)
    fmt.Printf("First call: %v, took %v\n", result, time.Since(start))
    
    // Second call (uses cached result)
    start = time.Now()
    result = memo.Get(42)
    fmt.Printf("Second call: %v, took %v\n", result, time.Since(start))
    
    // Invalidate a specific key
    memo.Invalidate(42)
    
    // Clear the entire cache
    memo.Clear()
}
```

## Why Different TTLs?

Having different expiration times for true and false results can be useful in many scenarios:

1. **Permission checking**: Cache positive results (user has permission) longer than negative results
2. **Resource availability**: Cache "resource not found" for less time than "resource exists"
3. **Conditional logic**: Cache positive conditions longer when they're more stable than negative ones
4. **Feature flags**: Cache enabled features longer than disabled ones if they change at different rates

## API

### Creating a Memoizer

```go
// Create a new memoizer with different TTLs for true and false results
memo := booltmemo.New(yourFunction, trueTTL, falseTTL)
```

### Methods

- `Get(key interface{}) bool` - Get the result for a key (computes if not cached or expired)
- `Invalidate(key interface{})` - Remove a specific key from the cache
- `Clear()` - Remove all entries from the cache
- `Stop()` - Stop the cleanup timer (call this when done using the memoizer)

## Thread Safety

The package is safe for concurrent use. Multiple goroutines can access the memoized function simultaneously.

## License

MIT License