// Package kubernetes implements a Kubernetes-based executor that runs job
// steps as pods in a Kubernetes cluster. This is currently a scaffold
// implementation with stub methods; the full implementation would use
// client-go to manage pod lifecycles.
//
// Architecture overview:
//
// The Kubernetes executor creates one pod per job. Each step runs as an init
// container or a main container within that pod, sharing a workspace volume.
// The executor streams pod logs in real-time and enforces timeouts by setting
// activeDeadlineSeconds on the pod spec.
//
// A full implementation would:
//   - Use client-go to connect to the cluster (in-cluster or kubeconfig).
//   - Create a PersistentVolumeClaim or emptyDir for the shared workspace.
//   - Build a pod spec with init containers for setup steps and a main container
//     for the primary step.
//   - Apply resource requests/limits, node selectors, tolerations, and affinity
//     rules from the runner configuration.
//   - Watch the pod until completion, streaming logs via the Kubernetes log API.
//   - Clean up the pod and any associated PVCs on Cleanup().
//   - Support service containers (sidecars) for databases, caches, etc.
package kubernetes

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/nficano/github-runner/internal/executor"
	"github.com/nficano/github-runner/pkg/api"
)

const (
	// version is the semantic version of the kubernetes executor.
	version = "0.1.0"
)

// KubernetesConfig configures the Kubernetes executor.
type KubernetesConfig struct {
	// Namespace is the Kubernetes namespace in which pods are created.
	// Defaults to "default" if empty.
	Namespace string

	// Image is the default container image for steps that do not specify one.
	Image string

	// ServiceAccountName is the Kubernetes service account to assign to job pods.
	ServiceAccountName string

	// NodeSelector constrains pods to nodes matching the given labels.
	NodeSelector map[string]string

	// CPURequest is the CPU resource request for step containers (e.g., "500m").
	CPURequest string

	// CPULimit is the CPU resource limit for step containers (e.g., "2").
	CPULimit string

	// MemoryRequest is the memory resource request (e.g., "256Mi").
	MemoryRequest string

	// MemoryLimit is the memory resource limit (e.g., "1Gi").
	MemoryLimit string

	// ImagePullSecrets is a list of secret names used for private registry
	// authentication.
	ImagePullSecrets []string

	// Annotations are additional annotations to apply to job pods.
	Annotations map[string]string

	// Labels are additional labels to apply to job pods.
	Labels map[string]string
}

// KubernetesExecutor runs job steps as Kubernetes pods. This is a scaffold
// implementation; all methods return ErrNotImplemented.
type KubernetesExecutor struct {
	cfg    KubernetesConfig
	logger *slog.Logger
	job    *api.Job
}

// ErrNotImplemented is returned by all stub methods.
var ErrNotImplemented = fmt.Errorf("kubernetes executor: not yet implemented")

// New creates a new KubernetesExecutor with the given configuration.
// A full implementation would establish a connection to the Kubernetes API
// server here using client-go, either via in-cluster config or a kubeconfig
// file.
func New(cfg KubernetesConfig) (*KubernetesExecutor, error) {
	if cfg.Namespace == "" {
		cfg.Namespace = "default"
	}

	return &KubernetesExecutor{
		cfg:    cfg,
		logger: slog.Default().With("executor", "kubernetes"),
	}, nil
}

// factory creates a KubernetesExecutor from an opaque config value.
func factory(cfg interface{}) (executor.Executor, error) {
	c, ok := cfg.(KubernetesConfig)
	if !ok {
		return nil, fmt.Errorf("kubernetes executor: expected KubernetesConfig, got %T", cfg)
	}
	return New(c)
}

// Register registers the kubernetes executor factory with the executor
// registry.
func Register() {
	executor.Register(executor.Kubernetes, factory)
}

// Info returns metadata about the kubernetes executor.
func (e *KubernetesExecutor) Info() api.ExecutorInfo {
	return api.ExecutorInfo{
		Name:    "kubernetes",
		Version: version,
		Features: []string{
			"pod-per-job",
			"resource-limits",
			"service-containers",
			"node-selection",
		},
	}
}

// Prepare initialises the Kubernetes execution environment for the given job.
//
// A full implementation would:
//   - Validate that the target namespace exists and the service account has
//     appropriate permissions.
//   - Create a PersistentVolumeClaim for the shared workspace, or configure
//     an emptyDir volume.
//   - Pull secrets for private registries.
//   - Create any required ConfigMaps for job/step environment variables.
func (e *KubernetesExecutor) Prepare(ctx context.Context, job *api.Job) error {
	e.job = job
	e.logger.InfoContext(ctx, "kubernetes executor: Prepare is a stub",
		"job_id", job.ID,
		"namespace", e.cfg.Namespace,
	)
	return ErrNotImplemented
}

// Run executes a single step as a container within the job's Kubernetes pod.
//
// A full implementation would:
//   - Build a pod spec using BuildPodSpec (see pod.go) with the step's
//     command, environment, image, and resource requirements.
//   - Create the pod via the Kubernetes API.
//   - Watch the pod for phase transitions (Pending -> Running -> Succeeded/Failed).
//   - Stream pod logs in real-time using the Kubernetes log API.
//   - Enforce the step timeout via context cancellation and
//     activeDeadlineSeconds.
//   - Collect the container exit code from the pod status.
//   - Delete the pod after execution completes.
func (e *KubernetesExecutor) Run(ctx context.Context, step *api.Step) (*api.StepResult, error) {
	e.logger.InfoContext(ctx, "kubernetes executor: Run is a stub",
		"step_id", step.ID,
		"step_name", step.Name,
	)
	return nil, ErrNotImplemented
}

// Cleanup releases all Kubernetes resources associated with the current job.
//
// A full implementation would:
//   - Delete the job's pod (if still running, with a grace period).
//   - Delete any PersistentVolumeClaims created for the workspace.
//   - Clean up ConfigMaps and Secrets created for the job.
//   - Ensure no orphaned resources remain in the namespace.
func (e *KubernetesExecutor) Cleanup(ctx context.Context) error {
	e.logger.InfoContext(ctx, "kubernetes executor: Cleanup is a stub")
	e.job = nil
	return nil
}
