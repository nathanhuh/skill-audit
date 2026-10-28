# Core walkthrough and engineering decisions

The assignment is four things: discover Claude skills, compare with two pinned upstream packages, preserve observed lineage, and report one explainable security pattern. The normal CLI path now needs only inventory and audit; recording a baseline or snapshot is an explicit audit option. Public acquisition and Skill X-Ray remain optional.

## 1. Discover packages, then inspect their contents

Read `internal/workflow/inventory.go`, followed by `internal/audit/discovery.go`.

`Discover` takes a context, filesystem roots, a home hint and traversal limits. It returns package identities, hashes and coverage diagnostics. A regular `SKILL.md` identifies a package. Scope and path keep same-named global and project packages distinct; names are metadata, not keys. The home argument affects classification; search roots determine what is actually traversed.

`Inspect` takes one package path and reads all supported regular bundled files as data. It returns hashes and temporary bytes, plus a completeness flag. It never follows links or executes scripts. A malformed manifest, oversized file, inaccessible entry or cancellation makes the observation incomplete. Filesystem presence is not evidence of current Claude loading.

**Why separate them?** Finding a package and determining its complete contents have different failure modes. The same inspection logic serves recursive discovery and the later audit re-read. Two passes cost extra I/O, but the audit can detect a package changed after inventory; saved hashes alone cannot supply current scanner evidence.

**Tradeoffs:** traversal is lexical and bounded; package limits are 1 MiB/file, 16 MiB/package, 1,000 entries. Device search has additional entry/depth/package limits. `WalkDir` can allocate a directory listing before a callback checks the budget. Cancellation is cooperative between traversal steps, not a hard interruption of an OS filesystem call. The no-symlink checks do not eliminate hostile filesystem races. Native Windows link/path semantics still need runtime tests.

## 2. Fingerprint bytes, then correlate evidence

Read `internal/audit/model.go`, `catalog.go` and `matching.go` in that order.

Every file gets SHA-256 over its raw bytes. A directory digest hashes a domain marker and sorted records of path byte length, path bytes and file hash. Sorting removes enumeration-order dependence; length encoding prevents ambiguous concatenation. File permissions, timestamps and excluded VCS metadata are outside this content identity. CRLF and LF intentionally produce different exact hashes.

An exact match requires complete inspection and the same directory digest. This includes bundled binaries and scripts. The catalog loads two complete pinned Anthropic packages and verifies their bytes against the committed manifest before use.

If exact matching fails, `Matcher.Correlate` uses path-qualified, trimmed text lines of at least 20 bytes. The score is intersection divided by union (Jaccard): three common lines among five distinct lines gives 0.60. A candidate needs at least three common lines and score 0.60. Names, frontmatter, license text and binary content do not influence this score. Multiple qualifying candidates remain ambiguous.

**Why this method?** It is small, deterministic, inspectable and easy to test. The thresholds are transparent heuristics, not empirically calibrated probabilities. A score of 0.90 does not mean 90% certainty of ancestry. Renamed files and large rewrites can be missed; shared templates can create candidates. A binary-only edit may leave text similarity at 1.0 while exact matching correctly fails.

`NewMatcher` prepares reference line sets once per audit, while each local skill gets its own feature set. It adds bounded reference-index memory, avoids repeated text parsing, and has no global cache to invalidate.

**Refactor:** matching now has its own file, independent of embedded catalog loading. Catalog evaluation identity hashes sorted references and content fingerprints, so a changed acquisition message does not look like a different set of comparison inputs. Separate licenses/diagnostics remain stored, but that identity is not a signature of the entire storage artifact. Older observations keep their original catalog IDs and may show a one-time catalog-change note.

## 3. Track observations, not invented history

Read `internal/audit/state.go`.

`SaveObservation` creates an immutable first observation or explicitly appends another observation. A normal audit never saves state. `Compare` compares the current complete file manifests against the first observation, reporting added, removed and modified paths independently of current upstream correlation.

The relationship basis matters: an initial exact match establishes what bytes were observed at that time; an initially modified candidate supports only inferred ancestry. Neither establishes who copied the skill. A missing package remains missing-or-undiscovered, with no automatic rename/move inference or fabricated file deletions.

