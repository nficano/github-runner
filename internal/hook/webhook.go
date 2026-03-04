package hook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/org/github-runner/pkg/api"
)

// WebhookHook sends an HTTP POST notification to a configured URL when
// job lifecycle events occur.
type WebhookHook struct {
	// URL is the webhook endpoint to POST to.
	URL string
	// Headers contains custom HTTP headers to include in the request.
	Headers map[string]string
	// Timeout is the HTTP request timeout.
	Timeout time.Duration
	client  *http.Client
	logger  *slog.Logger
}

// WebhookPayload is the JSON body sent to the webhook endpoint.
type WebhookPayload struct {
	Event      api.HookEvent `json:"event"`
	JobID      int64         `json:"job_id"`
	Repository string        `json:"repository"`
	Workflow   string        `json:"workflow"`
	Ref        string        `json:"ref"`
	SHA        string        `json:"sha"`
	Status     api.JobStatus `json:"status"`
	Timestamp  time.Time     `json:"timestamp"`
}

// NewWebhookHook creates a new webhook notification hook.
func NewWebhookHook(url string, timeout time.Duration, logger *slog.Logger) *WebhookHook {
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	return &WebhookHook{
		URL:     url,
		Headers: make(map[string]string),
		Timeout: timeout,
		client:  &http.Client{Timeout: timeout},
		logger:  logger,
	}
}

// Execute sends a webhook notification for the given event and job.
func (h *WebhookHook) Execute(ctx context.Context, event api.HookEvent, job *api.Job) error {
	payload := WebhookPayload{
		Event:      event,
		JobID:      job.ID,
		Repository: job.Repository,
		Workflow:   job.WorkflowName,
		Ref:        job.Ref,
		SHA:        job.SHA,
		Status:     job.Status,
		Timestamp:  time.Now().UTC(),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("webhook: marshalling payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("webhook: creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "github-runner-webhook/1.0")
	for k, v := range h.Headers {
		req.Header.Set(k, v)
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return fmt.Errorf("webhook: sending request to %s: %w", h.URL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook: %s returned status %d", h.URL, resp.StatusCode)
	}

	h.logger.DebugContext(ctx, "webhook sent",
		slog.String("url", h.URL),
		slog.String("event", string(event)),
		slog.Int64("job_id", job.ID),
		slog.Int("status_code", resp.StatusCode),
	)

	return nil
}
