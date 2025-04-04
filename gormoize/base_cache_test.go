package gormoize

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestBaseCache(t *testing.T) {
	t.Run("cleanup removes old items", func(t *testing.T) {
		// Create base cache with short intervals for testing
		cache := newBaseCache(100*time.Millisecond, 2000*time.Millisecond)
		defer cache.Stop() // Ensure cleanup goroutine is stopped

		// Add test items
		freshDB, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
		staleDB, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})

		cache.cacheMutex.Lock()
		cache.dbCache["fresh"] = &dbCacheEntry{
			db:       freshDB,
			lastUsed: time.Now(),
		}
		cache.dbCache["stale"] = &dbCacheEntry{
			db:       staleDB,
			lastUsed: time.Now().Add(-10 * time.Second),
		}
		cache.cacheMutex.Unlock()

		// No need to wait as long since cleanup runs immediately now
		time.Sleep(500 * time.Millisecond) // Small wait to ensure cleanup completes

		cache.cacheMutex.RLock()
		_, freshExists := cache.dbCache["fresh"]
		_, staleExists := cache.dbCache["stale"]
		cache.cacheMutex.RUnlock()

		assert.True(t, freshExists, "fresh item should not be cleaned up")
		assert.False(t, staleExists, "stale item should be cleaned up")
	})
}

func TestDBCache(t *testing.T) {
	t.Run("SetMockDB sets mock database", func(t *testing.T) {
		cache := New().WithCleanupInterval(time.Hour).WithMaxAge(time.Hour)
		mockDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
		if err != nil {
			t.Fatalf("failed to create mock DB: %v", err)
		}

		cache.WithMockDB(mockDB)
		db := cache.Get("any")
		assert.Equal(t, mockDB, db, "mockDB was not set correctly")

		// Open should return the mock DB
		mockDBFromOpen, err := cache.Open(sqlite.Open, "any")
		assert.NoError(t, err)
		assert.Equal(t, mockDB, mockDBFromOpen)
	})

	t.Run("Open adds entry to cache", func(t *testing.T) {
		cache := New().WithCleanupInterval(time.Hour).WithMaxAge(time.Hour)
		
		// Open a new connection
		db, err := cache.Open(sqlite.Open, ":memory:")
		assert.NoError(t, err)
		assert.NotNil(t, db)
		
		// Get should return the same connection
		cachedDB := cache.Get(":memory:")
		assert.Equal(t, db, cachedDB)
		
		// Close the connection
		cache.Close(":memory:")
		
		// Get should now return nil
		nilDB := cache.Get(":memory:")
		assert.Nil(t, nilDB)
	})

	t.Run("CloseAll closes all connections", func(t *testing.T) {
		cache := New().WithCleanupInterval(time.Hour).WithMaxAge(time.Hour)
		
		// Open a couple of connections
		db1, err := cache.Open(sqlite.Open, ":memory:1")
		assert.NoError(t, err)
		assert.NotNil(t, db1)
		
		db2, err := cache.Open(sqlite.Open, ":memory:2")
		assert.NoError(t, err)
		assert.NotNil(t, db2)
		
		// CloseAll should close all connections
		cache.CloseAll()
		
		// Get should now return nil for both
		nilDB1 := cache.Get(":memory:1")
		assert.Nil(t, nilDB1)
		
		nilDB2 := cache.Get(":memory:2")
		assert.Nil(t, nilDB2)
	})
}
