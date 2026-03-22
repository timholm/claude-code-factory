package registry

import "fmt"

// Repo represents a generated repository tracked in the registry.
type Repo struct {
	Name      string
	Language  string
	Problem   string
	SourceURL string
}

// InsertRepo inserts a repo into the repos table, silently ignoring duplicates.
func (r *Registry) InsertRepo(repo Repo) error {
	const q = `INSERT OR IGNORE INTO repos (name, language, problem, source_url)
	            VALUES (?, ?, ?, ?)`
	if _, err := r.DB.Exec(q, repo.Name, repo.Language, repo.Problem, repo.SourceURL); err != nil {
		return fmt.Errorf("registry.InsertRepo: %w", err)
	}
	return nil
}

// GetUnpushedRepos returns all repos where github_pushed = FALSE, ordered by created_at.
func (r *Registry) GetUnpushedRepos() ([]Repo, error) {
	const q = `SELECT name, language, problem, source_url
	           FROM repos
	           WHERE github_pushed = FALSE
	           ORDER BY created_at`

	rows, err := r.DB.Query(q)
	if err != nil {
		return nil, fmt.Errorf("registry.GetUnpushedRepos: %w", err)
	}
	defer rows.Close()

	var repos []Repo
	for rows.Next() {
		var repo Repo
		if err := rows.Scan(&repo.Name, &repo.Language, &repo.Problem, &repo.SourceURL); err != nil {
			return nil, fmt.Errorf("registry.GetUnpushedRepos scan: %w", err)
		}
		repos = append(repos, repo)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("registry.GetUnpushedRepos rows: %w", err)
	}
	return repos, nil
}

// MarkPushed sets github_pushed = TRUE and records the push timestamp for the named repo.
func (r *Registry) MarkPushed(name string) error {
	const q = `UPDATE repos
	           SET github_pushed = TRUE, github_push_at = CURRENT_TIMESTAMP
	           WHERE name = ?`
	if _, err := r.DB.Exec(q, name); err != nil {
		return fmt.Errorf("registry.MarkPushed: %w", err)
	}
	return nil
}
