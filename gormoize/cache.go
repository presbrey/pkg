package gormoize

import (
	"sync"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DBCache provides thread-safe caching of database connections
type DBCache struct {
	cleanupInterval time.Duration
	maxAge          time.Duration
	logMode         logger.LogLevel
	cache           *baseCache
	openMutex       sync.Mutex // Mutex to protect concurrent Open calls
}

// New creates a new DBCache instance with default settings
func New() *DBCache {
	return &DBCache{
		cleanupInterval: 5 * time.Minute,
		maxAge:          30 * time.Minute,
		logMode:         logger.Silent,
		openMutex:       sync.Mutex{},
	}
}

// WithCleanupInterval sets how often to check for stale connections
// Only used when MaxAge > 0
func (c *DBCache) WithCleanupInterval(interval time.Duration) *DBCache {
	c.cleanupInterval = interval
	return c
}

// WithMaxAge sets how long a connection can remain unused before being removed
// If 0, cleanup is disabled
func (c *DBCache) WithMaxAge(age time.Duration) *DBCache {
	c.maxAge = age
	return c
}

// WithLogMode sets the GORM logger mode
func (c *DBCache) WithLogMode(mode logger.LogLevel) *DBCache {
	c.logMode = mode
	return c
}

// WithMockDB sets a mock database for testing
func (c *DBCache) WithMockDB(db *gorm.DB) *DBCache {
	if c.cache == nil {
		c.initialize()
	}
	c.cache.SetMockDB(db)
	return c
}

// initialize creates the internal cache if not already created
func (c *DBCache) initialize() {
	if c.cache == nil {
		c.cache = newBaseCache(c.cleanupInterval, c.maxAge)
	}
}

// Get returns a cached gorm.DB instance for the given DSN if it exists
// If no instance exists for the DSN, returns nil
func (c *DBCache) Get(dsn string) *gorm.DB {
	c.initialize()
	return c.cache.Get(dsn)
}

// Open returns a gorm.DB for the given DSN
// If a DB for this DSN has already been created, the cached instance is returned
func (c *DBCache) Open(fn func(dsn string) gorm.Dialector, dsn string, additionalOpts ...gorm.Option) (*gorm.DB, error) {
	c.initialize()

	// First check if we already have this connection without locking
	if db := c.cache.Get(dsn); db != nil {
		return db, nil
	}

	// Lock to ensure only one goroutine creates a new connection
	c.openMutex.Lock()
	defer c.openMutex.Unlock()

	// Double-check if another goroutine created the connection while we were waiting
	if db := c.cache.Get(dsn); db != nil {
		return db, nil
	}

	// Create default options with silent logger to avoid spurious messages
	config := &gorm.Config{}

	// Check if any logger options are already set in additionalOpts
	hasLoggerOption := false
	for _, opt := range additionalOpts {
		if cfg, ok := opt.(*gorm.Config); ok && cfg.Logger != nil {
			// If any option is a Config with Logger set, we have a logger option
			hasLoggerOption = true
			break
		}
	}

	// Set logger based on logMode and whether logger options are already provided
	if !hasLoggerOption {
		if c.logMode == logger.Silent {
			// Set silent logger if in silent mode and no logger options provided
			config.Logger = logger.Discard
		} else if c.logMode != logger.Silent {
			// For non-silent modes, we'll let GORM use its default logger
			// This is the existing behavior
		}
	}

	opts := []gorm.Option{config}

	// Add any additional options
	opts = append(opts, additionalOpts...)

	// Create new connection
	db, err := gorm.Open(fn(dsn), opts...)
	if err != nil {
		return nil, err
	}

	// Cache the connection
	c.cache.Set(dsn, db)
	return db, nil
}

// Close removes the connection for the given DSN from the cache
func (c *DBCache) Close(dsn string) *DBCache {
	if c.cache != nil {
		c.cache.Remove(dsn)
	}
	return c
}

// CloseAll closes and removes all connections from the cache
func (c *DBCache) CloseAll() *DBCache {
	if c.cache != nil {
		c.cache.Stop()
		c.cache = nil
	}
	return c
}
