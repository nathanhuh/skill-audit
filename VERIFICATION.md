# Verification — behavior-preserving cleanup and upstream file comparison, 2026-10-29

Started on clean canonical `main` at `cc795d6`. Two changes, both uncommitted:

1. **Behavior-preserving cleanup.** One `anyIncomplete` helper replaces five repeated diagnostic loops. `Compare` computes skill-audit comparability once per skill instead of once per finding, and looks prior findings up in a set. One shared `diffFiles` helper produces baseline lineage lists (sorted, nil when empty, the same as before). An unreachable search-schema recheck in `cmd/skill-audit` was removed, since `Search.UnmarshalJSON` already refuses other schemas. Before the cleanup the suite had 192 passing tests and 3 skips; after it, the same 192 passed with no test changes.
2. **ADR-0014 upstream file comparison** (accepted, U1–U4). Inferred candidates carry `upstream_comparison` and `upstream_changes` (added/removed/modified relative to the pinned reference). Incomplete measurements report `unavailable`. Exact candidates, correlation, lineage, completeness and exit codes are unchanged. The new fields are additive and optional on report/v2 (ADR-0008 rule 2).

Checks (offline, `GOPROXY=off`, vendored): `gofmt -l cmd internal` prints nothing; `go vet ./...` exits 0; `go test -count=1 ./...` gives **196 passing, 3 skipped** (the same three skips as before); `scripts/verify-scenarios.py` maps 91 accepted scenarios (87 + U1–U4), with D7/P3/P4 still native-Windows-only; `sh scripts/demo.sh` exits 0. A compiled-CLI fixture copied `brand-guidelines` as `my-brand`, edited SKILL.md and audited with no baseline. Result: `correlation=possible_modified`, `lineage=not_compared`, `upstream_modified="SKILL.md"`, exit 0; JSON `upstream_comparison=compared`.

Not verified: native Linux and Windows, and the real Skill X-Ray, as before.

# Verification — host filenames and accepted scenario tests, 2026-10-29

Focused user request: fix the unreadable-baseline filename bug and ensure every accepted scenario ID has a test. Started on clean canonical main at **70316378c3407b62839d9c13c2146ef3818662dd**. Verified implementation: **ce92175af520df9c8fb80e82195aadc81e5df5ce**. The following documentation commit changes no production Go bytes. ADR-0009 P5 records the chosen host-local path rule; other fifth-review behavior changes were not made.

Default offline suite: **192 passing tests/subtests, no failures, three explicit skips** (case-sensitive O1; byte-invalid Unix filename rejected by this filesystem; opt-in D8 large fixture). D8 was then run with 50,000 metadata exclusions: **pass, 20.71 seconds**, one bounded summary, three examples and 49,997 omitted examples. Combining these runs verifies the runtime mappings for all available host tests. Native Windows D7/P3/P4 tests exist and compile, but were not run. P3 is a regression for the known direct-child-only cleanup gap and is expected to fail until Job Objects are implemented. P1 still requires another native host.

All **87 accepted IDs** have real Go test references. The [JSON map](docs/adr/scenario-tests.json), [dated source/evidence table](docs/adr/scenario-tests.md), normal-suite declaration guard and runtime-event checker distinguish present tests, executed assertions, skips and native-only requirements. Missing mappings, missing declarations or missing applicable runtime events fail the relevant check. Existing amendment IDs D13/R4/Q4 now have formal scenario-table rows without changing policy.

The ten-command compiled CLI smoke builds 7031637 from a temporary export, reproduces recorded/unreadable colon/backslash state, then verifies recovery without a write, readable snapshots and successful staging. New unit regressions first failed on both filenames. Host/Windows vet, scoped gofmt, native/Linux/Windows builds, Windows test compilation, fixture demo and six real published-format checks pass. No installed skill or skill script is executed; no real scanner, credential-dependent service or device sweep is used.

Commands use the original canonical checkout `/Users/nathanhuh/Documents/Documents - Nathan’s MacBook Pro/Interview_Repos/skill-audit`:

```sh
export GOPROXY=off GOSUMDB=off GOCACHE=/private/tmp/skill-audit-rewrite-gocache
gofmt -l cmd internal
go vet ./...
GOOS=windows GOARCH=amd64 go vet ./...
go test -json -count=1 ./... > /private/tmp/skill-audit-focused-tests.jsonl
SKILL_AUDIT_LARGE_FIXTURES=1 go test -json -count=1 ./internal/audit -run '^TestD8FiftyThousandMetadataExclusions$' > /private/tmp/skill-audit-large-scenario.jsonl
cat /private/tmp/skill-audit-focused-tests.jsonl /private/tmp/skill-audit-large-scenario.jsonl > /private/tmp/skill-audit-all-scenarios.jsonl
python3 scripts/verify-scenarios.py /private/tmp/skill-audit-all-scenarios.jsonl /private/tmp/skill-audit-scenarios-results.json
go build -o /private/tmp/skill-audit-host-paths ./cmd/skill-audit
go test -c -o /private/tmp/skill-audit-host-paths-helper.exe ./internal/audit
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /private/tmp/skill-audit-host-paths-linux ./cmd/skill-audit
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o /private/tmp/skill-audit-host-paths-windows.exe ./cmd/skill-audit
GOOS=windows GOARCH=amd64 go test -c -o /private/tmp/skill-audit-host-paths-windows.test.exe ./internal/audit
GOOS=windows GOARCH=amd64 go test -c -o /private/tmp/skill-audit-host-paths-windows-cli.test.exe ./cmd/skill-audit
host_paths_before_dir=$(mktemp -d /private/tmp/skill-audit-host-paths-before.XXXXXX)
git archive 70316378c3407b62839d9c13c2146ef3818662dd | tar -x -C "$host_paths_before_dir"
go -C "$host_paths_before_dir" build -o /private/tmp/skill-audit-host-paths-before ./cmd/skill-audit
python3 scripts/verify-host-paths.py /private/tmp/skill-audit-host-paths-before /private/tmp/skill-audit-host-paths /private/tmp/skill-audit-host-paths-helper.exe /private/tmp/skill-audit-host-paths-cli.json
sh scripts/demo.sh > /private/tmp/skill-audit-host-paths-demo.log
python3 scripts/verify-published.py /private/tmp/skill-audit-published /private/tmp/skill-audit-host-paths /private/tmp/skill-audit-host-paths-published.json
git diff --check
git diff --exit-code 7031637 -- go.mod go.sum vendor internal/audit/catalog .gitattributes
```

