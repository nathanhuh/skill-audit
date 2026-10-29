---
status: accepted
date: 2026-10-29
source: DESIGN.md scanner boundary (interface pin); review rounds 2026-10-28/29 (budgets); accepted by the user in ADR review on 2026-10-29 with best-effort reserve and shared-deadline probe retry clarifications
---

# 0006. External scanner integration and time budgets

## Context

The interface pin has been stable. The time budget has not. Over four reviews it went from a per-package probe, to one probe inside the first package's timeout, to a fixed 5 s probe, to "use `--timeout`", to the current fixed 5 s/30 s caps limited to a quarter of the remaining time. Each version broke a different scenario. No accepted rule existed, so each fix redefined the behavior. The underlying constraint comes from [0004](0004-observations-and-baselines.md) B3: a scanner failure must not invalidate complete content measurements or prevent their baseline eligibility. The operation-wide cancellation/timeout refusal in B6 still applies; the reserve below mitigates deadline exhaustion without guaranteeing completion.

## Decision: interface *(accepted)*

- The interface is pinned to `traceforce/skill-xray` commit `c89eacfdd643e50d5f8e3ed18cf3c4992d174ff2`, version `0.1.0`: `version`, then `scan --output REPORT --llm=false -- ABSOLUTE_LOCAL_PACKAGE`. SARIF 2.1.0 only. A zero exit alone is not completion; `properties.coverage` decides.
- Scanning runs on a staged copy of the inventoried bytes, in an audit-owned temporary directory outside every package. Arguments are passed directly to exec, output is bounded, and the scanner gets an environment allowlist. Nothing is installed and no cloud or LLM mode is enabled; `--llm=false` is passed explicitly. As stated in ADR-0001, the supplied executable is trusted code whose network behavior is not enforced.
- Statuses: `disabled`, `unavailable`, `failed`, `partial`, `completed_with_findings`, `completed_without_findings`. Local results survive any adapter failure.

## Decision: time budgets *(accepted)*

1. **One visible budget per phase.** A new `--scan-timeout` flag (default 30 s) bounds the version probe and each package scan. There are no hidden caps and no fraction-of-remaining rule.
2. **Local reserve.** A scanner phase starts only when the remaining operation time exceeds a reserve of 25% of `--timeout`. Its deadline is `min(--scan-timeout, remaining − reserve)`. When no time is left above the reserve, the phase is skipped with `failed: operation budget reserved for local measurement`, and step 3 applies.
3. **Stop after the first failure.** An unsuccessful probe after its allowed retry, a launch failure, a scan timeout or a reserve skip marks the scanner `failed` for the rest of the audit. Later packages get that cause without another launch. Per-package result problems (malformed SARIF, partial coverage, a non-zero exit with a report) do not trigger this.
4. **Retry.** Only the version probe may retry, once, after explicit temporary-failure exit 75. Both attempts share the original probe phase deadline; the retry does not reset its budget. Timeouts and other probe exits are not retried. Package scans are never automatically retried, including on exit 75.
5. **Cause precedence.** A configuration cause (`unavailable`: a relative path, a missing or non-regular file) is never replaced by a later cancellation message.

The reserve is best effort: scanner phase deadlines leave time for local measurement and recording, but remaining work can still exceed that reserve. If the overall operation is canceled or times out before recording commits, ADR-0004 B6 refuses recording and available report evidence is retained. A hanging probe consumes at most one probe phase budget, including any retry. A successful probe followed by a hanging package scan can consume two phase budgets; after that scan timeout, no later package launches the scanner. Earlier successful scans may also have consumed time. The reserve does not promise completion or a one-phase bound on total external-scanner time.

**Alternatives considered:**

- *Two passes:* measure all packages locally first, then run the scanner over the staged bytes. This gives the strongest B3 guarantee, but it needs either holding all package bytes (up to 16 MiB each) or re-reading them with hash verification. It costs more memory and complexity than the MVP warrants.
- *Keep fixed caps and a quarter of the remaining time (current code):* this fails X2–X4. Users cannot raise the caps, and repeated scan timeouts are never cached.

## Acceptance scenarios

| ID | Scenario | Required result |
| --- | --- | --- |
| X1 | Probe hangs | One probe timeout spent; subsequently observed packages `failed` with the probe cause; complete packages remain baseline-eligible if the overall operation finishes. |
| X2 | Scan hangs on every package, 25 packages, default timeouts | With local fixture work fitting in the remaining operation budget, at most one package scan times out; later packages do not launch it, local measurements finish and baseline recording is allowed. The successful probe has its own phase budget. |
| X3 | A package scan takes 45 s with `--scan-timeout 60s` and enough `--timeout` | Scan completes. |
| X4 | First probe takes 6 s (cold start) | Probe succeeds under the default. |
| X5 | Relative `--xray` path, then the operation deadline expires | Every package reports `unavailable` with the path cause. |
| X6 | Probe exits 75 once, then succeeds | One retry within the original probe phase deadline, then success if time remains. Other probe exits are not retried. |
| X7 | Non-executable or wrong-architecture binary | Launched once; scanner `failed` for the whole audit. |
| X8 | Package scan exits 75 | No automatic retry of that package scan; preserve its actual findings, status and coverage evidence. |
| X9 | Only the local reserve remains | Do not launch another scanner phase; report the reserve cause and continue local work within the operation deadline. |
| X10 | Remaining local work exceeds the reserve and the operation times out | Preserve available report evidence; refuse recording not yet committed under ADR-0004 B6. |

## Implementation gaps (at 5fed934)

| Area | State |
| --- | --- |
| Budget | Fixed `xrayProbeTimeout` 5 s and `xrayScanTimeout` 30 s, each `min(--timeout, cap, remaining/4)` (`internal/audit/xray.go:21-22,92`). No `--scan-timeout`. X3 and X4 fail. |
| Stop after failure | Probe failures are cached. Scan timeouts are not, so X2 fails: the remaining time shrinks about 3/4 per package until local inspection is canceled. |
| X5, X7 | Cause preservation and launch-failure caching implemented in 5fed934. Not re-tested during this documentation review. |
| X6, X8–X10 | Probe exit-75 retry exists, but each attempt currently creates a new deadline. Shared-deadline retry, the explicit reserve and revised scenarios remain to be implemented or verified. |

## Isolated rewrite evidence (2026-10-29)

Implementation revision `b7257a55c479501179e9a936f0d7cfffea11e751` on `codex/adr-rewrite`, separate from canonical main. This supplements the dated gap table above; it does not change the accepted decision.

X1–X10 are exercised with injected process/deadline seams and a copied Go test executable. The shared CLI harness observes two baseline scan launches versus one rewrite launch after a hanging scan. Probe retries share the original deadline; no scan retry occurs. X3/X4 assert phase admission without hidden caps but do not spend real 45 s / 6 s wall time. Real Skill X-Ray/OpenGrep remains unverified, and Windows descendant cleanup is still pending.

See [rewrite comparison](../rewrite-comparison.md) and [verification](../../VERIFICATION.md) for exact commands and limits.

## Review follow-up evidence (2026-10-29)

Implementation `f9200ee07f9a0f071c788a11de7467903ea44930` on the isolated rewrite branch. Review #4/#6: capped logs retain their actual process exit and a per-package console cause. Exited processes are distinct from launch failure. The non-embedding buffer prevents io.Copy ReadFrom bypass. Real fake-process tests launch later packages after log overflow. Temp bases are checked against every audited package and package ancestors before probe/staging writes; unsafe TMPDIR is refused without mutation.

See [review fixes](../review-fixes.md) and [verification](../../VERIFICATION.md).
