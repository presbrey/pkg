package gormoize_test

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/presbrey/pkg/gormoize"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

// mockDialector implements gorm.Dialector for testing
type mockDialector struct {
	openFunc func() (*gorm.DB, error)
}

func (m mockDialector) Name() string {
	return "mock"
}

func (m mockDialector) Initialize(*gorm.DB) error {
	return nil
}

func (m mockDialector) New(config gorm.Config) gorm.Dialector {
	return m
}

func (m mockDialector) Migrator(*gorm.DB) gorm.Migrator {
	return nil
}

func (m mockDialector) DataTypeOf(*schema.Field) string {
	return ""
}

func (m mockDialector) DefaultValueOf(*schema.Field) clause.Expression {
	return nil
}

func (m mockDialector) BindVarTo(clause.Writer, *gorm.Statement, interface{}) {
}

func (m mockDialector) QuoteTo(clause.Writer, string) {
}

func (m mockDialector) Explain(sql string, vars ...interface{}) string {
	return ""
}

func (m mockDialector) Open(*gorm.DB) error {
	return nil
}

// createTestDB creates a real SQLite in-memory database for testing
func createTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NotNil(t, db)
	return db
}

// TestConnectionCaching tests that connections are properly cached by DSN
func TestConnectionCaching(t *testing.T) {
	// Clear the cache before running the test
	gormoize.Instance().Clear()

	// Create two connections with the same DSN
	dsn := "test-dsn-1"

	// Prepare a mock database that will be returned by the dialector
	mockDB := createTestDB(t)

	mockDialFunc := func() (*gorm.DB, error) {
		return mockDB, nil
	}

	dialector := mockDialector{openFunc: mockDialFunc}

	// First connection should be created
	db1, err := gormoize.Connection().
		WithDSN(dsn).
		WithDialector(dialector).
		Get()

	require.NoError(t, err)
	require.NotNil(t, db1)

	// Second connection with the same DSN should return the cached one
	db2, err := gormoize.Connection().
		WithDSN(dsn).
		WithDialector(dialector).
		Get()

	require.NoError(t, err)
	require.NotNil(t, db2)

	// The two variables should point to the same database instance
	assert.Same(t, db1, db2)

	// MustGet db2 as db3
	db3 := gormoize.Connection().
		WithDSN(dsn).
		WithDialector(dialector).
		MustGet()

	// The two variables should point to the same database instance
	assert.Same(t, db1, db3)

	// Check the number of connections in the cache
	connections := gormoize.GetAll()
	assert.Len(t, connections, 1)
}

// TestDifferentDSNs tests that different DSNs create different connections
func TestDifferentDSNs(t *testing.T) {
	// Clear the cache before running the test
	gormoize.Instance().Clear()

	// Create mock databases
	mockDB1 := createTestDB(t)
	mockDB2 := createTestDB(t)

	// First connection
	dsn1 := "test-dsn-1"
	db1, err := gormoize.Connection().
		WithDSN(dsn1).
		WithFactory(func() (*gorm.DB, error) {
			return mockDB1, nil
		}).
		Get()

	require.NoError(t, err)
	require.NotNil(t, db1)

	// Second connection with different DSN
	dsn2 := "test-dsn-2"
	db2, err := gormoize.Connection().
		WithDSN(dsn2).
		WithFactory(func() (*gorm.DB, error) {
			return mockDB2, nil
		}).
		Get()

	require.NoError(t, err)
	require.NotNil(t, db2)

	// The two connections should be different
	assert.NotSame(t, db1, db2)

	// Check the number of connections in the cache
	connections := gormoize.GetAll()
	assert.Len(t, connections, 2)
}

// TestRemoveConnection tests removing a connection from the cache
func TestRemoveConnection(t *testing.T) {
	// Clear the cache before running the test
	gormoize.Instance().Clear()

	// Create a connection
	dsn := "test-dsn-to-remove"
	mockDB := createTestDB(t)

	db, err := gormoize.Connection().
		WithDSN(dsn).
		WithFactory(func() (*gorm.DB, error) {
			return mockDB, nil
		}).
		Get()

	require.NoError(t, err)
	require.NotNil(t, db)

	// Check that it's in the cache
	connections := gormoize.GetAll()
	assert.Len(t, connections, 1)

	// Remove the connection
	gormoize.Connection().
		WithDSN(dsn).
		Remove()

	// Check that it's been removed
	connections = gormoize.GetAll()
	assert.Len(t, connections, 0)
}

