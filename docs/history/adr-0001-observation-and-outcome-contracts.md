---
status: superseded by docs/adr 0002, 0004, 0005, 0007 (2026-10-29)
date: 2026-10-29
---

# Observed baselines and independent outcomes

Repeated review fixes changed the meaning of coverage, baselines, scope and exit codes. We choose baselines of fully measured observed skills and independent analysis, coverage and recording outcomes, with findings taking exit priority. This preserves useful evidence without claiming exhaustive discovery, and prevents one failure dimension from silently redefining another.

Accepted by the user on 2026-10-29. **This freezes intended behavior; it does not claim the current code implements every decision.** This ADR takes precedence over conflicting historical review plans in DESIGN.md. Terminology is defined in [CONTEXT.md](../../CONTEXT.md).

> **Superseded layout (2026-10-29).** The accepted rules below were split, unchanged and with the same scenario IDs, into [0002](../adr/0002-local-identity-and-installation-ownership.md) (S1–S2), [0004](../adr/0004-observations-and-baselines.md) (B1–B9), [0005](../adr/0005-outcomes-and-exit-codes.md) (E1–E8) and [0007](../adr/0007-reference-acquisition.md) (C1–C2). This file is kept for history.

## Baseline contract

- Baseline eligibility is per package: a complete content fingerprint may become an anchor. An incomplete or uninspected package cannot.
- A completed observation may record anchors for fully measured packages even when discovery elsewhere is partial. Preserve selected roots, exclusion-policy identity/version and coverage evidence in state.
- Known exclusions are listed. Known system aliases may be declared exclusions; an uninspected skill-manager link remains a coverage gap. Neither erases valid measurements of another package. Symlinks remain unfollowed for the MVP.
- Scanner failure does not invalidate complete content measurements. It prevents comparable detection claims for that scanner; a finding is called new only with comparable completed evaluations.
- Reject recording from canceled/timed-out runs or invalid state. A budget-limited discovery may preserve complete observed anchors, marked partial. Refuse a partial observation with no fully measured skills. A completed, genuinely empty selected scope may be recorded as empty.
- Existing anchors remain immutable. Later complete content without a usable baseline remains unanchored until an explicit new baseline is recorded; no automatic replacement, invented prior content or rename linkage.
- Partial discovery cannot establish deletion, resolution, unchanged content or safety for an unobserved skill.
- Evidence must stay bounded. Repeated routine exclusions can be summarized by rule/count and bounded examples; actionable gaps must identify affected skills or locations. Any evidence truncation remains explicit.

## Outcome and exit contract

Readable and versioned machine output must represent these independently:

1. Findings and their scanner attribution.
2. Discovery and detection coverage, including exclusions and gaps.
3. Actual recording outcome: not requested, recorded, not recorded, or committed with uncertain durability.

Exit priority for a run that can produce analysis is **findings (1) > failed recording before commit (3) > incomplete/operational result (2) > success (0)**.

| Findings | Recording | Coverage/operational issue | Exit |
| --- | --- | --- | ---: |
| Present | Any | Any | 1 |
| Absent | Failed before commit | Any | 3 |
| Absent | Recorded, not requested, or committed with uncertain durability | Present, including durability uncertainty | 2 |
| Absent | Recorded or not requested | Absent | 0 |

Recording failures always appear on stderr and in structured output, even when exit 1 wins. Automation requiring persistence checks the recording outcome, not just the integer exit. Invalid command configuration that prevents analysis remains an operational failure; it cannot assert findings or an attempted save.

Successful replacement is the recording commit point. A failed final durability check means committed with uncertain durability, not not-recorded; never retry automatically or undo evidence. Input diagnostic names cannot determine a recording outcome. An output failure may prevent delivery, but cannot retroactively turn committed state into an unsaved record.

## Installation ownership contract

Scope follows the installation that owns the package. A home-global plugin installation retains global ownership for its bundled skills, including `~/.claude/plugins/marketplaces/m/.claude/skills/x`. An inner `.claude` marker alone cannot turn it into a project installation. Community global layouts retain the same principle. Designated project/worktree installations remain project-owned, including ones stored inside an agent home. Presence, ownership and activation remain separate observations; existing identities are never silently rewritten.

## Cache and import contract

Cache capacity controls retained memory, not valid import eligibility. Admission failure or eviction must not classify a valid package as oversized, count a reload as additional content in the same logical package, or silently narrow requested import scope. File/package validation and explicit operation/request/output budgets remain separate. Those budgets may stop acquisition, with an explicit outcome; cache misses can increase request cost but are not semantic validation failures. This decision does not promise constant memory for accumulated catalog results.

## Acceptance matrix

These are required behavior scenarios for implementation, **not claims that tests already pass**.

