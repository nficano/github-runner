package config

import (
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
)

// validExecutors is the set of supported executor types.
var validExecutors = map[string]bool{
	"shell":       true,
	"docker":      true,
	"kubernetes":  true,
	"firecracker": true,
}

// Validate checks cfg for semantic correctness. It collects all
// validation errors and returns them joined into a single error.
// If the configuration is valid, nil is returned.
func Validate(cfg *Config) error {
	var errs []error

	errs = append(errs, validateGlobal(&cfg.Global)...)
	errs = append(errs, validateRunners(cfg.Runners)...)

	return errors.Join(errs...)
}

// validateGlobal checks global configuration fields.
func validateGlobal(g *GlobalConfig) []error {
	var errs []error

	validLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}
	if !validLevels[strings.ToLower(g.LogLevel)] {
		errs = append(errs, fmt.Errorf("global.log_level: invalid value %q, must be one of: debug, info, warn, error", g.LogLevel))
	}

	validFormats := map[string]bool{"json": true, "text": true}
	if !validFormats[strings.ToLower(g.LogFormat)] {
		errs = append(errs, fmt.Errorf("global.log_format: invalid value %q, must be one of: json, text", g.LogFormat))
	}

	if g.API.BaseURL != "" {
		if _, err := url.ParseRequestURI(g.API.BaseURL); err != nil {
			errs = append(errs, fmt.Errorf("global.api.base_url: invalid URL %q: %w", g.API.BaseURL, err))
		}
	}

	if g.API.MaxRetries < 0 {
		errs = append(errs, fmt.Errorf("global.api.max_retries: must be >= 0, got %d", g.API.MaxRetries))
	}

	return errs
}

// validateRunners checks the runners list and each individual runner.
func validateRunners(runners []RunnerConfig) []error {
	var errs []error

	if len(runners) == 0 {
		errs = append(errs, fmt.Errorf("runners: at least one runner must be defined"))
		return errs
	}

	names := make(map[string]bool, len(runners))
	for i := range runners {
		r := &runners[i]
		prefix := fmt.Sprintf("runners[%d]", i)

		if r.Name == "" {
			errs = append(errs, fmt.Errorf("%s.name: must not be empty", prefix))
		} else if names[r.Name] {
			errs = append(errs, fmt.Errorf("%s.name: duplicate runner name %q", prefix, r.Name))
		} else {
			names[r.Name] = true
			prefix = fmt.Sprintf("runners[%q]", r.Name)
		}

		if r.URL == "" {
			errs = append(errs, fmt.Errorf("%s.url: must not be empty", prefix))
		} else {
			u, err := url.ParseRequestURI(r.URL)
			if err != nil {
				errs = append(errs, fmt.Errorf("%s.url: invalid URL %q: %w", prefix, r.URL, err))
			} else if u.Scheme != "http" && u.Scheme != "https" {
				errs = append(errs, fmt.Errorf("%s.url: scheme must be http or https, got %q", prefix, u.Scheme))
			}
		}

		if r.Token == "" {
			errs = append(errs, fmt.Errorf("%s.token: must not be empty", prefix))
		}

		if r.Executor == "" {
			errs = append(errs, fmt.Errorf("%s.executor: must not be empty", prefix))
		} else if !validExecutors[r.Executor] {
			errs = append(errs, fmt.Errorf("%s.executor: invalid value %q, must be one of: shell, docker, kubernetes, firecracker", prefix, r.Executor))
		}

		if r.Concurrency <= 0 {
			errs = append(errs, fmt.Errorf("%s.concurrency: must be > 0, got %d", prefix, r.Concurrency))
		}

		if r.WorkDir != "" && !filepath.IsAbs(r.WorkDir) {
			errs = append(errs, fmt.Errorf("%s.work_dir: must be an absolute path, got %q", prefix, r.WorkDir))
		}

		if r.Executor == "docker" {
			errs = append(errs, validateDocker(prefix, &r.Docker)...)
		}

		if r.Executor == "kubernetes" {
			errs = append(errs, validateKubernetes(prefix, &r.Kubernetes)...)
		}
	}

	return errs
}

// validateDocker checks Docker executor configuration.
func validateDocker(prefix string, d *DockerConfig) []error {
	var errs []error

	if d.Image == "" {
		errs = append(errs, fmt.Errorf("%s.docker.image: must not be empty", prefix))
	}

	validPullPolicies := map[string]bool{
		"always":         true,
		"never":          true,
		"if-not-present": true,
	}
	if d.PullPolicy != "" && !validPullPolicies[d.PullPolicy] {
		errs = append(errs, fmt.Errorf("%s.docker.pull_policy: invalid value %q, must be one of: always, never, if-not-present", prefix, d.PullPolicy))
	}

	if d.CPUs < 0 {
		errs = append(errs, fmt.Errorf("%s.docker.cpus: must be >= 0, got %f", prefix, d.CPUs))
	}

	return errs
}

// validateKubernetes checks Kubernetes executor configuration.
func validateKubernetes(prefix string, k *KubernetesConfig) []error {
	var errs []error

	if k.Namespace == "" {
		errs = append(errs, fmt.Errorf("%s.kubernetes.namespace: must not be empty", prefix))
	}

	if k.Image == "" {
		errs = append(errs, fmt.Errorf("%s.kubernetes.image: must not be empty", prefix))
	}

	validPullPolicies := map[string]bool{
		"Always":       true,
		"Never":        true,
		"IfNotPresent": true,
	}
	if k.PullPolicy != "" && !validPullPolicies[k.PullPolicy] {
		errs = append(errs, fmt.Errorf("%s.kubernetes.pull_policy: invalid value %q, must be one of: Always, Never, IfNotPresent", prefix, k.PullPolicy))
	}

	return errs
}
