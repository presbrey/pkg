package main

import (
	"fmt"
	"log"

	"github.com/presbrey/pkg/gormoize"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func main() {
	// Example 1: Basic usage with Postgres
	postgresConnection()

	// Example 2: Multiple connections with different DSNs
	multipleConnections()

	// Example 3: Using a custom factory function
	customFactory()

	// Example 4: Getting all connections
	allConnections()
}

func postgresConnection() {
	dsn := "host=localhost user=gorm password=gorm dbname=gorm port=9920 sslmode=disable"

	// First call - creates a new connection
	db, err := gormoize.Connection().
		WithDSN(dsn).
		WithDialector(postgres.Open(dsn)).
		WithConfig(&gorm.Config{}).
		Get()

	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	fmt.Println("Postgres connection established")

	// Second call - reuses the existing connection from cache
	cachedDB, _ := gormoize.Connection().
		WithDSN(dsn).
		WithDialector(postgres.Open(dsn)).
		WithConfig(&gorm.Config{}).
		Get()

	// Both db and cachedDB point to the same connection
	fmt.Printf("Same connection: %v\n", db == cachedDB)
}

func multipleConnections() {
	// First database
	dsn1 := "host=localhost user=gorm password=gorm dbname=db1 port=9920 sslmode=disable"
	db1, _ := gormoize.Connection().
		WithDSN(dsn1).
		WithDialector(postgres.Open(dsn1)).
		Get()

	// Second database
	dsn2 := "host=localhost user=gorm password=gorm dbname=db2 port=9920 sslmode=disable"
	db2, _ := gormoize.Connection().
		WithDSN(dsn2).
		WithDialector(postgres.Open(dsn2)).
		Get()

	fmt.Printf("Multiple connections established: %v, %v\n", db1, db2)

	// Remove one of the connections
	gormoize.Connection().
		WithDSN(dsn1).
		Remove()

	fmt.Println("Connection to db1 removed from cache")
}

func customFactory() {
	dsn := "file::memory:?cache=shared"

	// Using a custom factory function
	db, err := gormoize.Connection().
		WithDSN(dsn).
		WithFactory(func() (*gorm.DB, error) {
			// Custom connection logic
			return gorm.Open(sqlite.Open(dsn), &gorm.Config{
				DisableForeignKeyConstraintWhenMigrating: true,
			})
		}).
		Get()

	if err != nil {
		log.Fatalf("Failed to connect to SQLite database: %v", err)
	}

	fmt.Printf("SQLite in-memory connection established via custom factory: %v\n", db)

	// The MustGet variant panics on error instead of returning it
	_ = gormoize.Connection().
		WithDSN(dsn).
		MustGet()

	fmt.Println("Connection retrieved with MustGet")
}

func allConnections() {
	// Get all cached connections
	connections := gormoize.GetAll()
	fmt.Printf("Total cached connections: %d\n", len(connections))

	// Clear all connections
	gormoize.Instance().Clear()
	fmt.Println("All connections cleared from cache")
}
