# claude-code-factory

An autonomous software factory that turns cutting-edge research papers into production-grade, monetizable developer tools — without human intervention.

It scrapes arXiv daily for the latest AI/ML research, uses Claude to identify the most commercially promising techniques, builds complete implementations with tests and documentation, and ships them to GitHub. Every LLM API call routes through [llm-router](https://github.com/timholm/llm-router) for cost-optimized model selection, semantic caching, and request deduplication.

---

## How It Works

```
┌─────────────────────────────────────────────────────────────────┐
│                    claude-code-factory                            │
│                                                                  │
│  ┌──────────┐   ┌──────────┐   ┌──────────┐   ┌──────────┐    │
│  │  GATHER   │──▶│ ANALYZE  │──▶│  BUILD   │──▶│  MIRROR  │    │
│  │ (hourly)  │   │ (daily)  │   │ (daily)  │   │ (nightly)│    │
│  │           │   │          │   │          │   │          │    │
│  │ arXiv API │   │ Claude   │   │ 3-agent  │   │ GitHub   │    │
│  │ 36 query  │   │ ranks    │   │ pipeline │   │ push     │    │
│  │ categories│   │ papers   │   │ per repo │   │ staggered│    │
│  │ ~500/run  │   │ → specs  │   │          │   │          │    │
│  └──────────┘   └──────────┘   └──────────┘   └──────────┘    │
│                       │              │                           │
│                       ▼              ▼                           │
│                ┌─────────────────────────┐                      │
│                │      llm-router         │                      │
│                │  Cost routing, caching, │                      │
│                │  dedup, health tracking │                      │
│                └─────────────────────────┘                      │
└─────────────────────────────────────────────────────────────────┘
```

### Gather (hourly)

Scrapes arXiv across **36 research categories** targeting monetizable LLM tooling:

- **Inference & serving** — speculative decoding, KV cache, quantization, model compression
- **RAG & retrieval** — dense retrieval, embeddings, retrieval-augmented generation
- **Agents & tool use** — planning, multi-agent, code agents
- **Fine-tuning & alignment** — LoRA, DPO, RLHF, direct preference optimization
- **Safety & guardrails** — prompt injection defense, hallucination detection, content filtering
- **Code generation** — repair, review, test generation
- **Cost optimization** — LLM routing, model cascading, token optimization
- **Context & memory** — long-context, compression, memory-augmented models
- **Multimodal** — vision-language, multimodal agents
- **Structured output** — JSON generation, function calling

Each query fetches the 15 most recent papers. Papers are deduplicated by URL and stored in SQLite for analysis.

### Analyze (daily)

Sends accumulated papers to Claude via llm-router. The prompt acts as a **product manager**:

> "Who pays for this? What's the integration point? What's the moat? Can an MVP ship in 300 lines?"

Outputs **30 product specs** per run, ranked by commercial potential. Each spec includes: repo name, problem statement, solution brief, language, expected files, and estimated complexity.

**llm-router integration:** Analysis routes through the router for semantic caching (similar paper batches return cached results), model selection (Haiku for analysis = cheap), and request deduplication. Falls back to Claude CLI if the router is unavailable.

### Build (daily) — The 3-Agent Pipeline

Each project goes through three sequential Claude Code phases with validation gates between them:

```
┌───────────────┐     ┌──────────────┐     ┌───────────────┐
│  AGENT 1:     │     │  AGENT 2:    │     │  AGENT 3:     │
│  BUILD        │────▶│  SEO         │────▶│  REVIEW       │
│               │     │              │     │               │
│  Model:Sonnet │     │  Model:Haiku │     │  Model:Sonnet │
│  Turns: 10-20 │     │  Turns: 8    │     │  Turns: 10    │
│  Full tools   │     │  No Bash     │     │  Full tools   │
│               │     │              │     │               │
│  Implements   │     │  README.md   │     │  Runs tests   │
│  core code    │     │  llms.txt    │     │  Fixes bugs   │
│  + tests      │     │  CLAUDE.md   │     │  Verifies docs│
│               │     │  AGENTS.md   │     │               │
└───────┬───────┘     └──────┬───────┘     └───────┬───────┘
        │                    │                     │
   validate:            validate:             validate:
   source files         SEO files             make test
   exist                exist                 MUST PASS
```

**Agent 1 — Build (Sonnet, 10-20 turns):** Reads SPEC.md, implements complete working code with real tests. No placeholders, no TODOs. Uses idiomatic patterns for the target language.

**Agent 2 — SEO (Haiku, 8 turns):** Generates LLM-discovery artifacts without modifying source code. Creates README.md (structured for both humans and AI agents), llms.txt (following the llms.txt standard), CLAUDE.md (instructions for Claude Code), and AGENTS.md (instructions for any AI coding agent). Tool-restricted: can only Read, Write, Edit, Glob, Grep — no Bash execution.

**Agent 3 — Review (Sonnet, 10 turns):** Code reviewer. Runs tests, fixes failures, verifies README accuracy, checks for dead code and obvious bugs. Does NOT add features or rewrite working code.

**Final gate:** `make test` must pass or the project is marked as failed and not shipped.

**Status flow:** `queued → building → seo → reviewing → shipped/failed`

**Parallel builds:** Configurable via `BUILD_WORKERS` env var. Runs N Claude Code processes simultaneously with semaphore-based concurrency control.

### Mirror (nightly)

Pushes shipped repos to GitHub under the configured user account. Creates public repos with descriptions, pushes via `--mirror`, staggers by 30 seconds to avoid rate limits.

---

## llm-router Integration

**Every LLM API call in the factory routes through [llm-router](https://github.com/timholm/llm-router).** When `LLM_ROUTER_URL` is set, the factory sets `ANTHROPIC_BASE_URL` to point at the router, so all of Claude Code's internal API calls flow through it transparently.

This provides:

| Feature | How the factory uses it |
|---------|------------------------|
| **Cost-optimized routing** | Router classifies each API call and picks the cheapest Claude model that can handle it (Haiku/Sonnet/Opus) |
| **Semantic caching** | Similar prompts return cached responses at $0 cost |
| **Request deduplication** | Parallel builds making identical API calls get coalesced into one upstream request |
| **Health-aware backends** | If Anthropic returns 5xx, the router marks it degraded and deprioritizes |
| **Cost tracking** | Every API call tracked per-model at `/stats` |
| **Feedback loop** | After each build ships/fails, the factory reports quality scores to `/v1/feedback` so the router auto-calibrates routing thresholds over time |
| **OAuth auto-discovery** | Router reads `~/.claude/.credentials.json` — no manual API key setup needed with Claude Code Max |

---

## LLM-SEO Strategy

Every repo the factory produces is optimized for AI agent discovery:

- **README.md** — structured with: What it does, Install, Usage (real examples), API reference, Architecture overview
- **llms.txt** — follows the [llms.txt standard](https://llmstxt.org) for LLM-readable project documentation
- **CLAUDE.md** — instructions for Claude Code working in the repo (build/test commands, architecture, patterns)
- **AGENTS.md** — instructions for any AI coding agent (Copilot, Cursor, Codex, etc.)
- **SPEC.md** — original requirements and design context

The goal: when an AI coding agent encounters a problem, it finds and uses these tools automatically.

---

## Database

SQLite with WAL mode and 10-second busy timeout for concurrent access.

### Tables

**raw_items** — Gathered research papers
```
id, source, url (unique), title, body, score, author,
created_at, gathered_at, fed_to_claude
```

**build_queue** — Projects being built
```
id, name, problem, source_url, solution, language, files,
estimated_lines, status, attempts, queued_at, started_at,
shipped_at, error_log
```

**repos** — Shipped projects
```
name (PK), language, problem, source_url, created_at,
last_maintained, version, lines_of_code, has_tests,
tests_pass, github_pushed, github_push_at
```

---

## Installation

```bash
# Clone
git clone https://github.com/timholm/claude-code-factory.git
cd claude-code-factory

# Build
make build    # → bin/factory (17MB Go binary)

# Test
make test     # 37 tests
```

### Prerequisites

- **Go 1.22+** for building the factory binary
- **Claude Code Max** subscription (OAuth-based, no API key needed)
- **llm-router** running locally or on the cluster (optional but recommended)
- **GitHub token** with `repo` scope for mirroring
- **Python 3.9+** (for building Python projects)
- **Node.js 18+** (for building TypeScript projects)

---

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `GITHUB_TOKEN` | (required) | GitHub personal access token with `repo` scope |
| `GITHUB_USER` | (required) | GitHub username for mirroring repos |
| `FACTORY_DATA_DIR` | `/srv/factory` | Directory for SQLite registry |
| `FACTORY_GIT_DIR` | `/srv/git` | Directory for bare git repos |
| `CLAUDE_BINARY` | `claude` | Path to Claude Code CLI |
| `LLM_ROUTER_URL` | (optional) | llm-router URL (e.g., `http://localhost:8080`) |
| `BUILD_WORKERS` | `1` | Number of parallel build workers |

---

## Usage

### Local development

```bash
# Start llm-router (optional, recommended)
cd ~/llm-router && ./bin/llm-router --config config.yaml &

# Run each phase
export LLM_ROUTER_URL=http://localhost:8080
export FACTORY_DATA_DIR=/tmp/factory
export FACTORY_GIT_DIR=/tmp/git
export GITHUB_TOKEN=ghp_...
export GITHUB_USER=yourusername

./bin/factory gather                    # scrape arXiv
./bin/factory analyze                   # generate specs
./bin/factory build                     # build projects
BUILD_WORKERS=6 ./bin/factory build     # parallel builds
./bin/factory mirror                    # push to GitHub
```

### Kubernetes deployment

```bash
kubectl apply -f deploy/namespace.yaml
kubectl apply -f deploy/pvcs.yaml
kubectl apply -f deploy/secrets.yaml
kubectl apply -f deploy/cronjobs.yaml
```

**CronJob schedule:**
- Gather: hourly (`0 * * * *`)
- Analyze: daily 6 AM (`0 6 * * *`)
- Build: daily 6:30 AM (`30 6 * * *`), 16-hour timeout
- Mirror: daily 11 PM (`0 23 * * *`)

---

## Project Structure

```
cmd/factory/main.go          — CLI entry point (gather, analyze, build, mirror)
internal/
  build/
    build.go                 — 3-agent pipeline orchestration, parallel workers
    claude.go                — Claude Code invocation with llm-router routing
    scaffold.go              — Language-specific boilerplate generation
    validate_test.go         — Validation + template tests
  analyze/
    analyze.go               — Claude analysis with router fallback
  gather/
    arxiv.go                 — arXiv scraper (36 categories, 15 results each)
    scraper.go               — Scraper interface + utilities
    gather.go                — Orchestrator
  registry/
    db.go                    — SQLite schema + migrations
    build_queue.go           — Build queue CRUD
    repos.go                 — Repos CRUD
    raw_items.go             — Raw items CRUD
  mirror/
    mirror.go                — GitHub push with staggered delays
  config/
    config.go                — Environment variable loading
  llmrouter/
    client.go                — llm-router HTTP client
prompts/
  analyze.md.tmpl            — Research-to-product analysis prompt
  build.md.tmpl              — Implementation prompt
  seo.md.tmpl                — LLM-SEO documentation prompt
  review.md.tmpl             — Code review + test verification prompt
deploy/
  namespace.yaml             — K8s namespace
  pvcs.yaml                  — Persistent volume claims (5Gi data, 50Gi repos)
  secrets.yaml               — GitHub token, Claude credentials
  cronjobs.yaml              — 4 scheduled jobs
```

---

## Research Sources

The factory scrapes **36 arXiv query categories** across the most commercially promising areas of AI/ML:

| Domain | Queries | Example topics |
|--------|---------|---------------|
| Inference | 6 | Speculative decoding, KV cache, quantization, model compression |
| RAG | 3 | Retrieval-augmented generation, dense retrieval, embeddings |
| Evaluation | 2 | LLM benchmarks, hallucination detection |
| Agents | 4 | Tool use, planning, multi-agent, code agents |
| Fine-tuning | 4 | LoRA, RLHF, DPO, direct preference optimization |
| Safety | 4 | Prompt injection, guardrails, output filtering |
| Code | 3 | Generation, repair, review |
| Structured output | 3 | JSON, function calling |
| Cost | 3 | Routing, cascading, token optimization |
| Context | 3 | Long-context, compression, memory |
| Multimodal | 2 | Vision-language, multimodal agents |

---

## Roadmap

The factory is being decomposed into a three-repo pipeline:

1. **[arxiv-archive](docs/superpowers/specs/2026-03-22-research-to-product-pipeline-design.md)** — Full local mirror of 800K+ arXiv papers with PostgreSQL + pgvector for semantic search and citation graph traversal.

2. **idea-engine** — Autonomous research agent that reads papers deeply, finds 7 related papers + 7 GitHub repos per concept, and synthesizes product specs backed by real research.

3. **claude-code-factory** (this repo) — Pure build executor consuming specs from idea-engine.

See the [design spec](docs/superpowers/specs/2026-03-22-research-to-product-pipeline-design.md) for the full architecture.

---

## License

MIT
