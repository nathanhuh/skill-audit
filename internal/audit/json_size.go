package audit

import (
	"bytes"
	"encoding/json"
	"strings"
)

// Count one newly retained value, formatted exactly as Encode, at its envelope
// depth. Existing accumulated content is never re-encoded for admission.
func formattedSize(v any, depth int) (int, error) {
	b, e := Encode(v)
	if e != nil {
		return 0, e
	}
	b = b[:len(b)-1]
	return len(b) + 2*depth*(bytes.Count(b, []byte{'\n'})+1), nil
}
func arrayGrowth(v any, count, depth int) (int, error) {
	n, e := formattedSize(v, depth)
	if count == 0 {
		return n + 2 + 2*(depth-1), e
	}
	return n + 2, e
}
func fieldPrefix(name string) int { b, _ := json.Marshal(name); return len(b) + 6 } // comma, newline, two spaces, colon-space
func mapItemGrowth(key string, value any, count, depth int) (int, error) {
	k, _ := json.Marshal(key)
	v, e := json.Marshal(value)
	n := 2*depth + len(k) + 2 + len(v)
	if count == 0 {
		return n + 2 + 2*(depth-1), e
	}
	return n + 2, e
}

type storeSize struct{ bytes, entries, attribution, diagnostics int }

func newStoreSize(c Catalog) storeSize { b, _ := Encode(c); return storeSize{bytes: len(b)} }
func (s storeSize) entry(r Reference) (int, error) {
	return arrayGrowth(wireEntry{r, r.Data}, s.entries, 2)
}
func (s storeSize) attribute(a Attribution, index int) (int, error) {
	return mapItemGrowth(attributionKey(a), a.Data, s.attribution+index, 2)
}
func (s storeSize) diagnostic(d Diagnostic) (int, error) { return arrayGrowth(d, s.diagnostics, 2) }
func attributionKey(a Attribution) string {
	return strings.TrimPrefix(a.Repository, "https://github.com/") + "@" + a.Commit + "/" + a.Path
}
