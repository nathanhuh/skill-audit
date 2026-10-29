# Verification — ADR step 2: observed anchors, 2026-10-29

Based on 71e3cee. B1–B9 now have scenario tests: complete packages survive unrelated exclusions/link/budget/scanner gaps; partial observations without usable anchors and interrupted runs are refused; completed empty scopes are allowed; missing packages and initially incomplete measurements never fabricate deletions or new findings. A real CLI B2/B4/E5 fixture records one anchor while returning coverage exit 2. B9 appends a later complete observation and verifies the original anchor set remains unchanged. Legacy state/v1 is comparison-readable and append-read-only; corrupt anchors are rejected. State/v2 references first-observation measurements, report/v2 preserves discovery evidence, inventory/v3 carries policy/home/limits/exclusions. macOS alias exclusions are covered by a policy table, without scanning the device.

Offline macOS arm64 / Go 1.23.5, same GOCACHE/GOPROXY/GOSUMDB settings as below. gofmt applied. go vet ./... exit 0 after correcting an alias-related unkeyed-literal warning. Full go test -count=1 ./... exit 0 (cmd 0.687s, audit 3.497s, workflow 1.267s); then focused B1–B9/legacy-state tests exit 0 (0.411s) after strengthening B9 append checks. Native build /private/tmp/skill-audit-adr-baselines exit 0. Fixture-only sh scripts/demo.sh exit 0, including comparable new SA001 and preserved baseline. No live services, installed skills or skill scripts were used. Native Linux/Windows and real Skill X-Ray remain unverified; cross-builds will run with final verification.

## Prior verification

# Verification — ADR step 1: outcomes, 2026-10-29

Canonical main started at cb4be34. Implemented E1–E8 with a table-driven operation-outcome test, real CLI baseline/snapshot/failure tests, and an injected post-rename directory-sync failure test. Findings take priority over recording and coverage; recording is independently structured in report/v2 and printed on stderr for every analyzed audit. Historical report/v1 observations remain readable without fabricated outcome metadata. Old coverage-first assertions were changed to require both exit 1 and retained gaps.

macOS arm64, Go 1.23.5; GOCACHE=/private/tmp/skill-audit-gocache GOPROXY=off GOSUMDB=off. gofmt applied; go vet ./... exit 0; go test -count=1 ./... exit 0 (cmd 0.477s, audit 3.360s, workflow 1.274s); native build /private/tmp/skill-audit-adr-exits exit 0; sh scripts/demo.sh exit 0 (audit finding exit 1). Tests use fixtures and loopback HTTP only; no live service calls. Initial suite exposed three old coverage-first assertions; all now match E3. Native Linux/Windows and real Skill X-Ray remain unverified. Baseline/ownership/exclusion changes are still pending in subsequent steps.

## Prior verification

# Verification — accepted contract freeze, 2026-10-29

This change records user-approved design in ADR-0001, domain language in CONTEXT.md, and pointers in AGENTS/DESIGN/README. It changes documentation only. All previously tracked non-documentation files were checked by SHA-256 against the clean `ed4c74f03cda5f916ce6ae5799160bef1c95ad63` checkout and remain identical, including Go source/tests, scripts, dependency/catalog bytes and licenses.

ADR-0001 contains 21 planned acceptance scenarios and an explicit implementation-gap table. **The current runtime does not yet implement all accepted baseline, exit and ownership contracts.** Passing the existing suite below is regression evidence for unchanged code, not proof that the planned scenarios pass.

Documentation validation: all new local Markdown links resolve, accepted status/date are present, all 21 scenario IDs are present, no trailing whitespace, and `git diff --check` exits 0.

Repository-required checks ran from the original canonical checkout on macOS arm64, Go 1.23.5. Go commands used `GOCACHE=/private/tmp/skill-audit-gocache GOPROXY=off GOSUMDB=off`; the suite used approved loopback-only fake HTTP servers, with no live service calls or skill-script execution.

- `gofmt -l cmd internal`: no output.
- `go vet ./...`: exit 0.
- `go test -count=1 ./...`: exit 0:

```text
ok  skill-audit/cmd/skill-audit    0.969s
ok  skill-audit/internal/audit    6.770s
ok  skill-audit/internal/workflow 1.751s
```

- `go build -o /private/tmp/skill-audit-contract-freeze ./cmd/skill-audit`: exit 0.
- `sh scripts/demo.sh`: exit 0; its finding-producing audit returned expected exit 1 under current behavior.

Cross-builds were not repeated for this documentation-only change; prior results are preserved below. Native Linux/Windows, real scanner/runtime and real device-wide discovery remain unverified. No code or schema migration, publishing, pushing or submission occurred.

