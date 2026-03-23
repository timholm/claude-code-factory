# Research-to-Product Pipeline — System Design

**Date:** 2026-03-22
**Status:** Approved
**Scope:** Three independent repos that together turn arXiv research into shipped products

---

## System Overview

Three repos, each with a single responsibility:

```
arxiv-archive          idea-engine              claude-code-factory
(data layer)           (intelligence layer)     (execution layer)

Sync arXiv papers  →   Find promising ideas  →  Build products
Store full text        7 papers + 7 repos       3-agent pipeline
Postgres + pgvector    per product concept       (build/seo/review)
HTTP API               Design product specs      Ship to GitHub
                       via llm-router

All three use llm-router for every LLM API call.
```

---

## Repo 1: arxiv-archive

**Purpose:** Full local mirror of arXiv CS/AI/ML papers with semantic search.

### Tech Stack
- **Language:** Go (single binary, arm64 compatible)
- **Database:** PostgreSQL with pgvector extension
- **Storage:** Flat text files on NFS-ZFS for full paper text
- **API:** HTTP REST server

### Data Sources
- **OAI-PMH** (metadata): arXiv's official bulk harvest API with resumption tokens
- **Semantic Scholar S2ORC** (full text): Parsed text from LaTeX sources, no PDF needed
- **Embedding** via llm-router: Abstract embeddings stored in pgvector for similarity search

### Categories Archived
`cs.AI, cs.CL, cs.LG, cs.SE, cs.CV, stat.ML` (~800K papers, ~100GB)

### Database Schema

```sql
-- Paper metadata
CREATE TABLE papers (
    arxiv_id        TEXT PRIMARY KEY,
    title           TEXT NOT NULL,
    abstract        TEXT,
    authors         TEXT,                -- JSON array
    categories      TEXT,                -- JSON array
    published       DATETIME,
    updated         DATETIME,
    doi             TEXT,
    journal_ref     TEXT,
    has_full_text   BOOLEAN DEFAULT FALSE,
    full_text_path  TEXT,
    embedding       vector(1536),        -- pgvector: abstract embedding
    fetched_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_papers_categories ON papers(categories);
CREATE INDEX idx_papers_published ON papers(published);
CREATE INDEX idx_papers_fulltext ON papers(has_full_text);
CREATE INDEX idx_papers_embedding ON papers USING ivfflat (embedding vector_cosine_ops);

-- Citation graph
CREATE TABLE refs (
    from_id     TEXT NOT NULL,
    to_id       TEXT NOT NULL,
    PRIMARY KEY (from_id, to_id)
);
CREATE INDEX idx_refs_to ON refs(to_id);

-- Sync state
CREATE TABLE sync_state (
    key     TEXT PRIMARY KEY,
    value   TEXT
);
```

### Sync Pipeline

```
archive sync (daily CronJob, 4 AM)

Step 1: OAI-PMH Harvest
  - Resume from last token (incremental)
  - Upsert metadata into Postgres
  - First sync: ~800K papers, 2-3 days
  - Daily: ~500-2000 new papers, ~5 minutes

Step 2: S2ORC Full Text Fetch
  - Batch query Semantic Scholar API for papers WHERE has_full_text = FALSE
  - Save text to /srv/arxiv/{category}/{arxiv_id}.txt
  - Rate: 100 req/sec with S2 API key

Step 3: Embed Abstracts
  - Papers WHERE embedding IS NULL
  - Send through llm-router (cached, deduped, cost-tracked)
  - Batch: 50 abstracts per call
  - Store 1536-dim vector in pgvector column

Step 4: Extract References
  - Parse citation links from S2ORC response
  - Insert into refs table (from_id, to_id)
```

### HTTP API

```
GET  /papers/:id              — full paper (metadata + text + refs)
GET  /papers/search?q=...     — full-text search (Postgres ts_vector)
GET  /papers/similar/:id      — vector similarity (pgvector cosine)
GET  /papers/similar?q=...    — vector similarity from free-text query
GET  /papers/:id/refs         — papers this one cites
GET  /papers/:id/cited-by     — papers that cite this one
GET  /papers/recent?cat=...&days=N  — recent papers by category
GET  /stats                   — archive stats
POST /sync                    — trigger manual sync
```

### CLI Commands

