---
status: accepted
date: 2026-10-29
source: former ADR-0001 "Cache and import contract" (accepted 2026-10-29, cb4be34); DESIGN.md API contract; review rounds 2026-10-28/29; remaining acquisition rules accepted by the user in ADR review on 2026-10-29 with complete-import serialized-size admission checks
---

# 0007. Reference acquisition, cache and budgets

## Context

The sync cache and import limits changed in every review round: a per-target 16 MiB budget, then per repo, then a cache-only bound. Caching moved from none, to every error, to permanent errors only. The accepted contract separated cache capacity from import eligibility, but it left ordering and total size open.

## Decision: API boundary *(accepted)*

GitHub REST `2022-11-28`, `application/vnd.github+json`. Only these GETs: `/search/code`, `/repos/{o}/{r}`, `/repos/{o}/{r}/commits/{ref}`, `/git/trees/{tree}?recursive=1`, `/git/blobs/{sha}`. Only explicitly public repositories. Each ref is resolved to an immutable commit. No clone, shell, redirects or arbitrary URLs. Git symlinks, submodules, truncated trees and oversized packages are rejected, never counted as exact. Root licenses and notices are kept as separate attribution.

## Decision: cache and import *(accepted)*

Cache capacity controls retained memory, not whether a valid import is eligible. Admission failure or eviction must not classify a valid package as oversized, count a reload as extra content in the same logical package, or silently narrow the requested import scope. File/package validation and explicit operation/request/output budgets stay separate. Those budgets may stop acquisition, with an explicit outcome. Cache misses can increase request cost but are not validation failures. This decision does not promise constant memory for accumulated catalog results.

## Decision: limits and error classes *(accepted)*

- Validation limits: 1 MiB per file, 16 MiB and 1,000 entries per package.
- Operation limits: 100 requests per invocation, bounded responses, 64 MiB serialized store.
- Memory limit: one active repository's tree plus at most 16 MiB of cached blobs.
- **Permanent errors** are cached for the invocation and not requested again: 404, explicitly private repository, truncated tree, unsafe tree path.
- **Transient errors** stay retryable on later targets: 403 or secondary rate limit, 5xx, transport failure.

## Decision: ordering and operation byte budget *(accepted)*

1. **Group targets by repository.** Process every target of an explicitly named repository together first (its full import, then its path hits). Then process the remaining repositories, each grouped, in lexical order. This keeps the explicit-first budget protection and removes the revisits that download a tree again.
2. **Raw-content budget.** Retained imported content is limited to 40 MiB per invocation, including package bytes and required attribution. Count each deduplicated catalog entry once and shared retained attribution once; cache reloads do not add to this logical import budget. This bounds retained content, not total wire bytes downloaded. Base64, metadata and JSON overhead mean that 40 MiB of raw content does not guarantee a serialized store below 64 MiB.
3. **Complete-import admission.** Before admitting an import, check both the raw-content budget and the actual serialized size of the candidate store using the output format that will be written, including package data, metadata, paths, attribution, diagnostics and formatting. Admit a complete package together with all its required attribution, or admit neither. Reserve room for a bounded `sync_budget` diagnostic so reporting the limit cannot itself exceed the 64 MiB output bound; subsequent diagnostic accumulation must preserve that bound too.
4. **Preserve completed imports.** If the next complete import would exceed either budget, stop acquisition before adding it, mark acquisition incomplete with `sync_budget`, and write the previously admitted entries and their required attribution. Do not silently truncate a package, drop attribution or let a budget overflow discard all completed work at final serialization. This guarantee concerns budget handling; filesystem write failures still require explicit errors and atomic-write handling.

*Alternative considered and rejected:* keep the current order and accept extra requests on revisit (allowed by the accepted cache contract), and keep no byte budget, so oversized runs fail at write time.

## Acceptance scenarios

