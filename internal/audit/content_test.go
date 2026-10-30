package audit

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNoLinksSuggestsResolvedPathForMissingOutput(t *testing.T) {
	root := tempDir(t)
	real := filepath.Join(root, "real")
	if e := os.Mkdir(real, 0700); e != nil {
		t.Fatal(e)
	}
	link := filepath.Join(root, "link")
	if e := os.Symlink(real, link); e != nil {
		t.Skip("symlinks unavailable:", e)
	}
	e := NoLinks(filepath.Join(link, "sub", "inventory.json"))
	want := filepath.Join(real, "sub", "inventory.json")
	if e == nil || !strings.Contains(e.Error(), fmt.Sprintf("use resolved path %q", want)) {
		t.Fatal(e)
	}
}

func TestNoLinksNamesDanglingTarget(t *testing.T) {
	root := tempDir(t)
	link := filepath.Join(root, "link")
	if e := os.Symlink(filepath.Join(root, "missing"), link); e != nil {
		t.Skip("symlinks unavailable:", e)
	}
	e := NoLinks(filepath.Join(link, "out.json"))
	if e == nil || !strings.Contains(e.Error(), fmt.Sprintf("its target %q cannot be resolved", filepath.Join(root, "missing"))) {
		t.Fatal(e)
	}
}
