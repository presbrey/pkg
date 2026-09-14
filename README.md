# Go Utility Packages

A collection of Go utilities for web applications, configuration, encoding, networking, caching, and service integrations.

## Development

All three modules require [Go 1.27.1](https://go.dev/dl/) or later. The root module's SQLite tests also require a C compiler (CGO), and `git-http-cache` requires Git at runtime.

```bash
make check  # Build, vet, and race-test all three modules
make vuln   # Scan all three modules for known vulnerabilities
make tidy   # Refresh module manifests and checksums
```

The nested modules `base92/cli` and `git-http-cache` are independent: running `go test ./...` at the repository root does not test them. CI checks every module and builds the container. Dependabot checks Go dependencies, GitHub Actions, and Docker base images weekly.

Echo integrations continue to use the v4 API. YAML configuration uses the [YAML organization's maintained v3 package](https://github.com/yaml/go-yaml).

PostgreSQL and MySQL integration tests are opt-in. Start disposable databases with `make -C gormoize setup-local-dbs`, then run `make -C gormoize test-integration`. Stop them with `make -C gormoize teardown-local-dbs`. Override `GORMMEMO_PG_DSN` and `GORMMEMO_MYSQL_DSN` when using other local test databases.

## Packages

### [base92](./base92)

Provides `Encode()` and `Decode()` functions for converting byte slices to and from text using the package's encoding scheme. Includes a [command-line utility](./base92/cli) with `encode` and `decode` commands that read files or standard input and write to standard output.

### [cdns](./cdns)

Provides utilities for interacting with various Content Delivery Network (CDN) providers (e.g., Cloudflare, Fly.io).

### [echofly](./echofly)

Provides middleware for the Echo web framework to make sessions sticky on Fly.io. This middleware ensures that user sessions are consistently routed to the same machine instance using cookies and the `Fly-Replay` header.

### [echovalidator](./echovalidator)

Provides a simple integration of the `go-playground/validator/v10` library with the Echo (`v4`) web framework. Supports instance-based and singleton validators, with automatic JSON tag usage for the former.

### [envtree](./envtree)

Loads environment variables from `.env` files in the current working directory and all parent directories. Existing environment variables are preserved, and values from closer files take precedence over those from parent directories. Supports custom filenames, discovering file paths without loading them, and convenience functions such as `LoadDefault()`, `MustLoadDefault()`, and `AutoLoad()`.

### [fly](./fly)

Provides Fly.io utilities for listing machines, querying their status, and retrieving or streaming logs through `flyctl`. Includes the [flysu command](./fly/flysu) for listing machines and viewing logs across applications, with region filtering.

### [hooks](./hooks)

Provides a flexible hook registration and execution system with priority support.

Key features:

-   **Generic Registry:** Create registries for specific context types (`hooks.NewRegistry[T]()`).
-   **Hook Functions:** Define hooks as `func(context T) error`.
-   **Priority:** Register hooks with priorities (`RegisterWithPriority`). Lower numbers run first.
-   **Execution:** Run hooks in order using `RunHooks(context)`. It returns a map of errors for failed hooks.
-   **Panic Recovery:** Recovers from panics in individual hooks, allowing others to run.

See the [example](/hooks/example/main.go) for usage.

### [slugs](./slugs)

A Go package for generating URL-safe slugs with a fluent API pattern.

### [sshforward](./sshforward)

Forwards a local TCP port to a port on the SSH server's localhost. Supports password authentication and private keys supplied inline or from a file, with optional automatic allocation of the local port.

### [syncmap](./syncmap)

`syncmap` is a package that extends `sync.Map` to synchronize its values from a remote JSON URL. It provides a thread-safe way to keep a local cache of remote JSON data that is periodically refreshed.

#### Key Features

- Extends the standard Go `sync.Map` with all its methods
- Periodically fetches and synchronizes data from a remote JSON endpoint
- Configurable refresh period, timeout, and TLS verification
- Custom HTTP headers support
- Error handling callback
- Data transformation capability
- Type-specific getters for common types (string, int, int64, float, bool, map)
- Keys() method to retrieve all keys as a string slice

### [syncthing](./syncthing)

`syncthing` is a more advanced package that builds on the concepts of `syncmap` but uses Go's generics feature to provide type-safe access to map values of any type. It implements a Fluent Interface pattern for a more elegant API.

#### Key Features

- Type-safe access to map values using Go generics
- Fluent Interface with method chaining for configuration
- Periodic synchronization with remote JSON endpoints
- Support for custom HTTP headers
- Separate callbacks for updates and deletions
- Proper type conversion for numeric types and nested maps
- Error handling and update notifications
- TLS configuration options

### [wait](./wait)

Provides utilities for waiting on custom conditions, network services, HTTP endpoints, files, and processes. Supports configurable retry limits, timeouts, context cancellation, and backoff strategies, with `All()` and `Any()` helpers for combining conditions.

### [git-http-cache](./git-http-cache)

`git-http-cache` is a simple HTTP server that serves files from a Git repository, with support for authentication and automatic updates.

#### Key Features

- Serves files from a Git repository via HTTP
- Supports bearer token authentication for API access
- Automatically pulls the latest changes from the repository at configurable intervals
- Supports both HTTPS and SSH repository URLs
- Supports authentication for private repositories using:
  - Personal Access Tokens (PAT) for HTTPS repositories
  - SSH keys for SSH repositories
- Environment variable support for configuration

#### Authentication for Private Repositories

The git-http-cache server supports two methods for authenticating with private Git repositories:

##### Personal Access Token (PAT) Authentication

For HTTPS repository URLs, you can use a Personal Access Token (PAT) for authentication. This is the recommended method for GitHub, GitLab, and other Git hosting services.

**Command-line flag:**
```bash
./git-http-cache -repo https://github.com/user/private-repo.git -token your_token_here
```

**Environment variable:**
```bash
export GIT_TOKEN=your_token_here
./git-http-cache -repo https://github.com/user/private-repo.git
```

When using PAT authentication, the server modifies the repository URL to include the token:
```
https://github.com/user/private-repo.git → https://TOKEN@github.com/user/private-repo.git
```

##### SSH Key Authentication

For SSH repository URLs (e.g., `git@github.com:user/private-repo.git`), you can use an SSH key for authentication.

**Command-line flag:**
```bash
./git-http-cache -repo git@github.com:user/private-repo.git -ssh-key /path/to/ssh/key
```

**Environment variable:**
```bash
export GIT_SSH_KEY=/path/to/ssh/key
./git-http-cache -repo git@github.com:user/private-repo.git
```

When using SSH key authentication, the server sets the `GIT_SSH_COMMAND` environment variable to specify the SSH key:
```
GIT_SSH_COMMAND="ssh -i /path/to/ssh/key -o StrictHostKeyChecking=no"
```

#### Usage Examples

**Basic usage with a public repository:**
```bash
./git-http-cache -repo https://github.com/user/public-repo.git -dir /tmp/repo
```

**Using a private repository with PAT:**
```bash
./git-http-cache -repo https://github.com/user/private-repo.git -token your_token_here -dir /tmp/repo
```

**Using a private repository with SSH key:**
```bash
./git-http-cache -repo git@github.com:user/private-repo.git -ssh-key ~/.ssh/id_rsa -dir /tmp/repo
```

**Using environment variables:**
```bash
export GIT_TOKEN=your_token_here
export GIT_SSH_KEY=~/.ssh/id_rsa
./git-http-cache -repo https://github.com/user/private-repo.git -dir /tmp/repo
```

**With API authentication:**
```bash
./git-http-cache -repo https://github.com/user/repo.git -keys key1,key2,key3
```

## Comparison

| Feature | syncmap | syncthing |
|---------|---------|-----------|
| API Style | Traditional | Fluent Interface |
| Type Safety | Runtime type checking | Compile-time generics |
| Callbacks | Single error handler | Multiple (error, update, delete, refresh) |
| Change Tracking | No | Yes (added/changed/deleted keys) |
| Nested Maps | Basic support | Enhanced support with type conversion |

## Quick Start

### syncmap Example

```go
import (
    "fmt"
    "time"
    "github.com/presbrey/pkg/syncmap"
)

func main() {
    rm := syncmap.NewRemoteMap("https://api.example.com/data", &syncmap.Options{
        RefreshPeriod: 30 * time.Second,
    })
    
    rm.Start()
    defer rm.Stop()
    
    if name, ok := rm.GetString("name"); ok {
        fmt.Printf("Name: %s\n", name)
    }
    
    // Get all keys
    keys := rm.Keys()
    fmt.Println("All keys:", keys)
}
```

### syncthing Example

```go
import (
    "fmt"
    "time"
    "github.com/presbrey/pkg/syncthing"
)

func main() {
    rm := syncthing.NewMapString[string]("https://api.example.com/data").
        WithRefreshPeriod(30 * time.Second).
        WithUpdateCallback(func(updated []string) {
            fmt.Printf("Updated keys: %v\n", updated)
        }).
        Start()
    
    defer rm.Stop()
    
    if value, ok := rm.Get("name"); ok {
        fmt.Printf("Name: %s\n", value)
    }
    
    // Get all keys
    keys := rm.Keys()
    fmt.Println("All keys:", keys)
}
```

## When to Use Which Package

- Use **syncmap** when:
  - You need a simpler API that extends the familiar `sync.Map`
  - You don't need compile-time type safety

- Use **syncthing** when:
  - You prefer a fluent, chainable API
  - You want compile-time type safety with generics
  - You need detailed change tracking (added/changed/deleted keys)
  - You're working with complex nested data structures

## License

MIT License