No schema/rule/evaluation identifier changes. Host-local paths apply to local manifests and decoded local finding paths; acquisition/catalog transport paths keep their existing portable validation. Raw bytes and directory/reference-catalog encodings are unchanged, as are dependencies, catalog bytes, attribution/licenses, .gitattributes and scanner/API pins. Published state remains comparison-readable and append-read-only; nothing is migrated, reset, pushed or submitted. Compact outcomes are in docs/host-paths-results.json.

Pending runtime evidence remains explicit: native Linux/Windows, a second host for P1, case-sensitive O1, native Windows junction/descendant/durability behavior, real Skill X-Ray/OpenGrep, live acquisition and exhaustive device discovery. Cross-compilation proves compilation only. The focused change does not settle SA001 per-file comparability or alter the other deferred review findings.

---

# Verification — fourth review on canonical main, 2026-10-29

Started on clean main at **1518aa555940d6e42b30aa52adde966954c0cd0e**. Verified implementation: **145b51e33a48fa1bfdbcbecb1eacfa72aaa44d40**. The following documentation commit changes no production Go bytes. ADR-0013 Q4 was explicitly approved before implementation. [Fourth-review record](docs/review-4-fixes.md) maps all ten items and [compact results](docs/review-4-results.json) record outcomes.

macOS arm64 / Go 1.23.5: **174 passing tests/subtests, zero failures, one skip** (TestO1DistinctCaseOnlyPathsNeedFilesystemIdentity requires a case-sensitive filesystem). Initial regressions failed for 1–5 and cleanup 8–10. The 13-command compiled before/after smoke reproduces five failures on the reviewed revision and confirms their fixes. Host/Windows vet, scoped gofmt, builds, fixture demo, prior smoke/compatibility checks and the restored combined-sync test pass.

All repository commands run from the original canonical checkout at `/Users/nathanhuh/Documents/Documents - Nathan’s MacBook Pro/Interview_Repos/skill-audit`. The baseline is built from a temporary export of 1518aa5. Nothing is installed, no live provider is called and skill scripts are never executed. The scanner is the portable Go test helper.

```sh
export GOPROXY=off GOSUMDB=off GOCACHE=/private/tmp/skill-audit-rewrite-gocache
gofmt -l cmd internal
go vet ./...
GOOS=windows GOARCH=amd64 go vet ./...
go test -json -count=1 ./... > /private/tmp/skill-audit-review4-tests.jsonl
go build -o /private/tmp/skill-audit-review4 ./cmd/skill-audit
go test -c -o /private/tmp/skill-audit-review4-helper.exe ./internal/audit
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /private/tmp/skill-audit-review4-linux ./cmd/skill-audit
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o /private/tmp/skill-audit-review4-windows.exe ./cmd/skill-audit
GOOS=windows GOARCH=amd64 go test -c -o /private/tmp/skill-audit-review4-windows.test.exe ./internal/audit
GOOS=windows GOARCH=amd64 go test -c -o /private/tmp/skill-audit-review4-windows-cli.test.exe ./cmd/skill-audit
review4_baseline_dir=$(mktemp -d /private/tmp/skill-audit-review4-baseline.XXXXXX)
git archive 1518aa555940d6e42b30aa52adde966954c0cd0e | tar -x -C "$review4_baseline_dir"
go -C "$review4_baseline_dir" build -o /private/tmp/skill-audit-review4-before ./cmd/skill-audit
sh scripts/demo.sh > /private/tmp/skill-audit-review4-demo.log
python3 scripts/review4-smoke.py /private/tmp/skill-audit-review4 /private/tmp/skill-audit-review4-helper.exe /private/tmp/skill-audit-review4-before /private/tmp/skill-audit-review4-cli.json
python3 scripts/review3-smoke.py /private/tmp/skill-audit-review4 /private/tmp/skill-audit-review4-helper.exe /private/tmp/skill-audit-review4-prior-cli.json
python3 scripts/review-smoke.py /private/tmp/skill-audit-review4 /private/tmp/skill-audit-review4-helper.exe /private/tmp/skill-audit-review4-smoke --large
python3 scripts/acceptance.py /private/tmp/skill-audit-frozen /private/tmp/skill-audit-review4 /private/tmp/skill-audit-review4-shared /private/tmp/skill-audit-review4-helper.exe
python3 scripts/verify-published.py /private/tmp/skill-audit-published /private/tmp/skill-audit-review4 /private/tmp/skill-audit-review4-published.json
git diff --check
git diff --exit-code 1518aa5 -- go.mod go.sum vendor internal/audit/catalog .gitattributes
```

The compiled smoke reports 13 commands, including five before/after failures, stable catalog identity, unchanged inventory/catalog input files and one newly stored observation timestamp. Two 200-duplicate-target tests preserve a later valid import under an injected 8,000-byte output budget. Per-query provenance uses fake HTTP transports for both providers, preserving successful, failed, invalid and empty queries. The direct sync --search saved-v2 --query fixture CLI path and saved-v3 sync both import the expected three references. Matching historical duplicate times remain readable; contradictory times fail explicitly.

Previous smoke checks pass 9 and 10 commands. The latter saves 66,010,508 bytes and 937 whole measurements under the production 64 MiB limit. The shared workflow still passes 17 commands per binary and 14 syntax samples. Six real published-format checks pass against the locally recorded binary from **3974a313ffdecd2a82d9e0b8699d9d61c47c9cc0**. The comparison reference remains **8c6f3b83d725c716246bdbdecc8495bb9cedbccb**. No files are migrated.

## Pre-publication freeze checklist (ADR-0008)

