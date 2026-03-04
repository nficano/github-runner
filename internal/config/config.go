// Package config provides TOML-based configuration loading, validation,
// environment variable interpolation, and hot-reload support for the
// self-hosted GitHub Actions runner.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

// Duration wraps time.Duration so it can be unmarshaled from a TOML string
// such as "30s", "5m", or "1h".
type Duration struct {
	time.Duration
}

// UnmarshalText implements encoding.TextUnmarshaler for TOML string parsing.
func (d *Duration) UnmarshalText(text []byte) error {
	parsed, err := time.ParseDuration(string(text))
	if err != nil {
		return fmt.Errorf("parsing duration %q: %w", string(text), err)
	}
	d.Duration = parsed
	return nil
}

// MarshalText implements encoding.TextMarshaler for TOML string output.
func (d Duration) MarshalText() ([]byte, error) {
	return []byte(d.Duration.String()), nil
}

// ByteSize represents a size in bytes, parsed from human-readable strings
// such as "512m", "2g", or "10g". Supported suffixes: b, k, m, g, t
// (case-insensitive). A bare number is treated as bytes.
type ByteSize uint64

// Common byte size constants.
const (
	_           = iota
	KB ByteSize = 1 << (10 * iota)
	MB
	GB
	TB
)

// UnmarshalText implements encoding.TextUnmarshaler for TOML string parsing.
func (b *ByteSize) UnmarshalText(text []byte) error {
	s := strings.TrimSpace(string(text))
	if s == "" {
		return fmt.Errorf("parsing byte size: empty string")
	}

	s = strings.ToLower(s)

	var multiplier ByteSize = 1
	switch {
	case strings.HasSuffix(s, "tb") || strings.HasSuffix(s, "ti"):
		multiplier = TB
		s = strings.TrimRight(s, "tbi")
	case strings.HasSuffix(s, "gb") || strings.HasSuffix(s, "gi"):
		multiplier = GB
		s = strings.TrimRight(s, "gbi")
	case strings.HasSuffix(s, "mb") || strings.HasSuffix(s, "mi"):
		multiplier = MB
		s = strings.TrimRight(s, "mbi")
	case strings.HasSuffix(s, "kb") || strings.HasSuffix(s, "ki"):
		multiplier = KB
		s = strings.TrimRight(s, "kbi")
	case strings.HasSuffix(s, "t"):
		multiplier = TB
		s = strings.TrimSuffix(s, "t")
	case strings.HasSuffix(s, "g"):
		multiplier = GB
		s = strings.TrimSuffix(s, "g")
	case strings.HasSuffix(s, "m"):
		multiplier = MB
		s = strings.TrimSuffix(s, "m")
	case strings.HasSuffix(s, "k"):
		multiplier = KB
		s = strings.TrimSuffix(s, "k")
	case strings.HasSuffix(s, "b"):
		s = strings.TrimSuffix(s, "b")
	}

	n, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return fmt.Errorf("parsing byte size %q: %w", string(text), err)
	}
	if n < 0 {
		return fmt.Errorf("parsing byte size %q: negative value", string(text))
	}

	*b = ByteSize(n * float64(multiplier))
	return nil
}

// String returns a human-readable representation of the byte size.
func (b ByteSize) String() string {
	switch {
	case b >= TB:
		return fmt.Sprintf("%.1ft", float64(b)/float64(TB))
	case b >= GB:
		return fmt.Sprintf("%.1fg", float64(b)/float64(GB))
	case b >= MB:
		return fmt.Sprintf("%.1fm", float64(b)/float64(MB))
	case b >= KB:
		return fmt.Sprintf("%.1fk", float64(b)/float64(KB))
	default:
		return fmt.Sprintf("%db", b)
	}
}

// Config is the top-level configuration structure for the runner application.
type Config struct {
	Global  GlobalConfig   `toml:"global"`
	Runners []RunnerConfig `toml:"runners"`
}

// GlobalConfig holds application-wide settings.
type GlobalConfig struct {
	LogLevel        string   `toml:"log_level"`
	LogFormat       string   `toml:"log_format"`
	MetricsListen   string   `toml:"metrics_listen"`
	HealthListen    string   `toml:"health_listen"`
	ShutdownTimeout Duration `toml:"shutdown_timeout"`
	CheckInterval   Duration `toml:"check_interval"`
	API             APIConfig `toml:"api"`
}