## Prior verification records

# Verification — import limits and persistence outcomes, 2026-10-29

Reviewed canonical original `main` at `2b6fc1f274a6d2ff54c6272466274885d6a30eae`. The supplied findings were hypotheses. The table separates executed reproductions, injected fault evidence and source qualifications.

| Item | Evidence and disposition |
| --- | --- |
| 1. Evicted content counted twice | **Reproduced before fix.** A valid 9 MiB package imported from A, then B, then A lost its final import under the stale ledger. Cache bytes now track retained memory only and reset on eviction; reload does not alter package eligibility. |
| 2. Monorepo shares 16 MiB cap | **Reproduced before fix.** Four distinct approximately 5 MiB packages returned only three. The previous code did emit `package_incomplete` (so the loss was not silent), but the aggregate cap was wrong. All four now import. MaxFile/MaxPackage/MaxEntries/request limits remain; a full 16 MiB cache declines further admission instead of rejecting valid bytes. The cache stays bounded. |
| 3. Transient lookup poisoned repo | **Reproduced before fix** for initial 502, 403 and dropped transport. Later targets now recover. Permanent private/truncated failures remain memoized with existing request-count tests. HTTP 404 is classified permanent by its typed status; transient status/transport errors are not memoized. |
| 4. Linked skills counted complete | **Reproduced using a rebuilt frozen 2b6fc1f binary**: a .claude/skills/foo link into .agents yielded no inspected packages, inventory exit 0 and accepted baseline. The current binary rejects the same legacy inventory with exit 3. Non-routine skipped links now make coverage incomplete; targets are never followed. Routine metadata/dependency exclusions remain notes. Legacy informational `policy_symlink` diagnostics are promoted to gaps during audit. Evidence: `/private/tmp/skill-audit-link-repro-qn_491hh/commands.json`. |
| 5. Committed save reported unsaved | **Injected final-directory-sync fault**, not an observed native filesystem failure. The test proves replacement bytes are visible and a typed `CommittedWriteError` is returned. The CLI classifies it as `state_recorded_durability_unconfirmed`, stderr says recorded/durability unconfirmed, and exit 2 avoids the failed-recording exit 3. Tests distinguish plain pre-commit errors. No automatic retry or duplicate write is performed. |
| 6. Fixed probe cap ignores timeout | **Qualified.** The CLI parent operation deadline already bounded probing, so it did not ignore an expired --timeout. The fixed five-second child cap could shorten longer configured budgets. Probe and scan now both use the configured timeout under their parent context. A deterministic process-boundary test captures a 12-second configured probe and a shorter parent deadline; fake executables still test real process arguments/timeouts. |
| 7. Cancellation repeats setup | **Not reproduced in the normal workflow**: it already breaks before later packages when canceled. The session API could retry setup if called again. Added an entry cancellation guard and terminal canceled-probe state. Tests cancel during a probe, call the session repeatedly and verify one process call and one temporary root; an already-canceled session creates none. |
| 8. Input diagnostic forces exit 3 | **Reproduced before fix** with an informational inventory diagnostic named state_not_recorded masking a real SA001 finding. Analysis ExitCode now handles findings/coverage only; a local CLI boolean from the actual state phase selects exit 3. Compiled checks cover forged inventory and catalog diagnostics, including a successfully saved finding snapshot that returns 1. Readable output receives the actual exit code explicitly. |
| 9. Output placement duplicated | **Source-confirmed cleanup.** Inventory passes observed package roots to the shared WriteJSON placement check. A regression removes SKILL.md after observation and verifies output remains forbidden inside that package. Preflight alias checks and final no-follow guards retain their distinct roles. |
| 10. Scanner wrapper only used in tests | **Source-confirmed cleanup.** Removed production XRayScan. Workflow and core report fixtures use the audit-scoped session; standalone test setup has a private test helper. No production references remain. |

No JSON schema, pinned catalog bytes, dependency version or scanner/rule version changed. Exit 3 still means an actual failed record phase before commit. A committed record with uncertain directory durability gets exit 2 plus an explicit recorded warning; callers should inspect state before retrying. Cache capacity is separate from import limits. Accumulated catalog/serialization memory is still proportional to retained results.

## Final checks from original main checkout

Host: macOS arm64, Go 1.23.5. Go commands used `GOCACHE=/private/tmp/skill-audit-gocache GOPROXY=off GOSUMDB=off`, pinned vendored dependencies and approved loopback-only fake HTTP servers. No live public service calls, scanner installation or skill-script execution occurred.

`gofmt -l cmd internal`: no output. `git diff --check`: exit 0. `go vet ./...`: exit 0. `go test -count=1 ./...`: exit 0:

```text
ok  skill-audit/cmd/skill-audit    0.713s
ok  skill-audit/internal/audit    6.171s
ok  skill-audit/internal/workflow 1.376s
```

- Native build `/private/tmp/skill-audit-outcomes`: exit 0.
- Linux amd64 cross-build `/private/tmp/skill-audit-outcomes-linux`: exit 0.
- Windows amd64 cross-build `/private/tmp/skill-audit-outcomes-windows.exe`: exit 0.
- Windows audit test compilation `/private/tmp/skill-audit-outcomes-audit.test.exe`: exit 0; compilation only.
- `sh scripts/demo.sh`: exit 0, expected finding audit exit 1.
- Twelve compiled CLI checks passed: routine-exclusion inventory/baseline, updated finding inventory, forged inventory/catalog diagnostics returning 1, successful finding snapshot, real baseline/lock failures returning 3, managed-link discovery and baseline refusal, legacy-link refusal, interrupted snapshot preservation and output placement rejection. Evidence: `/private/tmp/skill-audit-outcome-smoke-8kjx2ykb/commands.json`.
- Three additional frozen/current CLI checks reproduce the skipped-link regression on 2b6fc1f and show current refusal of its saved inventory. The old source was extracted to a disposable directory and built offline, without altering the canonical checkout.

Native Linux/Windows runtime, real Skill X-Ray integration, real device-wide discovery and a native post-rename directory-sync failure remain unverified. Timeout/cancellation conclusions use fixtures and deterministic process boundaries, not real scanner cold-start performance. No publishing, pushing or submission occurred.

## Prior verification records

# Verification — coverage/root/cache policy correction, 2026-10-29

Reviewed canonical original `main` at `cf23e89aac804a9d621c0f7d7c4da3e2724f973f`. Failing offline regressions reproduced routine-exclusion baseline refusal, hidden recording failures, community-installation scope changes, home-root starvation, repeated failed repository requests, retained inactive content caches and startup consuming package timeout. The component-depth collision, duplicate coverage checks and placement wording were confirmed by source inspection and covered by focused tests.

| Item | Result |
| --- | --- |
| 1. Routine exclusions block baselines | Inventory `.git`, dependency caches, implicit OS subtrees and skipped inventory links remain visible policy notes with `incomplete=false`. Unreadable paths, limits, rejected explicit roots and skipped package content remain gaps. A realistic project fixture now inventories and records baseline with .git/node_modules; an outside-package link remains visible, while a bundled link still prevents a complete baseline. |
| 2. Recording failure indistinguishable | Requested state-phase errors emit `state not recorded` on stderr, retain `state_not_recorded` in the report and return **exit 3**. Tests cover locks, existing baselines, corrupt snapshot state and interruption. Coverage-only/preflight failures retain exit 2. This is an intentional CLI exit-contract extension. |
| 3. Community global layouts | Scope covers home agent trees including .codex/superpowers/skills and .claude/community/skills. Known worktrees/projects containers and nearer nested agent markers remain project scoped; .github remains project scoped. Existing observations are never rewritten. |
| 4. Root depth collision | Component depth is calculated after Clean and volume/root removal. Host-root tests pass. Native-Windows drive/UNC cases compile but were not run on Windows. Explicit descendants are delegated to their own traversal, so correctness no longer depends on which ancestor runs first. |
| 5. Deep root starves home | Root priority precedes depth. Selected home and its global installation roots precede external roots. Tight-budget fixtures cover both HOME itself and a selected global skill root. Explicit subtree delegation preserves independent depth scopes. The per-directory visited map is removed; overlapping-root counts still match. |
| 6. Failed lookups repeated | Repository/ref failures are memoized for the invocation when requests were actually attempted. Four private targets use one request; four truncated-repository targets use three requests. Cancellation and budget refusals do not accumulate new failure memos. A new invocation retries with a fresh client. |
| 7. Content cache grows per repo | Only one repository's parsed tree and up to 16 MiB blob cache remain active. Compact pinned references, failures and import-byte counters are retained. Switching back reloads pinned content and consumes request/byte budget; eviction/revisit tests pass. Accumulated catalog data and serialization still consume memory. |
| 8. Startup poisons audit | Version probing gets a separate five-second budget per attempt and one retry for a failed probe process. Package timeouts begin after startup. Slow-start and deterministic first-process-failure fixtures pass; incompatible versions remain terminal. Parent operation cancellation still bounds every phase. |
| 9. Output says state / duplicate link guard | Neutral `OutputOutside` checks placement. `WriteJSON` uses one no-follow check at the atomic write boundary, and reports neutral placement errors. State separately checks before parent/lock creation and again before final replacement because both phases write. Tests prevent linked state paths from creating outside parents, and output links cannot overwrite their destinations. |
| 10. Completeness duplicated | `Report.HasCoverageGaps` covers report/package diagnostics, package Complete flags and scanner status. Both baseline eligibility and exit 2 use it; the extra baseline per-skill loop is removed. Policy notes do not erase genuine package/scanner gaps. |