```
archive sync                     — full pipeline
archive sync --step metadata     — only OAI-PMH
archive sync --step fulltext     — only S2ORC fetch
archive sync --step embed        — only embedding
archive search "query"           — full-text search
archive similar 2603.16514       — vector similarity
archive read 2603.16514          — print paper
archive refs 2603.16514          — citation graph
archive serve --addr :9090       — HTTP API server
archive stats                    — print stats
```

### Config

```
POSTGRES_URL          — Postgres connection string (required)
ARXIV_DATA_DIR        — flat file storage (default: /srv/arxiv)
LLM_ROUTER_URL        — for embedding calls (required)
S2_API_KEY            — Semantic Scholar API key (optional, faster with key)
ARXIV_CATEGORIES      — comma-separated (default: cs.AI,cs.CL,cs.LG,cs.SE,cs.CV,stat.ML)
```

### Deployment

- `archive serve` → K8s Deployment (always-on, port 9090)
- `archive sync` → K8s CronJob (daily 4 AM)
- PostgreSQL → K8s StatefulSet with pgvector extension
- PVC: `arxiv-data` (100Gi, nfs-zfs) for flat text files

---

## Repo 2: idea-engine

**Purpose:** Autonomous research agent that finds promising papers, gathers context (7 papers + 7 repos), and synthesizes product specs.

### Tech Stack
- **Language:** Go (single binary)
- **Dependencies:** arxiv-archive HTTP API, GitHub API, llm-router
- **Output:** Product specs (JSON) posted to factory's build queue or written to a shared queue

### Pipeline

```
idea-engine run (daily CronJob, 5 AM — after archive sync)

Step 1: Discover Candidates
  - Query arxiv-archive: GET /papers/recent?cat=cs.AI&days=7
  - Query arxiv-archive: GET /papers/search?q=<trending topics>
  - Filter: published in last 30 days, has full text, CS/AI category
  - Score by: citation velocity, category relevance, novelty
  - Select top 30 candidates

Step 2: Deep Research (per candidate)
  For each candidate paper:

  a) Read full text
     - GET /papers/:id from arxiv-archive
     - Extract key technique, results, limitations

  b) Find 7 related papers
     - GET /papers/similar/:id (vector similarity from archive)
     - GET /papers/:id/cited-by (papers building on this work)
     - GET /papers/:id/refs (papers this one builds on)
     - Rank by relevance, pick top 7
     - Read abstracts of all 7

  c) Find 7 GitHub repos
     - Search GitHub: paper title keywords → implementations
     - Search GitHub: technique name → related tools
     - Extract repo URLs mentioned in the paper text
     - Filter: stars > 10, updated in last 12 months, not archived
     - For each repo: fetch README + file tree via GitHub API
     - Rank by relevance + stars, pick top 7

Step 3: Synthesize Product Spec (per candidate)
  Send to Claude via llm-router:

  Context provided:
    - Candidate paper (full text)
    - 7 related papers (abstracts + key findings)
    - 7 GitHub repos (READMEs + file trees)

  Prompt:
    "You are a product engineer. Given this research and existing implementations:
     1. What product can be built that COMBINES the best techniques from these papers?
     2. What do existing repos do well? What gaps do they leave?
     3. Who pays for this product? What's the form factor (CLI, API, library)?
     4. Design a product spec that improves on all existing work.
     Output a JSON product spec."

  Output: ProductSpec JSON (name, problem, solution, language, files, estimated_lines,
          source_papers: [7 arxiv IDs], source_repos: [7 github URLs])

Step 4: Quality Gate
  - Discard specs where the product already exists (GitHub search for exact name)
  - Discard specs scoring below novelty threshold
  - Rank remaining by commercial potential
  - Output top 10-15 specs per run

Step 5: Deliver to Factory
  - POST specs to factory's build queue (via registry API or shared Postgres)
  - Or: write specs to a JSON file in a shared directory
  - Factory's build phase picks them up on its next run
```

### Database Schema (separate Postgres DB or schema)