**Why JSON rather than Git or a database?** The assignment needs fingerprints and observations, not branches, merges or restoration of historical contents. An exclusive writer lock, temporary file, file sync and replacement protect ordinary writes. Previous observations are preserved. State is capped at 32 MiB; stale locks require inspection. This is not tamper-proof provenance or a guarantee of Windows crash durability.

Only a finding absent from a comparable completed baseline evaluation receives `new`. A scanner/rule version mismatch yields a weaker label such as `present_in_changed_file`. SA001 v2 is not comparable with a v1 baseline because its recognized syntax changed. The current scope/v2 evaluation also differs from scope/v1: interpreter eligibility and evidence handling changed. The comparison remains baseline-relative, not an enterprise alert lifecycle. The optional scanner never receives `new` based only on its version string.

## 4. Parse one narrow risky shell pattern

Read `internal/audit/local.go` and the table-driven cases in `audit_test.go`.

The rule parses `.sh`/`.bash` into a shell syntax tree, then checks adjacent pipeline stages: literal HTTP(S) curl/wget stdout into literal sh/bash. It requires recognized options, including wget stdout behavior and interpreter option order. The stages may be part of a longer pipeline; unrelated redirections may remain, but redirections replacing the relevant stdout/stdin connection prevent a finding. Descriptor moves, variable descriptors and direct |& connections remain outside scope. The parser prevents comments, echoed examples and heredoc bodies from becoming commands merely because their text looks dangerous. The pattern is static evidence; nothing is downloaded or executed.

**Why a dependency here?** Correct shell syntax is too complex for a small regular expression. The pinned parser supplies syntax; our implementation supplies the rule, evidence, coverage and reporting. The YAML dependency has the same justification for metadata parsing.

**Limitations:** no general dataflow, variable expansion, command resolution, alias/function analysis, reachability analysis, Markdown instructions or PowerShell detection. The severity describes a risky execution mechanism, not malicious intent. Unsupported or failed parsing is visible coverage information. Adding another rule should be another explicit Go function and tests, not a rule-language framework.

**Correction:** a local evaluation now counts only findings it produced. A finding already supplied by another scanner cannot make the local rule claim it found something.

## 5. Orchestrate without retaining all local content

Read `internal/workflow/audit.go`, then `cmd/skill-audit/main.go`.

Audit validates all saved package identities first, sorts a copy for reproducible ordering, re-reads each package, detects inventory drift, correlates and scans it, then drops its raw byte map. Hashes and evidence remain in the report. Earlier findings survive a later cancellation, and the report records incomplete coverage.

**Why drop bytes?** Reports and lineage need hashes and evidence, not every local file. Retaining up to 16 MiB per package would make memory grow with the sum of all scanned package contents. Catalog data and accumulated report metadata still consume memory; this change is not a constant-memory guarantee.

Command-specific flags reject misleading combinations instead of silently ignoring them. File-role validation rejects aliases between output/state writers and selected inputs before evaluation and again before writes, preserving the active lineage file. The network client is constructed only in search/sync. The built-in catalog is the offline default; an invalid explicit catalog fails rather than falling back to an unrelated reference set.

Untrusted candidate reference IDs are quoted in terminal output; control characters must remain evidence, not terminal instructions.

Exit codes separate useful outcomes: 0 means no findings or operational gaps within selected coverage, 1 means findings, and 2 means an operational/incomplete result, possibly also containing findings. Zero is not a universal safety verdict.

## 6. Keep optional acquisition and scanning separate

`internal/workflow/github.go`, `registry.go` and `storage.go` handle bounded public-reference acquisition and its external catalog. Search produces leads; immutable commit resolution and full-byte comparison produce correlation evidence. A matching name alone is never provenance. These modules are outside the smallest assignment demonstration.

`internal/audit/xray.go` handles the pinned Skill X-Ray interface using argument arrays, cancellation, bounded output, staged inventoried bytes, strict report parsing and preserved scanner attribution. This trusted executable is not sandboxed by our adapter. Its runtime remains unverified here; fake executable tests exercise failure and coverage states. The original SA001 works when it is disabled or fails.

## Demonstrate the four requirements

Run `sh scripts/demo.sh`. It creates a temporary global Claude package and project Claude package, inventories them against the built-in catalog, records an exact baseline, adds a risky line to a bundled script as data, refreshes inventory and appends a snapshot. The expected report connects:

