package importer

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/lib/pq"

	"github.com/timholmquist/claude-code-factory/internal/registry"
)

// ProductSpec is the JSON structure produced by idea-engine.
// It maps to the fields in the candidates.spec_json column.
type ProductSpec struct {
	Name           string   `json:"name"`
	Problem        string   `json:"problem"`
	Solution       string   `json:"solution"`
	Language       string   `json:"language"`
	Files          []string `json:"files"`
	EstimatedLines int      `json:"estimated_lines"`
	SourcePapers   []string `json:"source_papers"`
	SourceRepos    []string `json:"source_repos"`
	SourceURL      string   `json:"source_url"`
	MarketAnalysis string   `json:"market_analysis"`
}

// ImportFromFile reads a JSON file containing one ProductSpec or an array of
// ProductSpec objects and enqueues each into the build queue. It returns the
// number of specs successfully enqueued.
func ImportFromFile(path string, reg *registry.Registry) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("importer: read %s: %w", path, err)
	}

	specs, err := parseSpecs(data)
	if err != nil {
		return 0, fmt.Errorf("importer: parse %s: %w", path, err)
	}

	return enqueueSpecs(specs, reg)
}

// ImportFromDir reads all .json files in a directory and enqueues their specs.
func ImportFromDir(dir string, reg *registry.Registry) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, fmt.Errorf("importer: readdir %s: %w", dir, err)
	}

	total := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		n, err := ImportFromFile(filepath.Join(dir, entry.Name()), reg)
		if err != nil {
			log.Printf("importer: skipping %s: %v", entry.Name(), err)
			continue
		}
		total += n
	}
	return total, nil
}

// ImportFromDB reads product specs from idea-engine's Postgres candidates table
// where status='synthesized', enqueues them into the local SQLite build queue,
// and marks them as 'delivered' in Postgres. Returns the number enqueued.
func ImportFromDB(postgresURL string, reg *registry.Registry) (int, error) {
	pg, err := sql.Open("postgres", postgresURL)
	if err != nil {
		return 0, fmt.Errorf("importer: open postgres: %w", err)
	}
	defer pg.Close()

	if err := pg.Ping(); err != nil {
		return 0, fmt.Errorf("importer: ping postgres: %w", err)
	}

	rows, err := pg.Query(
		`SELECT id, spec_json FROM candidates WHERE status IN ('synthesized', 'delivered') AND spec_json IS NOT NULL AND spec_json != '' ORDER BY id`,
	)
	if err != nil {
		return 0, fmt.Errorf("importer: query candidates: %w", err)
	}
	defer rows.Close()

	type candidate struct {
		id       int64
		specJSON string
	}

	var candidates []candidate
	for rows.Next() {
		var c candidate
		if err := rows.Scan(&c.id, &c.specJSON); err != nil {
			return 0, fmt.Errorf("importer: scan candidate: %w", err)
		}
		candidates = append(candidates, c)
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("importer: rows: %w", err)
	}

	enqueued := 0
	for _, c := range candidates {
		var spec ProductSpec
		if err := json.Unmarshal([]byte(c.specJSON), &spec); err != nil {
			log.Printf("importer: skipping candidate %d: invalid spec_json: %v", c.id, err)
			continue
		}

		exists, err := reg.SpecExists(spec.Name)
		if err != nil {
			log.Printf("importer: skipping %s: dedup check failed: %v", spec.Name, err)
			continue
		}
		if exists {
			log.Printf("importer: skipping %s: already exists", spec.Name)
			// Still mark as delivered so we don't re-process it.
			if _, err := pg.Exec(`UPDATE candidates SET status = 'imported', delivered_at = NOW() WHERE id = $1`, c.id); err != nil {
				log.Printf("importer: failed to mark candidate %d as delivered: %v", c.id, err)
			}
			continue
		}

		if err := reg.EnqueueSpec(specToBuildSpec(spec)); err != nil {
			log.Printf("importer: failed to enqueue %s: %v", spec.Name, err)
			continue
		}

		if _, err := pg.Exec(`UPDATE candidates SET status = 'imported', delivered_at = NOW() WHERE id = $1`, c.id); err != nil {
			log.Printf("importer: enqueued %s but failed to mark candidate %d as delivered: %v", spec.Name, c.id, err)
		}

		enqueued++
	}

	return enqueued, nil
}

// parseSpecs parses JSON data as either a single ProductSpec or an array of them.
func parseSpecs(data []byte) ([]ProductSpec, error) {
	// Trim whitespace to detect array vs object.
	trimmed := strings.TrimSpace(string(data))
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("empty input")
	}

	if trimmed[0] == '[' {
		var specs []ProductSpec
		if err := json.Unmarshal(data, &specs); err != nil {
			return nil, err
		}
		return specs, nil
	}

	var spec ProductSpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil, err
	}
	return []ProductSpec{spec}, nil
}

// enqueueSpecs handles dedup and enqueues each spec into the registry.
func enqueueSpecs(specs []ProductSpec, reg *registry.Registry) (int, error) {
	enqueued := 0
	for _, spec := range specs {
		if spec.Name == "" {
			log.Printf("importer: skipping spec with empty name")
			continue
		}

		exists, err := reg.SpecExists(spec.Name)
		if err != nil {
			log.Printf("importer: skipping %s: dedup check failed: %v", spec.Name, err)
			continue
		}
		if exists {
			log.Printf("importer: skipping %s: already exists", spec.Name)
			continue
		}

		if err := reg.EnqueueSpec(specToBuildSpec(spec)); err != nil {
			return enqueued, fmt.Errorf("enqueue %s: %w", spec.Name, err)
		}
		enqueued++
	}
	return enqueued, nil
}

// specToBuildSpec converts a ProductSpec into a registry.BuildSpec.
func specToBuildSpec(s ProductSpec) registry.BuildSpec {
	filesJSON, _ := json.Marshal(s.Files)
	papersJSON, _ := json.Marshal(s.SourcePapers)
	reposJSON, _ := json.Marshal(s.SourceRepos)

	return registry.BuildSpec{
		Name:           s.Name,
		Problem:        s.Problem,
		Solution:       s.Solution,
		Language:        s.Language,
		Files:          string(filesJSON),
		EstimatedLines: s.EstimatedLines,
		SourceURL:      s.SourceURL,
		SourcePapers:   string(papersJSON),
		SourceRepos:    string(reposJSON),
		MarketAnalysis: s.MarketAnalysis,
	}
}
