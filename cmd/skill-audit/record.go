package main

import (
	"errors"

	"skill-audit/internal/audit"
)

// The outcome comes from the write operation, never from input diagnostics.
func recordingDiagnostic(path string, err error) (audit.Diagnostic, bool) {
	d := audit.Diagnostic{Path: path, Code: "state_not_recorded", Message: err.Error(), Incomplete: true}
	var committed *audit.CommittedWriteError
	if errors.As(err, &committed) {
		d.Code = "state_recorded_durability_unconfirmed"
		return d, false
	}
	return d, true
}
