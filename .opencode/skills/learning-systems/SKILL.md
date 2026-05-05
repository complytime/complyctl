---
description: How the forge learns from outcomes
tags: [learning, forge, insights]
---

# Learning Systems

The forge improves over time by recording outcomes and querying insights.

## Recording Outcomes

After every forge completion, record the outcome:

```
forge_record_outcome(
  bead_id="<id>",
  duration_ms=120000,
  success=true,
  strategy="file-based",
  files_touched=["internal/foo/bar.go"],
  error_count=0,
  retry_count=0
)
```

## Querying Insights

### Strategy Insights

Which decomposition strategies work best:

```
forge_get_strategy_insights(task="<task description>")
```

Returns success rates for file-based, feature-based, and risk-based strategies.

### File Insights

Historical gotchas for specific files:

```
forge_get_file_insights(files=["internal/foo/bar.go"])
```

Returns past failure patterns, edge cases, and performance traps.

### Pattern Insights

Common failure patterns across all forges:

```
forge_get_pattern_insights()
```

Returns top 5 most frequent failure patterns with recommendations.

## When to Store vs Query

- **Store** after completing work: learnings, decisions, gotchas
- **Query** before starting work: check if someone solved it before
- Use `hivemind_store` for general learnings
- Use `forge_record_outcome` for structured forge metrics
