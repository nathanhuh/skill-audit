# Skill Audit

A small endpoint skill auditor for the Traceforce take-home. The CLI inventories local SKILL.md packages, acquires pinned public references, detects local changes and reports explainable static findings. It is an on-demand inventory and detection tool; it does not observe runtime agent execution or enforce policy.

The assignment path is **inventory → audit → explicit baseline/snapshot**. Audit defaults to two pinned Anthropic skills and works offline. Optional `search` and `sync` acquire additional references. Read [CORE_REVIEW.md](CORE_REVIEW.md) for a guided code walkthrough and [DESIGN.md](DESIGN.md) for the full contracts.

```sh
go build -o ./bin/skill-audit ./cmd/skill-audit
export PATH="$PWD/bin:$PATH"
```

The original project checkout on `main` is the canonical repository. The former V2 implementation has been promoted here; there is one active CLI. Before reviewing, verify the checkout path, branch and revision so findings refer to the current source.

## Fixture demonstration and verification

```sh
sh scripts/demo.sh
GOPROXY=off GOSUMDB=off go test ./...
go vet ./...
```

The demo creates a temporary Claude global skill and project skill. It inventories both scopes against the built-in catalog, records a baseline, changes a bundled script, refreshes inventory, and reports a modified upstream candidate plus new SA001 before appending a snapshot. It never contacts a registry or executes a skill script.

See DESIGN.md for tradeoffs, VERIFICATION.md for exact checks, and PORTABILITY.md for platform gaps. Default tests use fake HTTP handlers and local fixtures; no credentials, network service, or installed Skill X-Ray is required. On restricted sandboxes, httptest needs permission to bind a loopback listener.

## Machine discovery

```sh
# Accessible local filesystems, including multiple users/projects/agent directories.
# Do not add sudo reflexively: permission failures are useful coverage evidence.
skill-audit inventory --device --out /tmp/inventory.json

# Prefer a narrower root for a quick run; add roots as needed.
skill-audit inventory --root /home --root /opt/projects --out /tmp/inventory.json
```

Unix `--device` starts at `/`; Windows enumerates existing A–Z drive roots. Every encountered regular `SKILL.md` identifies a package, including nested project/agent folders, plugin cache layouts and generic packages outside agent paths. This finds packages on disk, not proof that a running agent uses them. Scope uses the nearest recognized agent directory. Packages throughout an agent directory in the selected home or a conventional /home/USER, /Users/USER, /root or Windows Users home are `global`, including community layouts such as `.codex/superpowers/skills`. Known `worktrees`/`projects` containers and nearest nested agent directories retain project scope. `.github` is consistently project scope. Other packages are `machine` with an unknown agent hint. These are path conventions, not an account-database or activation check. An unusual home needs an explicit `--home` hint to classify global installations. Corrected scope can change identities in existing inventories: refresh inventory; old state identities remain historical and may appear missing alongside a current `not_in_baseline` identity.

Walks use deterministic priority: directories containing the selected home first, known agent directory names next, conventional `Users`/`home`/`root` directories next, then other entries. Lexical order breaks ties. Selected-home/global roots keep priority over external roots. Component depth breaks ties within a priority group; an ancestor walk defers explicitly selected subtrees to their own traversal, preserving their depth/exclusion scope. Output roots/packages remain sorted. The home hint never adds a search root. This gets user evidence earlier in bounded device scans; it is not a guarantee that every user or project will be reached. Delegating explicit subtrees and deduplicating selected roots avoids repeated work without a per-directory visited map. Explicit dependency/depth-cut roots get their own scope; hard OS exclusions and symlink skipping still apply. Symlinks are not followed; Unix `/proc`, `/sys`, `/dev`, macOS `/System/Volumes/Data`, Git metadata, `node_modules`, `vendor`, virtual environments and Python caches are excluded with diagnostics. Defaults: 500,000 entries, 2,000 packages, depth 64, two-minute operation deadline. Flags adjust those limits. Permission errors, limits, cancellation and rejected explicit roots make coverage incomplete and return exit 2; the partial manifest is still written. Routine policy exclusions (`.git`, dependency/build caches, implicit OS subtrees and inventory symlinks) remain visible notes with `incomplete=false`. Skipped or unreadable content inside a skill package still makes that package incomplete. A baseline therefore describes observed packages within the declared no-follow/exclusion policy, rather than universal device coverage. Cancellation uses `inventory_canceled`, deadline expiry uses `inventory_timeout`, and budgets retain `entry_limit`/`package_limit`. Each selected root not started after interruption gets a diagnostic with the actual cause. Device scans can still return 2 for operational gaps. No invocation honestly guarantees every skill on inaccessible disks, other machines, containers or excluded mounts. No real device-wide scan was run during development. On macOS the alternate Data-volume tree is excluded even when selected explicitly; use unified paths such as `/Users`. This avoids the known duplicate view, but can omit Data-only content and does not deduplicate arbitrary mount aliases. Existing duplicate baseline identities are not rewritten.

