---
status: accepted
date: 2026-10-29
source: former ADR-0001 "Baseline contract" and B1–B9 (accepted by the user 2026-10-29, cb4be34); DESIGN.md lineage section; state-location amendment accepted by the user in ADR review on 2026-10-29
---

# 0004. Observations, baselines and anchors

## Context

Baseline eligibility first required a fully complete report. That made baselines impossible on real machines. Then it ignored coverage. The accepted contract makes eligibility a per-package decision.

## Decision *(accepted)*

- Baseline eligibility is per package. A complete content fingerprint may become an anchor. An incomplete or uninspected package cannot.
- A completed observation may record anchors for fully measured packages even when discovery elsewhere is partial. State keeps the selected roots, the exclusion-policy identity and version, and the coverage evidence.
- Known exclusions are listed. Known system aliases may be declared exclusions. An uninspected skill-manager link remains a coverage gap. Neither erases valid measurements of another package. Symlinks stay unfollowed for the MVP ([0003](0003-discovery-and-exclusion-policy.md)).
- A scanner failure does not invalidate complete content measurements. It prevents comparable detection claims for that scanner. A finding is called new only when both evaluations are comparable and completed.
- Recording is refused for canceled or timed-out runs and for invalid state. A budget-limited discovery may keep complete observed anchors, marked partial. A partial observation with no fully measured skills is refused. A completed, genuinely empty selected scope may be recorded as empty.
- Existing anchors are immutable. Later complete content without a usable baseline stays unanchored until a new baseline is recorded explicitly. There is no automatic replacement, invented prior content or rename linkage.
- Partial discovery cannot establish deletion, resolution, unchanged content or safety for a skill that was not observed.
- Evidence stays bounded. Repeated routine exclusions can be summarized by rule and count with bounded examples. Actionable gaps must identify the affected skills or locations. Any truncation of evidence is stated explicitly.

**Lineage mechanics.** State is a versioned JSON file outside every skill package, in a designated state location explicitly excluded from discovery: an immutable first observation (the baseline) plus append-only snapshots. Anchors come from the complete measurements in the first observation. Writes use a lock file, exclusive baseline creation and a same-directory temp file, file fsync and rename. Corrupt or unsupported state is a hard diagnostic and is never reset. Reads are capped at 32 MiB.

**State-location amendment (accepted 2026-10-29).** This decision supersedes the earlier requirement to store state "outside every scanned root": a device scan rooted at `/` makes that impossible. State must remain outside every skill package, but may be inside a selected discovery root. Its designated location is explicitly excluded from discovery, with the exclusion recorded under [ADR-0003](0003-discovery-and-exclusion-policy.md). This does not authorize broad, silent exclusions of unrelated content. Existing observations and anchors remain unchanged. Implementation of the designated exclusion is not established by acceptance of this rule.

**Finding labels.** `new` only when the same scanner, rule and evaluation version completed on both observations and the finding was absent from the baseline. Otherwise `present_in_changed_file` where applicable, or `not_compared`. External scanner findings never get `new` in the MVP, because a version string does not pin the scanner's rulesets.

## Acceptance scenarios *(accepted)*

| ID | Scenario | Required result |
| --- | --- | --- |
| B1 | Complete bar plus listed .git/dependency exclusions | Record bar's anchor; keep the policy and exclusion evidence. |
| B2 | Complete bar plus an uninspected linked foo | Baseline may record bar; discovery is partial; foo has no usable anchor. |
| B3 | Complete bytes plus failed optional scanner | Content anchor allowed; detection gap visible; that scanner cannot establish new findings. |
| B4 | Budget stops discovery after complete bar | Keep bar's anchor and the partial discovery evidence; no claims about unseen skills. |
| B5 | Partial run with no complete packages | Refuse baseline; keep the report and explain which usable measurements are missing. |
| B6 | Operator cancellation or operation timeout | Refuse recording that has not committed; keep available report evidence. If the commit already happened, keep and report the actual committed outcome. |
| B7 | Completed empty selected scope | Empty baseline allowed, with accurate scope and coverage evidence. |
| B8 | Missing tracked package during partial discovery | Missing/undiscovered, with no invented deletion or resolved finding. |
| B9 | First baseline measurement incomplete, later measurement complete | No fabricated change or new claim; no automatic anchor replacement. |
| B10 | Device discovery includes the designated state location | State may reside under a discovery root; its location is explicitly excluded and the exclusion is reported. |
| B11 | Requested state location lies inside a skill package | Reject the location; do not change package contents or existing state. |

