package importer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/timholmquist/claude-code-factory/internal/registry"
)

func testDB(t *testing.T) *registry.Registry {
	t.Helper()
	db, err := registry.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return &registry.Registry{DB: db}
}

func TestImportFromFile_SingleSpec(t *testing.T) {
	reg := testDB(t)

	spec := ProductSpec{
		Name:           "test-tool",
		Problem:        "testing is hard",
		Solution:       "make it easy",
		Language:       "go",
		Files:          []string{"main.go", "main_test.go"},
		EstimatedLines: 100,
		SourcePapers:   []string{"2603.12345"},
		SourceRepos:    []string{"https://github.com/foo/bar"},
		SourceURL:      "https://arxiv.org/abs/2603.12345",
		MarketAnalysis: "big market",
	}

	data, err := json.Marshal(spec)
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(t.TempDir(), "spec.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	n, err := ImportFromFile(path, reg)
	if err != nil {
		t.Fatalf("ImportFromFile: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 imported, got %d", n)
	}

	// Verify it's in the queue.
	got, err := reg.DequeueNext()
	if err != nil {
		t.Fatalf("DequeueNext: %v", err)
	}
	if got == nil {
		t.Fatal("expected spec in queue, got nil")
	}
	if got.Name != "test-tool" {
		t.Errorf("Name = %q, want %q", got.Name, "test-tool")
	}
	if got.Problem != "testing is hard" {
		t.Errorf("Problem = %q, want %q", got.Problem, "testing is hard")
	}
	if got.SourceURL != "https://arxiv.org/abs/2603.12345" {
		t.Errorf("SourceURL = %q, want %q", got.SourceURL, "https://arxiv.org/abs/2603.12345")
	}
}

func TestImportFromFile_Array(t *testing.T) {
	reg := testDB(t)

	specs := []ProductSpec{
		{
			Name:           "tool-a",
			Problem:        "problem a",
			Solution:       "solution a",
			Language:       "go",
			Files:          []string{"main.go"},
			EstimatedLines: 50,
		},
		{
			Name:           "tool-b",
			Problem:        "problem b",
			Solution:       "solution b",
			Language:       "python",
			Files:          []string{"main.py"},
			EstimatedLines: 80,
		},
	}

	data, err := json.Marshal(specs)
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(t.TempDir(), "specs.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	n, err := ImportFromFile(path, reg)
	if err != nil {
		t.Fatalf("ImportFromFile: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected 2 imported, got %d", n)
	}
}

func TestImportFromFile_Dedup(t *testing.T) {
	reg := testDB(t)

	spec := ProductSpec{
		Name:           "dedup-test",
		Problem:        "dedup problem",
		Solution:       "dedup solution",
		Language:       "go",
		Files:          []string{"main.go"},
		EstimatedLines: 30,
	}

	data, err := json.Marshal(spec)
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(t.TempDir(), "spec.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	// First import should succeed.
	n1, err := ImportFromFile(path, reg)
	if err != nil {
		t.Fatalf("first ImportFromFile: %v", err)
	}
	if n1 != 1 {
		t.Fatalf("first import: expected 1, got %d", n1)
	}

	// Second import of same spec should skip (dedup).
	n2, err := ImportFromFile(path, reg)
	if err != nil {
		t.Fatalf("second ImportFromFile: %v", err)
	}
	if n2 != 0 {
		t.Fatalf("second import: expected 0 (dedup), got %d", n2)
	}
}

func TestImportFromFile_DedupAgainstRepos(t *testing.T) {
	reg := testDB(t)

	// Insert a repo with the same name.
	if err := reg.InsertRepo(registry.Repo{
		Name:     "existing-tool",
		Language: "go",
		Problem:  "already built",
	}); err != nil {
		t.Fatal(err)
	}

	spec := ProductSpec{
		Name:           "existing-tool",
		Problem:        "same name",
		Solution:       "should be skipped",
		Language:       "go",
		Files:          []string{"main.go"},
		EstimatedLines: 20,
	}

	data, err := json.Marshal(spec)
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(t.TempDir(), "spec.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	n, err := ImportFromFile(path, reg)
	if err != nil {
		t.Fatalf("ImportFromFile: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected 0 (dedup against repos), got %d", n)
	}
}

func TestImportFromDir(t *testing.T) {
	reg := testDB(t)
	dir := t.TempDir()

	// Write two spec files.
	for _, name := range []string{"tool-x", "tool-y"} {
		spec := ProductSpec{
			Name:           name,
			Problem:        "problem " + name,
			Solution:       "solution " + name,
			Language:       "go",
			Files:          []string{"main.go"},
			EstimatedLines: 40,
		}
		data, _ := json.Marshal(spec)
		if err := os.WriteFile(filepath.Join(dir, name+".json"), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Write a non-json file — should be ignored.
	if err := os.WriteFile(filepath.Join(dir, "README.txt"), []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	n, err := ImportFromDir(dir, reg)
	if err != nil {
		t.Fatalf("ImportFromDir: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected 2 imported, got %d", n)
	}
}

func TestImportFromFile_EmptyName(t *testing.T) {
	reg := testDB(t)

	spec := ProductSpec{
		Name:           "",
		Problem:        "no name",
		Solution:       "skip me",
		Language:       "go",
		Files:          []string{"main.go"},
		EstimatedLines: 10,
	}

	data, err := json.Marshal(spec)
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(t.TempDir(), "spec.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	n, err := ImportFromFile(path, reg)
	if err != nil {
		t.Fatalf("ImportFromFile: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected 0 (empty name skipped), got %d", n)
	}
}

func TestParseSpecs_Single(t *testing.T) {
	input := `{"name":"single","problem":"p","solution":"s","language":"go","files":["main.go"],"estimated_lines":10}`
	specs, err := parseSpecs([]byte(input))
	if err != nil {
		t.Fatalf("parseSpecs: %v", err)
	}
	if len(specs) != 1 {
		t.Fatalf("expected 1 spec, got %d", len(specs))
	}
	if specs[0].Name != "single" {
		t.Errorf("Name = %q, want %q", specs[0].Name, "single")
	}
}

func TestParseSpecs_Array(t *testing.T) {
	input := `[{"name":"a","problem":"p","solution":"s","language":"go","files":[],"estimated_lines":1},{"name":"b","problem":"p","solution":"s","language":"go","files":[],"estimated_lines":2}]`
	specs, err := parseSpecs([]byte(input))
	if err != nil {
		t.Fatalf("parseSpecs: %v", err)
	}
	if len(specs) != 2 {
		t.Fatalf("expected 2 specs, got %d", len(specs))
	}
}

func TestParseSpecs_Empty(t *testing.T) {
	_, err := parseSpecs([]byte(""))
	if err == nil {
		t.Fatal("expected error on empty input")
	}
}

func TestSpecToBuildSpec(t *testing.T) {
	spec := ProductSpec{
		Name:           "convert-test",
		Problem:        "problem",
		Solution:       "solution",
		Language:       "go",
		Files:          []string{"main.go", "lib.go"},
		EstimatedLines: 150,
		SourcePapers:   []string{"2603.11111", "2603.22222"},
		SourceRepos:    []string{"https://github.com/a/b"},
		SourceURL:      "https://arxiv.org/abs/2603.11111",
		MarketAnalysis: "strong demand",
	}

	bs := specToBuildSpec(spec)

	if bs.Name != "convert-test" {
		t.Errorf("Name = %q", bs.Name)
	}
	if bs.EstimatedLines != 150 {
		t.Errorf("EstimatedLines = %d", bs.EstimatedLines)
	}
	if bs.MarketAnalysis != "strong demand" {
		t.Errorf("MarketAnalysis = %q", bs.MarketAnalysis)
	}

	// Verify JSON serialization of arrays.
	var files []string
	if err := json.Unmarshal([]byte(bs.Files), &files); err != nil {
		t.Fatalf("unmarshal files: %v", err)
	}
	if len(files) != 2 || files[0] != "main.go" {
		t.Errorf("files = %v", files)
	}

	var papers []string
	if err := json.Unmarshal([]byte(bs.SourcePapers), &papers); err != nil {
		t.Fatalf("unmarshal papers: %v", err)
	}
	if len(papers) != 2 || papers[0] != "2603.11111" {
		t.Errorf("papers = %v", papers)
	}
}
