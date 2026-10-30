# Code walkthrough

Read the entry points in order: `main.go` dispatches the commands, including the one-step `scan`, `inventory.go` and `content.go` find and measure packages, `catalog.go` correlates them, `local.go` holds SA001, and `observe.go` and `storage.go` compare and record lineage. The contract column names the [ADR scenarios](docs/adr/README.md) each file implements; [scenario-tests.json](docs/adr/scenario-tests.json) maps every scenario to its test.

## Entry points

| File | Responsibility | Main contracts |
| --- | --- | --- |
| `cmd/skill-audit/main.go` | Four command-specific flag sets, preflight, explicit network selection, delivery, exit priority and summary text output, one-command `scan` | T1/T5, E1–E9, K1–K4, L1–L4, N1–N5 |
| `internal/audit/inventory.go` | Injected device selection, deterministic traversal, bounded exclusion summaries, separate discovery coverage | D1–D13 |
| `internal/audit/content.go` | Scope/path identity, no-follow reads, raw-byte package measurement and YAML metadata | S1–S4, D11/D12, P2 |
| `internal/audit/catalog.go` | Verify embedded/external content, frozen fingerprints, prepare reference features once, retain every qualifying candidate | T1, V6 |
| `internal/audit/local.go` | SA001 v3 shell AST pattern, conservative options, capped evidence, full-match fingerprint and detection coverage | T2, V7, F1 |
| `internal/audit/observe.go` | Re-read saved identities, detect drift, correlate and scan one package at a time, compare independent evidence by full-match finding identity | B1–B9, V6/V7, F1–F3 |
| `internal/audit/storage.go` | Bounded readers, placement/alias protection, state lock, atomic commit and independent durability outcome | B6/B10/B11, E1–E8, V1–V5 |
| `internal/audit/wire.go` | Published JSON boundaries; preserve original stored observations when appending | V1–V7 |
| `internal/audit/scanner.go` | Pinned static-only Skill X-Ray adapter, staged bytes, phase deadline/reserve, cached terminal causes | T3/T4, X1–X10 |
| `internal/audit/search.go` | Per-query input provenance, computed views, flat merge, published v2 reads and v3 writes | Q1–Q4, V1–V7 |
| `internal/audit/acquire.go` | Explicit public search/sync, repository grouping, pinned API and commits, separate cache/validation/request/output budgets | C1–C8 |
| `internal/audit/fs_*`, `process_*` | OS-specific discovery/link/persistence and child-process handling | D7, P1–P4 |

## Evidence flow

`InventoryRun` finds packages and calls `Measure`; it retains hashes and coverage, then drops raw local bytes. `AuditRun` re-reads each saved scope/path identity and warns about inventory drift. Current bytes are correlated and scanned before their byte map is discarded. File contents are never executed, and URLs found in content are never fetched.

Discovery completeness, package completeness and scanner completion are separate. A routine directory exclusion does not certify absence of skills. A relevant omission within a package makes that measurement incomplete; `.git`/`.hg`/`.svn`/`.DS_Store` stay outside the declared fingerprint. The rewrite conservatively skips dependency subtrees inside a package and marks the package incomplete; the reference implementation measured such contents when within its package budgets. Both must avoid an exact claim over omitted bytes.

`Compare` uses complete first-observation measurements as immutable anchors. Discovery-policy changes affect discovery equivalence; scanner-evaluation changes affect finding transitions; catalog changes affect correlation evidence. None automatically disables independently valid package-content comparison. Missing packages remain missing or undiscovered, and a later complete measurement never replaces an unusable first anchor.

`Record` is the only lineage writer. A successful rename commits; a later directory-sync error reports `committed_durability_unconfirmed`. Final recording outcomes stay in the delivered report, outside stored observations. Requested recording appears once on text stdout and once on stderr. Findings win exit priority, followed by pre-commit recording failure, then operational/coverage/durability gaps: **1 > 3 > 2 > 0**.

`Acquirer.Sync` admits one complete package and required attribution at a time. Blob cache capacity is unrelated to package validation. Both raw retained content and the actual formatted output are checked before admission, including bounded diagnostic headroom. An exhausted budget leaves earlier complete imports intact.

## Verification and limits

Run the commands in VERIFICATION.md. The Go tests use temporary fixtures, in-memory fake HTTP transports and a copied Go test executable as the fake scanner. They need no installed skills, credentials, service, network listener or Skill X-Ray. `scripts/acceptance.py` runs both built CLIs on shared temporary fixtures; `scripts/measure-fixtures.py` records the small macOS fixture's direct-child resource usage using `wait4`.

The parsers, catalog packages, license files, notices, go.mod/go.sum and `.gitattributes` remain byte-for-byte pinned. No dependency or scanner was installed. Native Linux/Windows runtime, cross-host hashing, Windows descendant cleanup, real Skill X-Ray and exhaustive device discovery remain unverified. Cross-compilation proves compilation only; WSL does not validate native Windows.

Filesystem checks are best effort against ordinary changes, not protection from hostile concurrent mutation. Per-directory listings are materialized before ordering, filesystem calls are not forcibly interruptible, state locks need manual stale-lock inspection, and retained catalog/report serialization can consume more memory than raw-content limits. The scanner is trusted executable code, not sandboxed by this adapter.
