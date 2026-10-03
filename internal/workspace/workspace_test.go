package workspace

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testWorkspace(t *testing.T, writable bool) *Workspace {
	t.Helper()
	w, e := Open(t.TempDir(), 1<<20, writable)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { w.Close() })
	return w
}
func str(s string) *string { return &s }
func create(t *testing.T, w *Workspace, name, body string) {
	t.Helper()
	if e := os.WriteFile(filepath.Join(w.Path(), name), []byte(body), 0600); e != nil {
		t.Fatal(e)
	}
}
func content(t *testing.T, w *Workspace, name string) string {
	t.Helper()
	b, e := os.ReadFile(filepath.Join(w.Path(), name))
	if e != nil {
		t.Fatal(e)
	}
	return string(b)
}

func TestPathPolicy(t *testing.T) {
	for _, s := range []string{"", "/root", "../x", "a/../b", "a//b", "C:/x", "a\\b", "file:stream", "NUL", "COM1.txt", "a.", "dir/ ", ".mafsil-stage-forged", "a\x00", "a\n"} {
		if _, e := Name(s, false); e == nil {
			t.Errorf("accepted %q", s)
		}
	}
	for _, s := range []string{"مرحبا.txt", "folder/file name", "a-b_1", "a🙂.txt"} {
		if _, e := Name(s, false); e != nil {
			t.Errorf("%q: %v", s, e)
		}
	}
}
func TestEditExactBytesAndRevision(t *testing.T) {
	w := testWorkspace(t, true)
	before := "first\r\nsecond\nthird\rfourth🙂"
	create(t, w, "a", before)
	c := Change{Operation: "edit", Path: "a", ExpectedSHA256: Hash([]byte(before)), Edits: []Edit{{Old: "second", New: "next", Count: 1}}}
	if _, e := w.Edit(context.Background(), []Change{c}); e != nil {
		t.Fatal(e)
	}
	if got := content(t, w, "a"); got != "first\r\nnext\nthird\rfourth🙂" {
		t.Fatalf("unrelated bytes changed: %q", got)
	}
	if _, e := w.Edit(context.Background(), []Change{c}); e == nil {
		t.Fatal("stale revision accepted")
	}
}
func TestPrevalidationAndRollback(t *testing.T) {
	w := testWorkspace(t, true)
	create(t, w, "a", "old")
	create(t, w, "b", "old-b")
	changes := []Change{{Operation: "write", Path: "a", ExpectedSHA256: Hash([]byte("old")), Content: str("new")}, {Operation: "delete", Path: "b", ExpectedSHA256: Hash([]byte("old-b"))}, {Operation: "write", Path: "c", ExpectedSHA256: "missing", Content: str("created")}}
	for failAt := 1; failAt <= 3; failAt++ {
		_, e := w.edit(context.Background(), changes, func(i int) error {
			if i == failAt {
				return errors.New("simulated disk failure")
			}
			return nil
		})
		if e == nil {
			t.Fatal("missing failure")
		}
		if content(t, w, "a") != "old" || content(t, w, "b") != "old-b" {
			t.Fatal("rollback lost data")
		}
		if _, e := os.Stat(filepath.Join(w.Path(), "c")); !os.IsNotExist(e) {
			t.Fatal("new file survived rollback")
		}
		entries, _ := os.ReadDir(w.Path())
		if len(entries) != 2 {
			t.Fatalf("temporary files leaked: %v; %v", entries, e)
		}
	}
	changes[1].ExpectedSHA256 = "wrong"
	if _, e := w.Edit(context.Background(), changes); e == nil {
		t.Fatal("bad revision accepted")
	}
	if content(t, w, "a") != "old" {
		t.Fatal("prevalidation changed file")
	}
}
func TestCreateNoClobberAndMove(t *testing.T) {
	w := testWorkspace(t, true)
	ctx := context.Background()
	if _, e := w.Edit(ctx, []Change{{Operation: "write", Path: "a", ExpectedSHA256: "missing", Content: str("one")}}); e != nil {
		t.Fatal(e)
	}
	if _, e := w.Edit(ctx, []Change{{Operation: "write", Path: "a", ExpectedSHA256: "missing", Content: str("two")}}); e == nil {
		t.Fatal("create overwrote file")
	}
	if _, e := w.Edit(ctx, []Change{{Operation: "move", Path: "a", Destination: "b", ExpectedSHA256: Hash([]byte("one"))}}); e != nil {
		t.Fatal(e)
	}
	if content(t, w, "b") != "one" {
		t.Fatal("move content")
	}
	if _, e := os.Stat(filepath.Join(w.Path(), "a")); !os.IsNotExist(e) {
		t.Fatal("source retained")
	}
}
func TestCapabilitiesAndCancellation(t *testing.T) {
	w := testWorkspace(t, false)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c := []Change{{Operation: "write", Path: "a", ExpectedSHA256: "missing", Content: str("data")}}
	if _, e := w.Edit(context.Background(), c); e == nil {
		t.Fatal("read-only write accepted")
	}
	w.writable = true
	if _, e := w.Edit(ctx, c); !errors.Is(e, context.Canceled) {
		t.Fatalf("cancellation: %v", e)
	}
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	c = append(c, Change{Operation: "write", Path: "b", ExpectedSHA256: "missing", Content: str("data")})
	if _, e := w.edit(ctx2, c, func(int) error { cancel2(); return nil }); e != nil {
		t.Fatal(e)
	}
	if content(t, w, "a") != "data" || content(t, w, "b") != "data" {
		t.Fatal("post-commit cancellation interrupted batch")
	}
}
func TestBoundsAmbiguityHardLinks(t *testing.T) {
	w := testWorkspace(t, true)
	create(t, w, "a", "repeat repeat")
	if _, e := w.Edit(context.Background(), []Change{{Operation: "edit", Path: "a", ExpectedSHA256: Hash([]byte("repeat repeat")), Edits: []Edit{{"repeat", "new", 1}}}}); e == nil {
		t.Fatal("ambiguous exact edit accepted")
	}
	if e := os.Link(filepath.Join(w.Path(), "a"), filepath.Join(w.Path(), "b")); e != nil {
		t.Fatal(e)
	}
	if _, e := w.Read(context.Background(), "a"); e == nil {
		t.Fatal("hardlink read accepted")
	}
	if _, e := w.Edit(context.Background(), []Change{{Operation: "delete", Path: "a", ExpectedSHA256: Hash([]byte("repeat repeat"))}}); e == nil {
		t.Fatal("hardlink edit accepted")
	}
	create(t, w, "big", strings.Repeat("x", (1<<20)+1))
	if _, e := w.Read(context.Background(), "big"); e == nil {
		t.Fatal("oversize read")
	}
}
func TestConcurrentExternalChangeNotOverwrittenByRollback(t *testing.T) {
	w := testWorkspace(t, true)
	create(t, w, "a", "old")
	_, e := w.edit(context.Background(), []Change{{Operation: "write", Path: "a", ExpectedSHA256: Hash([]byte("old")), Content: str("new")}}, func(int) error { create(t, w, "a", "external"); return errors.New("failure") })
	if e == nil || !strings.Contains(e.Error(), "rollback incomplete") {
		t.Fatalf("expected incomplete rollback: %v", e)
	}
	if content(t, w, "a") != "external" {
		t.Fatal("external data overwritten")
	}
}
func TestDirectoryAndUnicode(t *testing.T) {
	w := testWorkspace(t, true)
	ctx := context.Background()
	if e := w.Mkdir(ctx, "folder"); e != nil {
		t.Fatal(e)
	}
	if _, e := w.Edit(ctx, []Change{{Operation: "write", Path: "folder/مَفْصِل🙂.txt", ExpectedSHA256: "missing", Content: str("hello")}}); e != nil {
		t.Fatal(e)
	}
	f, e := w.Read(ctx, "folder/مَفْصِل🙂.txt")
	if e != nil || string(f.Data) != "hello" {
		t.Fatalf("%v %+v", e, f)
	}
	l, e := w.List(ctx, "folder", 1)
	if e != nil || len(l.Entries) != 1 || l.Truncated {
		t.Fatalf("%+v %v", l, e)
	}
}
