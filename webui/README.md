# webui

Admin UI for the self-hosted `github-runner` daemon. Nuxt 4 + Tailwind v4.
Visual language is the [Theater](../) design system (Day/Night themes,
Newsreader / JetBrains Mono / Inter).

```sh
cd webui
npm install
npm run dev   # http://localhost:3000   ($ npm run dev -- --port 4141 etc.)
```

## What the daemon actually exposes

The runner has no admin JSON API. Every cell in this UI is sourced from
exactly one of:

| Source                          | Endpoint                                      |
|---------------------------------|-----------------------------------------------|
| Prometheus series (private registry, namespace `github_runner_`) | `GET :9252/metrics` |
| Liveness probe                  | `GET :8484/healthz` (always-ok if process up) |
| Readiness probe                 | `GET :8484/readyz` (`SetReady()` + `CheckRegistry`) |
| TOML config + `config.Watcher`  | on-disk `config.toml`, `SIGHUP` to reload     |
| In-process `Pool.activeJobs`    | exposed via `jobs_active` gauge                |

Series labels: every metric carries `runner=<pool name>`. There is **no**
historical job ledger, **no** per-worker CPU/mem, **no** log buffer, **no**
queue-depth metric. The webui surfaces only what the daemon tracks.

## Pages

| Route             | Purpose                                                                              |
|-------------------|--------------------------------------------------------------------------------------|
| `/`               | Dashboard — process info, /healthz·/readyz status, per-pool active gauge, aggregate counters |
| `/pools`          | One row per `[[runners]]` entry: capacity, active gauge, jobs success/error totals, poll p95, cache backend |
| `/pools/[name]`   | Pool detail — every Prometheus rollup filtered by `runner=<name>` plus full `RunnerConfig` (docker/k8s/cache blocks) |
| `/activity`       | Cumulative `jobs_total` rollups by status / pool / repository, plus `job_errors_total` / `poll_errors_total` by type |
| `/health`         | Raw `/healthz` and `/readyz` responses with check-by-check status                    |
| `/configuration`  | Resolved `GlobalConfig` + read-only TOML view; reload is `SIGHUP`-driven             |

## Honest gaps the UI shows

- Process metrics (goroutines, heap, fds, uptime) are surfaced **only if
  `metrics.RuntimeCollector` is registered**; the default `NewMetrics()`
  does not register it. The dashboard shows "runtime collector not
  registered" when `runtime_collector_registered: false`.
- `/readyz` only carries the `ready` key by default — the operator must
  register `GitHubAPICheck` / `DiskSpaceCheck` / `ExecutorCheck` to see
  more. The Health page surfaces this caveat when the registry is empty.
- The GCS cache backend's `Stats()` returns `errNotImplemented`. The Pool
  cache section shows a "stats unavailable" notice in that case rather
  than fake numbers.

## Wiring up the real backend

[server/api/](server/api/) currently serves curated mock data shaped after
the Go types in [`pkg/api/types.go`](../pkg/api/types.go),
[`internal/config/config.go`](../internal/config/config.go), and
[`internal/metrics/metrics.go`](../internal/metrics/metrics.go). To go
live, replace [server/utils/mock.ts](server/utils/mock.ts) with code that:

1. Scrapes `/metrics` and parses `github_runner_*` series (group by
   `runner` label, derive p50/p95 with `histogram_quantile()`).
2. GETs `/healthz` and `/readyz`.
3. Reads the TOML config off disk for the static fields.

Or, build a small admin HTTP server inside the daemon that exposes the
same shape directly.

## Theme

`theater-day` and `theater-night` share component vocabulary; only CSS
custom properties shift. Toggle in topbar; persisted in `localStorage`.
The TOML viewer renders as a nested `theater-night` block.
