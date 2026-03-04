package runner

import (
	"log/slog"
	"testing"
)

func TestLifecycleTransitions(t *testing.T) {
	tests := []struct {
		name       string
		transitions []JobState
		wantErr    bool
		wantFinal  JobState
	}{
		{
			name:        "happy path",
			transitions: []JobState{StateClaimed, StatePreparing, StateRunning, StatePostExec, StateCompleted, StateCleanup},
			wantFinal:   StateCleanup,
		},
		{
			name:        "failure during prepare",
			transitions: []JobState{StateClaimed, StatePreparing, StateFailed, StateCleanup},
			wantFinal:   StateCleanup,
		},
		{
			name:        "cancellation during running",
			transitions: []JobState{StateClaimed, StatePreparing, StateRunning, StateCancelled, StateCleanup},
			wantFinal:   StateCleanup,
		},
		{
			name:        "invalid transition",
			transitions: []JobState{StateRunning},
			wantErr:     true,
			wantFinal:   StateQueued,
		},
		{
			name:        "cannot skip states",
			transitions: []JobState{StateClaimed, StateRunning},
			wantErr:     true,
			wantFinal:   StateClaimed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lc := NewLifecycle(1, slog.Default())
			var lastErr error
			for _, s := range tt.transitions {
				if err := lc.Transition(s); err != nil {
					lastErr = err
					break
				}
			}

			if tt.wantErr && lastErr == nil {
				t.Error("expected error but got nil")
			}
			if !tt.wantErr && lastErr != nil {
				t.Errorf("unexpected error: %v", lastErr)
			}
			if lc.State() != tt.wantFinal {
				t.Errorf("final state = %s, want %s", lc.State(), tt.wantFinal)
			}
		})
	}
}

func TestLifecycleIsTerminal(t *testing.T) {
	lc := NewLifecycle(1, slog.Default())

	if lc.IsTerminal() {
		t.Error("new lifecycle should not be terminal")
	}

	_ = lc.Transition(StateClaimed)
	_ = lc.Transition(StatePreparing)
	_ = lc.Transition(StateFailed)

	if lc.IsTerminal() {
		t.Error("failed state should not be terminal (cleanup still needed)")
	}

	_ = lc.Transition(StateCleanup)
	if !lc.IsTerminal() {
		t.Error("cleanup state should be terminal")
	}
}

func TestLifecycleOnTransition(t *testing.T) {
	lc := NewLifecycle(1, slog.Default())

	var transitions []string
	lc.OnTransition(func(from, to JobState) {
		transitions = append(transitions, from.String()+"->"+to.String())
	})

	_ = lc.Transition(StateClaimed)
	_ = lc.Transition(StatePreparing)

	if len(transitions) != 2 {
		t.Fatalf("expected 2 transitions, got %d", len(transitions))
	}
	if transitions[0] != "queued->claimed" {
		t.Errorf("transition[0] = %q, want %q", transitions[0], "queued->claimed")
	}
	if transitions[1] != "claimed->preparing" {
		t.Errorf("transition[1] = %q, want %q", transitions[1], "claimed->preparing")
	}
}
