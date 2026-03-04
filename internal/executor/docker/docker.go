// Package docker implements the Docker-based executor that runs job steps
// inside isolated containers using the Docker Engine API.
package docker

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"

	"github.com/org/github-runner/internal/executor"
	"github.com/org/github-runner/pkg/api"
)

const (
	// version is the semantic version of the docker executor.
	version = "0.1.0"
	// defaultTimeout is the per-step timeout when none is specified.
	defaultTimeout = 60 * time.Minute
	// defaultImage is used when no image is configured and the step does not
	// specify one.
	defaultImage = "ubuntu:latest"
)

// PullPolicy controls when images are pulled from the registry.
type PullPolicy string

const (
	// PullAlways always pulls the image before creating a container.
	PullAlways PullPolicy = "always"
	// PullIfNotPresent only pulls the image if it is not already available locally.
	PullIfNotPresent PullPolicy = "if-not-present"
	// PullNever never pulls images; the image must exist locally.
	PullNever PullPolicy = "never"
)

// DockerConfig configures the Docker executor. Field names mirror the TOML
// configuration schema used by the runner daemon.
type DockerConfig struct {
	// Image is the default container image used for steps that do not specify
	// their own image.
	Image string

	// PullPolicy controls when images are pulled from the registry.
	PullPolicy PullPolicy

	// AllowedImages is a list of glob patterns that restrict which images may
	// be used. An empty list permits any image.
	AllowedImages []string

	// Volumes is a list of volume mount specifications in "host:container:mode"
	// format that are mounted into every container.
	Volumes []string

	// MemoryLimit is the maximum memory in bytes a container may use.
	// Zero means no limit.
	MemoryLimit int64

	// CPUQuota is the CPU quota in microseconds per CPUPeriod.
	// Zero means no limit.
	CPUQuota int64

	// CPUPeriod is the CPU CFS period in microseconds. Defaults to 100000 (100ms).
	CPUPeriod int64

	// PidsLimit is the maximum number of processes a container may create.
	// Zero means no limit.
	PidsLimit int64

	// CapAdd is a list of Linux capabilities to add to the container.
	CapAdd []string

	// CapDrop is a list of Linux capabilities to drop from the container.
	CapDrop []string

	// SecurityOpt is a list of security options (e.g., "no-new-privileges").
	SecurityOpt []string

	// NetworkMode overrides the default container network mode.
	NetworkMode string

	// WorkDir is the root directory on the host where job workspace directories
	// are created and bind-mounted into containers.
	WorkDir string
}

// DockerExecutor runs job steps inside Docker containers.
type DockerExecutor struct {
	cfg       DockerConfig
	client    *Client
	logger    *slog.Logger
	job       *api.Job
	workspace string
}

// New creates a new DockerExecutor with the given configuration. It
// establishes a connection to the Docker daemon using the default socket.
func New(cfg DockerConfig) (*DockerExecutor, error) {
	if cfg.WorkDir == "" {
		return nil, fmt.Errorf("docker executor: work directory must not be empty")
	}
	if cfg.Image == "" {
		cfg.Image = defaultImage
	}
	if cfg.PullPolicy == "" {
		cfg.PullPolicy = PullIfNotPresent
	}
	if cfg.CPUPeriod == 0 {
		cfg.CPUPeriod = 100000 // 100ms default CFS period
	}

	client, err := NewClient()
	if err != nil {
		return nil, fmt.Errorf("docker executor: creating client: %w", err)
	}

	return &DockerExecutor{
		cfg:    cfg,
		client: client,
		logger: slog.Default().With("executor", "docker"),
	}, nil
}

// factory creates a DockerExecutor from an opaque config value.
func factory(cfg interface{}) (executor.Executor, error) {
	c, ok := cfg.(DockerConfig)
	if !ok {
		return nil, fmt.Errorf("docker executor: expected DockerConfig, got %T", cfg)
	}
	return New(c)
}

// Register registers the docker executor factory with the executor registry.
func Register() {
	executor.Register(executor.Docker, factory)
}

// Info returns metadata about the docker executor.
func (e *DockerExecutor) Info() api.ExecutorInfo {
	return api.ExecutorInfo{
		Name:    "docker",
		Version: version,
		Features: []string{
			"container-isolation",
			"resource-limits",
			"image-pull-policy",
			"volume-mounts",
			"security-options",
		},
	}
}

