package audit

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// A new accepted scenario or renamed regression must update the coverage map.
// Parsing includes native-only test files; runtime evidence is reported separately.
func TestAcceptedADRScenariosHaveTests(t *testing.T) {
	root := filepath.Join("..", "..")
	var registry struct {
		Scenarios map[string][]string `json:"scenarios"`
	}
	b, e := os.ReadFile(filepath.Join(root, "docs/adr/scenario-tests.json"))
	if e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(b, &registry); e != nil {
		t.Fatal(e)
	}
	expected := map[string]bool{}
	row := regexp.MustCompile(`(?m)^\| ([A-Z][0-9]+) \|`)
	paths, e := filepath.Glob(filepath.Join(root, "docs/adr/[0-9]*.md"))
	if e != nil {
		t.Fatal(e)
	}
	for _, path := range paths {
		b, e = os.ReadFile(path)
		if e != nil {
			t.Fatal(e)
		}
		if !strings.Contains(string(b), "status: accepted\n") {
			continue
		}
		for _, match := range row.FindAllStringSubmatch(string(b), -1) {
			expected[match[1]] = true
		}
	}

	index, e := os.ReadFile(filepath.Join(root, "docs/adr/README.md"))
	if e != nil {
		t.Fatal(e)
	}
	ranges := regexp.MustCompile(`\| accepted \| ([A-Z])([0-9]+)–([A-Z])([0-9]+) \|`)
	for _, r := range ranges.FindAllStringSubmatch(string(index), -1) {
		if r[1] != r[3] {
			t.Fatalf("invalid scenario range %q", r[0])
		}
		first, _ := strconv.Atoi(r[2])
		last, _ := strconv.Atoi(r[4])
		for n := first; n <= last; n++ {
			id := r[1] + strconv.Itoa(n)
			if !expected[id] {
				t.Errorf("index scenario %s has no accepted ADR row", id)
			}
		}
	}
	functions := map[string]bool{}
	for _, pkg := range []string{"internal/audit", "cmd/skill-audit"} {
		files, e := filepath.Glob(filepath.Join(root, pkg, "*_test.go"))
		if e != nil {
			t.Fatal(e)
		}
		for _, file := range files {
			parsed, e := parser.ParseFile(token.NewFileSet(), file, nil, 0)
			if e != nil {
				t.Fatal(e)
			}
			for _, decl := range parsed.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Recv != nil || !strings.HasPrefix(fn.Name.Name, "Test") || fn.Body == nil || fn.Type.Params.NumFields() != 1 {
					continue
				}
				pointer, ok := fn.Type.Params.List[0].Type.(*ast.StarExpr)
				if !ok {
					continue
				}
				parameter, ok := pointer.X.(*ast.SelectorExpr)
				if !ok || parameter.Sel.Name != "T" {
					continue
				}
				functions[pkg+":"+fn.Name.Name] = true
			}
		}
	}
	for id := range expected {
		refs := registry.Scenarios[id]
		if len(refs) == 0 {
			t.Errorf("accepted scenario %s has no regression", id)
		}
		for _, ref := range refs {
			parts := strings.SplitN(ref, ":", 2)
			if len(parts) != 2 || !functions[parts[0]+":"+strings.SplitN(parts[1], "/", 2)[0]] {
				t.Errorf("%s refers to missing Go test %q", id, ref)
			}
		}
	}
	for id := range registry.Scenarios {
		if !expected[id] {
			t.Errorf("coverage map has undeclared scenario %s", id)
		}
	}
}
