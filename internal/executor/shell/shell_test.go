package shell

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nficano/github-runner/pkg/api"
)

func TestSanitizeEnvKey(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want bool
	}{
		{name: "simple", key: "HOME", want: true},
		{name: "with_underscore", key: "MY_VAR", want: true},
		{name: "leading_underscore", key: "_PRIVATE", want: true},
		{name: "with_digits", key: "VAR123", want: true},
		{name: "lowercase", key: "path", want: true},
		{name: "mixed_case", key: "myVar_1", want: true},
		{name: "starts_with_digit", key: "1VAR", want: false},
		{name: "has_dash", key: "MY-VAR", want: false},
		{name: "has_dot", key: "my.var", want: false},
		{name: "has_space", key: "MY VAR", want: false},
		{name: "empty", key: "", want: false},
		{name: "has_equals", key: "A=B", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeEnvKey(tt.key)
			if got != tt.want {
				t.Errorf("SanitizeEnvKey(%q) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}

func TestFilterEnv(t *testing.T) {
	tests := []struct {
		name      string
		env       []string
		allowlist []string
		denylist  []string
		want      []string
	}{
		{
			name:      "no_filters",
			env:       []string{"HOME=/home/user", "PATH=/usr/bin", "SHELL=/bin/bash"},
			allowlist: nil,
			denylist:  nil,
			want:      []string{"HOME=/home/user", "PATH=/usr/bin", "SHELL=/bin/bash"},
		},
		{
			name:      "allowlist_only",
			env:       []string{"HOME=/home/user", "PATH=/usr/bin", "SECRET=hunter2"},
			allowlist: []string{"HOME", "PATH"},
			denylist:  nil,
			want:      []string{"HOME=/home/user", "PATH=/usr/bin"},
		},
		{
			name:      "denylist_only",
			env:       []string{"HOME=/home/user", "PATH=/usr/bin", "SECRET=hunter2"},
			allowlist: nil,
			denylist:  []string{"SECRET"},
			want:      []string{"HOME=/home/user", "PATH=/usr/bin"},
		},
		{
			name:      "denylist_overrides_allowlist",
			env:       []string{"HOME=/home/user", "SECRET=hunter2"},
			allowlist: []string{"HOME", "SECRET"},
			denylist:  []string{"SECRET"},
			want:      []string{"HOME=/home/user"},
		},
		{
			name:      "invalid_keys_dropped",
			env:       []string{"GOOD=yes", "1BAD=no", "ALSO-BAD=no"},
			allowlist: nil,
			denylist:  nil,
			want:      []string{"GOOD=yes"},
		},
		{
			name:      "malformed_entries_dropped",
			env:       []string{"GOOD=yes", "NOEQUALS"},
			allowlist: nil,
			denylist:  nil,
			want:      []string{"GOOD=yes"},
		},
		{
			name:      "empty_env",
			env:       []string{},
			allowlist: nil,
			denylist:  nil,
			want:      []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FilterEnv(tt.env, tt.allowlist, tt.denylist)
			if len(got) != len(tt.want) {
				t.Fatalf("FilterEnv() returned %d entries, want %d\ngot:  %v\nwant: %v", len(got), len(tt.want), got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("FilterEnv()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestMergeEnv(t *testing.T) {
	tests := []struct {
		name     string
		base     map[string]string
		override map[string]string
		want     []string
		wantErr  bool
	}{
		{
			name:     "simple_merge",
			base:     map[string]string{"A": "1", "B": "2"},
			override: map[string]string{"C": "3"},
			want:     []string{"A=1", "B=2", "C=3"},
		},
		{
			name:     "override_wins",
			base:     map[string]string{"A": "old"},
			override: map[string]string{"A": "new"},
			want:     []string{"A=new"},
		},
		{
			name:     "invalid_key_in_base",
			base:     map[string]string{"1BAD": "val"},
			override: map[string]string{"GOOD": "val"},
			want:     []string{"GOOD=val"},
			wantErr:  true,
		},
		{
			name:     "both_nil",
			base:     nil,
			override: nil,
			want:     []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := MergeEnv(tt.base, tt.override)
			if (err != nil) != tt.wantErr {
				t.Fatalf("MergeEnv() error = %v, wantErr %v", err, tt.wantErr)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("MergeEnv() returned %d entries, want %d\ngot:  %v\nwant: %v", len(got), len(tt.want), got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("MergeEnv()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestShellExecutor_Prepare(t *testing.T) {
	workDir := t.TempDir()

	exec, err := New(ShellConfig{
		WorkDir: workDir,
		Shell:   "sh",
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	job := &api.Job{
		ID:   42,
		Name: "test-job",
	}

	ctx := context.Background()
	if err := exec.Prepare(ctx, job); err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}

	// Verify workspace was created.
	expectedDir := filepath.Join(workDir, "job-42")
	info, err := os.Stat(expectedDir)
	if err != nil {
		t.Fatalf("workspace directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("workspace path is not a directory")
	}

	// Cleanup.
	if err := exec.Cleanup(ctx); err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}
	if _, err := os.Stat(expectedDir); !os.IsNotExist(err) {
		t.Fatal("workspace not removed after Cleanup()")
	}
}

func TestShellExecutor_Run(t *testing.T) {
	tests := []struct {
		name       string
		step       api.Step
		wantExit   int
		wantConc   api.StepConclusion
		wantOutput string
		wantErr    bool
	}{
		{
			name: "successful_echo",
			step: api.Step{
				ID:   "step1",
				Name: "Echo hello",
				Run:  "echo hello",
			},
			wantExit:   0,
			wantConc:   api.ConclusionSuccess,
			wantOutput: "hello\n",
		},
		{
			name: "failing_command",
			step: api.Step{
				ID:   "step2",
				Name: "Exit 1",
				Run:  "exit 1",
			},
			wantExit: 1,
			wantConc: api.ConclusionFailure,
		},
		{
			name: "custom_exit_code",
			step: api.Step{
				ID:   "step3",
				Name: "Exit 42",
				Run:  "exit 42",
			},
			wantExit: 42,
			wantConc: api.ConclusionFailure,
		},
		{
			name: "step_env_visible",
			step: api.Step{
				ID:   "step4",
				Name: "Check env",
				Run:  "echo $MY_STEP_VAR",
				Env:  map[string]string{"MY_STEP_VAR": "step_value"},
			},
			wantExit:   0,
			wantConc:   api.ConclusionSuccess,
			wantOutput: "step_value\n",
		},
		{
			name: "custom_shell",
			step: api.Step{
				ID:    "step5",
				Name:  "Use sh",
				Run:   "echo from_sh",
				Shell: "sh",
			},
			wantExit:   0,
			wantConc:   api.ConclusionSuccess,
			wantOutput: "from_sh\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workDir := t.TempDir()
			var stdout bytes.Buffer

			exec, err := New(ShellConfig{
				WorkDir: workDir,
				Shell:   "sh",
				Stdout:  &stdout,
				Stderr:  &bytes.Buffer{},
			})
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}

			ctx := context.Background()
			job := &api.Job{
				ID:   1,
				Name: "test",
			}
			if err := exec.Prepare(ctx, job); err != nil {
				t.Fatalf("Prepare() error = %v", err)
			}
			defer func() {
				_ = exec.Cleanup(ctx)
			}()

			result, err := exec.Run(ctx, &tt.step)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Run() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}

			if result.ExitCode != tt.wantExit {
				t.Errorf("ExitCode = %d, want %d", result.ExitCode, tt.wantExit)
			}
			if result.Conclusion != tt.wantConc {
				t.Errorf("Conclusion = %q, want %q", result.Conclusion, tt.wantConc)
			}
			if result.Status != api.StepCompleted {
				t.Errorf("Status = %q, want %q", result.Status, api.StepCompleted)
			}
			if tt.wantOutput != "" && stdout.String() != tt.wantOutput {
				t.Errorf("Output = %q, want %q", stdout.String(), tt.wantOutput)
			}
			if result.StartedAt.IsZero() {
				t.Error("StartedAt should not be zero")
			}
			if result.CompletedAt.IsZero() {
				t.Error("CompletedAt should not be zero")
			}
		})
	}
}

func TestShellExecutor_RunBeforePrepare(t *testing.T) {
	workDir := t.TempDir()
	exec, err := New(ShellConfig{
		WorkDir: workDir,
		Shell:   "sh",
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, err = exec.Run(context.Background(), &api.Step{ID: "s", Run: "echo hi"})
	if err == nil {
		t.Fatal("Run() before Prepare() should return an error")
	}
}

func TestShellExecutor_Timeout(t *testing.T) {
	workDir := t.TempDir()
	var stdout bytes.Buffer

	exec, err := New(ShellConfig{
		WorkDir: workDir,
		Shell:   "sh",
		Stdout:  &stdout,
		Stderr:  &bytes.Buffer{},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := context.Background()
	job := &api.Job{ID: 1, Name: "timeout-test"}
	if err := exec.Prepare(ctx, job); err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	defer func() {
		_ = exec.Cleanup(ctx)
	}()

	// Use a context that times out quickly.
	timeoutCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()

	step := &api.Step{
		ID:   "slow",
		Name: "Slow step",
		Run:  "sleep 60",
	}

	result, err := exec.Run(timeoutCtx, step)
	if err != nil {
		t.Fatalf("Run() unexpected error = %v", err)
	}
	if result.Conclusion != api.ConclusionFailure {
		t.Errorf("Conclusion = %q, want %q", result.Conclusion, api.ConclusionFailure)
	}
}

func TestShellExecutor_Info(t *testing.T) {
	workDir := t.TempDir()
	exec, err := New(ShellConfig{
		WorkDir: workDir,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	info := exec.Info()
	if info.Name != "shell" {
		t.Errorf("Name = %q, want %q", info.Name, "shell")
	}
	if info.Version == "" {
		t.Error("Version should not be empty")
	}
}

func TestShellExecutor_CleanupIdempotent(t *testing.T) {
	workDir := t.TempDir()
	exec, err := New(ShellConfig{
		WorkDir: workDir,
		Shell:   "sh",
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := context.Background()

	// Cleanup without Prepare should succeed.
	if err := exec.Cleanup(ctx); err != nil {
		t.Fatalf("Cleanup() without Prepare error = %v", err)
	}

	// Cleanup twice after Prepare should succeed.
	job := &api.Job{ID: 1, Name: "test"}
	if err := exec.Prepare(ctx, job); err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	if err := exec.Cleanup(ctx); err != nil {
		t.Fatalf("Cleanup() first call error = %v", err)
	}
	if err := exec.Cleanup(ctx); err != nil {
		t.Fatalf("Cleanup() second call error = %v", err)
	}
}

func TestNew_Validation(t *testing.T) {
	tests := []struct {
		name    string
		cfg     ShellConfig
		wantErr bool
	}{
		{
			name:    "empty_workdir",
			cfg:     ShellConfig{},
			wantErr: true,
		},
		{
			name: "valid_config",
			cfg: ShellConfig{
				WorkDir: "/tmp/test",
			},
			wantErr: false,
		},
		{
			name: "default_shell",
			cfg: ShellConfig{
				WorkDir: "/tmp/test",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
