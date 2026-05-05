### Document-Enhanced Classification

If `gaze docscan` returns documentation files, enhance the mechanical
classification by applying document-signal scoring. Start from the
mechanical confidence score for each side effect, add document and AI
inference signal weights, detect contradictions, clamp to 0–100, and
re-apply thresholds.

**Document Signal Sources**

Extract signals from the documentation content and assign weights:

| Source | Weight Range | Evidence |
|--------|-------------|---------|
| `readme` | ±5 to ±15 | Module README explicitly names the function or its behavior (positive) or describes it as internal (negative) |
| `architecture_doc` | ±5 to ±20 | Architecture/design doc declares this function's contract (positive) or marks it as implementation detail (negative) |
| `specify_file` | ±5 to ±25 | `specs/` files document this as required behavior (positive) or mark it as optional (negative) |
| `api_doc` | ±5 to ±20 | API reference doc lists this function's return values or mutations (positive) or marks as non-public (negative) |
| `other_md` | ±2 to ±10 | Other markdown files reference this function (positive) or describe it as debug/internal (negative) |

**AI Inference Signals**

In addition to extracting explicit mentions, infer signals from patterns:

| Source | Weight Range | Evidence |
|--------|-------------|---------|
| `ai_pattern` | +5 to +15 | Recognizable design pattern (Repository, Factory, etc.) whose contract implies this side effect |
| `ai_layer` | +5 to +15 | Architectural layer analysis (e.g., service layer functions that mutate state are usually contractual) |
| `ai_corroboration` | +3 to +10 | Multiple independent document signals agree |

**Contradiction Penalty**

If document signals and mechanical signals point in opposite directions
(e.g., mechanical says contractual, docs say incidental), apply a
contradiction penalty of up to -20 to the confidence score.

**Classification Thresholds**

After recalculation, re-derive labels from updated confidence scores:

| Confidence | Label |
|-----------|-------|
| ≥ 80 | contractual |
| 50–79 | ambiguous |
| < 50 | incidental |
<!-- scaffolded by gaze dev -->
