package gormoize

import (
	"sync"

	"gorm.io/gorm"
)

// dbCache is a singleton instance that caches DB connections by DSN
var (
	instance *DBCache
	once     sync.Once
)

// DBCache provides thread-safe caching of database connections
type DBCache struct {
	connections map[string]*gorm.DB
	mutex       sync.RWMutex
}

// Instance returns the singleton instance of DBCache
func Instance() *DBCache {
	once.Do(func() {
		instance = &DBCache{
			connections: make(map[string]*gorm.DB),
		}
	})
	return instance
}

// Clear removes all cached connections
func (c *DBCache) Clear() *DBCache {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.connections = make(map[string]*gorm.DB)
	return c
}

// Connection starts a fluent chain for getting or creating a DB connection
func Connection() *ConnectionBuilder {
	return &ConnectionBuilder{
		cache: Instance(),
	}
}

// ConnectionBuilder implements the fluent pattern for obtaining DB connections
type ConnectionBuilder struct {
	cache     *DBCache
	dsn       string
	dialector gorm.Dialector
	config    *gorm.Config
	factory   func() (*gorm.DB, error)
	mockDB    *gorm.DB
}

// WithDSN sets the DSN for the connection
func (b *ConnectionBuilder) WithDSN(dsn string) *ConnectionBuilder {
	b.dsn = dsn
	return b
}

// WithDialector sets the GORM dialector for the connection
func (b *ConnectionBuilder) WithDialector(dialector gorm.Dialector) *ConnectionBuilder {
	b.dialector = dialector
	return b
}

// WithConfig sets the GORM config for the connection
func (b *ConnectionBuilder) WithConfig(config *gorm.Config) *ConnectionBuilder {
	b.config = config
	return b
}

// WithFactory sets a custom factory function for creating the connection
func (b *ConnectionBuilder) WithFactory(factory func() (*gorm.DB, error)) *ConnectionBuilder {
	b.factory = factory
	return b
}

// WithMockDB sets a specific *gorm.DB instance to be returned by Get/MustGet.
// This bypasses caching and creation logic, useful for testing.
func (b *ConnectionBuilder) WithMockDB(db *gorm.DB) *ConnectionBuilder {
	b.mockDB = db
	return b
}

// Get retrieves a cached connection or creates a new one
func (b *ConnectionBuilder) Get() (*gorm.DB, error) {
	if b.mockDB != nil {
		return b.mockDB, nil
	}

	if b.dsn == "" && b.factory == nil {
		panic("either dsn or factory must be provided")
	}

	b.cache.mutex.RLock()
	db, exists := b.cache.connections[b.dsn]
	b.cache.mutex.RUnlock()

	if exists {
		return db, nil
	}

	return b.create()
}

// MustGet retrieves a cached connection or creates a new one, panicking on error
func (b *ConnectionBuilder) MustGet() *gorm.DB {
	if b.mockDB != nil {
		return b.mockDB
	}
	db, err := b.Get()
	if err != nil {
		panic(err)
	}
	return db
}

// create establishes a new database connection
func (b *ConnectionBuilder) create() (*gorm.DB, error) {
	var (
		db  *gorm.DB
		err error
	)

	// Use factory if provided, otherwise use dialector
	if b.factory != nil {
		db, err = b.factory()
	} else if b.dialector != nil {
		// Ensure config is not nil before passing to gorm.Open
		if b.config == nil {
			b.config = &gorm.Config{}
		}
		db, err = gorm.Open(b.dialector, b.config)
	} else {
		panic("either dialector or factory must be provided")
	}

	if err != nil {
		return nil, err
	}

	// Store the connection in the cache
	b.cache.mutex.Lock()
	defer b.cache.mutex.Unlock()
	b.cache.connections[b.dsn] = db

	return db, nil
}

// Remove deletes a connection from the cache by DSN
func (b *ConnectionBuilder) Remove() *ConnectionBuilder {
	b.cache.mutex.Lock()
	defer b.cache.mutex.Unlock()
	delete(b.cache.connections, b.dsn)
	return b
}

// GetAll returns all cached connections
func GetAll() map[string]*gorm.DB {
	cache := Instance()
	cache.mutex.RLock()
	defer cache.mutex.RUnlock()

	// Return a copy to prevent concurrent map access issues
	result := make(map[string]*gorm.DB, len(cache.connections))
	for dsn, db := range cache.connections {
		result[dsn] = db
	}

	return result
}