| ID | Scenario | Required result |
| --- | --- | --- |
| B1 | Complete bar plus listed .git/dependency exclusions | Record bar's anchor; preserve the policy and exclusion evidence. |
| B2 | Complete bar plus an uninspected linked foo | Baseline may record bar; discovery is partial; foo has no usable anchor. |
| B3 | Complete bytes plus failed optional scanner | Content anchor allowed; detection gap visible; that scanner cannot establish new findings. |
| B4 | Budget stops discovery after complete bar | Preserve bar's anchor and partial discovery evidence; no claims about unseen skills. |
| B5 | Partial run with no complete packages | Refuse baseline; preserve report and explain missing usable measurements. |
| B6 | Operator cancellation or operation timeout | Refuse recording that has not committed; retain available report evidence. If commitment already occurred, preserve and report the actual committed outcome. |
| B7 | Completed empty selected scope | Empty baseline allowed with accurate scope/coverage evidence. |
| B8 | Missing tracked package during partial discovery | Missing/undiscovered, without invented deletion or resolved finding. |
| B9 | First baseline measurement incomplete, later measurement complete | No fabricated change/new claim and no automatic anchor replacement. |
| E1 | Findings, complete coverage, recorded | Exit 1; recording outcome says recorded. |
| E2 | Findings and failed recording | Exit 1; stderr and structured output still say not recorded. |
| E3 | Findings and partial detection/discovery | Exit 1; coverage stays partial. |
| E4 | No findings and failed recording | Exit 3; not recorded. |
| E5 | No findings, partial coverage, recorded | Exit 2; recording still says recorded. |
| E6 | No findings and committed durability warning | Exit 2; committed/uncertain, never not-recorded. |
| E7 | Findings and committed durability warning | Exit 1; warning and committed/uncertain outcome remain visible. |
| E8 | Input diagnostic named state_not_recorded | Cannot fabricate the recording outcome or select a persistence exit. |
| S1 | Global plugin containing a nested agent marker | Global installation ownership retained. |
| S2 | Skill in an agent-managed project worktree | Project ownership retained. |
| C1 | Cache full while next package is valid | Import remains eligible; validation/request/output budgets still apply. |
| C2 | Evict A, inspect B, return to A | Reload does not double-count logical imported content. |

## Implementation alignment at acceptance

Code examined: `ed4c74f03cda5f916ce6ae5799160bef1c95ad63` on canonical main. No runtime changes are part of accepting this ADR.

| Area | Current behavior / remaining work |
| --- | --- |
| Baselines | SaveObservation rejects any report-wide coverage gap, including optional scanner failures. It must separate usable content anchors from discovery/detection gaps and apply B1–B9. |
| Exit priority | Report.ExitCode prioritizes coverage over findings, and the CLI prioritizes failed recording over findings. Both differ from E1–E8. |
| Recording output | Typed committed-write warnings exist, but outcome is represented through diagnostics rather than an independent structured recording result. Formalize it without letting input diagnostics control it. |
| Scope | Nearest agent-marker classification can misclassify nested global plugins. Installation ownership needs an explicit rule. |
| Cache | Cache admission is already separate from package eligibility. Preserve that invariant and operation-budget reporting. |
| Data contracts | Scope, coverage policy and recording outcome must be versioned when their wire format changes. Historical observations remain intact; no silent state migration or origin claims. |

Future changes should be justified against this matrix. A review finding may expose an implementation gap; it does not, by itself, change the accepted policy. Policy revisions need a new explicit design decision superseding the relevant portion of this ADR.

## Runtime alignment — 2026-10-29

The historical acceptance-time gap table above remains a record of ed4c74f. The subsequent implementation adds E1–E8 outcome tests, B1–B9 anchor tests and S1–S2 installation-ownership tests, plus bounded exclusion and case-alias regressions. Current data contracts are report/v2, inventory/v3 and state/v2. Legacy state/v1 remains comparison-readable but append-read-only; new baselines require an explicit new path. No silent migration or identity rewrite occurs. C1/C2 cache/import invariants remain protected by existing tests; this work does not change their policy or add a total acquisition budget. See [VERIFICATION.md](../../VERIFICATION.md) for executed checks and limitations.


## Follow-up representation correction — 2026-10-29

New state/v3 derives anchors directly from complete measurements in the immutable first observation. It omits recording from stored observations because a write cannot contain its own final disk-sync outcome. Actual recording status remains in the final CLI report and requested-recording stderr; history alone cannot confirm that final durability check. State/v1 and state/v2 remain comparison-readable and append-read-only, without rewriting historical assertions. Report/v2 and inventory/v3 stay unchanged. This corrects representation without changing baseline eligibility or the E1–E8 outcome/exit matrix.