| Contract | Current / published | Evidence and disposition |
| --- | --- | --- |
| Report | v2 / v1 | Delivered complete/created_at remain; older v1 observations read without invented recording outcomes. v2 is unpublished. |
| Inventory | v3 / v2 | Both read; equivalent valid SHA-256 spellings normalize only in memory. v3 output/budget behavior is unchanged. |
| Discovery | v3 / none | New diagnostics expose existing root delegation; scope/exclusions are unchanged. v3 is unpublished. |
| State | v3 / v1 | New observations use observed_at alone. Older matching duplicate times retain historical fields; conflicting times are invalid. Published v1 stays comparison-readable and append-read-only; unpublished v2 stays rejected. |
| Search | v3 / v2 | Q4 explicitly accepts one batch per new query. Older grouped v3 and opaque-query v2 inputs stay intact. Unknown schemas fail. v3 is unpublished and changes in place. |
| Rule/evaluation | SA001 v3 / v2 | No rule/evaluation changes this round. Older evaluations cannot establish comparable new findings. |
| Fingerprints | directory/v1, reference-catalog/v1 / same | Encodings and raw content hashes are unchanged. Canonical valid lower-case input retains the same identity; upper-case catalog spelling reads as the same hash bytes. |
| Catalog/API/scanner | catalog/v1, catalog-store/v2; GitHub 2022-11-28; X-Ray c89eacf / same | Shapes and pins are unchanged; dependencies, catalog bytes, licenses and .gitattributes have no diff. |

This records the required checks before submission. No submission/archive or origin publication was authorized or performed; unpublished identifiers become frozen only at the ADR-0008 publication boundary. The pinned scanner source was inspected at a clean c89eacf checkout: exit 2 also covers preparation/ingest/write errors, so it cannot substitute for SARIF coverage.

Pending: native Linux/Windows execution, case-sensitive O1, native Windows junctions/descendant cleanup/NTFS durability, real Skill X-Ray/OpenGrep, live acquisition and exhaustive device discovery. Cross-compilation is not runtime validation. Ordinary audit remains read-only, and filesystem/process limits remain best effort. No roadmap monitoring, enforcement or cloud integration was added.

---

# Verification — third review on canonical main, 2026-10-29

Started on clean canonical `main` at **573eed67f416aa4c01f260baecc297cb59e08897**. Verified implementation: **77deb5b2eeceda97f5fa22614abf6f3c6e5acc7e**. The following documentation commit changes no production Go bytes. User-approved ADR-0012 R4 and ADR-0013 precede their implementation. [Third-review record](docs/review-3-fixes.md) maps all ten findings to the rules, fixes and source evidence; [machine results](docs/review-3-results.json) preserve outcomes.

Native macOS arm64 / Go 1.23.5: **157 passing tests/subtests, zero failures, one skip** (`TestO1DistinctCaseOnlyPathsNeedFilesystemIdentity`: this volume is case-insensitive). Initial fixture regressions failed for items 1–4, 7 and 8. Item 1's complete-SARIF/nonzero case is synthetic; source inspection disproves the claim that the pinned upstream exits 1 for security findings. Item 9's allegedly unused cache is reachable on transient tree failure, proven by a regression that passes before any code change. The lone-dash receiver regression also failed before the receiver correction. No live scanner or skill script was executed.

All commands run from `/Users/nathanhuh/Documents/Documents - Nathan’s MacBook Pro/Interview_Repos/skill-audit`. Scoped gofmt emits nothing. Host and Windows vet, offline tests, native build, fixture demo, Linux/Windows cross-builds and Windows test compilation pass. Native runtime validation is separate from compilation.

```sh
export GOPROXY=off GOSUMDB=off GOCACHE=/private/tmp/skill-audit-rewrite-gocache
gofmt -l cmd internal
go vet ./...
GOOS=windows GOARCH=amd64 go vet ./...
go test -json -count=1 ./... > /private/tmp/skill-audit-review3-tests.jsonl
go build -o /private/tmp/skill-audit-review3 ./cmd/skill-audit
go test -c -o /private/tmp/skill-audit-review3-helper.exe ./internal/audit
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /private/tmp/skill-audit-review3-linux ./cmd/skill-audit
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o /private/tmp/skill-audit-review3-windows.exe ./cmd/skill-audit
GOOS=windows GOARCH=amd64 go test -c -o /private/tmp/skill-audit-review3-windows.test.exe ./internal/audit
GOOS=windows GOARCH=amd64 go test -c -o /private/tmp/skill-audit-review3-windows-cli.test.exe ./cmd/skill-audit
sh scripts/demo.sh > /private/tmp/skill-audit-review3-demo.log
python3 scripts/review3-smoke.py /private/tmp/skill-audit-review3 /private/tmp/skill-audit-review3-helper.exe /private/tmp/skill-audit-review3-cli.json
python3 scripts/review-smoke.py /private/tmp/skill-audit-review3 /private/tmp/skill-audit-review3-helper.exe /private/tmp/skill-audit-review3-smoke --large
python3 scripts/acceptance.py /private/tmp/skill-audit-frozen /private/tmp/skill-audit-review3 /private/tmp/skill-audit-review3-shared /private/tmp/skill-audit-review3-helper.exe
python3 scripts/verify-published.py /private/tmp/skill-audit-published /private/tmp/skill-audit-review3 /private/tmp/skill-audit-review3-published.json
git diff --check
git diff --exit-code 573eed6 -- go.mod go.sum vendor internal/audit/catalog .gitattributes
```

The new compiled CLI smoke passes nine commands: normal nonzero scanner exits retain complete SARIF/security findings, reports expose complete/time, upper-case file/directory hashes compare unchanged without writing state, sh - yields SA001 v3, and missing packages keep discovery complete while overall coverage is partial. Search tests use fake HTTP transports: legacy-v2 plus live queries save v3 inputs and sync them without modifying the source. No provider credentials or live service is required.

The prior ten-command smoke still saves 66,010,508 bytes with 937 whole packages under the 64 MiB production bound. The shared harness passes 17 commands per binary and 14 syntax samples. Six real published-format checks pass against a locally recorded frozen binary from **3974a313ffdecd2a82d9e0b8699d9d61c47c9cc0**: inventory/v2, report/v1, state/v1 stay readable; legacy append is refused; finding exit priority and content comparison remain valid; published v2 evaluation cannot justify new v3 findings. Frozen reference binary remains **8c6f3b83d725c716246bdbdecc8495bb9cedbccb**. Older files are not migrated.

Freeze: search/v3 is the only new identifier. Search/v2 remains readable. Report/v2 adds overall complete/time and unpublished SA001 v3 adds accepted lone-dash forms in place. State/inventory versions, published directory/reference-catalog encodings, catalog-store shape, scanner/API pins, dependency files, vendored sources, catalog bytes, attribution/licenses and .gitattributes are unchanged. Local commits are not publication; nothing is pushed.

