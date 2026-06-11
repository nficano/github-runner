// Mock dataset that represents ONLY data the github-runner daemon actually
// exposes. There is no JSON admin API on the daemon — these endpoints model
// what an aggregator service would composite from:
//
//   - GET  /metrics  (Prometheus, namespace `github_runner_`)
//   - GET  /healthz  (always-ok alive check)
//   - GET  /readyz   (status + checks; check registry empty by default)
//   - the on-disk TOML config (RunnerConfig + GlobalConfig)
//   - the in-process Pool.activeJobs atomic counter (also in jobs_active gauge)
//
// Every field below maps to a source the runner exposes. Series the UI does
// not currently surface (job_duration / step_duration / executor_prepare_duration
// histograms, cache_operation_duration, cache_hit_ratio gauge) are omitted.

// ─── domain types ───────────────────────────────────────────────────────────

export type DockerConfig = {
  image: string;
  privileged: boolean;
  pull_policy: string;
  memory: string;
  cpus: number;
  network_mode: string;
};

export type KubernetesConfig = {
  namespace: string;
  service_account: string;
  cpu_limit: string;
  memory_limit: string;
};

export type CacheConfig = {
  type: "local" | "s3" | "gcs" | "";
  path: string;
  max_size: string;
  s3?: { bucket: string; region: string; prefix: string };
  gcs?: { bucket: string; prefix: string };
};

// Mirrors api.CacheStats. `supports_stats` is webui-only — reflects that
// the GCS backend's Stats() returns errNotImplemented.
export type CacheStats = {
  cache_type: string;
  entries: number;
  total_size: number;
  hit_count: number;
  miss_count: number;
  eviction_count: number;
  hit_ratio: number;
  supports_stats: boolean;
};

// poll_duration histogram p95 — the only latency cell the UI surfaces
export type PollDurationSummary = { p95: number };

export type PoolSnapshot = {
  // RunnerConfig
  name: string;
  url: string;
  executor: "shell" | "docker" | "kubernetes" | "firecracker";
  concurrency: number;
  labels: string[];
  work_dir: string;
  shell: string;
  ephemeral: boolean;
  docker?: DockerConfig;
  kubernetes?: KubernetesConfig;
  cache?: CacheConfig;
  environment_count: number; // values not exposed (secret-like)

  // Runtime — Prometheus series filtered by {runner=name}
  jobs_active: number;                                                          // jobs_active gauge
  jobs_total: { status: string; count: number }[];                              // jobs_total summed over repository
  jobs_by_repository: { repository: string; status: string; count: number }[];  // jobs_total full
  job_errors_total: { error_type: string; count: number }[];                    // job_errors_total
  poll_errors_total: { error_type: string; count: number }[];                   // poll_errors_total
  poll_duration: PollDurationSummary;                                           // poll_duration histogram
  heartbeat_errors_total: number;                                               // heartbeat_errors_total

  cache_stats?: CacheStats;
};

// ─── pools ──────────────────────────────────────────────────────────────────

