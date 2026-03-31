package target

import (
	"context"
	"errors"
	"testing"

	"bananas-pos/internal/job"
)

type lifecycleTarget struct {
	name          string
	startCalls    int
	shutdownCalls int
	startErr      error
	shutdownErr   error
}

func (t *lifecycleTarget) Name() string { return t.name }

func (t *lifecycleTarget) Send(context.Context, job.PrintJob) error { return nil }

func (t *lifecycleTarget) Health(context.Context) error { return nil }

func (t *lifecycleTarget) Start() error {
	t.startCalls++
	return t.startErr
}

func (t *lifecycleTarget) Shutdown() error {
	t.shutdownCalls++
	return t.shutdownErr
}

func TestSwitcherSetStartsNextTargetBeforeSwap(t *testing.T) {
	current := &lifecycleTarget{name: "current"}
	next := &lifecycleTarget{name: "next"}
	switcher := NewSwitcher(current, "")

	if err := switcher.Set(next); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	if next.startCalls != 1 {
		t.Fatalf("expected next target to start once, got %d", next.startCalls)
	}
	if current.shutdownCalls != 1 {
		t.Fatalf("expected current target to shut down once, got %d", current.shutdownCalls)
	}
	if switcher.Current() != next {
		t.Fatal("expected switcher to point at next target")
	}
}

func TestSwitcherSetDoesNotSwapWhenNextStartFails(t *testing.T) {
	current := &lifecycleTarget{name: "current"}
	next := &lifecycleTarget{name: "next", startErr: errors.New("boom")}
	switcher := NewSwitcher(current, "")

	err := switcher.Set(next)
	if err == nil {
		t.Fatal("expected error")
	}
	if switcher.Current() != current {
		t.Fatal("expected switcher to keep current target")
	}
	if current.shutdownCalls != 0 {
		t.Fatalf("expected current target to remain running, got %d shutdowns", current.shutdownCalls)
	}
}
