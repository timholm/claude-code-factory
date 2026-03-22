package registry

import (
	"database/sql"
	"fmt"
)

// BuildSpec represents a single item in the build queue.
type BuildSpec struct {
	ID             int64
	Name           string
	Problem        string
	SourceURL      string
	Solution       string
	Language       string
	Files          string // JSON array as string
	EstimatedLines int
	Status         string
	Attempts       int
	ErrorLog       string
}

// EnqueueSpec inserts a new BuildSpec into the build_queue with status 'queued'.
func (r *Registry) EnqueueSpec(s BuildSpec) error {
	_, err := r.DB.Exec(
		`INSERT INTO build_queue
			(name, problem, source_url, solution, language, files, estimated_lines, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, 'queued')`,
		s.Name, s.Problem, s.SourceURL, s.Solution, s.Language, s.Files, s.EstimatedLines,
	)
	if err != nil {
		return fmt.Errorf("registry.EnqueueSpec: %w", err)
	}
	return nil
}

// DequeueNext atomically selects the first 'queued' item, sets its status to
// 'building', increments attempts, and returns the spec. Returns nil, nil if
// the queue is empty.
func (r *Registry) DequeueNext() (*BuildSpec, error) {
	tx, err := r.DB.Begin()
	if err != nil {
		return nil, fmt.Errorf("registry.DequeueNext: begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	var s BuildSpec
	err = tx.QueryRow(
		`SELECT id, name, problem, source_url, solution, language, files,
		        estimated_lines, status, attempts, COALESCE(error_log, '')
		 FROM build_queue
		 WHERE status = 'queued'
		 ORDER BY id ASC
		 LIMIT 1`,
	).Scan(
		&s.ID, &s.Name, &s.Problem, &s.SourceURL, &s.Solution, &s.Language,
		&s.Files, &s.EstimatedLines, &s.Status, &s.Attempts, &s.ErrorLog,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("registry.DequeueNext: select: %w", err)
	}

	_, err = tx.Exec(
		`UPDATE build_queue
		 SET status = 'building', attempts = attempts + 1, started_at = CURRENT_TIMESTAMP
		 WHERE id = ?`,
		s.ID,
	)
	if err != nil {
		return nil, fmt.Errorf("registry.DequeueNext: update: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("registry.DequeueNext: commit: %w", err)
	}

	s.Status = "building"
	s.Attempts++
	return &s, nil
}

// MarkShipped sets a build_queue item's status to 'shipped'.
func (r *Registry) MarkShipped(id int64) error {
	_, err := r.DB.Exec(
		`UPDATE build_queue SET status = 'shipped', shipped_at = CURRENT_TIMESTAMP WHERE id = ?`,
		id,
	)
	if err != nil {
		return fmt.Errorf("registry.MarkShipped: %w", err)
	}
	return nil
}

// MarkFailed sets a build_queue item's status to 'failed' and records the error log.
func (r *Registry) MarkFailed(id int64, errLog string) error {
	_, err := r.DB.Exec(
		`UPDATE build_queue SET status = 'failed', error_log = ? WHERE id = ?`,
		errLog, id,
	)
	if err != nil {
		return fmt.Errorf("registry.MarkFailed: %w", err)
	}
	return nil
}

// RequeueForRetry resets a build_queue item's status back to 'queued' for another attempt.
func (r *Registry) RequeueForRetry(id int64) error {
	_, err := r.DB.Exec(
		`UPDATE build_queue SET status = 'queued' WHERE id = ?`,
		id,
	)
	if err != nil {
		return fmt.Errorf("registry.RequeueForRetry: %w", err)
	}
	return nil
}