Fresh inventory is required for the corrected routine-exclusion flags; older persisted diagnostics are not silently reinterpreted. Baselines remain immutable. Exit 3 is new; automation should treat it as failure to record, while exit 2 can accompany a successfully saved partial snapshot. JSON schemas and scanner/rule versions are unchanged.

## Final verification from original main checkout

macOS arm64, Go 1.23.5. Go commands used `GOCACHE=/private/tmp/skill-audit-gocache GOPROXY=off GOSUMDB=off`, pinned vendored dependencies, and approved loopback access for fake HTTP servers. No live public service calls or skill-script execution occurred.

`gofmt -l cmd internal`: no output. `git diff --check`: exit 0. `go vet ./...`: exit 0. `go test -count=1 ./...`: exit 0:

```text
ok  skill-audit/cmd/skill-audit    1.196s
ok  skill-audit/internal/audit    7.681s
ok  skill-audit/internal/workflow 1.878s
```

- Native build `/private/tmp/skill-audit-policy`: exit 0.
- Linux amd64 cross-build `/private/tmp/skill-audit-policy-linux`: exit 0.
- Windows amd64 cross-build `/private/tmp/skill-audit-policy-windows.exe`: exit 0.
- Windows audit/workflow test compilation `/private/tmp/skill-audit-policy-{audit,workflow}.test.exe`: exit 0; compilation only.
- `sh scripts/demo.sh`: exit 0, expected finding audit exit 1.
- Twelve compiled-CLI checks passed: community/global and nested/project inventory with routine exclusions, valid baseline, repeated baseline/locked/corrupt/interrupted record failures, partial inventory/read-only audit/baseline refusal, home priority, bundled-link inventory and baseline refusal. Evidence: `/private/tmp/skill-audit-policy-smoke-dhhcjz8p/commands.json`.

Native Linux/Windows, real Skill X-Ray and real device-wide discovery remain unverified. Cold-start retry uses a fake executable; it does not establish real scanner startup performance. Cache eviction bounds retained cache content, not total catalog/output memory. Large directories and genuine coverage gaps can still limit discovery. No publishing, pushing or submission occurred.

## Prior verification records

# Verification — boundary and reuse fixes, 2026-10-28

Reviewed and changed the canonical original checkout on `main`, starting at `3974a313ffdecd2a82d9e0b8699d9d61c47c9cc0`. The first seven items below were reproduced with failing offline regressions before their fixes. Request/probe repetition was measured before reuse changes. Shared-write duplication was established by source inspection.

| Item | Change and evidence |
| --- | --- |
| 1. Names transmitted before missing-output error | Mandatory sync output is checked before reading name-search input or requesting data. A fake transport recorded one request before the fix and zero after it; the compiled CLI reports the missing output before its transmission notice. |
| 2. Interrupted immutable baseline | State recording now accepts context, checks it before work and again before atomic rename. Baselines require complete selected-scope coverage/package evaluations; findings alone are allowed. Canceled/deadline audits retain reports with `state_not_recorded` and cannot create/append state. Tests preserve existing bytes, permit a later valid baseline, refuse incomplete baselines, and preserve non-interrupted partial snapshots. |
| 3. Nested worktree scope | Classification follows the nearest agent marker and limits global installations to immediate home-agent skills/plugins trees. Nested `.codex/worktrees` project fixtures retain project scope and the inner agent hint. Relative home hints preserve the same classification. |
| 4. Explicit root silently lost | Roots run deepest first; skipped directories are not marked visited. Fixtures cover dependency exclusions, depth-cut roots and admitted ancestors with deeper content. Overlap/budget/coverage tests remain passing. Hard OS exclusions and no-follow policy still apply. |
| 5. Cache hit bypasses mode/type/size | Regressions previously imported symlink, submodule and inconsistent-size entries sharing a fetched SHA. Validation now precedes every cache lookup, including a cached-length check; unsafe packages are refused while regular repeated bytes remain supported. |
| 6. Empty finding comparison | Incomplete baseline/current comparison explicitly resets all finding changes to `not_compared`, including previously set labels. Both incomplete-side cases pass. |
| 7. `.github` scope depends on home | Selected/custom and conventional homes now consistently classify `.github` as project scope. Current and alternate-home fixtures pass. |
| 8. Repeated repository requests | A sync client caches a validated repository/ref snapshot and verified blobs for one operation. Three target imports now use four total requests in the shared-content fixture (metadata, commit, tree, blob); distinct refs do not share resolution. The 16 MiB cached-byte bound spans targets for the same repository/ref. Request/size limits and diagnostics remain. |
| 9. Repeated write/hash/exclusion helpers | State and report/catalog output share `WriteAtomic`, including file flush, cancellation before rename and OS-specific directory sync. Destination preservation, successful replacement, temp cleanup and placement tests pass. Raw SHA-256 and fingerprint metadata exclusions are shared. Discovery dependency exclusions remain a separate policy. |
| 10. Per-package X-Ray probe | Workflow owns one lazy X-Ray session. The two-package fixture changed from two probes/two scans to one probe/two scans. Each scan still has an isolated staging/report directory; a missing second report stays failed rather than reusing the first SARIF. Probe failures are cached, session temp data is cleaned, and local findings survive. |

