# Design: inventory, acquire references, audit

## Accepted contracts — 2026-10-29

[ADR-0001](docs/adr/0001-observation-and-outcome-contracts.md) freezes baseline eligibility, independent outcomes and exit priority, installation ownership, and cache/import semantics. [CONTEXT.md](CONTEXT.md) defines the vocabulary. The ADR supersedes conflicting policies in the historical review notes below; its acceptance matrix is the implementation target. Acceptance in cb4be34 was documentation-only. The runtime alignment sections below and VERIFICATION.md describe the implemented scenarios; the ADR retains its historical gap table.

## Historical design and review notes

The following sections preserve prior decisions and implementation history; conflicting policies are superseded by ADR-0001.


## Import/write-outcome review (2026-10-29)

Keep per-file (1 MiB), per-package (16 MiB), entry and operation request limits independent of the active blob cache's 16 MiB memory budget. Reloading an evicted repository must not consume a stale import ledger; uncached validated bytes may still be imported. Cache only permanent resolution errors (404, explicit private status, truncated/structurally invalid tree), leaving 403/5xx/transport failures retryable on later targets.

All skipped symlinks outside explicit routine metadata/dependency exclusions remain incomplete, including manager-installed skills; never follow them. Treat legacy inventory link notes as coverage gaps during audit. A successful rename is the write commit point. Return a typed durability warning after it, report state recorded with unconfirmed durability, and use exit 2 rather than claiming no record. Exit 3 is owned by the CLI's actual failed state phase, never by diagnostic names in input data.

Use the configured timeout for probe and scan phases within the operation context. An already canceled session must perform no further setup; the workflow already stops between packages. Remove the unused production one-package scanner wrapper in favor of the session API. Route inventory output placement through the shared writer once, passing observed package roots to preserve protection if a manifest disappears.

Acceptance: reproduce eviction accounting, monorepo loss, transient-error poisoning, skipped-skill-link coverage and forged persistence exit; inject a directory-sync failure to distinguish committed data from pre-rename failure; capture probe deadlines/cancellation deterministically, retain real fake-executable tests, then run offline format/vet/tests/builds/demo and CLI checks. Keep schemas, pinned content and rule versions intact.


## Coverage policy and bounded startup/cache review (2026-10-29)

Routine inventory exclusions are visible policy notes rather than operational gaps. Unreadable paths, limits, cancellation, rejected explicit roots and skipped content inside skill packages remain incomplete. One report-level coverage predicate drives exit 2 and baseline eligibility. Recording failures retain the report, write a clear stderr message and return distinct exit 3. No JSON shape changes; exit 3 extends the CLI contract.

Restore global scope throughout home agent trees, including community installation directories, while keeping known worktrees/projects containers and nearest nested agent markers project scoped. Root priority precedes component depth. An ancestor walk delegates explicitly selected subtrees to their own root, preserving root-local depth without starving the selected home; no per-entry visited map is needed. Protected OS trees and symlinks remain excluded.

Memoize repository resolution failures for the invocation. Retain compact pinned commit/tree references, but only one active repository's tree/blob cache (16 MiB blobs plus the bounded tree). Revisiting another repository reloads pinned content; accumulated catalog output still needs memory. Give X-Ray startup its own five-second probe budget per attempt, one retry for a probe process failure and a fresh per-package scan budget. Operation cancellation bounds all phases. Share neutral output placement checks; no-follow validation belongs at read/write boundaries.

Acceptance: reproduce realistic baseline, record-error, scope and home-starvation failures before fixes; cover root component depth/delegation, memoized failures and active-cache eviction, slow/retried scanner startup, package symlink gaps, neutral output errors, full offline suite/builds and fixture-only CLI demo. Keep native OS/scanner runtime limits explicit.


## Boundary and operation reuse review (2026-10-28)

Validate mandatory sync output before constructing queries or issuing requests. Refuse state recording after interruption and refuse creation of incomplete baselines (findings alone are allowed); still emit partial reports. Snapshots may preserve non-interrupted partial observations. Check cancellation again before atomic replacement. An unavailable baseline comparison explicitly labels findings `not_compared`.

