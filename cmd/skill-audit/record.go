package main

import (
	"errors"
	"fmt"
	"io"

	"skill-audit/internal/audit"
)

// The outcome comes from the write operation, never from input diagnostics.
// Recording diagnostics do not change discovery or detection completeness.
func recordingDiagnostic(path string, err error) (audit.Diagnostic, bool) {
	d := audit.Diagnostic{Path: path, Code: "state_not_recorded", Message: err.Error(), Incomplete: false}
	var committed *audit.CommittedWriteError
	if errors.As(err, &committed) {
		d.Code = "state_recorded_durability_unconfirmed"
		return d, false
	}
	return d, true
}

func recordingOutcome(path string, err error) *audit.Recording {
	r := &audit.Recording{Status: audit.RecordingRecorded, Path: path}
	if err != nil {
		r.Status = audit.RecordingFailed
		r.Message = err.Error()
		var committed *audit.CommittedWriteError
		if errors.As(err, &committed) {
			r.Status = audit.RecordingUncertain
		}
	}
	return r
}

// Called with the actual operation outcome, after recording and before delivery.
func reportOutcome(stderr io.Writer, r *audit.Report) int {
	outcome := r.Recording
	if outcome == nil {
		outcome = &audit.Recording{Status: audit.RecordingNotRequested}
		r.Recording = outcome
	}
	label := map[string]string{
		audit.RecordingNotRequested: "state recording not requested",
		audit.RecordingRecorded:     "state recorded",
		audit.RecordingFailed:       "state not recorded",
		audit.RecordingUncertain:    "state recorded; durability unconfirmed",
	}[outcome.Status]
	if outcome.Status != audit.RecordingNotRequested {
		fmt.Fprintf(stderr, "skill-audit: %s: path=%q message=%q\n", label, outcome.Path, outcome.Message)
	}
	code := r.ExitCode()
	if code == 1 {
		return 1
	}
	if outcome.Status == audit.RecordingFailed {
		return 3
	}
	if outcome.Status == audit.RecordingUncertain {
		return 2
	}
	return code
}