Pending: native Linux/Windows tests/demo, case-sensitive O1, native Windows junctions/descendant cleanup/NTFS durability, real Skill X-Ray/OpenGrep, live acquisition and exhaustive device discovery. Checks remain best effort against ordinary filesystem changes. No runtime monitoring, policy enforcement or cloud integration was added.

---

# Verification — second review and authorized main promotion, 2026-10-29

Review started on clean `codex/adr-rewrite` at `d2242e0d4f0078a84213053add0921b621cb0c2c`. Verified implementation: **a59cddbc4c6b17940a249579131e71c31db95e5c**. The user explicitly approved ADR-0003 D13 (explicit roots before device roots), ADR-0012 (expanded receiver flags and SA001 v3) and promotion to canonical main. The documentation commit contains no further production Go changes. Canonical main is fast-forwarded from the preserved reference **8c6f3b83d725c716246bdbdecc8495bb9cedbccb** to the commit containing this record. No history is rewritten and nothing is pushed/published.

[Second-review results](docs/review-2-fixes.md) map every item to its rule and evidence. [Machine summary](docs/review-2-results.json) preserves checks. Tests first reproduced items 1–5 and the missing lock guidance. Item 8 was reachable, not dead: a late search diagnostic can exhaust headroom. A regression proves this, and the finalizer is now one block. Metadata filtering applies to newly imported references; existing catalogs/state are not migrated. Dependency omissions still prevent exact local claims under D11.

Native macOS arm64, Go 1.23.5: **127 passing tests/subtests, no failures**, one skip (`TestO1DistinctCaseOnlyPathsNeedFilesystemIdentity`, case-sensitive volume required). Scoped gofmt has no output; vendor remains untouched. Vet, offline tests, native build, demo and all existing fixture CLI harnesses pass. Linux/Windows builds and Windows test compilation pass; native runtime remains unverified.

```sh
export GOCACHE=/private/tmp/skill-audit-rewrite-gocache GOPROXY=off GOSUMDB=off
gofmt -l cmd internal
go vet ./...
go test -json -count=1 ./... > /private/tmp/skill-audit-review2-tests.jsonl
go test -count=1 ./...
go build -o /private/tmp/skill-audit-review2 ./cmd/skill-audit
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /private/tmp/skill-audit-review2-linux ./cmd/skill-audit
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o /private/tmp/skill-audit-review2-windows.exe ./cmd/skill-audit
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go test -c -o /private/tmp/skill-audit-review2-windows.test.exe ./internal/audit
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go test -c -o /private/tmp/skill-audit-review2-windows-cli.test.exe ./cmd/skill-audit
go test -c -o /private/tmp/skill-audit-review2-helper.exe ./internal/audit
sh scripts/demo.sh
python3 scripts/review-smoke.py /private/tmp/skill-audit-review2 /private/tmp/skill-audit-review2-helper.exe /private/tmp/skill-audit-review2-smoke --large
python3 scripts/acceptance.py /private/tmp/skill-audit-frozen /private/tmp/skill-audit-review2 /private/tmp/skill-audit-review2-shared /private/tmp/skill-audit-review2-helper.exe
python3 scripts/verify-published.py /private/tmp/skill-audit-published /private/tmp/skill-audit-review2 /private/tmp/skill-audit-review2-published.json
git diff --check
git diff --exit-code d2242e0 -- go.mod go.sum vendor internal/audit/catalog .gitattributes
```

The 10-command prior-review smoke still passes, including 66,010,508 bytes saved with 937 whole measurements under the production 64 MiB limit. The shared 17-command-per-binary / 14-syntax-sample harness passes. Six real published-format checks pass; a finding compared against the published v2 evaluation receives present_in_changed_file rather than new under v3. V2 observations and content comparisons stay readable and unchanged.

Promotion uses the original checkout at `/Users/nathanhuh/Documents/Documents - Nathan’s MacBook Pro/Interview_Repos/skill-audit`, after verifying its branch, revision and clean status. `git -c core.hooksPath=/dev/null merge --ff-only codex/adr-rewrite` updates local main without merge rewriting or hooks. Final checks confirm clean main and matching branch heads; a native offline build/test check is run from canonical main. No setup scripts, installed skills, network service or real Skill X-Ray are involved.

SA001 rule/evaluation deliberately changes to **v3 / SA001/v3;sh/v3.10.0;scope/v2** under the accepted decision. No published fingerprint domain encoding, catalog/search shape or external scanner pin changes. Vendored dependencies, pinned catalog bytes, licenses/notices and .gitattributes remain unchanged. Local main promotion is not origin publication.

Remaining limits: native case-sensitive O1, Linux/Windows runtime suites/demo, Windows junction/descendant/NTFS durability, real Skill X-Ray, live acquisition and exhaustive device discovery remain pending. Filesystem races and blocking OS calls retain their documented best-effort limits. No roadmap monitoring, policy enforcement, cloud integration, move tracking or linked-skill following was added.

---

# Verification — ten-item rewrite review fixes, 2026-10-29

Reviewed starting revision: `7a84ccc0a4d2c910606e73239c3b343db96ed98c`, clean worktree on `codex/adr-rewrite`. Fixed implementation: **f9200ee07f9a0f071c788a11de7467903ea44930**. The accompanying documentation commit changes no production Go. Canonical main remains `8c6f3b83d725c716246bdbdecc8495bb9cedbccb` and is preserved.

All ten items are addressed, with [per-item changes and ADR citations](docs/review-fixes.md) and [machine evidence](docs/review-fixes-results.json). #1–#3 first failed new regressions. The user explicitly accepted ADR-0010 and ADR-0011 before #8–#9 behavior changed. No published schema/fingerprint/rule/scanner pin changed; new optional coverage fields use the still-unpublished inventory/v3/report/v2 contracts. Pinned assets/dependencies/licenses and .gitattributes have no diff.

Native macOS arm64 / Go 1.23.5: **103 passing tests/subtests, no failures, one skip** (`TestO1DistinctCaseOnlyPathsNeedFilesystemIdentity`, requires a case-sensitive volume). Hardlink and planned case-alias state protection pass here. Native Linux/Windows runtime is still pending; their builds and Windows test compilation pass. Cross-compilation is not native runtime or WSL evidence.

These exact checks passed offline with temporary fixtures, fake HTTP transports and a copied Go test executable. No installed skills, skill-script execution, live credentials/services, scanner installation or content-derived URL fetches were involved. gofmt is scoped to project Go code; pinned vendor is untouched.