export const pools: PoolSnapshot[] = [
  {
    name: "spruce",
    url: "https://github.com/nficano",
    executor: "docker",
    concurrency: 4,
    labels: ["self-hosted", "linux", "docker", "x64"],
    work_dir: "/var/lib/github-runner/work",
    shell: "bash",
    ephemeral: false,
    docker: {
      image: "ubuntu:22.04",
      privileged: false,
      pull_policy: "if-not-present",
      memory: "4g",
      cpus: 2.0,
      network_mode: "bridge",
    },
    cache: {
      type: "local",
      path: "/var/lib/github-runner/cache",
      max_size: "10g",
    },
    environment_count: 4,

    jobs_active: 3,
    jobs_total: [
      { status: "success",   count: 14_822 },
      { status: "failure",   count: 412 },
      { status: "cancelled", count: 18 },
    ],
    jobs_by_repository: [
      { repository: "nficano/github-runner",       status: "success", count: 9_412 },
      { repository: "nficano/github-runner",       status: "failure", count: 311 },
      { repository: "nficano/python-pytube",       status: "success", count: 3_204 },
      { repository: "nficano/python-pytube",       status: "failure", count: 88 },
      { repository: "nficano/dotfiles",            status: "success", count: 2_206 },
      { repository: "nficano/dotfiles",            status: "failure", count: 13 },
    ],
    job_errors_total: [
      { error_type: "executor_prepare", count: 142 },
      { error_type: "step_failed",      count: 224 },
      { error_type: "timeout",          count: 41 },
      { error_type: "cancelled",        count: 18 },
    ],
    poll_errors_total: [
      { error_type: "rate_limit", count: 12 },
      { error_type: "network",    count: 4 },
    ],
    poll_duration: { p95: 0.94 },
    heartbeat_errors_total: 1,
    cache_stats: {
      cache_type: "local",
      entries: 1_284,
      total_size: 9_822_400_000,
      hit_count: 23_588,
      miss_count: 4_824,
      eviction_count: 18,
      hit_ratio: 0.83,
      supports_stats: true,
    },
  },
  {
    name: "fir",
    url: "https://github.com/nficano",
    executor: "shell",
    concurrency: 2,
    labels: ["self-hosted", "macos", "arm64"],
    work_dir: "/Users/runner/work",
    shell: "zsh",
    ephemeral: false,
    environment_count: 2,

    jobs_active: 1,
    jobs_total: [
      { status: "success",   count: 1_822 },
      { status: "failure",   count: 188 },
      { status: "cancelled", count: 4 },
    ],
    jobs_by_repository: [
      { repository: "nficano/github-runner", status: "success", count: 902 },
      { repository: "nficano/github-runner", status: "failure", count: 142 },
      { repository: "nficano/dotfiles",      status: "success", count: 920 },
      { repository: "nficano/dotfiles",      status: "failure", count: 46 },
    ],
    job_errors_total: [
      { error_type: "step_failed", count: 161 },
      { error_type: "timeout",     count: 23 },
    ],
    poll_errors_total: [{ error_type: "network", count: 21 }],
    poll_duration: { p95: 1.42 },
    heartbeat_errors_total: 0,
  },
  {
    name: "cedar",
    url: "https://github.com/nficano",
    executor: "kubernetes",
    concurrency: 8,
    labels: ["self-hosted", "linux", "k8s", "gpu"],
    work_dir: "/workspace",
    shell: "bash",
    ephemeral: true,
    kubernetes: {
      namespace: "ci",
      service_account: "github-runner",
      cpu_limit: "4",
      memory_limit: "8Gi",
    },
    cache: {
      type: "s3",
      path: "",
      max_size: "100g",
      s3: { bucket: "h12o-ci-cache", region: "us-east-1", prefix: "cedar/" },
    },
    environment_count: 6,

    jobs_active: 0,
    jobs_total: [
      { status: "success",   count: 188 },
      { status: "failure",   count: 12 },
      { status: "cancelled", count: 2 },
    ],
    jobs_by_repository: [
      { repository: "nficano/github-runner", status: "success", count: 188 },
      { repository: "nficano/github-runner", status: "failure", count: 12 },
    ],
    job_errors_total: [
      { error_type: "executor_prepare", count: 8 },
      { error_type: "step_failed",      count: 4 },
    ],
    poll_errors_total: [],
    poll_duration: { p95: 0.62 },
    heartbeat_errors_total: 0,
    cache_stats: {
      cache_type: "s3",
      entries: 412,
      total_size: 8_420_000_000,
      hit_count: 882,
      miss_count: 358,
      eviction_count: 0,
      hit_ratio: 0.71,
      supports_stats: true,
    },
  },
  {
    name: "ash",
    url: "https://github.com/nficano",
    executor: "firecracker",
    concurrency: 6,
    labels: ["self-hosted", "linux", "fc", "ephemeral"],
    work_dir: "/var/lib/github-runner/work",
    shell: "bash",
    ephemeral: true,
    cache: {
      type: "gcs",
      path: "",
      max_size: "50g",
      gcs: { bucket: "h12o-ci-cache-gcs", prefix: "ash/" },
    },
    environment_count: 0,

    jobs_active: 0,
    jobs_total: [
      { status: "success", count: 22 },
      { status: "failure", count: 1 },
    ],
    jobs_by_repository: [
      { repository: "nficano/github-runner", status: "success", count: 22 },
      { repository: "nficano/github-runner", status: "failure", count: 1 },
    ],
    job_errors_total: [{ error_type: "step_failed", count: 1 }],
    poll_errors_total: [{ error_type: "network", count: 188 }],
    poll_duration: { p95: 0.92 },
    heartbeat_errors_total: 22,
    cache_stats: {
      cache_type: "gcs",
      entries: 0,
      total_size: 0,
      hit_count: 0,
      miss_count: 0,
      eviction_count: 0,
      hit_ratio: 0,
      supports_stats: false, // GCS backend Stats() returns errNotImplemented
    },
  },
];

