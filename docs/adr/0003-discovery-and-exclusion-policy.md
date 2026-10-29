---
status: accepted
date: 2026-10-29
source: DESIGN.md inventory and traversal sections; former ADR-0001 baseline bullets on exclusions (accepted); review rounds 2026-10-28/29; accepted by the user in ADR review on 2026-10-29, including default-device scope, explicit dependency roots and separate discovery/package completeness
---

# 0003. Discovery, exclusions and links

## Context

Whether a skipped symlink counts as a coverage gap flipped twice across reviews. Exclusion checks moved between directories only and all entries. macOS alias handling changed twice. The accepted parts of the former ADR-0001 settled what a baseline needs, but not how discovery classifies what it skips.

## Decision

**Selected scope.** Accepted during user review on 2026-10-29: `inventory` with neither `--root` nor `--device` defaults to a device scan, using the same roots, exclusions and budgets as explicit `--device`. Supplying `--root` selects those roots without implicitly adding device roots; explicit `--device` remains available. This changes scope selection only: invoking `skill-audit` without a subcommand still shows usage. The default-device behavior is not yet implemented. Traversal is deterministic. Roots are ordered by priority, then by path-component depth, then lexically. Within a root, the home directory comes first, then agent directories, then conventional user containers, then the rest in lexical order. A selected descendant root is walked as its own root. Overlapping roots are deduplicated by filesystem identity (`os.SameFile`). Paths that differ only in case merge only after their filesystem identity is confirmed.

