// Package job provides the job model, step execution logic, and workflow
// command parsing for the GitHub Actions runner.
package job

import (
	"fmt"
	"time"

	"github.com/org/github-runner/pkg/api"
)

// Context holds the runtime context available to expressions and commands
// within a job. This mirrors the GitHub Actions context objects.
type Context struct {
	// GitHub holds repository and event metadata.
	GitHub GitHubContext `json:"github"`
	// Env holds environment variables available to the job.
	Env map[string]string `json:"env"`
	// Secrets holds secret values available to the job.
	Secrets map[string]string `json:"secrets"`
	// Steps holds outputs from previously completed steps.
	Steps map[string]*api.StepResult `json:"steps"`
	// Runner holds information about the runner executing this job.
	Runner RunnerContext `json:"runner"`
}

// GitHubContext holds GitHub-specific metadata available to expressions.
type GitHubContext struct {
	Repository string `json:"repository"`
	Ref        string `json:"ref"`
	SHA        string `json:"sha"`
	Workflow   string `json:"workflow"`
	RunID      int64  `json:"run_id"`
	JobName    string `json:"job"`
	Workspace  string `json:"workspace"`
	EventName  string `json:"event_name"`
}

// RunnerContext holds runner-specific metadata.
type RunnerContext struct {
	Name    string `json:"name"`
	OS      string `json:"os"`
	Arch    string `json:"arch"`
	TempDir string `json:"temp"`
	ToolDir string `json:"tool_cache"`
}

// NewContext creates a job context from a job and runner metadata.
func NewContext(job *api.Job, runnerName, runnerOS, runnerArch, tempDir string) *Context {
	return &Context{
		GitHub: GitHubContext{
			Repository: job.Repository,
			Ref:        job.Ref,
			SHA:        job.SHA,
			Workflow:   job.WorkflowName,
			RunID:      job.RunID,
			JobName:    job.Name,
			Workspace:  job.Workspace,
		},
		Env:     copyMap(job.Env),
		Secrets: copyMap(job.Secrets),
		Steps:   make(map[string]*api.StepResult),
		Runner: RunnerContext{
			Name:    runnerName,
			OS:      runnerOS,
			Arch:    runnerArch,
			TempDir: tempDir,
		},
	}
}

// SetStepResult records the result of a completed step for use in
// subsequent expression evaluation.
func (c *Context) SetStepResult(stepID string, result *api.StepResult) {
	c.Steps[stepID] = result
}

// MergedEnv returns the job environment merged with the step environment.
// Step-level values take precedence over job-level values.
func MergedEnv(job *api.Job, step *api.Step) map[string]string {
	merged := make(map[string]string, len(job.Env)+len(step.Env))
	for k, v := range job.Env {
		merged[k] = v
	}
	for k, v := range step.Env {
		merged[k] = v
	}
	return merged
}

// StepTimeout returns the effective timeout for a step. If the step has no
// timeout configured, it returns the job-level timeout. If neither is set,
// returns a default of 6 hours.
func StepTimeout(job *api.Job, step *api.Step) time.Duration {
	if step.TimeoutMinutes > 0 {
		return time.Duration(step.TimeoutMinutes) * time.Minute
	}
	if job.Timeout > 0 {
		return job.Timeout
	}
	return 6 * time.Hour
}

// ValidateStep checks that a step definition is valid. A step must have
// exactly one of Run or Uses set.
func ValidateStep(step *api.Step) error {
	if step.Run == "" && step.Uses == "" {
		return fmt.Errorf("step %q: must have either 'run' or 'uses'", step.Name)
	}
	if step.Run != "" && step.Uses != "" {
		return fmt.Errorf("step %q: cannot have both 'run' and 'uses'", step.Name)
	}
	return nil
}

func copyMap(m map[string]string) map[string]string {
	if m == nil {
		return make(map[string]string)
	}
	cp := make(map[string]string, len(m))
	for k, v := range m {
		cp[k] = v
	}
	return cp
}