No wire schema or scanner/rule version changed. Scope correction can change local identities: refresh inventories and review old baseline identities rather than silently rewriting history. Complete-baseline policy is stricter: broad device scans with coverage gaps need a narrower scope before creating a baseline. Historical incomplete states remain readable and receive unavailable comparisons where appropriate.

## Final verification from the original checkout

Host: macOS arm64, Go 1.23.5. Go commands used `GOCACHE=/private/tmp/skill-audit-gocache GOPROXY=off GOSUMDB=off` and pinned vendored dependencies. Full tests were approved to bind loopback-only fake HTTP servers. No live service calls, external scanner installation or skill-script execution occurred.

`gofmt -l cmd internal`: no output. `go vet ./...`: exit 0. `go test -count=1 ./...`: exit 0:

```text
ok  skill-audit/cmd/skill-audit    0.833s
ok  skill-audit/internal/audit    3.307s
ok  skill-audit/internal/workflow 1.376s
```

- Native build `/private/tmp/skill-audit-boundaries`: exit 0.
- Linux amd64 cross-build `/private/tmp/skill-audit-boundaries-linux`: exit 0.
- Windows amd64 cross-build `/private/tmp/skill-audit-boundaries-windows.exe`: exit 0.
- Windows audit-test compilation `/private/tmp/skill-audit-boundaries-audit.test.exe`: exit 0; compilation only.
- `sh scripts/demo.sh`: exit 0, with expected finding-producing audit exit 1.
- Nine compiled-CLI checks passed: scope inventory, relative-home equivalence, missing sync output, valid baseline, interrupted baseline/snapshot, partial inventory, incomplete-baseline refusal and overlapping explicit-root depth recovery. Evidence: `/private/tmp/skill-audit-boundary-final-y3w6_qh9/commands.json`.

Native Linux/Windows runtime, real Skill X-Ray integration and real device-wide discovery remain unverified. The supplied scanner remains trusted executable code; caching its probe does not prevent executable replacement races. Repository and scanner caches live only for the current operation. No publishing, pushing or submission occurred.

## Prior verification records

# Verification — promotion to original main checkout, 2026-10-28

The tested inventory/search/sync/audit implementation is promoted into the original repository on `main`. Before edits, all 85 source files matched the previous tested V2 archive by SHA-256. The original checkout's 73 files were backed up and verified against an archive before replacement. Eight obsolete V1-only files were removed from the active tree: COMPARISON.md, EXPANDED_SCOPE.md, REBUILD_PROMPT.md, internal/audit/device.go, device_test.go, public.go, public_test.go, and scripts/demo-device.sh. Subsequent promotion edits only identify the canonical checkout in README/AGENTS and record this verification.

A local main commit records the complete implementation, tests, pinned dependencies/catalog and licenses. The recovery archive and its manifest are retained under `.git/skill-audit-backups/promotion-l1havboq/` outside the reviewed source tree. Nothing is pushed, published or submitted.

The app refused to attach the primary repository as a managed worktree and refused to archive this chat's primary checkout. The attached V2 worktree therefore remains, synchronized to the canonical main commit; it must not be used as an independent development copy. AGENTS.md directs future edits/reviews to the primary checkout. The app attachment itself was not changed.

## Checks run from the original repository

Host: macOS arm64, Go 1.23.5. Go commands used `GOCACHE=/private/tmp/skill-audit-gocache GOPROXY=off GOSUMDB=off`. The full suite needed approved loopback access for local fake HTTP servers. No live service calls, installed-skill mutations or skill script execution occurred.

`gofmt -l cmd internal`: no output. `go vet ./...`: exit 0. `go test -count=1 ./...`: exit 0:

```text
ok  skill-audit/cmd/skill-audit    1.019s
ok  skill-audit/internal/audit    5.322s
ok  skill-audit/internal/workflow 1.164s
```