// TestClearConnections tests clearing all connections from the cache
func TestClearConnections(t *testing.T) {
	// Create multiple connections
	dsns := []string{"dsn1", "dsn2", "dsn3"}

	for _, dsn := range dsns {
		mockDB := createTestDB(t)

		_, err := gormoize.Connection().
			WithDSN(dsn).
			WithFactory(func() (*gorm.DB, error) {
				return mockDB, nil
			}).
			Get()

		require.NoError(t, err)
	}

	// Check all connections are in the cache
	connections := gormoize.GetAll()
	assert.Len(t, connections, len(dsns))

	// Clear all connections
	gormoize.Instance().Clear()

	// Check the cache is empty
	connections = gormoize.GetAll()
	assert.Len(t, connections, 0)
}

// TestFactoryError tests handling of factory function errors
func TestFactoryError(t *testing.T) {
	// Clear the cache before running the test
	gormoize.Instance().Clear()

	expectedError := errors.New("factory error")

	// Create a connection with a factory that returns an error
	dsn := "test-dsn-error"
	_, err := gormoize.Connection().
		WithDSN(dsn).
		WithFactory(func() (*gorm.DB, error) {
			return nil, expectedError
		}).
		Get()

	// The error should be propagated
	assert.Error(t, err)
	assert.Equal(t, expectedError, err)

	// No connection should be added to the cache
	connections := gormoize.GetAll()
	assert.Len(t, connections, 0)
}

// TestMustGetPanic tests that MustGet panics on error
func TestMustGetPanic(t *testing.T) {
	// Clear the cache before running the test
	gormoize.Instance().Clear()

	// Create a connection with a factory that returns an error
	dsn := "test-dsn-panic"

	assert.Panics(t, func() {
		gormoize.Connection().
			WithDSN(dsn).
			WithFactory(func() (*gorm.DB, error) {
				return nil, errors.New("factory error")
			}).
			MustGet()
	})
}

// TestGetWithoutDSNPanic tests that Get panics if no DSN is provided
func TestGetWithoutDSNPanic(t *testing.T) {
	assert.Panics(t, func() {
		gormoize.Connection().Get()
	})
}

// TestGetWithoutDialectorOrFactoryPanic tests that Create panics if neither
// a dialector nor a factory is provided
func TestGetWithoutDialectorOrFactoryPanic(t *testing.T) {
	assert.Panics(t, func() {
		gormoize.Connection().
			WithDSN("test-dsn").
			Get()
	})
}

// TestWithMockDB verifies that Get/MustGet return the provided mock DB
func TestWithMockDB(t *testing.T) {
	// Ensure the cache is clean before the test
	gormoize.Instance().Clear()

	mockDB := &gorm.DB{} // Just need a non-nil pointer for identity check
	dsn := "mock-dsn"

	// Test Get() with MockDB
	db1, err := gormoize.Connection().
		WithDSN(dsn). // Provide DSN to ensure mock takes precedence
		WithMockDB(mockDB).
		Get()

	require.NoError(t, err)
	assert.Same(t, mockDB, db1, "Get() should return the exact mock DB instance")

	// Test MustGet() with MockDB
	db2 := gormoize.Connection().
		WithDSN(dsn). // Provide DSN to ensure mock takes precedence
		WithMockDB(mockDB).
		MustGet()

	assert.Same(t, mockDB, db2, "MustGet() should return the exact mock DB instance")

	// Verify that the mock DB was not actually added to the cache
	connections := gormoize.GetAll()
	_, exists := connections[dsn]
	assert.False(t, exists, "Mock DB should not be added to the cache")
}

// TestConcurrentAccess tests thread safety with concurrent access
func TestConcurrentAccess(t *testing.T) {
	// Clear the cache before running the test
	gormoize.Instance().Clear()

	// Number of concurrent goroutines
	concurrency := 100

	// Create a wait group to synchronize goroutines
	var wg sync.WaitGroup
	wg.Add(concurrency)

	// Create a single mockDB to return
	mockDB := createTestDB(t)

	// Run concurrent goroutines that access the same DSN
	dsn := "concurrent-test-dsn"

	// Start the goroutines
	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()

			// Add a small random delay to increase chance of race conditions
			time.Sleep(time.Duration(1+i%5) * time.Millisecond)

			db, err := gormoize.Connection().
				WithDSN(dsn).
				WithFactory(func() (*gorm.DB, error) {
					// Sleep to simulate DB connection time and increase race condition chance
					time.Sleep(10 * time.Millisecond)
					return mockDB, nil
				}).
				Get()

			// These assertions in goroutines won't fail the test properly
			// but at least they'll log errors
			if err != nil {
				t.Errorf("Error getting connection: %v", err)
			}
			if db == nil {
				t.Errorf("Got nil DB from connection")
			}
		}()
	}

	// Wait for all goroutines to finish
	wg.Wait()

	// We should have exactly one connection in the cache
	connections := gormoize.GetAll()
	assert.Len(t, connections, 1, "Should have exactly one connection after concurrent access")
}

