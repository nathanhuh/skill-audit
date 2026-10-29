# Document-driven rewrite comparison

## Revisions and scope

- Preserved reference: **8c6f3b83d725c716246bdbdecc8495bb9cedbccb**, canonical `main` at `/Users/nathanhuh/Documents/Documents - Nathan’s MacBook Pro/Interview_Repos/skill-audit`.
- Reviewed rewrite implementation: **b7257a55c479501179e9a936f0d7cfffea11e751**, local branch `codex/adr-rewrite`, in `/Users/nathanhuh/.codex/worktrees/2376/skill-audit`. The accompanying documentation/verification commit changes no production Go bytes.
- Starting worktree was clean, detached at the exact reference revision. The human explicitly selected this worktree for the rewrite. Canonical source, branch and history were not changed; nothing was pushed, published, submitted or sent to another person/chat.
- Evidence: [rewrite-results.json](rewrite-results.json), VERIFICATION.md, and the reproducible fixture-only scripts below. Local full command/report evidence is `/private/tmp/skill-audit-acceptance-results/`.

The rewrite implements the four-command inventory/search/sync/audit CLI. It includes on-demand static observations, explicit baselines/snapshots, optional public acquisition and a static-only Skill X-Ray adapter. Embedded IDs, move inference, linked-skill traversal, monitoring, enforcement and cloud integration remain outside scope.

## Method and source boundary

Initial implementation used AGENTS.md, all nine accepted ADRs, CONTEXT.md, DESIGN.md, README.md, PORTABILITY.md, PRODUCT.md and pinned data/dependencies. Existing production Go and tests were not consulted during that phase. A fresh end-to-end implementation and contract tests passed before baseline source comparison began. The pre-comparison source snapshot is `/private/tmp/skill-audit-fresh-before-comparison.tar.gz`.

After that first slice, the baseline was exported with `git archive 8c6f3b83d725c716246bdbdecc8495bb9cedbccb` into `/private/tmp/skill-audit-rewrite-baseline`, built there, and inspected for regression/compatibility analysis. This is **not formal clean-room development**. Source-informed adaptations were:

1. Preserve the published state observation envelope (`observed_at` / `report`), catalog-store entries/data/attribution, saved-search fields, report roots/origins/lineage, array forms and string values.
2. Reproduce the frozen `reference-catalog/v1` JSON fingerprint. The initial document-only draft used a different metadata encoding; that draft was replaced before delivery. Both binaries now emit catalog ID `f58437d5b9196bef29a13a53948b0e40d0b4b7a311ae586192342c4368ff813b` for the embedded catalog.
3. Preserve the published SA001 conservative option subset, file eligibility, finding key and coverage lists. The comparison corpus caught an unsupported-shebang `outside_scope` discrepancy; it was corrected. Rule v2 and evaluation `SA001/v2;sh/v3.10.0;scope/v2` remain unchanged.
4. Use the actual pinned SARIF properties (`no-reported-gap`/`incomplete`, execution status, effective severity and original source data), rather than guessed generic coverage fields. Skill X-Ray attribution remains separate from SA001.
5. Preserve ownership hints and historical origins, and preserve raw older observations on append rather than reconstructing and dropping their fields.
6. The final documentation cross-check exposed a rewrite regression against ADR-0003 root priority: a deep explicit home-global root could lose its budget to a shallow external root. The fix separates root and sibling priority and is covered in both binary harness runs. ADR-0007 cache-independent validation also checks each declared size on a blob cache hit. These are corrections under existing rules, not new policy.

All 45 pinned files (vendored parser sources/licenses, catalog bytes/manifest/notices, go.mod, go.sum and `.gitattributes`) match their pre-edit SHA-256 values. No scanner or dependency was installed. Skill/catalog/scanner content was treated as data; no skill script ran and no content-derived URL was fetched.

