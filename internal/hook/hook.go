// Package hook defines the Hook interface and a composable HookChain that
// allows multiple hooks to be executed in sequence at well-defined points in
// the job lifecycle.
package hook

import (
	"context"
	"fmt"

	"github.com/nficano/github-runner/pkg/api"
)

// Hook is implemented by any value that needs to observe or modify behaviour
// at specific points in the job lifecycle. Hooks receive the event type and
// the full Job definition so they can make context-aware decisions.
type Hook interface {
	// Execute runs the hook logic for the given event. It should return an
	// error only if the failure is severe enough to abort the job.
	Execute(ctx context.Context, event api.HookEvent, job *api.Job) error
}

// HookChain composes zero or more hooks into a single Hook that executes
// them in order. Execution stops at the first hook that returns an error.
type HookChain struct {
	hooks []Hook
}

// NewHookChain creates a HookChain that will execute the provided hooks
// in the order they are given.
func NewHookChain(hooks ...Hook) *HookChain {
	return &HookChain{hooks: hooks}
}

// Append adds one or more hooks to the end of the chain.
func (c *HookChain) Append(hooks ...Hook) {
	c.hooks = append(c.hooks, hooks...)
}

// Len returns the number of hooks in the chain.
func (c *HookChain) Len() int {
	return len(c.hooks)
}

// Execute runs every hook in the chain sequentially. It stops and returns
// the first error encountered, wrapping it with the hook index for
// debuggability.
func (c *HookChain) Execute(ctx context.Context, event api.HookEvent, job *api.Job) error {
	for i, h := range c.hooks {
		if err := h.Execute(ctx, event, job); err != nil {
			return fmt.Errorf("hook chain[%d] (%s): %w", i, event, err)
		}
	}
	return nil
}
