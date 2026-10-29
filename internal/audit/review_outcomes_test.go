package audit

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReviewHungProbeLeavesRecordingBudget(t *testing.T) {
	root := temp(t)
	p := skill(t, root, "pkg", "body")
	scanner := NewXRayScanner(fakeScanner(t, "exit 99"), time.Second)
	defer scanner.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	calls := 0
	scanner.process = func(ctx context.Context, _, _ string, _ ...string) (string, string, int, error) {
		calls++
		<-ctx.Done()
		return "", "", -1, ctx.Err()
	}
	r := testReport(context.Background(), []Root{{Scope: "project", Path: p}}, catalog(t), "", time.Second)
	scanner.Scan(ctx, &r.Skills[0])
	if ctx.Err() != nil || calls != 1 {
		t.Fatalf("scanner exhausted operation or retried timeout: %v, calls=%d", ctx.Err(), calls)
	}
	if err := SaveObservation(ctx, filepath.Join(root, "state.json"), r, true); err != nil {
		t.Fatal(err)
	}
}

func TestReviewPermanentProbeFailureNotRetried(t *testing.T) {
	for _, cause := range []error{os.ErrPermission, &os.PathError{Op: "fork/exec", Path: "fixture", Err: os.ErrInvalid}} {
		scanner := NewXRayScanner(fakeScanner(t, "exit 99"), time.Second)
		calls := 0
		scanner.process = func(context.Context, string, string, ...string) (string, string, int, error) {
			calls++
			return "", "", -1, cause
		}
		s := Skill{}
		scanner.Scan(context.Background(), &s)
		scanner.Close()
		if calls != 1 {
			t.Fatalf("permanent launch failure retried %d times", calls)
		}
	}
}

func TestReviewCachedConfigurationSurvivesCancellation(t *testing.T) {
	scanner := NewXRayScanner("relative-scanner", time.Second)
	defer scanner.Close()
	first, second := Skill{}, Skill{}
	scanner.Scan(context.Background(), &first)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	scanner.Scan(ctx, &second)
	if first.Evaluations[0].Status != "unavailable" || second.Evaluations[0].Message != first.Evaluations[0].Message || second.Evaluations[0].Status != "unavailable" {
		t.Fatal(first.Evaluations, second.Evaluations)
	}
}

func TestReviewStoredObservationDoesNotPredictWriteOutcome(t *testing.T) {
	root := temp(t)
	p := skill(t, root, "pkg", "body")
	r := testReport(context.Background(), []Root{{Scope: "project", Path: p}}, catalog(t), "", time.Second)
	state := filepath.Join(root, "state.json")
	r.Recording = &Recording{Status: RecordingRecorded, Path: state}
	if err := SaveObservation(context.Background(), state, r, true); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(state)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), `"recording"`) {
		t.Fatal("stored observation predicts its own final write outcome")
	}
	if strings.Contains(string(b), `"baseline_anchors"`) {
		t.Fatal("stored redundant baseline anchors")
	}
	if r.Recording.Status != RecordingRecorded {
		t.Fatal("mutated caller report")
	}
}

func TestReviewCommittedStateDoesNotClaimConfirmedDurability(t *testing.T) {
	root := temp(t)
	pkg := skill(t, root, "pkg", "fixture")
	r := testReport(context.Background(), []Root{{Scope: "project", Path: pkg}}, catalog(t), "", time.Second)
	state := filepath.Join(root, "state.json")
	r.Recording = &Recording{Status: RecordingRecorded, Path: state}
	cause := errors.New("fixture directory sync failure")
	write := func(ctx context.Context, path string, data []byte) error {
		return writeAtomic(ctx, path, data, func(string) error { return cause })
	}
	for _, baseline := range []bool{true, false} {
		err := saveObservation(context.Background(), state, r, baseline, write)
		var committed *CommittedWriteError
		if !errors.As(err, &committed) || !errors.Is(err, cause) {
			t.Fatal(err)
		}
		st, err := LoadState(state)
		if err != nil {
			t.Fatal(err)
		}
		want := 2
		if baseline {
			want = 1
		}
		if len(st.Observations) != want {
			t.Fatal("committed observation lost", st)
		}
		for _, o := range st.Observations {
			if o.Report.Recording != nil {
				t.Fatal("history predicts durability", o.Report.Recording)
			}
		}
	}
}

func TestReviewLegacyStatesRemainReadOnly(t *testing.T) {
	root := temp(t)
	pkg := skill(t, root, "pkg", "fixture")
	r := testReport(context.Background(), []Root{{Scope: "project", Path: pkg}}, catalog(t), "", time.Second)
	for _, schema := range []string{LegacyStateSchema, AnchorStateSchema} {
		st := State{Schema: schema, Observations: []Observation{{ObservedAt: time.Now().UTC().Format(time.RFC3339Nano), Report: r}}}
		if schema == AnchorStateSchema {
			s := r.Skills[0]
			st.Anchors = []Anchor{{s.Scope, s.Path, s.Digest}}
			// Preserve even the historical recording assertion; do not rewrite evidence.
			st.Observations[0].Report.Recording = &Recording{Status: RecordingRecorded}
		}
		data, err := json.Marshal(st)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, "legacy.json")
		put(t, path, data)
		loaded, err := LoadState(path)
		if err != nil {
			t.Fatal(err)
		}
		current := r
		Compare(&current, loaded)
		if len(current.Lineage) != 1 || current.Lineage[0].Status != "unchanged" {
			t.Fatal(current.Lineage)
		}
		if err = SaveObservation(context.Background(), path, r, false); err == nil {
			t.Fatal("legacy append accepted")
		}
		after, err := os.ReadFile(path)
		if err != nil || string(after) != string(data) {
			t.Fatal("legacy evidence changed", err)
		}
	}
}
