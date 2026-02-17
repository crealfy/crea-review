# Priority Scoring

crea-review assigns priority scores to focus review on high-risk changes.

## Scoring Factors

| Factor | Weight | Description |
|--------|--------|-------------|
| **Complexity** | High | Cyclomatic complexity of changes |
| **Criticality** | High | Security, auth, database files |
| **Churn** | Medium | Frequently changed files |
| **Size** | Medium | Lines changed |
| **Test coverage** | Low | Missing test coverage |

## Priority Sorting

```bash
# Sort by priority (highest risk first)
creareview --sort priority

# Other sort options
creareview --sort alpha      # Alphabetical
creareview --sort modified   # Most recently modified
creareview --sort commit-new # Newest commits first
creareview --sort commit-old # Oldest commits first
```

## Critical Path Detection

Certain files are automatically marked high priority:

- `auth/`, `security/` directories
- Database migrations
- API handlers
- Configuration files