// TestGetAllCopy tests that GetAll returns a copy of the connections map
func TestGetAllCopy(t *testing.T) {
	// Clear the cache before running the test
	gormoize.Instance().Clear()

	// Create a connection
	dsn := "test-dsn-copy"
	mockDB := createTestDB(t)

	_, err := gormoize.Connection().
		WithDSN(dsn).
		WithFactory(func() (*gorm.DB, error) {
			return mockDB, nil
		}).
		Get()

	require.NoError(t, err)

	// Get a copy of the connections
	connections := gormoize.GetAll()
	assert.Len(t, connections, 1)

	// Modify the copy (this shouldn't affect the actual cache)
	for k := range connections {
		delete(connections, k)
	}

	// Check that the original cache still has the connection
	connectionsAfter := gormoize.GetAll()
	assert.Len(t, connectionsAfter, 1)
}

// TestSingletonInstance tests that Instance always returns the same instance
func TestSingletonInstance(t *testing.T) {
	// Get the instance multiple times
	instance1 := gormoize.Instance()
	instance2 := gormoize.Instance()

	// They should be the same object
	assert.Same(t, instance1, instance2)
}

// TestChainedOperations tests that method chaining works as expected
func TestChainedOperations(t *testing.T) {
	// Clear the cache before running the test
	gormoize.Instance().Clear()

	// Create a DSN and mockDB
	dsn := "test-dsn-chaining"
	mockDB := createTestDB(t)

	// Test chained operations
	db, err := gormoize.Connection().
		WithDSN(dsn).
		WithFactory(func() (*gorm.DB, error) {
			return mockDB, nil
		}).
		Get()

	require.NoError(t, err)
	require.NotNil(t, db)

	// Remove and verify it's gone
	gormoize.Connection().
		WithDSN(dsn).
		Remove()

	connections := gormoize.GetAll()
	assert.Len(t, connections, 0)

	// Add again with chained Clear
	_, err = gormoize.Connection().
		WithDSN(dsn).
		WithFactory(func() (*gorm.DB, error) {
			return mockDB, nil
		}).
		Get()

	require.NoError(t, err)

	gormoize.Instance().Clear()

	// Verify it's gone
	connections = gormoize.GetAll()
	assert.Len(t, connections, 0)
}

// // Define a test model for real DB operations
// type TestModel struct {
// 	ID   uint `gorm:"primarykey"`
// 	Name string
// }

// TestRealDBOperations tests operations on a real database
func TestRealDBOperations(t *testing.T) {
	// Clear the cache before running the test
	gormoize.Instance().Clear()

	// Create a connection to a real SQLite in-memory database
	dsn := "file::memory:?cache=shared"

	db, err := gormoize.Connection().
		WithDSN(dsn).
		WithDialector(sqlite.Open(dsn)).
		WithConfig(&gorm.Config{}).
		Get()

	require.NoError(t, err)
	require.NotNil(t, db)

	// Auto migrate the test model
	err = db.AutoMigrate(&TestModel{})
	require.NoError(t, err)

	// Create a test record
	testModel := TestModel{Name: "Test Record"}
	result := db.Create(&testModel)
	require.NoError(t, result.Error)
	require.NotZero(t, testModel.ID)

	// Retrieve the connection again from the cache
	cachedDB, err := gormoize.Connection().
		WithDSN(dsn).
		WithDialector(sqlite.Open(dsn)).
		Get()

	require.NoError(t, err)
	require.NotNil(t, cachedDB)

	// Query the record using the cached connection
	var retrievedModel TestModel
	result = cachedDB.First(&retrievedModel, testModel.ID)
	require.NoError(t, result.Error)
	assert.Equal(t, testModel.Name, retrievedModel.Name)
}
