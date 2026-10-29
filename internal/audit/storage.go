package audit

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func Decode(path string, limit int64, v any) error {
	b, e := ReadBounded(path, limit)
	if e != nil {
		return e
	}
	d := json.NewDecoder(bytes.NewReader(b))
	if e = d.Decode(v); e != nil {
		return fmt.Errorf("invalid JSON: %w", e)
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return fmt.Errorf("trailing JSON")
	}
	return nil
}
func Encode(v any) ([]byte, error) {
	b, e := json.MarshalIndent(v, "", "  ")
	return append(b, '\n'), e
}
func ReadInventory(path string) (Inventory, error) {
	var i Inventory
	e := Decode(path, StoreLimit, &i)
	if e != nil {
		return i, e
	}
	if i.Schema != "skill-audit/inventory/v2" && i.Schema != InventorySchema {
		return i, fmt.Errorf("unsupported inventory schema %q", i.Schema)
	}
	if i.Schema != InventorySchema {
		i.Policy = "unknown"
	}
	seen := map[string]bool{}
	for _, p := range i.Packages {
		if !filepath.IsAbs(p.Path) || filepath.Clean(p.Path) != p.Path || (p.Scope != "global" && p.Scope != "project" && p.Scope != "machine") || seen[Identity(p)] {
			return i, fmt.Errorf("invalid inventory identity")
		}
		seen[Identity(p)] = true
	}
	return i, nil
}
func ReadCatalog(path string) (Catalog, error) {
	var c Catalog
	e := Decode(path, StoreLimit, &c)
	if e == nil {
		e = ValidateCatalog(c)
	}
	return c, e
}
func OutputSafe(path string, inputs []string, packages []Package) error {
	path = Abs(path)
	if e := NoLinks(path); e != nil {
		return e
	}
	if i, e := os.Lstat(path); e == nil && !i.Mode().IsRegular() {
		return fmt.Errorf("output must be a regular file path")
	} else if e != nil && !os.IsNotExist(e) {
		return e
	}
	for _, p := range inputs {
		if p == "" {
			continue
		}
		p = Abs(p)
		if strings.EqualFold(p, path) {
			return fmt.Errorf("output aliases input %q", p)
		}
		a, e := os.Stat(path)
		b, f := os.Stat(p)
		if e == nil && f == nil && os.SameFile(a, b) {
			return fmt.Errorf("output aliases input %q", p)
		}
	}
	for _, p := range packages {
		if Within(p.Path, path) {
			return fmt.Errorf("output is inside skill package %q", p.Path)
		}
	}
	for q := filepath.Dir(path); ; q = filepath.Dir(q) {
		if _, e := os.Lstat(filepath.Join(q, "SKILL.md")); e == nil {
			return fmt.Errorf("output is inside skill package %q", q)
		}
		if filepath.Dir(q) == q {
			break
		}
	}
	return nil
}

type WriteResult struct {
	Committed bool
	Err       error
}

