# Git Repository File Server

A Go HTTP server that clones a Git repository, serves its contents via HTTP with bearer token authentication, and automatically pulls updates at regular intervals.

## Features

- **Git Integration**: Automatically clones a Git repository on startup
- **Auto-Update**: Pulls latest changes from the repository at configurable intervals (default: 1 minute)
- **Bearer Token Authentication**: Supports multiple static bearer tokens for API security
- **File Serving**: Serves repository contents using Go's built-in `http.FileServer`
- **Graceful Shutdown**: Properly handles interrupt signals for clean shutdown
- **Configurable Port**: Uses PORT environment variable or defaults to 5000

## Installation

```bash
# Clone this project
git clone <this-project-url>
cd git-http-cache

# Build the server
go build -o git-http-cache main.go

# Run tests
go test -v
```

## Usage

### Basic Usage

```bash
# Serve a public repository without authentication
./git-http-cache -repo https://github.com/user/repo.git

# With authentication (multiple keys)
./git-http-cache -repo https://github.com/user/repo.git -keys "key1,key2,key3"

# Custom clone directory
./git-http-cache -repo https://github.com/user/repo.git -dir ./my-repo

# Custom pull interval
./git-http-cache -repo https://github.com/user/repo.git -interval 5m
```

### Command Line Flags

- `-repo` (required): Git repository URL to clone and serve
- `-keys`: Comma-separated list of bearer tokens for authentication (optional)
- `-dir`: Directory to clone the repository into (default: `./repo`)
- `-interval`: Interval for pulling updates from git (default: `1m`)

### Environment Variables

- `PORT`: HTTP server port (default: `5000`)

## Authentication

When bearer keys are configured, all requests must include an Authorization header:

```bash
# Example with curl
curl -H "Authorization: Bearer your-secret-key" http://localhost:5000/index.html

# Example with wget
wget --header="Authorization: Bearer your-secret-key" http://localhost:5000/index.html
```

If no keys are configured, the server accepts all requests without authentication.

## Examples

### 1. Serve a Documentation Site

```bash
# Clone and serve a documentation repository
./git-http-cache \
  -repo https://github.com/myorg/docs.git \
  -keys "doc-reader-key-1,doc-reader-key-2" \
  -interval 5m

# Access the documentation
curl -H "Authorization: Bearer doc-reader-key-1" http://localhost:5000/
```

### 2. Serve Static Assets

```bash
# Serve a repository of static assets with frequent updates
PORT=8080 ./git-http-cache \
  -repo https://github.com/myorg/assets.git \
  -dir ./assets \
  -interval 30s
```

### 3. Private Repository with SSH

```bash
# Make sure SSH keys are configured for git
./git-http-cache \
  -repo git@github.com:myorg/private-repo.git \
  -keys "private-key-1,private-key-2"
```

## Docker Usage

Create a `Dockerfile`:

```dockerfile
FROM golang:1.21-alpine AS builder
RUN apk add --no-cache git
WORKDIR /app
COPY . .
RUN go build -o git-http-cache main.go

FROM alpine:latest
RUN apk add --no-cache git
WORKDIR /app
COPY --from=builder /app/git-http-cache .
EXPOSE 5000
ENTRYPOINT ["./git-http-cache"]
```

Build and run:

```bash
# Build Docker image
docker build -t git-http-cache .

# Run with environment variables
docker run -d \
  -p 5000:5000 \
  -e PORT=5000 \
  git-http-cache \
  -repo https://github.com/user/repo.git \
  -keys "key1,key2"
```

## Testing

Run the test suite:

```bash
# Run all tests
go test -v

# Run with coverage
go test -v -cover

# Run benchmarks
go test -bench=.

# Run specific test
go test -v -run TestAuthMiddleware
```

## Security Considerations

1. **Bearer Tokens**: Store bearer tokens securely and rotate them regularly
2. **HTTPS**: Use a reverse proxy (nginx, traefik) to add HTTPS in production
3. **Private Repos**: Ensure proper SSH key configuration for private repositories
4. **File Access**: The server exposes all files in the repository - ensure no sensitive data

## Performance

- Files are served directly from disk using Go's efficient `http.FileServer`
- Git pull operations run in a separate goroutine to avoid blocking requests
- Concurrent requests are handled efficiently by Go's HTTP server
- Use the benchmark tests to measure performance in your environment

## Graceful Shutdown

The server handles SIGINT and SIGTERM signals for graceful shutdown:
- Stops accepting new connections
- Waits for existing connections to complete (10-second timeout)
- Stops the auto-update routine
- Cleans up resources

## Troubleshooting

### Git Clone Fails
- Check repository URL is correct
- For private repos, ensure SSH keys are configured
- Check network connectivity

### Authentication Issues
- Verify bearer token format: `Authorization: Bearer <token>`
- Check token is in the configured list (no extra spaces)
- If no auth needed, don't configure any keys

### Auto-Pull Not Working
- Check git pull works manually in the clone directory
- Verify repository permissions
- Check server logs for error messages

## License

MIT License - See LICENSE file for details

## Contributing

1. Fork the repository
2. Create a feature branch
3. Commit your changes
4. Push to the branch
5. Create a Pull Request