// APIConfig holds settings for the GitHub API client.
type APIConfig struct {
	BaseURL      string   `toml:"base_url"`
	Timeout      Duration `toml:"timeout"`
	MaxRetries   int      `toml:"max_retries"`
	RetryBackoff Duration `toml:"retry_backoff"`
}

// RunnerConfig holds per-runner settings.
type RunnerConfig struct {
	Name        string            `toml:"name"`
	URL         string            `toml:"url"`
	Token       string            `toml:"token"`
	Executor    string            `toml:"executor"`
	Concurrency int               `toml:"concurrency"`
	Labels      []string          `toml:"labels"`
	WorkDir     string            `toml:"work_dir"`
	Shell       string            `toml:"shell"`
	Ephemeral   bool              `toml:"ephemeral"`
	Docker      DockerConfig      `toml:"docker"`
	Kubernetes  KubernetesConfig  `toml:"kubernetes"`
	Cache       CacheConfig       `toml:"cache"`
	Environment EnvironmentConfig `toml:"environment"`
}

// DockerConfig holds Docker executor settings.
type DockerConfig struct {
	Image         string            `toml:"image"`
	Privileged    bool              `toml:"privileged"`
	PullPolicy    string            `toml:"pull_policy"`
	Memory        string            `toml:"memory"`
	CPUs          float64           `toml:"cpus"`
	NetworkMode   string            `toml:"network_mode"`
	Volumes       []string          `toml:"volumes"`
	AllowedImages []string          `toml:"allowed_images"`
	DNS           []string          `toml:"dns"`
	CapDrop       []string          `toml:"cap_drop"`
	CapAdd        []string          `toml:"cap_add"`
	Runtime       string            `toml:"runtime"`
	Tmpfs         map[string]string `toml:"tmpfs"`
}

// KubernetesConfig holds Kubernetes executor settings.
type KubernetesConfig struct {
	Namespace      string            `toml:"namespace"`
	Image          string            `toml:"image"`
	ServiceAccount string            `toml:"service_account"`
	CPURequest     string            `toml:"cpu_request"`
	CPULimit       string            `toml:"cpu_limit"`
	MemoryRequest  string            `toml:"memory_request"`
	MemoryLimit    string            `toml:"memory_limit"`
	NodeSelector   map[string]string `toml:"node_selector"`
	PullPolicy     string            `toml:"pull_policy"`
}

// CacheConfig holds cache settings for a runner.
type CacheConfig struct {
	Type    string   `toml:"type"`
	Path    string   `toml:"path"`
	MaxSize ByteSize `toml:"max_size"`
	S3      S3Config `toml:"s3"`
	GCS     GCSConfig `toml:"gcs"`
}

// S3Config holds S3-specific cache settings.
type S3Config struct {
	Bucket   string `toml:"bucket"`
	Region   string `toml:"region"`
	Endpoint string `toml:"endpoint"`
	Prefix   string `toml:"prefix"`
}

// GCSConfig holds Google Cloud Storage-specific cache settings.
type GCSConfig struct {
	Bucket string `toml:"bucket"`
	Prefix string `toml:"prefix"`
}

// EnvironmentConfig is a map of environment variable names to values
// that are injected into job execution environments.
type EnvironmentConfig map[string]string

// Load reads a TOML configuration file from the given path, applies
// environment variable interpolation, and returns the parsed Config.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file %s: %w", path, err)
	}

	return LoadFromBytes(data)
}

// LoadFromBytes parses TOML configuration from raw bytes, applies
// environment variable interpolation, and returns the parsed Config.
// This is useful for testing without needing a file on disk.
func LoadFromBytes(data []byte) (*Config, error) {
	// Interpolate environment variables before parsing.
	interpolated := interpolateEnv(string(data))

	cfg := DefaultConfig()
	if _, err := toml.Decode(interpolated, cfg); err != nil {
		return nil, fmt.Errorf("parsing config TOML: %w", err)
	}

	return cfg, nil
}