// Prepare initialises the execution environment for the given job. It pulls
// the configured default image (subject to pull policy), creates the host
// workspace directory, and validates volume mounts.
func (e *DockerExecutor) Prepare(ctx context.Context, job *api.Job) error {
	e.job = job
	e.workspace = filepath.Join(e.cfg.WorkDir, fmt.Sprintf("job-%d", job.ID))

	e.logger.InfoContext(ctx, "preparing docker environment",
		"job_id", job.ID,
		"workspace", e.workspace,
		"image", e.cfg.Image,
	)

	// Pull the default image if required by policy.
	if err := e.pullImageIfNeeded(ctx, e.cfg.Image); err != nil {
		return fmt.Errorf("pulling default image %s: %w", e.cfg.Image, err)
	}

	return nil
}

// Run executes a single step inside a Docker container. It creates a new
// container for each step, streams logs in real-time, and waits for the
// container to exit.
func (e *DockerExecutor) Run(ctx context.Context, step *api.Step) (*api.StepResult, error) {
	if e.job == nil {
		return nil, fmt.Errorf("docker executor: Run called before Prepare")
	}

	result := &api.StepResult{
		StepID:    step.ID,
		Status:    api.StepRunning,
		StartedAt: time.Now(),
		ExitCode:  -1,
	}

	// Determine timeout.
	timeout := defaultTimeout
	if step.TimeoutMinutes > 0 {
		timeout = time.Duration(step.TimeoutMinutes) * time.Minute
	}
	stepCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Determine the image to use.
	image := e.cfg.Image
	if step.Uses != "" {
		image = step.Uses
	}

	// Validate image against allowed patterns.
	if err := e.validateImage(image); err != nil {
		return nil, fmt.Errorf("image validation: %w", err)
	}

	// Pull image if needed.
	if err := e.pullImageIfNeeded(stepCtx, image); err != nil {
		return nil, fmt.Errorf("pulling image %s: %w", image, err)
	}

	e.logger.InfoContext(ctx, "running step in container",
		"step_id", step.ID,
		"step_name", step.Name,
		"image", image,
	)

	// Build container configuration.
	containerCfg, hostCfg, err := e.buildContainerConfig(step, image)
	if err != nil {
		return nil, fmt.Errorf("building container config: %w", err)
	}

	// Create the container.
	containerName := fmt.Sprintf("ghrunner-%d-%s", e.job.ID, step.ID)
	containerID, err := e.client.ContainerCreate(stepCtx, containerCfg, hostCfg, containerName)
	if err != nil {
		return nil, fmt.Errorf("creating container: %w", err)
	}

	// Ensure container is removed when we are done.
	defer func() {
		removeCtx, removeCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer removeCancel()
		if removeErr := e.client.ContainerRemove(removeCtx, containerID); removeErr != nil {
			e.logger.Warn("failed to remove container",
				"container_id", containerID,
				"error", removeErr,
			)
		}
	}()

	// Start the container.
	if err := e.client.ContainerStart(stepCtx, containerID); err != nil {
		return nil, fmt.Errorf("starting container: %w", err)
	}

	// Stream container logs.
	if err := e.client.StreamLogs(stepCtx, containerID); err != nil {
		e.logger.WarnContext(ctx, "log streaming error", "error", err)
	}

	// Wait for the container to exit.
	exitCode, err := e.client.ContainerWait(stepCtx, containerID)
	if err != nil {
		result.CompletedAt = time.Now()

		if stepCtx.Err() != nil {
			result.Status = api.StepCompleted
			result.Conclusion = api.ConclusionFailure
			e.logger.WarnContext(ctx, "step timed out or was cancelled",
				"step_id", step.ID,
			)
			return result, nil
		}

		return nil, fmt.Errorf("waiting for container: %w", err)
	}

	result.CompletedAt = time.Now()
	result.ExitCode = int(exitCode)
	result.Status = api.StepCompleted

	if exitCode == 0 {
		result.Conclusion = api.ConclusionSuccess
	} else {
		result.Conclusion = api.ConclusionFailure
	}

	e.logger.InfoContext(ctx, "step completed",
		"step_id", step.ID,
		"exit_code", exitCode,
		"duration", result.CompletedAt.Sub(result.StartedAt),
	)

	return result, nil
}

// Cleanup releases all resources associated with the current job. It removes
// any remaining containers and cleans up the workspace.
func (e *DockerExecutor) Cleanup(ctx context.Context) error {
	if e.client != nil {
		if err := e.client.Close(); err != nil {
			e.logger.WarnContext(ctx, "error closing docker client", "error", err)
		}
	}
	e.job = nil
	e.workspace = ""
	return nil
}