Classify by the nearest recognized agent directory. Global scope requires a selected/conventional home with an immediate agent `skills` or `plugins` tree; `.github` remains project scope. Nested worktrees are project identities. Existing stored identities are not rewritten. Visit deeper explicit roots before ancestors, then use existing priority/lexical order. Mark directories visited only after depth/exclusion checks; always preserve hard no-follow/OS exclusions.

Within one sync client/operation, resolve each repository/ref once and reuse its validated tree and bounded blob cache. Validate every entry's mode/type/size before consulting cached content, and check cached length against declared size. The 16 MiB import-byte budget now spans targets in the same repository/ref. No persistent cache or network in audit. Share SHA-256, metadata exclusions and the atomic file-write primitive; preserve the distinction between traversal exclusions and package-fingerprint metadata exclusions.

Probe the pinned Skill X-Ray version once per audit session. Keep an audit-owned temporary root, isolated staging/report subdirectories for each package, per-scan timeout and cleanup. Reuse probe failures without retrying each package; local findings survive. No scanner interface/version changes, downloads or cloud modes.

Acceptance: offline failing regressions for request ordering, interrupted/partial baseline persistence, nested scope and `.github`, explicit roots, cache-hit symlinks/size mismatches and unavailable finding labels; request/probe-count fixtures, stale-SARIF isolation, atomic replacement and existing failure tests. Run format, vet, tests, builds and fixture demo from the canonical original `main` checkout. Preserve wire schemas and scanner rule versions.


## Traversal follow-up plan (2026-10-28)

The earlier Data-tree exclusion fixes a known duplicate view, not budget starvation before reaching user directories. Replace the standard lexical-only walk with a small deterministic priority walk: directories containing the selected home first, known agent directory names second, conventional `Users`/`home`/`root` containers third, then other entries; lexical order breaks ties. Apply the same priorities across selected roots, while keeping output root/package ordering stable. Prioritization improves useful partial results; a large home can still exhaust limits and no scan promises all device skills.

Keep overlapping-root deduplication: the current map is useful, but only directory paths need to be remembered. Reuse the root's no-follow file info in the walker rather than statting it again. Keep every existing exclusion, symlink, depth, entry, package and cancellation boundary. Directory listings are still materialized before sorting, as with filepath.WalkDir; entry limits do not bound the memory of one unusually wide directory.

Distinguish cancellation (`inventory_canceled`) from deadline (`inventory_timeout`) and entry/package exhaustion. Report each root not started after interruption with the actual cause; do not imply it was inspected. Keep exit 2 for operational gaps. No JSON shape/schema change; diagnostic codes are additive. Existing zero CLI limits are rejected, the retired `--public-requests` flag remains unsupported, and a zero internal request budget must make zero requests.

Acceptance: reproduce user-directory starvation and missing cancellation diagnostics in temporary fixtures before editing; verify deterministic priority, overlapping roots counted once, symlinks/depth/package limits, cancellation before and during traversal, pending roots and zero-limit rejection; run formatting, vet, offline tests, native/cross builds and fixture-only CLI smoke/demo. Windows remains source-inspected/cross-compiled, not runtime-verified.


## Correctness review plan (2026-10-28)

Address the current inventory/search/sync/audit implementation, rather than restoring the retired CLI. Exclude macOS's `/System/Volumes/Data` alternate tree with a visible coverage diagnostic (including explicit roots there); prefer the unified filesystem paths and do not claim arbitrary mount deduplication. Report current identities absent from the baseline as `not_in_baseline`, with no inferred installation time, file additions, baseline origin or `new` findings. This is an additive status in the existing string-valued report contract; earlier observations remain readable.

Parse the shebang's interpreter token and a small documented argument subset, accepting CRLF/trailing whitespace and ordinary sh/bash option flags. Use the pinned parser's existing CRLF handling and raw byte offsets; fingerprints remain raw. Truncate evidence at a UTF-8 boundary. Advance the scanner evaluation scope version so older coverage cannot establish new findings. Isolate directory sync behind OS-specific helpers; Windows skips unsupported directory fsync, with weaker documented durability, and Unix post-rename failures explicitly say the observation was already replaced.

Prepare reference line sets once per audit with a small matcher object. Centralize lexical path containment for state placement and scope classification. Verify missing-home override and explicit repository acquisition in offline regressions, and process explicitly named repositories before search hits to protect the shared request budget. No new dependencies or network-dependent tests.

