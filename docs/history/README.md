# History

Dated records of how skill-audit reached its current design. They are evidence, not policy: where they conflict with an ADR in [docs/adr](../adr/README.md), the ADR wins.

| Record | What it covers |
| --- | --- |
| [design-review-notes.md](design-review-notes.md) | Review plans moved from DESIGN.md, 2026-10-28/29 |
| [adr-0001-observation-and-outcome-contracts.md](adr-0001-observation-and-outcome-contracts.md) | The original single ADR, superseded by ADRs 0002, 0004, 0005 and 0007 |
| [rewrite-comparison.md](rewrite-comparison.md) | Comparison of the ADR-driven rewrite with the frozen earlier implementation |
| [review-fixes.md](review-fixes.md), [review-2-fixes.md](review-2-fixes.md), [review-3-fixes.md](review-3-fixes.md), [review-4-fixes.md](review-4-fixes.md) | Fixes for each review round of the rewrite, with their `*-results.json` evidence |
| [host-paths-and-scenarios.md](host-paths-and-scenarios.md) | Host filename fix and the accepted-scenario test inventory |

## Promotion of the rewrite

The document-driven rewrite is the implementation selected for canonical `main`, promoted locally at the user's request after review and fixture verification. The frozen reference `8c6f3b83d725c716246bdbdecc8495bb9cedbccb` remains in Git history. See [host-local filename fix and scenario tests](host-paths-and-scenarios.md) for the latest focused changes, [second-review record](review-2-fixes.md) for promotion evidence, and [rewrite-comparison.md](rewrite-comparison.md) for the dated original comparison.

Former opening of CORE_REVIEW.md: Start with [the filename fix and scenario map](host-paths-and-scenarios.md), the reviewed revision and [docs/adr/README.md](../adr/README.md). The frozen reference is `8c6f3b83d725c716246bdbdecc8495bb9cedbccb` and remains in history; the rewrite is now canonical main. This is document-driven development followed by source-informed compatibility analysis, not a formal clean-room exercise.

## Review follow-up (from CORE_REVIEW.md)

The ten-item review at 7a84ccc is handled in docs/history/review-fixes.md. Read inventory_budget.go and json_size.go for bounded inventory evidence and incremental formatted-byte admission. Runtime cancellation cannot be forged by diagnostic names. State-location mismatches remain visible instead of retroactively changing saved discovery policy. Rejected roots cannot hide packages reached through an admitted root. The non-embedding scanner console buffer prevents io.Copy from bypassing its Write cap; console overflow is a per-package coverage cause, distinct from launch failure. All staging is checked before mutation. Accepted decisions for new policy are ADR-0010 and ADR-0011.

## Second review and promotion (from CORE_REVIEW.md)

The latest focused record is docs/history/host-paths-and-scenarios.md; all accepted IDs map to tests in docs/adr/scenario-tests.json. Canonical main contains the rewrite and reviewed fixes. Search/v3 per-query provenance and the SA001 v3 lone-dash amendment were approved explicitly. The second-review record preserves the fast-forward promotion evidence; historical sections retain their named revisions.
