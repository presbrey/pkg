.PHONY: build run test clean docker-build docker-run help

# Variables
BINARY_NAME=git-http-cache
DOCKER_IMAGE=git-http-cache:latest
GO=go
GOFLAGS=-v
PORT?=5000

# Default target
all: test build

## help: Show this help message
help:
	@echo 'Usage:'
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' | sed -e 's/^/ /'

## build: Build the binary
build:
	$(GO) build $(GOFLAGS) -o $(BINARY_NAME) main.go
	@echo "Binary built: $(BINARY_NAME)"

## run: Run the server (requires -repo flag)
run: build
	@if [ -z "$(repo)" ]; then \
		echo "Error: repo parameter is required"; \
		echo "Usage: make run repo=https://github.com/user/repo.git"; \
		exit 1; \
	fi
	PORT=$(PORT) ./$(BINARY_NAME) -repo $(repo) $(ARGS)

## test: Run tests
test:
	$(GO) test $(GOFLAGS) -race -cover ./...

## test-verbose: Run tests with verbose output
test-verbose:
	$(GO) test $(GOFLAGS) -race -cover -v ./...

## benchmark: Run benchmarks
benchmark:
	$(GO) test -bench=. -benchmem ./...

## coverage: Generate test coverage report
coverage:
	$(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

## lint: Run golint (requires golangci-lint)
lint:
	@which golangci-lint > /dev/null || (echo "golangci-lint not installed. Run: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest" && exit 1)
	golangci-lint run

## fmt: Format code
fmt:
	$(GO) fmt ./...
	@echo "Code formatted"

## vet: Run go vet
vet:
	$(GO) vet ./...
	@echo "Vet complete"

## clean: Remove binary and test artifacts
clean:
	rm -f $(BINARY_NAME)
	rm -f coverage.out coverage.html
	rm -rf repo/
	@echo "Cleanup complete"

## docker-build: Build Docker image
docker-build:
	docker build -t $(DOCKER_IMAGE) .
	@echo "Docker image built: $(DOCKER_IMAGE)"

## docker-run: Run Docker container (requires repo parameter)
docker-run: docker-build
	@if [ -z "$(repo)" ]; then \
		echo "Error: repo parameter is required"; \
		echo "Usage: make docker-run repo=https://github.com/user/repo.git"; \
		exit 1; \
	fi
	docker run -d \
		--name $(BINARY_NAME) \
		-p $(PORT):$(PORT) \
		-e PORT=$(PORT) \
		$(DOCKER_IMAGE) \
		-repo $(repo) \
		$(if $(keys),-keys "$(keys)") \
		$(if $(interval),-interval $(interval))
	@echo "Container started: $(BINARY_NAME)"
	@echo "Access at: http://localhost:$(PORT)"

## docker-stop: Stop and remove Docker container
docker-stop:
	docker stop $(BINARY_NAME) 2>/dev/null || true
	docker rm $(BINARY_NAME) 2>/dev/null || true
	@echo "Container stopped and removed"

## docker-logs: Show Docker container logs
docker-logs:
	docker logs -f $(BINARY_NAME)

## docker-compose-up: Start with docker-compose
docker-compose-up:
	docker-compose up -d
	@echo "Services started with docker-compose"

## docker-compose-down: Stop docker-compose services
docker-compose-down:
	docker-compose down
	@echo "Services stopped"

## install: Install the binary to GOPATH/bin
install: build
	$(GO) install
	@echo "Installed to $$(go env GOPATH)/bin/$(BINARY_NAME)"

## uninstall: Remove the binary from GOPATH/bin
uninstall:
	rm -f $$(go env GOPATH)/bin/$(BINARY_NAME)
	@echo "Uninstalled from $$(go env GOPATH)/bin/"

## dev: Run with example repo for development
dev: build
	PORT=$(PORT) ./$(BINARY_NAME) \
		-repo https://github.com/golang/example.git \
		-dir ./dev-repo \
		-interval 30s \
		$(ARGS)

## example-auth: Run example with authentication
example-auth: build
	PORT=$(PORT) ./$(BINARY_NAME) \
		-repo https://github.com/golang/example.git \
		-keys "test-key-1,test-key-2" \
		-dir ./example-repo \
		-interval 1m
	@echo "Server running with auth. Test with:"
	@echo "  curl -H 'Authorization: Bearer test-key-1' http://localhost:$(PORT)/"

# Check if git is installed
check-deps:
	@which git > /dev/null || (echo "Error: git is not installed" && exit 1)
	@which $(GO) > /dev/null || (echo "Error: go is not installed" && exit 1)
	@echo "Dependencies OK"

.DEFAULT_GOAL := help