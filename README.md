# Crealfy Review — AI-Powered Code Review

[![valksor](https://badgen.net/static/org/valksor/green)](https://github.com/valksor)
[![BSD-3-Clause](https://img.shields.io/badge/BSD--3--Clause-green?style=flat)](https://github.com/crealfy/crea-review/blob/master/LICENSE)
[![GitHub Release](https://img.shields.io/github/release/crealfy/crea-review.svg?style=flat)](https://github.com/crealfy/crea-review/releases/latest)
[![GitHub last commit](https://img.shields.io/github/last-commit/crealfy/crea-review.svg?style=flat)](https://github.com/crealfy/crea-review/commits/master)

[![Go Report Card](https://goreportcard.com/badge/github.com/crealfy/crea-review)](https://goreportcard.com/report/github.com/crealfy/crea-review)
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/crealfy/crea-review)

---

**crea-review** is a Go CLI that provides AI-powered code review with intelligent batching, session tracking, and priority scoring. It replaces CodeRabbit with a more flexible, language-agnostic review tool.

**CLI binary**: `creareview` (no hyphen)

---

## What It Does

Traditional code review tools process files individually. crea-review intelligently batches related files, tracks review sessions, and prioritizes files by risk score.

```bash
# Review uncommitted changes
creareview

# Review against a branch
creareview --base main

# Review with priority sorting
creareview --sort priority

# Continue a previous session
creareview --continue 1
```

---

## Key Features

| Feature | Description |
|---------|-------------|
| **Intelligent batching** | Groups related files (packages, test pairs) |
| **Session tracking** | Resume reviews, re-review specific files |
| **Priority scoring** | Focus on high-risk changes first |
| **Linter integration** | Include linter output in context |
| **AI-powered analysis** | Uses Claude or Codex via crea-pipe |

---

## Installation

### Install Script (Recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/crealfy/crea-review/master/install.sh | bash
```

### Pre-built Binary

```bash
# Download for your platform (macOS ARM64 example)
curl -L https://github.com/crealfy/crea-review/releases/latest/download/creareview-darwin-arm64 -o creareview
chmod +x creareview
sudo mv creareview /usr/local/bin/
```

### Build from Source

```bash
git clone https://github.com/crealfy/crea-review.git
cd crea-review
make install
```

### Prerequisites

crea-review uses crea-pipe for AI backend communication:

```bash
curl -fsSL https://raw.githubusercontent.com/crealfy/crea-pipe/master/install.sh | bash
```

---

## CLI Usage

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

# crea-review specific flags
  --backend string       AI backend: claude, codex (default: claude)
  --with-linters         Include linter output
  --max-files int        Max files per batch (default: 50)
  --sort string          Sort: priority, none, alpha, modified, commit-new, commit-old
  --session int          Re-review files from session N
  --continue int         Continue from session N
  --list-sessions        List all sessions
```

---

## Documentation

- [Full Documentation](https://crealfy.com/docs/crea-review/nightly)
- [Quickstart](https://crealfy.com/docs/crea-review/nightly/#/quickstart)

---

## Development

```bash
make build        # Build binary
make test         # Run tests
make quality      # Lint and security checks
make install      # Install to $GOPATH/bin
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

---

## License

[BSD 3-Clause License](LICENSE)
