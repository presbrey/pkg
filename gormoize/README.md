# GORM Memoization SDK

A lightweight, thread-safe Go library for memoizing GORM database connections by their DSN (Data Source Name).

## Features

- **Connection Reuse**: Efficiently reuse database connections based on their DSN
- **Fluent API**: Modern builder pattern for intuitive connection management
- **Thread Safety**: Concurrent access protection with read/write mutexes
- **Flexible Creation**: Support for custom connection factory functions
- **Memory Management**: Methods to clear or selectively remove cached connections

## Installation

```bash
go get github.com/presbrey/pkg/gormoize
```

## Usage Examples

### Basic Usage

```go
import (
    "github.com/presbrey/pkg/gormoize"
    "gorm.io/driver/postgres"
    "gorm.io/gorm"
)

func main() {
    dsn := "host=localhost user=gorm password=gorm dbname=gorm port=9920 sslmode=disable"
    
    // First call - creates a new connection
    db, err := gormoize.Connection().
        WithDSN(dsn).
        WithDialector(postgres.Open(dsn)).
        WithConfig(&gorm.Config{}).
        Get()
    
    if err != nil {
        panic(err)
    }
    
    // Second call - reuses the existing connection from cache
    cachedDB, _ := gormoize.Connection().
        WithDSN(dsn).
        Get()
    
    // Both db and cachedDB point to the same connection
}
```

### Using a Custom Factory Function

```go
dsn := "file::memory:?cache=shared"

db, err := gormoize.Connection().
    WithDSN(dsn).
    WithFactory(func() (*gorm.DB, error) {
        // Custom connection logic
        return gorm.Open(sqlite.Open(dsn), &gorm.Config{
            DisableForeignKeyConstraintWhenMigrating: true,
        })
    }).
    Get()
```

### Connection Management

```go
// Get all cached connections
connections := gormoize.GetAll()

// Remove a specific connection
gormoize.Connection().
    WithDSN("your-dsn").
    Remove()

// Clear all connections
gormoize.Instance().Clear()
```

## Design Principles

1. **Singleton Cache**: The `DBCache` is implemented as a singleton to ensure a single source of truth for all database connections.

2. **Fluent Interface**: The builder pattern with method chaining creates a readable and discoverable API.

3. **Minimal Dependencies**: Only depends on GORM, no other external libraries required.

4. **Thread Safety**: All operations on the connection cache are protected by appropriate mutex locks.

5. **Error Handling**: Both error-returning and panic-on-error variants are provided to suit different coding styles.

## Advanced Usage

### Using Different Dialectors

```go
// PostgreSQL
pgDB, _ := gormmemo.Connection().
    WithDSN("postgres-dsn").
    WithDialector(postgres.Open("postgres-dsn")).
    Get()

// MySQL
mysqlDB, _ := gormmemo.Connection().
    WithDSN("mysql-dsn").
    WithDialector(mysql.Open("mysql-dsn")).
    Get()

// SQLite
sqliteDB, _ := gormmemo.Connection().
    WithDSN("sqlite-dsn").
    WithDialector(sqlite.Open("sqlite-dsn")).
    Get()
```

## License

MIT