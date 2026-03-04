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
git clone https://github.com/nficano/github-runner.git
cd github-runner
make build
```

The binary is written to `bin/github-runner`.

### Install via Homebrew

```sh
brew install nficano/tap/github-runner
```

### Register a runner

```sh
github-runner register \
  --url https://github.com/nficano/github-runner \
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

## Deployment

### Docker

```sh
docker run -v /etc/github-runner:/etc/github-runner \
  ghcr.io/nficano/github-runner:latest
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

## License

[MIT](https://github.com/nficano/github-runner/blob/main/LICENSE)
