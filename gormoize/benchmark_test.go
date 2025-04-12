package gormoize_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/presbrey/pkg/gormoize"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// BenchmarkConnectionCreation benchmarks the creation of a new connection
func BenchmarkConnectionCreation(b *testing.B) {
	// Clear the cache before running the benchmark
	gormoize.Instance().Clear()

	for i := 0; i < b.N; i++ {
		// Use a unique DSN for each iteration to avoid caching
		dsn := fmt.Sprintf("benchmark-creation-%d", i)

		_, err := gormoize.Connection().
			WithDSN(dsn).
			WithFactory(func() (*gorm.DB, error) {
				return gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
			}).
			Get()

		if err != nil {
			b.Fatalf("Failed to create connection: %v", err)
		}
	}
}

// BenchmarkConnectionRetrieval benchmarks retrieving an existing connection
func BenchmarkConnectionRetrieval(b *testing.B) {
	// Clear the cache before running the benchmark
	gormoize.Instance().Clear()

	// Create a connection first
	dsn := "benchmark-retrieval"
	_, err := gormoize.Connection().
		WithDSN(dsn).
		WithFactory(func() (*gorm.DB, error) {
			return gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
		}).
		Get()

	if err != nil {
		b.Fatalf("Failed to create initial connection: %v", err)
	}

	// Benchmark retrieving the connection
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := gormoize.Connection().
			WithDSN(dsn).
			Get()

		if err != nil {
			b.Fatalf("Failed to retrieve connection: %v", err)
		}
	}
}

// BenchmarkConcurrentRetrieval benchmarks concurrent retrieval of the same connection
func BenchmarkConcurrentRetrieval(b *testing.B) {
	// Clear the cache before running the benchmark
	gormoize.Instance().Clear()

	// Create a connection first
	dsn := "benchmark-concurrent-retrieval"
	_, err := gormoize.Connection().
		WithDSN(dsn).
		WithFactory(func() (*gorm.DB, error) {
			return gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
		}).
		Get()

	if err != nil {
		b.Fatalf("Failed to create initial connection: %v", err)
	}

	// Benchmark concurrent retrievals
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := gormoize.Connection().
				WithDSN(dsn).
				Get()

			if err != nil {
				b.Fatalf("Failed to retrieve connection: %v", err)
			}
		}
	})
}

// BenchmarkConcurrentCreation benchmarks concurrent creation of different connections
func BenchmarkConcurrentCreation(b *testing.B) {
	// Clear the cache before running the benchmark
	gormoize.Instance().Clear()

	// Create a sync.Map to track unique DSNs across goroutines
	var dsnTracker sync.Map

	// Benchmark concurrent creations
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			// Use a unique DSN for each iteration
			dsn := fmt.Sprintf("benchmark-concurrent-creation-%p-%d", &pb, i)
			i++

			// Make sure we haven't used this DSN before
			if _, loaded := dsnTracker.LoadOrStore(dsn, struct{}{}); loaded {
				b.Fatalf("DSN collision detected: %s", dsn)
			}

			_, err := gormoize.Connection().
				WithDSN(dsn).
				WithFactory(func() (*gorm.DB, error) {
					return gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
				}).
				Get()

			if err != nil {
				b.Fatalf("Failed to create connection: %v", err)
			}
		}
	})
}

// BenchmarkGetAll benchmarks retrieving all connections
func BenchmarkGetAll(b *testing.B) {
	// Clear the cache before running the benchmark
	gormoize.Instance().Clear()

	// Create multiple connections
	numConnections := 100
	for i := 0; i < numConnections; i++ {
		dsn := fmt.Sprintf("benchmark-getall-%d", i)
		_, err := gormoize.Connection().
			WithDSN(dsn).
			WithFactory(func() (*gorm.DB, error) {
				return gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
			}).
			Get()

		if err != nil {
			b.Fatalf("Failed to create connection: %v", err)
		}
	}

	// Benchmark GetAll
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		connections := gormoize.GetAll()
		if len(connections) != numConnections {
			b.Fatalf("Expected %d connections, got %d", numConnections, len(connections))
		}
	}
}

// BenchmarkClear benchmarks clearing all connections
func BenchmarkClear(b *testing.B) {
	// Prepare benchmark
	numConnectionsPerRun := 100

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		// Create multiple connections
		for j := 0; j < numConnectionsPerRun; j++ {
			dsn := fmt.Sprintf("benchmark-clear-%d-%d", i, j)
			_, err := gormoize.Connection().
				WithDSN(dsn).
				WithFactory(func() (*gorm.DB, error) {
					return gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
				}).
				Get()

			if err != nil {
				b.Fatalf("Failed to create connection: %v", err)
			}
		}
		b.StartTimer()

		// Benchmark Clear operation
		gormoize.Instance().Clear()
	}
}

// BenchmarkDualUseCase benchmarks a realistic use case with both retrievals and creations
func BenchmarkDualUseCase(b *testing.B) {
	// Clear the cache before running the benchmark
	gormoize.Instance().Clear()

	// Create some initial connections
	numInitialConnections := 10
	dsns := make([]string, numInitialConnections)

	for i := 0; i < numInitialConnections; i++ {
		dsn := fmt.Sprintf("benchmark-dual-%d", i)
		dsns[i] = dsn
		_, err := gormoize.Connection().
			WithDSN(dsn).
			WithFactory(func() (*gorm.DB, error) {
				return gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
			}).
			Get()

		if err != nil {
			b.Fatalf("Failed to create initial connection: %v", err)
		}
	}

	// Benchmark mixed usage (80% retrieval, 20% creation)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if i%5 == 0 {
			// Create a new connection (20% of the time)
			dsn := fmt.Sprintf("benchmark-dual-new-%d", i)
			_, err := gormoize.Connection().
				WithDSN(dsn).
				WithFactory(func() (*gorm.DB, error) {
					return gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
				}).
				Get()

			if err != nil {
				b.Fatalf("Failed to create connection: %v", err)
			}
		} else {
			// Retrieve an existing connection (80% of the time)
			idx := i % numInitialConnections
			_, err := gormoize.Connection().
				WithDSN(dsns[idx]).
				Get()

			if err != nil {
				b.Fatalf("Failed to retrieve connection: %v", err)
			}
		}
	}
}

// BenchmarkWithFactory benchmarks the creation with factory method
func BenchmarkWithFactory(b *testing.B) {
	// Clear the cache before running the benchmark
	gormoize.Instance().Clear()

	for i := 0; i < b.N; i++ {
		// Use a unique DSN for each iteration to avoid caching
		dsn := fmt.Sprintf("benchmark-factory-%d", i)

		_, err := gormoize.Connection().
			WithDSN(dsn).
			WithFactory(func() (*gorm.DB, error) {
				return gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
			}).
			Get()

		if err != nil {
			b.Fatalf("Failed to create connection: %v", err)
		}
	}
}

// BenchmarkWithDialector benchmarks the creation with dialector
func BenchmarkWithDialector(b *testing.B) {
	// Clear the cache before running the benchmark
	gormoize.Instance().Clear()

	for i := 0; i < b.N; i++ {
		// Use a unique DSN for each iteration to avoid caching
		dsn := fmt.Sprintf("benchmark-dialector-%d", i)

		_, err := gormoize.Connection().
			WithDSN(dsn).
			WithDialector(sqlite.Open("file::memory:")).
			WithConfig(&gorm.Config{}).
			Get()

		if err != nil {
			b.Fatalf("Failed to create connection: %v", err)
		}
	}
}
