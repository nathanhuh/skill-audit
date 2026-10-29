---
status: accepted
date: 2026-10-29
source: review of codex/adr-rewrite 7a84ccc, issue 8
---

# 0010. Bounded inventory output

## Decision (accepted 2026-10-29)

Retain a useful partial inventory under the existing 64 MiB formatted JSON write bound. Merely capping gap count is insufficient: package file manifests and frontmatter metadata can also exceed the bound.

1. Reserve space for scope/coverage metadata and a final `inventory_output_budget` diagnostic. Admit each measured package, including its complete file manifest and metadata, only when the actual current output format will fit. If the next whole measurement cannot fit, stop collection, mark discovery partial, identify the location where collection stopped and preserve earlier measurements. Do not truncate a manifest while leaving its package complete.
2. Bound retained discovery diagnostic details to 1 MiB of formatted JSON and 1,000 records, whichever is reached first. Retain actionable examples until that limit; then summarize omitted records by diagnostic code with counts and explicitly state that affected-location detail was truncated. Preserve partial coverage. Summaries and the final budget diagnostic must fit reserved headroom.
3. Scope metadata, exclusion summaries and package metadata count toward the same 64 MiB total. Never drop selected roots silently. If even the mandatory scope envelope cannot fit, report an explicit configuration/output failure; do not claim a saved inventory.
4. The same serialization representation is used for admission and final writing. Tests inject smaller budgets, compare accounting against actual formatted bytes, and prove that a saved partial inventory remains readable. Inventory/v3 is unpublished and can gain explicit truncation evidence in place under ADR-0008.

Rejected alternative: preserve the existing final-write refusal when inventory exceeds 64 MiB. This keeps all in-memory evidence but can leave no saved inventory.

## Acceptance scenarios

| ID | Scenario | Required result |
| --- | --- | --- |
| I1 | Gap evidence exceeds its diagnostic budget | Save readable partial inventory with explicit omitted counts; no clean coverage claim. |
| I2 | Next package manifest or metadata exceeds remaining output capacity | Keep earlier whole measurements; stop visibly before admitting it. |
| I3 | Required scope envelope alone cannot fit | Explicit failure, no false saved-inventory claim. |
| I4 | Final formatted output | At most 64 MiB, including summaries and budget diagnostics. |

Accepted by the user on 2026-10-29 in this review thread.

## Review follow-up evidence (2026-10-29)

Implementation `f9200ee07f9a0f071c788a11de7467903ea44930` on the isolated rewrite branch. I1–I4 pass with smaller injected limits and production 64 MiB CLI evidence: 937 of 1,000 whole measurements retained in a 66,010,508-byte saved partial inventory. A 3,000-link fixture records bounded detail and omitted counts; mandatory-envelope failure cannot save output.

See [review fixes](../review-fixes.md) and [verification](../../VERIFICATION.md).

## Second-review implementation evidence (2026-10-29)

Verified source `a59cddbc4c6b17940a249579131e71c31db95e5c`. The injected final-accounting overflow backstop marks coverage incomplete, records an explicit diagnostic and is checked before stdout delivery as well as file output. See [second-review record](../review-2-fixes.md).
