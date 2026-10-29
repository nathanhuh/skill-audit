# Accepted scenario tests

Checked at `ce92175af520df9c8fb80e82195aadc81e5df5ce` on canonical main. The machine-readable map is [scenario-tests.json](scenario-tests.json). `TestAcceptedADRScenariosHaveTests` checks accepted ADR rows, index ranges and actual Go test declarations; missing mappings and renamed/deleted tests fail `go test`. `scripts/verify-scenarios.py` joins this map with Go JSON test events, including subtests.

This table records executed fixture assertions, not a blanket support claim. D7/P3/P4 have native Windows tests that compile but were not run here. P3 is expected to fail until the existing descendant-cleanup gap is implemented. P1 still needs a second native host; O1 needs a case-sensitive volume.

| ID | Go tests | Evidence on this host |
| --- | --- | --- |
| B1 | [TestD1D2D10D11D12B1B2B5B7B10B11](../../internal/audit/contracts_test.go#L67) | Mapped assertions pass. |
| B2 | [TestD1D2D10D11D12B1B2B5B7B10B11](../../internal/audit/contracts_test.go#L67) | Mapped assertions pass. |
| B3 | [TestX1X2B3StopAndAnchor](../../internal/audit/scanner_contracts_test.go#L28) | Mapped assertions pass. |
| B4 | [TestD5AndB4](../../internal/audit/contracts_test.go#L197) | Mapped assertions pass. |
| B5 | [TestD1D2D10D11D12B1B2B5B7B10B11](../../internal/audit/contracts_test.go#L67) | Mapped assertions pass. |
| B6 | [TestB6E4E6E8V5](../../internal/audit/contracts_test.go#L118); [TestReviewB6RuntimeInterruptionSurvivesChangedRecordingContext](../../internal/audit/review_regression_test.go#L169) | Mapped assertions pass. |
| B7 | [TestB7CompletedEmptyScopeRecords](../../internal/audit/scenario_gaps_test.go#L101) | Mapped assertions pass. |
| B8 | [TestV6V7B8B9IndependentEvidence](../../internal/audit/contracts_test.go#L231) | Mapped assertions pass. |
| B9 | [TestV6V7B8B9IndependentEvidence](../../internal/audit/contracts_test.go#L231) | Mapped assertions pass. |
| B10 | [TestD1D2D10D11D12B1B2B5B7B10B11](../../internal/audit/contracts_test.go#L67) | Mapped assertions pass. |
| B11 | [TestD1D2D10D11D12B1B2B5B7B10B11](../../internal/audit/contracts_test.go#L67) | Mapped assertions pass. |
| C1 | [TestC1C2C5GroupingCacheAdmissionAndReload](../../internal/audit/acquire_contracts_test.go#L86) | Mapped assertions pass. |
| C2 | [TestC2EvictedRepositoryReloadKeepsIdentity](../../internal/audit/scenario_gaps_test.go#L113); [TestC1C2C5GroupingCacheAdmissionAndReload](../../internal/audit/acquire_contracts_test.go#L86) | Mapped assertions pass. |
| C3 | [TestC3C4ErrorClasses](../../internal/audit/acquire_contracts_test.go#L112) | Mapped assertions pass. |
| C4 | [TestC3C4ErrorClasses](../../internal/audit/acquire_contracts_test.go#L112) | Mapped assertions pass. |
| C5 | [TestC1C2C5GroupingCacheAdmissionAndReload](../../internal/audit/acquire_contracts_test.go#L86) | Mapped assertions pass. |
| C6 | [TestC6C7C8CompleteAdmissionAndSharedAttribution](../../internal/audit/acquire_contracts_test.go#L133) | Mapped assertions pass. |
| C7 | [TestC6C7C8CompleteAdmissionAndSharedAttribution](../../internal/audit/acquire_contracts_test.go#L133) | Mapped assertions pass. |
| C8 | [TestC6C7C8CompleteAdmissionAndSharedAttribution](../../internal/audit/acquire_contracts_test.go#L133) | Mapped assertions pass. |
| D1 | [TestD1D2D10D11D12B1B2B5B7B10B11](../../internal/audit/contracts_test.go#L67) | Mapped assertions pass. |
| D2 | [TestD1D2D10D11D12B1B2B5B7B10B11](../../internal/audit/contracts_test.go#L67) | Mapped assertions pass. |
| D3 | [TestD8BoundedExclusionsAndD3Links](../../internal/audit/contracts_test.go#L285); [TestReviewD3D6RejectedAliasCannotHideValidRoot](../../internal/audit/review_regression_test.go#L45) | Mapped assertions pass. |
| D4 | [TestD4ImplicitMacAliasRuleKey](../../internal/audit/scenario_gaps_test.go#L39) | Mapped assertions pass. |
| D5 | [TestD5AndB4](../../internal/audit/contracts_test.go#L197) | Mapped assertions pass. |
| D6 | [TestD6OverlappingRootsAndCaseIdentity](../../internal/audit/contracts_test.go#L262); [TestReview4DuplicateSelectedRootHasEvidence](../../internal/audit/fourth_review_test.go#L73) | Mapped assertions pass. |
| D7 | [TestD7P4NativeWindowsJunctionIsNotFollowed](../../internal/audit/scenarios_windows_test.go#L16) | Not run: native Windows. |
| D8 | [TestD8BoundedExclusionsAndD3Links](../../internal/audit/contracts_test.go#L285); [TestD8FiftyThousandMetadataExclusions](../../internal/audit/scenario_gaps_test.go#L57) | Fast summary and exact 50,000-entry fixture pass. |
| D9 | [TestT2S1S2S3S4D9](../../internal/audit/contracts_test.go#L38) | Mapped assertions pass. |
| D10 | [TestD1D2D10D11D12B1B2B5B7B10B11](../../internal/audit/contracts_test.go#L67) | Mapped assertions pass. |
| D11 | [TestD1D2D10D11D12B1B2B5B7B10B11](../../internal/audit/contracts_test.go#L67) | Mapped assertions pass. |
| D12 | [TestD1D2D10D11D12B1B2B5B7B10B11](../../internal/audit/contracts_test.go#L67) | Mapped assertions pass. |
| D13 | [TestD13ExplicitRootsBeforeInjectedDeviceRoot](../../internal/audit/second_review_test.go#L13) | Mapped assertions pass. |
| E1 | [TestE1ThroughE8PriorityTable/E1](../../internal/audit/contracts_test.go#L432) | Mapped assertions pass. |
| E2 | [TestE1ThroughE8PriorityTable/E2](../../internal/audit/contracts_test.go#L432) | Mapped assertions pass. |
| E3 | [TestE1ThroughE8PriorityTable/E3](../../internal/audit/contracts_test.go#L432) | Mapped assertions pass. |
| E4 | [TestE1ThroughE8PriorityTable/E4](../../internal/audit/contracts_test.go#L432) | Mapped assertions pass. |
| E5 | [TestE1ThroughE8PriorityTable/E5](../../internal/audit/contracts_test.go#L432) | Mapped assertions pass. |
| E6 | [TestE1ThroughE8PriorityTable/E6](../../internal/audit/contracts_test.go#L432) | Mapped assertions pass. |
| E7 | [TestE1ThroughE8PriorityTable/E7](../../internal/audit/contracts_test.go#L432) | Mapped assertions pass. |
| E8 | [TestE1ThroughE8PriorityTable/E8](../../internal/audit/contracts_test.go#L432) | Mapped assertions pass. |
| E9 | [TestE9TextSnapshotStdout](../../cmd/skill-audit/main_test.go#L322) | Mapped assertions pass. |
| I1 | [TestI1GapDetailCapPreservesReadablePartialInventory](../../internal/audit/inventory_budget_test.go#L13); [TestI1TruncationEvidenceSurvivesAuditAndStoredObservation](../../internal/audit/inventory_budget_test.go#L154) | Mapped assertions pass. |
| I2 | [TestI2FileManifestIsNeverPartiallyAdmitted](../../internal/audit/inventory_budget_test.go#L136); [TestI2I4WholePackageAdmissionStopsWithinFormattedBound](../../internal/audit/inventory_budget_test.go#L53) | Mapped assertions pass. |
| I3 | [TestI3MandatoryEnvelopeFailureNeverWrites](../../internal/audit/inventory_budget_test.go#L74) | Mapped assertions pass. |
| I4 | [TestI2I4WholePackageAdmissionStopsWithinFormattedBound](../../internal/audit/inventory_budget_test.go#L53) | Mapped assertions pass. |
| O1 | [TestO1DistinctCaseOnlyPathsNeedFilesystemIdentity](../../internal/audit/review_regression_test.go#L143) | Skipped: case-sensitive volume required. |
| O2 | [TestO2ConfirmedAliasesStillRefused](../../internal/audit/review_regression_test.go#L158) | Mapped assertions pass. |
| O3 | [TestO3PlannedCaseAliasesCannotOverwriteCommittedState](../../cmd/skill-audit/main_test.go#L289) | Mapped assertions pass. |
| P1 | [TestFingerprintsFrozenDomains](../../internal/audit/contracts_test.go#L346) | Pinned golden bytes pass locally; second-host proof pending. |
| P2 | [TestP2LocalRuleAndWorkflow](../../internal/audit/contracts_test.go#L143) | Mapped assertions pass. |
| P3 | [TestP3NativeWindowsCancellationStopsDescendants](../../internal/audit/scenarios_windows_test.go#L35) | Not run: native Windows. |
| P4 | [TestD7P4NativeWindowsJunctionIsNotFollowed](../../internal/audit/scenarios_windows_test.go#L16) | Not run: native Windows. |
| P5 | [TestP5HostFilenamesRecordReadAndAppend](../../internal/audit/host_paths_test.go#L23); [TestP5HostFilenamesStageAndKeepFindingPaths](../../internal/audit/host_paths_test.go#L63); [TestP5LocalPathsRejectTraversalAndUnsafeHostForms](../../internal/audit/host_paths_test.go#L97); [TestP5NonUTF8FilenameCannotBecomeAnchor](../../internal/audit/host_paths_test.go#L139) | Mapped assertions pass; unsupported byte-name fixture skipped. |
| Q1 | [TestQ1Q2Q3SearchProvenanceRoundtrip](../../internal/audit/third_review_test.go#L248); [TestSearchSyncSavedAndExplicitScopesCLI](../../cmd/skill-audit/main_test.go#L181) | Mapped assertions pass. |
| Q2 | [TestQ1Q2Q3SearchProvenanceRoundtrip](../../internal/audit/third_review_test.go#L248); [TestSearchSyncSavedAndExplicitScopesCLI](../../cmd/skill-audit/main_test.go#L181) | Mapped assertions pass. |
| Q3 | [TestQ1Q2Q3SearchProvenanceRoundtrip](../../internal/audit/third_review_test.go#L248); [TestSearchSyncSavedAndExplicitScopesCLI](../../cmd/skill-audit/main_test.go#L181) | Mapped assertions pass. |
| Q4 | [TestQ4PerQueryProvenance](../../internal/audit/fourth_review_test.go#L225) | Mapped assertions pass. |
| R1 | [TestR1R2SA001ReceiverOptions](../../internal/audit/second_review_test.go#L132) | Mapped assertions pass. |
| R2 | [TestR1R2SA001ReceiverOptions](../../internal/audit/second_review_test.go#L132) | Mapped assertions pass. |
| R3 | [TestR3V2BaselineCannotEstablishNewV3Finding](../../internal/audit/second_review_test.go#L152) | Mapped assertions pass. |
| R4 | [TestR4SA001LoneDash](../../internal/audit/third_review_test.go#L231) | Mapped assertions pass. |
| S1 | [TestT2S1S2S3S4D9](../../internal/audit/contracts_test.go#L38) | Mapped assertions pass. |
| S2 | [TestT2S1S2S3S4D9](../../internal/audit/contracts_test.go#L38) | Mapped assertions pass. |
| S3 | [TestT2S1S2S3S4D9](../../internal/audit/contracts_test.go#L38) | Mapped assertions pass. |
| S4 | [TestT2S1S2S3S4D9](../../internal/audit/contracts_test.go#L38) | Mapped assertions pass. |
| T1 | [TestT1T4OfflineAuditWithoutRequestsOrCredentials](../../internal/audit/scenario_gaps_test.go#L16) | Mapped assertions pass. |
| T2 | [TestT2NeverExecuteSkillData](../../internal/audit/contracts_test.go#L332) | Mapped assertions pass. |
| T3 | [TestT3T4RealProcessStagingAndSARIF](../../internal/audit/scanner_contracts_test.go#L166) | Mapped assertions pass. |
| T4 | [TestT1T4OfflineAuditWithoutRequestsOrCredentials](../../internal/audit/scenario_gaps_test.go#L16); [TestT3T4RealProcessStagingAndSARIF](../../internal/audit/scanner_contracts_test.go#L166); [TestT1T4AcquisitionRejectsUnsafeAndAttributionFailure](../../internal/audit/acquire_contracts_test.go#L189) | Mapped assertions pass. |
| T5 | [TestT5NameOptInBeforeRequests](../../internal/audit/acquire_contracts_test.go#L215) | Mapped assertions pass. |
| V1 | [TestV1V2V3V4CLI](../../cmd/skill-audit/main_test.go#L92) | Mapped assertions pass. |
| V2 | [TestV1V2V3V4CLI](../../cmd/skill-audit/main_test.go#L92) | Mapped assertions pass. |
| V3 | [TestV1V2V3V4CLI](../../cmd/skill-audit/main_test.go#L92) | Mapped assertions pass. |
| V4 | [TestV1V2V3V4CLI](../../cmd/skill-audit/main_test.go#L92) | Mapped assertions pass. |
| V5 | [TestB6E4E6E8V5](../../internal/audit/contracts_test.go#L118) | Mapped assertions pass. |
| V6 | [TestV6V7B8B9IndependentEvidence](../../internal/audit/contracts_test.go#L231) | Mapped assertions pass. |
| V7 | [TestV6V7B8B9IndependentEvidence](../../internal/audit/contracts_test.go#L231) | Mapped assertions pass. |
| X1 | [TestX1X2B3StopAndAnchor](../../internal/audit/scanner_contracts_test.go#L28) | Mapped assertions pass. |
| X2 | [TestX1X2B3StopAndAnchor](../../internal/audit/scanner_contracts_test.go#L28) | Mapped assertions pass. |
| X3 | [TestX3X4VisibleBudget](../../internal/audit/scanner_contracts_test.go#L71) | 60-second phase admission checked through the deadline seam; no 45-second real scan. |
| X4 | [TestX4DefaultProbeAllowsColdStart](../../internal/audit/scenario_gaps_test.go#L78) | Default probe deadline exceeds 6 seconds; no real cold scanner startup. |
| X5 | [TestX5CausePrecedence](../../internal/audit/contracts_test.go#L222) | Mapped assertions pass. |
| X6 | [TestX6SharedProbeDeadlineAndX8NoScanRetry](../../internal/audit/scanner_contracts_test.go#L98) | Mapped assertions pass. |
| X7 | [TestX7LaunchOnceAndX9Reserve](../../internal/audit/scanner_contracts_test.go#L127) | Mapped assertions pass. |
| X8 | [TestX6SharedProbeDeadlineAndX8NoScanRetry](../../internal/audit/scanner_contracts_test.go#L98) | Mapped assertions pass. |
| X9 | [TestX7LaunchOnceAndX9Reserve](../../internal/audit/scanner_contracts_test.go#L127) | Mapped assertions pass. |
| X10 | [TestX10OperationTimeoutRefusesRecording](../../internal/audit/scanner_contracts_test.go#L156) | Mapped assertions pass. |

The 50,000-entry fixture is opt-in for ordinary development: `SKILL_AUDIT_LARGE_FIXTURES=1 go test ./internal/audit -run ^TestD8FiftyThousandMetadataExclusions$ -count=1`. It was run separately here and passed. All packages/files used by tests are disposable fixtures. Source links and runtime results are dated evidence; the JSON map and guard are the maintained contract.
