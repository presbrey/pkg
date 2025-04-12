package gormoize_test

import (
	"os"
	"testing"

	"github.com/presbrey/pkg/gormoize"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Skip tests that require external databases if env vars not set
func shouldSkipExternalDBTests() bool {
	return os.Getenv("GORMMEMO_RUN_INTEGRATION_TESTS") != "true"
}

// TestModel is a simple model for testing database operations
type TestModel struct {
	ID   uint `gorm:"primarykey"`
	Name string
}

// TestSQLiteIntegration tests integration with SQLite
func TestSQLiteIntegration(t *testing.T) {
	// Clear the cache before running the test
	gormoize.Instance().Clear()

	// Create an on-disk test database
	tempFile := "test.db"
	defer os.Remove(tempFile) // Clean up after test

	dsn := tempFile + "?cache=shared"
	db, err := gormoize.Connection().
		WithDSN(dsn).
		WithDialector(sqlite.Open(dsn)).
		WithConfig(&gorm.Config{}).
		Get()

	require.NoError(t, err)
	require.NotNil(t, db)

	// Test database operations
	err = db.AutoMigrate(&TestModel{})
	require.NoError(t, err)

	// Create a record
	model := TestModel{Name: "SQLite Test"}
	result := db.Create(&model)
	require.NoError(t, result.Error)

	// Retrieve the connection from cache and verify data persists
	cachedDB, err := gormoize.Connection().
		WithDSN(dsn).
		Get()

	require.NoError(t, err)
	require.NotNil(t, cachedDB)

	var found TestModel
	result = cachedDB.First(&found, model.ID)
	require.NoError(t, result.Error)
	assert.Equal(t, "SQLite Test", found.Name)
}

// TestPostgresIntegration tests integration with PostgreSQL
func TestPostgresIntegration(t *testing.T) {
	if shouldSkipExternalDBTests() {
		t.Skip("Skipping PostgreSQL integration test")
	}

	// Get PostgreSQL DSN from environment variable
	dsn := os.Getenv("GORMMEMO_PG_DSN")
	if dsn == "" {
		dsn = "host=localhost user=postgres password=postgres dbname=gormmemo_test port=5432 sslmode=disable"
	}

	// Clear the cache before running the test
	gormoize.Instance().Clear()

	// Connect to PostgreSQL
	db, err := gormoize.Connection().
		WithDSN(dsn).
		WithDialector(postgres.Open(dsn)).
		WithConfig(&gorm.Config{}).
		Get()

	if err != nil {
		t.Skipf("Skipping PostgreSQL test due to connection error: %v", err)
		return
	}

	require.NotNil(t, db)

	// Clean up test table
	db.Exec("DROP TABLE IF EXISTS test_models")

	// Test database operations
	err = db.AutoMigrate(&TestModel{})
	require.NoError(t, err)

	// Create a record
	model := TestModel{Name: "PostgreSQL Test"}
	result := db.Create(&model)
	require.NoError(t, result.Error)

	// Retrieve the connection from cache and verify data persists
	cachedDB, err := gormoize.Connection().
		WithDSN(dsn).
		Get()

	require.NoError(t, err)
	require.NotNil(t, cachedDB)

	var found TestModel
	result = cachedDB.First(&found, model.ID)
	require.NoError(t, result.Error)
	assert.Equal(t, "PostgreSQL Test", found.Name)

	// Clean up
	db.Exec("DROP TABLE IF EXISTS test_models")
}

// TestMySQLIntegration tests integration with MySQL
func TestMySQLIntegration(t *testing.T) {
	if shouldSkipExternalDBTests() {
		t.Skip("Skipping MySQL integration test")
	}

	// Get MySQL DSN from environment variable
	dsn := os.Getenv("GORMMEMO_MYSQL_DSN")
	if dsn == "" {
		dsn = "root:mysql@tcp(localhost:3306)/gormmemo_test?charset=utf8mb4&parseTime=True&loc=Local"
	}

	// Clear the cache before running the test
	gormoize.Instance().Clear()

	// Connect to MySQL
	db, err := gormoize.Connection().
		WithDSN(dsn).
		WithDialector(mysql.Open(dsn)).
		WithConfig(&gorm.Config{}).
		Get()

	if err != nil {
		t.Skipf("Skipping MySQL test due to connection error: %v", err)
		return
	}

	require.NotNil(t, db)

	// Clean up test table
	db.Exec("DROP TABLE IF EXISTS test_models")

	// Test database operations
	err = db.AutoMigrate(&TestModel{})
	require.NoError(t, err)

	// Create a record
	model := TestModel{Name: "MySQL Test"}
	result := db.Create(&model)
	require.NoError(t, result.Error)

	// Retrieve the connection from cache and verify data persists
	cachedDB, err := gormoize.Connection().
		WithDSN(dsn).
		Get()

	require.NoError(t, err)
	require.NotNil(t, cachedDB)

	var found TestModel
	result = cachedDB.First(&found, model.ID)
	require.NoError(t, result.Error)
	assert.Equal(t, "MySQL Test", found.Name)

	// Clean up
	db.Exec("DROP TABLE IF EXISTS test_models")
}

// TestConfigPersistence tests that connection config persists when retrieving from cache
func TestConfigPersistence(t *testing.T) {
	// Clear the cache before running the test
	gormoize.Instance().Clear()

	dsn := "file::memory:?cache=shared"

	// Create a connection with a specific config
	config := &gorm.Config{
		SkipDefaultTransaction: true,
		PrepareStmt:            true,
	}

	db, err := gormoize.Connection().
		WithDSN(dsn).
		WithDialector(sqlite.Open(dsn)).
		WithConfig(config).
		Get()

	require.NoError(t, err)
	require.NotNil(t, db)

	// Create a table and verify the prepared statement works
	err = db.AutoMigrate(&TestModel{})
	require.NoError(t, err)

	// Insert some data
	model := TestModel{Name: "Config Test"}
	result := db.Create(&model)
	require.NoError(t, result.Error)

	// Get the connection again and verify config was persisted
	// This is done indirectly by verifying DB operations work
	cachedDB, err := gormoize.Connection().
		WithDSN(dsn).
		Get()

	require.NoError(t, err)
	require.NotNil(t, cachedDB)

	// Query with the cached DB
	var found TestModel
	result = cachedDB.First(&found, model.ID)
	require.NoError(t, result.Error)
	assert.Equal(t, "Config Test", found.Name)
}

// TestMultipleDBTypes tests using multiple database types simultaneously
func TestMultipleDBTypes(t *testing.T) {
	// Clear the cache before running the test
	gormoize.Instance().Clear()

	// Create SQLite connection
	sqliteDSN := "file::memory:?cache=shared"
	sqliteDB, err := gormoize.Connection().
		WithDSN(sqliteDSN).
		WithDialector(sqlite.Open(sqliteDSN)).
		Get()

	require.NoError(t, err)
	require.NotNil(t, sqliteDB)

	// Set up SQLite DB
	err = sqliteDB.AutoMigrate(&TestModel{})
	require.NoError(t, err)

	// Insert data into SQLite
	sqliteModel := TestModel{Name: "SQLite Multi-DB Test"}
	result := sqliteDB.Create(&sqliteModel)
	require.NoError(t, result.Error)

	// Skip external DB tests if not enabled
	if shouldSkipExternalDBTests() {
		// Just verify we have one cached connection
		connections := gormoize.GetAll()
		assert.Len(t, connections, 1)
		return
	}

	// Create PostgreSQL connection if available
	pgDSN := os.Getenv("GORMMEMO_PG_DSN")
	if pgDSN != "" {
		pgDB, err := gormoize.Connection().
			WithDSN(pgDSN).
			WithDialector(postgres.Open(pgDSN)).
			Get()

		if err == nil {
			// Clean up and set up PostgreSQL DB
			pgDB.Exec("DROP TABLE IF EXISTS test_models")
			err = pgDB.AutoMigrate(&TestModel{})
			require.NoError(t, err)

			// Insert data into PostgreSQL
			pgModel := TestModel{Name: "PostgreSQL Multi-DB Test"}
			result = pgDB.Create(&pgModel)
			require.NoError(t, result.Error)
		}
	}

	// Create MySQL connection if available
	mysqlDSN := os.Getenv("GORMMEMO_MYSQL_DSN")
	if mysqlDSN != "" {
		mysqlDB, err := gormoize.Connection().
			WithDSN(mysqlDSN).
			WithDialector(mysql.Open(mysqlDSN)).
			Get()

		if err == nil {
			// Clean up and set up MySQL DB
			mysqlDB.Exec("DROP TABLE IF EXISTS test_models")
			err = mysqlDB.AutoMigrate(&TestModel{})
			require.NoError(t, err)

			// Insert data into MySQL
			mysqlModel := TestModel{Name: "MySQL Multi-DB Test"}
			result = mysqlDB.Create(&mysqlModel)
			require.NoError(t, result.Error)
		}
	}

	// Check all connections are properly cached
	connections := gormoize.GetAll()
	numDBs := 1 // SQLite always included
	if os.Getenv("GORMMEMO_PG_DSN") != "" {
		numDBs++
	}
	if os.Getenv("GORMMEMO_MYSQL_DSN") != "" {
		numDBs++
	}

	assert.Len(t, connections, numDBs)

	// Cleanup
	if pgDB, ok := connections[pgDSN]; ok {
		pgDB.Exec("DROP TABLE IF EXISTS test_models")
	}
	if mysqlDB, ok := connections[mysqlDSN]; ok {
		mysqlDB.Exec("DROP TABLE IF EXISTS test_models")
	}
}
