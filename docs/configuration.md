# Configuration reference

github-runner uses a TOML configuration file. The default path is
`/etc/github-runner/config.toml`, overridden with `--config`.

## Environment variable interpolation

String values support `${VAR_NAME}` syntax. The variable is replaced with its
environment value at load time. Missing variables resolve to an empty string
with a warning logged.

```toml
token = "${RUNNER_TOKEN}"
```

## Hot reload

The runner watches the config file for changes and reloads on modification.
Send `SIGHUP` to trigger a reload manually. Invalid configs are rejected with
a log warning; the previous config remains active.

---

## `[global]`

Application-wide settings.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `log_level` | string | `"info"` | Log level: `debug`, `info`, `warn`, `error` |
| `log_format` | string | `"json"` | Log format: `json`, `text` |
| `metrics_listen` | string | `"127.0.0.1:9252"` | Address for the Prometheus metrics endpoint |
| `health_listen` | string | `"127.0.0.1:8484"` | Address for the health check endpoints |
| `shutdown_timeout` | duration | `"30s"` | Max wait time for in-flight jobs during shutdown |
| `check_interval` | duration | `"3s"` | Interval between job polling requests |

### `[global.api]`

GitHub API client settings.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `base_url` | string | `"https://api.github.com"` | API base URL (change for GitHub Enterprise) |
| `timeout` | duration | `"30s"` | HTTP request timeout |
| `max_retries` | int | `3` | Max retry attempts for transient failures |
| `retry_backoff` | duration | `"1s"` | Base duration for exponential backoff |

---

## `[[runners]]`

Each `[[runners]]` block defines an independent runner pool. Multiple pools
can be configured with different executors and concurrency.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | *required* | Unique name for this runner pool |
| `url` | string | *required* | GitHub repository, org, or enterprise URL |
| `token` | string | *required* | Registration token (supports `${VAR}`) |
| `executor` | string | *required* | Executor type: `shell`, `docker`, `kubernetes`, `firecracker` |
| `concurrency` | int | `1` | Number of parallel jobs this pool handles |
| `labels` | string[] | `[]` | Labels for job routing |
| `work_dir` | string | `""` | Working directory for job workspaces (must be absolute) |
| `shell` | string | `"bash"` | Default shell for `run:` steps |
| `ephemeral` | bool | `false` | De-register after one job |

---

### `[runners.docker]`

Docker executor settings. Required when `executor = "docker"`.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `image` | string | `"ubuntu:22.04"` | Default container image |
| `privileged` | bool | `false` | Run containers in privileged mode |
| `pull_policy` | string | `"if-not-present"` | Image pull policy: `always`, `if-not-present`, `never` |
| `memory` | string | `""` | Memory limit (e.g., `"2g"`, `"512m"`) |
| `cpus` | float | `0` | CPU limit (fractional cores, e.g., `2.0`) |
| `network_mode` | string | `"bridge"` | Docker network mode: `bridge`, `host`, `none` |
| `volumes` | string[] | `[]` | Volume mounts in `host:container:mode` format |
| `allowed_images` | string[] | `[]` | Glob patterns for allowed images |
| `dns` | string[] | `[]` | DNS servers for containers |
| `cap_drop` | string[] | `["ALL"]` | Linux capabilities to drop |
| `cap_add` | string[] | `[]` | Linux capabilities to add |
| `runtime` | string | `""` | OCI runtime (e.g., `"sysbox-runc"`) |
| `tmpfs` | map | `{}` | tmpfs mounts: `{ "/tmp" = "rw,noexec,size=512m" }` |

---

### `[runners.kubernetes]`

Kubernetes executor settings. Required when `executor = "kubernetes"`.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `namespace` | string | `"github-runner"` | Target namespace |
| `image` | string | `"ubuntu:22.04"` | Default container image |
| `service_account` | string | `""` | Kubernetes service account |
| `cpu_request` | string | `"500m"` | CPU resource request |
| `cpu_limit` | string | `"2000m"` | CPU resource limit |
| `memory_request` | string | `"512Mi"` | Memory resource request |
| `memory_limit` | string | `"4Gi"` | Memory resource limit |
| `node_selector` | map | `{}` | Node selector key-value pairs |
| `pull_policy` | string | `"IfNotPresent"` | Image pull policy |

---

### `[runners.cache]`

Cache settings for the runner pool.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `type` | string | `"local"` | Backend: `local`, `s3`, `gcs` |
| `path` | string | `""` | Directory for local cache |
| `max_size` | size | `"0"` | Max cache size (e.g., `"10g"`) |

#### `[runners.cache.s3]`

S3-compatible cache backend settings.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `bucket` | string | `""` | S3 bucket name |
| `region` | string | `""` | AWS region |
| `endpoint` | string | `""` | Custom endpoint for S3-compatible services |
| `prefix` | string | `""` | Key prefix within the bucket |

#### `[runners.cache.gcs]`

Google Cloud Storage cache backend settings (scaffold).

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `bucket` | string | `""` | GCS bucket name |
| `prefix` | string | `""` | Object prefix within the bucket |

---

### `[runners.environment]`

Key-value pairs injected as environment variables into every job.

```toml
[runners.environment]
CI = "true"
RUNNER_POOL = "docker-fast"
```

---

## Type reference

### Duration

Standard Go duration strings: `"30s"`, `"5m"`, `"1h"`, `"100ms"`.

### Size

Human-readable byte sizes (case-insensitive):

| Suffix | Unit |
|--------|------|
| `b` or none | bytes |
| `k`, `kb` | kilobytes (1024) |
| `m`, `mb` | megabytes (1024^2) |
| `g`, `gb` | gigabytes (1024^3) |
| `t`, `tb` | terabytes (1024^4) |

Fractional values are supported: `"1.5g"`.

---

## Validation rules

The config is validated at load time. All errors are collected and reported
together. The following rules are enforced:

- At least one `[[runners]]` block must be defined
- Each runner must have `name`, `url`, `token`, and `executor`
- No duplicate runner names
- `executor` must be one of: `shell`, `docker`, `kubernetes`, `firecracker`
- `concurrency` must be > 0
- `work_dir` must be an absolute path (if set)
- `url` must be a valid HTTP/HTTPS URL
- Docker config must have a non-empty `image` and valid `pull_policy` when
  executor is `docker`
- Kubernetes config must have a non-empty `namespace` and `image` when
  executor is `kubernetes`
- `log_level` must be one of: `debug`, `info`, `warn`, `error`
- `log_format` must be one of: `json`, `text`

## Example

See [configs/config.example.toml](https://github.com/nficano/github-runner/blob/main/configs/config.example.toml) for a fully
annotated example with all available options.
