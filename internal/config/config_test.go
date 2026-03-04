package config

import (
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// ByteSize parsing
// ---------------------------------------------------------------------------

func TestByteSize_UnmarshalText(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    ByteSize
		wantErr bool
	}{
		{name: "bytes bare number", input: "1024", want: 1024},
		{name: "bytes with suffix", input: "512b", want: 512},
		{name: "kilobytes lower", input: "1k", want: KB},
		{name: "kilobytes upper", input: "2K", want: 2 * KB},
		{name: "megabytes", input: "256m", want: 256 * MB},
		{name: "gigabytes", input: "10g", want: 10 * GB},
		{name: "terabytes", input: "1t", want: TB},
		{name: "gigabytes with gb suffix", input: "2gb", want: 2 * GB},
		{name: "megabytes with mb suffix", input: "512mb", want: 512 * MB},
		{name: "fractional gigabytes", input: "1.5g", want: ByteSize(1.5 * float64(GB))},
		{name: "empty string", input: "", wantErr: true},
		{name: "invalid", input: "abc", wantErr: true},
		{name: "negative", input: "-1g", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var bs ByteSize
			err := bs.UnmarshalText([]byte(tt.input))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for input %q, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for input %q: %v", tt.input, err)
			}
			if bs != tt.want {
				t.Errorf("ByteSize(%q) = %d, want %d", tt.input, bs, tt.want)
			}
		})
	}
}