Acceptance: fixture regression tests for each applicable finding, old-version comparison, incomplete first observations, Unicode JSON evidence and unchanged raw hashes; fake HTTP tests for explicit full imports alongside narrow search hints and limits; macOS exclusion policy tables; state append/readback tests and native-Windows-only regression; formatting, vet, full offline tests, builds and fixture demo. Cross-builds do not validate native Windows/Linux runtime behavior.


## Targeted review fixes (2026-10-28)

F1: validate selected file roles before any scan, network request or observation write. A report/catalog output must not alias a selected input or the active state; state recording must not alias an input or report output. Compare cleaned absolute paths and existing file identities, reject symlinks, and conservatively reject case-only aliases even on case-sensitive filesystems. Recheck before writes. This prevents accidental CLI collisions, not hostile filesystem races.

F2/F3: keep SA001 narrow but recognize adjacent simple stages in a longer pipeline and allow redirects that leave the downloader stdout / interpreter stdin connection intact. Reject redirects targeting that connection; handle numeric descriptor copies/closure conservatively and leave descriptor moves/variable descriptors outside scope. Honor option termination and explicit stdin mode in shell arguments. Bump the rule/evaluation version to v2 so old evaluations cannot justify a newly introduced finding label. Never execute fixture script content.

Acceptance: reproduce each review case through the CLI, preserve exact bytes and observation count for rejected path collisions, test normalized/aliased paths and all selected input types, test positives and negatives for redirects and argument order, test v1/v2 non-comparability, then run the offline suite, vet, builds and fixture demo. Build a source archive from an explicit allowlist including untracked source, pinned dependencies and licenses; verify the extracted copy builds. No commit, push or submission is implied by packaging.

The sole CLI is `cmd/skill-audit`, using the inventory/search/sync/audit workflow. `internal/workflow` owns discovery, acquisition and orchestration; `internal/audit` owns package inspection, fingerprints, matching, local rules, lineage and the optional Skill X-Ray adapter. Local evidence collection is separate from explicit network acquisition.

1. `inventory --device --out inventory.json` walks accessible filesystem roots for every SKILL.md package. On Unix device defaults to `/`; on Windows it enumerates existing drive roots. Repeated `--root` narrows/extends explicit searches. Deterministic traversal prioritizes home/user/agent paths with lexical tie-breaking and excludes virtual OS trees, symlinks, Git metadata and common dependency/build caches, with explicit diagnostics. Permission errors, deadlines, entry/depth/package limits and exclusions prevent universal completeness claims. Nested package manifests are found too. Scope/path and agent hints are observations, not activation proof.
2. `search --query 'web artifacts'` sends only the explicit query to GitHub's authenticated code search, with `filename:SKILL.md` added. No local content is transmitted. An explicit --inventory plus --search-names option sends deduplicated recorded names, capped at ten queries. Search coverage is bounded and results are only leads, not correlations.
3. `sync --repo owner/repo --ref HEAD --out catalog.json` checks that a repo is public, resolves the ref to an immutable commit, reads its Git tree and blobs, and imports bounded complete SKILL.md packages into a versioned local catalog. Git symlinks/submodules or truncated/oversized trees/packages are rejected, not counted as exact. Root licenses/notices are retained as attribution separately from package bytes. Search/sync are the only network commands. Local cache is not automatic redistribution permission.
4. `audit --inventory inventory.json [--catalog catalog.json]` re-reads inventory paths, detects content drift, correlates against the built-in pinned catalog or an explicit override and runs local scanning. Optional `--xray` keeps upstream scanner isolation. Optional `--state --record baseline|snapshot` reuses immutable-baseline lineage. Audit is offline except an explicitly supplied trusted external executable's own behavior; our adapter never enables its network/LLM modes.

The explicit acquisition step is more reviewable than an implicit internet search during every scan: local private data never becomes a query without the operator choosing it; results can be reproduced offline against saved commits. A user may search by a skill name, but full content comparison still decides whether it correlates. A renamed skill might need a different user query. GitHub index limits, auth/rate limits and incomplete results remain visible.

