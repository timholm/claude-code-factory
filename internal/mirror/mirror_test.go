package mirror

import (
	"context"
	"testing"
	"time"
)

// TestStaggerDelay verifies that stagger sleeps for at least the requested duration.
func TestStaggerDelay(t *testing.T) {
	delay := 100 * time.Millisecond
	start := time.Now()
	stagger(context.Background(), delay)
	elapsed := time.Since(start)

	if elapsed < 90*time.Millisecond {
		t.Errorf("stagger slept %v, want >= 90ms", elapsed)
	}
}

// TestStaggerDelayContextCancel verifies that stagger returns early when the
// context is cancelled.
func TestStaggerDelayContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	delay := 5 * time.Second
	start := time.Now()
	stagger(ctx, delay)
	elapsed := time.Since(start)

	if elapsed >= time.Second {
		t.Errorf("stagger took %v with cancelled context, want near-instant", elapsed)
	}
}
