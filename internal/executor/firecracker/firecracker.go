// Package firecracker implements a Firecracker-based executor that runs job
// steps inside lightweight microVMs for strong isolation with near-native
// performance.
//
// Architecture overview:
//
// Firecracker is a virtual machine monitor (VMM) developed by AWS that creates
// and manages microVMs. Each microVM boots in less than 125ms and provides
// hardware-level isolation using KVM, making it ideal for running untrusted
// workloads.
//
// The Firecracker executor would:
//
//  1. Provision a microVM per job using a pre-built root filesystem image that
//     contains the tools needed for CI/CD (git, compilers, runtimes, etc.).
//
//  2. Configure the VM with constrained resources (vCPUs, memory) based on the
//     runner configuration and the job's requirements.
//
//  3. Mount the workspace into the VM using a virtio-blk or virtio-fs device,
//     enabling the job to access repository files.
//
//  4. Execute step commands inside the VM via a lightweight agent process
//     running within the guest OS. The agent communicates with the host over
//     a VSOCK (virtio socket) connection, receiving commands and streaming
//     stdout/stderr back to the host.
//
//  5. Enforce timeouts by sending SIGTERM/SIGKILL to the VM process or by
//     using Firecracker's built-in shutdown mechanism.
//
//  6. Clean up by stopping the VM and removing its resources (socket files,
//     log files, drive images) upon job completion.
//
// Key benefits:
//   - Strong isolation: Each job runs in its own VM with a separate kernel.
//   - Fast boot: microVMs start in ~125ms, adding minimal overhead.
//   - Resource control: CPU and memory limits are enforced at the hypervisor level.
//   - Security: No shared kernel surface between jobs, unlike containers.
//
// Prerequisites for a full implementation:
//   - Linux host with KVM support (/dev/kvm).
//   - Firecracker binary installed and accessible.
//   - Pre-built kernel (vmlinux) and root filesystem (rootfs.ext4) images.
//   - The firecracker-go-sdk (github.com/firecracker-microvm/firecracker-go-sdk).
package firecracker

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/nficano/github-runner/internal/executor"
	"github.com/nficano/github-runner/pkg/api"
)

const (
	// version is the semantic version of the firecracker executor.
	version = "0.1.0"
)

// FirecrackerConfig configures the Firecracker executor.
type FirecrackerConfig struct {
	// FirecrackerBinary is the path to the firecracker binary.
	// Defaults to "firecracker" (resolved via PATH).
	FirecrackerBinary string

	// KernelImage is the path to the uncompressed Linux kernel image (vmlinux).
	KernelImage string

	// RootDrive is the path to the root filesystem image (ext4).
	RootDrive string

	// VCPUCount is the number of virtual CPUs to allocate to each microVM.
	// Defaults to 2.
	VCPUCount int

	// MemSizeMib is the amount of memory in MiB to allocate to each microVM.
	// Defaults to 1024.
	MemSizeMib int

	// WorkDir is the host directory where VM-related files (sockets, logs)
	// are created.
	WorkDir string

	// NetworkInterface configures the network interface for the microVM.
	// Empty means no network.
	NetworkInterface string

	// KernelArgs are additional kernel boot arguments.
	KernelArgs string
}

// FirecrackerExecutor runs job steps inside Firecracker microVMs. This is a
// scaffold implementation; all methods return ErrNotImplemented.
type FirecrackerExecutor struct {
	cfg    FirecrackerConfig
	logger *slog.Logger
	job    *api.Job
}

// ErrNotImplemented is returned by all stub methods.
var ErrNotImplemented = fmt.Errorf("firecracker executor: not yet implemented")

// New creates a new FirecrackerExecutor with the given configuration.
//
// A full implementation would:
//   - Validate that /dev/kvm is accessible.
//   - Verify that the firecracker binary exists and is executable.
//   - Validate that the kernel image and root filesystem exist.
//   - Prepare the work directory for VM socket and log files.
func New(cfg FirecrackerConfig) (*FirecrackerExecutor, error) {
	if cfg.FirecrackerBinary == "" {
		cfg.FirecrackerBinary = "firecracker"
	}
	if cfg.VCPUCount == 0 {
		cfg.VCPUCount = 2
	}
	if cfg.MemSizeMib == 0 {
		cfg.MemSizeMib = 1024
	}

	return &FirecrackerExecutor{
		cfg:    cfg,
		logger: slog.Default().With("executor", "firecracker"),
	}, nil
}

// factory creates a FirecrackerExecutor from an opaque config value.
func factory(cfg interface{}) (executor.Executor, error) {
	c, ok := cfg.(FirecrackerConfig)
	if !ok {
		return nil, fmt.Errorf("firecracker executor: expected FirecrackerConfig, got %T", cfg)
	}
	return New(c)
}

// Register registers the firecracker executor factory with the executor
// registry.
func Register() {
	executor.Register(executor.Firecracker, factory)
}

// Info returns metadata about the firecracker executor.
func (e *FirecrackerExecutor) Info() api.ExecutorInfo {
	return api.ExecutorInfo{
		Name:    "firecracker",
		Version: version,
		Features: []string{
			"microvm-isolation",
			"hardware-level-security",
			"fast-boot",
			"resource-limits",
		},
	}
}

// Prepare initialises the Firecracker microVM for the given job.
//
// A full implementation would:
//   - Create a copy-on-write overlay of the root filesystem for this job,
//     ensuring jobs do not mutate the base image.
//   - Configure the VM with the specified vCPU count and memory.
//   - Set up a VSOCK listener for host-guest communication.
//   - Boot the microVM and wait for the guest agent to become ready.
//   - Mount the workspace into the VM via virtio-fs or a block device.
func (e *FirecrackerExecutor) Prepare(ctx context.Context, job *api.Job) error {
	e.job = job
	e.logger.InfoContext(ctx, "firecracker executor: Prepare is a stub",
		"job_id", job.ID,
		"vcpus", e.cfg.VCPUCount,
		"mem_mib", e.cfg.MemSizeMib,
	)
	return ErrNotImplemented
}

// Run executes a single step inside the Firecracker microVM.
//
// A full implementation would:
//   - Send the step's command to the guest agent over VSOCK.
//   - The guest agent would execute the command in a shell, streaming
//     stdout/stderr back over VSOCK.
//   - Enforce the step timeout by cancelling the context and, if needed,
//     forcefully stopping the guest process.
//   - Collect the exit code from the guest agent.
//   - Support step-level environment variables and working directory overrides.
func (e *FirecrackerExecutor) Run(ctx context.Context, step *api.Step) (*api.StepResult, error) {
	e.logger.InfoContext(ctx, "firecracker executor: Run is a stub",
		"step_id", step.ID,
		"step_name", step.Name,
	)
	return nil, ErrNotImplemented
}

// Cleanup releases all resources associated with the microVM.
//
// A full implementation would:
//   - Send a shutdown command to the guest agent for a graceful shutdown.
//   - If the VM does not stop within a grace period, forcefully kill the
//     Firecracker process.
//   - Remove the VM's socket file, log file, and copy-on-write overlay.
//   - Ensure no orphaned processes remain.
func (e *FirecrackerExecutor) Cleanup(ctx context.Context) error {
	e.logger.InfoContext(ctx, "firecracker executor: Cleanup is a stub")
	e.job = nil
	return nil
}