API contract: GitHub REST 2022-11-28, `application/vnd.github+json`, GET /search/code, /repos/{owner}/{repo}, /repos/{owner}/{repo}/commits/{ref}, /repos/{owner}/{repo}/git/trees/{tree}?recursive=1, /repos/{owner}/{repo}/git/blobs/{sha}. No shell, git clone, arbitrary download URL, scripts or redirects. Requests have timeout, response/file/total byte bounds, and a request budget. Credentials come only from GH_TOKEN/GITHUB_TOKEN and are never serialized. Default tests use httptest and fixtures. No provider/plugin framework.

Catalog storage contains actual bytes (base64 JSON), hashes, repo/commit/path and retained attribution. Optional external runtime catalogs remain outside scanned packages and can be copied to an offline VM. The pinned two-skill catalog is the default for offline audit, tests and the demo. State is JSON rather than a database. Desktop Windows remains compile-only until native path, state replacement and process cleanup testing; see PORTABILITY.md.

Configurable registry acquisition supports the documented Skills.sh v1 search response using --registry-url and a separate SKILL_AUDIT_REGISTRY_TOKEN. GitHub remains the default. Saved search output is consumable by sync; GitHub package paths or registry directory-slug hints select the package before full content comparison. This is not arbitrary-registry compatibility or registry-backed proof of origin.

## Consolidation acceptance (2026-10-28)

Remove the earlier scan/baseline/snapshot/catalog command implementation and its single-project traversal. Promote the current inventory/search/sync/audit implementation to the sole command named skill-audit. Keep wire schema identifiers unchanged: schema versions describe data contracts, not competing product editions. Consolidate README, design, verification and demo; retain required licenses and offline fixtures. Move core tests to explicit package fixtures and keep recursive-discovery tests at the workflow interface. Validate command removal, baseline persistence, partial failures and the complete fixture demo, then run formatting, vet, tests and native/cross builds. Product recommendations are proposals in PRODUCT.md, not silently implemented enterprise features.

## Inventory and fingerprints

Local identity is `(scope, absolute lexical package path)`, independent of declared name. Priority traversal with lexical tie-breaking and sorted root/package report arrays. Skip all symlinks (including path components), special files, and excluded `.git`, `.hg`, `.svn`, `.DS_Store` metadata. Report every skip; only intentional metadata exclusions leave the content fingerprint complete. Bound each file to 1 MiB, each package to 16 MiB and 1,000 entries. Inventory requires explicit roots or --device. Unavailable selected roots and I/O/metadata errors become diagnostics and exit 2 while independent packages continue.

YAML frontmatter is parsed as data with a pinned YAML parser. Optional string name defaults to directory name; description/name type errors, duplicate fields, invalid YAML, and unterminated frontmatter are diagnostics. This is not a full Claude schema validator.

Hash raw bytes with SHA-256. Directory digest v1 hashes a domain separator followed by sorted records: uint64 big-endian path byte length, UTF-8 slash-relative path bytes, and the 32-byte file hash. No timestamps, permissions, empty directories, state files, or excluded metadata. Incomplete packages have observed file hashes but no authoritative directory digest and cannot be exact matches.

## Catalog fixtures and matching

Retain two complete Apache-2.0 fixture packages, brand-guidelines and web-artifacts-builder, from anthropics/skills at dbd4588f9e1033efb41dad4bef2f7947c8993d44. Preserve licenses and third-party notices outside package comparison metadata. These pinned packages support offline audit, tests and the demo; audit uses these by default or an external catalog selected by the operator. Manifest records repository, commit, upstream path, per-file hashes, and directory digest; verify embedded files against it on load. Catalog evaluation identity hashes sorted reference metadata and per-file fingerprints with the skill-audit/reference-catalog/v1 domain marker; it excludes separate acquisition diagnostics and attribution packaging.

Exact means complete directory digest equality. Modified candidates use one method: Jaccard similarity of sets of nonblank trimmed content lines (minimum length 20 bytes), keyed by relative file path, excluding SKILL.md frontmatter and license/notice files. Only UTF-8 text without NUL bytes contributes. Threshold 0.60 with at least three shared lines. Names never contribute. Every qualifying candidate is retained; multiple exact or modified candidates are ambiguous, not arbitrarily ranked into certainty. Report shared/union counts and score. This detects small edits and bundled changes but misses major rewrites, cannot infer authorship, and can confuse shared templates. A match is content evidence, not proof of who copied it.

