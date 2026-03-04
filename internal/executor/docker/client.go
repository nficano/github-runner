package docker

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
)

// Client wraps the Docker Engine API client and provides higher-level
// operations used by the DockerExecutor.
type Client struct {
	docker *client.Client
	logger *slog.Logger
}

// NewClient creates a new Client that connects to the Docker daemon using
// environment variables (DOCKER_HOST, DOCKER_TLS_VERIFY, etc.) or the
// default Unix socket.
func NewClient() (*Client, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("creating docker client: %w", err)
	}

	return &Client{
		docker: cli,
		logger: slog.Default().With("component", "docker-client"),
	}, nil
}

// Close releases the underlying Docker client resources.
func (c *Client) Close() error {
	if c.docker != nil {
		return c.docker.Close()
	}
	return nil
}

// ImagePull pulls the specified image from a registry. Progress information
// is logged but not returned to the caller. The pull blocks until the image
// is fully downloaded or the context is cancelled.
func (c *Client) ImagePull(ctx context.Context, ref string) error {
	c.logger.InfoContext(ctx, "pulling image", "image", ref)

	reader, err := c.docker.ImagePull(ctx, ref, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("pulling image %s: %w", ref, err)
	}
	defer reader.Close()

	// Drain the pull progress stream. In production this could be wired to
	// a progress reporter; for now we discard the output to ensure the pull
	// completes.
	if _, err := io.Copy(io.Discard, reader); err != nil {
		return fmt.Errorf("reading pull progress for %s: %w", ref, err)
	}

	c.logger.InfoContext(ctx, "image pulled successfully", "image", ref)
	return nil
}

// ImageExists checks whether the specified image is available locally.
func (c *Client) ImageExists(ctx context.Context, ref string) (bool, error) {
	_, _, err := c.docker.ImageInspectWithRaw(ctx, ref)
	if err != nil {
		if client.IsErrNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("inspecting image %s: %w", ref, err)
	}
	return true, nil
}

// ContainerCreate creates a new container with the given configuration and
// returns its ID.
func (c *Client) ContainerCreate(ctx context.Context, cfg *container.Config, hostCfg *container.HostConfig, name string) (string, error) {
	c.logger.InfoContext(ctx, "creating container",
		"name", name,
		"image", cfg.Image,
	)

	resp, err := c.docker.ContainerCreate(ctx, cfg, hostCfg, nil, nil, name)
	if err != nil {
		return "", fmt.Errorf("creating container %s: %w", name, err)
	}

	for _, warning := range resp.Warnings {
		c.logger.WarnContext(ctx, "container creation warning",
			"name", name,
			"warning", warning,
		)
	}

	return resp.ID, nil
}

// ContainerStart starts a previously created container.
func (c *Client) ContainerStart(ctx context.Context, containerID string) error {
	c.logger.InfoContext(ctx, "starting container", "container_id", containerID)

	if err := c.docker.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return fmt.Errorf("starting container %s: %w", containerID, err)
	}
	return nil
}

// ContainerWait blocks until the specified container stops and returns its
// exit code.
func (c *Client) ContainerWait(ctx context.Context, containerID string) (int64, error) {
	statusCh, errCh := c.docker.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)

	select {
	case err := <-errCh:
		if err != nil {
			return -1, fmt.Errorf("waiting for container %s: %w", containerID, err)
		}
	case status := <-statusCh:
		if status.Error != nil {
			return status.StatusCode, fmt.Errorf("container %s exited with error: %s", containerID, status.Error.Message)
		}
		return status.StatusCode, nil
	case <-ctx.Done():
		return -1, fmt.Errorf("context cancelled waiting for container %s: %w", containerID, ctx.Err())
	}

	return -1, fmt.Errorf("unexpected state waiting for container %s", containerID)
}

// ContainerRemove forcefully removes a container and its volumes.
func (c *Client) ContainerRemove(ctx context.Context, containerID string) error {
	c.logger.InfoContext(ctx, "removing container", "container_id", containerID)

	return c.docker.ContainerRemove(ctx, containerID, container.RemoveOptions{
		Force:         true,
		RemoveVolumes: true,
	})
}

// StreamLogs attaches to the container's stdout and stderr streams and copies
// them to os.Stdout and os.Stderr respectively. It blocks until the streams
// are closed or the context is cancelled.
func (c *Client) StreamLogs(ctx context.Context, containerID string) error {
	opts := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Timestamps: false,
	}

	reader, err := c.docker.ContainerLogs(ctx, containerID, opts)
	if err != nil {
		return fmt.Errorf("attaching to container logs %s: %w", containerID, err)
	}
	defer reader.Close()

	// Docker multiplexes stdout and stderr into a single stream with 8-byte
	// headers. StdCopy demultiplexes them.
	if _, err := io.Copy(os.Stdout, reader); err != nil {
		// Context cancellation during streaming is expected.
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("streaming logs for container %s: %w", containerID, err)
	}

	return nil
}
