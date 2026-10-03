package workspace

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestJunctionsAreBlocked(t *testing.T) {
	w := testWorkspace(t, true)
	outside := t.TempDir()
	os.WriteFile(filepath.Join(outside, "private"), []byte("fixture"), 0600)
	link := filepath.Join(w.Path(), "link")
	script := filepath.Join(t.TempDir(), "junction.ps1")
	os.WriteFile(script, []byte("param($LinkPath,$TargetPath)\n$ErrorActionPreference='Stop'\nNew-Item -ItemType Junction -Path $LinkPath -Target $TargetPath | Out-Null\n"), 0600)
	cmd := exec.Command("pwsh.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-File", script, link, outside)
	if out, e := cmd.CombinedOutput(); e != nil {
		t.Fatalf("junction fixture: %v %s", e, out)
	}
	t.Cleanup(func() { os.Remove(link) })
	if _, e := w.Read(context.Background(), "link/private"); e == nil {
		t.Fatal("junction read accepted")
	}
	if _, e := w.List(context.Background(), "link", 20); e == nil {
		t.Fatal("junction list accepted")
	}
	if _, e := w.Directory("link"); e == nil {
		t.Fatal("junction cwd accepted")
	}
	if _, e := w.Edit(context.Background(), []Change{{Operation: "write", Path: "link/created", ExpectedSHA256: "missing", Content: str("bad")}}); e == nil {
		t.Fatal("junction write accepted")
	}
	if _, e := os.Stat(filepath.Join(outside, "created")); !os.IsNotExist(e) {
		t.Fatal("escaped write")
	}
}
func fileDACL(t *testing.T, path string) string {
	t.Helper()
	f, e := os.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	h, e := reopen(f, windows.READ_CONTROL)
	if e != nil {
		t.Fatal(e)
	}
	defer windows.CloseHandle(h)
	sd, e := windows.GetSecurityInfo(h, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if e != nil {
		t.Fatal(e)
	}
	return sd.String()
}
func TestDACLAndAlternateStreams(t *testing.T) {
	w := testWorkspace(t, true)
	create(t, w, "a", "old")
	p := filepath.Join(w.Path(), "a")
	before := fileDACL(t, p)
	if _, e := w.Edit(context.Background(), []Change{{Operation: "write", Path: "a", ExpectedSHA256: Hash([]byte("old")), Content: str("new")}}); e != nil {
		t.Fatal(e)
	}
	if got := fileDACL(t, p); got != before {
		t.Fatalf("DACL changed: %s => %s", before, got)
	}
	if _, e := w.Edit(context.Background(), []Change{{Operation: "move", Path: "a", ExpectedSHA256: Hash([]byte("new")), Destination: "b"}}); e != nil {
		t.Fatal(e)
	}
	p = filepath.Join(w.Path(), "b")
	if got := fileDACL(t, p); got != before {
		t.Fatal("move changed DACL")
	}
	if e := os.WriteFile(p+":fixture", []byte("keep stream"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e := w.Edit(context.Background(), []Change{{Operation: "write", Path: "b", ExpectedSHA256: Hash([]byte("new")), Content: str("bad")}}); e == nil {
		t.Fatal("stream loss accepted")
	}
	b, e := os.ReadFile(p + ":fixture")
	if e != nil || string(b) != "keep stream" {
		t.Fatal("alternate stream changed", e)
	}
}