```sh
export GOCACHE=/private/tmp/skill-audit-rewrite-gocache GOPROXY=off GOSUMDB=off
gofmt -l cmd internal
go vet ./...
go test -json -count=1 ./... > /private/tmp/skill-audit-review-tests.jsonl
go build -o /private/tmp/skill-audit-review-fixes ./cmd/skill-audit
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /private/tmp/skill-audit-review-linux ./cmd/skill-audit
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o /private/tmp/skill-audit-review-windows.exe ./cmd/skill-audit
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go test -c -o /private/tmp/skill-audit-review-windows-audit.test.exe ./internal/audit
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go test -c -o /private/tmp/skill-audit-review-windows-cli.test.exe ./cmd/skill-audit
go test -c -o /private/tmp/skill-audit-review-helper.exe ./internal/audit
sh scripts/demo.sh
python3 scripts/review-smoke.py /private/tmp/skill-audit-review-fixes /private/tmp/skill-audit-review-helper.exe /private/tmp/skill-audit-review-smoke --large
python3 scripts/acceptance.py /private/tmp/skill-audit-frozen /private/tmp/skill-audit-review-fixes /private/tmp/skill-audit-review-shared /private/tmp/skill-audit-review-helper.exe
python3 scripts/verify-published.py /private/tmp/skill-audit-published /private/tmp/skill-audit-review-fixes /private/tmp/skill-audit-review-published.json
go test ./internal/audit -run '^$' -bench '^BenchmarkStoreAdmission$' -benchtime=200ms -count=1
git diff --check
git diff --exit-code 7a84ccc -- go.mod go.sum vendor internal/audit/catalog .gitattributes
```

Frozen/published binaries use the temporary export commands in the preserved verification record below. New smoke checks passed 10 compiled CLI invocations, including actual macOS /tmp aliases, fake input interruption, per-package console overflow, unsafe TMPDIR, undeclared in-scope state, 3,000 skipped links and the actual 64 MiB output bound. The large fixture saved **66,010,508 bytes with 937 whole measurements from 1,000 candidates**, explicitly partial. The original shared harness still passes 17 commands per binary and 14 syntax samples; six real published-format CLI checks still pass without migration.

New incremental admission measurements for 8/16/32 entries: 0.710/1.390/2.781 ms, allocated bytes 600,840/1,197,930/2,398,791. The measured size-accounting path is approximately linear; it is not a before/after full-command performance claim. Every mixed-addition test matches actual formatted JSON size, and final writing retains the hard cap.

Remaining limits: native case-sensitive O1 runtime, Linux/Windows full suites/demo, Windows junctions/descendants/NTFS durability, real Skill X-Ray/OpenGrep, live acquisition and exhaustive device discovery remain unverified. Scope/coverage envelope overflow is explicitly unsavable under ADR-0010; ordinary useful inventories retain whole earlier measurements. State remains capped at 32 MiB independently. Filesystem races and OS calls remain best effort; no cloud integration, runtime monitoring, policy enforcement, move tracking, migration, publication or main promotion was added.

---

# Verification — isolated document-driven rewrite, 2026-10-29

Reviewed implementation: **b7257a55c479501179e9a936f0d7cfffea11e751**, local branch `codex/adr-rewrite`, `/Users/nathanhuh/.codex/worktrees/2376/skill-audit`. The following documentation/verification commit changes no production Go bytes. Preserved reference/canonical main: **8c6f3b83d725c716246bdbdecc8495bb9cedbccb**. The worktree began clean at that revision; canonical main was not promoted, edited, switched or rewritten.

See [docs/rewrite-comparison.md](docs/rewrite-comparison.md) for source-boundary history, scenario coverage and measured differences, and [docs/rewrite-results.json](docs/rewrite-results.json) for compact machine evidence. Prior verification records below describe the reference implementation at their named revisions; they are not claims about pending native runtimes.

## Environment and isolation

macOS 27.0.1 arm64, Go 1.23.5. Commands use `GOCACHE=/private/tmp/skill-audit-rewrite-gocache GOPROXY=off GOSUMDB=off` with existing vendored dependencies. All tests/demos use temporary fixtures, injected device roots and in-memory fake HTTP transports. No real device sweep, installed-skill mutation, credential-dependent test, live API, network listener, external scanner installation or skill-script execution occurred. The fake scanner is a copied/compiled Go test executable; supplied scanner code is still trusted, not sandboxed.

## Exact checks

All commands below exited 0. `gofmt -l` produced no output. Security-finding CLI checks inside the demo/harness intentionally return 1; gap/refusal/legacy checks assert their required 2/3 outcomes.

```sh
export GOCACHE=/private/tmp/skill-audit-rewrite-gocache GOPROXY=off GOSUMDB=off
gofmt -l cmd internal/audit
go vet ./...
go test -count=1 ./...
go build -o /private/tmp/skill-audit-rewrite ./cmd/skill-audit
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /private/tmp/skill-audit-rewrite-linux ./cmd/skill-audit
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o /private/tmp/skill-audit-rewrite-windows.exe ./cmd/skill-audit
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go test -c -o /private/tmp/skill-audit-rewrite-windows-audit.test.exe ./internal/audit
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go test -c -o /private/tmp/skill-audit-rewrite-windows-cli.test.exe ./cmd/skill-audit
go test -c -o /private/tmp/skill-audit-fake-scanner.exe ./internal/audit
sh scripts/demo.sh
python3 scripts/acceptance.py /private/tmp/skill-audit-frozen /private/tmp/skill-audit-rewrite /private/tmp/skill-audit-acceptance-results /private/tmp/skill-audit-fake-scanner.exe
python3 scripts/measure-fixtures.py /private/tmp/skill-audit-frozen /private/tmp/skill-audit-rewrite /private/tmp/skill-audit-fixture-resources.json
git diff --check
git diff --exit-code 8c6f3b83d725c716246bdbdecc8495bb9cedbccb -- .gitattributes go.mod go.sum vendor internal/audit/catalog
```

The frozen binary was built in a temporary export, using the same offline environment:

```sh
mkdir -p /private/tmp/skill-audit-rewrite-baseline
git archive 8c6f3b83d725c716246bdbdecc8495bb9cedbccb | tar -x -C /private/tmp/skill-audit-rewrite-baseline
(cd /private/tmp/skill-audit-rewrite-baseline && go build -o /private/tmp/skill-audit-frozen ./cmd/skill-audit)
```

