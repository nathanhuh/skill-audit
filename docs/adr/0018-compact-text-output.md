---
status: accepted
date: 2026-10-30
source: user's first `skill-audit scan` of v0.1.1 on their own machine ("Wall of texts not good"); compact layout chosen by the user on 2026-10-30 from a preview
---

# 0018. Text lists only skills that need attention

## Context

[ADR-0015](0015-text-output-summary.md) grouped diagnostics and hid uninformative scanner lines, but kept one line per skill and full example paths for every routine exclusion. A scan of 63 skills, 61 of them unknown and clean, still printed about 75 lines. The useful content was two exact matches, one coverage gap and the summary.

## Decision (accepted 2026-10-30)

1. **Only skills that need attention are listed in text.** A skill gets text lines when it matched an upstream reference, has findings, was measured incompletely or has diagnostics, has a baseline lineage other than `not_compared` or `unchanged` (for example `changed` or `not_in_baseline`), or has an evaluation that ADR-0015 rule 2 would print. Other skills are only counted. The summary line adds `listed=N` after `skills=N`; this amends ADR-0015 L3.
2. **Routine exclusions collapse into one line of counts**, largest first, e.g. `excluded .DS_Store=69 .git=49 node_modules=8`. A repeated diagnostic code prints its count, one example path and the first message; this amends ADR-0015 L1, which allowed three examples.
3. **Candidate lines are shorter**: reference, repository, a 12-character commit, basis and score. Upstream and baseline file-change lines are unchanged.
4. **Nothing else changes.** Paths stay absolute. `inventory` text still lists every package, since that list is its purpose. JSON, `report.json`, exit codes, the E9 recording line and every versioned identifier are unchanged ([ADR-0008](0008-versioned-data-contracts.md)); text is not a versioned contract.

Rejected alternative: a `--all` flag to list every skill. The JSON report already lists every skill, consistent with ADR-0015's rejection of a verbose flag.

## Acceptance scenarios

| ID | Scenario | Required result |
| --- | --- | --- |
| K1 | Audit of an exact catalog copy, a clean unknown skill and a skill with an SA001 finding | Text lists the exact copy and the finding's skill, not the clean skill; summary has `listed=2`; JSON lists all three. |
| K2 | Inventory with `.git` and `node_modules` folders and three skipped symlinks | One `excluded` line with `.git=1` and `node_modules=1`; one `symlink_skipped` line with `count=3` and a single `example=`; no per-rule exclusion lines. |
| K3 | Audit against a baseline after one clean unknown skill changed and a new one appeared | Both are listed, with lineage `changed` and `not_in_baseline`. |
| K4 | Modified copy of a catalog skill | Candidate line shows a 12-character commit and no `upstream_path`; the `upstream_modified=` line is still printed. |

## Implementation gaps

None at acceptance. Implemented together with this ADR; see [scenario-tests.json](scenario-tests.json).
