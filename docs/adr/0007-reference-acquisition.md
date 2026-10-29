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