The publication freeze also checked the locally recorded `origin/main` revision **3974a313ffdecd2a82d9e0b8699d9d61c47c9cc0** without fetching. A temporary export of that exact revision produced real inventory/v2, report/v1 and state/v1 fixtures. `scripts/verify-published.py` confirmed comparison, unchanged legacy state bytes, append refusal, and exit 1 over the failed requested legacy append when a new finding exists. This supplements the shared frozen-baseline harness; it does not claim the local remote-tracking ref is a live remote check.

## Shared CLI results and differences

`scripts/acceptance.py` executed **17 commands per binary** using separately created equivalent temporary fixtures. It normalizes timestamps and fixture-root paths, asserts contract projections, and retains complete normalized reports/command outputs. It is not a claim of byte-identical reports or exhaustive regression equivalence.

| Evidence or difference | Baseline | Rewrite | Assessment |
| --- | --- | --- | --- |
| Global/project identities, two pinned exact hashes and catalog fingerprint | Pass | Pass | Preserved S1–S4 / V6 behavior. |
| Edit bundled script, refresh, compare and append snapshot | `changed`, SA001 `new` | Same | B1–B9 / V7; original observation preserved. |
| Ordinary audit leaves state bytes unchanged | Pass | Pass | ADR-0004 ordinary-read boundary. |
| Published state/v1 comparison / refused append | Pass | Pass | V1/V2, exit 3 without findings. |
| Published catalog-store/v2 external input | Pass | Pass | ADR-0008 compatibility. |
| Fourteen shell syntax/eligibility/evidence fixtures | Same projections | Same projections | Bounded SA001 regression evidence, not comprehensive shell safety. |
| Complete package plus skipped link | Anchor recorded; discovery partial | Same | B2/B3; coverage does not erase valid bytes. |
| Incomplete package with skipped linked content | No anchor | Same | D11 / B5. |
| Explicit home-global root versus shallow external root under budget | Global package reached | Same | ADR-0003 root priority / D5 regression fixed before delivery. |
| Explicit dependency root | Package discovered | Package discovered | D10 was already implemented in the reference; not a rewrite improvement. |
| Requested recording on text stdout | Missing | One summary | Expected E9 baseline gap. Stderr remains once. |
| Two packages with hanging fake scans, 4 s operation budget | Two scan launches | One launch; terminal cause reused | Expected X2/X9 change. Both runs preserve content and record baseline. |
| No scope flags | Refused | Injected default-device selection passes | Expected D9 gap; never exercised against real device roots. |
| Designated state location under discovery root | No dedicated exclusion mechanism | Exact file reserved with reported exclusion | B10/B11; `inventory --state PATH`, no parent-directory exclusion. |
| state/v2 reader | Accepted | Rejected | Expected V5, not a compatibility regression: v2 was unpublished. |
| Acquisition ordering and total output admission | Baseline gaps documented | Repository-grouped admission and bounded diagnostics tested | C5–C8; fake transports and smaller injected exhaustion budgets, not live GitHub. |

A further conservative difference follows D11: discovery dependency exclusions also stop measurement of relevant dependency subtrees inside a package, making it incomplete. The reference measured that content within its package limits. The rewrite never calls omitted bytes an exact fingerprint or anchor; metadata-only omissions remain complete under D12. Invalid frontmatter, unsupported shell syntax and size/read failures stay visible.

No remaining policy gap was identified during this implementation. Native platform acceptance scenarios and live integrations remain unverified as listed below; a future finding outside the accepted rules still needs an ADR decision before a fix.

## Architecture, dependencies and resource observations

The rewrite keeps one internal Go package with explicit file boundaries for discovery, measurement, matching, observation/comparison, persistence, wire compatibility, acquisition and external processes. It avoids the former workflow/audit wrappers. `wire.go` separates published representations from the internal observation model; this costs some conversion code but makes version compatibility explicit. Filesystem/process differences stay in `fs_*` and `process_*`.

| Non-vendored Go source | Baseline | Rewrite |
| --- | ---: | ---: |
| Production files | 24 | 16 |
| Production lines | 3,379 | 3,393 |
| Production bytes | 102,907 | 96,024 |
| Test files / lines | 26 / 3,900 | 5 / 1,340 |