1. Global/project identity and absolute fixture path.
2. Exact or possible-modified upstream evidence at the pinned commit.
3. The script modified since the observed baseline.
4. SA001 with path, line, evidence and a comparable `new` label.

The final audit intentionally returns 1; the demo treats that expected finding as success. Temporary fixtures are removed. No real installed skills, public network, or script execution are involved.

## Follow-up correctness review

The macOS Data mount is a documented exclusion to avoid walking a known alternate view twice; this is intentionally simpler than tracking arbitrary mount identities and does not claim full-device completeness. New current identities get `not_in_baseline` instead of disappearing from lineage; the wording distinguishes an observation from a historical installation claim. Windows skips the unsupported directory sync after replacement but still flushes the file. Native filesystem and crash behavior need runtime validation.

`Within` is the shared lexical containment helper for scope and state placement. It does not replace symlink checks or file-identity checks: those answer different questions. Explicit repository imports precede search hits and retain full scope, subject to visible acquisition bounds. The missing-home fatal path and old repository-count truncation are absent from the sole current CLI; regression tests protect their replacements.

The priority walker in `internal/workflow/walk.go` exists because `filepath.WalkDir` cannot change sibling order: a lexical walk can consume a device budget in system trees before user directories. It keeps the same callback boundary for exclusions, errors, limits and cancellation. Directory-only deduplication prevents overlapping selected roots from charging twice; removing it outright would regress that behavior. The root's `Lstat` metadata is reused. This is a deliberate small traversal seam, not a filesystem framework. All ordering tests use temporary synthetic trees.

Cancellation and deadline diagnostics now identify both the interrupted root and roots not started, preserving prior findings. This differs from budget exhaustion and does not claim an interrupted scan is complete. Known names guide ordering; they do not establish trust or activation. Per-directory listings and very large homes remain memory/coverage limitations.

## Boundary fixes and reuse

Mandatory command inputs are checked before network transmission. Cancellation prevents state recording, with a second check before atomic replacement. A complete audit can establish a baseline even when findings exist; incomplete first observations are refused so an immutable baseline does not permanently start with unusable coverage. Partial reports remain inspectable, and non-interrupted partial snapshots remain possible.

Scope now follows the nearest agent marker and recognized home installation trees. A `.claude` project nested in `.codex/worktrees` therefore stays project scoped. `.github` is always project scoped. Scope corrections do not rewrite earlier identities, so old inventories should be refreshed and existing baseline identity changes reviewed explicitly.

Explicit roots run deepest first; marking only admitted directories avoids silently losing a root that a broader walk excluded or cut by depth. Root-local depth and dependency exclusions can be overridden by a narrower explicit root; symlinks and protected OS trees remain excluded.

One sync client pins a repository/ref once and shares bounded blob content across its targets. Validation of mode/type/declared size happens before cache lookup, because a SHA identifies bytes, not whether a tree entry is a symlink or regular file. One X-Ray session similarly caches a version probe, while per-package staging/report directories preserve isolation. These caches last only for their operation, without a persistent invalidation protocol.

`WriteAtomic` hides temporary-file creation, flushing, cancellation before rename, replacement and OS-specific directory syncing. State still owns locking and immutable-baseline policy; catalog/report output owns its own size and placement checks. Hash and metadata-name helpers are shared without treating discovery dependency exclusions as fingerprint exclusions.

## Coverage and resource policy refinement

`HasCoverageGaps` checks report diagnostics, package completeness and scanner status in one place. Inventory policy notes remain visible without blocking baselines; skipped package content still blocks them. Persistence failures have explicit stderr plus exit 3, preserving automation's distinction from a coverage-only exit 2. Previous exit contracts should be updated when integrating scripts.

Root priority precedes component depth, and explicit subtrees are delegated to their own traversal. This preserves independent root scope without a depth-first race against the home directory or a per-entry visited map. Global scope covers community installation paths inside home agent trees; known project containers and nested agent markers remain project scoped.

Repository failures and compact pinned references last for the invocation. A single active content cache limits retained tree/blob data; returning to an evicted repository can spend requests and byte budget again. Catalog output still retains imported content. Scanner startup has a separate bounded budget and a single probe-process retry, followed by fresh package scan budgets. Read/write boundaries enforce no-follow checks; state additionally checks before creating parents/locks. `OutputOutside` only determines placement and uses neutral error text.