func TestByteSize_String(t *testing.T) {
	tests := []struct {
		input ByteSize
		want  string
	}{
		{0, "0b"},
		{512, "512b"},
		{KB, "1.0k"},
		{10 * GB, "10.0g"},
		{TB, "1.0t"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.input.String(); got != tt.want {
				t.Errorf("ByteSize(%d).String() = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Duration parsing
// ---------------------------------------------------------------------------

func TestDuration_UnmarshalText(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    time.Duration
		wantErr bool
	}{
		{name: "seconds", input: "30s", want: 30 * time.Second},
		{name: "minutes", input: "5m", want: 5 * time.Minute},
		{name: "hours", input: "1h", want: time.Hour},
		{name: "composite", input: "1m30s", want: 90 * time.Second},
		{name: "milliseconds", input: "100ms", want: 100 * time.Millisecond},
		{name: "invalid", input: "bogus", wantErr: true},
		{name: "empty", input: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var d Duration
			err := d.UnmarshalText([]byte(tt.input))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for input %q, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for input %q: %v", tt.input, err)
			}
			if d.Duration != tt.want {
				t.Errorf("Duration(%q) = %v, want %v", tt.input, d.Duration, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Env var interpolation
// ---------------------------------------------------------------------------

func TestInterpolateEnv(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		envs   map[string]string
		want   string
	}{
		{
			name:  "simple substitution",
			input: "token = ${MY_TOKEN}",
			envs:  map[string]string{"MY_TOKEN": "secret123"},
			want:  "token = secret123",
		},
		{
			name:  "multiple variables",
			input: "${HOST}:${PORT}",
			envs:  map[string]string{"HOST": "localhost", "PORT": "8080"},
			want:  "localhost:8080",
		},
		{
			name:  "missing variable becomes empty",
			input: "val = ${DOES_NOT_EXIST}",
			envs:  map[string]string{},
			want:  "val = ",
		},
		{
			name:  "no variables unchanged",
			input: "plain string without vars",
			envs:  map[string]string{},
			want:  "plain string without vars",
		},
		{
			name:  "nested env vars",
			input: "${OUTER}",
			envs:  map[string]string{"OUTER": "${INNER}", "INNER": "resolved"},
			want:  "resolved",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set env vars for this test.
			for k, v := range tt.envs {
				t.Setenv(k, v)
			}

			got := interpolateEnv(tt.input)
			if got != tt.want {
				t.Errorf("interpolateEnv(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Config loading from TOML
// ---------------------------------------------------------------------------

const minimalTOML = `
[global]
log_level = "debug"
log_format = "text"

[[runners]]
name = "test-runner"
url = "https://github.com/org/repo"
token = "ghp_test123"
executor = "docker"
concurrency = 2
labels = ["self-hosted", "linux"]
work_dir = "/tmp/runner"
`

const fullTOML = `
[global]
log_level = "info"
log_format = "json"
metrics_listen = "0.0.0.0:9252"
health_listen = "0.0.0.0:8484"
shutdown_timeout = "60s"
check_interval = "5s"

[global.api]
base_url = "https://api.github.com"
timeout = "15s"
max_retries = 5
retry_backoff = "2s"

[[runners]]
name = "docker-fast"
url = "https://github.com/org/repo"
token = "ghp_test123"
executor = "docker"
concurrency = 4
labels = ["self-hosted", "linux", "docker"]
work_dir = "/var/lib/runner"
shell = "bash"
ephemeral = true

  [runners.docker]
  image = "ubuntu:22.04"
  privileged = false
  pull_policy = "if-not-present"
  memory = "2g"
  cpus = 2.0
  network_mode = "bridge"
  volumes = ["/cache:/cache:ro"]
  allowed_images = ["ubuntu:*", "node:*"]
  dns = ["8.8.8.8"]
  cap_drop = ["ALL"]
  cap_add = ["NET_BIND_SERVICE"]

  [runners.cache]
  type = "local"
  path = "/cache"
  max_size = "10g"

  [runners.cache.s3]
  bucket = "my-cache"
  region = "us-east-1"

  [runners.environment]
  CI = "true"
  RUNNER_TOOL_CACHE = "/opt/hostedtoolcache"
`

func TestLoadFromBytes_Minimal(t *testing.T) {
	cfg, err := LoadFromBytes([]byte(minimalTOML))
	if err != nil {
		t.Fatalf("LoadFromBytes failed: %v", err)
	}

	if cfg.Global.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want %q", cfg.Global.LogLevel, "debug")
	}
	if cfg.Global.LogFormat != "text" {
		t.Errorf("LogFormat = %q, want %q", cfg.Global.LogFormat, "text")
	}
	if len(cfg.Runners) != 1 {
		t.Fatalf("len(Runners) = %d, want 1", len(cfg.Runners))
	}

	r := cfg.Runners[0]
	if r.Name != "test-runner" {
		t.Errorf("Runner.Name = %q, want %q", r.Name, "test-runner")
	}
	if r.Executor != "docker" {
		t.Errorf("Runner.Executor = %q, want %q", r.Executor, "docker")
	}
	if r.Concurrency != 2 {
		t.Errorf("Runner.Concurrency = %d, want 2", r.Concurrency)
	}
}

func TestLoadFromBytes_Full(t *testing.T) {
	cfg, err := LoadFromBytes([]byte(fullTOML))
	if err != nil {
		t.Fatalf("LoadFromBytes failed: %v", err)
	}

	// Global
	if cfg.Global.ShutdownTimeout.Duration != 60*time.Second {
		t.Errorf("ShutdownTimeout = %v, want 60s", cfg.Global.ShutdownTimeout.Duration)
	}
	if cfg.Global.CheckInterval.Duration != 5*time.Second {
		t.Errorf("CheckInterval = %v, want 5s", cfg.Global.CheckInterval.Duration)
	}

	// API
	if cfg.Global.API.MaxRetries != 5 {
		t.Errorf("API.MaxRetries = %d, want 5", cfg.Global.API.MaxRetries)
	}
	if cfg.Global.API.RetryBackoff.Duration != 2*time.Second {
		t.Errorf("API.RetryBackoff = %v, want 2s", cfg.Global.API.RetryBackoff.Duration)
	}

	// Runner
	r := cfg.Runners[0]
	if !r.Ephemeral {
		t.Error("Runner.Ephemeral = false, want true")
	}
	if r.Docker.CPUs != 2.0 {
		t.Errorf("Docker.CPUs = %f, want 2.0", r.Docker.CPUs)
	}
	if r.Docker.NetworkMode != "bridge" {
		t.Errorf("Docker.NetworkMode = %q, want %q", r.Docker.NetworkMode, "bridge")
	}
	if len(r.Docker.Volumes) != 1 || r.Docker.Volumes[0] != "/cache:/cache:ro" {
		t.Errorf("Docker.Volumes = %v, want [/cache:/cache:ro]", r.Docker.Volumes)
	}

	// Cache
	if r.Cache.MaxSize != 10*GB {
		t.Errorf("Cache.MaxSize = %d, want %d", r.Cache.MaxSize, 10*GB)
	}
	if r.Cache.S3.Bucket != "my-cache" {
		t.Errorf("Cache.S3.Bucket = %q, want %q", r.Cache.S3.Bucket, "my-cache")
	}

	// Environment
	if r.Environment["CI"] != "true" {
		t.Errorf("Environment[CI] = %q, want %q", r.Environment["CI"], "true")
	}
}

func TestLoadFromBytes_EnvInterpolation(t *testing.T) {
	tomlData := `
[[runners]]
name = "env-test"
url = "https://github.com/org/repo"
token = "${TEST_RUNNER_TOKEN}"
executor = "shell"
concurrency = 1
work_dir = "/tmp/runner"
`
	t.Setenv("TEST_RUNNER_TOKEN", "ghp_interpolated_value")

	cfg, err := LoadFromBytes([]byte(tomlData))
	if err != nil {
		t.Fatalf("LoadFromBytes failed: %v", err)
	}

	if cfg.Runners[0].Token != "ghp_interpolated_value" {
		t.Errorf("Token = %q, want %q", cfg.Runners[0].Token, "ghp_interpolated_value")
	}
}

func TestLoadFromBytes_Defaults(t *testing.T) {
	// A minimal TOML that does not set global fields should still
	// get defaults from DefaultConfig.
	tomlData := `
[[runners]]
name = "defaults-test"
url = "https://github.com/org/repo"
token = "tok"
executor = "shell"
concurrency = 1
work_dir = "/tmp/runner"
`
	cfg, err := LoadFromBytes([]byte(tomlData))
	if err != nil {
		t.Fatalf("LoadFromBytes failed: %v", err)
	}

	if cfg.Global.API.BaseURL != "https://api.github.com" {
		t.Errorf("API.BaseURL = %q, want default %q", cfg.Global.API.BaseURL, "https://api.github.com")
	}
	if cfg.Global.ShutdownTimeout.Duration != 30*time.Second {
		t.Errorf("ShutdownTimeout = %v, want default 30s", cfg.Global.ShutdownTimeout.Duration)
	}
}

// ---------------------------------------------------------------------------
// Validation
// ---------------------------------------------------------------------------

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr string // substring expected in error; empty means no error
	}{
		{
			name: "valid minimal config",
			cfg: &Config{
				Global: DefaultConfig().Global,
				Runners: []RunnerConfig{
					{
						Name:        "ok",
						URL:         "https://github.com/org/repo",
						Token:       "tok",
						Executor:    "shell",
						Concurrency: 1,
						WorkDir:     "/tmp/work",
					},
				},
			},
			wantErr: "",
		},
		{
			name: "no runners",
			cfg: &Config{
				Global: DefaultConfig().Global,
			},
			wantErr: "at least one runner must be defined",
		},
		{
			name: "missing runner name",
			cfg: &Config{
				Global: DefaultConfig().Global,
				Runners: []RunnerConfig{
					{
						URL:         "https://github.com/org/repo",
						Token:       "tok",
						Executor:    "shell",
						Concurrency: 1,
					},
				},
			},
			wantErr: "name: must not be empty",
		},
		{
			name: "invalid executor",
			cfg: &Config{
				Global: DefaultConfig().Global,
				Runners: []RunnerConfig{
					{
						Name:        "bad-exec",
						URL:         "https://github.com/org/repo",
						Token:       "tok",
						Executor:    "podman",
						Concurrency: 1,
					},
				},
			},
			wantErr: "must be one of: shell, docker, kubernetes, firecracker",
		},
		{
			name: "zero concurrency",
			cfg: &Config{
				Global: DefaultConfig().Global,
				Runners: []RunnerConfig{
					{
						Name:        "zero-conc",
						URL:         "https://github.com/org/repo",
						Token:       "tok",
						Executor:    "shell",
						Concurrency: 0,
					},
				},
			},
			wantErr: "concurrency: must be > 0",
		},
		{
			name: "relative work_dir",
			cfg: &Config{
				Global: DefaultConfig().Global,
				Runners: []RunnerConfig{
					{
						Name:        "rel-path",
						URL:         "https://github.com/org/repo",
						Token:       "tok",
						Executor:    "shell",
						Concurrency: 1,
						WorkDir:     "relative/path",
					},
				},
			},
			wantErr: "must be an absolute path",
		},
		{
			name: "invalid url scheme",
			cfg: &Config{
				Global: DefaultConfig().Global,
				Runners: []RunnerConfig{
					{
						Name:        "bad-url",
						URL:         "ftp://github.com/org/repo",
						Token:       "tok",
						Executor:    "shell",
						Concurrency: 1,
					},
				},
			},
			wantErr: "scheme must be http or https",
		},
		{
			name: "missing token",
			cfg: &Config{
				Global: DefaultConfig().Global,
				Runners: []RunnerConfig{
					{
						Name:        "no-token",
						URL:         "https://github.com/org/repo",
						Executor:    "shell",
						Concurrency: 1,
					},
				},
			},
			wantErr: "token: must not be empty",
		},
		{
			name: "docker executor missing image",
			cfg: &Config{
				Global: DefaultConfig().Global,
				Runners: []RunnerConfig{
					{
						Name:        "docker-no-image",
						URL:         "https://github.com/org/repo",
						Token:       "tok",
						Executor:    "docker",
						Concurrency: 1,
						Docker:      DockerConfig{Image: ""},
					},
				},
			},
			wantErr: "docker.image: must not be empty",
		},
		{
			name: "duplicate runner names",
			cfg: &Config{
				Global: DefaultConfig().Global,
				Runners: []RunnerConfig{
					{
						Name:        "dupe",
						URL:         "https://github.com/org/repo",
						Token:       "tok",
						Executor:    "shell",
						Concurrency: 1,
					},
					{
						Name:        "dupe",
						URL:         "https://github.com/org/repo",
						Token:       "tok",
						Executor:    "shell",
						Concurrency: 1,
					},
				},
			},
			wantErr: "duplicate runner name",
		},
		{
			name: "valid docker config",
			cfg: &Config{
				Global: DefaultConfig().Global,
				Runners: []RunnerConfig{
					{
						Name:        "docker-ok",
						URL:         "https://github.com/org/repo",
						Token:       "tok",
						Executor:    "docker",
						Concurrency: 2,
						WorkDir:     "/tmp/work",
						Docker: DockerConfig{
							Image:      "ubuntu:22.04",
							PullPolicy: "always",
							CPUs:       1.0,
						},
					},
				},
			},
			wantErr: "",
		},
		{
			name: "valid kubernetes config",
			cfg: &Config{
				Global: DefaultConfig().Global,
				Runners: []RunnerConfig{
					{
						Name:        "k8s-ok",
						URL:         "https://github.com/org/repo",
						Token:       "tok",
						Executor:    "kubernetes",
						Concurrency: 1,
						WorkDir:     "/tmp/work",
						Kubernetes: KubernetesConfig{
							Namespace:  "runners",
							Image:      "ubuntu:22.04",
							PullPolicy: "Always",
						},
					},
				},
			},
			wantErr: "",
		},
		{
			name: "invalid log level",
			cfg: &Config{
				Global: GlobalConfig{
					LogLevel:  "verbose",
					LogFormat: "json",
				},
				Runners: []RunnerConfig{
					{
						Name:        "log-level",
						URL:         "https://github.com/org/repo",
						Token:       "tok",
						Executor:    "shell",
						Concurrency: 1,
					},
				},
			},
			wantErr: "log_level: invalid value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.cfg)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// DefaultConfig
// ---------------------------------------------------------------------------

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Global.LogLevel != "info" {
		t.Errorf("default LogLevel = %q, want %q", cfg.Global.LogLevel, "info")
	}
	if cfg.Global.LogFormat != "json" {
		t.Errorf("default LogFormat = %q, want %q", cfg.Global.LogFormat, "json")
	}
	if cfg.Global.API.BaseURL != "https://api.github.com" {
		t.Errorf("default API.BaseURL = %q, want %q", cfg.Global.API.BaseURL, "https://api.github.com")
	}
	if cfg.Global.API.MaxRetries != 3 {
		t.Errorf("default API.MaxRetries = %d, want 3", cfg.Global.API.MaxRetries)
	}
	if cfg.Global.ShutdownTimeout.Duration != 30*time.Second {
		t.Errorf("default ShutdownTimeout = %v, want 30s", cfg.Global.ShutdownTimeout.Duration)
	}
	if cfg.Global.CheckInterval.Duration != 3*time.Second {
		t.Errorf("default CheckInterval = %v, want 3s", cfg.Global.CheckInterval.Duration)
	}
	if len(cfg.Runners) != 0 {
		t.Errorf("default Runners length = %d, want 0", len(cfg.Runners))
	}
}
