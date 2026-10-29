package audit

import (
	"fmt"
	"testing"
)

func TestStoreIncrementalSizeEqualsFormattedBytes(t *testing.T) {
	c := Catalog{Schema: StoreSchema, Skills: []Reference{}}
	s := newStoreSize(c)
	for i := 0; i < 20; i++ {
		r := Reference{ID: fmt.Sprint(i), Repository: "https://github.com/a/x", Files: []File{{Path: "a<&.txt"}}, Data: map[string][]byte{"a<&.txt": []byte("\x00binary\xff")}}
		n, e := s.entry(r)
		if e != nil {
			t.Fatal(e)
		}
		s.bytes += n
		s.entries++
		c.Skills = append(c.Skills, r)
		a := Attribution{Repository: r.Repository, Commit: "commit", Path: fmt.Sprintf("LICENSE%d<&", i), Data: []byte{0, 1, 2, 3}}
		n, e = s.attribute(a, 0)
		if e != nil {
			t.Fatal(e)
		}
		s.bytes += n
		s.attribution++
		c.Attribution = append(c.Attribution, a)
		d := diag("failure", fmt.Sprintf("unicode-한글-%d", i), "bounded\nmessage <&")
		n, e = s.diagnostic(d)
		if e != nil {
			t.Fatal(e)
		}
		s.bytes += n
		s.diagnostics++
		c.Diagnostics = append(c.Diagnostics, d)
		actual, _ := Encode(c)
		if s.bytes != len(actual) {
			t.Fatalf("step %d counted %d actual %d", i, s.bytes, len(actual))
		}
	}
}
func BenchmarkStoreAdmission(b *testing.B) {
	for _, count := range []int{8, 16, 32} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			data := make([]byte, 16<<10)
			b.ReportAllocs()
			for n := 0; n < b.N; n++ {
				s := newStoreSize(Catalog{Schema: StoreSchema, Skills: []Reference{}})
				for i := 0; i < count; i++ {
					delta, _ := s.entry(Reference{ID: fmt.Sprint(i), Data: map[string][]byte{"data": data}})
					s.bytes += delta
					s.entries++
				}
			}
		})
	}
}