## Implementation gaps (at 5fed934)

| Area | State |
| --- | --- |
| B1–B9 | Implemented in 4b5a86a with tests. In 5fed934, state/v3 derives anchors from the first observation and stops storing recording outcomes in observations. |
| B10–B11 | State-location amendment accepted during this review; designated exclusion and fixture coverage require implementation review. No runtime verification performed in this pass. |
| B3 vs scanner budgets | **At risk.** A hung scanner can use up the operation deadline, so B6 then refuses recording. The accepted [0006](0006-external-scanner-budgets.md) X2/X9 reserve and stop-after-timeout rules mitigate this; implementation is pending. The reserve is best effort and B6 still applies if the operation deadline expires. |

## Isolated rewrite evidence (2026-10-29)

Implementation revision `b7257a55c479501179e9a936f0d7cfffea11e751` on `codex/adr-rewrite`, separate from canonical main. This supplements the dated gap table above; it does not change the accepted decision.

B1–B11 have scenario-oriented fixture coverage for complete per-package anchors under unrelated gaps, empty and unusable observations, scanner failure, immutable observations, lock/recording refusal, missing/unanchored identities, designated state exclusion and placement. Final-sync uncertainty is injected at the persistence boundary; it is not a crash-durability experiment. Ordinary audit remains read-only.

See [rewrite comparison](../history/rewrite-comparison.md) and [verification](../../VERIFICATION.md) for exact commands and limits.

## Review follow-up evidence (2026-10-29)

Implementation `f9200ee07f9a0f071c788a11de7467903ea44930` on the isolated rewrite branch. Review #5: an in-scope state path different from the inventory designation is reported as state_exclusion_unconfirmed and cannot be recorded without refreshing inventory --state. No retroactive exclusion is claimed. Truncation evidence survives audit and stored observations under ADR-0010.

See [review fixes](../history/review-fixes.md) and [verification](../../VERIFICATION.md).

## Second-review implementation evidence (2026-10-29)

Verified source `a59cddbc4c6b17940a249579131e71c31db95e5c`. Per-skill incomplete diagnostics and explicit partial discovery survive report/state readback. Final output-budget refusal cannot claim complete coverage. Manual stale-lock policy is unchanged, with actionable error guidance. See [second-review record](../history/review-2-fixes.md).

## Third-review evidence (2026-10-29)

Implementation `77deb5b2eeceda97f5fa22614abf6f3c6e5acc7e` on canonical main. Valid hex hashes normalize in the in-memory comparison view. Published/current stored observation bytes remain unchanged, and invalid file manifests/fingerprints remain rejected. Overall report completeness survives serialization alongside independent discovery coverage. See [third-review record](../history/review-3-fixes.md) and [verification](../../VERIFICATION.md).

## Fourth-review evidence (2026-10-29)

Implementation `145b51e33a48fa1bfdbcbecb1eacfa72aaa44d40` on canonical main. Inventory, catalog and state readers canonicalize valid SHA-256 text in memory; hash bytes and original inputs remain unchanged. New observations store only observed_at. Older matching duplicate times are read and historical fields are retained; contradictory times are rejected as invalid state. See [fourth-review record](../history/review-4-fixes.md).

## Focused filename/scenario evidence (2026-10-29)

Implementation `ce92175af520df9c8fb80e82195aadc81e5df5ce`. Host-local filename validation is shared with measurement and staging under ADR-0009 P5. Complete colon/backslash measurements now remain readable anchors; old valid files recover without migration or reset. The empty-scope B7 test also exercises readable state through the full operation. See [focused record](../history/host-paths-and-scenarios.md) and the [scenario map](scenario-tests.md).
