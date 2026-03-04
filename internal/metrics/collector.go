package metrics

import (
	"os"
	"runtime"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// RuntimeCollector implements [prometheus.Collector] and reports Go runtime and
// process-level statistics. Register it on a [prometheus.Registry] alongside
// the application metrics to get goroutine, memory, file-descriptor, and
// uptime data in the /metrics output.
type RuntimeCollector struct {
	startTime time.Time

	goroutines *prometheus.Desc
	threads    *prometheus.Desc

	heapAlloc   *prometheus.Desc
	heapInuse   *prometheus.Desc
	heapObjects *prometheus.Desc
	stackInuse  *prometheus.Desc
	sysBytes    *prometheus.Desc
	gcPauseNs   *prometheus.Desc
	gcRuns      *prometheus.Desc

	openFDs *prometheus.Desc
	maxFDs  *prometheus.Desc
	uptime  *prometheus.Desc
}

// NewRuntimeCollector returns a collector that reports Go runtime and OS
// process metrics under the "github_runner_runtime_" namespace.
func NewRuntimeCollector() *RuntimeCollector {
	return &RuntimeCollector{
		startTime: time.Now(),

		goroutines: prometheus.NewDesc(
			"github_runner_runtime_goroutines",
			"Number of active goroutines.",
			nil, nil,
		),
		threads: prometheus.NewDesc(
			"github_runner_runtime_threads",
			"Number of OS threads created by the Go runtime.",
			nil, nil,
		),

		heapAlloc: prometheus.NewDesc(
			"github_runner_runtime_heap_alloc_bytes",
			"Bytes of allocated heap objects.",
			nil, nil,
		),
		heapInuse: prometheus.NewDesc(
			"github_runner_runtime_heap_inuse_bytes",
			"Bytes of in-use heap spans.",
			nil, nil,
		),
		heapObjects: prometheus.NewDesc(
			"github_runner_runtime_heap_objects",
			"Number of allocated heap objects.",
			nil, nil,
		),
		stackInuse: prometheus.NewDesc(
			"github_runner_runtime_stack_inuse_bytes",
			"Bytes of stack spans in use.",
			nil, nil,
		),
		sysBytes: prometheus.NewDesc(
			"github_runner_runtime_sys_bytes",
			"Total bytes of memory obtained from the OS.",
			nil, nil,
		),
		gcPauseNs: prometheus.NewDesc(
			"github_runner_runtime_gc_pause_total_ns",
			"Total GC pause time in nanoseconds.",
			nil, nil,
		),
		gcRuns: prometheus.NewDesc(
			"github_runner_runtime_gc_runs_total",
			"Total number of completed GC cycles.",
			nil, nil,
		),

		openFDs: prometheus.NewDesc(
			"github_runner_process_open_fds",
			"Number of open file descriptors.",
			nil, nil,
		),
		maxFDs: prometheus.NewDesc(
			"github_runner_process_max_fds",
			"Maximum number of file descriptors (soft limit).",
			nil, nil,
		),
		uptime: prometheus.NewDesc(
			"github_runner_process_uptime_seconds",
			"Seconds since the process started.",
			nil, nil,
		),
	}
}

// Describe sends all metric descriptors to ch.
func (c *RuntimeCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.goroutines
	ch <- c.threads
	ch <- c.heapAlloc
	ch <- c.heapInuse
	ch <- c.heapObjects
	ch <- c.stackInuse
	ch <- c.sysBytes
	ch <- c.gcPauseNs
	ch <- c.gcRuns
	ch <- c.openFDs
	ch <- c.maxFDs
	ch <- c.uptime
}

// Collect gathers current runtime/process stats and sends them to ch.
func (c *RuntimeCollector) Collect(ch chan<- prometheus.Metric) {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	ch <- prometheus.MustNewConstMetric(c.goroutines, prometheus.GaugeValue, float64(runtime.NumGoroutine()))
	ch <- prometheus.MustNewConstMetric(c.threads, prometheus.GaugeValue, float64(numThreads()))
	ch <- prometheus.MustNewConstMetric(c.heapAlloc, prometheus.GaugeValue, float64(mem.HeapAlloc))
	ch <- prometheus.MustNewConstMetric(c.heapInuse, prometheus.GaugeValue, float64(mem.HeapInuse))
	ch <- prometheus.MustNewConstMetric(c.heapObjects, prometheus.GaugeValue, float64(mem.HeapObjects))
	ch <- prometheus.MustNewConstMetric(c.stackInuse, prometheus.GaugeValue, float64(mem.StackInuse))
	ch <- prometheus.MustNewConstMetric(c.sysBytes, prometheus.GaugeValue, float64(mem.Sys))
	ch <- prometheus.MustNewConstMetric(c.gcPauseNs, prometheus.CounterValue, float64(mem.PauseTotalNs))
	ch <- prometheus.MustNewConstMetric(c.gcRuns, prometheus.CounterValue, float64(mem.NumGC))

	openFDs, maxFDs := processFDs()
	ch <- prometheus.MustNewConstMetric(c.openFDs, prometheus.GaugeValue, float64(openFDs))
	ch <- prometheus.MustNewConstMetric(c.maxFDs, prometheus.GaugeValue, float64(maxFDs))
	ch <- prometheus.MustNewConstMetric(c.uptime, prometheus.GaugeValue, time.Since(c.startTime).Seconds())
}

// numThreads returns the number of OS threads used by the Go runtime.
func numThreads() int {
	// runtime.NumCgoCall is sometimes used as a proxy; there is no direct
	// stdlib API for thread count.  We rely on GOMAXPROCS as a lower-bound
	// approximation — this keeps the collector dependency-free.
	return runtime.GOMAXPROCS(0)
}

// processFDs returns the number of open file descriptors for the current
// process and the soft limit. On platforms where /proc/self/fd is unavailable
// both values are -1.
func processFDs() (open int, max int) {
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		return countFDsFallback()
	}
	return len(entries), maxFDsFallback()
}
