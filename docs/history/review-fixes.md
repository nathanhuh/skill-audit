# Review fixes for codex/adr-rewrite

Reviewed starting revision: `7a84ccc0a4d2c910606e73239c3b343db96ed98c`, clean worktree. Fixed implementation: `f9200ee07f9a0f071c788a11de7467903ea44930`. The final documentation commit changes no production Go. Canonical `main` remains `8c6f3b83d725c716246bdbdecc8495bb9cedbccb` and was not edited, switched, merged, pushed or rewritten.

All ten review items are addressed. Issues 1–3 first failed new regressions before their fixes. Issues 8 and 9 were implemented only after the user explicitly accepted [ADR-0010](adr/0010-inventory-output-budget.md) and [ADR-0011](adr/0011-output-path-identity.md) in this thread. This supplements the dated original rewrite comparison; it does not turn its earlier gap assessment into a current audit result.

| Review item | Contract | Change and evidence |
| --- | --- | --- |
| 1. Imported interruption names veto recording | ADR-0005 E8 / ADR-0004 B6 | Imported inventory/catalog diagnostics stay evidence. Recording uses the operation context and a private runtime interruption fact, not diagnostic codes. Eight injected-name cases now record complete package anchors; genuine interruption still refuses recording even with a replacement recording context. Compiled CLI catalog fixture records and exits 2 for coverage, not 3. |
| 2. Explicit metadata root silently excluded | ADR-0003 explicit-root rule / D10 | Every rejected explicit root gets `excluded_root` and partial coverage. Routine metadata omission remains valid during traversal. Explicit `.git` fixture now saves partial inventory and exits 2. |
| 3. Rejected link root hides valid coverage | ADR-0003 D3/D6 | Only no-follow-validated, admitted directory roots participate in delegation/identity deduplication. The real macOS `/tmp/.../x` versus `/private/tmp/...` compiled fixture discovers the package through the valid root and retains the alias gap. |
| 4. Console cap becomes global launch failure | ADR-0006 stop rule 3 / interface bounds | Keep actual process exit and a typed console-limit cause; preserve valid SARIF and mark that package partial. Later packages still launch. Already-exited processes are also distinct from launch failures. A real copied Go fake scanner floods both streams on two package scans. Replacing the embedded bytes.Buffer also removes its promoted ReadFrom method, which otherwise lets io.Copy bypass Write's cap. |
| 5. Requested state path lacks discovery exclusion | ADR-0004 B10 / ADR-0003 state location | Audit preserves the inventory's actual designated path. A different state path within the selected scope produces `state_exclusion_unconfirmed`; recording is refused with guidance to refresh inventory using that path. Exclusion is not fabricated retroactively. Refreshed scope then records successfully. Filesystem-confirmed ancestor identity also detects case-alias scope membership. |
| 6. TMPDIR can point inside installed content | ADR-0006 staging interface / B11 | Before any probe or staging write, validate the resolved temp base against every audited package and existing SKILL.md ancestors. Refuse unsafe configuration with an explicit scanner cause; no temporary scanner files or probe are created there. The fake process launch counter stays zero and the fixture temp directory remains empty. |
| 7. Case variants revisit GitHub repository | ADR-0007 C5 | Group repository names case-insensitively, with canonical lowercase acquisition keys. Ref/path spelling remains case-sensitive. Explicit `Anthropics/Skills` plus `anthropics/skills` hit makes one tree request and one retained import. Existing catalog/state bytes are not normalized or migrated. |
| 8. Oversized inventory loses saved evidence | Accepted ADR-0010 I1–I4 | Admit whole measurements under the actual 64 MiB formatted output bound; reserve coverage headroom. Cap gap details at 1 MiB / 1,000 records and retain explicit omitted counts. Save a stopping location and partial coverage. Mandatory-envelope failure is explicit and cannot write a false saved inventory. Truncation evidence survives audit and stored history. |
| 9. Case-only distinct files rejected | Accepted ADR-0011 O1–O3 | Exact lexical equality or confirmed SameFile identity rejects aliases; spelling alone does not. Writers revalidate before mutation, so a planned report alias cannot replace committed state. Hardlink and planned-alias tests pass here. O1's distinct-case-files runtime test is skipped on this case-insensitive volume and remains a native case-sensitive-filesystem check. |
| 10. Repeated whole-catalog encoding | ADR-0007 C7/C8 size guarantee, efficiency | Count each new entry, unique attribution item and diagnostic at its actual formatted depth; never re-encode retained catalog content for admission. Final writing keeps its exact-size backstop. Accounting tests equal actual formatted JSON after every mixed addition, including escaped/Unicode keys and binary data. |

## Verification

Native host: macOS arm64, Go 1.23.5. **103 tests/subtests passed**, no failures; one runtime skip: `TestO1DistinctCaseOnlyPathsNeedFilesystemIdentity` requires a case-sensitive filesystem. Windows-only tests were compiled, not executed. All default tests are offline and use temporary fixtures, fake transports and a copied Go test executable. No skill script was executed, real installed skill modified, credential/service required, scanner installed, or content-derived URL fetched.

Passed: scoped gofmt, `go vet ./...`, offline tests, native build, fixture demo, 10 compiled review checks, the shared 17-command-per-binary / 14-shell-sample harness, and six CLI checks using files from the pinned published revision. Linux/Windows builds and Windows test compilation pass; native runtime evidence is still pending.

Production-bound inventory fixture: **66,010,508 bytes saved**, **937 whole package measurements retained** from 1,000 candidates with 70,000-byte names. Output stays below 67,108,864 bytes, with an explicit stopping location. A separate 3,000-link fixture saves bounded detail and omitted counts. Smaller injected limits exercise file-manifest admission, diagnostic byte/count caps, mandatory-envelope failure and exact accounting.

Incremental admission benchmark, 16 KiB raw data per new entry, 200 ms benchmark interval (descriptive macOS measurements):

| New entries | Time/op | Allocated bytes/op |
| --- | ---: | ---: |
| 8 | 0.710 ms | 600,840 |
| 16 | 1.390 ms | 1,197,930 |
| 32 | 2.781 ms | 2,398,791 |

The measured path scales approximately linearly. This is not a general endpoint performance claim or a before/after whole-command benchmark.

Full machine summary: [review-fixes-results.json](review-fixes-results.json). Temporary command/report evidence: `/private/tmp/skill-audit-review-smoke`, `/private/tmp/skill-audit-review-shared`, `/private/tmp/skill-audit-review-tests.jsonl`, `/private/tmp/skill-audit-review-benchmark.txt`. Exact commands are recorded at the top of VERIFICATION.md.

## Compatibility and limits

No published schema, fingerprint domain, SA001 version or external scanner pin changed. Inventory/v3 and report/v2 gain optional truncation/coverage evidence while still unpublished, as allowed by ADR-0008. Older published inputs remain readable and legacy state append remains refused without migration. Vendored parsers, catalogs, licenses/notices, go.mod/go.sum and .gitattributes are unchanged.

Native Linux/Windows, Windows junctions/descendant cleanup/NTFS durability, real Skill X-Ray/OpenGrep, live authenticated acquisition and exhaustive device discovery remain unverified. Filesystem checks remain best effort against ordinary changes, not hostile mutation. No publication or main promotion occurred.
