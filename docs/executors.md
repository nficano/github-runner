# Executors

github-runner uses a pluggable executor model. Each executor backend implements
the same interface, allowing runner pools to swap execution strategies without
changing the job lifecycle logic.

## Executor interface

Every executor implements four methods:

```go
type Executor interface {
    Prepare(ctx context.Context, job *api.Job) error
    Run(ctx context.Context, step api.Step) (*api.StepResult, error)
    Cleanup(ctx context.Context) error
    Info() api.ExecutorInfo
}
```

| Method    | Purpose |
|-----------|---------|
| `Prepare` | Set up the execution environment (create workspace, pull image, etc.) |
| `Run`     | Execute a single step and return its result |
| `Cleanup` | Tear down the environment (remove workspace, stop containers) |
| `Info`    | Return metadata about the executor's capabilities |

Executors are registered at init time and instantiated by name via a factory:

```go
executor.Register("docker", func() executor.Executor { return docker.New(cfg) })
exec, err := executor.New("docker")
```

---

## Shell

The shell executor runs steps directly on the host using `os/exec`. It is the
simplest backend and requires no additional infrastructure.

### How it works

1. **Prepare** — Creates a job-specific workspace directory under the
   configured `work_dir`.
2. **Run** — Spawns a shell process (`bash` by default) with `-e` to fail on
   errors. The step script is passed via stdin. Environment variables from the
   job, step, and runner config are merged and injected.
3. **Cleanup** — Removes the workspace directory.

### Configuration

```toml
[[runners]]
name = "shell-pool"
executor = "shell"
shell = "bash"       # bash, sh, zsh, pwsh
work_dir = "/var/lib/github-runner/work"
```

### Environment handling

The shell executor provides fine-grained control over environment variables:

- **Allowlist** — Only pass listed host variables to job steps.
- **Denylist** — Strip specific variables from the host environment.
- **Merge order** — Host env → job env → step env → runner variables
  (later values win).
- **Sanitisation** — Variable names are validated against POSIX rules
  (letters, digits, underscores only).

Runner-injected variables:

| Variable | Value |
|----------|-------|
| `GITHUB_ACTIONS` | `true` |
| `GITHUB_WORKSPACE` | Job workspace path |
| `RUNNER_TOOL_CACHE` | `<work_dir>/_tool_cache` |
| `RUNNER_TEMP` | `<work_dir>/_temp` |

### Security considerations

The shell executor runs with the same privileges as the runner process.
Untrusted workflows can access host resources. Recommendations:

- Run the runner as a dedicated unprivileged user.
- Use `work_dir` on a separate filesystem or partition.
- Prefer the Docker or Firecracker executor for untrusted workloads.

---

## Docker

The Docker executor runs each step in an isolated container using the Docker
Engine API (not the CLI). It provides resource limits, network isolation, and
image allow-listing.

### How it works

1. **Prepare** — Connects to the Docker daemon and pulls the default image
   according to the configured pull policy.
2. **Run** — For each step:
   - Validates the image against `allowed_images` glob patterns.
   - Creates a container with the step script, environment, volumes, and
     resource constraints.
   - Starts the container and streams stdout/stderr.
   - Waits for the container to exit and collects the exit code.
   - Removes the container.
3. **Cleanup** — Closes the Docker client connection.

### Configuration

```toml
[[runners]]
name = "docker-pool"
executor = "docker"
concurrency = 4

  [runners.docker]
  image = "ubuntu:22.04"
  privileged = false
  pull_policy = "if-not-present"   # always, if-not-present, never
  memory = "2g"
  cpus = 2.0
  network_mode = "bridge"          # bridge, host, none
  volumes = ["/cache:/cache:ro"]
  allowed_images = ["ubuntu:*", "node:*"]
  dns = ["8.8.8.8"]
  cap_drop = ["ALL"]
  cap_add = ["NET_BIND_SERVICE"]
  runtime = ""                     # e.g. "sysbox-runc"
  tmpfs = { "/tmp" = "rw,noexec,size=512m" }
```

### Pull policies

| Policy | Behaviour |
|--------|-----------|
| `always` | Pull the image before every job, even if it exists locally. |
| `if-not-present` | Pull only if the image is not already available locally. |
| `never` | Never pull; fail if the image is missing. |

### Image allow-listing

When `allowed_images` is set, only images matching at least one glob pattern
are permitted. This prevents workflows from using arbitrary images:

```toml
allowed_images = ["ubuntu:*", "node:18-*", "ghcr.io/nficano/*"]
```

### Volume mounts

Volumes use `host:container[:mode]` syntax. The mode is `rw` (read-write) by
default; use `ro` for read-only.

Sensitive host paths are blocked by default:

- `/dev`, `/proc`, `/sys`
- `/etc`
- `/var/run/docker.sock`
- `/` (root)