## Lineage

Versioned JSON manifest outside all scanned roots: immutable baseline plus append-only observations, UTC observation time, catalog identity, local identity, snapshot digest and hashes, scanner evaluations and findings, and upstream reference with basis (`observed_exact_match` or `inferred_candidate`; the state reader also accepts previously recorded `user_declared_origin`). Unknown or ambiguous origin stays unresolved. Each observation stores its own correlation; later observations do not rewrite baseline origin.

Added/removed/modified paths compare current complete inventory with the baseline, separately from upstream correlation. Missing tracked packages are reported as missing, not silently deleted. Incomplete inventories produce an unavailable comparison rather than false removals. A changed catalog disables upstream historical comparison and is reported; local baseline comparison remains possible. Corrupt/unsupported state is a hard diagnostic, never reset. A lock file prevents concurrent writes; exclusive baseline creation and same-directory temp/file-fsync/rename protect writes. Directory fsync is attempted on Unix; Windows skips that unsupported operation and has weaker, unverified replacement durability. Preserve previous observations; cap state reads at 32 MiB. Stale locks require manual inspection/removal. No Git requirement, rename tracking, content recovery, author attribution, prior history, or tamper-proof claims.

Only label a finding `new` when the same scanner/rule evaluation version completed on both observations and it was absent from baseline. Otherwise label it `present_in_changed_file` when applicable. External scanner findings never get `new` in this MVP because its version string alone does not pin every ruleset/configuration input.

## Scanner boundary

Own rule SA001 v2: parse .sh/.bash files with the pinned mvdan shell parser; find adjacent pipeline stages from a literal curl/wget HTTP(S) request on stdout into literal sh/bash, including within longer pipelines. Redirections must preserve the relevant pipe descriptors; shell option termination is interpreted in order. Recognize a documented conservative option subset; wget must request stdout. AST parsing avoids treating comments, quoted examples, or heredoc text as commands. No execution, URL fetching, expansion evaluation, alias/function resolution, general dataflow, descriptor moves/variable descriptors, direct |& analysis, or Markdown fence analysis. Unsupported shell files and syntax errors are coverage notes/gaps. Non-shell files are hashed but outside this rule's analysis scope. Report source, rule/version, high severity, relative file, line, bounded evidence, rationale and limitations. A risky pattern is not malicious intent.

Skill X-Ray is a separate optional scanner, not original detection logic. Interface pinned to traceforce/skill-xray commit c89eacfdd643e50d5f8e3ed18cf3c4992d174ff2, version 0.1.0: `version`, then `scan --output REPORT --llm=false -- ABSOLUTE_LOCAL_PACKAGE`. SARIF 2.1.0 only. Preserve original results/run properties, severity, rule ID, tool version, evidence and locations. Check `properties.coverage`; zero exit alone cannot imply completion. Statuses: disabled, unavailable, failed, partial, completed_with_findings, completed_without_findings. Local results survive adapter failures.

Use direct exec argument arrays, context timeout/cancellation, bounded stdout/stderr and report read, isolated temporary report directory outside packages, version refusal, and no URL targets. Run on a staged copy of the exact inventoried bytes to exclude symlinks and changes during scanner execution; incomplete inventory forces partial status. No installation or network/LLM flags. User-supplied executables are trusted code; this is not a process sandbox. OpenGrep 1.29.0 is separately required by upstream for code coverage. Missing engine must remain a gap. Real execution only if the tool is already available and safe; otherwise source-verified and fake-tested, runtime unverified.

## Output, dependencies, and acceptance

Report/v2, inventory/v3 and state/v2 have separate version markers. Readable escaped terminal strings and deterministic JSON ordering (observation timestamps intentionally vary for explicit snapshots). Priority: security findings 1, then failed pre-commit recording 3, then incomplete/operational or durability uncertainty 2, otherwise 0. Recording is independent structured output, always printed on stderr. Disabled optional scanning is not a gap. Historical state/v1 remains readable and is never silently migrated.

Use standard libraries except pinned `gopkg.in/yaml.v3 v3.0.1` for correct frontmatter and `mvdan.cc/sh/v3 v3.10.0` for shell syntax. Vendor dependencies for offline builds/tests. Avoid concurrency, database, service, UI, generic plugins, and fuzzy/LLM matching.