Like the standard Go walker, the priority walker materializes each directory listing before ordering it. An unusually wide directory can still consume substantial memory; entry limits do not bound that individual listing. A large user directory can also exhaust the budget before later users or projects. Use explicit narrow roots for predictable coverage.

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

Audit does no internet lookup. Optional `--xray /absolute/executable` uses the pinned static-only Skill X-Ray adapter. Findings, origin uncertainty, coverage, baseline comparability, and exit codes use the semantics below. Report/output destinations must differ from selected state, inventory, catalog and saved-search inputs; state recording also requires a distinct destination. Cleaned absolute aliases, existing file aliases and case-only aliases are rejected, and symlinks remain unsupported. Validation runs before work and again before writes. Tool-owned inventory/catalog output is rejected inside an existing skill package; state is also checked against the audited package paths. Snapshot state preserves earlier observations; `baseline` refuses replacement. New baselines require complete package/coverage evaluations within the declared scan policy; findings and routine traversal exclusions are allowed. `Report.HasCoverageGaps` is shared by exit classification and baseline eligibility. Refresh inventories generated by older versions that marked routine exclusions incomplete. Choose narrower roots when genuine coverage gaps remain. Cancellation/deadline expiry prevents both baseline and snapshot writes, including a final check before atomic replacement. A failed state phase for requested recording adds `state_not_recorded` to the emitted report, writes `state not recorded` to stderr and returns exit 3; partial evidence remains available. This includes locking, existing baselines, corrupt snapshot state, coverage refusal and interruption. Argument/preflight errors still use exit 2. Non-interrupted partial audits may still be recorded as snapshots. Machine scope is added to the state validator. An unavailable file comparison labels every finding `not_compared`. A current identity absent from the baseline gets `not_in_baseline` and a diagnostic, including when discovery was incomplete; this does not prove an installation time. It has no baseline origin or inferred file additions, and its findings get `not_compared`. Current upstream candidates remain on the skill.

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

`sync` validates mandatory `--out` before reading search-name inputs or making requests. Within one invocation, successful repository/ref resolution and failed lookups are memoized. Only the active repository retains its tree and verified blob cache; switching repositories releases that cache. Returning to a repository reloads content at its pinned commit/tree and consumes the same request/import-byte budgets. A new invocation starts fresh. Returned cached content is treated as immutable. A different ref gets a separate snapshot, and every tree entry is validated before a blob cache hit.

A search returns leads, never established origins. Query names are not a matching score. GitHub search limits and `incomplete_results` are preserved; results are capped by `--limit`. Explicit `--repo` imports retain their full-repository scope alongside path/slug hints and run before search-derived targets. `sync --limit` bounds packages per repository call, not the number of repositories; omitted packages produce a diagnostic. Explicit requests still share the operation deadline and request budget. Import resolves the ref to a full immutable commit, reads its Git tree, verifies Git blob identities, computes our raw-byte SHA-256 hashes and imports complete packages. Symlinks, submodules, malformed identities, oversized content, missing targets and truncated trees cannot become exact references. GitHub search hits target their returned package path; the importer does not replace a missing target with the first alphabetic package. Search results can become stale between search and commit resolution, and a missing target remains a gap.

When using optional acquisition, all imported references stay in the catalog even when no local package matches, so repo/commit/path/hashes remain reviewable. Catalogs include bytes as base64 JSON, per-file hashes and separate root attribution/license files. They are local reference caches; a public repository is not automatically open-source or safe to redistribute. Imports do not install or execute skills. `sync --out` replaces that chosen catalog file; use another filename to preserve earlier catalog versions. Lineage history is separate and never overwritten by sync.

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

