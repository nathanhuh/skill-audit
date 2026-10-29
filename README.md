# Skill Audit

A small endpoint skill auditor for the Traceforce take-home. The CLI inventories local SKILL.md packages, acquires pinned public references, detects local changes and reports explainable static findings. It is an on-demand inventory and detection tool; it does not observe runtime agent execution or enforce policy.

The assignment path is **inventory → audit → explicit baseline/snapshot**. Audit defaults to two pinned Anthropic skills and works offline. Optional `search` and `sync` acquire additional references. Read [CORE_REVIEW.md](CORE_REVIEW.md) for a guided code walkthrough and [DESIGN.md](DESIGN.md) for the full contracts.

```sh
go build -o ./bin/skill-audit ./cmd/skill-audit
export PATH="$PWD/bin:$PATH"
```

## Brief coverage

| Brief requirement | How | Shown by |
| --- | --- | --- |
| 1. Discover global and project-level Claude skills on the device | `inventory --device` (or `--root`) classifies every `SKILL.md` package as `global`, `project` or `machine` by installation ownership | `scripts/demo.sh`; [ADR-0002](docs/adr/0002-local-identity-and-installation-ownership.md), [ADR-0003](docs/adr/0003-discovery-and-exclusion-policy.md) |
| 2. Correlate against 2–3 skills from anthropics/skills | Two skills pinned to commit `dbd4588` are embedded; `audit` reports `exact`, `possible_modified` (content similarity, names ignored), `ambiguous` or `unknown` | `scripts/demo.sh`; [Catalog fixtures and matching](DESIGN.md#catalog-fixtures-and-matching) |
| 3. Track lineage: pulled from open source, then modified | Each modified candidate lists files changed relative to the pinned upstream reference, with no baseline needed; `--record baseline\|snapshot` adds changes relative to your own recorded observation | `scripts/demo.sh`; [ADR-0014](docs/adr/0014-upstream-file-comparison.md), [ADR-0004](docs/adr/0004-observations-and-baselines.md) |
| 4. One security-relevant finding type | SA001: a downloaded script piped into `sh`/`bash` (`curl … \| bash`), parsed rather than executed, with file, line and evidence | `scripts/demo.sh`; [SA001 v3 coverage](#sa001-v3-coverage) |

Everything else is optional: `search`/`sync` acquire more references, and `--xray` adds the external Skill X-Ray scanner. The core path above works offline with no flags beyond those shown.

The document-driven rewrite is the implementation selected for canonical `main`, promoted locally at the user's request after review and fixture verification. The frozen reference `8c6f3b83d725c716246bdbdecc8495bb9cedbccb` remains in Git history. See [host-local filename fix and scenario tests](docs/history/host-paths-and-scenarios.md) for the latest focused changes and [second-review record](docs/history/review-2-fixes.md) for promotion evidence, and [docs/history/rewrite-comparison.md](docs/history/rewrite-comparison.md) for the dated original comparison. There is one active CLI. Verify path, branch and revision before reviewing.

Binding decisions are in the [architecture decision records](docs/adr/README.md). All fourteen ADRs are accepted. The rewritten implementation and scenario tests follow those decisions; platform runtime checks and real Skill X-Ray remain explicitly pending. Gap tables retain their original revision context, with rewrite evidence appended. See VERIFICATION.md for commands, freeze checks and limitations.

## Fixture demonstration and verification

```sh
sh scripts/demo.sh
GOPROXY=off GOSUMDB=off go test ./...
go vet ./...
```

The demo creates a temporary Claude global skill and project skill. It inventories both scopes against the built-in catalog, records a baseline, changes a bundled script, refreshes inventory, and reports a modified upstream candidate plus new SA001 before appending a snapshot. It never contacts a registry or executes a skill script.

See DESIGN.md for tradeoffs, VERIFICATION.md for exact checks, and PORTABILITY.md for platform gaps. All accepted ADR scenario IDs map to tests in [scenario-tests.json](docs/adr/scenario-tests.json), with a [source/evidence table](docs/adr/scenario-tests.md). The normal Go suite rejects missing mappings or renamed tests; native-only cases remain explicitly unverified. Default tests use fake HTTP handlers and local fixtures; no credentials, network service, or installed Skill X-Ray is required. On restricted sandboxes, httptest needs permission to bind a loopback listener.

## Machine discovery

```sh
# Accessible local filesystems, including multiple users/projects/agent directories.
# Do not add sudo reflexively: permission failures are useful coverage evidence.
skill-audit inventory --device --out /tmp/inventory.json

# Prefer a narrower root for a quick run; add roots as needed.
skill-audit inventory --root /home --root /opt/projects --out /tmp/inventory.json
```

With no scope flags, `inventory` defaults to device selection. `--root` alone remains narrow. Unix device discovery starts at `/`; Windows enumerates existing A–Z drive roots. Every encountered regular `SKILL.md` identifies a package, including nested project/agent folders, plugin cache layouts and generic packages outside agent paths. This finds packages on disk, not proof that a running agent uses them. Scope follows installation ownership; the nearest recognized agent directory supplies only a fallback project hint. Packages throughout an agent directory in the selected home or a conventional /home/USER, /Users/USER, /root or Windows Users home are `global`, including community layouts such as `.codex/superpowers/skills`. A home-global plugin/community installation retains global scope across nested agent markers, including `~/.claude/plugins/marketplaces/m/.claude/skills/x`. Known `worktrees`/`projects` containers explicitly delegate ownership to projects. `.github` is consistently project scope. Other packages are `machine` with an unknown agent hint. These are path conventions, not an account-database or activation check. An unusual home needs an explicit `--home` hint to classify global installations. Corrected scope can change identities in existing inventories: refresh inventory; old state identities remain historical and may appear missing alongside a current `not_in_baseline` identity.

Explicit roots precede device-added roots, retaining deterministic priority/depth/lexical order within each group (ADR-0003 D13). Walks use deterministic priority: directories containing the selected home first, known agent directory names next, conventional `Users`/`home`/`root` directories next, then other entries. Lexical order breaks ties. Selected-home/global roots keep priority over external roots. Component depth breaks ties within a priority group; an ancestor walk defers explicitly selected subtrees to their own traversal, preserving their depth/exclusion scope. Output roots/packages remain sorted. The home hint never adds a search root. This gets user evidence earlier in bounded device scans; it is not a guarantee that every user or project will be reached. Delegating explicit subtrees and deduplicating selected roots avoids repeated work without a per-directory visited map. Case aliases require filesystem-identity confirmation; spelling alone never merges distinct directories. Selected root aliases remain listed. Explicit dependency/depth-cut roots get their own scope; hard OS exclusions and symlink skipping still apply. Symlinks are not followed; Unix `/proc`, `/sys`, `/dev`, macOS `/System/Volumes/Data` and aliases `/etc`, `/tmp`, `/var` (their `/private` targets remain discoverable), Git metadata, `node_modules`, `vendor`, virtual environments and Python caches are excluded with diagnostics. Defaults: 500,000 entries, 2,000 packages, depth 64, two-minute operation deadline. Flags adjust those limits. Permission errors, limits, cancellation and rejected explicit roots make coverage incomplete and return exit 2; the partial manifest is still written. Routine policy exclusions (`.git`, `.DS_Store`, dependency/build caches and implicit OS subtrees) are summarized by rule/count of encountered excluded entries (not unseen descendants), with at most three examples and an explicit omitted_examples count. One informational diagnostic per rule keeps terminal evidence bounded; individual skipped links and errors retain their affected paths. Skipped symlinks outside those exclusions remain incomplete because their targets may contain skills; manager-installed links are never followed. Skipped or unreadable content inside a skill package still makes that package incomplete. A baseline therefore describes observed packages within the declared no-follow/exclusion policy, rather than universal device coverage. Cancellation uses `inventory_canceled`, deadline expiry uses `inventory_timeout`, and budgets retain `entry_limit`/`package_limit`. Each selected root not started after interruption gets a diagnostic with the actual cause. Device scans can still return 2 for operational gaps. Fresh inventory link diagnostics remain coverage gaps; audit preserves input diagnostics without the obsolete compatibility rewrite. No invocation honestly guarantees every skill on inaccessible disks, other machines, containers or excluded mounts. No real device-wide scan was run during development. Explicit macOS /etc, /tmp and /var roots (including descendants) reach the no-follow check and report a symlink gap; use their /private equivalents. Discovered aliases remain summarized policy exclusions. On macOS the alternate Data-volume tree is excluded even when selected explicitly; use unified paths such as `/Users`. This avoids the known duplicate view, but can omit Data-only content and does not deduplicate arbitrary mount aliases. Existing duplicate baseline identities are not rewritten.

Like the standard Go walker, the priority walker materializes each directory listing before ordering it. An unusually wide directory can still consume substantial memory; entry limits do not bound that individual listing. A large user directory can also exhaust the budget before later users or projects. Use explicit narrow roots for predictable coverage.

Inventory output is admitted under a 64 MiB formatted JSON budget. Package measurements are admitted whole; if the next measurement cannot fit, collection stops with `inventory_output_budget` and `budget_stop`, preserving earlier measurements. Discovery gap details are capped at 1 MiB/1,000 records, with explicit `diagnostics_truncated` and exact `omitted_diagnostics` counts; their summary uses reserved headroom. If mandatory scope/coverage metadata cannot fit, output fails explicitly. Truncation evidence survives audit and stored observations. See ADR-0010.

The inventory records identities, agent hints and hashes, not local skill contents. It is not an atomic filesystem snapshot. Later audit re-reads the packages and reports `inventory_drift` if bytes changed or inventory was incomplete; refresh inventory to establish the new current observation.

## Audit offline and record lineage

```sh
skill-audit audit --inventory /tmp/inventory.json
skill-audit audit --inventory /tmp/inventory.json \
  --state /tmp/state.json --record baseline --json

# After local changes, refresh inventory, then compare or append a snapshot.
skill-audit inventory --root /home/developer/projects --out /tmp/inventory.json
skill-audit audit --inventory /tmp/inventory.json \
  --state /tmp/state.json --record snapshot
```

**Modified copies of upstream skills.** Lineage has two independent parts. *Upstream comparison* needs no baseline: when a local package is a `possible_modified` match for a catalog skill, each candidate lists the files added, removed or modified relative to that pinned reference (`upstream_modified="SKILL.md"` in text, `upstream_changes` in JSON). That covers a skill that was edited before skill-audit first ran. A difference is relative to the pinned commit and does not say whether the user or a newer upstream version changed it. An incomplete local measurement reports `upstream_comparison=unavailable` instead of guessing. *Baseline lineage* compares against your own recorded observation. See [ADR-0014](docs/adr/0014-upstream-file-comparison.md).

Audit does no internet lookup. Optional `--xray /absolute/executable` uses the pinned static-only Skill X-Ray adapter. Findings, origin uncertainty, coverage, baseline comparability, and exit codes use the semantics below. Report/output destinations must differ from selected state, inventory, catalog and saved-search inputs; state recording also requires a distinct destination. Exact cleaned absolute paths and confirmed filesystem aliases are rejected; distinct case-only files on case-sensitive filesystems are permitted. Symlinks remain unsupported. Planned destinations are revalidated before each write, preserving committed state if a later report destination aliases it. Validation runs before work and again before writes. Tool-owned inventory/catalog output is rejected inside an existing skill package; state is also checked against the audited package paths. Snapshot state preserves earlier observations; `baseline` refuses replacement. New baselines select fully measured packages as immutable anchors, even when discovery elsewhere or an optional scanner is incomplete. Discovery gaps remain visible and affect the exit code. A partial baseline with no complete packages is refused; completed empty scopes are allowed. Uninspected or incomplete packages cannot anchor. Scanner failure prevents comparable detection claims, without invalidating content bytes. Canceled/timed-out observations cannot be recorded. Non-interrupted snapshots may preserve partial observations without changing anchors. Requested recording prints one independent outcome to text stdout, once to stderr, and in report JSON: findings take exit priority over save failures and gaps. Successful replacement commits the record; a final directory-sync warning means saved with uncertain durability, so inspect before retrying. Machine scope is added to the state validator. An unavailable file comparison labels every finding `not_compared`. A current identity absent from the baseline gets `not_in_baseline` and a diagnostic, including when discovery was incomplete; this does not prove an installation time. It has no baseline origin or inferred file additions, and its findings get `not_compared`. Current upstream candidates remain on the skill.

When state lies under a discovery root, use the same `--state PATH` on inventory to reserve that exact file and record `state/location` exclusion evidence. This flag never excludes its parent directory. If audit selects a different state path within the discovery roots, it reports `state_exclusion_unconfirmed` and refuses recording until inventory is refreshed with that path; it does not invent retroactive exclusion evidence. State must be outside every skill package. Ordinary inventory and audit do not write state.

The binary includes the two pinned Anthropic references; it needs no catalog setup for the assignment. Supply `--catalog /path/to/catalog.json` to override them with an acquired catalog. An invalid explicit catalog is an error, never a silent fallback. The licensed source fixtures and vendored parser dependencies keep builds/tests offline; deployment needs the binary and only an optional external catalog.

## Optional: find candidates and import references

GitHub is the default search service. Its code-search API requires `GH_TOKEN` or `GITHUB_TOKEN`; public explicit repository imports can work anonymously, subject to rate limits. Never put credentials in command arguments or saved JSON.

```sh
# Explicit query: no local files are read or uploaded by this command.
skill-audit search --query 'web artifacts' --out /tmp/candidates.json

# Explicit opt-in: send deduplicated names from inventory, never local file contents.
# At most 10 distinct names per invocation; narrow inventory or use --query for more.
skill-audit search --inventory /tmp/inventory.json --search-names --out /tmp/candidates.json

# Consume saved search results; import the package paths found by search.
skill-audit sync --search /tmp/candidates.json --out /tmp/catalog.json

# Or search and acquire in one explicit network step.
skill-audit sync --query 'web artifacts' --out /tmp/catalog.json

# Known upstream: no code-search authorization required for public repositories.
skill-audit sync --repo anthropics/skills --ref dbd4588f9e1033efb41dad4bef2f7947c8993d44 --limit 20 --out /tmp/catalog.json
```

`sync` validates mandatory `--out` before reading search-name inputs or making requests. It groups all targets for explicit repositories first (full import, then hits), then remaining repositories lexically. A repository/ref stays pinned to its successful immutable resolution; one active repository holds its tree and at most 16 MiB of verified cached blobs. Cache misses do not invalidate a package or double-count retained content. Permanent 404/private/truncated/unsafe-tree failures are memoized; transient 403/5xx/transport failures stay retryable on later targets, within 100 requests.

Complete imports are admitted together with required attribution under both 40 MiB retained raw content and the actual 64 MiB formatted JSON output bound. Shared retained attribution and deduplicated entries count once. Already admitted attribution is reused even when the blob cache is full. Diagnostic headroom is reserved. A `sync_budget` result keeps earlier complete imports and stops before the next package rather than discarding the entire store. These budgets do not promise constant memory for accumulated results or serialization.

Search writes `skill-audit/search/v3`: an `inputs` list preserves one batch per newly requested query, including its selected provider, hits, completeness and diagnostics. Failed and empty queries keep their batch. Earlier grouped v3 evidence stays grouped because its hit membership is unknown. Published search/v2 is still accepted as one input; its joined query is kept opaque. To save combined evidence, use `search --search /tmp/candidates.json --query 'another query' --out /tmp/combined.json`, then sync that file. `sync --search ... --query ...` accepts the same combination. Duplicate hits keep their input membership; sync deduplicates imported references. See ADR-0013.

A search returns leads, never established origins. Query names are not a matching score. GitHub search limits and `incomplete_results` are preserved; results are capped by `--limit`. Explicit `--repo` imports retain their full-repository scope alongside path/slug hints and run before search-derived targets. `sync --limit` bounds packages per repository call, not the number of repositories; omitted packages produce a diagnostic. Explicit requests still share the operation deadline and request budget. Import resolves the ref to a full immutable commit, reads its Git tree, verifies Git blob identities, computes our raw-byte SHA-256 hashes and imports complete packages. Symlinks, submodules, malformed identities, oversized content, missing targets and truncated trees cannot become exact references. GitHub search hits target their returned package path; the importer does not replace a missing target with the first alphabetic package. Search results can become stale between search and commit resolution, and a missing target remains a gap.

When using optional acquisition, all imported references stay in the catalog even when no local package matches, so repo/commit/path/hashes remain reviewable. Catalogs include bytes as base64 JSON, per-file hashes and separate root attribution/license files. New imports omit declared fingerprint metadata (.git/.hg/.svn/.DS_Store) using the local measurement policy; re-sync older metadata-containing imports rather than migrating their stored bytes. Relevant dependency content stays in references; a local package that omits it remains incomplete and cannot claim exact matching. They are local reference caches; a public repository is not automatically open-source or safe to redistribute. Imports do not install or execute skills. `sync --out` replaces that chosen catalog file; use another filename to preserve earlier catalog versions. Lineage history is separate and never overwritten by sync.

### Configurable registry URL

```sh
# Endpoint must implement the documented Skills.sh v1 search response.
# SKILL_AUDIT_REGISTRY_TOKEN is separate from GitHub credentials.
skill-audit search --registry-url https://skills.sh/api/v1/skills/search \
  --query 'web artifacts' --out /tmp/candidates.json
skill-audit sync --search /tmp/candidates.json --out /tmp/catalog.json
```

The adapter expects `data[]` records with `source`, `sourceType`, optional `slug`, and `url`, plus `count`. Only GitHub sources are imported. A registry slug is a retrieval hint matched against a package directory name; a renamed/unresolved slug reports a missing target rather than guessing another package. Arbitrary registries with other schemas are not compatible merely because they have a URL.

Skills.sh is Vercel's documented discovery directory, not a provenance or security authority. Its [v1 API](https://www.skills.sh/docs/api) documents Vercel OIDC bearer authentication. Supply an appropriate token as `SKILL_AUDIT_REGISTRY_TOKEN` if available; this project never installs Vercel tooling or obtains tokens. The GitHub token is never forwarded to the registry. Only the explicitly configured endpoint gets the registry token; redirects are refused. Authenticated live search is unverified here.

HTTP acquisition uses the [GitHub REST 2022-11-28 contract](https://docs.github.com/en/rest/search/search?apiVersion=2022-11-28), request deadlines, a 100-request operation budget, bounded responses, a 1 MiB per-blob limit, and a 16 MiB package-content limit. The active cache retains at most one bounded tree response and up to 16 MiB of blobs. Once full, further verified bytes are returned without caching; this cap is separate from per-package eligibility and does not truncate a monorepo. Accumulated catalog entries/attribution and JSON serialization still use memory; this is not a constant-memory guarantee for the complete command. Permanent failures that actually attempted requests are memoized; transient failures and request-budget refusals do not create permanent failure entries. No repository shell command, remote script, Git hook, arbitrary download URL, or LLM is involved. Errors do not echo credential values or response bodies. Native Windows/Linux runtime and transport behavior still need validation; both targets cross-compile.

## Exit codes and evidence

- `0`: no findings or operational gaps in the selected scope.
- `1`: security findings, taking priority over coverage or recording failures.
- `2`: no security findings, but an operational/preflight failure, incomplete coverage or uncertain durability.
- `3`: no security findings and the actual requested recording failed before commit. Input diagnostic names cannot select this exit.

JSON reports expose overall `complete` and `created_at`; `discovery.complete` separately describes discovery. Inventory, catalog and state readers normalize valid hex hash spelling in memory while preserving input files and historical observation fields.

A home hint never adds a root. Inventory without scope flags uses device roots; select explicit roots for predictable coverage. Unavailable selected roots are coverage errors. Unknown correlation means no qualifying match in the selected reference catalog; it is not evidence of maliciousness. Exact matches also are not safety verdicts. Ordinary audit never writes lineage unless `--record` is explicit.

## Original code and reused capabilities

Our implementation owns discovery, content fingerprints, deterministic similarity, observations, SA001, coverage reporting and external adapters. SA001 uses the pinned `mvdan.cc/sh/v3 v3.10.0` parser; YAML metadata uses `gopkg.in/yaml.v3 v3.0.1`. Their sources and licenses remain under `vendor/` for offline builds.

Optional [Skill X-Ray](https://github.com/traceforce/skill-xray) supplies additional scanner results, retaining its own rule identities and severity. The adapter targets commit `c89eacfdd643e50d5f8e3ed18cf3c4992d174ff2`, version `0.1.0`: `version`, then `scan --output REPORT --llm=false -- ABSOLUTE_STAGED_PACKAGE`. It consumes SARIF 2.1.0 and checks coverage, rather than interpreting exit success as a clean scan. The upstream static engine requires OpenGrep 1.29.0. No binary/dependency is automatically installed. Obtain the pinned tool and dependencies separately; setup can require network access. Real integration is still unverified here. The executable is trusted code, not sandboxed by this adapter. A temp base within any audited package or an existing package ancestor is refused before staging writes; scanner output cannot mutate installed skill contents through TMPDIR. Console caps preserve the actual process exit and report a per-package log gap; they do not masquerade as launch failures. One audit session lazily probes its version. `--scan-timeout` defaults to 30 seconds for the probe and each package scan; it can be raised explicitly. Each phase uses the smaller of that timeout and remaining operation time minus a reserve of 25% of `--timeout`. A probe/launch failure, scan timeout or reserve skip stops later scanner launches, preserving its cause. Only an explicit temporary-failure probe exit (75) gets one retry within the original shared phase deadline; timeouts, launch failures and unsupported versions remain terminal for the session. Operator cancellation and the overall deadline still bound the audit and can prevent recording (B6). These budgets do not guarantee completion of slow filesystem work. Already-canceled sessions perform no setup, and the audit loop stops before later packages. Each package still gets a fresh staging/report subdirectory under the session temporary root, preventing stale report reuse; each package receives a fresh bounded scan budget after startup, with bounded cleanup. A mid-audit executable replacement is outside this trusted-code boundary. There is no Traceforce cloud integration or MCP-Xray substitution.

The two Apache-2.0 catalog fixtures are `brand-guidelines` and `web-artifacts-builder` from `anthropics/skills` at `dbd4588f9e1033efb41dad4bef2f7947c8993d44`. Their complete bytes, hashes, licenses and attribution are retained under `internal/audit/catalog/`. Skill scripts are data and are never executed by tests or the demo.

## Repository walkthrough

- `cmd/skill-audit`: the sole user-facing command and its offline workflow tests.
- `internal/audit`: discovery, package measurement, matching, own rule, lineage, compatibility and optional network/scanner boundaries. See CORE_REVIEW.md for file-level entry points.
- `scripts/demo.sh`: temporary global/project skills, exact baseline, bundled modification, new finding and appended snapshot.

The former scan/baseline/snapshot/catalog CLI has been removed. Use `audit --record baseline|snapshot` for observations. Current contracts are report/v2, inventory/v3 and state/v3; these identify data contracts, not separate executables. Inventory/v2 remains readable with unknown policy metadata. Published state/v1 is readable for comparison but read-only for recording; unpublished state/v2 is rejected: preserve it and explicitly create a new baseline at another path. No silent migration occurs. State/v3 derives anchors from complete measurements in the first observation, with no redundant baseline_anchors array. Files, origins and scanner evaluations remain stored there once. New observations store time only as observed_at; delivered reports expose created_at. Matching older duplicate timestamps remain readable; conflicting times are rejected. Stored observations omit recording: the final disk-sync result cannot be known until after their bytes are committed. The final CLI report carries the actual recording outcome; history alone cannot attest to that final durability check. Every observation preserves selected roots, policy/version, exclusions and gaps.

Command help lists only relevant flags; irrelevant flags are errors. Catalog identity depends on sorted reference metadata and content hashes, not acquisition diagnostics or attribution packaging. Previously recorded catalog IDs may differ once after the fingerprint correction; old observations remain intact. See [PRODUCT.md](PRODUCT.md) only for deferred proposals outside the assignment.

## SA001 v3 coverage

The rule recognizes adjacent literal curl/wget and sh/bash stages joined by `|`, including pairs inside a longer pipeline. Stderr-only redirects and unrelated numeric descriptor copies/closures are allowed. Redirects that replace downloader stdout or interpreter stdin are rejected; descriptor moves, variable descriptors, grouped-stage dataflow and direct `|&` connections remain outside scope. Shell option order matters: `bash -- -s` selects a filename, whereas `bash -s -- -s` reads commands from stdin. These semantics follow the [Bash invocation rules](https://www.gnu.org/s/bash/manual/bash.html#Invoking-Bash) and [pipeline/redirection rules](https://www.gnu.org/software/bash/manual/html_node/Pipelines). Fixtures are parsed as data, never executed.

ADR-0012 expands receiver flags to `-e`, `-u`, `-x`, `-v`, `-s` and grouped forms. Its accepted lone-dash amendment treats `-` as option termination: `sh -` qualifies; a later filename still needs an earlier `-s`. Command mode, parse-only `-n`, unknown/value-taking options and filenames without `-s` remain excluded. SA001 is now v3; earlier v1/v2 baselines cannot establish comparable `new` findings. Existing observations are preserved.

Shebang eligibility recognizes sh/bash interpreter tokens, optional whitespace/CRLF, flag-only arguments such as `-e`/`-eux`, `env sh/bash`, and unquoted `env -S sh/bash` forms. Unknown flags, command/option values, environment assignments and quoted split strings remain unsupported with partial coverage. The pinned parser handles CRLF while hashes and evidence refer to raw bytes; evidence is capped at 240 bytes on a UTF-8 boundary, plus an ellipsis. This is static pattern analysis, not a prediction that an OS will execute a particular shebang.

The current evaluation identity is `SA001/v3;sh/v3.10.0;scope/v2`. V2 and older evaluations cannot justify `new` labels under v3; compatible package-content comparisons remain available. Windows state writes skip unsupported directory syncing; file flushing still precedes replacement, but crash durability/atomic replacement remain unverified on native Windows. See PORTABILITY.md.

### ADR runtime alignment: independent outcomes

Reports now use `skill-audit/report/v2`. The `recording` object carries `status`, optional `path` and `message`: `not_requested`, `recorded`, `not_recorded`, or `committed_durability_unconfirmed`. Requested recording prints its outcome once on text stdout and once on stderr; ordinary audits stay quiet there and retain recording.status=not_requested in JSON. Priority is **1 > 3 > 2 > 0**; automation needing persistence must inspect this object. A post-replacement directory-sync warning means the state is saved: inspect before retrying. Historical report/v1 observations remain readable, with no fabricated recording field. Per-package anchors, installation ownership and bounded discovery exclusions follow ADRs [0002](docs/adr/0002-local-identity-and-installation-ownership.md), [0003](docs/adr/0003-discovery-and-exclusion-policy.md) and [0004](docs/adr/0004-observations-and-baselines.md). State/v1 is read-only and state/v2 is unsupported; create a new baseline path to adopt state/v3.
