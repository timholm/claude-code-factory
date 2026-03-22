package registry

import (
	"testing"
)

func TestEnqueueAndDequeue(t *testing.T) {
	r := testDB(t)

	spec := BuildSpec{
		Name:           "hello-world",
		Problem:        "print hello world",
		SourceURL:      "https://example.com",
		Solution:       "use fmt.Println",
		Language:       "go",
		Files:          `["main.go"]`,
		EstimatedLines: 10,
	}

	if err := r.EnqueueSpec(spec); err != nil {
		t.Fatalf("EnqueueSpec: %v", err)
	}

	got, err := r.DequeueNext()
	if err != nil {
		t.Fatalf("DequeueNext: %v", err)
	}
	if got == nil {
		t.Fatal("DequeueNext returned nil, expected a spec")
	}
	if got.Name != "hello-world" {
		t.Errorf("Name = %q, want %q", got.Name, "hello-world")
	}
	if got.Status != "building" {
		t.Errorf("Status = %q, want %q", got.Status, "building")
	}
	if got.Attempts != 1 {
		t.Errorf("Attempts = %d, want 1", got.Attempts)
	}
}

func TestDequeueEmptyReturnsNil(t *testing.T) {
	r := testDB(t)

	got, err := r.DequeueNext()
	if err != nil {
		t.Fatalf("DequeueNext on empty queue: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil from empty queue, got %+v", got)
	}
}

func TestMarkShipped(t *testing.T) {
	r := testDB(t)

	spec := BuildSpec{
		Name:      "ship-me",
		Problem:   "ship it",
		Solution:  "just ship",
		Language:  "go",
		Files:     `["main.go"]`,
	}
	if err := r.EnqueueSpec(spec); err != nil {
		t.Fatalf("EnqueueSpec: %v", err)
	}

	got, err := r.DequeueNext()
	if err != nil {
		t.Fatalf("DequeueNext: %v", err)
	}
	if got == nil {
		t.Fatal("expected spec, got nil")
	}

	if err := r.MarkShipped(got.ID); err != nil {
		t.Fatalf("MarkShipped: %v", err)
	}

	// Queue should now be empty — shipped items are not 'queued'
	next, err := r.DequeueNext()
	if err != nil {
		t.Fatalf("DequeueNext after ship: %v", err)
	}
	if next != nil {
		t.Errorf("expected nil after shipping, got %+v", next)
	}
}

func TestMarkFailed(t *testing.T) {
	r := testDB(t)

	spec := BuildSpec{
		Name:      "fail-me",
		Problem:   "fail it",
		Solution:  "oops",
		Language:  "go",
		Files:     `["main.go"]`,
	}
	if err := r.EnqueueSpec(spec); err != nil {
		t.Fatalf("EnqueueSpec: %v", err)
	}

	got, err := r.DequeueNext()
	if err != nil {
		t.Fatalf("DequeueNext: %v", err)
	}
	if got == nil {
		t.Fatal("expected spec, got nil")
	}

	errMsg := "compilation failed: undefined: foo"
	if err := r.MarkFailed(got.ID, errMsg); err != nil {
		t.Fatalf("MarkFailed: %v", err)
	}

	// Verify status is 'failed' — dequeue should return nil (not queued)
	next, err := r.DequeueNext()
	if err != nil {
		t.Fatalf("DequeueNext after fail: %v", err)
	}
	if next != nil {
		t.Errorf("expected nil after marking failed, got %+v", next)
	}
}

func TestRequeueForRetry(t *testing.T) {
	r := testDB(t)

	spec := BuildSpec{
		Name:      "retry-me",
		Problem:   "try again",
		Solution:  "eventually",
		Language:  "go",
		Files:     `["main.go"]`,
	}
	if err := r.EnqueueSpec(spec); err != nil {
		t.Fatalf("EnqueueSpec: %v", err)
	}

	// First dequeue — sets to 'building'
	first, err := r.DequeueNext()
	if err != nil {
		t.Fatalf("first DequeueNext: %v", err)
	}
	if first == nil {
		t.Fatal("expected spec on first dequeue, got nil")
	}
	if first.Status != "building" {
		t.Errorf("first dequeue Status = %q, want 'building'", first.Status)
	}

	// Requeue back to 'queued'
	if err := r.RequeueForRetry(first.ID); err != nil {
		t.Fatalf("RequeueForRetry: %v", err)
	}

	// Second dequeue — should get it again
	second, err := r.DequeueNext()
	if err != nil {
		t.Fatalf("second DequeueNext: %v", err)
	}
	if second == nil {
		t.Fatal("expected spec on second dequeue after requeue, got nil")
	}
	if second.ID != first.ID {
		t.Errorf("second dequeue ID = %d, want %d", second.ID, first.ID)
	}
	if second.Status != "building" {
		t.Errorf("second dequeue Status = %q, want 'building'", second.Status)
	}
	if second.Attempts != 2 {
		t.Errorf("second dequeue Attempts = %d, want 2", second.Attempts)
	}
}
