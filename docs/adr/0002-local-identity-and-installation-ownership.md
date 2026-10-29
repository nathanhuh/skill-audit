---
status: accepted
date: 2026-10-29
source: former ADR-0001 "Installation ownership contract" (accepted 2026-10-29, cb4be34); DESIGN.md; scope-and-path identity accepted by the user in ADR review on 2026-10-29 after considering move tracking; remaining classification rules accepted by the user on 2026-10-29 ("Sounds good. I accept them.")
---

# 0002. Local identity and installation ownership

## Context

The rule that decides whether a package is global or project changed in three review rounds in a row: an immediate `skills/` or `plugins/` subtree, then the nearest agent marker, then ownership. Each change rewrote stored identities and broke comparisons against existing baselines.

## Decision

**Local identity** *(accepted)* is `(scope, absolute lexical package path)`. A declared name is never an identity. Two packages with the same name in different scopes are different identities. Moving a package to a different path creates a different identity; there is no automatic move or rename linkage. Matching content could support a possible-move hint, but cannot distinguish a move from copying followed by deletion, and edits during a move can defeat exact matching. Such inference adds ambiguity and is outside this decision; existing observations and anchors remain unchanged.

**Installation ownership** *(accepted)*. Scope follows the installation that owns the package. A home-global plugin installation keeps global ownership of its bundled skills, including `~/.claude/plugins/marketplaces/m/.claude/skills/x`. An inner `.claude` marker alone cannot turn it into a project installation. Community global layouts follow the same principle. Designated project and worktree installations stay project-owned, including ones stored inside an agent home. Presence, ownership and activation are separate observations. Existing identities are never silently rewritten.

**Operational rule** *(accepted; matches the code at 5fed934)*:

1. Walk up from the package directory. If an agent directory's parent is the selected `--home` or a conventional home (`/Users/<u>`, `/home/<u>`, `/root`, `<drive>:\Users\<u>`), that agent directory owns the package:
   - If the first path segment under it is `worktrees` or `projects`, scope is **project**.
   - Otherwise scope is **global**, except under `.github`, which is never global.
2. Otherwise, if any agent directory lies on the path, scope is **project**. The nearest one supplies the agent hint.
3. Otherwise scope is **machine**.

A conventional home counts even when it is not the selected `--home`. So "global" means "owned by some user's home installation", not "owned by the current user".

**Corrections never migrate history.** A changed classification shows up as `tracked_missing` plus `not_in_baseline`. A new baseline is required to adopt it ([0008](0008-versioned-data-contracts.md)).

**Future work (not accepted behavior).** Opt-in embedded metadata with a persistent package ID could help track moves. The [product roadmap](../../PRODUCT.md#later-persistent-identity-across-moves) records this proposal and its copy/duplicate-ID questions. It requires a new decision before changing identity or anchor linkage.

## Acceptance scenarios

| ID | Scenario | Required result |
| --- | --- | --- |
| S1 | Global plugin containing a nested agent marker | Global installation ownership is kept. *(accepted)* |
| S2 | Skill in an agent-managed project worktree | Project ownership is kept. *(accepted)* |
| S3 | `~/.github/skills/x` | Project scope, not global. *(accepted)* |
| S4 | Same declared name in a global and a project package | Two distinct identities; no merge or ambiguity error. *(accepted)* |

## Implementation gaps (at 5fed934)

| Area | State |
| --- | --- |
| S1–S2 | Implemented in 64d3524 with tests. |
| S3–S4 | Behavior is in `classify` and the existing identity tests. Not re-verified for this ADR. |
| Machine scope | Exists in the code; accepted during ADR review on 2026-10-29. Not re-verified for this ADR. |

## Isolated rewrite evidence (2026-10-29)

Implementation revision `b7257a55c479501179e9a936f0d7cfffea11e751` on `codex/adr-rewrite`, separate from canonical main. This supplements the dated gap table above; it does not change the accepted decision.

S1–S4 use temporary home-global plugin, worktree/project and .github fixtures with duplicate names. Scope plus lexical absolute path remains the key. Current inventory identities are preserved during audit; no move/rename linkage or historical identity migration was added.

See [rewrite comparison](../history/rewrite-comparison.md) and [verification](../../VERIFICATION.md) for exact commands and limits.
