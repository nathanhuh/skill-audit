---
status: accepted
date: 2026-10-29
source: former ADR-0001 "Outcome and exit contract" (accepted by the user 2026-10-29, cb4be34); text output E9 accepted by the user in ADR review on 2026-10-29
---

# 0005. Outcomes and exit codes

## Context

Exit priority was redefined in three reviews: coverage over findings, then a failed save over findings, then a diagnostic name controlling exit 3. Automation could not tell "not saved" from "coverage was partial".

## Decision *(accepted)*

Readable and versioned machine output represents these three things independently:

1. Findings and their scanner attribution.
2. Discovery and detection coverage, including exclusions and gaps.
3. The actual recording outcome: `not_requested`, `recorded`, `not_recorded`, or `committed_durability_unconfirmed`.

The exit priority for a run that can produce analysis is **findings (1) > recording failed before commit (3) > incomplete/operational result (2) > success (0)**.

| Findings | Recording | Coverage/operational issue | Exit |
| --- | --- | --- | ---: |
| Present | Any | Any | 1 |
| Absent | Failed before commit | Any | 3 |
| Absent | Recorded, not requested, or committed with uncertain durability | Present, including durability uncertainty | 2 |
| Absent | Recorded or not requested | Absent | 0 |

Recording failures always appear on stderr and in structured output, even when exit 1 wins. Automation that requires persistence must check the recording outcome, not only the exit code. Invalid command configuration that prevents analysis is an operational failure. It cannot claim findings or an attempted save.

A successful replacement is the recording commit point. If the final durability check fails, the outcome is committed with uncertain durability, not not-recorded. Never retry automatically or undo evidence. Diagnostic names in input data cannot determine a recording outcome. An output failure may prevent delivery, but it cannot retroactively turn committed state into an unsaved record.

## Text output *(accepted)*

With `--state PATH --record baseline|snapshot`, the observation is saved to the selected state file if recording succeeds. The recording summary reports the actual outcome; printing it does not itself save state. The final recording outcome belongs to the report, not the stored observation, because it is known only after the write.

When recording is requested, text-mode stdout includes one `recording=<status> path=… message=…` line, and stderr carries the same outcome once. When recording is not requested, neither stream prints a recording line. JSON always contains the `recording` object. A user who captures only stdout can still see whether state was saved.

## Acceptance scenarios

| ID | Scenario | Required result |
| --- | --- | --- |
| E1 | Findings, complete coverage, recorded | Exit 1; recording outcome says recorded. |
| E2 | Findings and failed recording | Exit 1; stderr and structured output still say not recorded. |
| E3 | Findings and partial detection/discovery | Exit 1; coverage stays partial. |
| E4 | No findings and failed recording | Exit 3; not recorded. |
| E5 | No findings, partial coverage, recorded | Exit 2; recording still says recorded. |
| E6 | No findings and committed durability warning | Exit 2; committed/uncertain, never not-recorded. |
| E7 | Findings and committed durability warning | Exit 1; the warning and the committed/uncertain outcome stay visible. |
| E8 | Input diagnostic named state_not_recorded | Cannot fabricate the recording outcome or select a persistence exit. |
| E9 | Text mode, `--record snapshot`, stdout only captured | stdout contains the recording line. *(accepted)* |

## Implementation gaps (at 5fed934)

| Area | State |
| --- | --- |
| E1–E8 | Implemented in 71e3cee with tests (`cmd/skill-audit/outcome_test.go`). |
| E9 | **Gap.** `audit.Print` no longer prints a recording line, so text stdout never shows the recording outcome. E9 is now accepted; implementation remains pending. |

## Isolated rewrite evidence (2026-10-29)

Implementation revision `b7257a55c479501179e9a936f0d7cfffea11e751` on `codex/adr-rewrite`, separate from canonical main. This supplements the dated gap table above; it does not change the accepted decision.

E1–E8 have a priority table plus actual CLI finding/recording/delivery checks. E9 now prints the actual requested recording outcome once on text stdout and once on stderr. JSON recording remains independent from stored observations; output-file delivery failure does not erase a committed recording or suppress an available stdout summary.

See [rewrite comparison](../rewrite-comparison.md) and [verification](../../VERIFICATION.md) for exact commands and limits.

## Review follow-up evidence (2026-10-29)

Implementation `f9200ee07f9a0f071c788a11de7467903ea44930` on the isolated rewrite branch. Review #1/E8: imported diagnostic names no longer control recording. The current operation context and private runtime interruption fact enforce B6. Complete anchors record alongside fake inventory/catalog interruption diagnostics; genuine cancellation still refuses recording.

See [review fixes](../review-fixes.md) and [verification](../../VERIFICATION.md).

## Third-review evidence (2026-10-29)

Implementation `77deb5b2eeceda97f5fa22614abf6f3c6e5acc7e` on canonical main. Report JSON now explicitly includes overall complete and created_at. Scanner/drift/missing gaps can keep overall complete false while discovery.complete remains true. Complete scanner SARIF plus a normal non-zero exit retains its completion and exit evidence; security findings retain exit-1 priority. See [third-review record](../review-3-fixes.md) and [verification](../../VERIFICATION.md).