// atomicWrite's commit point is successful Rename. A subsequent sync error cannot undo it.
func atomicWrite(ctx context.Context, path string, b []byte, exclusive bool, syncDir func(string) error) WriteResult {
	if e := NoLinks(path); e != nil {
		return WriteResult{Err: e}
	}
	dir := filepath.Dir(path)
	f, e := os.CreateTemp(dir, ".skill-audit-write-*")
	if e != nil {
		return WriteResult{Err: e}
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	e = f.Chmod(0600)
	if e == nil {
		_, e = f.Write(b)
	}
	if e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e == nil {
		e = ce
	}
	if e == nil {
		e = ctx.Err()
	}
	if e != nil {
		return WriteResult{Err: e}
	}
	if exclusive {
		if _, e = os.Lstat(path); e == nil {
			return WriteResult{Err: fmt.Errorf("baseline already exists")}
		} else if !os.IsNotExist(e) {
			return WriteResult{Err: e}
		}
	}
	if e = os.Rename(tmp, path); e != nil {
		return WriteResult{Err: e}
	}
	return WriteResult{true, syncDir(dir)}
}
func WriteJSON(ctx context.Context, path string, v any, packages ...Package) error {
	b, e := Encode(v)
	if e != nil {
		return e
	}
	if len(b) > StoreLimit {
		return fmt.Errorf("serialized output exceeds 64 MiB")
	}
	if e = OutputSafe(path, nil, packages); e != nil {
		return e
	}
	r := atomicWrite(ctx, Abs(path), b, false, syncDirectory)
	if r.Committed && r.Err != nil {
		return fmt.Errorf("output committed, durability unconfirmed: %w", r.Err)
	}
	return r.Err
}
func ReadState(path string) (State, error) {
	var s State
	if e := Decode(path, StateLimit, &s); e != nil {
		return s, e
	}
	if s.Schema != StateSchema && s.Schema != "skill-audit/state/v1" {
		return s, fmt.Errorf("unsupported state schema %q", s.Schema)
	}
	if len(s.Observations) == 0 {
		return s, fmt.Errorf("state has no baseline")
	}
	for _, r := range s.Observations {
		if s.Schema == StateSchema && r.Recording != nil {
			return s, fmt.Errorf("stored current observations cannot include recording outcomes")
		}
		if r.CatalogID == "" {
			return s, fmt.Errorf("missing catalog identity")
		}
		if r.Schema != ReportSchema && r.Schema != "skill-audit/report/v1" {
			return s, fmt.Errorf("unsupported observation schema")
		}
		seen := map[string]bool{}
		for _, x := range r.Skills {
			if !filepath.IsAbs(x.Path) || (x.Scope != "global" && x.Scope != "project" && x.Scope != "machine") || seen[Identity(x.Package)] {
				return s, fmt.Errorf("invalid state identity")
			}
			seen[Identity(x.Package)] = true
			files := map[string]bool{}
			for _, f := range x.Files {
				h, e := hex.DecodeString(f.SHA256)
				if !SafeRelative(f.Path) || files[f.Path] || f.Size < 0 || e != nil || len(h) != 32 {
					return s, fmt.Errorf("invalid state file manifest")
				}
				files[f.Path] = true
			}
			if x.Complete && (x.Digest != Digest(x.Files) || !files["SKILL.md"]) {
				return s, fmt.Errorf("invalid baseline fingerprint")
			}
		}
	}
	return s, nil
}
func Record(ctx context.Context, path, mode string, r Report, old *State, syncDir func(string) error) Recording {
	result := Recording{Status: "not_recorded", Path: Abs(path)}
	fail := func(e error) Recording { result.Message = e.Error(); return result }
	if mode != "baseline" && mode != "snapshot" {
		return fail(fmt.Errorf("record mode must be baseline or snapshot"))
	}
	if e := ctx.Err(); e != nil {
		return fail(e)
	}
	ps := []Package{}
	complete := 0
	for _, s := range r.Skills {
		ps = append(ps, s.Package)
		if s.Complete {
			complete++
		}
	}
	if e := OutputSafe(path, nil, ps); e != nil {
		return fail(e)
	}
	for _, d := range r.Diagnostics {
		if d.Incomplete && (d.Code == "inventory_canceled" || d.Code == "inventory_timeout" || d.Code == "audit_interrupted" || d.Code == "audit_canceled") {
			return fail(fmt.Errorf("interrupted observation cannot be recorded"))
		}
	}
	if (!r.DiscoveryComplete || len(r.Skills) > 0) && complete == 0 {
		return fail(fmt.Errorf("no complete package measurements in partial observation"))
	}
	if mode == "snapshot" && (old == nil || old.Schema != StateSchema) {
		return fail(fmt.Errorf("snapshot requires current state; record a new baseline at another path"))
	}
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return fail(e)
	}
	lock, e := os.OpenFile(path+".lock", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return fail(fmt.Errorf("state lock unavailable: %w", e))
	}
	lock.Close()
	defer os.Remove(path + ".lock")
	var state State
	if mode == "baseline" {
		state.Schema = StateSchema
	} else {
		current, e := ReadState(path)
		if e != nil {
			return fail(e)
		}
		a, _ := Encode(current)
		b, _ := Encode(old)
		if !bytes.Equal(a, b) {
			return fail(fmt.Errorf("state changed since comparison"))
		}
		state = current
	}
	r.Recording = nil
	if r.CatalogID == "" {
		r.CatalogID = CatalogID(Catalog{})
	}
	if r.CreatedAt.IsZero() {
		r.CreatedAt = time.Now().UTC()
	}
	state.Observations = append(state.Observations, r)
	b, e := Encode(state)
	if e != nil {
		return fail(e)
	}
	if len(b) > StateLimit {
		return fail(fmt.Errorf("state exceeds 32 MiB"))
	}
	if syncDir == nil {
		syncDir = syncDirectory
	}
	w := atomicWrite(ctx, Abs(path), b, mode == "baseline", syncDir)
	if !w.Committed {
		return fail(w.Err)
	}
	result.Status = "recorded"
	result.Message = "observation saved"
	if w.Err != nil {
		result.Status = "committed_durability_unconfirmed"
		result.Message = "state committed; durability unconfirmed: " + w.Err.Error()
	}
	return result
}
