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

// Execute all registered hooks (all phases)
context := &MyContext{}
errors := registry.RunAll(context)

// Check for errors
if errors != nil {
    // Handle errors
}
```

### Priority-Based Execution and Phases

Hooks are divided into three phases based on priority:
- **Early**: priority < 0 (`RunEarly`)
- **Middle**: priority == 0 (`RunMiddle`)
- **Late**: priority > 0 (`RunLate`)

You can run hooks by phase or all together:

```go
// Register hooks with different priorities
// Lower values run first (like Unix nice)

// High priority (runs first, Early phase)
registry.RegisterWithPriority(func(ctx *MyContext) error {
    // This runs first
    return nil
}, -10)

// Normal priority (Middle phase)
registry.Register(func(ctx *MyContext) error {
    // This runs in the middle
    return nil
})

// Low priority (runs last, Late phase)
registry.RegisterWithPriority(func(ctx *MyContext) error {
    // This runs last
    return nil
}, 10)

// Run only Early hooks
errsEarly := registry.RunEarly(context)

// Run only Middle hooks
errsMiddle := registry.RunMiddle(context)

// Run only Late hooks
errsLate := registry.RunLate(context)

// Run all hooks in order (Early, Middle, Late)
errsAll := registry.RunAll(context)

### Advanced Priority Execution

In addition to the standard phases, you can run hooks based on more specific priority criteria:

```go
// Run hooks with priority within a specific range (inclusive)
// For example, run hooks with priority between -5 and 5
errsRange := registry.RunPriorityRange(context, -5, 5)

// Run hooks with priority strictly less than a value
// For example, run hooks with priority less than 0 (equivalent to RunEarly)
errsLessThan := registry.RunPriorityLessThan(context, 0)

// Run hooks with priority strictly greater than a value
// For example, run hooks with priority greater than 0 (equivalent to RunLate)
errsGreaterThan := registry.RunPriorityGreaterThan(context, 0)

// Run hooks with a specific priority level
// For example, run hooks with priority exactly 0 (equivalent to RunMiddle)
errsLevel := registry.RunLevel(context, 0)
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