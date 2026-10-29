# Architecture decision records

Each ADR records one decision: its rule, consequences, acceptance scenarios with stable IDs, and a gap table that compares the rule with the code at a named revision. Vocabulary is defined in [CONTEXT.md](../../CONTEXT.md).

| ADR | Decision | Status | Scenarios |
| --- | --- | --- | --- |
| [0001](0001-scope-and-trust-model.md) | Scope and trust model | accepted | T1–T5 |
| [0002](0002-local-identity-and-installation-ownership.md) | Local identity and installation ownership | accepted | S1–S4 |
| [0003](0003-discovery-and-exclusion-policy.md) | Discovery, exclusions and links | accepted | D1–D13 |
| [0004](0004-observations-and-baselines.md) | Observations, baselines and anchors | accepted | B1–B11 |
| [0005](0005-outcomes-and-exit-codes.md) | Outcomes and exit codes | accepted | E1–E9 |
| [0006](0006-external-scanner-budgets.md) | External scanner integration and time budgets | accepted | X1–X10 |
| [0007](0007-reference-acquisition.md) | Reference acquisition, cache and budgets | accepted | C1–C8 |
| [0008](0008-versioned-data-contracts.md) | Versioned data contracts | accepted | V1–V7 |
| [0009](0009-portability-boundaries.md) | Portability boundaries | accepted | P1–P5 |
| [0010](0010-inventory-output-budget.md) | Inventory output and gap-detail budgets | accepted | I1–I4 |
| [0011](0011-output-path-identity.md) | Output/input filesystem identity | accepted | O1–O3 |
| [0012](0012-sa001-interpreter-options.md) | SA001 interpreter flag subset and v3 | accepted | R1–R4 |
| [0013](0013-search-provenance.md) | Search provenance across merged inputs | accepted | Q1–Q4 |
| [0014](0014-upstream-file-comparison.md) | File-level comparison with the matched upstream reference | accepted | U1–U4 |

## Status

- **accepted**: the user approved this rule. The ADR names the date and source.
- **proposed**: drafted from existing docs, code, or review findings. It is not binding until the user accepts it. Sections marked **Needs decision** state the recommendation and the alternatives.
- **superseded by NNNN**: kept for history and no longer binding.

Accepted content from the former single ADR-0001 (`0001-observation-and-outcome-contracts.md`, accepted 2026-10-29 in `cb4be34`) now lives in 0002, 0004, 0005 and 0007, with its scenario IDs unchanged. Those rules stay accepted. Only the file layout changed.

## Process rules

1. **A review finding must cite an ADR.** It either names the scenario or rule it breaks (for example "violates X2"), or it is labelled **policy gap**.
2. **A policy gap is never fixed directly.** It first gets a decision: an amendment to an ADR, or a new ADR. Then the fix cites that decision.
3. **An accepted rule changes only through a new decision.** That decision marks which part it supersedes. A review finding alone cannot change accepted policy.
4. **Gap tables are dated evidence.** They describe the code at a named revision. Implementing a scenario updates the table and VERIFICATION.md; it does not change the rule.
5. Historical review plans are in [docs/history/design-review-notes.md](../history/design-review-notes.md). Wherever they conflict with an ADR, the ADR wins.

## Scenario test inventory

Every accepted ID is registered in [scenario-tests.json](scenario-tests.json), with dated source/runtime evidence in [scenario-tests.md](scenario-tests.md). `TestAcceptedADRScenariosHaveTests` checks the index ranges, accepted scenario rows and real Go test declarations. Update the map when accepting a new ID or renaming a test. After `go test -json`, `scripts/verify-scenarios.py` checks the mapped runtime events, including subtests; native-only requirements are reported explicitly.