Counts exclude Markdown, Python acceptance/measurement scripts, catalog content and vendor. Test counts reflect replacement with scenario-based suites and the shared black-box harness; fewer files/lines are **not proof of correctness**. Dependencies remain exactly yaml.v3 v3.0.1 and mvdan sh/v3 v3.10.0.

The macOS `wait4` measurement fixture contains 100 packages, 900 files and 28,227,490 raw content bytes. Three runs per phase/binary, filesystem caches uncontrolled:

| Phase | Baseline median wall | Rewrite median wall | Baseline maximum RSS | Rewrite maximum RSS |
| --- | ---: | ---: | ---: | ---: |
| Inventory | 107.305 ms | 110.702 ms | 18,137,088 B | 17,858,560 B |
| Audit | 181.156 ms | 195.124 ms | 18,923,520 B | 19,382,272 B |

The rewrite is slower on this small workload, with similar observed peak memory. These measurements are descriptive, not a general performance win or endpoint resource guarantee. Local raw byte maps are dropped after each measured package. Catalog accumulation, report metadata and JSON serialization still use memory; directory listing memory and uninterruptible OS calls are not covered by entry/deadline budgets. The first `/usr/bin/time -l` approach could not read a sandbox-restricted macOS sysctl; measurement uses direct-child `wait4` instead.

## ADR evidence and remaining limitations

| ADR | Rewrite evidence | Limits |
| --- | --- | --- |
| 0001 T1–T5 | Offline audit/measurement; non-execution marker; explicit opt-in/name cap; copied Go fake scanner; credential/argument tests | Supplied scanner remains trusted executable code; its network behavior is not enforced. |
| 0002 S1–S4 | Home-global nested plugin, project worktree, `.github`, same names, scope/path keys | Host-specific lexical identity; no move tracking. |
| 0003 D1–D12 | Routine exclusions, links, limits/pending roots, case aliases, default device injection, explicit dependency roots, package vs discovery completeness | D4 OS-alias policy source checked; D8 uses eight fixture exclusions to exercise bounded examples, not 50,000 real entries. D7 needs native Windows junction execution. |
| 0004 B1–B11 | Immutable first measurement, partial eligibility, empty scope, unavailable anchors, lock/record refusal, state exclusion/placement | Cancellation and final-sync errors are controlled fixtures/injections, not crash-durability proof. |
| 0005 E1–E9 | Priority table plus CLI recording/delivery failures and once-per-stream text summary | A failed stream cannot receive a report; successful commit remains independently reported on available outputs. |
| 0006 X1–X10 | Shared probe deadline, exit-75 retry, stop-after-timeout, reserve skip, launch failure, strict SARIF, real copied Go process/staging | X3/X4 use injected deadline admission checks; no actual 45 s scan or 6 s cold probe. Real Skill X-Ray/OpenGrep not run. |
| 0007 C1–C8 | In-memory fake GitHub/registry, Git blob checks, cache misses, pinned retries, grouping, attribution and raw/formatted-output admission | Exhaustion tests inject smaller limits; production 40/64 MiB constants are frozen. No authenticated/live provider test. |
| 0008 V1–V7 | Older published readers, no migration, v2 refusal, independent evidence comparison, frozen domain fixture checks | Publication freeze inventory is recorded in VERIFICATION.md; no publication occurred. |
| 0009 P1–P4 | OS boundary isolation, LF/CRLF behavior, Linux/Windows builds, Windows test compilation | P1 cross-host hash agreement, P3 descendant cleanup, P4/D7 junction runtime, Linux/native Windows full tests/demo and NTFS durability remain pending. WSL is not native Windows evidence. |

The existing POSIX demo still needs a Windows-native replacement before a support claim. Real device-wide discovery, real mount aliases, OS Ctrl-C injection, hostile filesystem races and crash/interrupted-write behavior were not exercised. No platform framework, service, database, UI or roadmap feature was added.

See VERIFICATION.md for exact commands and the current evidence record.
