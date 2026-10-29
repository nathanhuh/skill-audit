---
status: accepted
date: 2026-10-29
source: review of codex/adr-rewrite 7a84ccc, issue 9
---

# 0011. Output and input path identity

## Decision (accepted 2026-10-29)

Reject aliases by exact cleaned absolute path or confirmed filesystem identity (`os.SameFile`), rather than case-folded spelling alone. On a case-sensitive filesystem, existing `Report.json` and `report.json` may be different files and should be usable in different roles.

- Retain no-follow checks and package-placement protection.
- Existing hardlinks and case aliases confirmed by the filesystem remain collisions.
- A nonexistent output with a distinct lexical spelling is permitted. Revalidate all roles before every write; if an earlier state write creates an alias on a case-insensitive filesystem, reject the subsequent report write rather than replace state. Preserve and report the actual committed recording outcome.
- This is best-effort ordinary-filesystem protection, not a guarantee against hostile concurrent mutation. Native Windows behavior remains subject to ADR-0009 runtime validation.

Alternative: retain unconditional case-folded rejection, including distinct files on case-sensitive filesystems. Document it as a conservative cross-platform restriction.

## Acceptance scenarios

| ID | Scenario | Required result  |
| --- | --- | --- |
| O1 | Two distinct case-only files on a case-sensitive filesystem | Allow; neither role is silently overwritten. |
| O2 | Case aliases or hardlinks to the same existing file | Refuse before replacement. |
| O3 | Planned destinations become aliases after a committed state write | Refuse report replacement; preserve committed state and its outcome. |

Accepted by the user on 2026-10-29 in this review thread. This supersedes README's unconditional case-only-alias refusal statement.
