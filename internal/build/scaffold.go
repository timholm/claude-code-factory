package build

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Scaffold generates boilerplate files for a new project in dir.
// All languages get LICENSE (MIT) and SPEC.md.
// Language-specific files are generated based on the language parameter.
// Unknown languages fall back to bash behaviour.
func Scaffold(dir, name, language, problem, sourceURL, solution, files string, lines int, ghUser string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("scaffold: mkdir %s: %w", dir, err)
	}

	// Always generate LICENSE, SPEC.md, and CLAUDE.md
	if err := writeLicense(dir, ghUser); err != nil {
		return err
	}
	if err := writeSpec(dir, name, language, problem, sourceURL, solution, files, lines); err != nil {
		return err
	}
	if err := writeClaudeMD(dir, name, language); err != nil {
		return err
	}

	// Language-specific boilerplate
	switch strings.ToLower(language) {
	case "go":
		return scaffoldGo(dir, name)
	case "python":
		return scaffoldPython(dir, name)
	case "typescript", "ts":
		return scaffoldTypeScript(dir, name)
	default:
		// bash (and any unknown language)
		return scaffoldBash(dir, name)
	}
}

// writeLicense writes an MIT LICENSE file.
func writeLicense(dir, ghUser string) error {
	year := time.Now().Year()
	author := ghUser
	if author == "" {
		author = "Factory"
	}
	content := fmt.Sprintf(`MIT License

Copyright (c) %d %s

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
`, year, author)
	return writeFile(filepath.Join(dir, "LICENSE"), content)
}

// writeSpec writes SPEC.md with project metadata and the problem/solution.
func writeSpec(dir, name, language, problem, sourceURL, solution, files string, lines int) error {
	content := fmt.Sprintf(`# %s

**Language:** %s
**Source:** %s
**Estimated lines:** %d

## Problem

%s

## Solution

%s

## Expected Files

%s
`, name, language, sourceURL, lines, problem, solution, files)
	return writeFile(filepath.Join(dir, "SPEC.md"), content)
}

// scaffoldGo generates Go-specific boilerplate: go.mod, Makefile, .gitignore.
func scaffoldGo(dir, name string) error {
	goMod := fmt.Sprintf(`module github.com/factory/%s

go 1.22
`, name)
	if err := writeFile(filepath.Join(dir, "go.mod"), goMod); err != nil {
		return err
	}

	makefile := fmt.Sprintf(`.PHONY: build test clean

build:
	go build -o bin/%s ./...

test:
	go test ./...

clean:
	rm -rf bin/
`, name)
	if err := writeFile(filepath.Join(dir, "Makefile"), makefile); err != nil {
		return err
	}

	gitignore := `bin/
*.test
*.out
vendor/
`
	return writeFile(filepath.Join(dir, ".gitignore"), gitignore)
}

// scaffoldPython generates Python-specific boilerplate: pyproject.toml, Makefile, .gitignore.
func scaffoldPython(dir, name string) error {
	pyproject := fmt.Sprintf(`[project]
name = "%s"
version = "0.1.0"
requires-python = ">=3.11"

[build-system]
requires = ["setuptools>=68"]
build-backend = "setuptools.backends.legacy:build"
`, name)
	if err := writeFile(filepath.Join(dir, "pyproject.toml"), pyproject); err != nil {
		return err
	}

	makefile := `.PHONY: test lint

test:
	python3 -m pytest

lint:
	python3 -m ruff check .
`
	if err := writeFile(filepath.Join(dir, "Makefile"), makefile); err != nil {
		return err
	}

	gitignore := `__pycache__/
*.pyc
*.pyo
.venv/
dist/
*.egg-info/
.pytest_cache/
`
	return writeFile(filepath.Join(dir, ".gitignore"), gitignore)
}

// scaffoldTypeScript generates TypeScript-specific boilerplate: package.json, tsconfig.json, Makefile, .gitignore.
func scaffoldTypeScript(dir, name string) error {
	pkgJSON := fmt.Sprintf(`{
  "name": "%s",
  "version": "0.1.0",
  "private": true,
  "scripts": {
    "build": "tsc",
    "test": "node --test"
  },
  "devDependencies": {
    "typescript": "^5.0.0"
  }
}
`, name)
	if err := writeFile(filepath.Join(dir, "package.json"), pkgJSON); err != nil {
		return err
	}

	tsconfig := `{
  "compilerOptions": {
    "target": "ES2022",
    "module": "NodeNext",
    "moduleResolution": "NodeNext",
    "outDir": "dist",
    "strict": true,
    "esModuleInterop": true
  },
  "include": ["src/**/*"]
}
`
	if err := writeFile(filepath.Join(dir, "tsconfig.json"), tsconfig); err != nil {
		return err
	}

	makefile := `.PHONY: build test clean

build:
	npm run build

test:
	npm test

clean:
	rm -rf dist/ node_modules/
`
	if err := writeFile(filepath.Join(dir, "Makefile"), makefile); err != nil {
		return err
	}

	gitignore := `node_modules/
dist/
*.js.map
`
	return writeFile(filepath.Join(dir, ".gitignore"), gitignore)
}

// scaffoldBash generates bash-specific boilerplate: Makefile (shellcheck), .gitignore.
func scaffoldBash(dir, name string) error {
	makefile := fmt.Sprintf(`.PHONY: test lint

test:
	bash -n %s.sh

lint:
	shellcheck %s.sh
`, name, name)
	if err := writeFile(filepath.Join(dir, "Makefile"), makefile); err != nil {
		return err
	}

	gitignore := `*.log
tmp/
`
	return writeFile(filepath.Join(dir, ".gitignore"), gitignore)
}

// writeClaudeMD writes a CLAUDE.md that helps Claude Code orient quickly in the workspace.
// This reduces wasted turns — Claude reads CLAUDE.md automatically on startup.
func writeClaudeMD(dir, name, language string) error {
	testCmd := "make test"
	buildCmd := "make build"

	content := fmt.Sprintf(`# %s

## Build & Test
- Build: %s
- Test: %s
- Language: %s

## Instructions
- Read SPEC.md for full requirements
- Implement all functionality described in SPEC.md
- Write real tests that pass
- Run make test before finishing
`, name, buildCmd, testCmd, language)

	return writeFile(filepath.Join(dir, "CLAUDE.md"), content)
}

// writeFile writes content to path, creating parent directories as needed.
func writeFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("scaffold: mkdir for %s: %w", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("scaffold: write %s: %w", path, err)
	}
	return nil
}
