.PHONY: test test-unit test-integration bench lint coverage clean

# Default target
all: test lint

# Run all tests
test: test-unit test-integration

# Run only unit tests
test-unit:
	go test -v ./... -short

# Run integration tests including external database tests
test-integration:
	GORMMEMO_RUN_INTEGRATION_TESTS=true \
	GORMMEMO_PG_DSN="host=localhost user=postgres password=postgres dbname=gormmemo_test port=5432 sslmode=disable" \
	GORMMEMO_MYSQL_DSN="root:mysql@tcp(localhost:3306)/gormmemo_test?charset=utf8mb4&parseTime=True&loc=Local" \
	go test -v ./... -run Integration

# Run benchmarks
bench:
	go test -bench=. -benchmem ./...

# Run linting
lint:
	golangci-lint run ./...

# Generate code coverage report
coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	go tool cover -func=coverage.out

# Clean up generated files
clean:
	rm -f coverage.out coverage.html
	rm -f test.db

# Setup local databases for testing
setup-local-dbs:
	docker run --name gormmemo-postgres -e POSTGRES_USER=postgres -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=gormmemo_test -p 5432:5432 -d postgres:14
	docker run --name gormmemo-mysql -e MYSQL_ROOT_PASSWORD=mysql -e MYSQL_DATABASE=gormmemo_test -p 3306:3306 -d mysql:8.0

# Stop and remove local test databases
teardown-local-dbs:
	docker stop gormmemo-postgres gormmemo-mysql || true
	docker rm gormmemo-postgres gormmemo-mysql || true