HTTP acquisition uses the [GitHub REST 2022-11-28 contract](https://docs.github.com/en/rest/search/search?apiVersion=2022-11-28), request deadlines, a 100-request operation budget, bounded responses, a 1 MiB per-blob limit, and a 16 MiB fetched-content budget per repository/ref for the entire sync operation. The active cache retains at most one bounded tree response and up to 16 MiB of blobs. Accumulated catalog entries/attribution and JSON serialization still use memory; this is not a constant-memory guarantee for the complete command. Failed lookups that actually attempted requests are memoized; request-budget refusals do not create unbounded failure entries. No repository shell command, remote script, Git hook, arbitrary download URL, or LLM is involved. Errors do not echo credential values or response bodies. Native Windows/Linux runtime and transport behavior still need validation; both targets cross-compile.

## Exit codes and evidence

- `0`: no findings or operational gaps in the selected scope.
- `1`: findings, with no operational gaps.
- `2`: an operational/preflight failure or incomplete coverage, even if findings also exist.
- `3`: a requested state-recording phase failed; stderr and the report identify `state_not_recorded`.

A missing default home directory is not automatically scanned: choose roots explicitly or use device mode. Unavailable selected roots are coverage errors. Unknown correlation means no qualifying match in the selected reference catalog; it is not evidence of maliciousness. Exact matches also are not safety verdicts. Ordinary audit never writes lineage unless `--record` is explicit.

## Original code and reused capabilities

Our implementation owns discovery, content fingerprints, deterministic similarity, observations, SA001, coverage reporting and external adapters. SA001 uses the pinned `mvdan.cc/sh/v3 v3.10.0` parser; YAML metadata uses `gopkg.in/yaml.v3 v3.0.1`. Their sources and licenses remain under `vendor/` for offline builds.

Optional [Skill X-Ray](https://github.com/traceforce/skill-xray) supplies additional scanner results, retaining its own rule identities and severity. The adapter targets commit `c89eacfdd643e50d5f8e3ed18cf3c4992d174ff2`, version `0.1.0`: `version`, then `scan --output REPORT --llm=false -- ABSOLUTE_STAGED_PACKAGE`. It consumes SARIF 2.1.0 and checks coverage, rather than interpreting exit success as a clean scan. The upstream static engine requires OpenGrep 1.29.0. No binary/dependency is automatically installed. Obtain the pinned tool and dependencies separately; setup can require network access. Real integration is still unverified here. The executable is trusted code, not sandboxed by this adapter. One audit session probes its version with a separate five-second startup budget per attempt. A probe process failure gets one retry; unsupported versions and unavailable executables remain terminal for the session. The operation deadline bounds startup as well as scanning. Each package still gets a fresh staging/report subdirectory under the session temporary root, preventing stale report reuse; each package receives a fresh scan timeout after startup, with bounded cleanup. A mid-audit executable replacement is outside this trusted-code boundary. There is no Traceforce cloud integration or MCP-Xray substitution.

The two Apache-2.0 catalog fixtures are `brand-guidelines` and `web-artifacts-builder` from `anthropics/skills` at `dbd4588f9e1033efb41dad4bef2f7947c8993d44`. Their complete bytes, hashes, licenses and attribution are retained under `internal/audit/catalog/`. Skill scripts are data and are never executed by tests or the demo.

## Repository walkthrough

- `cmd/skill-audit`: the sole user-facing command and its offline workflow tests.
- `internal/workflow`: device inventory, public acquisition, external catalog persistence and offline audit orchestration.
- `internal/audit`: package inspection, matching, own rule, lineage and optional Skill X-Ray adapter.
- `scripts/demo.sh`: temporary global/project skills, exact baseline, bundled modification, new finding and appended snapshot.

The former scan/baseline/snapshot/catalog CLI has been removed. Use `audit --record baseline|snapshot` for observations. Saved schema identifiers retain their existing versions; a `/v1` report or `/v2` inventory identifies a data contract, not a second executable.

Command help lists only relevant flags; irrelevant flags are errors. Catalog identity depends on sorted reference metadata and content hashes, not acquisition diagnostics or attribution packaging. Previously recorded catalog IDs may differ once after the fingerprint correction; old observations remain intact. See [PRODUCT.md](PRODUCT.md) only for deferred proposals outside the assignment.

## SA001 v2 coverage

The rule recognizes adjacent literal curl/wget and sh/bash stages joined by `|`, including pairs inside a longer pipeline. Stderr-only redirects and unrelated numeric descriptor copies/closures are allowed. Redirects that replace downloader stdout or interpreter stdin are rejected; descriptor moves, variable descriptors, grouped-stage dataflow and direct `|&` connections remain outside scope. Shell option order matters: `bash -- -s` selects a filename, whereas `bash -s -- -s` reads commands from stdin. These semantics follow the [Bash invocation rules](https://www.gnu.org/s/bash/manual/bash.html#Invoking-Bash) and [pipeline/redirection rules](https://www.gnu.org/software/bash/manual/html_node/Pipelines). Fixtures are parsed as data, never executed.

The evaluation version changed to SA001/v2. An older SA001/v1 baseline cannot establish that a v2 finding is new; changed-file findings receive the weaker comparison label. Existing observations are preserved.

Shebang eligibility recognizes sh/bash interpreter tokens, optional whitespace/CRLF, flag-only arguments such as `-e`/`-eux`, `env sh/bash`, and unquoted `env -S sh/bash` forms. Unknown flags, command/option values, environment assignments and quoted split strings remain unsupported with partial coverage. The pinned parser handles CRLF while hashes and evidence refer to raw bytes; evidence is capped at 240 bytes on a UTF-8 boundary, plus an ellipsis. This is static pattern analysis, not a prediction that an OS will execute a particular shebang.

The current evaluation identity is `SA001/v2;sh/v3.10.0;scope/v2`: the rule is still v2, but expanded file eligibility and corrected evidence changed the evaluation scope. Older scope/v1 baselines cannot justify `new` labels. Windows state writes skip unsupported directory syncing; file flushing still precedes replacement, but crash durability/atomic replacement remain unverified on native Windows. See PORTABILITY.md.