- Native build `/private/tmp/skill-audit-promoted`: exit 0.
- Linux amd64 cross-build `/private/tmp/skill-audit-promoted-linux`: exit 0.
- Windows amd64 cross-build `/private/tmp/skill-audit-promoted-windows.exe`: exit 0.
- `sh scripts/demo.sh`: exit 0, with expected finding audit exit 1.
- Eight compiled CLI checks: help succeeds for inventory/search/sync/audit, and retired scan/catalog/baseline/snapshot commands are rejected with exit 2.

Native Linux/Windows runtime, real device-wide discovery and real Skill X-Ray integration remain unverified. Promotion changes repository ownership/location, not those evidence limits.

## Prior implementation verification

# Verification — traversal follow-up, 2026-10-28

This section records evidence for the latest ten-item pass separately from the prior review. The cited `internal/audit/device.go` and `public.go` belong to the retired implementation and are absent from the active inventory/search/sync/audit CLI. Source inspection and native runtime evidence are not interchangeable.

| Latest item | Current assessment and evidence |
| --- | --- |
| 1. User directories starved by device budget | The duplicate Data-tree risk was already addressed, but ordering starvation remained. **Reproduced before changes** using synthetic Users/home trees: 50 Applications files exhausted a 20-entry budget with zero packages. After priority traversal, both global and project packages appear before a visible entry-limit diagnostic. This demonstrates the failure mechanism, not the claim that every real macOS device scan fails. No real device-wide scan was run. |
| 2. Newly added skills omitted | Fixed in the previous patch; current complete/incomplete first-observation and snapshot regressions pass. `not_in_baseline` does not establish installation time. |
| 3. Explicit repository narrowed by search | The current CLI keeps full-repository targets distinct from path/slug hints. Existing CLI fake-transport regressions pass; no current narrowing failure was reproduced. |
| 4. Windows state failure | Source-inspected, not reproduced on Windows. The prior OS-specific helper skips directory Sync on Windows. Native NTFS replacement/durability and the Windows-only regression remain **not run**. Cross-build success is only compilation evidence. |
| 5. Flags/CRLF shebangs skipped | Previously fixed; retained tests exercise supported headers and raw-byte evidence. No additional scanner change in this follow-up. |
| 6. Cancellation misreported | The current code used generic `walk_error`, not the retired `device_limit`, and omitted pending roots. **Reproduced before changes** with canceled/deadline contexts. Now `inventory_canceled` / `inventory_timeout` identify interrupted and unstarted roots. Mid-traversal cancellation preserves collected package evidence. Context cancellation is tested; an actual OS Ctrl-C signal was not injected. |
| 7. HOME override blocked | Not present in current CLI; existing regression unsets HOME/USERPROFILE and passes with explicit `--home`. |
| 8. Zero limits become defaults | Not present in current CLI. Zero entry/package/depth/search limits fail before work; retired `--public-requests` is an unknown flag. A zero internal request budget makes zero requests instead of resetting to 100. New tests and compiled-CLI checks pass. |
| 9. Duplicate containment | Addressed in the previous patch through `audit.Within`; file-alias and symlink checks serve different purposes. |
| 10. Visited map / repeated stat | The current map is useful for overlapping roots. It now stores directory paths only, rather than every file. Root Lstat metadata is reused by the priority walker. Regression and compiled CLI checks verify equal package and entry counts with overlapping roots. |

## Final checks for this follow-up

macOS arm64, Go 1.23.5. Commands used `GOCACHE=/private/tmp/skill-audit-gocache GOPROXY=off GOSUMDB=off` and vendored dependencies. The full suite was authorized to bind loopback listeners for its fake HTTP servers; no live public search or registry was called.

`gofmt -l cmd internal`: no output. `go vet ./...`: exit 0. `go test -count=1 ./...`: exit 0:

```text
ok  skill-audit/cmd/skill-audit    0.505s
ok  skill-audit/internal/audit    5.991s
ok  skill-audit/internal/workflow 0.783s
```

- `go build -o /private/tmp/skill-audit-priority ./cmd/skill-audit`: exit 0.
- `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /private/tmp/skill-audit-priority-linux ./cmd/skill-audit`: exit 0.
- `CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o /private/tmp/skill-audit-priority-windows.exe ./cmd/skill-audit`: exit 0.
- `sh scripts/demo.sh`: exit 0; expected finding audit exit 1.
- Eleven compiled-CLI invocations: deterministic bounded priority discovery twice, deadline diagnostics for two unstarted roots, six invalid/retired limit combinations, and two overlapping-root comparisons. All expected exits/assertions passed. Evidence: `/private/tmp/skill-audit-priority-smoke-q96167ux/{commands,priority,timeout}.json`.

