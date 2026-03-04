// Package kubernetes (pod.go) contains the pod specification builder for the
// Kubernetes executor. The BuildPodSpec function constructs a Kubernetes pod
// specification for running a single job step.
//
// Pod configuration strategy:
//
// Each job step maps to a single-container pod. The pod spec is built with
// the following considerations:
//
//   - Image: Determined by the step configuration or the executor's default.
//   - Command: The step's shell command is wrapped in a shell invocation
//     (e.g., ["sh", "-e", "-c", "..."]).
//   - Environment: Job-level and step-level environment variables are merged
//     and set on the container spec. Secrets are injected via Kubernetes
//     Secret references rather than plaintext.
//   - Resources: CPU and memory requests/limits are derived from the executor
//     configuration and can be overridden per-step.
//   - Volumes: A shared workspace volume (emptyDir or PVC) is mounted at
//     /workspace. Additional volumes can be configured for caching, tool
//     binaries, etc.
//   - Security: The pod runs as a non-root user by default, with
//     readOnlyRootFilesystem where feasible. Privileged mode is never enabled.
//   - Scheduling: NodeSelector, tolerations, and affinity rules from the
//     configuration are applied to ensure pods land on appropriate nodes.
//   - Metadata: Labels and annotations from the configuration are applied,
//     along with standard labels for job ID, step ID, and runner identity
//     to enable observability and cleanup.
//   - Timeouts: activeDeadlineSeconds is set based on the step's timeout
//     configuration to ensure runaway pods are terminated.
//   - Restart policy: Always set to "Never" since steps should not be retried
//     automatically.
package kubernetes

import (
	"fmt"

	"github.com/org/github-runner/pkg/api"
)

// PodSpec is a placeholder type representing a Kubernetes pod specification.
// A full implementation would use corev1.PodSpec from k8s.io/api/core/v1.
type PodSpec struct {
	// Name is the pod name, derived from the job ID and step ID.
	Name string
	// Namespace is the target Kubernetes namespace.
	Namespace string
	// Image is the container image to use.
	Image string
	// Command is the entrypoint command for the container.
	Command []string
	// Env is the list of environment variables.
	Env map[string]string
	// CPURequest is the CPU resource request.
	CPURequest string
	// CPULimit is the CPU resource limit.
	CPULimit string
	// MemoryRequest is the memory resource request.
	MemoryRequest string
	// MemoryLimit is the memory resource limit.
	MemoryLimit string
	// ServiceAccountName is the Kubernetes service account.
	ServiceAccountName string
	// NodeSelector constrains pod scheduling.
	NodeSelector map[string]string
	// Labels are applied to the pod metadata.
	Labels map[string]string
	// Annotations are applied to the pod metadata.
	Annotations map[string]string
	// ActiveDeadlineSeconds is the pod timeout in seconds.
	ActiveDeadlineSeconds *int64
}

// BuildPodSpec constructs a PodSpec for executing the given step within the
// context of the provided job and executor configuration.
//
// This is a stub implementation that returns a partially populated PodSpec.
// A full implementation would:
//   - Use corev1.PodSpec from k8s.io/api/core/v1.
//   - Configure volume mounts for the shared workspace.
//   - Set up init containers for repository checkout.
//   - Apply security contexts (runAsNonRoot, readOnlyRootFilesystem, etc.).
//   - Configure image pull secrets from the executor config.
//   - Set tolerations and affinity rules.
//   - Inject secrets via Kubernetes Secret volume mounts or env references.
func BuildPodSpec(cfg KubernetesConfig, job *api.Job, step *api.Step) PodSpec {
	// Determine the image.
	img := cfg.Image
	if step.Uses != "" {
		img = step.Uses
	}

	// Build the command.
	var cmd []string
	if step.Run != "" {
		shell := "sh"
		if step.Shell != "" {
			shell = step.Shell
		}
		cmd = []string{shell, "-e", "-c", step.Run}
	}

	// Merge environment variables (job env + step env).
	env := make(map[string]string)
	for k, v := range job.Env {
		env[k] = v
	}
	for k, v := range step.Env {
		env[k] = v
	}
	env["GITHUB_WORKSPACE"] = "/workspace"
	env["GITHUB_REPOSITORY"] = job.Repository
	env["GITHUB_REF"] = job.Ref
	env["GITHUB_SHA"] = job.SHA

	// Calculate the active deadline.
	var deadline *int64
	if step.TimeoutMinutes > 0 {
		secs := int64(step.TimeoutMinutes * 60)
		deadline = &secs
	}

	// Build labels with standard identifiers.
	labels := make(map[string]string)
	for k, v := range cfg.Labels {
		labels[k] = v
	}
	labels["github-runner/job-id"] = fmt.Sprintf("%d", job.ID)
	labels["github-runner/step-id"] = step.ID

	// Build annotations.
	annotations := make(map[string]string)
	for k, v := range cfg.Annotations {
		annotations[k] = v
	}

	return PodSpec{
		Name:                  fmt.Sprintf("ghrunner-%d-%s", job.ID, step.ID),
		Namespace:             cfg.Namespace,
		Image:                 img,
		Command:               cmd,
		Env:                   env,
		CPURequest:            cfg.CPURequest,
		CPULimit:              cfg.CPULimit,
		MemoryRequest:         cfg.MemoryRequest,
		MemoryLimit:           cfg.MemoryLimit,
		ServiceAccountName:    cfg.ServiceAccountName,
		NodeSelector:          cfg.NodeSelector,
		Labels:                labels,
		Annotations:           annotations,
		ActiveDeadlineSeconds: deadline,
	}
}
