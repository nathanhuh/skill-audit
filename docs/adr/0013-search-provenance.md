---
status: accepted
date: 2026-10-29
source: third review of main 573eed6, item 6; explicitly approved provenance batches in v3 by the user
---

# 0013. Search provenance across merged inputs

## Decision

Search/v3 stores a flat ordered list of input batches. Each retains its source, query list, hits, completeness and diagnostics. Both `search --search saved.json --query q` and `sync --search saved.json --query q` accept this combination. Combining saved search and fresh queries preserves every input, including failed or empty inputs, without assigning old hits to the newest source/query. Nested merges flatten the input lists in supplied order. Hit membership remains per input; acquisition may deduplicate identical targets and imported references without deleting provenance.

Writers emit search/v3. Readers continue to accept published search/v2, treating its source and opaque query string as one batch; the old semicolon-joined query string is not split speculatively. Reading never rewrites a file. Query transmission still requires explicit --query or --search-names. No content-derived URL is fetched.

This changes the search data contract, so search/v2 is frozen and search/v3 is the new identifier under ADR-0008. The API pin, catalog store, repository grouping, shared budgets and search-partial evidence from ADR-0007 remain unchanged. Catalogs describe pinned imports; this decision does not add persistent search history to catalog/state files.

Rejected alternative: disallow saved/live combinations. That is simpler but removes a useful workflow rather than retaining its evidence.

## Acceptance scenarios

| ID | Scenario | Required result |
| --- | --- | --- |
| Q1 | Published v2 saved hit plus a fresh query from another source | Each input keeps its source, query, hit membership and diagnostics in v3. |
| Q2 | Merge an already merged search, with overlapping hits and a failed/empty input | Inputs stay flat; all evidence survives; sync imports each deduplicated reference once. |
| Q3 | Read v2 or v3, then ordinary sync | Both are accepted; source files stay byte-for-byte unchanged; unknown schemas are rejected. |

## Third-review evidence (2026-10-29)

Implementation `77deb5b2eeceda97f5fa22614abf6f3c6e5acc7e` on canonical main. Q1–Q3 pass with different-source batches, an opaque published-v2 query, failed/empty inputs, nested flat merges and deduplicated sync imports. CLI tests save merged v3, consume it in sync and preserve the v2 source file. See [third-review record](../review-3-fixes.md) and [verification](../../VERIFICATION.md).

## Accepted amendment: one batch per new query (2026-10-29)

Explicitly approved by the user during the fourth review. Newly requested queries each produce one provenance batch, retaining the selected provider, that query's hits, completeness and diagnostics. Invalid, failed and empty-result queries keep their batch; selecting a provider is not proof of an HTTP request. Diagnostics identify their query. A multi-name invocation therefore preserves hit membership and independent failures for every requested name. This supersedes per-invocation grouping only for newly collected evidence. Published v2 and previously grouped unpublished v3 inputs remain intact; their query/hit membership cannot be recovered and must not be guessed. Merges preserve those groups while flattening the list. Unpublished search/v3 changes in place under ADR-0008. Q4 covers two successful queries plus failed/invalid/empty queries and legacy grouped-input readback.
