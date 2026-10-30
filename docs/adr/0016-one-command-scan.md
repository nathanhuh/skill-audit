---
status: accepted
date: 2026-10-30
source: user request on 2026-10-30 to run every step and create reports without options; scope "~/.claude + current folder" and output "~/.skill-audit/reports/<timestamp>/" chosen by the user; a subcommand instead of bare invocation suggested by the user
---

# 0016. One-command scan

## Context

The core workflow needs two commands, explicit roots and output paths: `inventory --root … --out …`, then `audit --inventory … --out …`. A first run on a real machine showed the cost. The device default exhausted its entry budget in caches, a relative output under a macOS `mktemp` folder was refused, and the report had to be requested separately. [ADR-0003](0003-discovery-and-exclusion-policy.md) keeps bare `skill-audit` as usage, so a one-step path needs its own subcommand.

## Decision (accepted 2026-10-30)

1. **`skill-audit scan` runs inventory, then audit, with no required options.** It calls the same inventory and audit code paths as the explicit commands, so discovery policy, budgets, output checks and exit codes ([ADR-0005](0005-outcomes-and-exit-codes.md)) are unchanged. The exit code is the audit's.
2. **Scope: `~/.claude` plus the current folder.** These are the global and project locations Claude Code itself reads. `~/.claude` is added when it exists. The current folder is resolved to its real path and added unless it is the home folder, contains the home folder, or lies inside `~/.claude` or `~/.skill-audit`. When it is skipped, scan says so and suggests running from a project folder. With no root left, scan fails with exit 2 and an explanation.
3. **Outputs: a new folder per run.** `inventory.json` and `report.json` go to `~/.skill-audit/reports/<UTC time>/`, created with owner-only permissions; a run never overwrites an earlier one. That folder is outside every scan root by rule 2. Text output prints the selected roots first, then the audit summary ([ADR-0015](0015-text-output-summary.md)), then the saved paths.
4. **Offline and read-only, as before.** Scan never records a baseline or snapshot ([ADR-0004](0004-observations-and-baselines.md)), never uses the network ([ADR-0001](0001-scope-and-trust-model.md)), and takes only `--timeout` and `--json`. `--json` prints the report JSON on stdout and moves scan's own notes to stderr. Custom roots, outputs, catalogs, recording and Skill X-Ray stay with `inventory` and `audit`; other flags and arguments are refused with a pointer to them.

Rejected alternatives: making bare `skill-audit` run the scan, which would change ADR-0003 and start a scan for anyone looking for help; scanning the whole device by default, which on the first real machine never reached the projects folder; writing into the current folder, which would put files inside the project being scanned.

## Acceptance scenarios

| ID | Scenario | Required result |
| --- | --- | --- |
| N1 | `scan` from a project folder, with one global and one project skill | Both skills reported with the right scope; `inventory.json` and `report.json` saved under `<home>/.skill-audit/reports/<UTC time>/`; text shows the roots first and the saved paths last; exit code equals the audit's. |
| N2 | `scan` from the home folder | Only `~/.claude` is scanned; a note says how to include project skills; nothing under `~/.skill-audit` is scanned. |
| N3 | Two `scan` runs, the second with `--json` | Two distinct report folders; the first run's files are unchanged; the second run's stdout is the report JSON alone. |
| N4 | `scan` over existing skills | No state file is written; recording is `not_requested`; every skill file's bytes are unchanged. |
| N5 | `scan --out x.json` or `scan extra` | Refused with exit 2 and a pointer to `inventory` and `audit`; no report folder is created. |

## Implementation gaps

None at acceptance. Implemented together with this ADR; see [scenario-tests.json](scenario-tests.json).
