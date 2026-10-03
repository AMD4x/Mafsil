package process

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/AMD4x/Mafsil/internal/config"
	"github.com/AMD4x/Mafsil/internal/workspace"
)

func TestProcessHelper(t *testing.T) {
	index := -1
	for i, a := range os.Args {
		if a == "--" {
			index = i + 1
			break
		}
	}
	if index < 0 {
		return
	}
	mode := os.Args[index]
	switch mode {
	case "streams":
		fmt.Print(strings.Repeat("A", 100000))
		fmt.Fprint(os.Stderr, strings.Repeat("B", 100000))
	case "echo":
		line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		fmt.Print("GOT:" + line)
	case "sleep":
		time.Sleep(30 * time.Second)
	case "args":
		for _, a := range os.Args[index+1:] {
			fmt.Printf("[%s]", a)
		}
	case "env":
		fmt.Printf("fixture=%s HOME=%s", os.Getenv("MAFSIL_FIXTURE_SECRET"), os.Getenv("HOME"))
	case "tree":
		p := exec.Command(os.Args[0], "-test.run=TestProcessHelper", "--", "sleep")
		if e := p.Start(); e != nil {
			os.Exit(4)
		}
		fmt.Printf("CHILD=%d\n", p.Process.Pid)
		time.Sleep(30 * time.Second)
	case "screen":
		fmt.Print("\x1b[2J\x1b[HSCREEN_OK\x1b[3;2HPosition")
		time.Sleep(30 * time.Second)
	case "size":
		fmt.Println(terminalSize())
		bufio.NewReader(os.Stdin).ReadString('\n')
		fmt.Println(terminalSize())
	}
	os.Exit(0)
}
func testManager(t *testing.T) *Manager {
	t.Helper()
	base := t.TempDir()
	root := filepath.Join(base, "workspace")
	os.Mkdir(root, 0700)
	c := config.Default(root)
	c.AllowExec = true
	c.ScratchDirectory = filepath.Join(base, "scratch")
	w, e := workspace.Open(root, c.Limits.FileBytes, true)
	if e != nil {
		t.Fatal(e)
	}
	m := New(c, w)
	t.Cleanup(func() { m.Close(); w.Close() })
	return m
}
func helper(mode string) Start {
	return Start{Program: os.Args[0], Args: []string{"-test.run=TestProcessHelper", "--", mode}, WaitMS: 3000, TimeoutSeconds: 30}
}
func finish(t *testing.T, m *Manager, x Snapshot) Snapshot {
	t.Helper()
	deadline := time.Now().Add(6 * time.Second)
	for x.Running && time.Now().Before(deadline) {
		var e error
		x, e = m.IO(context.Background(), IO{SessionID: x.SessionID, WaitMS: 500})
		if e != nil {
			t.Fatal(e)
		}
	}
	if x.Running {
		t.Fatal("process did not finish")
	}
	return x
}
func TestDrainAndReplay(t *testing.T) {
	m := testManager(t)
	x, e := m.Start(context.Background(), helper("streams"))
	if e != nil {
		t.Fatal(e)
	}
	x = finish(t, m, x)
	if x.ExitCode == nil || *x.ExitCode != 0 || x.Stdout != strings.Repeat("A", 100000) || x.Stderr != strings.Repeat("B", 100000) {
		t.Fatalf("incomplete output: %+v %d/%d", x.ExitCode, len(x.Stdout), len(x.Stderr))
	}
	y, e := m.IO(context.Background(), IO{SessionID: x.SessionID})
	if e != nil || y.Stdout != x.Stdout {
		t.Fatal("replay failed", e)
	}
	z, e := m.IO(context.Background(), IO{SessionID: x.SessionID, StdoutOffset: x.StdoutOffset, StderrOffset: x.StderrOffset})
	if e != nil || z.Stdout != "" || z.Stderr != "" {
		t.Fatal("offset replayed output", e)
	}
}
func TestArgumentQuotingAndEnvironment(t *testing.T) {
	m := testManager(t)
	a := helper("args")
	args := []string{"with space", `quote"here`, `ends\`, "$notExpanded; &", "مفصل"}
	a.Args = append(a.Args, args...)
	x, e := m.Start(context.Background(), a)
	if e != nil {
		t.Fatal(e)
	}
	x = finish(t, m, x)
	if x.Stdout != "["+strings.Join(args, "][")+"]" {
		t.Fatalf("argument corruption: %q", x.Stdout)
	}
	t.Setenv("MAFSIL_FIXTURE_SECRET", "synthetic-do-not-inherit")
	x, e = m.Start(context.Background(), helper("env"))
	if e != nil {
		t.Fatal(e)
	}
	x = finish(t, m, x)
	if strings.Contains(x.Stdout, "synthetic-do-not-inherit") || !strings.Contains(x.Stdout, m.ws.Path()) {
		t.Fatal("environment policy failed")
	}
}
func TestStdinAndCancellation(t *testing.T) {
	m := testManager(t)
	a := helper("echo")
	a.WaitMS = 0
	x, e := m.Start(context.Background(), a)
	if e != nil {
		t.Fatal(e)
	}
	x, e = m.IO(context.Background(), IO{SessionID: x.SessionID, Input: "hello\n", WaitMS: 1500})
	if e != nil {
		t.Fatal(e)
	}
	x = finish(t, m, x)
	if !strings.Contains(x.Stdout, "GOT:hello") {
		t.Fatalf("stdin: %q", x.Stdout)
	}
	a = helper("sleep")
	a.WaitMS = 0
	x, e = m.Start(context.Background(), a)
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	if _, e = m.IO(ctx, IO{SessionID: x.SessionID, Input: strings.Repeat("x", 65536)}); e == nil {
		t.Fatal("blocked write ignored cancellation")
	}
	s, _ := m.get(x.SessionID)
	select {
	case <-s.done:
	case <-time.After(4 * time.Second):
		t.Fatal("cancelled process survived")
	}
}
func TestTimeoutIdleAndCloseTree(t *testing.T) {
	m := testManager(t)
	a := helper("sleep")
	a.TimeoutSeconds = 1
	x, e := m.Start(context.Background(), a)
	if e != nil {
		t.Fatal(e)
	}
	x = finish(t, m, x)
	if x.Reason != "timeout" {
		t.Fatalf("timeout: %+v", x)
	}
	m.cfg.Limits.IdleSeconds = 1
	a = helper("sleep")
	x, e = m.Start(context.Background(), a)
	if e != nil {
		t.Fatal(e)
	}
	x = finish(t, m, x)
	if x.Reason != "idle timeout" {
		t.Fatalf("idle: %+v", x)
	}
	m.cfg.Limits.IdleSeconds = 300
	a = helper("tree")
	a.WaitMS = 500
	x, e = m.Start(context.Background(), a)
	if e != nil {
		t.Fatal(e)
	}
	var child int
	for i := 0; i < 10; i++ {
		if _, e = fmt.Sscanf(x.Stdout, "CHILD=%d", &child); e == nil {
			break
		}
		x, e = m.IO(context.Background(), IO{SessionID: x.SessionID, WaitMS: 200})
		if e != nil {
			t.Fatal(e)
		}
	}
	if child == 0 {
		t.Fatalf("child pid missing: %q", x.Stdout)
	}
	if _, e = m.CloseSession(context.Background(), x.SessionID); e != nil {
		t.Fatal(e)
	}
	if processAlive(child) {
		t.Fatalf("descendant %d survived", child)
	}
}
func TestConcurrentCapacity(t *testing.T) {
	m := testManager(t)
	m.cfg.Limits.Sessions = 3
	var wg sync.WaitGroup
	var mu sync.Mutex
	started := 0
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			a := helper("sleep")
			a.WaitMS = 0
			if _, e := m.Start(context.Background(), a); e == nil {
				mu.Lock()
				started++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if started != 3 {
		t.Fatalf("started %d sessions, want 3", started)
	}
}
func TestTTYScreenResizeAndInput(t *testing.T) {
	m := testManager(t)
	a := helper("screen")
	a.TTY = true
	a.WaitMS = 1000
	x, e := m.Start(context.Background(), a)
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 10 && !strings.Contains(x.Screen.Text, "SCREEN_OK"); i++ {
		x, e = m.IO(context.Background(), IO{SessionID: x.SessionID, WaitMS: 200})
		if e != nil {
			t.Fatal(e)
		}
	}
	if x.Screen == nil || !strings.Contains(x.Screen.Text, "SCREEN_OK") {
		t.Fatalf("screen: %+v", x)
	}
	if _, e = m.CloseSession(context.Background(), x.SessionID); e != nil {
		t.Fatal(e)
	}
	a = helper("size")
	a.TTY = true
	a.Columns = 80
	a.Rows = 20
	a.WaitMS = 500
	x, e = m.Start(context.Background(), a)
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 10 && !strings.Contains(x.Stdout, "80x20"); i++ {
		x, e = m.IO(context.Background(), IO{SessionID: x.SessionID, WaitMS: 200})
		if e != nil {
			t.Fatal(e)
		}
	}
	if !strings.Contains(x.Stdout, "80x20") {
		t.Fatalf("initial size: %q", x.Stdout)
	}
	x, e = m.IO(context.Background(), IO{SessionID: x.SessionID, Columns: 110, Rows: 35, Input: "\r\n", WaitMS: 1500})
	if e != nil {
		t.Fatal(e)
	}
	x = finish(t, m, x)
	if !strings.Contains(x.Stdout, "110x35") {
		t.Fatalf("resize: %q", x.Stdout)
	}
}
func TestPersistentShell(t *testing.T) {
	m := testManager(t)
	x, e := m.Start(context.Background(), Start{Interactive: true, TTY: true, TimeoutSeconds: 30, WaitMS: 300})
	if e != nil {
		t.Fatal(e)
	}
	cmd := "value=41\n"
	next := "printf 'VALUE=%s\\n' $((value+1))\n"
	if runtime.GOOS == "windows" {
		cmd = "$value=41\r"
		next = "Write-Output ('VALUE='+($value+1))\r"
	}
	if _, e = m.IO(context.Background(), IO{SessionID: x.SessionID, Input: cmd, WaitMS: 400}); e != nil {
		t.Fatal(e)
	}
	x, e = m.IO(context.Background(), IO{SessionID: x.SessionID, Input: next, WaitMS: 700})
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 10 && !strings.Contains(x.Stdout, "VALUE=42"); i++ {
		x, e = m.IO(context.Background(), IO{SessionID: x.SessionID, WaitMS: 200})
		if e != nil {
			t.Fatal(e)
		}
	}
	if !strings.Contains(x.Stdout, "VALUE=42") {
		t.Fatalf("shell state failed: %q", x.Stdout)
	}
}
func TestRingBound(t *testing.T) {
	r := ring{max: 10}
	r.write([]byte(strings.Repeat("a", 100)))
	s, end, lost := r.read(0)
	if len(s) != 10 || end != 100 || lost != 90 {
		t.Fatal(strconv.Itoa(len(s)), end, lost)
	}
	r.write([]byte("xyz"))
	s, end, lost = r.read(100)
	if s != "xyz" || end != 103 || lost != 0 {
		t.Fatal(s, end, lost)
	}
}
