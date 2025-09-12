# Build stage
FROM golang:1.21-alpine AS builder

# Install git for the build process
RUN apk add --no-cache git

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod ./

# Download dependencies (if any in the future)
RUN go mod download

# Copy source code
COPY *.go ./

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o git-http-cache .

# Run tests during build (optional)
RUN go test -v

# Final stage
FROM alpine:latest

# Install git and ca-certificates for HTTPS git operations
RUN apk --no-cache add git ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 -S appuser && \
    adduser -u 1000 -S appuser -G appuser

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/git-http-cache .

# Create directory for repository with correct permissions
RUN mkdir -p /app/repo && chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Expose default port
EXPOSE 5000

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:5000/ || exit 1

# Default entrypoint
ENTRYPOINT ["./git-http-cache"]

# Default command (can be overridden)
CMD ["-repo", "https://github.com/golang/example.git", "-dir", "/app/repo"]

# Authentication options:
#
# 1. Personal Access Token (PAT) for HTTPS repositories:
#    docker run -e GIT_TOKEN=your_token_here git-http-cache -repo https://github.com/user/private-repo.git
#    or
#    docker run git-http-cache -repo https://github.com/user/private-repo.git -git-token your_token_here
#
# 2. SSH Key for SSH repositories:
#    docker run -e GIT_SSH_KEY=/path/to/ssh/key -v /host/path/to/ssh/key:/path/to/ssh/key:ro git-http-cache -repo git@github.com:user/private-repo.git
#    or
#    docker run -v /host/path/to/ssh/key:/path/to/ssh/key:ro git-http-cache -repo git@github.com:user/private-repo.git -ssh-key-path /path/to/ssh/key
#
# 3. Bearer token authentication for API access:
#    docker run -e BEARER_KEYS=key1,key2,key3 git-http-cache
#    or
#    docker run git-http-cache -bearer-keys key1,key2,key3