Priority ordering is a heuristic, not an exhaustive-discovery guarantee. A large home may still exhaust the budget before another user/project. Directory listings are materialized before sorting, so entry limits do not cap a single unusually wide listing's memory. Existing Data-only/mount-alias exclusions remain. Native Linux/Windows, real device-wide discovery, actual Ctrl-C signal injection and real Skill X-Ray integration were not run. No real installed skills were modified or executed.

## Previous verification records

# Verification — ten-item correctness review, 2026-10-28

Reviewed against the sole current `cmd/skill-audit` workflow. Source paths in the review sometimes refer to the retired CLI; their behavioral equivalents were checked before edits.

| Item | Current disposition |
| --- | --- |
| 1. macOS duplicate Data traversal | Same risk in `workflow/inventory.go`; exclude `/System/Volumes/Data` and descendants on macOS, including explicit selections, with existing incomplete-coverage diagnostics. Unified `/Users` remains eligible. Policy tables cover exact boundaries and other OSes. No real device-wide scan or mounted-volume integration test was run. |
| 2. Packages absent from baseline | Fixed: emit `not_in_baseline`, no baseline origin/file-addition claims, findings `not_compared`. Complete/incomplete fixtures, duplicate names across scopes, immutable baseline and appended snapshot checked. |
| 3. HOME before flags | Already absent in this CLI: lookup errors are ignored and `--home` overrides the default. Regression unsets HOME and USERPROFILE and tests both explicit and absent home hints. Empty home no longer makes a relative agent directory global. The retired `catalog` command remains retired. |
| 4. Shebang eligibility | Fixed: interpreter token, CRLF/trailing whitespace, sh/bash flag-only arguments and supported env forms. Unsupported interpreters/arguments remain partial. Raw hashes, evidence offsets and line numbers tested. |
| 5. Hints shrink explicit imports | Already absent: full-repo and hinted targets are distinct. CLI fake-transport regressions verify full imports survive both GitHub path hints and registry slug hints. |
| 6. Explicit repo lost to result limit | No repository-count truncation in current CLI. Regression tests per-repository `--limit=1`, visible omitted-package diagnostics and both explicit/search repositories. Hardened ordering so explicit full imports run first against the shared request budget. |
| 7. Windows directory Sync | Fixed at the OS boundary: Windows skips unsupported directory Sync; file Sync remains before replacement. Unix post-replacement errors explain that state is already replaced. Native Windows baseline/append/readback test added and cross-compiled, **not run**. NTFS atomicity/crash durability unverified. |
| 8. UTF-8 evidence | Fixed: 240-byte prefix ends at a rune boundary. 2-, 3- and 4-byte boundary cases round-trip through JSON without introduced replacement characters. |
| 9. Repeated reference features | `NewMatcher` computes reference line sets once per audit. Existing exact/modified/ambiguous/unknown tests and repeated correlation evidence regression pass. No global mutable cache or new dependency. |
| 10. Repeated path logic / Stat | The cited CLI containment copy and repeated device Stat were already removed. Remaining scope/state lexical containment now shares `audit.Within`; boundary tests added. Symlink and same-file checks remain separate because they serve different purposes. |

The report/state JSON shape and schema IDs remain unchanged; `not_in_baseline` is an additive string-valued lineage status/diagnostic. The scanner evaluation changes to `SA001/v2;sh/v3.10.0;scope/v2`; its rule version remains v2. Tests ensure earlier scope/v1 evaluations cannot establish `new` findings.

## Final checks for this review

Host: macOS arm64, Go 1.23.5. Final Go commands used `GOCACHE=/private/tmp/skill-audit-gocache GOPROXY=off GOSUMDB=off` and existing vendored dependencies. The initial sandboxed suite could not bind loopback ports; the final test run was explicitly approved for its local fake HTTP servers. No live service calls, external scanner installation or skill execution occurred.

`gofmt -l cmd internal` produced no output. `go vet ./...` exited 0. `go test -count=1 ./...` exited 0:

```text
ok  skill-audit/cmd/skill-audit    0.959s
ok  skill-audit/internal/audit    6.515s
ok  skill-audit/internal/workflow 0.823s
```