Acceptance tests: duplicate names across scopes; unavailable explicit roots; symlink and size skips; malformed YAML; bundled-byte changes; exact/modified/ambiguous/unknown and misleading names; baseline immutability and append persistence; additions/deletions/missing directory/corrupt state/catalog change; detector positives and comments/strings/heredocs/non-executed downloads; fake scanner timeout/bad JSON/no report/partial coverage/version errors; local findings retained after external failure. A temporary demo copies the catalog, records baseline, modifies a bundled shell script with download-to-shell, and shows modification plus a comparable new finding. Finish with gofmt, vet, offline tests, build and CLI smoke checks.


## Core review and refactor plan (2026-10-28)

Keep the four assignment behaviors and the existing digest, similarity threshold, rule semantics and state format. Make the two verified embedded Anthropic packages the default audit catalog; keep --catalog as an explicit override and search/sync as optional acquisition. Remove the now-redundant fixture exporter. Separate matching from catalog loading so its pure decision is easy to review.

Catalog evaluation identity will hash sorted reference metadata and content fingerprints with a domain marker, excluding acquisition diagnostics and separate attribution files. This identifies the inputs used for correlation; it is not an integrity signature for the entire stored artifact. Previously saved catalogs/state remain readable but the corrected identity can produce a one-time catalog_changed diagnostic; old observations are never rewritten.

Validate all inventory identities before touching package contents or launching a scanner. Sort a copy of packages for deterministic evaluation without mutating caller input. Make package inspection cooperatively cancellable, check cancellation between packages, preserve partial results, and release local byte buffers after each package has been correlated/scanned. OS filesystem calls already in progress cannot be forcibly canceled by these checks. Keep hashes, diagnostics and findings in the returned report.

Acceptance: default offline CLI correlation and lineage demo; explicit catalog override; catalog identity stable under entry order/diagnostic changes but changed by reference/content changes; malformed inventory rejected before scanner work; canceled and incomplete scans never look complete; returned reports retain findings/hashes but no local package bytes; existing detector and lineage regressions still pass.

CLI review: register flags only for the command that consumes them, so audit flags on inventory (and vice versa) fail instead of silently doing nothing. Construct the network client only in search/sync. This changes invalid-invocation handling, not the four command responsibilities.

## ADR runtime alignment: content anchors

State/v2 records explicit baseline anchors (scope, path, digest) referencing complete measurements in its first observation. That observation retains all packages, including unusable measurements, scanner evaluations, selected roots, home hint, traversal limits, exclusion policy/version and listed exclusions. Thus later complete bytes cannot silently replace an incomplete first measurement. Content completeness selects anchors; report-wide coverage affects outcomes separately. Empty completed scopes are permitted; partial baselines need at least one complete measurement. Cancellation/deadline evidence prevents recording, and snapshots preserve anchors. Missing identities stay missing/undiscovered, without deletion claims.

Inventory/v3 adds explicit discovery policy metadata; report/v2 carries this evidence and the independent operation result. Legacy inventory/v2 policy is unknown, with its original diagnostics retained. State/v1 remains readable for comparisons but append is refused: explicitly create a new baseline elsewhere, preserving prior history. No automatic migration or scope rewrite. Known macOS aliases /etc, /tmp, /var and the alternate Data tree are listed exclusions; arbitrary manager links remain gaps.

## ADR runtime alignment: ownership and bounded evidence

Home-global agent installations own bundled packages across nested markers. Worktrees/projects containers explicitly delegate project ownership; .github remains project scoped. Scope corrections do not migrate historical identities. Routine discovery exclusions are grouped by rule with counts, three examples and explicit omitted-example counts. Actionable link/read/limit gaps retain locations. Explicit system-excluded roots remain errors rather than pretending a requested root was searched.

Root deduplication stores only selected root identities. Whole-root aliases use os.SameFile; case-spelled overlapping subtrees delegate only after matching lexical spelling or verifying filesystem identity. Different case-sensitive directories must not merge. Home case aliases likewise require identity confirmation. Symlinks remain rejected before alias handling. No arbitrary mount-alias or filesystem-race guarantee is added. The obsolete policy_symlink input rewrite is removed; fresh discovery owns its coverage classification. Cache/import behavior is unchanged by this alignment.
