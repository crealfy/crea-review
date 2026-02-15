# CLAUDE.md

# IT IS YEAR 2026 !!! Please use 2026 in web searches!!!

Guidance for Claude Code when working with crea-review.

## Project Overview

crea-review is a **Go CLI** that provides AI-powered code review with intelligent batching, session tracking, and priority scoring. It replaces CodeRabbit with a more flexible, language-agnostic review tool.

**Key design**: Uses `crea-pipe/pkg/*` as a library for AI communication (no binary dependency).

**CLI binary**: `creareview` (no hyphen)

---

## Critical Rules

### 1. crea-pipe: Import Directly

Use `github.com/crealfy/crea-pipe/pkg/*` packages directly. **NO wrappers, NO re-implementations.**

```go
// GOOD - Direct import
import "github.com/crealfy/crea-pipe/pkg/git"
diff, _ := git.Diff(ctx, path, from, to)

// GOOD - Use agent directly
import "github.com/crealfy/crea-pipe/pkg/agent/claude"
agent := claude.New()
response, _ := agent.Run(ctx, prompt)

// BAD - Re-implementing what crea-pipe already has
func getDiff(path, from, to string) string { ... }  // Don't do this
```

### 2. go-toolkit: Import Directly

Use `github.com/valksor/go-toolkit` packages directly. **NO type aliases, NO wrapper functions.**

Mark code that could move to go-toolkit later with:
```go
// TODO(go-toolkit): candidate for go-toolkit
```

### 3. Tests Required

Every feature MUST include:

| Requirement  | Location                   | Target        |
|--------------|----------------------------|---------------|
| Unit tests   | `*_test.go` next to source | 80%+ coverage |

Write tests FIRST (TDD). Use table-driven tests.

### 4. Quality Checks

Run checks **only for code you changed**:

| Changed              | Command                                     |
|----------------------|---------------------------------------------|
| `cmd/`, `pkg/`, `*.go` | `make quality` + targeted tests           |
| `*.md`               | None                                        |

### 5. Use Make Commands

Always use `make` commands:

| Operation | Command              |
|-----------|----------------------|
| Build     | `make build`         |
| Test      | `make test`          |
| Quality   | `make quality`       |
| Format    | `make fmt`           |
| Install   | `make install`       |

### 6. No nolint Abuse

**`//nolint` is a LAST RESORT.** Always: specify linter name, include justification.

### 7. File Size < 500 Lines

Keep all Go files under 500 lines.

### 8. Git Command Policy

Same as crea-pipe:
- **Tier 1** (always allowed): read-only commands
- **Tier 2** (user-requested only): add, commit
- **Tier 3** (never): push, pull, merge, rebase, reset

---

## Commands

### Build & Development

```bash
make build | install | test | coverage | quality | fmt | tidy | hooks | race
```

### CLI Usage

```bash
creareview [flags]

# CodeRabbit-compatible flags
  -t, --type string      Review type: all, committed, uncommitted
  --base string          Base branch for comparison
  --base-commit string   Base commit for comparison
  --cwd string           Working directory
  -c, --config files     Additional instruction files
  --plain                Plain text output
  --prompt-only          AI-optimized output (pipeable)
  --no-color             Disable colors

# creareview-specific flags
  --backend string       AI backend: claude, codex (default: claude)
  --with-linters         Include linter output
  --max-files int        Max files per batch (default: 50)
  --sort string          Sort: priority, none, alpha, modified, commit-new, commit-old
  --session int          Re-review files from session N
  --continue int         Continue from session N
  --list-sessions        List all sessions
```

---

## Architecture

### Entry Point

| Path                        | Description                          |
|-----------------------------|--------------------------------------|
| `cmd/creareview/main.go`    | CLI entry → flag parsing → review    |

### Core Packages

| Package               | Responsibility                                              |
|-----------------------|-------------------------------------------------------------|
| `pkg/context/`        | Gather diff, files, related files, linter output            |
| `pkg/session/`        | Review session management and persistence                   |
| `pkg/priority/`       | File priority scoring and critical path detection           |
| `pkg/batch/`          | Smart file batching (package groups, test pairs)            |
| `pkg/review/`         | Build prompts and call AI via crea-pipe/pkg/agent           |
| `pkg/output/`         | Format findings as JSON for piping                          |

### Imported from crea-pipe

| Package                       | Usage                                      |
|-------------------------------|-------------------------------------------|
| `crea-pipe/pkg/git`           | Diff, Status, CurrentBranch, RepoRoot     |
| `crea-pipe/pkg/agent`         | Agent interface                           |
| `crea-pipe/pkg/agent/claude`  | Claude CLI agent                          |
| `crea-pipe/pkg/agent/codex`   | Codex CLI agent                           |
| `crea-pipe/pkg/session`       | Session storage pattern (adapted)         |

### Data Flow

```
creareview --base main
  ├─ Parse flags
  ├─ git.Diff() → raw diff
  ├─ Parse diff → file list
  ├─ Score files by priority
  ├─ Batch related files
  ├─ For each batch:
  │   ├─ Build review prompt
  │   ├─ agent.Run(prompt) → AI response
  │   └─ Parse findings
  ├─ Save session state
  └─ Output JSON with implementation_prompt
```

---

## Code Style

- **Imports**: stdlib → third-party → crea-pipe → local (alphabetical)
- **Naming**: PascalCase exported, camelCase unexported
- **Errors**: `fmt.Errorf("prefix: %w", err)`
- **Logging**: `log/slog`
- **Modern Go**: `slices.Contains()`, `maps.Clone()`, `context.Context`

---

## State Directory

```
~/.valksor/crealfy/review/
└── <project-hash>/
    ├── project.json
    └── sessions/
        ├── 1/
        │   ├── meta.json
        │   ├── files.json
        │   └── findings.json
        └── latest -> 1/
```