```sql
-- Candidate papers being researched
CREATE TABLE candidates (
    id              SERIAL PRIMARY KEY,
    arxiv_id        TEXT NOT NULL,
    discovered_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
    status          TEXT DEFAULT 'pending',  -- pending, researching, synthesized, delivered, skipped
    score           FLOAT,                   -- commercial potential score
    research_json   TEXT,                    -- cached research context (7 papers + 7 repos)
    spec_json       TEXT,                    -- generated product spec
    delivered_at    DATETIME
);

-- Track which ideas have been shipped (dedup)
CREATE TABLE shipped_ideas (
    name            TEXT PRIMARY KEY,        -- product name (kebab-case)
    arxiv_id        TEXT,
    shipped_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

### CLI Commands

```
idea-engine run                  — full pipeline: discover → research → synthesize → deliver
idea-engine discover             — only find candidates
idea-engine research <arxiv_id>  — deep research one paper
idea-engine synthesize <arxiv_id> — generate product spec for one paper
idea-engine deliver              — push ready specs to factory
idea-engine list                 — show pipeline status
idea-engine stats                — throughput, success rate, etc.
```

### Config

```
ARCHIVE_URL           — arxiv-archive HTTP API (e.g., http://localhost:9090)
LLM_ROUTER_URL        — for all Claude calls (required)
GITHUB_TOKEN          — for GitHub repo search
POSTGRES_URL          — for idea-engine state (can share with archive or separate)
FACTORY_QUEUE_URL     — where to deliver specs (factory API or shared dir)
CANDIDATES_PER_RUN    — how many papers to research per run (default: 30)
SPECS_PER_RUN         — max specs to output per run (default: 15)
```

### Deployment

- `idea-engine run` → K8s CronJob (daily 5 AM, after archive sync at 4 AM)
- Shares Postgres with arxiv-archive (different schema/tables)
- Needs: arxiv-archive service running, llm-router running, GitHub token

---

## Repo 3: claude-code-factory (Updates)

**What changes:** The factory's `gather` and `analyze` phases are replaced by idea-engine. The factory becomes a pure build executor.

### Removed
- `internal/gather/` — entire package (arxiv scraper removed)
- `internal/analyze/` — entire package (Claude analysis removed)
- `prompts/analyze.md.tmpl` — no longer needed
- `factory gather` command — removed
- `factory analyze` command — removed

### Added
- `factory import` command — reads specs from idea-engine's output (JSON file or API)
- Or: idea-engine directly inserts into factory's `build_queue` table

### Unchanged
- `internal/build/` — 3-agent pipeline (build → seo → review)
- `internal/mirror/` — GitHub push
- `internal/registry/` — SQLite for build queue + repos
- All llm-router integration (ANTHROPIC_BASE_URL routing, feedback, etc.)

### New Flow

```
OLD: factory gather → factory analyze → factory build → factory mirror
NEW: idea-engine delivers specs → factory build → factory mirror
```

The factory's `build_queue` table becomes the handoff point. Idea-engine writes specs, factory reads and builds them.

---

## Cross-Repo Data Flow

```
                    ┌────────────────┐
                    │  arxiv-archive │
                    │  :9090         │
                    │                │
                    │  800K papers   │
                    │  pgvector      │
                    │  citation graph│
                    └───────┬────────┘
                            │ HTTP API
                    ┌───────▼────────┐
                    │  idea-engine   │
                    │  (daily 5 AM)  │
                    │                │
                    │  7 papers      │───── GitHub API ────▶ 7 repos
                    │  + 7 repos     │
                    │  → Claude      │───── llm-router ───▶ Claude
                    │  → spec        │
                    └───────┬────────┘
                            │ specs (JSON)
                    ┌───────▼────────┐
                    │  factory       │
                    │  (daily 6 AM)  │
                    │                │
                    │  build/seo/    │───── llm-router ───▶ Claude
                    │  review        │
                    │  → git repos   │
                    └───────┬────────┘
                            │ git push
                    ┌───────▼────────┐
                    │  GitHub        │
                    │  (daily 11 PM) │
                    └────────────────┘

llm-router (:8080) sits behind ALL Claude calls across all three repos.
```

---

## Scheduling

```
4:00 AM  — arxiv-archive sync (pull new papers, embed, index)
5:00 AM  — idea-engine run (research candidates, synthesize specs)
6:30 AM  — factory build (build specs from idea-engine)
11:00 PM — factory mirror (push to GitHub)
```

---

## Implementation Order

1. **arxiv-archive** — build first, it's the foundation
2. **idea-engine** — depends on archive being populated
3. **claude-code-factory updates** — remove gather/analyze, add import

---

## Success Criteria

- Archive contains 800K+ papers with full text and embeddings
- Idea-engine produces 10-15 product specs per day
- Each spec is backed by 7 papers + 7 repos (real research, not hallucinated)
- Factory builds and ships products through the 3-agent pipeline
- All LLM calls route through llm-router (cached, tracked, cost-optimized)
- Daily autonomous cycle: sync → research → build → ship
