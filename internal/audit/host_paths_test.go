package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func hostFilenameFixtures() []string {
	names := []string{"normal name.md", "nested/日本語.md"}
	if runtime.GOOS != "windows" {
		names = append(names, "a:b.md", `literal\name.md`)
	}
	return names
}
func TestP5HostFilenamesRecordReadAndAppend(t *testing.T) {
	for _, name := range hostFilenameFixtures() {
		t.Run(name, func(t *testing.T) {
			root := tempDir(t)
			fixture(t, root, "SKILL.md", "# fixture")
			fixture(t, root, name, "original bytes")
			inv := InventoryRun(context.Background(), fixtureOptions(root))
			state := filepath.Join(tempDir(t), "state.json")
			baseline := AuditRun(context.Background(), inv, Catalog{}, AuditOptions{StatePath: state, Record: "baseline"})
			if baseline.Recording.Status != "recorded" || Exit(baseline) != 0 {
				t.Fatal(baseline)
			}
			loaded, e := ReadState(state)
			if e != nil {
				t.Fatal("recorded state cannot be read", e)
			}
			if len(loaded.Observations) != 1 || !loaded.Observations[0].Skills[0].Complete {
				t.Fatal(loaded)
			}
			before, _ := os.ReadFile(state)
			unchanged := AuditRun(context.Background(), inv, Catalog{}, AuditOptions{StatePath: state})
			if Exit(unchanged) != 0 || unchanged.Skills[0].Lineage.Status != "unchanged" || len(unchanged.Skills[0].Lineage.Modified) != 0 {
				t.Fatal(unchanged)
			}
			after, _ := os.ReadFile(state)
			if !bytes.Equal(before, after) {
				t.Fatal("ordinary audit changed state")
			}
			fixture(t, root, name, "changed bytes")
			changed := AuditRun(context.Background(), InventoryRun(context.Background(), fixtureOptions(root)), Catalog{}, AuditOptions{StatePath: state, Record: "snapshot"})
			if changed.Recording.Status != "recorded" || changed.Skills[0].Lineage.Status != "changed" || len(changed.Skills[0].Lineage.Modified) != 1 || changed.Skills[0].Lineage.Modified[0] != name {
				t.Fatal(changed)
			}
			loaded, e = ReadState(state)
			if e != nil || len(loaded.Observations) != 2 {
				t.Fatal(loaded, e)
			}
		})
	}
}
func TestP5HostFilenamesStageAndKeepFindingPaths(t *testing.T) {
	for _, name := range hostFilenameFixtures() {
		t.Run(name, func(t *testing.T) {
			root := tempDir(t)
			fixture(t, root, "SKILL.md", "# fixture")
			fixture(t, root, name, "fixture bytes")
			p := Measure(context.Background(), root, "")
			s := scannerFixture(t)
			defer s.Close()
			s.process = func(_ context.Context, _ string, args []string) (string, int, error) {
				if args[0] == "version" {
					return "skill-xray 0.1.0", 0, nil
				}
				staged := filepath.Join(args[len(args)-1], filepath.FromSlash(name))
				b, e := os.ReadFile(staged)
				if e != nil || string(b) != "fixture bytes" {
					t.Fatal(string(b), e)
				}
				result := map[string]any{"ruleId": "fixture", "message": map[string]string{"text": "fixture finding"}, "properties": map[string]string{"effectiveSeverity": "medium", "sxv": "execution", "coverage": "no-reported-gap"}, "locations": []any{map[string]any{"physicalLocation": map[string]any{"artifactLocation": map[string]string{"uri": url.PathEscape(name)}}}}}
				results, _ := json.Marshal([]any{result})
				report := strings.Replace(cleanSARIF, `"results":[]`, `"results":`+string(results), 1)
				if e = os.WriteFile(args[2], []byte(report), 0600); e != nil {
					t.Fatal(e)
				}
				return "", 0, nil
			}
			e, findings := s.Scan(context.Background(), p)
			if !Completed(e) || len(findings) != 1 || findings[0].File != name {
				t.Fatal(e, findings)
			}
		})
	}
}

func TestP5LocalPathsRejectTraversalAndUnsafeHostForms(t *testing.T) {
	bad := []string{"", ".", "..", "../escape", "a/../escape", "a/./b", "a//b", "/absolute", "a/", "nul\x00name", string([]byte{'x', 0xff})}
	if runtime.GOOS == "windows" {
		bad = append(bad, "C:/escape", `a\b`, "a:b.md", "NUL", "COM1.txt")
	}
	for _, name := range bad {
		if LocalRelative(name) {
			t.Errorf("accepted unsafe local path %q", name)
		}
	}
	for _, name := range hostFilenameFixtures() {
		if !LocalRelative(name) {
			t.Errorf("rejected legal host path %q", name)
		}
	}
	p := measuredFixture(t)
	p.Files[0].Path = "../escape"
	p.Digest = Digest(p.Files)
	r := Report{Schema: ReportSchema, CatalogID: CatalogID(Catalog{}), CreatedAt: time.Now().UTC(), Complete: true, DiscoveryComplete: true, Skills: []Skill{{Package: p}}}
	b, e := Encode(State{Schema: StateSchema, Observations: []Report{r}})
	if e != nil {
		t.Fatal(e)
	}
	state := fixture(t, tempDir(t), "state.json", string(b))
	if _, e = ReadState(state); e == nil {
		t.Fatal("state accepted a traversal manifest")
	}
	s := scannerFixture(t)
	defer s.Close()
	s.process = func(_ context.Context, _ string, args []string) (string, int, error) {
		if args[0] == "version" {
			return "skill-xray 0.1.0", 0, nil
		}
		t.Fatal("unsafe package launched")
		return "", 0, nil
	}
	e2, _ := s.Scan(context.Background(), p)
	if e2.Status != "failed" || !strings.Contains(e2.Message, "unsafe staged path") {
		t.Fatal(e2)
	}
}

func TestP5NonUTF8FilenameCannotBecomeAnchor(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("byte-invalid Unix filename fixture")
	}
	root := tempDir(t)
	fixture(t, root, "SKILL.md", "# fixture")
	if e := os.WriteFile(filepath.Join(root, string([]byte{'x', 0xff})), []byte("unrepresentable filename"), 0600); e != nil {
		t.Skipf("filesystem rejects byte-invalid names: %v", e)
	}
	p := Measure(context.Background(), root, "")
	if p.Complete || p.Digest != "" || !hasCode(p.Diagnostics, "invalid_path_encoding") {
		t.Fatal(p)
	}
	inv := InventoryRun(context.Background(), fixtureOptions(root))
	r := AuditRun(context.Background(), inv, Catalog{}, AuditOptions{StatePath: filepath.Join(tempDir(t), "state.json"), Record: "baseline"})
	if r.Recording.Status != "not_recorded" {
		t.Fatal("unrepresentable filename produced an anchor", r)
	}
}
