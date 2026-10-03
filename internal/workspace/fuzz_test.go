package workspace

import (
	"path/filepath"
	"strings"
	"testing"
)

func FuzzName(f *testing.F) {
	for _, s := range []string{"a", "../b", "x/../y", "CON", "COM¹.txt", "مرحبا"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		n, e := Name(s, false)
		if e == nil {
			if !filepath.IsLocal(n) || strings.Contains(n, ".."+string(filepath.Separator)) {
				t.Fatalf("unsafe accepted path %q", n)
			}
		}
	})
}
