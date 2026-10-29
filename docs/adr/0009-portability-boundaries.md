---
status: accepted
date: 2026-10-29
source: PORTABILITY.md, AGENTS.md; accepted by the user in ADR review on 2026-10-29 with filesystem-discovery boundary and cross-host evidence corrections
---

# 0009. Portability boundaries

## Context

Linux and native Windows are future targets. Only macOS arm64 has runtime evidence. PORTABILITY.md holds the detailed checklist. This ADR states the rules the checklist follows.

## Decision

- **OS-specific code is isolated at filesystem and child-process boundaries.** The filesystem boundary includes discovery (drive enumeration, path handling and link/reparse-point detection) as well as persistence (`atomic_sync_*`); child-process control remains isolated in `process_*`. The core stays portable. No platform framework.
- **Evidence levels are kept distinct.** A cross-build proves compilation only. WSL is not native Windows. A platform is claimed as supported only after native tests, the demo and the PORTABILITY.md runtime plan have passed on it.
- **Bytes are never normalized.** Hashes are taken over raw bytes, so CRLF differs from LF. `.gitattributes` disables text conversion for the pinned catalog.
- **Durability claims follow the platform.** Windows skips directory fsync and makes no crash-durability promise. Its replacement durability is unverified until tested on NTFS.
- **Links include platform equivalents.** On Windows, junctions and reparse points fall under [0003](0003-discovery-and-exclusion-policy.md)'s no-follow rule.
- **Process cleanup is not assumed.** Unix process groups do not cover Windows. Job Objects remain pending.
- Local identities are host-specific absolute paths. State does not move between operating systems.

## Acceptance scenarios

| ID | Scenario | Required result |
| --- | --- | --- |
| P1 | Identical fixture bytes and slash-relative file paths on two hosts | Identical file and directory hashes. |
| P2 | Same fixture converted to CRLF | Exact hashes change; similarity may still match. |
| P3 | Native Windows scanner cancellation | No descendant process survives. |
| P4 | Native Windows junction in a skills directory | Not followed and reported as a coverage gap (= D7). |

## Implementation gaps (at 5fed934)

| Area | State |
| --- | --- |
| P1 | **Pending cross-host validation.** Single-host macOS checks establish local hashing behavior, not agreement between hosts. Compare identical package bytes and relative paths on another target platform before claiming P1 is verified. |
| P2 | LF/CRLF hashing behavior has macOS fixture evidence; no additional runtime checks were performed during this documentation review. |
| P3–P4 | Not implemented or verified on native Windows; see PORTABILITY.md "Work before claiming native Windows support" and [ADR-0003](0003-discovery-and-exclusion-policy.md) D7. |
| Filesystem boundary | The expanded boundary permits platform-specific discovery handling; it does not imply that Windows drive, path or reparse-point behavior has been validated. |

## Isolated rewrite evidence (2026-10-29)

Implementation revision `b7257a55c479501179e9a936f0d7cfffea11e751` on `codex/adr-rewrite`, separate from canonical main. This supplements the dated gap table above; it does not change the accepted decision.

Filesystem policy/discovery/link/persistence and process behavior are isolated in fs_* and process_* files. LF/CRLF and same-host fixture hashes pass; P1 cross-host agreement is still pending. Linux/Windows cross-builds and Windows test compilation pass. P3 Windows descendant cleanup, P4/D7 native junctions, NTFS durability, native Linux/Windows full tests/demo and a Windows-native demo remain pending. WSL is not native Windows evidence.

See [rewrite comparison](../rewrite-comparison.md) and [verification](../../VERIFICATION.md) for exact commands and limits.
