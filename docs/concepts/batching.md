# Batching

crea-review intelligently groups files for review instead of processing them individually.

## Why Batch?

- **Context** — Related files are reviewed together
- **Efficiency** — Fewer AI calls with better results
- **Coherence** — Changes across files are understood as a unit

## Batching Rules

Files are grouped by:

| Rule | Example |
|------|---------|
| **Package** | All files in `internal/storage/` together |
| **Test pairs** | `handler.go` with `handler_test.go` |
| **Related imports** | Files that import each other |

## Configuration

```bash
# Limit batch size
creareview --max-files 30

# No batching (one file at a time)
creareview --max-files 1
```

## Batch Size

Default is 50 files per batch. Larger batches provide more context but use more tokens.