// ─── global config (mirrors GlobalConfig) ──────────────────────────────────

export const global_config = {
  log_level: "info",
  log_format: "json",
  metrics_listen: "127.0.0.1:9252",
  health_listen: "127.0.0.1:8484",
  shutdown_timeout: "30s",
  check_interval: "3s",
  api: {
    base_url: "https://api.github.com",
    timeout: "30s",
    max_retries: 3,
    retry_backoff: "1s",
  },
};

// ─── process info ───────────────────────────────────────────────────────────
// build_version: -ldflags injected; runtime stats: only present if
// metrics.RuntimeCollector is registered (NOT registered by default).

export const process_info = {
  build_version: "v0.4.2",
  hostname: "ci-control-01.h12o.io",
  started_at: "2026-04-20T12:20:11Z",
  runtime_collector_registered: true,
  runtime: {
    uptime_seconds: 1_245_122,
  },
};

// ─── /healthz and /readyz ──────────────────────────────────────────────────
// /healthz: always {status:"ok", checks:{alive:"ok"}}
// /readyz: SetReady() controls "ready" key + CheckRegistry adds the rest.

export const healthz = {
  status: "ok" as const,
  checks: { alive: "ok" } as Record<string, string>,
};

export const readyz = {
  status: "ok" as const,
  checks: {
    ready:           "ok",
    github_api:      "ok",
    disk_space:      "ok",
    executor_docker: "ok",
  } as Record<string, string>,
  default_registry_empty: false,
};

// ─── on-disk config ────────────────────────────────────────────────────────

export const config_toml = `[global]
log_level      = "info"
log_format     = "json"
metrics_listen = "127.0.0.1:9252"
health_listen  = "127.0.0.1:8484"
shutdown_timeout = "30s"
check_interval = "3s"

  [global.api]
  base_url      = "https://api.github.com"
  timeout       = "30s"
  max_retries   = 3
  retry_backoff = "1s"

# ── docker pool ──
[[runners]]
name        = "spruce"
url         = "https://github.com/nficano"
token       = "\${SPRUCE_RUNNER_TOKEN}"
executor    = "docker"
concurrency = 4
labels      = ["self-hosted", "linux", "docker", "x64"]
work_dir    = "/var/lib/github-runner/work"

  [runners.docker]
  image       = "ubuntu:22.04"
  pull_policy = "if-not-present"
  memory      = "4g"
  cpus        = 2.0

  [runners.cache]
  type     = "local"
  path     = "/var/lib/github-runner/cache"
  max_size = "10g"

# ── shell pool ──
[[runners]]
name        = "fir"
url         = "https://github.com/nficano"
token       = "\${FIR_RUNNER_TOKEN}"
executor    = "shell"
concurrency = 2
labels      = ["self-hosted", "macos", "arm64"]
work_dir    = "/Users/runner/work"
shell       = "zsh"

# ── k8s pool ──
[[runners]]
name        = "cedar"
url         = "https://github.com/nficano"
token       = "\${CEDAR_RUNNER_TOKEN}"
executor    = "kubernetes"
concurrency = 8
labels      = ["self-hosted", "linux", "k8s", "gpu"]
ephemeral   = true

  [runners.kubernetes]
  namespace       = "ci"
  service_account = "github-runner"
  cpu_limit       = "4"
  memory_limit    = "8Gi"

  [runners.cache]
  type     = "s3"
  max_size = "100g"

    [runners.cache.s3]
    bucket = "h12o-ci-cache"
    region = "us-east-1"
    prefix = "cedar/"

# ── firecracker pool ──
[[runners]]
name        = "ash"
url         = "https://github.com/nficano"
token       = "\${ASH_RUNNER_TOKEN}"
executor    = "firecracker"
concurrency = 6
labels      = ["self-hosted", "linux", "fc", "ephemeral"]
ephemeral   = true

  [runners.cache]
  type     = "gcs"
  max_size = "50g"

    [runners.cache.gcs]
    bucket = "h12o-ci-cache-gcs"
    prefix = "ash/"
`;

// ─── full snapshot wrapper ──────────────────────────────────────────────────

export const snapshot = {
  process: process_info,
  global: global_config,
  health: { healthz, readyz },
  pools,
  config: {
    path: "/etc/github-runner/config.toml",
    last_reloaded: "2026-05-04T17:55:01Z",
    reload_signal: "SIGHUP",
  },
};
