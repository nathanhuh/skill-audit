---
status: accepted
date: 2026-10-29
source: brief requirement 3 gap ("originally pulled from open source, but then modified by the user"); requested by the user 2026-10-29 after the brief-coverage check at 7031637
---

# 0014. File-level comparison with the matched upstream reference

## Context

The brief asks the tool to track a skill that was "originally pulled from open source, but then modified by the user". Before this decision, file-level changes came only from a baseline the user had recorded. A skill modified before its first audit was correlated as `possible_modified` with an upstream commit and a similarity score. Its lineage was `not_compared`, and nothing showed which files differed from upstream. The catalog already stores per-file hashes for every reference, so the comparison needs no new data.

## Decision (accepted 2026-10-29)

1. **Every inferred candidate gets a file comparison.** For each candidate with basis `inferred_candidate`, whether the correlation is `possible_modified` or `ambiguous`, compare the local package's file manifest with that reference's manifest. Results use the relative manifest paths:
   - `added`: present locally, absent from the reference.
   - `removed`: present in the reference, absent locally.
   - `modified`: present in both, with different SHA-256 (hex compared case-insensitively).

   Changes are sorted by path. An `observed_exact_match` candidate has identical content by digest, so it gets no comparison.
2. **Incomplete measurements make no claim.** The comparison runs only when the local measurement is complete. Otherwise the candidate says `upstream_comparison: "unavailable"` and lists no changes, because skipped or unread files would appear as false `removed` entries.
3. **Representation.** The comparison is additive and optional on `Candidate`: `upstream_comparison` (`compared` or `unavailable`) and `upstream_changes` (`[{path, kind}]`). Under [ADR-0008](0008-versioned-data-contracts.md) rule 2 this needs no schema bump, and report/v2 is unpublished in any case. Text output prints `upstream_added=`, `upstream_removed=` and `upstream_modified=` lines under each candidate, or `upstream_comparison=unavailable`.
4. **Evidence, not attribution.** A difference is relative to the *pinned reference commit*. It can come from a local edit or from upstream changes the user installed from a different commit. The tool does not claim which one, or who made the change ([ADR-0001](0001-scope-and-trust-model.md) "Evidence, not verdicts").
5. **Independent of everything else.** It does not change correlation status, thresholds, the candidate set, the catalog identity, baseline lineage, completeness or exit codes. Baseline lineage ([ADR-0004](0004-observations-and-baselines.md)) and upstream comparison answer different questions: "what changed since I recorded it" and "what differs from the open-source version". Both are reported.

Rejected alternative: show upstream differences only when a baseline exists. That leaves the brief's main case, a skill modified before the tool ran, without file-level evidence.

## Acceptance scenarios

| ID | Scenario | Required result |
| --- | --- | --- |
| U1 | Copy of a catalog skill with one edited file, one added file and one deleted file, no baseline | `possible_modified`; candidate `upstream_comparison=compared` with exactly those three changes, sorted by path; lineage stays `not_compared`. |
| U2 | Unmodified copy of a catalog skill | `exact`; the candidate has no upstream comparison fields. |
| U3 | Modified copy whose local measurement is incomplete | Candidate says `upstream_comparison=unavailable` and lists no changes. |
| U4 | Same modified copy audited with and without a baseline | Upstream comparison identical in both; baseline lineage, completeness and exit code are unaffected by it. |

## Implementation gaps

None at acceptance. Implemented together with this ADR; see [scenario-tests.json](scenario-tests.json).
