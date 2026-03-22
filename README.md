# claude-code-factory

An autonomous software factory that continuously monitors GitHub issues, Hacker News, and Reddit for pain points in Claude Code, then uses Claude to generate, build, test, and publish minimal focused tools that address those problems. It runs on a K8s cluster, wakes on a schedule, and ships 15-30 new repositories per day without human intervention.

## Architecture

```
factory binary
    |
    +-- gather  --> scrapes GitHub / HN / Reddit --> SQLite registry (raw_items)
    |
    +-- analyze --> Claude ranks problems, writes build specs --> SQLite registry (build_queue)
    |
    +-- build   --> Claude scaffolds + implements + tests each spec --> bare git repos (FACTORY_GIT_DIR)
    |
    +-- mirror  --> pushes bare repos to GitHub with 30s stagger
```

## How It Works

**Gather (hourly)** — Scrapes GitHub issues tagged with `claude-code`, Hacker News stories and comments, and relevant Reddit threads. Deduplicates by URL and stores raw items in SQLite.

**Analyze (daily)** — Claude reads the accumulated raw items and selects the top 30 problems worth solving. For each problem it writes a build spec: repo name, description, language, and a concise implementation brief.

**Build (daily)** — For each queued spec, Claude scaffolds a project directory, implements the solution, writes tests, and commits the result to a bare git repo. Each build takes roughly 5-10 minutes.

**Mirror (daily)** — Pushes each bare repo to GitHub under the configured user account, staggering pushes by 30 seconds to avoid rate limits. Marks repos as mirrored in the registry.

## Prerequisites

- Kubernetes cluster (arm64 nodes)
- Claude Code Max plan (the build step runs `claude` as a subprocess)
- GitHub personal access token with `repo` scope
- `gh` CLI installed and authenticated on the cluster nodes

## Quickstart

```sh
# Build
make build

# Test locally
FACTORY_DATA_DIR=/tmp/factory GITHUB_TOKEN=ghp_xxx ./factory gather

# Deploy to K8s
kubectl apply -f deploy/
```

## Configuration

| Variable | Default | Description |
|---|---|---|
| `GITHUB_TOKEN` | (required) | GitHub personal access token |
| `GITHUB_USER` | (required) | GitHub username for mirroring repos |
| `FACTORY_DATA_DIR` | `/srv/factory` | Directory for SQLite registry and working files |
| `FACTORY_GIT_DIR` | `/srv/git` | Directory for bare git repositories |
| `CLAUDE_BINARY` | `claude` | Path to the Claude Code binary |
| `REDDIT_USER_AGENT` | `factory/1.0` | User-agent string for Reddit API requests |

## Projected Throughput

- 15-30 new repositories per day
- 450-900 per month
- 2,000+ repositories within 3-5 months
