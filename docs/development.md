# Development

Guide for building, testing, and contributing to github-runner.

## Prerequisites

- Go 1.22 or later
- Docker (for Docker executor and integration tests)
- golangci-lint (for linting)
- gofumpt and goimports (for formatting)
- goreleaser (for release builds, optional)

## Building

```sh
# Build the binary
make build

# Install to $GOPATH/bin
make install

# Build Docker image
make docker-build

# Snapshot release build (all platforms)
make goreleaser
```

The binary is written to `bin/github-runner` with version information embedded
via ldflags:

```sh
bin/github-runner version
```

### Build variables

| Variable | Default | Description |
|----------|---------|-------------|
| `VERSION` | `git describe` | Semantic version tag |
| `COMMIT` | `git rev-parse --short HEAD` | Git commit hash |
| `DATE` | Current UTC time | Build timestamp |

Override at build time:

```sh
make build VERSION=1.0.0 COMMIT=abc1234
```

## Testing

```sh
# Run unit tests with race detection
make test

# Run integration tests (requires Docker)
make test-integration

# Generate coverage report
make coverage

# Run benchmarks
make bench
```

### Test conventions

- **Table-driven tests** — All tests use the table-driven pattern with
  named sub-tests.
- **Race detection** — Tests run with `-race` by default via `GOTESTFLAGS`.
- **No external dependencies** — Unit tests mock external services. No
  network calls, no Docker daemon required.
- **Integration tag** — Tests that require external services use the
  `integration` build tag and run separately.

### Writing tests

Follow these patterns:

```go
func TestFunctionName(t *testing.T) {
    tests := []struct {
        name    string
        input   InputType
        want    OutputType
        wantErr bool
    }{
        {
            name:  "descriptive case name",
            input: InputType{...},
            want:  OutputType{...},
        },
        {
            name:    "error case",
            input:   InputType{...},
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := FunctionName(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("FunctionName() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if got != tt.want {
                t.Errorf("FunctionName() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

### Mocking

External interfaces are defined for testability:

- `github.GitHubClient` — Mock GitHub API calls.
- `cache.Cache` — Mock cache operations.
- `executor.Executor` — Mock executor backends.
- `secret.SecretProvider` — Mock secret retrieval.
- `artifact.ArtifactStore` — Mock artifact operations.

No mock generation framework is required. Tests define inline
implementations of these interfaces.

## Code quality

```sh
# Run all checks (vet + lint + test)
make check

# Format code
make fmt

# Run go vet
make vet

# Run golangci-lint
make lint

# Tidy modules
make tidy
```

### Style guide

The project follows:

- [Effective Go](https://go.dev/doc/effective-go)
- [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md)

Key conventions:

| Convention | Rule |
|------------|------|
| Logging | `log/slog` with structured fields |
| Context | First parameter on all I/O functions |
| Errors | Wrap with `fmt.Errorf("...: %w", err)` |
| Init functions | Not allowed |
| Panic | Never used for control flow |
| Global state | No mutable global state |

## Project layout

```
cmd/github-runner/       Entry point (main.go)
internal/
  cli/                   Cobra command definitions
  config/                TOML config loading, validation, hot reload
  runner/                Manager, pool, worker, poller, lifecycle
  executor/              Executor interface and implementations
    shell/               Shell executor (os/exec)
    docker/              Docker executor (Engine API)
    kubernetes/          Kubernetes executor (scaffold)
    firecracker/         Firecracker executor (scaffold)
  github/                GitHub API client with retries
  cache/                 Cache backends (local, S3, GCS)
  artifact/              Artifact upload/download/retention
  secret/                Secret masking and storage
  hook/                  Pre/post job hooks and webhooks
  job/                   Job model, step execution, commands
  log/                   Structured logging with masking
  metrics/               Prometheus metrics and server
  health/                Health check endpoints
  version/               Build version info
