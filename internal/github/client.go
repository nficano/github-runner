package github

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/nficano/github-runner/pkg/api"
)

// ---------------------------------------------------------------------------
// Interface
// ---------------------------------------------------------------------------

// GitHubClient defines the operations a runner needs to interact with the
// GitHub Actions service.
type GitHubClient interface {
	// AcquireJob attempts to acquire the next queued job matching the
	// runner's labels. A nil *JobResponse means no job is available.
	AcquireJob(ctx context.Context, runnerID int64) (*JobResponse, error)

	// ReportJobStatus sends a job-level status update.
	ReportJobStatus(ctx context.Context, jobID int64, status api.JobStatus) error

	// ReportStepStatus sends a step-level status update.
	ReportStepStatus(ctx context.Context, jobID int64, result *api.StepResult) error

	// SendHeartbeat notifies GitHub that the runner is still alive.
	SendHeartbeat(ctx context.Context, runnerID int64, jobID int64) error

	// UploadLog uploads a batch of log lines for a running job.
	UploadLog(ctx context.Context, jobID int64, lines []StepLog) error

	// RegisterRunner registers a new self-hosted runner.
	RegisterRunner(ctx context.Context, opts api.RegisterOptions) (*RunnerRegistrationResponse, error)

	// RemoveRunner de-registers an existing runner.
	RemoveRunner(ctx context.Context, runnerID int64) error

	// ListRunners returns all runners visible to the authenticated token.
	ListRunners(ctx context.Context) (*RunnerList, error)
}

// ---------------------------------------------------------------------------
// Client options
// ---------------------------------------------------------------------------

// ClientOptions configures the GitHub API client.
type ClientOptions struct {
	// BaseURL is the GitHub API base URL (e.g., "https://api.github.com").
	BaseURL string
	// Token is the Bearer token for authentication.
	Token string
	// Owner is the repository owner or organisation (e.g., "my-org").
	Owner string
	// Repo is the repository name (e.g., "my-repo"). When empty, the
	// client operates at the organisation level.
	Repo string
	// CABundlePath is an optional path to a PEM-encoded CA certificate
	// bundle for TLS verification. When empty, the system roots are used.
	CABundlePath string
	// MaxRetries is the maximum number of retry attempts for transient
	// errors. Defaults to 3 if zero.
	MaxRetries int
	// Logger is the structured logger to use. If nil, slog.Default() is used.
	Logger *slog.Logger
}

// ---------------------------------------------------------------------------
// Implementation
// ---------------------------------------------------------------------------

// client is the unexported implementation of GitHubClient.
type client struct {
	httpClient *http.Client
	baseURL    string
	token      string
	owner      string
	repo       string
	maxRetries int
	rateLimit  RateLimitInfo
	logger     *slog.Logger
}

// NewClient creates a new GitHubClient with connection pooling, optional
// proxy support (via standard environment variables), and custom CA bundle.
func NewClient(opts ClientOptions) (GitHubClient, error) {
	if opts.BaseURL == "" {
		return nil, fmt.Errorf("github client: base URL is required: %w", ErrInvalidOptions)
	}
	if opts.Token == "" {
		return nil, fmt.Errorf("github client: token is required: %w", ErrInvalidOptions)
	}
	if opts.Owner == "" {
		return nil, fmt.Errorf("github client: owner is required: %w", ErrInvalidOptions)
	}
	if opts.MaxRetries <= 0 {
		opts.MaxRetries = defaultMaxRetries
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}

	tlsCfg := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	if opts.CABundlePath != "" {
		pem, err := os.ReadFile(opts.CABundlePath)
		if err != nil {
			return nil, fmt.Errorf("github client: reading CA bundle %q: %w", opts.CABundlePath, err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("github client: no valid certificates in CA bundle %q: %w", opts.CABundlePath, ErrInvalidOptions)
		}
		tlsCfg.RootCAs = pool
	}

	transport := &http.Transport{
		// Connection pooling.
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		// Timeouts.
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout:  30 * time.Second,
		// Proxy from environment (HTTP_PROXY, HTTPS_PROXY, NO_PROXY).
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSClientConfig: tlsCfg,
	}

	return &client{
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   60 * time.Second,
		},
		baseURL:    opts.BaseURL,
		token:      opts.Token,
		owner:      opts.Owner,
		repo:       opts.Repo,
		maxRetries: opts.MaxRetries,
		logger:     opts.Logger,
	}, nil
}

// sentinel errors
var (
	// ErrInvalidOptions indicates one or more required ClientOptions fields
	// are missing or invalid.
	ErrInvalidOptions = fmt.Errorf("invalid client options")
)

const (
	defaultMaxRetries = 3
	// Base delay between retry attempts (doubled each time).
	retryBaseDelay = 1 * time.Second
)

// ---------------------------------------------------------------------------
// API methods
// ---------------------------------------------------------------------------

