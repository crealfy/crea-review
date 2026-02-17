# Quickstart

Get crea-review running in 2 minutes.

---

## Install

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

---

## Prerequisites

crea-review requires crea-pipe for AI backend communication. Install it first:

```bash
curl -fsSL https://raw.githubusercontent.com/crealfy/crea-pipe/master/install.sh | bash
```

---

## Basic Usage

```bash
# Review uncommitted changes
creareview

# Review against main branch
creareview --base main

# Review specific commit range
creareview --base-commit abc123

# Include linter output
creareview --with-linters

# Sort by priority (high-risk first)
creareview --sort priority
```

---

## Next Steps

- **[Commands](commands/usage.md)** — Full CLI reference
- **[Batching](concepts/batching.md)** — How files are grouped
- **[Sessions](concepts/sessions.md)** — Review session management
