---
status: accepted
date: 2026-10-29
source: AGENTS.md, DESIGN.md, PRODUCT.md; accepted by the user in ADR review on 2026-10-29 ("The scope matches my intent.")
---

# 0001. Scope and trust model

## Context

Every other ADR assumes these boundaries. Until now they were spread across AGENTS.md, DESIGN.md and PRODUCT.md as working rules, not as a stated decision.

## Decision

**Product scope.** skill-audit is an on-demand CLI with four commands: `inventory`, `search`, `sync` and `audit`. It inventories local SKILL.md packages, correlates them with pinned public references, tracks lineage when the user asks for it, and reports explainable static findings. It does not monitor agents at runtime, enforce policy, or integrate with a cloud service. It has no service, database, UI, plugin framework or unnecessary concurrency. PRODUCT.md keeps proposed company-facing work separate from what is implemented.

**Untrusted content.** Discovered skill files, scanner output, catalog content and upstream fixtures are untrusted data. They are never executed, never treated as instructions, and URLs inside them are never fetched. Shell analysis parses scripts and never runs them.

**Network boundary.** Only `search` and `sync` make network requests in skill-audit itself. `audit` and `inventory` operate offline; an explicitly configured external scanner is trusted executable code whose network behavior is not enforced by skill-audit. Search sends only an explicit `--query`. Inventory names are sent only with `--inventory` plus `--search-names`, as at most ten distinct names. Credentials come only from `GH_TOKEN`/`GITHUB_TOKEN` (and `SKILL_AUDIT_REGISTRY_TOKEN` for the registry) and are never serialized.

**External scanner.** The optional scanner is code the user supplies and trusts. It is not sandboxed. The adapter never enables cloud or LLM modes, never installs or downloads anything, and keeps the original scanner attribution. Skill X-Ray is the supported optional external scanner and is disabled unless explicitly configured with `--xray`. The built-in SA001 detector runs independently. Detailed integration and budget proposals remain under review in [0006](0006-external-scanner-budgets.md); accepting this scope does not accept those proposals.

**Evidence, not verdicts.** A correlation is content evidence. It does not establish authorship or origin. A finding is a pattern with known limitations. It does not establish malicious intent or that the code ran.

**Dependencies.** Standard library, plus the pinned and vendored `gopkg.in/yaml.v3 v3.0.1` and `mvdan.cc/sh/v3 v3.10.0`. Upstream catalog content is pinned to a commit, and licenses and notices are kept separate from package bytes.

## Consequences

- Any feature that needs runtime hooks, a daemon or remote storage is out of scope and needs a new ADR.
- Tests and demos use temporary fixtures and loopback-only fake HTTP servers. Default tests run offline without the external scanner.

## Acceptance scenarios

| ID | Scenario | Required result |
| --- | --- | --- |
| T1 | `audit` with any catalog | skill-audit itself makes no network requests. Without `--xray`, no external scanner is launched; with `--xray`, the supplied executable's network behavior is outside this guarantee. |
| T2 | A fixture skill script with an observable side effect if run | Inventory and audit never trigger the side effect. |
| T3 | Scanner invocation | Arguments include `--llm=false`. No install, download or URL target. |
| T4 | Token set in the environment | The token appears in no output file, report or diagnostic. |
| T5 | `search --inventory --search-names` with more than ten names | Refused before any request is made. |

## Implementation gaps

Not re-audited against the code in this pass. Existing tests cover T3–T5 according to DESIGN.md. T1 and T2 have no dedicated test that this pass confirmed.
