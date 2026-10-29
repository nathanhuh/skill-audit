---
status: accepted
date: 2026-10-29
source: AGENTS.md ("preserve existing schema versions unless the data contract changes"); former ADR-0001 data-contract row; review rounds 2026-10-29; accepted by the user in ADR review on 2026-10-29 with evidence-specific compatibility and ADR-0005 exit-priority clarifications
---

# 0008. Versioned data contracts

## Context

Reviews raised two opposite complaints: schemas bumped where the change could have been derived (state/v3 locking out v2 snapshots), and behavior changed without a bump (discovery/v3). There was no rule for when a version changes or what readers owe older files.

## Current identifiers

| Kind | Identifier | Published on origin/main |
| --- | --- | --- |
| Report | `skill-audit/report/v2` | no (v1 published) |
| Inventory | `skill-audit/inventory/v3` | no (v2 published) |
| Discovery policy | `skill-audit/discovery/v3` | no |
| State | `skill-audit/state/v3` | no (v1 published; v2 local only) |
| Catalog / catalog store | `catalog/v1`, `catalog-store/v2` | yes |
| Search | `search/v2` | yes |
| Fingerprint domains | `directory/v1`, `reference-catalog/v1` | yes |
| Local rule | SA001 v2 | yes |

## Decision *(accepted)*

1. **Freeze on publication.** An identifier is frozen once it reaches `origin/main` or a submitted archive. Before that, an unpublished identifier may change in place, with no compatibility obligation.
2. **When to bump a frozen identifier.**
   - *Shape schemas* (report, inventory, state, catalog, search): bump for any removed or renamed field, any changed meaning, or any changed set of allowed values. Additive optional fields and additive diagnostic codes do not bump.
   - *Behavior identities* (discovery policy, rule/evaluation versions, fingerprint domains): bump for any behavior change that could alter what is reported or compared. Different identities do not establish comparable results for the affected kind of evidence. Assess compatibility independently for discovery coverage, content fingerprints, scanner evaluations and reference-catalog correlation. A discovery-policy change prevents claims of equivalent discovery coverage but does not disable comparison of complete package measurements using the same fingerprint algorithm and local identity. A scanner-evaluation change prevents comparable finding-transition claims for that scanner without disabling otherwise valid content comparison. Changed reference catalogs leave their upstream correlation evidence separate while local baseline comparison can remain available.
3. **Readers support every published version. Writers emit only the current one.**
4. **No migration.** State in an older published version can still be compared but cannot be appended to. Adopting a new version means recording a new baseline at a new path. Files are never upgraded in place, silently or explicitly. Old observations are never rewritten.
5. **Unpublished intermediates are dropped before publication.** state/v2 existed only in local commits, so its reader is removed instead of being supported forever.
6. **Freeze checklist before publishing.** Each changed identifier is either bumped or confirmed unchanged against the last published version. VERIFICATION.md records the result.

**Alternative considered and rejected for (4):** read state/v2 and append state/v3 observations to it, with a visible `state_upgraded` diagnostic. This is cheaper for users who already have state, but it adds a migration path that 0004's immutable-history principle otherwise avoids. Only local test files use v2 today.

## Consequences

- The latest review findings that state/v3 blocks v2 snapshots and that discovery/v3 changed without a bump are resolved by rules 1 and 5. No bump is needed now. Both are frozen together at publication.
- A post-publication change to exclusion rules creates `discovery/v4`; its discovery coverage is not treated as equivalent to v3 coverage. Complete package content remains comparable where local identity and fingerprint algorithm are compatible.
- A failed attempt to append to an older state version reports `not_recorded`. Exit priority follows ADR-0005: exit 1 when security findings are present, otherwise exit 3 for the failed requested recording.

## Acceptance scenarios

| ID | Scenario | Required result |
| --- | --- | --- |
| V1 | Published state/v1 file, `--record snapshot` | Comparison works where the evidence is compatible; append refused with guidance to record a new baseline elsewhere and recording outcome `not_recorded`. Exit 3 without security findings; exit 1 with findings, following ADR-0005. |
| V2 | Published report/v1 observation in state | Read without a fabricated `recording` field. |
| V3 | Published inventory/v2 input | Accepted; discovery policy reported as unknown; original diagnostics kept. |
| V4 | Unknown schema identifier | Rejected with a hard diagnostic; nothing is reset. |
| V5 | state/v2 file after rule 5 | Rejected as an unsupported schema. |
| V6 | Discovery-policy identifier changes, but the same local package has complete measurements under the same fingerprint algorithm | Do not claim equivalent discovery coverage; preserve valid baseline-relative package-content comparison. |
| V7 | Scanner evaluation version changes while package fingerprints remain comparable | Do not infer a comparable new/resolved finding for that scanner; preserve valid content comparison and other independently comparable evidence. |

## Implementation gaps (at 5fed934)

| Area | State |
| --- | --- |
| V1–V4 | Behavior present (`internal/audit/state.go`, `internal/workflow/audit.go:15`). |
| V5 | **Gap.** The state/v2 reader (`AnchorStateSchema`) still exists and validates explicit anchors. |
| V6–V7 | Evidence-specific compatibility is accepted; implementation and fixture coverage have not been fully reviewed or verified in this documentation pass. |
| Freeze | Not yet run. Must run before the take-home is submitted. |

## Isolated rewrite evidence (2026-10-29)

Implementation revision `b7257a55c479501179e9a936f0d7cfffea11e751` on `codex/adr-rewrite`, separate from canonical main. This supplements the dated gap table above; it does not change the accepted decision.

V1–V7 are covered by published state/report/inventory/catalog/search representations, state/v2 refusal, read-only legacy append behavior and independently comparable discovery/content/scanner/catalog evidence. The embedded directory and reference-catalog fingerprints match the frozen binary. No published identifier was bumped or intentionally redefined; the publication freeze checklist is in VERIFICATION.md. Nothing was published.

See [rewrite comparison](../rewrite-comparison.md) and [verification](../../VERIFICATION.md) for exact commands and limits.