pkg/api/                 Shared types for plugin interface
configs/                 Example configuration files
docs/                    Documentation
scripts/                 Deployment and completion scripts
```

### Package dependencies

Dependencies flow inward. Packages in `internal/` depend on `pkg/api` for
shared types but never on each other's internals:

```
cmd/github-runner → internal/cli → internal/config
                                 → internal/runner → internal/executor
                                                   → internal/github
                                                   → internal/cache
                                                   → internal/artifact
                                                   → internal/secret
                                                   → internal/hook
                                                   → internal/job
                                 → internal/log
                                 → internal/metrics
                                 → internal/health
```

## Adding a new executor

1. Create a package under `internal/executor/<name>/`.
2. Implement the `executor.Executor` interface.
3. Register the executor in the factory:

```go
func init() {
    executor.Register("my-executor", func() executor.Executor {
        return New(DefaultConfig())
    })
}
```

4. Add configuration fields to `internal/config/config.go`.
5. Add validation rules to `internal/config/validate.go`.
6. Add tests.
7. Document in `docs/executors.md`.

## Adding a new CLI command

1. Create `internal/cli/<command>.go`.
2. Define a `newCmdName()` function returning `*cobra.Command`.
3. Add the command to the root in `internal/cli/root.go`.
4. Add shell completions in `scripts/completions/`.

## Release process

Releases are automated via GoReleaser:

```sh
# Tag a release
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0

# GoReleaser builds and publishes:
# - Linux/macOS binaries (amd64, arm64)
# - Docker multi-arch images (ghcr.io)
# - Homebrew formula
# - SHA-256 checksums
```

### Release artefacts

| Artefact | Format | Description |
|----------|--------|-------------|
| Binaries | tar.gz (Linux), zip (macOS) | Standalone executables |
| Docker images | Multi-arch manifest | `ghcr.io/nficano/github-runner:<version>` |
| Homebrew | Formula in tap | `brew install nficano/tap/github-runner` |
| Checksums | SHA-256 | `checksums.txt` |

### Docker images

Multi-architecture images are published to GitHub Container Registry:

```
ghcr.io/nficano/github-runner:1.0.0          # Multi-arch manifest
ghcr.io/nficano/github-runner:1.0.0-amd64    # Linux AMD64
ghcr.io/nficano/github-runner:1.0.0-arm64    # Linux ARM64
ghcr.io/nficano/github-runner:latest         # Latest release
```

## Deployment

### systemd (Linux)

```sh
sudo cp bin/github-runner /usr/local/bin/
sudo cp scripts/github-runner.service /etc/systemd/system/
sudo systemctl enable --now github-runner
```

The systemd unit includes security hardening:

- `DynamicUser=yes` or dedicated service user
- `ProtectSystem=strict`
- `ProtectHome=yes`
- `NoNewPrivileges=yes`
- `ReadWritePaths` limited to work and log directories

### launchd (macOS)

```sh
brew install nficano/tap/github-runner
brew services start github-runner
```

### Docker

```sh
docker run -d \
  -v /etc/github-runner:/etc/github-runner:ro \
  -v /var/lib/github-runner:/var/lib/github-runner \
  ghcr.io/nficano/github-runner:latest
```

## Make targets

| Target | Description |
|--------|-------------|
| `make build` | Build the binary |
| `make install` | Install to `$GOPATH/bin` |
| `make test` | Run unit tests with race detection |
| `make test-integration` | Run integration tests |
| `make lint` | Run golangci-lint |
| `make fmt` | Format code with gofumpt and goimports |
| `make vet` | Run go vet |
| `make coverage` | Generate HTML coverage report |
| `make bench` | Run benchmarks |
| `make generate` | Run go generate |
| `make docker-build` | Build Docker image |
| `make goreleaser` | Snapshot release build |
| `make clean` | Remove build artefacts |
| `make tidy` | Tidy go modules |
| `make check` | Run vet + lint + test |
| `make help` | Show available targets |
