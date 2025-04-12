# Hook Registry Package

A flexible, type-safe hook registry system for Go applications with priority-based execution.

## Features

- Generic hook registry that works with any context type
- Priority-based hook execution (like Unix nice - lower values run first)
- Thread-safe hook registration and execution
- Panic recovery for robust execution
- Comprehensive error reporting
- Simple, clean API

## Installation

```bash
go get github.com/presbrey/pkg/hooks
```

## Usage

### Basic Usage

```go
// Create a new hook registry for your context type
registry := hooks.NewRegistry[*MyContext]()

// Register a hook with default priority (0)
registry.Register(func(ctx *MyContext) error {
    // Do something with the context
    return nil
})

// Execute all registered hooks
context := &MyContext{}
errors := registry.RunHooks(context)

// Check for errors
if errors != nil {
    // Handle errors
}
```

### Priority-Based Execution

```go
// Register hooks with different priorities
// Lower values run first (like Unix nice)

// High priority (runs first)
registry.RegisterWithPriority(func(ctx *MyContext) error {
    // This runs first
    return nil
}, -10)

// Normal priority
registry.Register(func(ctx *MyContext) error {
    // This runs second
    return nil
})

// Low priority (runs last)
registry.RegisterWithPriority(func(ctx *MyContext) error {
    // This runs last
    return nil
}, 10)
```

### Managing the Registry

```go
// Get the number of registered hooks
count := registry.Count()

// Clear all registered hooks
registry.Clear()
```

## Example Application

See the [example/main.go](./example/main.go) file for a complete usage example showing how to use the hook registry in a web application context.

## Testing

Run the tests with:

```bash
go test -v
```

Run the benchmarks with:

```bash
go test -bench=.
```

## License

MIT License