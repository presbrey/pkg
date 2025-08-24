# Wait Library for Go

A comprehensive Go library for waiting on various conditions with configurable retry strategies, timeouts, and backoff algorithms.

## Features

- **Flexible Wait Conditions**: Wait for custom conditions, network connectivity, HTTP endpoints, files, processes, and more
- **Multiple Retry Strategies**: Fixed, linear, exponential backoff, Fibonacci, decorrelated jitter, and custom strategies
- **Context Support**: Full context.Context support for cancellation and timeouts
- **Composable**: Chain multiple conditions with `All()` and `Any()` functions
- **Network Utilities**: Comprehensive network waiting functions (TCP, UDP, HTTP, DNS)
- **Error Handling**: Detailed error reporting with timeout, retry limit, and cancellation errors

## Installation

```bash
go get github.com/yourusername/wait
```

## Quick Start

```go
package main

import (
    "log"
    "time"
    "github.com/yourusername/wait"
)

func main() {
    // Wait for network connectivity
    err := wait.ForNetwork()
    if err != nil {
        log.Fatal(err)
    }

    // Wait for a TCP service with exponential backoff
    err = wait.ForTCP("localhost:8080",
        wait.DefaultOptions().
            WithTimeout(30 * time.Second).
            WithStrategy(wait.NewExponentialBackoffStrategy(
                1*time.Second,  // initial
                2.0,            // multiplier
                10*time.Second, // max
                true,           // jitter
            )))
    if err != nil {
        log.Fatal(err)
    }
}
```

## Core Functions

### Basic Wait Functions

- `Until(condition, opts)` - Wait until condition returns true
- `UntilWithResult(condition, opts)` - Wait and return a result
- `Poll(fn, opts)` - Poll a function until it succeeds
- `All(conditions, opts)` - Wait for all conditions
- `Any(conditions, opts)` - Wait for any condition

### Network Wait Functions

- `ForNetwork()` - Wait for global network connectivity
- `ForLocalNetwork()` - Wait for local network connectivity
- `ForTCP(address)` - Wait for TCP connection
- `ForUDP(address)` - Wait for UDP connection
- `ForHTTP(url)` - Wait for HTTP 200 response
- `ForHTTPStatus(url, statuses)` - Wait for specific HTTP status
- `ForHTTPSHealthy(url)` - Wait for any 2xx response
- `ForDNS(hostname)` - Wait for DNS resolution
- `ForPort(port)` - Wait for localhost port
- `ForMultiplePorts(ports)` - Wait for multiple ports
- `ForAnyPort(ports)` - Wait for any port

### File System Wait Functions

- `ForFile(path)` - Wait for file existence
- `ForFileContent(path, minSize)` - Wait for file with content
- `ForFileRemoval(path)` - Wait for file removal
- `ForDirectory(path)` - Wait for directory

### Process Wait Functions

- `ForProcess(pid)` - Wait for process existence
- `ForProcessExit(pid)` - Wait for process to exit
- `ForCommand(name, args)` - Wait for command success

## Retry Strategies

### Fixed Strategy
Wait for a fixed duration between attempts.

```go
strategy := wait.NewFixedStrategy(2 * time.Second)
```

### Linear Strategy
Increase wait time linearly.

```go
strategy := wait.NewLinearStrategy(
    1*time.Second,        // initial
    500*time.Millisecond, // increment
    10*time.Second,       // max
)
```

### Exponential Backoff Strategy
Exponentially increase wait time with optional jitter.

```go
strategy := wait.NewExponentialBackoffStrategy(
    100*time.Millisecond, // initial
    2.0,                  // multiplier
    30*time.Second,       // max
    true,                 // add jitter
)
```

### Fibonacci Strategy
Use Fibonacci sequence for wait times.

```go
strategy := wait.NewFibonacciStrategy(
    100*time.Millisecond, // unit
    10*time.Second,       // max
)
```

### Decorrelated Jitter Strategy
AWS-style decorrelated jitter for distributed systems.

```go
strategy := wait.NewDecorrelatedJitterStrategy(
    100*time.Millisecond, // base
    10*time.Second,       // max
)
```

### Custom Strategy
Define custom wait durations.

```go
durations := []time.Duration{
    100*time.Millisecond,
    500*time.Millisecond,
    1*time.Second,
    5*time.Second,
}
strategy := wait.NewCustomStrategy(durations, false) // don't repeat
```

## Options

Configure wait behavior with options:

```go
opts := wait.DefaultOptions().
    WithMaxRetries(10).
    WithTimeout(30 * time.Second).
    WithStrategy(strategy).
    WithContext(ctx)
```

## Advanced Usage

### Wait Groups

```go
group := wait.NewGroup()

group.Add(func() (bool, error) {
    // Check database
    return isDatabaseReady(), nil
})

group.Add(func() (bool, error) {
    // Check cache
    return isCacheReady(), nil
})

err := group.Wait(wait.DefaultOptions().WithTimeout(30 * time.Second))
```

### Custom Conditions

```go
// Wait with custom condition
err := wait.Until(func() (bool, error) {
    value, err := checkSomething()
    if err != nil {
        return false, err // Return error to stop waiting
    }
    return value > threshold, nil
}, opts)

// Wait and get result
result, err := wait.UntilWithResult(func() (interface{}, bool, error) {
    data, err := fetchData()
    if err != nil {
        return nil, false, nil // Retry on error
    }
    if data.IsReady {
        return data, true, nil // Success
    }
    return nil, false, nil // Not ready, keep waiting
}, opts)
```

### Context Cancellation

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

// Cancel from another goroutine
go func() {
    <-someSignal
    cancel()
}()

err := wait.ForNetwork(
    wait.DefaultOptions().WithContext(ctx),
)

if err == wait.ErrCanceled {
    // Operation was canceled
}
```

## Error Types

- `ErrTimeout` - Timeout exceeded
- `ErrMaxRetriesReached` - Maximum retries reached
- `ErrCanceled` - Operation canceled via context

## Best Practices

1. **Always set timeouts**: Prevent infinite waiting
2. **Use appropriate strategies**: Exponential backoff for network, fixed for local resources
3. **Add jitter**: Prevent thundering herd in distributed systems
4. **Handle errors**: Check for timeout vs cancellation vs retry exhaustion
5. **Use context**: Enable graceful shutdowns and cancellation

## Examples

See `example_test.go` for comprehensive examples of all features.

## License

MIT License - see LICENSE file for details