func (c *client) AcquireJob(ctx context.Context, runnerID int64) (*JobResponse, error) {
	path := c.actionsPath("/runner/jobs/acquire")
	body := JobRequest{RunnerID: runnerID}

	var resp JobResponse
	if err := c.doJSON(ctx, http.MethodPost, path, body, &resp); err != nil {
		// A 404 or 204 means no job is available.
		if apiErr, ok := asAPIError(err); ok && (apiErr.StatusCode == http.StatusNotFound || apiErr.StatusCode == http.StatusNoContent) {
			return nil, nil
		}
		return nil, fmt.Errorf("acquiring job for runner %d: %w", runnerID, err)
	}
	return &resp, nil
}

func (c *client) ReportJobStatus(ctx context.Context, jobID int64, status api.JobStatus) error {
	path := c.actionsPath(fmt.Sprintf("/jobs/%d/status", jobID))
	update := JobStatusUpdate{Status: string(status)}

	if err := c.doJSON(ctx, http.MethodPatch, path, update, nil); err != nil {
		return fmt.Errorf("reporting job %d status %q: %w", jobID, status, err)
	}
	return nil
}

func (c *client) ReportStepStatus(ctx context.Context, jobID int64, result *api.StepResult) error {
	path := c.actionsPath(fmt.Sprintf("/jobs/%d/steps/%s/status", jobID, result.StepID))
	update := StepStatusUpdate{
		StepID:     result.StepID,
		Status:     string(result.Status),
		Conclusion: string(result.Conclusion),
		StartedAt:  result.StartedAt,
		FinishedAt: result.CompletedAt,
		ExitCode:   result.ExitCode,
	}

	if err := c.doJSON(ctx, http.MethodPatch, path, update, nil); err != nil {
		return fmt.Errorf("reporting step %q status for job %d: %w", result.StepID, jobID, err)
	}
	return nil
}

func (c *client) SendHeartbeat(ctx context.Context, runnerID int64, jobID int64) error {
	path := c.actionsPath("/runner/heartbeat")
	hb := Heartbeat{
		RunnerID: runnerID,
		JobID:    jobID,
		SentAt:   time.Now().UTC(),
	}

	if err := c.doJSON(ctx, http.MethodPost, path, hb, nil); err != nil {
		return fmt.Errorf("sending heartbeat for runner %d: %w", runnerID, err)
	}
	return nil
}

func (c *client) UploadLog(ctx context.Context, jobID int64, lines []StepLog) error {
	path := c.actionsPath(fmt.Sprintf("/jobs/%d/logs", jobID))
	upload := LogUpload{
		JobID: jobID,
		Lines: lines,
	}

	if err := c.doJSON(ctx, http.MethodPost, path, upload, nil); err != nil {
		return fmt.Errorf("uploading logs for job %d: %w", jobID, err)
	}
	return nil
}

func (c *client) RegisterRunner(ctx context.Context, opts api.RegisterOptions) (*RunnerRegistrationResponse, error) {
	path := c.actionsPath("/runners/register")
	req := RunnerRegistrationRequest{
		Name: opts.Name,
	}
	for _, l := range opts.Labels {
		req.Labels = append(req.Labels, Label{Name: l, Type: "custom"})
	}

	var resp RunnerRegistrationResponse
	if err := c.doJSON(ctx, http.MethodPost, path, req, &resp); err != nil {
		return nil, fmt.Errorf("registering runner %q: %w", opts.Name, err)
	}
	return &resp, nil
}

func (c *client) RemoveRunner(ctx context.Context, runnerID int64) error {
	path := c.actionsPath(fmt.Sprintf("/runners/%d", runnerID))

	if err := c.doJSON(ctx, http.MethodDelete, path, nil, nil); err != nil {
		return fmt.Errorf("removing runner %d: %w", runnerID, err)
	}
	return nil
}

func (c *client) ListRunners(ctx context.Context) (*RunnerList, error) {
	path := c.actionsPath("/runners")

	var resp RunnerList
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, fmt.Errorf("listing runners: %w", err)
	}
	return &resp, nil
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// actionsPath builds the full API path for Actions endpoints.
func (c *client) actionsPath(suffix string) string {
	if c.repo != "" {
		return fmt.Sprintf("/repos/%s/%s/actions%s", c.owner, c.repo, suffix)
	}
	return fmt.Sprintf("/orgs/%s/actions%s", c.owner, suffix)
}

