---
status: accepted
date: 2026-10-30
source: external review of v0.1.0 (d9956e0) reported by Dots from ChatGPT, issue 1; "bump evaluation identity" chosen by the user on 2026-10-30
---

# 0017. Finding identity uses the full matched command

## Context

SA001 evidence is capped at 240 bytes plus an ellipsis so reports stay readable. The same truncated text was also the finding's identity when comparing with a baseline. Two different commands that share their first 240 bytes therefore compared as one finding. The external review reproduced this: a long download URL changed after byte 240 was correctly shown as a modified file, but its finding was labelled `existing`. That violates the [ADR-0004](0004-observations-and-baselines.md) finding-label rule: a finding absent from the baseline must not be reported as present in it. Detection and the exit code were unaffected.

## Decision (accepted 2026-10-30)

1. **Identity is a fingerprint of the full match.** Each SA001 finding stores `match_sha256`, the lowercase SHA-256 of the complete matched pipeline bytes. Finding comparison uses scanner, rule, rule version, file and this fingerprint. Display evidence stays capped at 240 bytes.
2. **The evaluation identity is bumped** from `SA001/v3;sh/v3.10.0;scope/v2` (published in v0.1.0) to `SA001/v3;sh/v3.10.0;scope/v2;identity/v2`, as [ADR-0008](0008-versioned-data-contracts.md) requires for a change in what is compared. Detection is unchanged, so the rule version stays SA001 v3.
3. **Older observations are handled conservatively.** Under ADR-0004 and ADR-0008 V7, a baseline evaluated under the previous identity cannot establish `new` or `existing`: its findings compare as `present_in_changed_file` where the file changed, otherwise `not_compared`. Package-content comparison is unaffected. Recording a new baseline restores finding labels.
4. **Representation.** `match_sha256` is an additive optional field on findings in report/v2 and state/v3, so no shape version changes (ADR-0008 rule 2). External scanner findings are unchanged; they never get `new` (ADR-0004).

Rejected alternative: keep the identity and compare older untruncated evidence directly. It would keep some old baselines useful, but changes comparison without a version bump.

## Acceptance scenarios

| ID | Scenario | Required result |
| --- | --- | --- |
| F1 | Baseline with a download pipeline longer than 240 bytes; the URL then changes after byte 240 | The finding is `new`, not `existing`; both observations show the same truncated evidence; their `match_sha256` values differ. |
| F2 | Same long pipeline, unchanged | The finding is `existing`. |
| F3 | Baseline evaluated under the previous identity, without `match_sha256` | No `new` or `existing` labels: `present_in_changed_file` for the changed file; lineage still reports the content change. |

## Implementation gaps

None at acceptance. Implemented together with this ADR; see [scenario-tests.json](scenario-tests.json).
