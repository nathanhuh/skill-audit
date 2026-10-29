# Host-local filenames and accepted scenario tests

Focused scope requested after the review of clean main at **70316378c3407b62839d9c13c2146ef3818662dd**: fix the unreadable-baseline filename bug and ensure every accepted scenario ID has a test. Verified implementation: **ce92175af520df9c8fb80e82195aadc81e5df5ce**. Other findings from that review, including registry completeness, per-file SA001 comparability and unrelated cleanup, were left outside this focused change.

## Filename rule and fix

ADR-0009 P5 records the chosen host-local rule. Local manifests use canonical slash-relative UTF-8 names accepted by the host boundary, including colon and literal backslash on Unix. Absolute/volume paths, dot/dot-dot/empty components, NUL and traversal are rejected. Existing non-UTF-8 coverage gaps remain because JSON/fingerprints cannot represent those names losslessly. Local state remains host-specific. Acquisition/catalog transport paths retain their existing separate portable rule.

Measurement, state reading, scanner staging and decoded local SARIF finding paths now share LocalRelative. Content bytes, hash domains, schemas, scanner/API pins and dependencies are unchanged. There is no migration or baseline reset. A valid old baseline containing these Unix names can now be read and compared without rewriting it.

Before the fix, new regressions failed for a:b.md and a literal-backslash filename: recording succeeded, state readback failed, and staging returned unsafe staged path. After the fix, tests record/read/compare/append both names, stage their exact bytes and retain scanner finding paths. Traversal is rejected by both state reading and staging. The Unix byte-invalid-name fixture is skipped on this Mac because the filesystem rejects its creation; the pure validator checks still reject that representation.

The ten-command compiled CLI check builds the reviewed revision in a temporary export, reproduces its recorded-but-unreadable state, and proves that the fixed binary reads the same file without changing it. It appends readable unchanged/changed snapshots, confirms the modified colon filename, and stages the package through a trusted Go fake scanner. Skill scripts are never run.

## Scenario coverage

All **87 accepted IDs** have registered Go tests, including new P5. The [readable map](adr/scenario-tests.md) links each ID to source assertions; [scenario-tests.json](adr/scenario-tests.json) is the machine contract. A normal Go test verifies that the ADR index, declared scenario rows and real test declarations agree. The result checker also verifies that mapped subtests executed or have an explicit native-only requirement.

Added coverage includes offline/no-credential audit evidence (T1/T4), the macOS alias rule key (D4), exact 50,000-entry exclusion cardinality (D8), genuinely empty readable baselines (B7), default cold-probe admission (X4), repository reload identity/accounting (C2), text snapshot stdout (E9), and native Windows junction/descendant acceptance tests (D7/P4/P3). X8 now explicitly asserts one package scan launch. Existing accepted amendment IDs D13/R4/Q4 have formal table rows; their policy is unchanged.

Default suite: **192 passing tests/subtests, zero failures, three explicit skips**. The separate D8 large fixture passes in 20.71 seconds, so its default skip is covered by that run. The remaining host skips are case-sensitive O1 and a byte-invalid filename the filesystem cannot create. Native Windows D7/P3/P4 tests are present and compile but are not runtime-verified here. P3 intentionally detects the known direct-child-only cancellation gap; native execution is expected to fail until Job Objects are implemented. P1 has pinned golden-byte checks on this host, not a completed two-host experiment.

Host and Windows vet, scoped gofmt, native/Linux/Windows builds, Windows test compilation, temporary-fixture demo and six published-format CLI checks pass. See VERIFICATION.md and [compact results](host-paths-results.json). No scanner install, live provider, real device scan, installed-skill change, push or submission occurred.