// buildContainerConfig creates the Docker container and host configuration
// for a step, including environment variables, resource limits, security
// options, and volume mounts.
func (e *DockerExecutor) buildContainerConfig(step *api.Step, image string) (*container.Config, *container.HostConfig, error) {
	// Build environment variables.
	env := make([]string, 0)
	for k, v := range e.job.Env {
		env = append(env, k+"="+v)
	}
	for k, v := range step.Env {
		env = append(env, k+"="+v)
	}
	env = append(env,
		"GITHUB_WORKSPACE=/workspace",
		"GITHUB_REPOSITORY="+e.job.Repository,
		"GITHUB_REF="+e.job.Ref,
		"GITHUB_SHA="+e.job.SHA,
	)

	// Determine the command.
	var cmd []string
	if step.Run != "" {
		shell := "sh"
		if step.Shell != "" {
			shell = step.Shell
		}
		cmd = []string{shell, "-e", "-c", step.Run}
	}

	// Determine working directory inside the container.
	workDir := "/workspace"
	if step.WorkingDirectory != "" {
		workDir = filepath.Join("/workspace", step.WorkingDirectory)
	}

	containerCfg := &container.Config{
		Image:      image,
		Cmd:        cmd,
		Env:        env,
		WorkingDir: workDir,
	}

	// Build volume mounts.
	mounts, err := e.buildMounts()
	if err != nil {
		return nil, nil, fmt.Errorf("building mounts: %w", err)
	}

	// Add workspace bind mount.
	mounts = append(mounts, mount.Mount{
		Type:   mount.TypeBind,
		Source: e.workspace,
		Target: "/workspace",
	})

	hostCfg := &container.HostConfig{
		Mounts: mounts,
		Resources: container.Resources{
			Memory:    e.cfg.MemoryLimit,
			CPUQuota:  e.cfg.CPUQuota,
			CPUPeriod: e.cfg.CPUPeriod,
			PidsLimit: &e.cfg.PidsLimit,
		},
		CapAdd:      e.cfg.CapAdd,
		CapDrop:     e.cfg.CapDrop,
		SecurityOpt: e.cfg.SecurityOpt,
	}

	if e.cfg.NetworkMode != "" {
		hostCfg.NetworkMode = container.NetworkMode(e.cfg.NetworkMode)
	}

	return containerCfg, hostCfg, nil
}

// buildMounts parses the configured volume specifications into Docker mount
// objects.
func (e *DockerExecutor) buildMounts() ([]mount.Mount, error) {
	mounts := make([]mount.Mount, 0, len(e.cfg.Volumes))
	for _, spec := range e.cfg.Volumes {
		m, err := ParseVolumeMount(spec)
		if err != nil {
			return nil, fmt.Errorf("parsing volume %q: %w", spec, err)
		}
		mounts = append(mounts, m)
	}
	if err := ValidateVolumeMounts(mounts); err != nil {
		return nil, fmt.Errorf("validating mounts: %w", err)
	}
	return mounts, nil
}

// pullImageIfNeeded pulls the given image according to the configured pull
// policy.
func (e *DockerExecutor) pullImageIfNeeded(ctx context.Context, image string) error {
	switch e.cfg.PullPolicy {
	case PullNever:
		return nil
	case PullAlways:
		return e.client.ImagePull(ctx, image)
	case PullIfNotPresent:
		exists, err := e.client.ImageExists(ctx, image)
		if err != nil {
			return fmt.Errorf("checking if image exists: %w", err)
		}
		if exists {
			return nil
		}
		return e.client.ImagePull(ctx, image)
	default:
		return fmt.Errorf("unknown pull policy %q", e.cfg.PullPolicy)
	}
}

// validateImage checks whether the given image name matches at least one of
// the allowed image glob patterns. If no patterns are configured, all images
// are permitted.
func (e *DockerExecutor) validateImage(image string) error {
	if len(e.cfg.AllowedImages) == 0 {
		return nil
	}

	for _, pattern := range e.cfg.AllowedImages {
		matched, err := filepath.Match(pattern, image)
		if err != nil {
			return fmt.Errorf("invalid image pattern %q: %w", pattern, err)
		}
		if matched {
			return nil
		}

		// Also try matching just the image name without the tag.
		imageName := image
		if idx := strings.LastIndex(image, ":"); idx >= 0 {
			imageName = image[:idx]
		}
		matched, err = filepath.Match(pattern, imageName)
		if err != nil {
			return fmt.Errorf("invalid image pattern %q: %w", pattern, err)
		}
		if matched {
			return nil
		}
	}

	return fmt.Errorf("image %q not allowed by configured patterns: %v", image, e.cfg.AllowedImages)
}