**Never follow links.** Symlinks, symlinked path components and, on Windows, junctions and other reparse points are never followed. This holds in every case for the current version. [Linked-skill support](../../PRODUCT.md#later-linked-skill-support) is a roadmap proposal requiring a separate decision before relaxing this boundary.

**Every skipped entry is in one of two classes:**

1. **Listed exclusion.** A rule in the exclusion policy omitted the entry on purpose. It does not make discovery partial. The policy covers VCS metadata (`.git`, `.hg`, `.svn`), OS metadata files (`.DS_Store`), dependency and build caches (`node_modules`, `vendor`, `.venv`, `venv`, `__pycache__`), virtual OS trees, and known OS aliases encountered *during* traversal (macOS `/etc`, `/tmp`, `/var`, `/System/Volumes/Data`). Listed exclusions are summarized by rule, with a count, three examples and an omitted-example count. *(The bounded summary is accepted, from the former ADR-0001.)*
2. **Coverage gap.** Any other skipped link (including skill-manager links), unreadable path, limit (entries, depth, packages), cancellation or deadline, rejected explicit root, or root not started. Each gap names its location, and discovery is reported as partial. *(Accepted: "an uninspected skill-manager link remains a coverage gap.")*

**Explicit roots are never excluded silently.** Explicitly selecting a dependency directory, such as `--root /project/node_modules`, overrides its routine dependency-directory exclusion. Protected OS exclusions and the no-follow rule still apply, with a diagnostic identifying any rejected selected root. A root the user selected under a symlinked component (for example macOS `/tmp/x`) is reported as a link-path gap that suggests the resolved path. It is not reported as a policy exclusion.

**Discovery and package completeness are separate.** Routine dependency exclusions can leave discovery complete under its declared policy; they do not establish that the excluded locations contain no skills. Skipping relevant content inside a discovered package makes that package measurement incomplete, so it cannot establish an exact match or a baseline anchor. Declared fingerprint metadata exclusions, such as `.git`, remain outside the fingerprint and do not make it incomplete.

**Where checks run.** Exclusion rules apply to directories, links and named metadata files. Ordinary files are not checked against subtree rules.

**State-location exclusion (accepted 2026-10-29).** The designated state location is explicitly excluded and reported, as specified by the [ADR-0004 state-location amendment](0004-observations-and-baselines.md). It must be outside every skill package; this does not authorize silently excluding unrelated content. Implementation remains to be reviewed.

**Policy identity.** `DiscoveryPolicy` names the exclusion rule set. A change to the rules, rule keys or gap classification needs a new identifier, under the freeze rule in [0008](0008-versioned-data-contracts.md).

## Consequences

- A device or home inventory reports bounded exclusion evidence even with tens of thousands of `.DS_Store` files.
- A device inventory almost always has some gaps. Baselines still record fully measured packages ([0004](0004-observations-and-baselines.md) B2/B4).
- Adding a new exclusion rule is a policy change, not a bug fix.

## Acceptance scenarios

| ID | Scenario | Required result |
| --- | --- | --- |
| D1 | `.git` and `node_modules` in a project | Listed exclusions, summarized by rule; discovery not partial. |
| D2 | `~/.claude/skills/foo` is a link to a manager directory | Coverage gap naming the link; discovery partial. |
| D3 | `inventory --root /tmp/x` on macOS | Link-path gap that suggests `/private/tmp/x`; not `excluded_root`. |
| D4 | `/tmp` reached while walking `/` on macOS | Listed exclusion under a `system/tmp` rule key (no double slash). |
| D5 | Entry limit reached mid-walk | Partial; `entry_limit` diagnostic; every root not started is listed with the cause. |
| D6 | `--root ~/Projects --root ~/projects/app` on a case-insensitive volume | `app` walked once. On a case-sensitive volume, distinct directories are not merged. |
| D7 | Native Windows junction to a skills directory | Not followed; reported as a coverage gap. |
| D8 | 50,000 `.DS_Store` files | One rule summary with a count and three examples. |
| D9 | `inventory` with no scope options | Same device roots, exclusions and budgets as `inventory --device`; explicit `--root` alone stays limited to the selected roots. *(accepted)* |
| D10 | Explicit `--root /project/node_modules` | Routine dependency exclusion is overridden for the selected root; protected OS exclusions and no-follow checks still apply with visible diagnostics. |
| D11 | Relevant content inside a discovered skill cannot be measured | Package measurement is incomplete; no exact match or baseline anchor, regardless of discovery completeness. |
| D12 | Only declared fingerprint metadata, such as `.git`, is omitted inside a package | Metadata omission alone does not make the package fingerprint incomplete. |

## Implementation gaps (at 5fed934)

| Area | State |
| --- | --- |
| D1, D4, D8 | Rule-key summaries implemented (207a4f5, 5fed934). |
| D3 | Implemented in 5fed934 ("explicit macOS alias roots reach no-follow diagnostics"). |
| D6 | Case-insensitive path covered on macOS. The case-sensitive test is skipped on this volume, so Linux validation is pending. |
| D7 | **Gap.** The walk gate checks only `os.ModeSymlink` (`internal/workflow/inventory.go:167`). With Go 1.23 on Windows, junctions report `ModeIrregular`, so they would be skipped without a diagnostic. Found by source inspection only; there is no native Windows run. |
| D9 | **Gap.** The CLI currently refuses inventory without `--root` or `--device`; default-device scope selection is accepted but not shipped. |
| D10–D12 | Accepted during this review; implementation and fixture coverage have not been re-verified in this pass. |
| Policy identity | `skill-audit/discovery/v3` changed behavior in place across 207a4f5 and 5fed934. That is allowed because v3 is unpublished ([0008](0008-versioned-data-contracts.md)). It must be frozen before publication. |

## Isolated rewrite evidence (2026-10-29)

Implementation revision `b7257a55c479501179e9a936f0d7cfffea11e751` on `codex/adr-rewrite`, separate from canonical main. This supplements the dated gap table above; it does not change the accepted decision.

Fixture tests cover D1/D2/D3/D5/D6/D8–D12, including injected device defaults, explicit dependency-root selection, case-alias identity confirmation, bounded examples and separate package completeness. D8 exercises eight exclusions, not a 50,000-entry workload. D4 OS-alias keys are source checked. The Windows reparse-attribute gate compiles; D7 still needs native junction execution. Exact state-file exclusion uses inventory --state PATH, rejects directories and does not exclude a parent subtree.

See [rewrite comparison](../rewrite-comparison.md) and [verification](../../VERIFICATION.md) for exact commands and limits.
