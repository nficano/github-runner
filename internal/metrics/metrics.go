// Package metrics defines Prometheus metrics for the GitHub Actions runner and
// exposes them via a dedicated HTTP server. All metrics are registered on a
// custom [prometheus.Registry] so that the default global registry is never
// modified, making the package safe for testing and embedding.
package metrics

import "github.com/prometheus/client_golang/prometheus"

const namespace = "github_runner"

// Metrics holds every Prometheus collector used by the runner. Obtain an
// instance via [NewMetrics]; do not construct one manually.
type Metrics struct {
	// Registry is the custom registry that owns all collectors below.
	Registry *prometheus.Registry

	// JobsTotal counts completed jobs partitioned by runner, status, and repo.
	JobsTotal *prometheus.CounterVec

	// JobDurationSeconds records the wall-clock duration of each job.
	JobDurationSeconds *prometheus.HistogramVec

	// JobsActive tracks the number of jobs currently being executed.
	JobsActive *prometheus.GaugeVec

	// JobErrorsTotal counts job-level errors by type.
	JobErrorsTotal *prometheus.CounterVec

	// CacheHitRatio exposes the ratio of cache hits to total lookups.
	CacheHitRatio *prometheus.GaugeVec

	// CacheOperationDuration records the latency of cache operations.
	CacheOperationDuration *prometheus.HistogramVec

	// ExecutorPrepareDuration records how long it takes to prepare an
	// executor environment (pull image, create VM, etc.).
	ExecutorPrepareDuration *prometheus.HistogramVec

	// PollDuration records the latency of each poll cycle against the
	// GitHub API.
	PollDuration *prometheus.HistogramVec

	// PollErrorsTotal counts errors encountered while polling for jobs.
	PollErrorsTotal *prometheus.CounterVec

	// HeartbeatErrorsTotal counts failed heartbeat attempts.
	HeartbeatErrorsTotal *prometheus.CounterVec

	// StepDurationSeconds records the wall-clock duration of individual
	// workflow steps.
	StepDurationSeconds *prometheus.HistogramVec
}

// NewMetrics creates a full set of runner metrics registered on a private
// [prometheus.Registry]. The returned [Metrics] value is ready to use.
func NewMetrics() *Metrics {
	reg := prometheus.NewRegistry()

	m := &Metrics{
		Registry: reg,

		JobsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "jobs_total",
			Help:      "Total number of jobs executed, partitioned by runner, status, and repository.",
		}, []string{"runner", "status", "repository"}),

		JobDurationSeconds: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "job_duration_seconds",
			Help:      "Wall-clock duration of jobs in seconds.",
			Buckets:   prometheus.ExponentialBuckets(1, 2, 14), // 1s .. ~4.5h
		}, []string{"runner", "status"}),

		JobsActive: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "jobs_active",
			Help:      "Number of jobs currently being executed.",
		}, []string{"runner"}),

		JobErrorsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "job_errors_total",
			Help:      "Total number of job-level errors by type.",
		}, []string{"runner", "error_type"}),

		CacheHitRatio: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "cache_hit_ratio",
			Help:      "Ratio of cache hits to total lookups.",
		}, []string{"runner", "cache_type"}),

		CacheOperationDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "cache_operation_duration_seconds",
			Help:      "Latency of cache operations in seconds.",
			Buckets:   prometheus.DefBuckets,
		}, []string{"runner", "operation"}),

		ExecutorPrepareDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "executor_prepare_duration_seconds",
			Help:      "Time to prepare an executor environment in seconds.",
			Buckets:   prometheus.ExponentialBuckets(0.5, 2, 12), // 0.5s .. ~17m
		}, []string{"runner", "executor"}),

		PollDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "poll_duration_seconds",
			Help:      "Latency of each poll cycle in seconds.",
			Buckets:   prometheus.DefBuckets,
		}, []string{"runner"}),

		PollErrorsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "poll_errors_total",
			Help:      "Total number of poll errors by type.",
		}, []string{"runner", "error_type"}),

		HeartbeatErrorsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "heartbeat_errors_total",
			Help:      "Total number of failed heartbeat attempts.",
		}, []string{"runner"}),

		StepDurationSeconds: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "step_duration_seconds",
			Help:      "Wall-clock duration of individual workflow steps in seconds.",
			Buckets:   prometheus.ExponentialBuckets(0.1, 2, 16), // 0.1s .. ~1.8h
		}, []string{"runner", "status"}),
	}

	// Register every collector with the private registry.
	reg.MustRegister(
		m.JobsTotal,
		m.JobDurationSeconds,
		m.JobsActive,
		m.JobErrorsTotal,
		m.CacheHitRatio,
		m.CacheOperationDuration,
		m.ExecutorPrepareDuration,
		m.PollDuration,
		m.PollErrorsTotal,
		m.HeartbeatErrorsTotal,
		m.StepDurationSeconds,
	)

	return m
}
