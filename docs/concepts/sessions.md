# Sessions

crea-review tracks review sessions for continuity and re-review.

## What Is a Session?

A session captures:

- Files reviewed
- Findings discovered
- Review metadata

Sessions are stored in `~/.valksor/crealfy/review/<project>/sessions/`.

## Using Sessions

```bash
# List all sessions
creareview --list-sessions

# Continue from session 1
creareview --continue 1

# Re-review files from session 2
creareview --session 2
```

## Session Storage

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

## When to Use

- **Continue** — Pick up where you left off
- **Session** — Re-review specific files with fresh eyes
- **List** — See review history
