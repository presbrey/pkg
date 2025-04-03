# Syncthing

A generic Go package for synchronizing maps with remote JSON endpoints.

## Overview

`syncthing` is a Go package that extends the standard library's `sync.Map` to synchronize with remote JSON endpoints. It uses Go's generics feature to provide type-safe access to map values of any type.

This package implements a Fluent Interface pattern, allowing for method chaining when configuring and using the `MapString`.

## Features

- Type-safe access to map values using Go generics
- Fluent Interface with method chaining for configuration
- Periodic synchronization with remote JSON endpoints
- Support for custom HTTP headers
- Separate callbacks for updates and deletions
- Proper type conversion for numeric types and nested maps
- Error handling and update notifications
- TLS configuration options

## Usage

### Basic Usage

```go

func main() {
	// Create a new MapString with string values using the Fluent Interface
	rm := syncthing.NewMapString[string]("https://api.example.com/data").
		WithRefreshPeriod(1 * time.Minute).
		WithTimeout(10 * time.Second).
		WithErrorHandler(func(err error) {
			log.Printf("Error: %v", err)
		}).
		WithUpdateCallback(func(updated []string) {
			log.Printf("Updated keys: %v", updated)
		}).
		WithDeleteCallback(func(deleted []string) {
			log.Printf("Deleted keys: %v", deleted)
		}).
		WithRefreshCallback(func() {
			log.Printf("Map refreshed at %v", time.Now())
		}).
		Start()
	
	// Don't forget to stop the synchronization when done
	defer rm.Stop()
	
	// Get a string value
	value, ok := rm.Get("key")
	if ok {
		fmt.Println("Value:", value)
	}
	
	// Get with default value
	value = rm.GetWithDefault("key", "default")
	fmt.Println("Value with default:", value)
}
```

### Working with Different Value Types

```go
// For string values
stringMap := syncthing.NewMapString[string]("https://api.example.com/data").
	WithRefreshPeriod(5 * time.Minute).
	Start()
stringValue, ok := stringMap.Get("name")

// For integer values
intMap := syncthing.NewMapString[int]("https://api.example.com/data").
	WithRefreshPeriod(5 * time.Minute).
	Start()
intValue, ok := intMap.Get("count")

// For boolean values
boolMap := syncthing.NewMapString[bool]("https://api.example.com/data").
	WithRefreshPeriod(5 * time.Minute).
	Start()
boolValue, ok := boolMap.Get("enabled")

// For any values (interface{})
anyMap := syncthing.NewMapString[any]("https://api.example.com/data").
	WithRefreshPeriod(5 * time.Minute).
	Start()
anyValue, ok := anyMap.Get("something")

// With default values
intValue := intMap.GetWithDefault("count", 0)
stringValue := stringMap.GetWithDefault("name", "Unknown")
```

### Working with Nested Maps

```go
// Get a nested map with any values
anyMap := syncthing.NewMapString[any]("https://api.example.com/data").Start()
nestedMap, ok := anyMap.GetMap("config")
if ok {
    // Access values in the nested map
    fmt.Println(nestedMap["setting"])
}

// For strongly typed nested maps, use the appropriate type parameter
stringMapMap := syncthing.NewMapString[map[string]string]("https://api.example.com/data").Start()
stringMap, ok := stringMapMap.Get("stringMap")
if ok {
    for k, v := range stringMap {
        fmt.Printf("%s: %s\n", k, v)
    }
}

// For boolean maps
boolMapMap := syncthing.NewMapString[map[string]bool]("https://api.example.com/data").Start()
boolMap, ok := boolMapMap.Get("boolMap")
if ok {
    for k, v := range boolMap {
        fmt.Printf("%s: %t\n", k, v)
    }
}

// For integer maps
intMapMap := syncthing.NewMapString[map[string]int]("https://api.example.com/data").Start()
intMap, ok := intMapMap.Get("intMap")
if ok {
    for k, v := range intMap {
        fmt.Printf("%s: %d\n", k, v)
    }
}

// For float maps
floatMapMap := syncthing.NewMapString[map[string]float64]("https://api.example.com/data").Start()
floatMap, ok := floatMapMap.Get("floatMap")
if ok {
    for k, v := range floatMap {
        fmt.Printf("%s: %f\n", k, v)
    }
}
```

### Getting All Keys

```go
keys := rm.Keys()
fmt.Println("All keys:", keys)
```

## Configuration Options

The `MapString` can be configured using the following chainable methods:

- `WithDeleteCallback(callback func([]string))`: Set a function to be called when keys are deleted
- `WithErrorHandler(handler func(error))`: Set a function to be called when an error occurs
- `WithHeader(key, value string)`: Add a single HTTP header
- `WithHeaders(headers map[string]string)`: Set multiple HTTP headers
- `WithIgnoreTLSVerify(ignore bool)`: Disable TLS certificate verification
- `WithRefreshCallback(callback func())`: Set a function to be called after each refresh operation, regardless of changes
- `WithRefreshPeriod(period time.Duration)`: Set the time between refreshes (default: 5 minutes)
- `WithTimeout(timeout time.Duration)`: Set the HTTP request timeout (default: 30 seconds)
- `WithUpdateCallback(callback func([]string))`: Set a function to be called when keys are updated

## Type Conversions

The package automatically handles type conversions for:

- Numeric types (int, int64, float64) - JSON numbers are decoded as float64 by default
- Boolean values
- String values
- Nested maps (map[string]string, map[string]bool, map[string]int, map[string]float64)

## License

[License information]
