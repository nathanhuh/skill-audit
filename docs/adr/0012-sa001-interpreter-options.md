---
status: accepted
date: 2026-10-29
source: second review of codex/adr-rewrite d2242e0, item 6
---

# 0012. SA001 shell interpreter options

## Decision (accepted 2026-10-29)

Extend the literal `sh`/`bash` receiver subset to flag-only `-e`, `-u`, `-x`, `-v` and `-s`, including grouped forms such as `-eux` and `-se`. These flags do not select a command string or script filename. Keep ordered `--` handling and filename semantics: `bash -- -s` does not qualify; `bash -s -- argument` does. Command mode `-c`, value-taking options (`-o`, `-O`), long options, startup files and unknown flags remain outside this rule. The parse-only flag `-n` remains excluded. This remains syntax evidence without reachability/execution claims.

Rejected alternative: keep the current receiver subset (`-s` only), preserving SA001 v2 unchanged.

This changes published SA001 behavior. Bump the rule from v2 to v3 and the evaluation identity to `SA001/v3;sh/v3.10.0;scope/v2`. Previous v2 evaluations cannot justify `new` transitions, while compatible content comparisons remain available (ADR-0008 V7). Existing observations and rule-version findings remain unchanged.

## Acceptance scenarios

| ID | Scenario | Required result |
| --- | --- | --- |
| R1 | curl HTTP(S) into bash -e / sh -ux / bash -se | Finding under the new rule/evaluation version. |
| R2 | Receiver has -c, -o value, unknown flags or filename without -s | No finding from this bounded rule. |
| R3 | v2 baseline versus new option coverage | Content comparison works; no comparable new finding claim. |
| R4 | Lone-dash receiver option termination | Stdin forms qualify; subsequent filenames require an earlier -s; a dash after -- remains a filename. |


Accepted explicitly by the user in this review thread on 2026-10-29.

## Second-review implementation evidence (2026-10-29)

Verified source `a59cddbc4c6b17940a249579131e71c31db95e5c`. R1–R3 pass: grouped supported flags qualify, excluded command/parse/value modes do not, and v2 evaluations cannot establish new v3 findings. See [second-review record](../history/review-2-fixes.md).

## Accepted amendment: lone-dash option termination (2026-10-29)

The user explicitly approved this amendment in the third review thread. Before options have ended, a lone `-` ends receiver options just like `--`; it does not select a script. Thus `sh -`, `bash -e -` and `bash -s - argument` qualify. A subsequent filename without an earlier `-s` does not qualify; `sh -- -` is also a filename form and remains outside this subset. This adds R4: test both stdin and filename forms. SA001 v3/evaluation remains unpublished, so it changes in place under ADR-0008 rule 1. Published v2 remains unchanged.

## Third-review evidence (2026-10-29)

Implementation `77deb5b2eeceda97f5fa22614abf6f3c6e5acc7e` on canonical main. R4 passes for sh -, bash -e -, grouped supported flags with a lone dash, and excluded following filenames and a dash after --. The compiled CLI emits SA001 v3 for curl HTTP(S) piped to sh -. See [third-review record](../history/review-3-fixes.md) and [verification](../../VERIFICATION.md).

## Later amendment

[ADR-0017](0017-finding-identity.md) (2026-10-30) keeps SA001 v3 detection unchanged and bumps the evaluation identity to `SA001/v3;sh/v3.10.0;scope/v2;identity/v2`, so findings compare by a fingerprint of the full matched command instead of the 240-byte evidence.
