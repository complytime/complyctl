## Example Output

Below is a concrete example of the expected report format. Use
this as the definitive formatting reference. Adapt the data to the
actual project — do not copy these specific numbers or function
names. The recommendations and function names below are fictional.

```markdown
🔍 Gaze Full Quality Report
Project: github.com/example/project · Branch: main
Gaze Version: v1.0.0 · Go: 1.24.6 · Date: 2026-02-28
---
📊 CRAP Summary
| Metric | Value |
|--------|-------|
| Total functions analyzed | 216 |
| Average complexity | 6.2 |
| Average line coverage | 79.0% |
| Average CRAP score | 7.7 |
| CRAPload | 24 (functions ≥ threshold 15) | <!-- T from summary.crap_threshold -->

Top 5 Worst CRAP Scores
| Function | CRAP | Complexity | Coverage | File |
|----------|------|-----------|----------|------|
| (*Handler).ServeHTTP | 42.0 | 6 | 0.0% | internal/api/handler.go:163 |
| processQueue | 38.5 | 8 | 12.0% | internal/worker/queue.go:89 |
| parseConfig | 31.2 | 5 | 0.0% | cmd/app/config.go:42 |
| (*Store).Migrate | 28.0 | 7 | 15.0% | internal/db/store.go:201 |
| validateInput | 22.4 | 4 | 0.0% | internal/api/validate.go:55 |

Three of the five have 0% test coverage; the other two have minimal coverage with high complexity.

GazeCRAP Quadrant Distribution
| Quadrant | Count | Meaning |
|----------|-------|---------|
| 🟢 Q1 — Safe | 29 | Low complexity, high contract coverage |
| 🟡 Q2 — Complex But Tested | 1 | High complexity, contracts verified |
| 🔴 Q4 — Dangerous | 4 | Complex AND contracts not adequately verified |
| ⚪ Q3 — Needs Tests | 0 | Simple but underspecified |

GazeCRAPload: 4 — All 4 Q4 functions have adequate line coverage but high cyclomatic complexity (15–18), meaning they need decomposition, not more tests.
---
🧪 Quality Summary
> ⚠️ Module-level quality analysis returned 0 tests — run per-package analysis for detailed results.
---
🏷️ Classification Summary
| Classification | Count | % |
|---------------|-------|---|
| Contractual | 73 | 31.3% |
| Ambiguous | 155 | 66.5% |
| Incidental | 8 | 3.4% |

The 66.5% ambiguous rate is typical for projects without extensive documentation; document-enhanced scoring in full mode can reduce this.
---
🏥 Overall Health Assessment

Summary Scorecard
| Dimension | Grade | Details |
|-----------|-------|---------|
| CRAPload | 🟡 C+ | 24/216 functions (11%) above threshold |
| GazeCRAPload | 🟢 A | Only 4 functions above threshold |
| Avg Line Coverage | 🟢 B+ | 79.0% — solid foundation |
| Contract Coverage | 🟡 C | 31.3% contractual, 66.5% ambiguous |
| Complexity | 🟡 B- | Average 6.2, but 24 functions exceed threshold |

Top 5 Prioritized Recommendations
1. 🔴 Add tests for zero-coverage functions — ServeHTTP, parseConfig, and validateInput have 0% coverage with moderate-to-high complexity.
2. 🔴 Increase coverage for processQueue — 12% coverage on complexity-8 function handling critical work queue logic.
3. 🟡 Decompose high-complexity functions — 4 Q4 functions have complexity 15–18 and need to be broken into smaller units.
4. 🟡 Resolve ambiguous classifications — 66.5% ambiguous rate can be reduced with project documentation providing stronger signal evidence.
5. 🟢 Run per-package quality analysis — module-level returned 0 tests; per-package analysis provides granular contract coverage data.
```
<!-- scaffolded by gaze dev -->
