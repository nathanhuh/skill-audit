---
status: accepted
date: 2026-10-30
source: first run of v0.1.0 on the user's own machine; accepted by the user on 2026-10-30 as part of the v0.1.1 plan ("A and B together")
---

# 0015. Text output is a summary; JSON is the evidence

## Context

Before this decision only one text-output rule was accepted: the requested recording line ([ADR-0005](0005-outcomes-and-exit-codes.md) E9). [ADR-0010](0010-inventory-output-budget.md) bounds diagnostics in JSON, not their terminal rendering. In practice the text printer wrote one line per diagnostic and two scanner lines per skill. A real run over a home directory printed about a thousand `symlink_skipped` lines, and an audit of 63 skills printed 126 scanner lines that said nothing beyond "no findings". The evidence was correct but hard to read, and the JSON report already carries every record.

## Decision (accepted 2026-10-30)

1. **Diagnostics are grouped by code in text.** Text prints one line per diagnostic code, in order of first appearance. A code that occurs once keeps its existing `"code" path=… message=…` line. A repeated code prints `"code" count=N examples=[…] omitted_examples=M message=…`, with up to three example paths and the first message, matching the existing exclusion summary. JSON keeps every record, within the ADR-0010 limits.
2. **Scanner lines print only when they add information.** A `disabled` evaluation prints no text line, and neither does a built-in `skill-audit` evaluation with status `completed_without_findings`. Every other evaluation prints: findings, partial or failed evaluations, and any Skill X-Ray evaluation that actually ran. JSON keeps every evaluation.
3. **Audit text ends with one summary line:** `summary skills=… global=… project=… machine=… exact=… possible_modified=… ambiguous=… unknown=… findings=… complete=…`. It comes after diagnostics and before the requested recording line.
4. **Nothing else changes.** The E9 recording line, stderr recording outcome, exit codes, JSON shapes and every versioned identifier are unchanged. Text output is not a versioned data contract under [ADR-0008](0008-versioned-data-contracts.md), so no identifier is bumped. Automation should read `--json` or `--out`.

Rejected alternative: a `--verbose` flag that restores per-record text. JSON already serves that need without another flag combination to test.

## Acceptance scenarios

| ID | Scenario | Required result |
| --- | --- | --- |
| L1 | Inventory with three skipped symlinks under one root | Text has exactly one `"symlink_skipped"` line with `count=3`; JSON has three `symlink_skipped` diagnostics. |
| L2 | Audit of one unknown clean skill and one skill with an SA001 finding | No scanner line for the clean skill; the finding's skill prints its `skill-audit` scanner line and the finding. |
| L3 | Same audit | The last text line is the summary with the correct scope, correlation and finding counts. |
| L4 | Same audit with `--record baseline`, then with `--json` | Text stdout and stderr each carry the recording line once (E9); the exit code matches between text and JSON runs; JSON still lists every evaluation. |

## Implementation gaps

None at acceptance. Implemented together with this ADR; see [scenario-tests.json](scenario-tests.json).

## Later amendment

[ADR-0018](0018-compact-text-output.md) (2026-10-30) lists only skills that need attention, adds `listed=N` to the summary line (L3), collapses routine exclusions into one line of counts, and prints one example path for a repeated diagnostic code instead of up to three (L1).