// doJSON performs an HTTP request with JSON encoding/decoding, automatic
// retry with exponential backoff, and rate-limit awareness. If respBody is
// nil the response body is discarded.
func (c *client) doJSON(ctx context.Context, method, path string, reqBody interface{}, respBody interface{}) error {
	url := c.baseURL + path

	var attempt int
	for {
		// Respect rate limits before sending the request.
		if wait := c.rateLimit.BackoffDuration(); wait > 0 {
			c.logger.WarnContext(ctx, "rate limit backoff",
				slog.Duration("wait", wait),
				slog.Int("remaining", c.rateLimit.Remaining),
			)
			timer := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				timer.Stop()
				return fmt.Errorf("%s %s: %w", method, path, ctx.Err())
			case <-timer.C:
			}
		}

		// Marshal request body.
		var bodyReader io.Reader
		if reqBody != nil {
			data, err := json.Marshal(reqBody)
			if err != nil {
				return fmt.Errorf("%s %s: marshalling request: %w", method, path, err)
			}
			bodyReader = bytes.NewReader(data)
		}

		req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
		if err != nil {
			return fmt.Errorf("%s %s: creating request: %w", method, path, err)
		}
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("Accept", "application/json")
		if reqBody != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		req.Header.Set("User-Agent", "github-runner/1.0")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			if !isRetryableNetError(err) || attempt >= c.maxRetries {
				return fmt.Errorf("%s %s: %w", method, path, err)
			}
			c.logger.WarnContext(ctx, "transient network error, retrying",
				slog.String("method", method),
				slog.String("path", path),
				slog.Int("attempt", attempt+1),
				slog.String("error", err.Error()),
			)
			if err := c.backoff(ctx, attempt); err != nil {
				return err
			}
			attempt++
			continue
		}

		// Always update rate-limit tracking from response headers.
		c.updateRateLimit(resp)

		// Read and close the body.
		defer resp.Body.Close()
		respData, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("%s %s: reading response body: %w", method, path, err)
		}

		// Success.
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			if respBody != nil && len(respData) > 0 {
				if err := json.Unmarshal(respData, respBody); err != nil {
					return fmt.Errorf("%s %s: unmarshalling response: %w", method, path, err)
				}
			}
			return nil
		}

		// Build API error.
		apiErr := &APIError{
			StatusCode: resp.StatusCode,
			RequestID:  resp.Header.Get("X-GitHub-Request-Id"),
		}
		// Try to extract a message from the JSON body.
		var errBody struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(respData, &errBody) == nil && errBody.Message != "" {
			apiErr.Message = errBody.Message
		} else {
			apiErr.Message = string(respData)
		}

		// Retry on transient status codes.
		if apiErr.IsRetryable() && attempt < c.maxRetries {
			c.logger.WarnContext(ctx, "retryable API error, retrying",
				slog.String("method", method),
				slog.String("path", path),
				slog.Int("status", resp.StatusCode),
				slog.Int("attempt", attempt+1),
			)
			if err := c.backoff(ctx, attempt); err != nil {
				return err
			}
			attempt++
			continue
		}

		return apiErr
	}
}

// backoff sleeps for an exponentially increasing duration. It respects context
// cancellation and returns an error if the context is done.
func (c *client) backoff(ctx context.Context, attempt int) error {
	delay := retryBaseDelay * time.Duration(math.Pow(2, float64(attempt)))
	timer := time.NewTimer(delay)
	select {
	case <-ctx.Done():
		timer.Stop()
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// updateRateLimit extracts rate limit information from the response headers.
func (c *client) updateRateLimit(resp *http.Response) {
	limitStr := resp.Header.Get("X-RateLimit-Limit")
	remainStr := resp.Header.Get("X-RateLimit-Remaining")
	resetStr := resp.Header.Get("X-RateLimit-Reset")

	if limitStr == "" || remainStr == "" || resetStr == "" {
		return
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		return
	}
	remaining, err := strconv.Atoi(remainStr)
	if err != nil {
		return
	}
	resetUnix, err := strconv.ParseInt(resetStr, 10, 64)
	if err != nil {
		return
	}

	c.rateLimit.Update(limit, remaining, time.Unix(resetUnix, 0))
}

// isRetryableNetError returns true for network-level errors that are likely
// transient (timeouts, connection resets, etc.).
func isRetryableNetError(err error) bool {
	if netErr, ok := err.(net.Error); ok {
		return netErr.Timeout()
	}
	return false
}

// asAPIError attempts to extract an *APIError from the error chain.
func asAPIError(err error) (*APIError, bool) {
	var apiErr *APIError
	if ok := errorAs(err, &apiErr); ok {
		return apiErr, true
	}
	return nil, false
}

// errorAs is a thin wrapper to keep the import list clean; it calls
// errors.As under the hood. We inline it to avoid importing "errors" just
// for this one call when we already have fmt.Errorf with %w.
func errorAs(err error, target interface{}) bool {
	// This is equivalent to errors.As but avoids importing errors since the
	// standard library errors.As is accessible via the errors package.
	type asInterface interface {
		As(interface{}) bool
	}
	for err != nil {
		if x, ok := err.(asInterface); ok {
			if x.As(target) {
				return true
			}
		}
		// Check direct type assertion.
		if apiErr, ok := target.(**APIError); ok {
			if e, ok := err.(*APIError); ok {
				*apiErr = e
				return true
			}
		}
		// Unwrap.
		type unwrapper interface {
			Unwrap() error
		}
		u, ok := err.(unwrapper)
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}
