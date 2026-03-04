package config

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// ReloadFunc is called when the configuration file changes and the new
// config is successfully loaded and validated. Implementations must be
// safe for concurrent invocation.
type ReloadFunc func(cfg *Config)

// Watcher monitors a configuration file for changes and invokes a
// callback with the newly loaded Config. Rapid changes are debounced
// so the callback is not called more often than the configured debounce
// interval.
type Watcher struct {
	path     string
	debounce time.Duration
	callback ReloadFunc
	watcher  *fsnotify.Watcher

	mu      sync.Mutex
	stopped bool
}

// NewWatcher creates a Watcher that monitors the file at path and calls
// fn when a valid configuration change is detected. Changes that occur
// within debounce of each other are coalesced into a single callback
// invocation. A typical debounce value is 100ms.
func NewWatcher(path string, debounce time.Duration, fn ReloadFunc) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("creating fsnotify watcher: %w", err)
	}

	if err := fsw.Add(path); err != nil {
		fsw.Close()
		return nil, fmt.Errorf("watching config file %s: %w", path, err)
	}

	return &Watcher{
		path:     path,
		debounce: debounce,
		callback: fn,
		watcher:  fsw,
	}, nil
}

// Run starts the watch loop. It blocks until ctx is cancelled or an
// unrecoverable error occurs. When ctx is cancelled, the underlying
// fsnotify watcher is closed and Run returns nil.
func (w *Watcher) Run(ctx context.Context) error {
	slog.Info("config watcher started", slog.String("path", w.path))
	defer func() {
		w.mu.Lock()
		w.stopped = true
		w.mu.Unlock()
		w.watcher.Close()
		slog.Info("config watcher stopped")
	}()

	var timer *time.Timer

	for {
		select {
		case <-ctx.Done():
			return nil

		case event, ok := <-w.watcher.Events:
			if !ok {
				return nil
			}
			// We only care about writes and creates (editors that
			// atomically replace files trigger Create).
			if !event.Has(fsnotify.Write) && !event.Has(fsnotify.Create) {
				continue
			}

			slog.Debug("config file changed",
				slog.String("path", w.path),
				slog.String("op", event.Op.String()),
			)

			// Debounce: reset the timer on every qualifying event.
			if timer != nil {
				timer.Stop()
			}
			timer = time.AfterFunc(w.debounce, func() {
				w.reload()
			})

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return nil
			}
			slog.Error("config watcher error", slog.Any("error", err))
		}
	}
}

// reload loads, validates, and delivers the new configuration.
func (w *Watcher) reload() {
	w.mu.Lock()
	if w.stopped {
		w.mu.Unlock()
		return
	}
	w.mu.Unlock()

	cfg, err := Load(w.path)
	if err != nil {
		slog.Error("failed to reload config",
			slog.String("path", w.path),
			slog.Any("error", err),
		)
		return
	}

	if err := Validate(cfg); err != nil {
		slog.Error("reloaded config is invalid, keeping current config",
			slog.String("path", w.path),
			slog.Any("error", err),
		)
		return
	}

	slog.Info("config reloaded successfully", slog.String("path", w.path))
	w.callback(cfg)
}