The JSON test log `/private/tmp/skill-audit-rewrite-tests.jsonl` records 76 passing tests/subtests, no failed tests and no runtime skips. It was produced with `go test -json -count=1 ./...`; Windows-only code was excluded from the native run as intended.

Native tests passed in `skill-audit/cmd/skill-audit` and `skill-audit/internal/audit`. The shared harness passed 17 commands per binary, including an audit of a 14-sample shell corpus, matching hashes/catalog identity, scope, content changes, finding transitions and immutable observations. Expected differences include E9 text stdout and X2 stop-after-timeout (two baseline scan launches, one rewrite launch). D10 explicit dependency-root selection already passed in the reference. Full normalized commands/reports are in `/private/tmp/skill-audit-acceptance-results/`.

The resource fixture measured 100 packages / 900 files / 28,227,490 raw bytes, three samples per phase/binary. Baseline vs rewrite medians: inventory 107.305 vs 110.702 ms; audit 181.156 vs 195.124 ms. Maximum observed RSS is approximately 17.9–19.4 MB for both. These are small macOS fixture observations, not general performance or constant-memory claims. Direct-child `wait4` replaces an unavailable sandbox-restricted `/usr/bin/time -l` sysctl measurement.

The local remote-tracking `origin/main` revision was **3974a313ffdecd2a82d9e0b8699d9d61c47c9cc0**. Its published schema definitions, catalog fingerprint encoding and unchanged local rule were inspected without fetching. An additional temporary export built `/private/tmp/skill-audit-published`, then emitted real older-format fixtures for this check:

```sh
mkdir -p /private/tmp/skill-audit-published-reference
git archive 3974a313ffdecd2a82d9e0b8699d9d61c47c9cc0 | tar -x -C /private/tmp/skill-audit-published-reference
(cd /private/tmp/skill-audit-published-reference && go build -o /private/tmp/skill-audit-published ./cmd/skill-audit)
python3 scripts/verify-published.py /private/tmp/skill-audit-published /private/tmp/skill-audit-rewrite /private/tmp/skill-audit-published-compat.json
```

All six CLI invocations passed: real inventory/v2, report/v1 and state/v1 readback/comparison; refused legacy append without mutation; new finding plus failed requested append exits 1. This validates the pinned locally recorded publication, not live remote freshness.

## Publication freeze check (no publication performed)

| Contract | Result |
| --- | --- |
| `skill-audit/catalog/v1`, `catalog-store/v2`, `search/v2` | Published shapes/field meanings retained; external store and saved search round-trips tested. No bump. |
| `directory/v1`, `reference-catalog/v1` | Raw-byte domain encodings retained; exact pinned directory hashes and catalog ID match the frozen binary. No bump. |
| SA001 rule v2 / `SA001/v2;sh/v3.10.0;scope/v2` | Conservative eligibility/options and finding identity retained; shared syntax/status/evidence projections agree. No bump. |
| report/v1, state/v1, inventory/v2 readers | Published older evidence remains readable without migration or fabricated recording. Legacy state append refused. |
| report/v2, inventory/v3, discovery/v3, state/v3 | Unpublished identifiers retained under ADR-0008. Additions include explicit state-location/coverage evidence. Writers emit current formats. |
| state/v2 | Unpublished intermediate reader removed; unsupported schema diagnostic. |
| Catalog/dependencies/licenses/notices/checkout bytes | 45 pre-edit SHA-256 values match; git diff for pinned assets is empty. `.gitattributes` remains byte-preserving. |

Each kind of evidence is compared independently: policy change does not disable compatible complete content comparison; scanner evaluation change does not establish a new finding; catalog change keeps historical correlation separate.

## Remaining limitations and evidence levels

- Native Linux/Windows full tests, demos, permissions, process cancellation and persistence are **not run**. Cross-builds/test compilation only prove compilation. Windows-only `TestNativeWindowsStateReplacement` is compiled, not executed. WSL is not native Windows evidence.
- P1 cross-host fingerprint agreement, P3 Windows descendant cleanup/Job Objects, P4/D7 native junction behavior and NTFS replacement/crash durability remain pending. Windows skips directory fsync and makes no crash-durability promise.
- Real Skill X-Ray/OpenGrep, authenticated GitHub/registry integration, real device discovery, arbitrary mounts, OS Ctrl-C and crash/interrupted-write behavior remain unverified. The POSIX demo still needs a native Windows counterpart.
- X3/X4 use injected phase-deadline admission assertions, not actual 45 s scans or 6 s cold starts. Acquisition exhaustion tests use smaller injected limits while the production constants remain 40/64 MiB. D8 uses eight exclusion fixtures to exercise bounded examples; D4 alias keys are source checked. Distinct case-sensitive directories were not exercised on this case-insensitive volume.
- Filesystem operations can block beyond cooperative deadlines; checks do not prevent hostile concurrent mutation. Directory listing materialization, accumulated catalog/report content and JSON serialization are not constant-memory operations. No automatic stale-lock removal, state migration, move linkage or linked-skill traversal was added.
- Findings, coverage and recording remain independent. A stream failure can prevent delivery, but cannot undo a committed state write.

No push, publication, submission, contact, scanner download, service/database/UI or cloud integration occurred.

---

## Preserved earlier verification records

# Verification — committed comparison baseline, 2026-10-29

Prepared the canonical `main` checkout from parent revision `5fed934` as the frozen reference for a separate document-driven rewrite. All nine ADRs are accepted; their pending implementation gaps remain explicit. Corrected stale README wording that still called accepted ADRs proposed, and aligned the move-tracking roadmap's state-placement wording with ADR-0004. The sole Go-file change remains an ADR reference in a test comment.

Environment: macOS arm64, Go 1.23.5. Executed in the canonical checkout:

- `gofmt -w cmd/skill-audit/outcome_test.go`; `gofmt -l cmd internal`: no unformatted files.
- `git diff --check`: passed.
- `GOPROXY=off GOSUMDB=off go vet ./...`: passed.
- `GOPROXY=off GOSUMDB=off go test -count=1 ./...`: passed (CLI 2.797s, audit 6.712s, workflow 2.569s).
- `GOPROXY=off GOSUMDB=off go build -o /private/tmp/skill-audit-adr-baseline ./cmd/skill-audit`: passed.
- `GOPROXY=off GOSUMDB=off sh scripts/demo.sh`: passed using temporary global/project fixtures, baseline recording, a bundled-script byte edit, refresh, attributed SA001 finding and snapshot recording. The fixture script was inspected as data, never executed.

