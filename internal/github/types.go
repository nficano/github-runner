// Package github provides types and a client for interacting with the GitHub
// Actions service API. This includes runner registration, job acquisition,
// status reporting, and log upload.
package github

import (
	"net/http"
	"sync"
	"time"
)

// ---------------------------------------------------------------------------
// Runner registration
// ---------------------------------------------------------------------------

// RunnerRegistrationRequest is the payload sent to register a new self-hosted
// runner with a GitHub repository or organisation.
type RunnerRegistrationRequest struct {
	Name   string   `json:"name"`
	Labels []Label  `json:"labels,omitempty"`
}

// RunnerRegistrationResponse is returned by GitHub after a successful runner
// registration.
type RunnerRegistrationResponse struct {
	ID     int64   `json:"id"`
	Name   string  `json:"name"`
	OS     string  `json:"os"`
	Status string  `json:"status"`
	Labels []Label `json:"labels,omitempty"`
	Token  string  `json:"token"`
}

// Label represents a runner label as seen by the GitHub API.
type Label struct {
	ID   int64  `json:"id,omitempty"`
	Name string `json:"name"`
	Type string `json:"type,omitempty"` // "read-only" or "custom"
}

// Runner represents a registered self-hosted runner.
type Runner struct {
	ID     int64   `json:"id"`
	Name   string  `json:"name"`
	OS     string  `json:"os"`
	Status string  `json:"status"`
	Busy   bool    `json:"busy"`
	Labels []Label `json:"labels,omitempty"`
}

// RunnerList is the paged response for listing runners.
type RunnerList struct {
	TotalCount int      `json:"total_count"`
	Runners    []Runner `json:"runners"`
}

// ---------------------------------------------------------------------------
// Job acquisition & reporting
// ---------------------------------------------------------------------------

// JobRequest is the payload sent when a runner acquires a queued job.
type JobRequest struct {
	RunnerID int64 `json:"runner_id"`
}

// JobResponse is the payload returned when a job is acquired. It contains
// everything the runner needs to execute the job.
type JobResponse struct {
	ID             int64             `json:"id"`
	RunID          int64             `json:"run_id"`
	Name           string            `json:"name"`
	WorkflowName   string            `json:"workflow_name"`
	Labels         []string          `json:"labels,omitempty"`
	Steps          []StepPayload     `json:"steps,omitempty"`
	Secrets        map[string]string `json:"secrets,omitempty"`
	Env            map[string]string `json:"env,omitempty"`
	Workspace      string            `json:"workspace"`
	Repository     string            `json:"repository"`
	Ref            string            `json:"ref"`
	SHA            string            `json:"sha"`
	TimeoutMinutes int               `json:"timeout_minutes"`
	CreatedAt      time.Time         `json:"created_at"`
}

// StepPayload mirrors the step definition in the Actions API response.
type StepPayload struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	Run              string            `json:"run,omitempty"`
	Uses             string            `json:"uses,omitempty"`
	With             map[string]string `json:"with,omitempty"`
	Env              map[string]string `json:"env,omitempty"`
	If               string            `json:"if,omitempty"`
	TimeoutMinutes   int               `json:"timeout_minutes,omitempty"`
	ContinueOnError  bool              `json:"continue_on_error,omitempty"`
	Shell            string            `json:"shell,omitempty"`
	WorkingDirectory string            `json:"working_directory,omitempty"`
}

// JobStatusUpdate is sent to report the overall status of a job.
type JobStatusUpdate struct {
	Status     string    `json:"status"`
	Conclusion string    `json:"conclusion,omitempty"`
	StartedAt  time.Time `json:"started_at,omitempty"`
	FinishedAt time.Time `json:"finished_at,omitempty"`
}

// StepStatusUpdate is sent to report the status of an individual step.
type StepStatusUpdate struct {
	StepID     string    `json:"step_id"`
	Status     string    `json:"status"`
	Conclusion string    `json:"conclusion,omitempty"`
	StartedAt  time.Time `json:"started_at,omitempty"`
	FinishedAt time.Time `json:"finished_at,omitempty"`
	ExitCode   int       `json:"exit_code,omitempty"`
}

// Heartbeat is sent periodically so GitHub knows the runner is still alive.
type Heartbeat struct {
	RunnerID int64     `json:"runner_id"`
	JobID    int64     `json:"job_id,omitempty"`
	SentAt   time.Time `json:"sent_at"`
}

// ---------------------------------------------------------------------------
// Logs
// ---------------------------------------------------------------------------

// StepLog represents a chunk of log output for a step.
type StepLog struct {
	StepID string `json:"step_id"`
	// Line is the 1-based log line number.
	Line    int       `json:"line"`
	Content string    `json:"content"`
	Time    time.Time `json:"time"`
}

// LogUpload packages a batch of log lines for upload.
type LogUpload struct {
	JobID int64     `json:"job_id"`
	Lines []StepLog `json:"lines"`
}

// ---------------------------------------------------------------------------
// Workflow
// ---------------------------------------------------------------------------

// WorkflowRun represents a single execution of a GitHub Actions workflow.
type WorkflowRun struct {
	ID         int64     `json:"id"`
	Name       string    `json:"name"`
	HeadBranch string    `json:"head_branch"`
	HeadSHA    string    `json:"head_sha"`
	Status     string    `json:"status"`
	Conclusion string    `json:"conclusion,omitempty"`
	URL        string    `json:"html_url"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// ---------------------------------------------------------------------------
// Rate limiting
// ---------------------------------------------------------------------------

// RateLimitInfo tracks GitHub API rate limit state so the client can back
// off before hitting 403 responses. Fields are populated from the
// X-RateLimit-* response headers.
type RateLimitInfo struct {
	// Limit is the maximum number of requests allowed in the window.
	Limit int `json:"limit"`
	// Remaining is the number of requests left in the current window.
	Remaining int `json:"remaining"`
	// ResetAt is the time when the rate limit window resets.
	ResetAt time.Time `json:"reset_at"`

	mu sync.RWMutex
}

// Update stores new rate limit values. It is safe for concurrent use.
func (r *RateLimitInfo) Update(limit, remaining int, resetAt time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Limit = limit
	r.Remaining = remaining
	r.ResetAt = resetAt
}

// ShouldBackoff returns true if the client should wait before making another
// request. The threshold is 5% of the total limit.
func (r *RateLimitInfo) ShouldBackoff() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.Limit == 0 {
		return false
	}
	return r.Remaining <= r.Limit/20
}

// BackoffDuration returns how long the client should wait before retrying.
// If the rate limit has not been exceeded it returns zero.
func (r *RateLimitInfo) BackoffDuration() time.Duration {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.Remaining > r.Limit/20 {
		return 0
	}
	d := time.Until(r.ResetAt)
	if d < 0 {
		return 0
	}
	return d
}

// ---------------------------------------------------------------------------
// API error
// ---------------------------------------------------------------------------

// APIError represents a non-2xx response from the GitHub API.
type APIError struct {
	StatusCode int    `json:"status_code"`
	Message    string `json:"message"`
	RequestID  string `json:"request_id,omitempty"`
}

// Error implements the error interface.
func (e *APIError) Error() string {
	if e.RequestID != "" {
		return "github api: " + http.StatusText(e.StatusCode) + ": " + e.Message + " (request_id=" + e.RequestID + ")"
	}
	return "github api: " + http.StatusText(e.StatusCode) + ": " + e.Message
}

// IsRetryable returns true if the error represents a transient condition
// that may succeed on retry (429, 502, 503, 504).
func (e *APIError) IsRetryable() bool {
	switch e.StatusCode {
	case http.StatusTooManyRequests,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}