| ID | Scenario | Required result |
| --- | --- | --- |
| C1 | Cache full while the next package is valid | Import stays eligible; validation, request and output budgets still apply. *(accepted)* |
| C2 | Evict A, inspect B, return to A | The reload does not double-count logical imported content. *(accepted)* |
| C3 | Repo A returns 502 once, then succeeds | A later target for A is retried and imported. |
| C4 | Repo A returns 404 | Requested once per invocation; later targets reuse the error. |
| C5 | `sync --repo a/x --query q` where hits include a/x and b/y | The a/x tree is fetched once. |
| C6 | Next complete import would exceed the 40 MiB raw-content budget | Acquisition stops before admitting it, with `sync_budget`; earlier entries and their attribution are written. |
| C7 | Raw content fits 40 MiB but the candidate serialized store, including diagnostic reserve, would exceed 64 MiB | Stop before admitting that import; write the earlier complete imports and a bounded `sync_budget` diagnostic within the output limit. |
| C8 | Adding a package and its required attribution would exceed either budget | Admit neither; retain earlier packages and their attribution. Shared retained attribution and deduplicated entries are not counted twice. |

## Implementation gaps (at 5fed934)

| Area | State |
| --- | --- |
| C1–C4 | Implemented in 2b6fc1f and ed4c74f with tests. |
| C5 | **Gap.** Targets are sorted explicit-full-first, then by repository (`cmd/skill-audit/main.go:236`), so explicit repos' path hits are revisited after other repos and their trees are fetched again. |
| C6–C8 | **Gap.** There is no operation byte budget or per-import serialized-size admission check. An oversized store fails as a whole in `WriteJSON` (`internal/workflow/storage.go:98`). Complete-import admission, diagnostic headroom and attribution accounting are accepted but remain to be implemented and fixture-tested. |

## Isolated rewrite evidence (2026-10-29)

Implementation revision `b7257a55c479501179e9a936f0d7cfffea11e751` on `codex/adr-rewrite`, separate from canonical main. This supplements the dated gap table above; it does not change the accepted decision.

C1–C8 use in-memory fake HTTP transports: cache misses, transient/permanent errors, pinned retry, repository grouping, complete admission with attribution and actual formatted-output headroom. Exhaustion tests inject smaller budgets while production 40 MiB raw / 64 MiB store constants stay pinned. No authenticated or live provider call was made. Earlier complete entries and required attribution survive budget exhaustion.

See [rewrite comparison](../rewrite-comparison.md) and [verification](../../VERIFICATION.md) for exact commands and limits.

## Review follow-up evidence (2026-10-29)

Implementation `f9200ee07f9a0f071c788a11de7467903ea44930` on the isolated rewrite branch. Review #7/#10: repository grouping uses canonical case-insensitive GitHub keys; refs/paths keep their case. Incremental formatted-size accounting encodes only new records/attribution/diagnostics, matches actual bytes in mixed-addition tests, and preserves C7/C8 admission plus final write cap. The 8/16/32-entry benchmark scales approximately linearly.

See [review fixes](../review-fixes.md) and [verification](../../VERIFICATION.md).

## Second-review implementation evidence (2026-10-29)

Verified source `a59cddbc4c6b17940a249579131e71c31db95e5c`. New imports omit declared fingerprint metadata consistently with local collection. Existing stores remain unchanged. The reachable late search-diagnostic budget path has a regression, one finalizer and shared attribution-key formatting. See [second-review record](../review-2-fixes.md).

## Third-review evidence (2026-10-29)

Implementation `77deb5b2eeceda97f5fa22614abf6f3c6e5acc7e` on canonical main. Admitted attribution is reused independently of the blob cache: the empty-cache 40-package/three-license fixture uses 46 requests, compared with exhaustion at 100 after 24 imports before the fix. The resolution cache is intentionally retained: a transient tree failure retries the tree on a later target while keeping metadata and commit resolution at one request each. ADR-0013 separately governs merged search provenance. See [third-review record](../review-3-fixes.md) and [verification](../../VERIFICATION.md).

## Fourth-review evidence (2026-10-29)

Implementation `145b51e33a48fa1bfdbcbecb1eacfa72aaa44d40` on canonical main. Exact/equivalent acquisition targets and identical full diagnostic causes deduplicate separately from retained search provenance. Distinct missing targets/long causes retain evidence. Two 200-duplicate fixtures preserve a later valid import under an 8,000-byte output limit. C3 transient retries and immutable resolution remain covered on later distinct targets. See [fourth-review record](../review-4-fixes.md).
