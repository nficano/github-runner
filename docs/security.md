# Security

This document covers the security model, secret handling, trust boundaries, and
hardening recommendations for github-runner.

## Trust boundaries

```
┌─────────────────────────────────────────────┐
│  GitHub API (external, untrusted network)   │
└───────────────────────┬─────────────────────┘
                        │ HTTPS + Bearer token
┌───────────────────────▼─────────────────────┐
│  Runner process (trusted)                   │
│  ┌──────────┐ ┌──────────┐ ┌─────────────┐ │
│  │ Poller   │ │ Worker   │ │ Registrar   │ │
│  └──────────┘ └────┬─────┘ └─────────────┘ │
│                    │                        │
│  ┌─────────────────▼──────────────────────┐ │
│  │  Executor (isolation boundary)         │ │
│  │  ┌────────────────────────────────┐    │ │
│  │  │  Job step (untrusted code)     │    │ │
│  │  └────────────────────────────────┘    │ │
│  └────────────────────────────────────────┘ │
└─────────────────────────────────────────────┘
```

Key boundaries:

1. **Network boundary** — All communication with GitHub uses HTTPS with
   Bearer token authentication. Custom CA bundles and proxy configuration are
   supported for enterprise environments.

2. **Process boundary** — The runner process is trusted. It manages tokens,
   secrets, and job state. It should run as a dedicated unprivileged user.

3. **Executor boundary** — Job steps execute inside the executor's isolation
   domain (process, container, or microVM). This is the primary defence
   against malicious workflow code.

---

## Secret handling

### Storage

Secrets are stored in a `MemoryStore` that provides:

- **Copy-on-read** — `GetSecret()` returns a copy of the secret value,
  preventing callers from mutating the stored version.
- **Zeroisation** — `ZeroAll()` overwrites secret memory with zeroes using
  `unsafe.Pointer` before releasing it. Old values are also zeroed when
  overwritten by `Set()`.
- **Concurrency safety** — All operations are protected by `sync.RWMutex`.

For external secret backends, a `VaultProvider` scaffold is included for
HashiCorp Vault integration.

### Masking

The `Masker` is an `io.Writer` that intercepts all job output and replaces
secret values with `***`. It handles three encodings of each secret:

| Encoding | Example input | Masked output |
|----------|---------------|---------------|
| Plaintext | `ghp_abc123` | `***` |
| Base64 | `Z2hwX2FiYzEyMw==` | `***` |
| URL-encoded | `ghp%5Fabc123` | `***` |

Implementation details:

- Secrets are registered via `AddSecret()` before job execution.
- The masker buffers incomplete lines to catch secrets split across
  `Write()` boundaries.
- `Flush()` must be called at end of stream to emit any buffered content.
- The masker is thread-safe for concurrent writes from multiple goroutines.

### Log masking

The `MaskingHandler` wraps `slog.Handler` to apply the same masking to
structured log output. It masks:

- The log message text.
- All string-typed attribute values.
- Attributes within groups (recursive).

This ensures secrets cannot leak through runner diagnostic logs.

### Workflow commands

The `::add-mask::` workflow command allows workflows to dynamically register
additional secrets at runtime:

```yaml
- run: echo "::add-mask::$MY_DYNAMIC_SECRET"
```

The command parser in `internal/job/command.go` detects these commands and
registers the value with the masker.

---

## Executor isolation

### Shell

The shell executor provides **no isolation** beyond OS-level process
separation. The job step runs as the same user as the runner process and can
access the host filesystem, network, and other system resources.

**Mitigations:**

- Run the runner as a dedicated unprivileged user.
- Use `work_dir` on a separate partition with `noexec` if possible.
- Use environment denylist to strip sensitive host variables.
- Reserve shell executor for trusted, first-party workflows only.

### Docker

The Docker executor provides **container-level isolation** with hardened
defaults:

| Default | Setting |
|---------|---------|
| Capabilities | All dropped (`cap_drop = ["ALL"]`) |
| Privileged mode | Disabled |
| New privileges | Blocked (`no-new-privileges`) |
| Network | Bridge mode (isolated) |
| Sensitive mounts | Blocked (`/dev`, `/proc`, `/sys`, `/etc`, Docker socket) |

Additional hardening options:

- **Allowed images** — Restrict which images workflows can use via glob
  patterns.
- **Read-only volumes** — Mount shared caches as read-only.
- **Custom runtime** — Use `sysbox-runc` or `gVisor` for additional
  sandboxing.
- **Resource limits** — Prevent resource exhaustion with memory and CPU
  limits.
- **DNS** — Override DNS to prevent exfiltration via DNS tunnelling.

### Kubernetes

The Kubernetes executor (planned) provides **pod-level isolation** with:

- Dedicated service accounts with minimal RBAC permissions.
- Network policies for pod-to-pod isolation.
- Resource quotas and limit ranges.
- Node selectors to isolate runner pods on dedicated nodes.

### Firecracker

The Firecracker executor (planned) provides **hardware-level isolation**:

- Each job runs in its own microVM with a separate kernel.
- No shared kernel with the host (unlike containers).
- VSOCK communication only — no shared filesystem or network namespace.
- Ideal for running untrusted code from public repositories.

---

## API authentication

### Token handling

- Registration tokens are provided via configuration with `${VAR}`
  interpolation, keeping tokens out of config files on disk.
- Tokens are sent in the `Authorization: Bearer <token>` header over HTTPS.
- Token values are registered with the secret masker to prevent accidental
  logging.

### Rate limiting

The GitHub API client tracks rate limit state from `X-RateLimit-*` response
headers:

- **Backoff threshold** — When remaining requests fall below 5% of the
  limit, the client pauses until the reset window.
- **Thread safety** — Rate limit state is protected by `sync.RWMutex`.
- **Retry logic** — Transient errors (429, 502, 503, 504) trigger
  exponential backoff with jitter up to the configured `max_retries`.

### TLS

- All API communication uses HTTPS by default.
- Custom CA bundles are supported for GitHub Enterprise behind corporate
  proxies.
- HTTP proxy configuration is supported via standard environment variables
  (`HTTP_PROXY`, `HTTPS_PROXY`, `NO_PROXY`).

---

## Volume security

The Docker executor validates volume mounts to prevent common security
mistakes:

1. **Absolute paths required** — Both source and target must be absolute
   paths.
2. **No duplicate targets** — Prevents mount shadowing.
3. **Sensitive path blocking** — The following host paths are rejected:

   | Path | Reason |
   |------|--------|
   | `/` | Root filesystem access |
   | `/dev` | Device access |
   | `/proc` | Process information |
   | `/sys` | Kernel parameters |
   | `/etc` | System configuration |
   | `/var/run/docker.sock` | Docker daemon access (container escape) |

---

## Hardening checklist

- [ ] Run the runner process as a dedicated unprivileged user
- [ ] Use environment variable interpolation for tokens (`${RUNNER_TOKEN}`)
- [ ] Set `work_dir` to a dedicated partition
- [ ] Use Docker or Firecracker executor for untrusted workloads
- [ ] Configure `allowed_images` to restrict container images
- [ ] Drop all capabilities (`cap_drop = ["ALL"]`)
- [ ] Set resource limits (memory, CPU) to prevent exhaustion
- [ ] Enable structured JSON logging for audit trails
- [ ] Place the runner behind a firewall, restrict egress
- [ ] Rotate registration tokens regularly
- [ ] Use ephemeral mode for public repository runners
- [ ] Monitor metrics for anomalous job patterns
- [ ] Review and restrict volume mounts to read-only where possible
