# Skill observations

The language of local skill inventory, content correlation, observed lineage and static security findings.

## Language

**Skill package**:
A local collection identified by a SKILL.md manifest and its bundled content. Presence does not establish that an agent loads or executes it.
_Avoid_: Active skill, running skill

**Local identity**:
The combination of installation scope and local package path that distinguishes an observed skill package. A declared name alone is not an identity.
_Avoid_: Skill name as identity

**Installation ownership**:
The global or project installation that owns a skill package, including packages nested inside its plugin layout. An incidental nested agent-directory name does not transfer ownership.
_Avoid_: Nearest folder determines ownership

**Observation**:
Evidence collected during one selected-scope run, including measured packages, findings, exclusions and coverage gaps. It can preserve useful evidence despite partial coverage.
_Avoid_: Complete endpoint truth

**Package measurement**:
An observation of a skill package's content and its fingerprint. Only a complete measurement can establish a content comparison anchor.
_Avoid_: Complete measurement when content was skipped

**Discovery coverage**:
The extent to which the selected locations were examined for skill packages. It is distinct from the completeness of an individual package measurement.
_Avoid_: Scan completeness without qualification

**Detection coverage**:
The extent to which the selected scanners evaluated the observed content. Complete content measurements do not imply complete detection coverage.
_Avoid_: Clean when detection was unavailable

**Exclusion policy**:
The declared rules defining locations or content intentionally omitted from an observation. Known exclusions remain visible; unknown skipped skill targets remain coverage gaps.
_Avoid_: Unreported skip

**Comparison anchor**:
A complete package measurement selected as the reference for later content comparisons. An absent or incomplete measurement provides no usable anchor.
_Avoid_: Inferred prior content

**Baseline**:
The initial recorded set of usable comparison anchors together with observation scope and coverage evidence. It does not certify exhaustive discovery or safety.
_Avoid_: Clean-device certificate, full-device baseline without coverage evidence

**Snapshot**:
A subsequent recorded observation that preserves the existing baseline. A later complete measurement does not silently replace an anchor.
_Avoid_: Automatic baseline refresh

**Finding**:
A scanner-attributed observation of a security-relevant pattern, with evidence and limitations. It does not establish malicious intent or observed execution.
_Avoid_: Proven attack

**Recording outcome**:
Whether a requested observation was recorded, not recorded, or committed with uncertain durability; it can also be not requested. It is independent of findings and coverage.
_Avoid_: Exit code as proof that recording succeeded

**Correlation evidence**:
Content evidence connecting a local skill package to a public reference. It does not prove authorship, who copied it, or publisher approval.
_Avoid_: Proven origin from a name or similarity score
