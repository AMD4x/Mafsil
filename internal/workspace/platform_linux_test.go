package workspace

import (
	"context"
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
	"testing"
)

func TestLinksFIFOAndPinnedRoot(t *testing.T) {
	w := testWorkspace(t, true)
	outside := t.TempDir()
	os.WriteFile(filepath.Join(outside, "private"), []byte("outside"), 0600)
	if e := os.Symlink(outside, filepath.Join(w.Path(), "link")); e != nil {
		t.Fatal(e)
	}
	if _, e := w.Read(context.Background(), "link/private"); e == nil {
		t.Fatal("symlink escape")
	}
	if e := unix.Mkfifo(filepath.Join(w.Path(), "fifo"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e := w.Read(context.Background(), "fifo"); e == nil {
		t.Fatal("FIFO accepted")
	}
	if _, e := w.List(context.Background(), "fifo", 10); e == nil {
		t.Fatal("FIFO accepted as directory")
	}
	if _, e := w.Directory("fifo"); e == nil {
		t.Fatal("FIFO accepted as working directory")
	}
	if e := w.Mkdir(context.Background(), "fifo/child"); e == nil {
		t.Fatal("FIFO accepted as directory parent")
	}
	if _, e := w.Edit(context.Background(), []Change{{Operation: "write", Path: "fifo/child", ExpectedSHA256: "missing", Content: str("bad")}}); e == nil {
		t.Fatal("FIFO accepted as edit parent")
	}
	if _, e := w.Edit(context.Background(), []Change{{Operation: "write", Path: "link/private", ExpectedSHA256: "missing", Content: str("bad")}}); e == nil {
		t.Fatal("symlink write")
	}
}
func TestPreserveMetadataAndRejectPrivilegedMode(t *testing.T) {
	w := testWorkspace(t, true)
	create(t, w, "a", "old")
	path := filepath.Join(w.Path(), "a")
	os.Chmod(path, 0640)
	if e := unix.Setxattr(path, "user.mafsil-test", []byte("keep"), 0); e != nil {
		t.Fatal(e)
	}
	if _, e := w.Edit(context.Background(), []Change{{Operation: "write", Path: "a", ExpectedSHA256: Hash([]byte("old")), Content: str("new")}}); e != nil {
		t.Fatal(e)
	}
	b := make([]byte, 16)
	n, e := unix.Getxattr(path, "user.mafsil-test", b)
	if e != nil || string(b[:n]) != "keep" {
		t.Fatal("xattr lost", e)
	}
	fi, _ := os.Stat(path)
	if fi.Mode().Perm() != 0640 {
		t.Fatal("mode lost")
	}
	os.Chmod(path, 0750|os.ModeSetuid)
	if _, e := w.Edit(context.Background(), []Change{{Operation: "write", Path: "a", ExpectedSHA256: Hash([]byte("new")), Content: str("bad")}}); e == nil {
		t.Fatal("setuid file accepted")
	}
}
