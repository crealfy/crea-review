# Review Documentation

**crea-review** is an AI-powered code review tool with intelligent batching, session tracking, and priority scoring.

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
```

---

## Key Features

| Feature | Description |
|---------|-------------|
| **Intelligent batching** | Groups related files (packages, test pairs) |
| **Session tracking** | Resume reviews, re-review specific files |
| **Priority scoring** | Focus on high-risk changes first |
| **Linter integration** | Include linter output in context |
| **AI-powered analysis** | Uses Claude or Codex for review |

---

## Quick Navigation

- **[Quickstart](quickstart.md)** — Get running in 2 minutes
- **[Commands](commands/usage.md)** — CLI usage and flags
- **[Concepts](concepts/batching.md)** — How batching, sessions, and priority work

---

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/crealfy/crea-review/master/install.sh | bash
```

See **[Quickstart](quickstart.md)** for other installation options.
