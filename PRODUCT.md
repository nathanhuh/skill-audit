# Product direction: endpoint skill visibility for security teams

## Customer outcome and current scope

Help a security team answer: which skill packages exist on an endpoint, which public references they resemble, what changed, and which evidence warrants investigation? Help developers explain legitimate customization without treating every unknown or modified skill as malicious.

Traceforce publicly emphasizes on-device AI visibility, tool-call context, fleet deployment and dangerous-action control. This motivates the priorities below; it does not establish an integration contract or the company's private roadmap. [Traceforce product positioning](https://www.traceforce.ai/). Skill X-Ray already supplies static skill analysis, so the distinct value here is inventory, change evidence and investigation workflow around attributed scanner results. [Skill X-Ray](https://github.com/traceforce/skill-xray).

Implemented: bounded recursive inventory, scope/path/agent hints, public candidate discovery, pinned external reference catalogs, offline static audit, explicit baseline/snapshots, SA001 and an optional Skill X-Ray adapter. These are on-demand observations. Presence is not activation; a risky pattern is not observed execution; origin similarity is not publisher identity or approval.

Everything below is a proposal, not shipped functionality. No Traceforce ingestion interface was verified, and the public documentation site was inaccessible during this review. Define an export contract locally before claiming any product integration.

## Prioritized increments

| Priority | Customer job | Small feature | Acceptance evidence |
| --- | --- | --- | --- |
| Next: trustworthy observations | IT needs to distinguish a healthy clean scan from a blind endpoint | Observation envelope with operator-supplied endpoint ID, scan ID, start/end time, OS/tool/rule/catalog versions, scan scope, budgets, scanned/skipped counts and separate collection/detection status | Inject a permission failure and scanner failure; partial evidence survives and neither becomes a clean result. Distinct endpoints cannot accidentally share lineage. |
| Next: actionable changes | An analyst needs to decide whether this scan requires action | Stable finding identity, first/last observed times, new/persistent/resolved/unknown transitions, changed-file context, and explicit comparison versions | Identical reruns create no duplicate alert; a rule change requires reassessment; an unreadable or unscanned file never resolves its previous finding. Resolution requires a comparable successful evaluation. |
| Next: company policy and exceptions | Security needs to permit approved customization without losing visibility | A small versioned policy file selecting severity thresholds and approved exact reference/content digests; narrow exceptions with owner, reason and expiry | Approved repository name alone cannot approve modified bytes. Expired exceptions stop suppressing alerts. Raw findings remain present alongside policy disposition. Unknown origin remains a separate fact. |
| Pilot: predictable deployment | IT needs a reliable collector on real endpoints | Native Linux acceptance first, then Windows; resource budgets, useful health diagnostics, documented exit codes, version reporting and repeatable scheduler invocations | Measure wall time, peak memory and files read on a fixture VM. Exercise interrupted writes, denied permissions and process cancellation under an unprivileged account. Cross-builds alone do not qualify. |
| Pilot: controlled fleet enrichment | Companies need private source protection and reproducible intelligence | Acquire catalogs once in a controlled environment, distribute versioned artifacts, show freshness and verify approved catalog checksums; support internal repositories through a separately scoped acquisition path later | Endpoints audit with network disabled; changing catalogs records a new evaluation context. Private skill names/content are not sent to public search by default. A signature later establishes artifact publisher/integrity, never safety of every included skill. |
| Pilot: investigation handoff | Analysts need findings in their existing workflow | Versioned event export with endpoint/observation/package/finding IDs, owner tags, evidence references, scanner attribution and redaction controls; add SARIF if a receiving workflow needs it | Duplicate delivery is idempotent, scanner identity survives normalization, and redacted output omits sensitive paths/evidence. Validate against an agreed receiver schema before integrating Traceforce or a SIEM. |
| Later: runtime context and response | Responders need to know whether a risky skill was actually used and what containment will do | Correlate with approved agent/process telemetry; propose a response plan with scope, expected effects, approval record and rollback | Distinguish present, configured, and observed executed. File reads alone do not prove execution. Test response effectiveness with the real agent, including already-loaded skills, before enabling containment. |

## Recommended next take-home slice

Implement the observation envelope and finding transitions first. They deepen the existing inventory → baseline → modification → audit demonstration without requiring a backend or a larger ruleset. Preserve baseline-relative file changes separately from latest-observation alert transitions: they answer different questions.

Demonstrate two fixture endpoint IDs, one repeated unchanged scan, one modified script, and one unreadable package. The expected story is one new actionable finding, no duplicate alert on the unchanged endpoint, and a visible coverage gap rather than a falsely clean result. Endpoint labels should be operator-controlled; avoid hardware serials or usernames as automatic identifiers.

Policies and export follow once observation identity and comparability are stable. Expand detector coverage only with concrete customer examples and false-positive fixtures. Preserve the existing direct rule and source-attributed X-Ray findings rather than counting duplicated reports as separate threats.

## Deployment and discovery tradeoffs

An occasional broad discovery sweep finds new projects; cheaper recurring scans can revisit known packages and configured workspace roots. Cached inventories must expire, and a new discovery sweep must eventually find newly created projects. A future filesystem watcher is an optimization, not proof that no changes were missed: event loss, restart and unsupported filesystems require reconciliation.

Current discovery has exclusions, depth/entry/package limits and a deadline. Current name-derived public search allows ten names per invocation, and requests/results are bounded. These are visible MVP constraints, not fleet completeness. A company deployment needs resumable bounded acquisition, deduplication by immutable reference, private catalog support and catalog freshness before claiming broad correlation coverage.

Keep acquisition credentials out of endpoint collection where possible. Reports may themselves contain sensitive usernames, paths and script snippets: define local retention, access control and redaction before central transport. Explicit evidence collection does not authorize uploading full skill content.

## Measures of usefulness

- Successful observations among expected endpoint check-ins; distinguish missing check-ins from scan failures and partial coverage.
- Detection delay from a controlled fixture change to an actionable report, alongside endpoint CPU/memory/I/O cost.
- Duplicate alerts per unchanged finding and analyst-confirmed false positives by rule/version.
- Time to identify the endpoint, owner, changed file, scanner and comparison basis from a report.
- Catalog age and correlation coverage against the evaluated catalog, without presenting unknown matches as attacks.

Fleet metrics require a deployment roster and aggregation outside this CLI; they are not available merely from one local report. Targets should be agreed during a pilot rather than fabricated for the take-home.

## Interview framing

“This is the inventory, change-evidence and static-detection portion of an endpoint security workflow for agent skills. Detection is independent of public enrichment. I chose observable coverage and reproducible evidence before runtime enforcement. The next product increment makes observations attributable and alerts deduplicated so a security team can act on them.”
