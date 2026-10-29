package audit

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"gopkg.in/yaml.v3"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

func Sum(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func Digest(files []File) string {
	h := sha256.New()
	h.Write([]byte("skill-audit/directory/v1\x00"))
	fs := append([]File(nil), files...)
	sort.Slice(fs, func(i, j int) bool { return fs[i].Path < fs[j].Path })
	for _, f := range fs {
		_ = binary.Write(h, binary.BigEndian, uint64(len(f.Path)))
		h.Write([]byte(f.Path))
		b, _ := hex.DecodeString(f.SHA256)
		h.Write(b)
	}
	return hex.EncodeToString(h.Sum(nil))
}
func Within(parent, child string) bool {
	r, e := filepath.Rel(parent, child)
	return e == nil && r != ".." && !strings.HasPrefix(r, ".."+string(filepath.Separator)) && !filepath.IsAbs(r)
}
func Abs(p string) string { a, _ := filepath.Abs(p); return filepath.Clean(a) }

// NoLinks checks every existing component; missing leaf paths are permitted for outputs.
func NoLinks(p string) error {
	p = Abs(p)
	var components []string
	for q := p; ; q = filepath.Dir(q) {
		components = append(components, q)
		if filepath.Dir(q) == q {
			break
		}
	}
	for i := len(components) - 1; i >= 0; i-- {
		q := components[i]
		stat, e := os.Lstat(q)
		if os.IsNotExist(e) {
			return nil
		}
		if e != nil {
			return e
		}
		if isLink(stat) {
			resolved, _ := filepath.EvalSymlinks(p)
			return fmt.Errorf("link path %q is not followed; use resolved path %q", q, resolved)
		}
	}
	return nil
}
func Metadata(n string) bool { return n == ".git" || n == ".hg" || n == ".svn" || n == ".DS_Store" }
func Dependency(n string) bool {
	switch n {
	case "node_modules", "vendor", ".venv", "venv", "__pycache__":
		return true
	}
	return false
}
func ReadBounded(path string, limit int64) ([]byte, error) {
	if e := NoLinks(path); e != nil {
		return nil, e
	}
	before, e := os.Lstat(path)
	if e != nil {
		return nil, e
	}
	if !before.Mode().IsRegular() {
		return nil, fmt.Errorf("not a regular file")
	}
	f, e := os.Open(path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	i, e := f.Stat()
	if e != nil {
		return nil, e
	}
	if !i.Mode().IsRegular() || !os.SameFile(before, i) {
		return nil, fmt.Errorf("not a regular file")
	}
	if i.Size() > limit {
		return nil, fmt.Errorf("file exceeds %d bytes", limit)
	}
	b, e := io.ReadAll(io.LimitReader(f, limit+1))
	if int64(len(b)) > limit {
		return nil, fmt.Errorf("file exceeds %d bytes", limit)
	}
	return b, e
}
func Classify(path, home string) (string, string) {
	agent := "unknown"
	for q := path; ; q = filepath.Dir(q) {
		n := filepath.Base(q)
		if isAgent(n) {
			if agent == "unknown" {
				agent = n
			}
			par := filepath.Dir(q)
			if (home != "" && sameDirectory(par, Abs(home))) || conventionalHome(par) {
				scope := "global"
				r, _ := filepath.Rel(q, path)
				first := strings.Split(r, string(filepath.Separator))[0]
				if n == ".github" || first == "projects" || first == "worktrees" {
					scope = "project"
				}
				if scope == "project" {
					return scope, agent
				}
				return scope, n
			}
		}
		if filepath.Dir(q) == q {
			break
		}
	}
	if agent != "unknown" {
		return "project", agent
	}
	return "machine", agent
}
func isAgent(n string) bool {
	switch n {
	case ".claude", ".codex", ".agents", ".cursor", ".github", ".gemini", ".opencode":
		return true
	}
	return false
}
func conventionalHome(p string) bool {
	if p == "/root" {
		return true
	}
	par := filepath.Dir(p)
	return par == "/Users" || par == "/home" || strings.EqualFold(filepath.Base(par), "Users") && filepath.VolumeName(par) != ""
}
func Measure(ctx context.Context, path, home string) Package {
	p := Package{Path: Abs(path), Name: filepath.Base(path), Complete: true, Files: []File{}, Data: map[string][]byte{}}
	p.Scope, p.Agent = Classify(p.Path, home)
	fail := func(code, path string, e error) {
		p.Complete = false
		p.Diagnostics = append(p.Diagnostics, diag(code, path, e.Error()))
	}
	if e := NoLinks(p.Path); e != nil {
		fail("link_path", p.Path, e)
		return p
	}
	count, total := 0, int64(0)
	stopped := false
	var walk func(string, int)
	walk = func(dir string, depth int) {
		if stopped {
			return
		}
		if e := ctx.Err(); e != nil {
			fail("measurement_interrupted", dir, e)
			return
		}
		if depth > 64 {
			fail("package_depth", dir, fmt.Errorf("package depth limit"))
			return
		}
		entries, e := os.ReadDir(dir)
		if e != nil {
			fail("read_error", dir, e)
			return
		}
		for _, entry := range entries {
			if ctx.Err() != nil {
				fail("measurement_interrupted", dir, ctx.Err())
				return
			}
			count++
			if count > PackageEntries {
				stopped = true
				fail("package_limit", dir, fmt.Errorf("package entry limit"))
				return
			}
			name := entry.Name()
			full := filepath.Join(dir, name)
			if Metadata(name) {
				continue
			}
			i, e := os.Lstat(full)
			if e != nil {
				fail("read_error", full, e)
				continue
			}
			if isLink(i) {
				fail("symlink_skipped", full, fmt.Errorf("link not followed"))
				continue
			}
			if i.IsDir() {
				if Dependency(name) {
					fail("content_excluded", full, fmt.Errorf("dependency content omitted"))
					continue
				}
				walk(full, depth+1)
				if stopped {
					return
				}
				continue
			}
			if !i.Mode().IsRegular() {
				fail("special_file", full, fmt.Errorf("special file not read"))
				continue
			}
			b, e := ReadBounded(full, FileLimit)
			if e != nil {
				fail("read_error", full, e)
				continue
			}
			total += int64(len(b))
			if total > PackageLimit {
				stopped = true
				fail("package_limit", full, fmt.Errorf("package byte limit"))
				return
			}
			rel, _ := filepath.Rel(p.Path, full)
			rel = filepath.ToSlash(rel)
			if !utf8.ValidString(rel) {
				fail("invalid_path_encoding", dir, fmt.Errorf("non-UTF-8 path skipped: %q", rel))
				continue
			}
			p.Files = append(p.Files, File{rel, Sum(b), int64(len(b))})
			p.Data[rel] = b
		}
	}
	walk(p.Path, 0)
	sort.Slice(p.Files, func(i, j int) bool { return p.Files[i].Path < p.Files[j].Path })
	if b, ok := p.Data["SKILL.md"]; ok {
		parseMetadata(&p, b)
	} else {
		fail("missing_manifest", p.Path, fmt.Errorf("SKILL.md not measured"))
	}
	if p.Complete {
		p.Digest = Digest(p.Files)
	}
	return p
}
func Body(b []byte) []byte {
	lines := bytes.SplitAfter(b, []byte("\n"))
	if len(lines) == 0 || strings.TrimSpace(string(lines[0])) != "---" {
		return b
	}
	offset := len(lines[0])
	for _, l := range lines[1:] {
		offset += len(l)
		if strings.TrimSpace(string(l)) == "---" {
			return b[offset:]
		}
	}
	return b
}
func parseMetadata(p *Package, b []byte) {
	lines := strings.Split(string(b), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return
	}
	end := 0
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	bad := func(s string) {
		p.Complete = false
		p.Diagnostics = append(p.Diagnostics, diag("frontmatter", p.Path, s))
	}
	if end == 0 {
		bad("unterminated YAML frontmatter")
		return
	}
	var m map[string]any
	if e := yaml.Unmarshal([]byte(strings.Join(lines[1:end], "\n")), &m); e != nil {
		bad("invalid YAML frontmatter")
		return
	}
	for _, key := range []string{"name", "description"} {
		v, ok := m[key]
		if !ok {
			continue
		}
		s, ok := v.(string)
		if !ok || strings.TrimSpace(s) == "" {
			bad(key + " must be a nonempty string")
			continue
		}
		if key == "name" && s != "" {
			p.Name = s
		}
		if key == "description" {
			p.Description = s
		}
	}
}

func sameDirectory(a, b string) bool {
	if a == b {
		return true
	}
	if !strings.EqualFold(a, b) {
		return false
	}
	x, e := os.Lstat(a)
	y, f := os.Lstat(b)
	return e == nil && f == nil && x.IsDir() && y.IsDir() && os.SameFile(x, y)
}

func fingerprintMetadataPath(path string) bool {
	for _, part := range strings.Split(path, "/") {
		if Metadata(part) {
			return true
		}
	}
	return false
}

// utf8Prefix returns at most limit bytes without splitting a UTF-8 encoding.
func utf8Prefix(b []byte, limit int) []byte {
	if len(b) > limit {
		b = b[:limit]
	}
	for !utf8.Valid(b) {
		b = b[:len(b)-1]
	}
	return b
}
