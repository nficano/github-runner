package config

import "time"

// DefaultConfig returns a Config populated with sensible default values.
// Fields left at their zero value are intentionally optional and expected
// to be supplied by the user's TOML file.
func DefaultConfig() *Config {
	return &Config{
		Global: GlobalConfig{
			LogLevel:        "info",
			LogFormat:       "json",
			MetricsListen:   "127.0.0.1:9252",
			HealthListen:    "127.0.0.1:8484",
			ShutdownTimeout: Duration{30 * time.Second},
			CheckInterval:   Duration{3 * time.Second},
			API: APIConfig{
				BaseURL:      "https://api.github.com",
				Timeout:      Duration{30 * time.Second},
				MaxRetries:   3,
				RetryBackoff: Duration{1 * time.Second},
			},
		},
	}
}

// DefaultRunnerConfig returns a RunnerConfig with default values applied.
// Runner-specific defaults are merged when a runner entry does not specify
// a particular field.
func DefaultRunnerConfig() RunnerConfig {
	return RunnerConfig{
		Concurrency: 1,
		Shell:       "bash",
		Ephemeral:   false,
		Docker: DockerConfig{
			Image:       "ubuntu:22.04",
			Privileged:  false,
			PullPolicy:  "if-not-present",
			NetworkMode: "bridge",
			CapDrop:     []string{"ALL"},
		},
		Kubernetes: KubernetesConfig{
			Namespace:  "default",
			Image:      "ubuntu:22.04",
			PullPolicy: "IfNotPresent",
		},
		Cache: CacheConfig{
			Type: "local",
			Path: "/cache",
		},
	}
}