The baseline still has the documented ADR implementation gaps. These checks validate its existing behavior, not the newly accepted but unimplemented rules. No real installed skills, real Skill X-Ray, public network acquisition or native Linux/Windows runtime validation was used.

# Verification — interactive ADR review, 2026-10-29

Documentation review on canonical `main` at `5fed934`, including existing uncommitted changes. The user accepted all nine ADRs (0001–0009) during this review, with amendments and acceptance sources recorded in the ADRs. New accepted behavior, including default-device inventory, designated state exclusion, text recording summaries and scanner budget changes, is not implied to be implemented. ADR-0006 now specifies a best-effort 25% local reserve, configurable 30-second phase budgets, stop-after-timeout behavior, and one exit-75 probe retry sharing the original phase deadline; package scans are not automatically retried. ADR-0007 now requires complete-import admission under both a 40 MiB retained raw-content budget and the actual 64 MiB serialized-store bound, including required attribution and diagnostic headroom; prior complete imports survive budget exhaustion. These acquisition changes remain unimplemented. ADR-0008 accepts publication-based version freezing, published-format readers, no state migration, and removal of unpublished intermediate readers. It scopes compatibility to each kind of evidence and preserves ADR-0005 exit priority when legacy-state appends fail. Reader removal, compatibility verification and the pre-publication freeze checklist remain pending. ADR-0009 accepts isolated filesystem discovery/persistence and child-process boundaries. Cross-host fingerprint agreement remains pending validation with identical bytes and relative paths on another target platform; single-host macOS checks do not establish it. Native Linux/Windows runtime work remains pending.

This review changes documentation only. Checked edited text and `git diff --check`; no Go tests, build, demo or real external-scanner execution was performed for these decision updates. Earlier verification entries below describe prior work and do not validate the newly accepted behavior.

# Verification — ADR set restructure, 2026-10-29

Documentation-only change on canonical `main` at `5fed934`. The former single ADR-0001 was split into nine ADRs under `docs/adr/` with an index and process rules. Its accepted rules and scenario IDs (B1–B9, E1–E8, S1–S2, C1–C2) are unchanged. The former file moved to `docs/history/`, marked superseded. Dated review plans moved verbatim from DESIGN.md to `docs/history/design-review-notes.md`, and DESIGN.md keeps only the architecture sections. ADRs 0006 and 0008 contain recommendations that need a user decision. The gap tables come from source inspection of `5fed934`, not from new test runs. The only Go change is one test comment (`ADR-0001` to `ADR-0005`).

Checks: every relative Markdown link in `docs/`, AGENTS.md, README.md, DESIGN.md and PORTABILITY.md resolves; `git diff --check` exits 0; `gofmt -l cmd internal` prints nothing; `go vet ./...` and `go test -count=1 ./...` exit 0 (offline, `GOPROXY=off`).

# Verification — review corrections, 2026-10-29

Reviewed and edited the canonical original checkout on `main`, starting from clean `207a4f5d4d68810ff0e8d94c7fcf7210f35acd61`. These corrections are local, uncommitted changes. The detached checkout was not edited. Initial failing fixture regressions reproduced a hung version probe exhausting the operation deadline, repeated permanent launch failures, cancellation replacing a cached configuration error, explicit macOS aliases hidden behind policy exclusions, and speculative recording status in saved history.

Addressed review items #1–#4, #6 and #9; centralized exclusion classification also addresses #7–#8. Catalog acquisition/output budgeting (#5) and repository target grouping (#10) are deferred. Findings retain ADR exit priority; no total acquisition budget was added.

- Scanner phases use independent capped budgets, bounded further by a quarter of remaining operation time. Only probe exit 75 retries once. Real fake-executable tests hang both version and scan phases under a two-second operation deadline and still measure both packages and record the baseline. Injected launch errors are attempted once. Cached configuration diagnostics survive later cancellation. Overall cancellation still refuses recording.
- Injected directory-sync failures after baseline replacement and snapshot replacement return committed-write warnings; saved observations remain readable without speculative recording status. Actual CLI outcomes remain in report/v2 and requested-recording stderr. State/v3 derives anchors from its first observation; B1–B9 regressions retain immutable anchor eligibility. Valid state/v1 and state/v2 remain comparison-readable, reject append, and retain identical bytes. Corrupt legacy v2 anchors remain rejected.
- A temporary package under /private/tmp is discoverable; its explicit /tmp alias reports a symlink gap with no policy exclusion. Table tests cover /etc, /tmp and /var as explicit roots versus discovered aliases, and stable system/proc summary keys. Ordinary files avoid subtree classification; metadata and link exclusions remain covered by the existing suite.
- Default audit has no recording stderr; JSON retains not_requested. Requested recording is printed once on stderr. Failed persistence with findings retains exit 1, and actual save failures remain explicit.

Environment: macOS arm64, Go 1.23.5. Go commands used `GOPROXY=off GOSUMDB=off` and the normal host build cache; sandbox access to that cache and canonical checkout was explicitly approved. Tests used temporary fixtures and local HTTP handlers. No installed skills, external scanner installation, skill-script execution, public service calls, publishing, pushing or submission.

Executed checks:

- `gofmt -w cmd/skill-audit internal/audit internal/workflow`; `gofmt -l cmd internal`: no output. `git diff --check`: exit 0.
- `GOPROXY=off GOSUMDB=off go vet ./...`: exit 0.
- `GOPROXY=off GOSUMDB=off go test -count=1 ./...`: exit 0; cmd/skill-audit 2.489s, internal/audit 4.195s, internal/workflow 2.214s.
- `GOPROXY=off GOSUMDB=off go build -o /private/tmp/skill-audit-review ./cmd/skill-audit`: exit 0.
- `GOPROXY=off GOSUMDB=off CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /private/tmp/skill-audit-review-linux ./cmd/skill-audit`: exit 0.
- `GOPROXY=off GOSUMDB=off CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o /private/tmp/skill-audit-review-windows.exe ./cmd/skill-audit`: exit 0.
- `GOPROXY=off GOSUMDB=off sh scripts/demo.sh`: exit 0; temporary global/project baseline, bundled modification, comparable new SA001 and appended snapshot.
- `python3 /private/tmp/skill-audit-review-smoke.py`: exit 0 against the compiled CLI; quiet human/JSON ordinary audit, recorded baseline, state/v3 without redundant anchors or recording, byte-identical state after ordinary comparison and refused baseline replacement, new finding with snapshot exit 1, and two stored observations. The smoke harness and binaries are temporary verification artifacts; durable regression tests are in the repository.
- `git diff --exit-code HEAD -- go.mod go.sum vendor internal/audit/catalog`: no differences; dependencies, upstream bytes and attribution unchanged.

