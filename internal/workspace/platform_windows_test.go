package workspace

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
func fileSecurity(t *testing.T, path string) string {
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
	sd, e := windows.GetSecurityInfo(h, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.OWNER_SECURITY_INFORMATION)
	if e != nil {
		t.Fatal(e)
	}
	owner, _, e := sd.Owner()
	if e != nil {
		t.Fatal(e)
	}
	acl, _, e := sd.DACL()
	if e != nil {
		t.Fatal(e)
	}
	control, _, e := sd.Control()
	if e != nil {
		t.Fatal(e)
	}
	// Compare the promised owner, exact ACEs and inheritance protection. Windows
	// may normalize SE_DACL_AUTO_INHERITED when SetSecurityInfo applies an ACL;
	// GetSecurityInfo can also return group metadata we did not request.
	snapshot, e := windows.NewSecurityDescriptor()
	if e != nil {
		t.Fatal(e)
	}
	if e = snapshot.SetOwner(owner, false); e != nil {
		t.Fatal(e)
	}
	if e = snapshot.SetDACL(acl, true, false); e != nil {
		t.Fatal(e)
	}
	if e = snapshot.SetControl(windows.SE_DACL_PROTECTED, control&windows.SE_DACL_PROTECTED); e != nil {
		t.Fatal(e)
	}
	result := snapshot.String()
	runtime.KeepAlive(sd)
	if result == "" {
		t.Fatal("empty security snapshot")
	}
	return result
}
func TestProtectedDACLIsPreserved(t *testing.T) {
	w := testWorkspace(t, true)
	create(t, w, "protected", "old")
	p := filepath.Join(w.Path(), "protected")
	user, e := windows.GetCurrentProcessToken().GetTokenUser()
	if e != nil {
		t.Fatal(e)
	}
	sd, e := windows.SecurityDescriptorFromString("D:P(A;;FA;;;" + user.User.Sid.String() + ")")
	if e != nil {
		t.Fatal(e)
	}
	acl, _, e := sd.DACL()
	if e != nil {
		t.Fatal(e)
	}
	if e = windows.SetNamedSecurityInfo(p, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil); e != nil {
		t.Fatal(e)
	}
	runtime.KeepAlive(sd)
	before := fileSecurity(t, p)
	if _, e = w.Edit(context.Background(), []Change{{Operation: "write", Path: "protected", ExpectedSHA256: Hash([]byte("old")), Content: str("new")}}); e != nil {
		t.Fatal(e)
	}
	if after := fileSecurity(t, p); after != before {
		t.Fatalf("replacement changed protected security: %s => %s", before, after)
	}
	if _, e = w.Edit(context.Background(), []Change{{Operation: "move", Path: "protected", ExpectedSHA256: Hash([]byte("new")), Destination: "moved"}}); e != nil {
		t.Fatal(e)
	}
	if after := fileSecurity(t, filepath.Join(w.Path(), "moved")); after != before {
		t.Fatalf("move changed protected security: %s => %s", before, after)
	}
}
func TestUserOwnedFileKeepsOwner(t *testing.T) {
	w := testWorkspace(t, true)
	create(t, w, "user-owned", "old")
	p := filepath.Join(w.Path(), "user-owned")
	user, e := windows.GetCurrentProcessToken().GetTokenUser()
	if e != nil {
		t.Fatal(e)
	}
	// Elevated runners may create files owned by their default owner group.
	// Exercise replacement when the source instead belongs to the user SID.
	if e = windows.SetNamedSecurityInfo(p, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION, user.User.Sid, nil, nil, nil); e != nil {
		t.Fatal(e)
	}
	before := fileSecurity(t, p)
	if _, e = w.Edit(context.Background(), []Change{{Operation: "write", Path: "user-owned", ExpectedSHA256: Hash([]byte("old")), Content: str("new")}}); e != nil {
		t.Fatal(e)
	}
	if after := fileSecurity(t, p); after != before {
		t.Fatalf("replacement changed owner or DACL: %s => %s", before, after)
	}
}
func TestDACLAndAlternateStreams(t *testing.T) {
	w := testWorkspace(t, true)
	create(t, w, "a", "old")
	p := filepath.Join(w.Path(), "a")
	before := fileSecurity(t, p)
	if _, e := w.Edit(context.Background(), []Change{{Operation: "write", Path: "a", ExpectedSHA256: Hash([]byte("old")), Content: str("new")}}); e != nil {
		t.Fatal(e)
	}
	if got := fileSecurity(t, p); got != before {
		t.Fatalf("DACL changed: %s => %s", before, got)
	}
	if _, e := w.Edit(context.Background(), []Change{{Operation: "move", Path: "a", ExpectedSHA256: Hash([]byte("new")), Destination: "b"}}); e != nil {
		t.Fatal(e)
	}
	p = filepath.Join(w.Path(), "b")
	if got := fileSecurity(t, p); got != before {
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
