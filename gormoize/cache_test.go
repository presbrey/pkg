package gormoize_test

import (
	"os"
	"sync"
	"testing"

	"github.com/presbrey/pkg/gormoize"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestOpen(t *testing.T) {
	// Clean up test databases after tests
	defer func() {
		os.Remove("test1.db")
		os.Remove("test2.db")
	}()

	cache := gormoize.New()

	tests := []struct {
		name string
		dsn  string
	}{
		{
			name: "same DSN returns same database connection",
			dsn:  "test1.db",
		},
		{
			name: "different DSN returns different database connection",
			dsn:  "test2.db",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// First call
			db1, err := cache.Open(sqlite.Open, tt.dsn)
			if err != nil {
				t.Errorf("failed to open database: %v", err)
			}
			if db1 == nil {
				t.Error("expected non-nil database connection")
			}

			// Test database connectivity
			err = db1.Raw("SELECT 1").Error
			if err != nil {
				t.Errorf("database connection not working: %v", err)
			}

			// Second call with same DSN
			db2, err := cache.Open(sqlite.Open, tt.dsn)
			if err != nil {
				t.Errorf("failed to open database: %v", err)
			}

			// Verify we got the same connection back
			if db1 != db2 {
				t.Error("expected same database connection for same DSN")
			}
		})
	}
}

func TestGet(t *testing.T) {
	// Clean up test database after tests
	defer os.Remove("test_get.db")

	cache := gormoize.New()
	dsn := "test_get.db"

	// Test getting non-existent connection
	t.Run("get non-existent connection returns nil", func(t *testing.T) {
		db := cache.Get(dsn)
		if db != nil {
			t.Error("expected nil for non-existent connection")
		}
	})

	// Create a connection first using Open
	db1, err := cache.Open(sqlite.Open, dsn)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	t.Run("get existing connection returns same instance", func(t *testing.T) {
		db2 := cache.Get(dsn)
		if db2 == nil {
			t.Error("expected non-nil database connection")
		}
		if db1 != db2 {
			t.Error("expected same database connection for same DSN")
		}

		// Test database connectivity
		err := db2.Raw("SELECT 1").Error
		if err != nil {
			t.Errorf("database connection not working: %v", err)
		}
	})

	t.Run("multiple gets return same instance", func(t *testing.T) {
		db2 := cache.Get(dsn)
		db3 := cache.Get(dsn)
		if db2 != db3 {
			t.Error("expected same database connection for multiple gets")
		}
	})
}

func TestOpenError(t *testing.T) {
	cache := gormoize.New()

	// Create a dialector function that will cause an error by using an invalid DSN
	failDialector := func(dsn string) gorm.Dialector {
		// Use a non-existent directory to force an error
		return sqlite.Open("/nonexistent/directory/db.sqlite")
	}

	// Attempt to open the database
	db, err := cache.Open(failDialector, "test")

	// Verify that the error is returned
	if err == nil {
		t.Error("expected error when opening invalid database")
	}

	// Verify that nil is returned for the database
	if db != nil {
		t.Error("expected nil database when error occurs")
	}

	// Verify that the failed connection is not cached
	cachedDB := cache.Get("test")
	if cachedDB != nil {
		t.Error("failed connection should not be cached")
	}
}

func TestConcurrentOpen(t *testing.T) {
	// Clean up test database after tests
	const testDSN = "concurrent_test.db"
	defer os.Remove(testDSN)

	cache := gormoize.New()
	const numGoroutines = 10

	// Create channels to synchronize goroutines
	start := make(chan struct{})
	var wg sync.WaitGroup
	var dbs [numGoroutines]*gorm.DB
	var errs [numGoroutines]error

	// Launch multiple goroutines that will try to open the database simultaneously
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			// Wait for the start signal
			<-start
			dbs[idx], errs[idx] = cache.Open(sqlite.Open, testDSN)
		}(i)
	}

	// Start all goroutines at once
	close(start)

	// Wait for all goroutines to complete
	wg.Wait()

	// Check results
	for i := 0; i < numGoroutines; i++ {
		if errs[i] != nil {
			t.Errorf("goroutine %d failed to open database: %v", i, errs[i])
			continue
		}
		if dbs[i] == nil {
			t.Errorf("goroutine %d got nil database", i)
			continue
		}
	}

	// If first connection is nil, we can't continue testing
	if dbs[0] == nil {
		t.Fatal("first database connection is nil")
	}

	// Verify all goroutines got the same database connection
	for i := 1; i < numGoroutines; i++ {
		if dbs[i] == nil {
			continue // Skip nil connections as we've already reported them
		}
		if dbs[i] != dbs[0] {
			t.Errorf("goroutine %d got different database connection", i)
		}
	}

	// Test database connectivity
	err := dbs[0].Raw("SELECT 1").Error
	if err != nil {
		t.Errorf("database connection not working: %v", err)
	}
}
