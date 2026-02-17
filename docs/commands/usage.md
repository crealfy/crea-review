# CLI Usage

## Synopsis

```bash
creareview [flags]
```

Note: The binary is named `creareview` (no hyphen).

## Flags

### CodeRabbit-compatible Flags

| Flag | Description |
|------|-------------|
| `-t, --type` | Review type: `all`, `committed`, `uncommitted` |
| `--base` | Base branch for comparison |
| `--base-commit` | Base commit for comparison |
| `--cwd` | Working directory |
| `-c, --config` | Additional instruction files |
| `--plain` | Plain text output |
| `--prompt-only` | AI-optimized output (pipeable) |
| `--no-color` | Disable colors |

### crea-review Specific Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--backend` | `claude` | AI backend: `claude` or `codex` |
| `--with-linters` | `false` | Include linter output |
| `--max-files` | `50` | Max files per batch |
| `--sort` | `none` | Sort: `priority`, `none`, `alpha`, `modified`, `commit-new`, `commit-old` |
| `--session` | - | Re-review files from session N |
| `--continue` | - | Continue from session N |
| `--list-sessions` | `false` | List all sessions |

## Examples

```bash
# Review uncommitted changes
creareview

# Review against main branch
creareview --base main

# Review with priority sorting
creareview --sort priority

# Include linter context
creareview --with-linters

# Continue a previous session
creareview --continue 1

# List all sessions
creareview --list-sessions
```

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error |
| 2 | No changes to review |
