# Observability

github-runner provides structured logging, Prometheus metrics, and health
endpoints for production monitoring.

## Logging

### Configuration

```toml
[global]
log_level = "info"     # debug, info, warn, error
log_format = "json"    # json, text
```

CLI flags override the config file:

```sh
github-runner start --log-level debug --log-format text
```

### Structured logging

All logging uses Go's `log/slog` package with consistent field names:

| Field | Description |
|-------|-------------|
| `job_id` | GitHub job identifier |
| `repository` | Repository in `owner/repo` format |
| `workflow` | Workflow name |
| `runner_name` | Runner pool name |
| `pool_name` | Pool identifier |
| `executor` | Executor type (shell, docker, etc.) |
| `step` | Step name or ID |
| `duration` | Operation duration |
| `error` | Error message |
| `component` | Subsystem name |

Example JSON log output:

```json
{
  "time": "2026-03-03T12:00:00Z",
  "level": "INFO",
  "msg": "job completed",
  "job_id": 12345,
  "repository": "org/repo",
  "workflow": "CI",
  "duration": "45.2s",
  "component": "worker"
}
```

### Component loggers

Each subsystem creates a child logger with its component name:

```go
logger := log.WithComponent(parentLogger, "poller")
logger := log.WithJobContext(parentLogger, jobID, repo, workflow)
```

### Secret masking in logs

The `MaskingHandler` wraps the slog handler to redact secret values from log
output. It masks:

- Log message text
- String attribute values
- Attributes within nested groups

This is enabled automatically when secrets are registered with the masker
via `log.SetupWithMask()`.

---

## Metrics

### Endpoint

Prometheus metrics are served on a configurable address:

```toml
[global]
metrics_listen = "127.0.0.1:9252"
```

Scrape the `/metrics` path:

```sh
curl http://127.0.0.1:9252/metrics
```

### Available metrics

#### Job metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `github_runner_jobs_total` | Counter | `pool`, `status` | Total jobs processed |
| `github_runner_job_duration_seconds` | Histogram | `pool`, `executor` | Job execution duration |
| `github_runner_jobs_active` | Gauge | `pool` | Currently running jobs |
| `github_runner_job_errors_total` | Counter | `pool`, `error_type` | Job execution errors |

#### Step metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `github_runner_step_duration_seconds` | Histogram | `pool`, `step` | Individual step duration |

#### Cache metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `github_runner_cache_hit_ratio` | Gauge | `backend` | Cache hit/miss ratio |
| `github_runner_cache_operation_duration` | Histogram | `backend`, `operation` | Cache operation latency |

#### Infrastructure metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `github_runner_executor_prepare_duration` | Histogram | `executor` | Executor preparation time |
| `github_runner_poll_duration` | Histogram | `pool` | Job poll request duration |
| `github_runner_poll_errors_total` | Counter | `pool` | Poll request failures |
| `github_runner_heartbeat_errors_total` | Counter | `pool` | Heartbeat send failures |

#### Runtime metrics

The runtime collector exports Go process metrics:

| Metric | Type | Description |
|--------|------|-------------|
| `github_runner_goroutines` | Gauge | Current goroutine count |
| `github_runner_threads` | Gauge | OS thread count |
| `github_runner_heap_alloc_bytes` | Gauge | Heap allocation in bytes |
| `github_runner_heap_inuse_bytes` | Gauge | Heap memory in use |
| `github_runner_gc_pause_seconds` | Summary | GC pause durations |
| `github_runner_open_fds` | Gauge | Open file descriptors |
| `github_runner_uptime_seconds` | Gauge | Process uptime |

### Custom registry

Metrics use a private Prometheus registry to avoid polluting the global
default registry. This prevents conflicts when embedding github-runner as a
library.

### Prometheus scrape configuration

```yaml
scrape_configs:
  - job_name: "github-runner"
    static_configs:
      - targets: ["127.0.0.1:9252"]
    scrape_interval: 15s
```

### Grafana dashboard

Key panels to create:

- **Job throughput** — `rate(github_runner_jobs_total[5m])` by pool and
  status
- **Active jobs** — `github_runner_jobs_active` by pool
- **Job duration p99** — `histogram_quantile(0.99, github_runner_job_duration_seconds)`
- **Error rate** — `rate(github_runner_job_errors_total[5m])`
- **Poll latency** — `histogram_quantile(0.95, github_runner_poll_duration)`
- **Cache hit rate** — `github_runner_cache_hit_ratio`
- **Goroutines** — `github_runner_goroutines`
- **Memory** — `github_runner_heap_alloc_bytes`

---

## Health endpoints

### Configuration

```toml
[global]
health_listen = "127.0.0.1:8484"
```

### Liveness: `/healthz`

Returns `200 OK` if the process is running. Use this for Kubernetes
liveness probes.

```sh
curl http://127.0.0.1:8484/healthz
```

```json
{
  "status": "ok"
}
```

### Readiness: `/readyz`

Returns `200 OK` when the runner is ready to accept jobs. Returns `503
Service Unavailable` during startup or shutdown drain.

```sh
curl http://127.0.0.1:8484/readyz
```

```json
{
  "status": "ok",
  "checks": {
    "github_api": "ok",
    "disk_space": "ok",
    "executor": "ok"
  }
}
```

### Registered checks

| Check | Description |
|-------|-------------|
| `github_api` | HEAD request to GitHub API base URL |
| `disk_space` | Minimum free disk space via statfs |
| `executor` | Executor backend operational (e.g., `docker info`) |

### Kubernetes probes

```yaml
livenessProbe:
  httpGet:
    path: /healthz
    port: 8484
  initialDelaySeconds: 5
  periodSeconds: 10

readinessProbe:
  httpGet:
    path: /readyz
    port: 8484
  initialDelaySeconds: 10
  periodSeconds: 5
```

### Readiness lifecycle

1. **Startup** — Health server starts, readiness is `false`.
2. **Ready** — After pools are initialised, `SetReady(true)` is called.
3. **Shutdown** — On receiving SIGTERM/SIGINT, `SetReady(false)` is called
   immediately to stop receiving traffic while in-flight jobs drain.

---

## pprof

The metrics server includes pprof endpoints for runtime profiling at
`/debug/pprof/`. These are useful for diagnosing performance issues:

```sh
go tool pprof http://127.0.0.1:9252/debug/pprof/heap
go tool pprof http://127.0.0.1:9252/debug/pprof/goroutine
```

---

## Alerting recommendations

| Alert | Condition | Severity |
|-------|-----------|----------|
| High error rate | `rate(github_runner_job_errors_total[5m]) > 0.1` | Warning |
| No jobs processed | `increase(github_runner_jobs_total[30m]) == 0` | Info |
| Pool saturated | `github_runner_jobs_active == <concurrency>` | Warning |
| Poll failures | `rate(github_runner_poll_errors_total[5m]) > 0` | Warning |
| Heartbeat failures | `rate(github_runner_heartbeat_errors_total[5m]) > 0` | Critical |
| High memory | `github_runner_heap_alloc_bytes > 1e9` | Warning |
| Readiness down | `/readyz` returns 503 | Critical |
