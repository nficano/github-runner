# github-runner

A self-hosted GitHub Actions runner built from scratch in Go. Designed for
teams that need fine-grained control over their CI/CD infrastructure with
support for multiple concurrent runners, pluggable executors, and
production-grade observability.

## Features

- **Multi-pool architecture** -- Run independent runner pools with different
  executors and concurrency settings from a single process.
- **Pluggable executors** -- Shell, Docker (via Engine API), Kubernetes
  (pod-per-job), and Firecracker (microVM) backends.
- **Concurrent job execution** -- Each pool runs N workers in parallel with
  full isolation between jobs.
- **Secret masking** -- Automatic redaction of secret values (including
  base64 and URL-encoded variants) in all log output.
- **Caching** -- Local filesystem with LRU eviction, S3-compatible, and GCS
  backends with zstd compression.
- **Artifact management** -- Upload, download, retention enforcement, and
  SHA-256 integrity verification.
- **Observability** -- Structured logging (slog), Prometheus metrics, health
  endpoints (`/healthz`, `/readyz`), and pprof support.
- **Graceful lifecycle** -- Context-based cancellation, in-flight job drain
  on shutdown, heartbeat reporting, and hot config reload via SIGHUP.
- **Ephemeral mode** -- Single-use runners that auto-deregister after one
  job, ideal for autoscaling.

## Quick start

### Install from source

Requires Go 1.22+.

```sh
git clone https://github.com/org/github-runner.git
cd github-runner
make build
```

The binary is written to `bin/github-runner`.

### Install via Homebrew

```sh
brew install org/tap/github-runner
```

### Register a runner

```sh
github-runner register \
  --url https://github.com/your-org/your-repo \
  --token YOUR_REGISTRATION_TOKEN \
  --executor docker \
  --name my-runner \
  --labels self-hosted,linux,docker
```

### Start the runner

```sh
github-runner start --config /etc/github-runner/config.toml
```

### Verify connectivity

```sh
github-runner verify
```

## Configuration

Configuration uses TOML. A minimal example:

```toml
[global]
log_level = "info"
log_format = "json"
check_interval = "3s"

[global.api]
base_url = "https://api.github.com"

[[runners]]
name = "docker-pool"
url = "https://github.com/your-org/your-repo"
token = "${RUNNER_TOKEN}"
executor = "docker"
concurrency = 4
labels = ["self-hosted", "linux", "docker"]

  [runners.docker]
  image = "ubuntu:22.04"
  pull_policy = "if-not-present"
  memory = "2g"
  cpus = 2.0
```

Environment variables in `${VAR}` syntax are interpolated at load time.

See [configs/config.example.toml](configs/config.example.toml) for a fully
annotated reference and [docs/configuration.md](docs/configuration.md) for
detailed documentation.

## CLI reference

```
github-runner <command> [flags]

Commands:
  register      Register a new runner with GitHub
  unregister    Remove runner registration
  start         Start the runner daemon
  stop          Signal a running daemon to stop
  run           Execute a single job and exit
  list          List registered runners
  verify        Test connectivity and authentication
  status        Show live worker status
  cache         Cache management (clear, stats, prune)
  exec          Run a workflow locally (experimental)
  version       Print version information

Global flags:
  --config      Config file path (default /etc/github-runner/config.toml)
  --log-level   Log level: debug, info, warn, error (default info)
  --log-format  Log format: json, text (default json)
```

Exit codes: `0` success, `1` general error, `2` config error, `3` auth error.

## Documentation

| Document | Description |
|----------|-------------|
| [Architecture](docs/architecture.md) | System design, component diagram, concurrency model |
| [Configuration](docs/configuration.md) | Complete config reference with all fields |
| [Executors](docs/executors.md) | Shell, Docker, Kubernetes, and Firecracker backends |
| [Security](docs/security.md) | Sandboxing, secret handling, trust boundaries |
| [Observability](docs/observability.md) | Logging, Prometheus metrics, health checks |
| [Development](docs/development.md) | Building, testing, contributing guidelines |

## Deployment

### Docker

```sh
docker run -v /etc/github-runner:/etc/github-runner \
  ghcr.io/org/github-runner:latest
```

### systemd (Linux)

```sh
sudo cp bin/github-runner /usr/local/bin/
sudo cp scripts/github-runner.service /etc/systemd/system/
sudo systemctl enable --now github-runner
```

### launchd (macOS via Homebrew)

```sh
brew services start github-runner
```

## Project layout

```
cmd/github-runner/       Entry point
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
```

## License

[MIT](LICENSE)
