// Package health implements liveness and readiness probes for the GitHub
// Actions runner, following the Kubernetes probe conventions. The probes are
// exposed as HTTP endpoints (/healthz and /readyz) that return JSON responses
// and appropriate status codes.
package health

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"
)

const (
	defaultReadTimeout  = 5 * time.Second
	defaultWriteTimeout = 10 * time.Second
	defaultIdleTimeout  = 60 * time.Second
	shutdownTimeout     = 5 * time.Second
)

// StatusOK is returned when all checks pass.
const StatusOK = "ok"

// StatusError is returned when one or more checks fail.
const StatusError = "error"

// Response is the JSON body returned by health endpoints.
type Response struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks"`
}

// HealthServer serves the /healthz and /readyz HTTP endpoints.
type HealthServer struct {
	httpServer *http.Server
	logger     *slog.Logger
	registry   *CheckRegistry

	mu    sync.RWMutex
	ready bool
}

// NewHealthServer creates a [HealthServer] bound to addr that evaluates checks
// from registry. The server starts in a not-ready state; call [HealthServer.SetReady]
// when the runner is ready to accept jobs.
func NewHealthServer(addr string, registry *CheckRegistry, logger *slog.Logger) *HealthServer {
	s := &HealthServer{
		logger:   logger,
		registry: registry,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleLiveness)
	mux.HandleFunc("/readyz", s.handleReadiness)

	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  defaultReadTimeout,
		WriteTimeout: defaultWriteTimeout,
		IdleTimeout:  defaultIdleTimeout,
	}

	return s
}

// SetReady marks the runner as ready (or not ready) to accept jobs. This
// controls the /readyz response.
func (s *HealthServer) SetReady(ready bool) {
	s.mu.Lock()
	s.ready = ready
	s.mu.Unlock()
}

// Start begins serving health endpoints. It blocks until ctx is cancelled or
// an unrecoverable error occurs.
func (s *HealthServer) Start(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.httpServer.Addr)
	if err != nil {
		return fmt.Errorf("health server listen: %w", err)
	}

	s.logger.InfoContext(ctx, "health server started", slog.String("addr", ln.Addr().String()))

	go func() {
		<-ctx.Done()
		s.logger.InfoContext(ctx, "health server shutting down")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()

		if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
			s.logger.ErrorContext(ctx, "health server shutdown error", slog.String("error", err.Error()))
		}
	}()

	if err := s.httpServer.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("health server serve: %w", err)
	}
	return nil
}

// Shutdown gracefully stops the health server, allowing in-flight requests to
// complete within the deadline of ctx.
func (s *HealthServer) Shutdown(ctx context.Context) error {
	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("health server shutdown: %w", err)
	}
	return nil
}

// handleLiveness implements the /healthz endpoint. If the process is running
// and the handler is reachable the probe succeeds.
func (s *HealthServer) handleLiveness(w http.ResponseWriter, r *http.Request) {
	resp := Response{
		Status: StatusOK,
		Checks: map[string]string{"alive": StatusOK},
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleReadiness implements the /readyz endpoint. It runs every registered
// check and reports an aggregate status.
func (s *HealthServer) handleReadiness(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	ready := s.ready
	s.mu.RUnlock()

	resp := Response{
		Status: StatusOK,
		Checks: make(map[string]string),
	}
	httpStatus := http.StatusOK

	if !ready {
		resp.Status = StatusError
		resp.Checks["ready"] = "runner is not yet ready"
		httpStatus = http.StatusServiceUnavailable
	} else {
		resp.Checks["ready"] = StatusOK
	}

	// Evaluate all registered health checks.
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	results := s.registry.RunAll(ctx)
	for name, err := range results {
		if err != nil {
			resp.Status = StatusError
			resp.Checks[name] = err.Error()
			httpStatus = http.StatusServiceUnavailable
		} else {
			resp.Checks[name] = StatusOK
		}
	}

	writeJSON(w, httpStatus, resp)
}

// writeJSON serialises resp as JSON and writes it to w with the given HTTP
// status code.
func writeJSON(w http.ResponseWriter, status int, resp Response) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	// Encoding errors at this point are not recoverable; best-effort write.
	_ = json.NewEncoder(w).Encode(resp)
}