Targeted case-filesystem verification (run to expose skip details):

```text
=== RUN   TestCaseAliasedRootsDoNotDuplicatePackages
--- PASS: TestCaseAliasedRootsDoNotDuplicatePackages (0.01s)
=== RUN   TestDistinctCaseSensitiveRootsStayDistinct
    adr_exclusions_test.go:113: case-insensitive fixture filesystem; alias test covers merging
--- SKIP: TestDistinctCaseSensitiveRootsStayDistinct (0.00s)
PASS
ok  	skill-audit/internal/workflow	0.313s
```

Native Windows-only persistence tests were not run on this macOS host; the distinct-case test skips on this case-insensitive volume. Linux and native Windows runtime, real Skill X-Ray, and whole-device scans remain unverified. Cross-builds prove compilation only. Fake scanners still use /bin/sh; Windows process-tree cleanup and native helper adaptation remain pending. Scanner budgets cannot guarantee completion of slow local I/O, and actual operation expiry still prevents recording. State history cannot attest to its final directory-sync outcome; capture the final CLI report when that outcome is needed. Existing state/v1 or state/v2 needs an explicit new baseline path to adopt v3.

## Prior verification

# Verification — ADR step 4 and final alignment, 2026-10-29

Based on canonical main 64d3524, following outcome commit 71e3cee and anchor commit 4b5a86a. Routine exclusions now aggregate by rule/count, three examples and an explicit omitted count. A 1,000-.DS_Store fixture produces three total summaries (.DS_Store, .git, node_modules), not 1,002 per-entry diagnostics; the evidence survives state round-trip. Actionable skipped links retain paths and incomplete coverage. Explicit system-excluded roots remain reported. The obsolete policy_symlink rewrite is removed.

Filesystem-identity-confirmed case-alias root/overlap tests **ran and passed** on this case-insensitive macOS fixture volume; audits receive one package identity and traversal entry counts stay unduplicated. The separate distinct-case-directory test **skipped explicitly** because this volume cannot create that fixture. This is not Linux or native Windows runtime evidence. A first suite run caught a relative-home-hint regression in the new identity helper; resolving both paths absolutely fixed it, and its existing regression now passes.

Additional CLI regressions exercise actual E1/E2 save success/failure with findings, B3 scanner unavailability with a recorded content anchor, and failed report delivery after commitment. Recording diagnostics do not alter analysis coverage; their independent outcome controls persistence exits. Delivery failure cannot override a higher-priority finding or rewrite saved state as unsaved. Documentation now describes the implemented contracts and labels the ADR acceptance-time gap table as historical. Existing C1/C2 cache regressions still pass; acquisition policy was not changed.

Environment: macOS arm64, Go 1.23.5. All Go commands used GOCACHE=/private/tmp/skill-audit-gocache GOPROXY=off GOSUMDB=off. Only loopback fixture servers were allowed; no public service calls, installed skill modifications or skill-script execution occurred.

- gofmt -l cmd internal: no output; git diff --check: exit 0.
- go vet ./...: exit 0.
- go test -count=1 ./...: exit 0, final results:

```text
ok  skill-audit/cmd/skill-audit    0.919s
ok  skill-audit/internal/audit    6.280s
ok  skill-audit/internal/workflow 2.169s
```

- Native build: go build -o /private/tmp/skill-audit-adr-final ./cmd/skill-audit, exit 0.
- CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /private/tmp/skill-audit-adr-final-linux ./cmd/skill-audit: exit 0.
- CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o /private/tmp/skill-audit-adr-final-windows.exe ./cmd/skill-audit: exit 0.
- sh scripts/demo.sh: exit 0 against final source; exact upstream baseline, bundled modification, possible-modified upstream candidate, comparable new SA001, appended snapshot and expected finding exit 1.
- Separate compiled CLI smoke with temporary duplicate-name global-plugin/project packages, 50 metadata exclusions and a linked package: partial inventory exit 2; baseline recorded with two anchors and exit 2; changed bundled shell yields new SA001 and snapshot exit 1; repeat baseline retains exit 1 but reports not_recorded and preserves bytes; ordinary audit reports not_requested and leaves state unchanged; case-alias overlapping roots return one package. All assertions passed; fixtures cleaned. The initial smoke harness incorrectly supplied --json to inventory (already JSON by default); correcting that harness call passed, without a CLI change.
- git diff cb4be34 --exit-code -- go.mod go.sum vendor internal/audit/catalog: no differences. Pinned dependencies, upstream catalog bytes, hashes and licenses remain unchanged.

Current wire contracts: report/v2, inventory/v3, state/v2. Legacy inventory/v2 is readable with unknown policy metadata; historical state/v1 is readable for comparison but cannot be appended silently. Use a new explicit baseline path to adopt v2. Native Linux/Windows runtime (including distinct-case filesystem behavior), real Skill X-Ray and whole-device scans remain unverified. No publishing, pushing, submission or scanner installation occurred.

## Prior verification

# Verification — ADR step 3: installation ownership, 2026-10-29

Based on 4b5a86a. S1/S2 tests discover real temporary packages in global plugins (including nested .claude markers), global community layouts, direct global installs, managed worktrees and projects, plus .github project ownership. Duplicate names retain separate identities. Round-trip baseline comparison stays unchanged; an intentionally different historical scope remains missing alongside the current not_in_baseline identity, without rewriting history.

Offline macOS arm64 / Go 1.23.5: gofmt applied; go vet ./... exit 0; go test -count=1 ./... exit 0 (cmd 0.820s, audit 6.322s, workflow 1.457s). Native build /private/tmp/skill-audit-adr-ownership exit 0; fixture-only sh scripts/demo.sh exit 0. No network service, installed skill or script execution. Native Linux/Windows and real Skill X-Ray remain unverified.

## Prior verification

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