### Resource limits

| Field | Description | Example |
|-------|-------------|---------|
| `memory` | Container memory limit | `"2g"`, `"512m"` |
| `cpus` | CPU cores (fractional) | `2.0`, `0.5` |
| `cap_drop` | Linux capabilities to drop | `["ALL"]` |
| `cap_add` | Linux capabilities to add | `["NET_BIND_SERVICE"]` |

### Security options

The Docker executor applies hardened defaults:

- All capabilities dropped by default (`cap_drop = ["ALL"]`).
- No new privileges flag set (`--security-opt=no-new-privileges`).
- Privileged mode disabled by default.
- Custom OCI runtimes supported (e.g., `sysbox-runc` for rootless containers).

---

## Kubernetes

The Kubernetes executor runs each job as a pod in a Kubernetes cluster. It is
currently a scaffold implementation with the interface defined but execution
returning `ErrNotImplemented`.

### Planned architecture

1. **Prepare** — Connect to the cluster (in-cluster or kubeconfig), create a
   job-specific namespace or use the configured one.
2. **Run** — Build a pod spec per step with resource requests/limits, service
   account, node selectors, and the step script as the entrypoint. Create the
   pod, stream logs, and wait for completion.
3. **Cleanup** — Delete the pod and any associated resources.

### Configuration

```toml
[[runners]]
name = "k8s-pool"
executor = "kubernetes"

  [runners.kubernetes]
  namespace = "github-runner"
  image = "ubuntu:22.04"
  service_account = "runner-sa"
  cpu_request = "500m"
  cpu_limit = "2000m"
  memory_request = "512Mi"
  memory_limit = "4Gi"
  pull_policy = "IfNotPresent"
  node_selector = { "disktype" = "ssd" }
```

### Pod specification

The pod builder constructs pods with:

- **Labels** — `github-runner/job-id`, `github-runner/runner-name` for
  identification and cleanup.
- **Annotations** — Custom annotations from config plus job metadata.
- **Resource limits** — CPU and memory requests/limits from config.
- **Service account** — For RBAC-scoped access within the cluster.
- **Node selector** — Target specific node pools.
- **Active deadline** — Derived from step timeout for automatic pod
  termination.

### Features (planned)

- Pod-per-job isolation
- Resource limits and requests
- Service containers (sidecars)
- Node selection and affinity
- Image pull secrets
- Custom annotations and labels

---

## Firecracker

The Firecracker executor runs each job inside a lightweight microVM using
[Firecracker](https://firecracker-microvm.github.io/). It provides
hardware-level isolation with sub-second boot times. This is currently a
scaffold implementation.

### Planned architecture

1. **Prepare** — Configure a Firecracker microVM with the specified kernel,
   root filesystem, vCPU count, and memory. Mount the job workspace via
   virtio-blk or virtio-fs.
2. **Run** — Boot the microVM, communicate with a guest agent over VSOCK to
   execute step scripts, stream logs back, and collect exit codes. Enforce
   step timeouts by terminating the VM process.
3. **Cleanup** — Stop the microVM and remove temporary disk images and
   sockets.

### Configuration

```toml
[[runners]]
name = "firecracker-pool"
executor = "firecracker"

  [runners.firecracker]
  binary = "firecracker"           # Path to firecracker binary
  kernel = "/var/lib/firecracker/vmlinux"
  root_drive = "/var/lib/firecracker/rootfs.ext4"
  vcpus = 2
  memory_mb = 1024
  network = "tap0"
```

### Features (planned)

- MicroVM-per-job isolation
- Hardware-level security boundaries
- Fast boot times (< 200ms)
- Resource limits (vCPU, memory)
- Workspace mounting via virtio
- VSOCK-based guest agent communication

### Use cases

Firecracker is ideal for:

- Running untrusted third-party workflow code with strong isolation.
- High-density workloads where container overhead is acceptable but VM
  isolation is desired.
- Environments where Docker-in-Docker is not permitted.

---

## Choosing an executor

| Executor | Isolation | Setup complexity | Performance | Best for |
|----------|-----------|-----------------|-------------|----------|
| Shell | Process-level | Minimal | Fastest | Trusted code, simple builds |
| Docker | Container-level | Low | Fast | General purpose, untrusted code |
| Kubernetes | Pod-level | Medium | Moderate | Cloud-native, auto-scaling |
| Firecracker | VM-level | High | Fast | Maximum isolation, security-critical |

## Custom executors

The executor factory pattern allows registering custom backends:

```go
import "github.com/nficano/github-runner/internal/executor"

func init() {
    executor.Register("my-executor", func() executor.Executor {
        return &MyExecutor{}
    })
}
```

Your executor must implement all four interface methods. The runner pool calls
them in order: `Prepare` → `Run` (per step) → `Cleanup`.
