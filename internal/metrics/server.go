package metrics

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	defaultReadTimeout  = 5 * time.Second
	defaultWriteTimeout = 10 * time.Second
	defaultIdleTimeout  = 60 * time.Second
	shutdownTimeout     = 5 * time.Second
)

// MetricsServer exposes a /metrics endpoint backed by the custom Prometheus
// registry in [Metrics]. Start it with [MetricsServer.Start] and shut it down
// gracefully with [MetricsServer.Shutdown].
type MetricsServer struct {
	httpServer *http.Server
	logger     *slog.Logger
}

// NewMetricsServer creates a [MetricsServer] that listens on addr and serves
// metrics from the given [Metrics] registry. logger is used for operational
// messages; pass slog.Default() if you have no preference.
func NewMetricsServer(addr string, m *Metrics, logger *slog.Logger) *MetricsServer {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(m.Registry, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	}))

	return &MetricsServer{
		httpServer: &http.Server{
			Addr:         addr,
			Handler:      mux,
			ReadTimeout:  defaultReadTimeout,
			WriteTimeout: defaultWriteTimeout,
			IdleTimeout:  defaultIdleTimeout,
		},
		logger: logger,
	}
}

// Start begins serving metrics requests. It blocks until ctx is cancelled or
// an unrecoverable error occurs. When ctx is cancelled the server is shut down
// gracefully, giving in-flight requests up to 5 seconds to complete.
func (s *MetricsServer) Start(ctx context.Context) error {
	// Create a listener early so we can report the actual address.
	ln, err := net.Listen("tcp", s.httpServer.Addr)
	if err != nil {
		return fmt.Errorf("metrics server listen: %w", err)
	}

	s.logger.InfoContext(ctx, "metrics server started", slog.String("addr", ln.Addr().String()))

	// Shut down gracefully when the parent context is cancelled.
	go func() {
		<-ctx.Done()
		s.logger.InfoContext(ctx, "metrics server shutting down")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()

		if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
			s.logger.ErrorContext(ctx, "metrics server shutdown error", slog.String("error", err.Error()))
		}
	}()

	if err := s.httpServer.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("metrics server serve: %w", err)
	}
	return nil
}

// Shutdown gracefully stops the metrics server, giving in-flight requests up
// to the provided context's deadline to finish.
func (s *MetricsServer) Shutdown(ctx context.Context) error {
	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("metrics server shutdown: %w", err)
	}
	return nil
}