- `go build -o /private/tmp/skill-audit-review-10 ./cmd/skill-audit`: exit 0.
- `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /private/tmp/skill-audit-review-10-linux ./cmd/skill-audit`: exit 0.
- `CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o /private/tmp/skill-audit-review-10-windows.exe ./cmd/skill-audit`: exit 0.
- `CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go test -c -o /private/tmp/skill-audit-review-10-audit.test.exe ./internal/audit`: exit 0; compilation only.
- `sh scripts/demo.sh`: exit 0; its finding-producing audit returned the expected exit 1.
- Nine additional compiled-CLI invocations: four command helps, fixture inventory with HOME unset, exact baseline, refreshed inventory, ordinary audit and appended snapshot. Verified CRLF/argument shebang detection, Unicode evidence, original package `changed`/finding `new`, added package `not_in_baseline`/finding `not_compared`, unchanged state bytes after ordinary audit and two preserved observations. Evidence: `/private/tmp/skill-audit-review-10-smoke-0y3jxg69/{commands,report}.json`.

Native Linux/Windows runtime, real mount alias traversal, crash interruption and real Skill X-Ray integration remain unverified. This patch does not promise general mount deduplication or universal discovery; Data-only macOS content may be excluded and old duplicate identities remain in existing baselines.

## Earlier review record

# Verification — targeted review fixes, 2026-10-28

The pasted review's F1–F3 were reproduced with failing regressions before code changes. The active state was replaced by a report, stderr-redirection and longer-pipeline cases produced no finding, and bash -- -s produced a false positive. The changes address these cases without adding enterprise features or executing fixture scripts.

## Final tests

All Go commands used `GOCACHE=/private/tmp/skill-audit-gocache GOPROXY=off GOSUMDB=off` with vendored dependencies. `go test -count=1 ./...` exited 0:

```text
ok  skill-audit/cmd/skill-audit    0.561s
ok  skill-audit/internal/audit    5.288s
ok  skill-audit/internal/workflow 0.942s
```

The fake GitHub/registry tests required permission only for loopback listeners; there were no live service calls. Focused regressions also passed before the full run.

F1 tests preserve exact state bytes and both observations for ordinary audit and explicit record/output collisions. Other tests cover inventory/catalog/saved-search input collisions, state/input collisions, a nonexistent baseline destination, relative and normalized paths, hardlinks, symlinks and case aliases. Validation occurs before work and is repeated before writes. Case-only paths are conservatively rejected even on case-sensitive filesystems. This is not protection from hostile concurrent filesystem mutation.

F2/F3 tests include adjacent pipeline stages, stderr redirection and duplication, unaffected receiver stdout, redirects that sever the pipe, descriptor moves outside scope, no duplicate findings in longer pipelines, precise evidence location and interpreter option order. SA001's version is now 2, with a regression proving that v1 evaluations cannot establish new v2 findings. Existing state remains readable.

## Builds and fixture smoke

- `gofmt -l cmd internal`: exit 0; no output.
- `go vet ./...`: exit 0; no output.
- `go build -o /private/tmp/skill-audit-targeted ./cmd/skill-audit`: exit 0.
- `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /private/tmp/skill-audit-targeted-linux ./cmd/skill-audit`: exit 0.
- `CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o /private/tmp/skill-audit-targeted-windows.exe ./cmd/skill-audit`: exit 0.
- `sh scripts/demo.sh`: exit 0. Global/project Claude fixtures, exact baseline, bundled modification, possible modified upstream, comparable new SA001/v2 finding and appended snapshot passed. The inner audit returned expected exit 1.

Seventeen separate built-CLI invocations also passed, covering all reviewed shell forms plus redirects severing the connection, baseline/snapshot creation and output collisions. The selected state's SHA-256 and two observations remained unchanged after rejected ordinary and snapshot audits. Source fixture strings were never executed. Local evidence: `/private/tmp/skill-audit-fix-checks-fqf3i8jc/commands.json` and `results.json`.

## Delivery and remaining limits

The Go implementation still exists as working files rather than in Git HEAD. A Git-only submission of HEAD is not runnable. The local source bundle includes the intended untracked code, tests, module files, vendored dependencies/licenses, pinned catalog/attribution, demo and documentation, with a SHA-256 file manifest. Packaging is not publication or submission; no Git history is changed.

Real Skill X-Ray, authenticated public search/registry integration, native Linux/Windows execution, crash durability and exhaustive device coverage remain unverified. Cross-compilation is compilation evidence only. SA001 still excludes descriptor moves/variable descriptors, direct |& connections, expansions, command resolution and general dataflow. Its completion status applies to this bounded rule, not comprehensive shell safety.

Source bundle verification: the extracted tree at `/private/tmp/skill-audit-delivery-oo_klmg5/skill-audit` built successfully with offline Go settings and passed `sh scripts/demo.sh`. All 75 source-file hashes matched the bundle manifest. The verification documentation was then updated and the archive regenerated; compiled source bytes did not change. The bundle is `/private/tmp/skill-audit-reviewed-source.tar.gz` and contains `SOURCE_MANIFEST.json`.
