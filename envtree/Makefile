.PHONY: help test test-verbose test-race test-cover lint fmt vet clean install example

# Default target
help:
	@echo "Available targets:"
	@echo "  make test         - Run tests"
	@echo "  make test-verbose - Run tests with verbose output"
	@echo "  make test-race    - Run tests with race detector"
	@echo "  make test-cover   - Run tests with coverage report"
	@echo "  make lint         - Run linter (requires golangci-lint)"
	@echo "  make fmt          - Format code"
	@echo "  make vet          - Run go vet"
	@echo "  make clean        - Clean build artifacts"
	@echo "  make install      - Install dependencies"
	@echo "  make example      - Run example code"

# Run tests
test:
	go test -v ./...

# Run tests with verbose output
test-verbose:
	go test -v -race ./...

# Run tests with race detector
test-race:
	go test -race ./...

# Run tests with coverage
test-cover:
	go test -v -race -coverprofile=coverage.txt -covermode=atomic ./...
	go tool cover -html=coverage.txt -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run linter (requires golangci-lint to be installed)
lint:
	@which golangci-lint > /dev/null || (echo "golangci-lint not found. Install from https://golangci-lint.run/"; exit 1)
	golangci-lint run ./...

# Format code
fmt:
	go fmt ./...
	gofmt -s -w .

# Run go vet
vet:
	go vet ./...

# Clean build artifacts
clean:
	rm -f coverage.txt coverage.html
	go clean

# Install dependencies
install:
	go mod download
	go mod tidy

# Run example
example:
	cd examples && go run main.go

# Run all checks (format, vet, lint, test)
check: fmt vet lint test

# Update dependencies
update:
	go get -u ./...
	go mod tidy