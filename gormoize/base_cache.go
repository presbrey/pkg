package gormoize

import (
	"sync"
	"time"

	"gorm.io/gorm"
)

type dbCacheEntry struct {
	db       *gorm.DB
	lastUsed time.Time
}

// baseCache provides common caching functionality with cleanup support
type baseCache struct {
	cacheMutex sync.RWMutex
	dbCache    map[string]*dbCacheEntry

	cleanupInterval time.Duration
	maxAge          time.Duration
	mockDB          *gorm.DB
	stopCleanup     chan struct{}
}

// newBaseCache creates a new baseCache instance with the given options
func newBaseCache(cleanupInterval, maxAge time.Duration) *baseCache {
	cache := &baseCache{
		cacheMutex: sync.RWMutex{},
		dbCache:    make(map[string]*dbCacheEntry),

		cleanupInterval: cleanupInterval,
		maxAge:          maxAge,
		stopCleanup:     make(chan struct{}),
	}
	if maxAge > 0 {
		go cache.startCleanup()
	}
	return cache
}

// Get returns a cached gorm.DB instance for the given key if it exists
func (c *baseCache) Get(key string) *gorm.DB {
	c.cacheMutex.RLock()
	defer c.cacheMutex.RUnlock()

	if c.mockDB != nil {
		return c.mockDB
	}

	if entry, exists := c.dbCache[key]; exists {
		entry.lastUsed = time.Now()
		return entry.db
	}
	return nil
}

// lastUsed returns a map of key to last used time for all cached items
func (c *baseCache) lastUsed() map[string]time.Time {
	lastUsed := make(map[string]time.Time)
	for key, entry := range c.dbCache {
		lastUsed[key] = entry.lastUsed
	}
	return lastUsed
}

// Remove removes the specified item from the cache and performs any necessary cleanup
func (c *baseCache) Remove(key string) {
	c.cacheMutex.Lock()
	defer c.cacheMutex.Unlock()
	
	c.cleanupItem(key)
}

// cleanupItem removes the specified item from the cache and performs any necessary cleanup
func (c *baseCache) cleanupItem(key string) {
	entry := c.dbCache[key]

	// remove the specified item from the cache
	// if key is nil or DNE, this is a no-op
	delete(c.dbCache, key)

	if entry != nil {
		// close the database connection
		sqlDB, err := entry.db.DB()
		if err == nil {
			sqlDB.Close()
		}
	}
}

// startCleanup starts the cleanup routine that removes old items
func (c *baseCache) startCleanup() {
	// Run cleanup immediately once
	c.cleanup()

	ticker := time.NewTicker(c.cleanupInterval)
	defer ticker.Stop()

	// Start cleanup loop
	for {
		select {
		case <-ticker.C:
			c.cleanup()
		case <-c.stopCleanup:
			return
		}
	}
}

// SetMockDB sets the mock DB used for testing
func (c *baseCache) SetMockDB(db *gorm.DB) {
	c.cacheMutex.Lock()
	defer c.cacheMutex.Unlock()
	c.mockDB = db
}

// Stop stops the cleanup routine and closes all database connections
func (c *baseCache) Stop() {
	c.cacheMutex.Lock()
	defer c.cacheMutex.Unlock()

	// Clean up all database connections
	for key := range c.dbCache {
		c.cleanupItem(key)
	}

	if c.maxAge > 0 {
		close(c.stopCleanup)
	}
}

// Set adds or updates the cache entry for the provided key with the given *gorm.DB instance.
func (c *baseCache) Set(key string, db *gorm.DB) {
	c.cacheMutex.Lock()
	defer c.cacheMutex.Unlock()
	// Add or update the cache entry with the current time
	c.dbCache[key] = &dbCacheEntry{
		db:       db,
		lastUsed: time.Now(),
	}
}

// cleanup removes items that haven't been used for longer than maxAge
func (c *baseCache) cleanup() {
	c.cacheMutex.Lock()
	defer c.cacheMutex.Unlock()

	now := time.Now()
	for key, lastUsed := range c.lastUsed() {
		if now.Sub(lastUsed) > c.maxAge {
			c.cleanupItem(key)
		}
	}
}
