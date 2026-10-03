// Package process owns finite-lifetime child processes and terminal sessions.
// Command execution runs with the server user's authority; it is not a sandbox.
package process

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/AMD4x/Mafsil/internal/config"
	"github.com/AMD4x/Mafsil/internal/terminal"
	"github.com/AMD4x/Mafsil/internal/workspace"
)

type native struct {
	pid         int
	in          io.WriteCloser
	out, errout io.ReadCloser
	wait        func() (int, error)
	kill        func() error
	resize      func(int, int) error
	close       func()
}
type Start struct {
	Command        string   `json:"command,omitempty"`
	Program        string   `json:"program,omitempty"`
	Args           []string `json:"args,omitempty"`
	Interactive    bool     `json:"interactive,omitempty"`
	Cwd            string   `json:"cwd,omitempty"`
	TTY            bool     `json:"tty,omitempty"`
	Columns        int      `json:"columns,omitempty"`
	Rows           int      `json:"rows,omitempty"`
	WaitMS         int      `json:"waitMs,omitempty"`
	TimeoutSeconds int      `json:"timeoutSeconds,omitempty"`
}
type IO struct {
	SessionID    string `json:"sessionId"`
	Input        string `json:"input,omitempty"`
	CloseInput   bool   `json:"closeInput,omitempty"`
	Columns      int    `json:"columns,omitempty"`
	Rows         int    `json:"rows,omitempty"`
	WaitMS       int    `json:"waitMs,omitempty"`
	StdoutOffset int64  `json:"stdoutOffset,omitempty"`
	StderrOffset int64  `json:"stderrOffset,omitempty"`
}
type Snapshot struct {
	SessionID     string             `json:"sessionId"`
	PID           int                `json:"pid"`
	Running       bool               `json:"running"`
	ExitCode      *int               `json:"exitCode,omitempty"`
	Reason        string             `json:"reason,omitempty"`
	TTY           bool               `json:"tty"`
	Stdout        string             `json:"stdout"`
	Stderr        string             `json:"stderr"`
	StdoutOffset  int64              `json:"stdoutOffset"`
	StderrOffset  int64              `json:"stderrOffset"`
	StdoutDropped int64              `json:"stdoutDropped"`
	StderrDropped int64              `json:"stderrDropped"`
	Screen        *terminal.Snapshot `json:"screen,omitempty"`
}
type session struct {
	id             string
	p              *native
	done           chan struct{}
	mu             sync.Mutex
	stdout, stderr ring
	screen         *terminal.Screen
	code           int
	reason         string
	finished       bool
	created, last  time.Time
	writeGate      chan struct{}
}
type Manager struct {
	cfg      config.Config
	ws       *workspace.Workspace
	mu       sync.Mutex
	sessions map[string]*session
	closed   bool
}

func New(cfg config.Config, ws *workspace.Workspace) *Manager {
	return &Manager{cfg: cfg, ws: ws, sessions: map[string]*session{}}
}

func (m *Manager) Start(ctx context.Context, a Start) (Snapshot, error) {
	if !m.cfg.AllowExec {
		return Snapshot{}, errors.New("command execution is disabled")
	}
	if e := ctx.Err(); e != nil {
		return Snapshot{}, e
	}
	forms := 0
	if a.Command != "" {
		forms++
	}
	if a.Program != "" {
		forms++
	}
	if a.Interactive {
		forms++
	}
	if forms != 1 {
		return Snapshot{}, errors.New("choose exactly one of command, program or interactive")
	}
	if len(a.Command) > 65536 || len(a.Args) > 128 {
		return Snapshot{}, errors.New("command or argument count exceeds limit")
	}
	if a.Program == "" && len(a.Args) > 0 {
		return Snapshot{}, errors.New("args requires program")
	}
	if a.Interactive && !a.TTY {
		return Snapshot{}, errors.New("interactive sessions require tty=true")
	}
	if a.WaitMS < 0 || a.WaitMS > 30000 {
		return Snapshot{}, errors.New("waitMs must be 0–30000")
	}
	if a.TimeoutSeconds == 0 {
		a.TimeoutSeconds = min(120, m.cfg.Limits.TimeoutSeconds)
	}
	if a.TimeoutSeconds < 1 || a.TimeoutSeconds > m.cfg.Limits.TimeoutSeconds {
		return Snapshot{}, errors.New("timeoutSeconds exceeds the configured limit")
	}
	if a.Cwd == "" {
		a.Cwd = "."
	}
	dir, e := m.ws.Directory(a.Cwd)
	if e != nil {
		return Snapshot{}, e
	}
	if a.TTY {
		if a.Columns == 0 {
			a.Columns = 100
		}
		if a.Rows == 0 {
			a.Rows = 30
		}
		if !validSize(a.Columns, a.Rows) {
			return Snapshot{}, errors.New("terminal dimensions must be 20–300 columns, 5–100 rows")
		}
	} else if a.Columns != 0 || a.Rows != 0 {
		return Snapshot{}, errors.New("terminal dimensions require tty=true")
	}
	program, args, e := m.invocation(a)
	if e != nil {
		return Snapshot{}, e
	}
	for _, arg := range args {
		if strings.ContainsRune(arg, 0) || len(arg) > 65536 {
			return Snapshot{}, errors.New("invalid process argument")
		}
	}
	if e = os.MkdirAll(m.cfg.ScratchDirectory, 0700); e != nil {
		return Snapshot{}, e
	}
	// Scratch is configured by the operator, never taken from a tool argument.
	for path := filepath.Clean(m.cfg.ScratchDirectory); ; path = filepath.Dir(path) {
		fi, e := os.Lstat(path)
		if e != nil {
			return Snapshot{}, e
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			return Snapshot{}, errors.New("scratch path must not contain links")
		}
		if filepath.Dir(path) == path {
			break
		}
	}
	scratch, e := os.MkdirTemp(m.cfg.ScratchDirectory, "mafsil-session-")
	if e != nil {
		return Snapshot{}, e
	}
	env := Environment(os.Environ(), m.ws.Path(), scratch, a.TTY)
	m.mu.Lock()
	if e = ctx.Err(); e != nil {
		m.mu.Unlock()
		os.Remove(scratch)
		return Snapshot{}, e
	}
	if m.closed {
		m.mu.Unlock()
		os.Remove(scratch)
		return Snapshot{}, errors.New("process manager is closed")
	}
	m.pruneLocked()
	active := 0
	for _, s := range m.sessions {
		s.mu.Lock()
		if !s.finished {
			active++
		}
		s.mu.Unlock()
	}
	if active >= m.cfg.Limits.Sessions {
		m.mu.Unlock()
		os.Remove(scratch)
		return Snapshot{}, errors.New("active session limit reached; close a session")
	}
	p, e := launch(program, args, dir, env, a.TTY, a.Columns, a.Rows)
	if e != nil {
		m.mu.Unlock()
		os.Remove(scratch)
		return Snapshot{}, e
	}
	b := make([]byte, 16)
	if _, e = rand.Read(b); e != nil {
		m.mu.Unlock()
		p.kill()
		p.wait()
		p.close()
		os.Remove(scratch)
		return Snapshot{}, e
	}
	s := &session{id: hex.EncodeToString(b), p: p, done: make(chan struct{}), created: time.Now(), last: time.Now(), writeGate: make(chan struct{}, 1), stdout: ring{max: m.cfg.Limits.OutputBytes}, stderr: ring{max: m.cfg.Limits.OutputBytes}}
	if a.TTY {
		s.screen = terminal.New(a.Columns, a.Rows)
	}
	m.sessions[s.id] = s
	m.mu.Unlock()
	var pumps sync.WaitGroup
	pump := func(r io.Reader, stderr bool) {
		defer pumps.Done()
		buf := make([]byte, 8192)
		for {
			n, e := r.Read(buf)
			if n > 0 {
				s.mu.Lock()
				if stderr {
					s.stderr.write(buf[:n])
				} else {
					s.stdout.write(buf[:n])
					if s.screen != nil {
						s.screen.Feed(buf[:n])
					}
				}
				s.mu.Unlock()
			}
			if e != nil {
				return
			}
		}
	}
	pumps.Add(1)
	go pump(p.out, false)
	if p.errout != nil {
		pumps.Add(1)
		go pump(p.errout, true)
	}
	go func() {
		code, e := p.wait()
		drained := make(chan struct{})
		go func() { pumps.Wait(); close(drained) }()
		select {
		case <-drained:
		case <-time.After(2 * time.Second):
			p.out.Close()
			if p.errout != nil {
				p.errout.Close()
			}
			<-drained
			if e == nil {
				e = errors.New("output drain timed out")
			}
		}
		p.close()
		// Delete only this task-owned directory. Never follow a link outside it.
		cleanup := os.RemoveAll(scratch)
		s.mu.Lock()
		s.code = code
		if e != nil && code == 0 {
			s.code = 1
		}
		if s.reason == "" && e != nil {
			s.reason = "process or output capture failed"
		}
		if cleanup != nil {
			s.reason = "process ended; scratch cleanup failed"
		}
		s.finished = true
		s.mu.Unlock()
		close(s.done)
	}()
	go s.watch(time.Duration(a.TimeoutSeconds)*time.Second, time.Duration(m.cfg.Limits.IdleSeconds)*time.Second)
	if e = s.await(ctx, a.WaitMS); e != nil {
		s.stop("cancelled")
		<-s.done
		return Snapshot{}, e
	}
	return s.snapshot(0, 0), nil
}

func (m *Manager) invocation(a Start) (string, []string, error) {
	if a.Program != "" {
		if !filepath.IsAbs(a.Program) {
			return "", nil, errors.New("program must be an absolute executable path")
		}
		return a.Program, a.Args, nil
	}
	shell := m.cfg.Shell
	if shell == "" {
		name := "bash"
		if runtime.GOOS == "windows" {
			name = "pwsh.exe"
		}
		p, e := exec.LookPath(name)
		if e != nil {
			return "", nil, fmt.Errorf("shell unavailable; configure an absolute shell path: %w", e)
		}
		shell = p
	}
	if runtime.GOOS == "windows" {
		if a.Interactive {
			return shell, []string{"-NoLogo", "-NoProfile", "-NoExit"}, nil
		}
		return shell, []string{"-NoLogo", "-NoProfile", "-NonInteractive", "-Command", a.Command}, nil
	}
	if a.Interactive {
		return shell, []string{"--noprofile", "--norc", "-i", "+m"}, nil
	}
	return shell, []string{"--noprofile", "--norc", "-c", a.Command}, nil
}

// Environment is allowlisted, not an attempt to recognize every secret name.
// HOME and temporary directories point at operator-selected workspace state.
func Environment(in []string, home, scratch string, tty bool) []string {
	allow := map[string]bool{"PATH": true, "SYSTEMROOT": true, "WINDIR": true, "COMSPEC": true, "PATHEXT": true, "LANG": true, "LC_ALL": true}
	result := map[string]string{}
	for _, pair := range in {
		k, v, ok := strings.Cut(pair, "=")
		if ok && allow[strings.ToUpper(k)] {
			result[strings.ToUpper(k)] = v
		}
	}
	result["HOME"] = home
	result["USERPROFILE"] = home
	result["TMP"] = scratch
	result["TEMP"] = scratch
	result["TMPDIR"] = scratch
	result["APPDATA"] = scratch
	result["LOCALAPPDATA"] = scratch
	result["XDG_CONFIG_HOME"] = scratch
	result["XDG_CACHE_HOME"] = scratch
	result["XDG_DATA_HOME"] = scratch
	result["XDG_STATE_HOME"] = scratch
	result["POWERSHELL_TELEMETRY_OPTOUT"] = "1"
	result["POWERSHELL_UPDATECHECK"] = "Off"
	result["DOTNET_CLI_TELEMETRY_OPTOUT"] = "1"
	if tty {
		result["TERM"] = "xterm-256color"
	} else {
		result["TERM"] = "dumb"
	}
	keys := make([]string, 0, len(result))
	for k := range result {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		out = append(out, k+"="+result[k])
	}
	return out
}

func (m *Manager) get(id string) (*session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.sessions[id]
	if s == nil {
		return nil, errors.New("unknown or expired session")
	}
	return s, nil
}
func (m *Manager) IO(ctx context.Context, a IO) (Snapshot, error) {
	if len(a.Input) > 65536 || a.WaitMS < 0 || a.WaitMS > 30000 || a.StdoutOffset < 0 || a.StderrOffset < 0 {
		return Snapshot{}, errors.New("input, offset or wait exceeds limit")
	}
	s, e := m.get(a.SessionID)
	if e != nil {
		return Snapshot{}, e
	}
	select {
	case s.writeGate <- struct{}{}:
		defer func() { <-s.writeGate }()
	case <-ctx.Done():
		return Snapshot{}, ctx.Err()
	}
	s.mu.Lock()
	s.last = time.Now()
	finished := s.finished
	if a.StdoutOffset > s.stdout.end || a.StderrOffset > s.stderr.end {
		s.mu.Unlock()
		return Snapshot{}, errors.New("output offset is ahead of captured output")
	}
	if a.Columns != 0 || a.Rows != 0 {
		if s.screen == nil || finished {
			s.mu.Unlock()
			return Snapshot{}, errors.New("resize requires a live terminal")
		}
		cols, rows := a.Columns, a.Rows
		if cols == 0 {
			cols = s.screen.Columns()
		}
		if rows == 0 {
			rows = s.screen.Rows()
		}
		if !validSize(cols, rows) {
			s.mu.Unlock()
			return Snapshot{}, errors.New("invalid terminal dimensions")
		}
		if e = s.p.resize(cols, rows); e != nil {
			s.mu.Unlock()
			return Snapshot{}, e
		}
		s.screen.Resize(cols, rows)
	}
	s.mu.Unlock()
	if a.Input != "" || a.CloseInput {
		if finished {
			return Snapshot{}, errors.New("session has exited")
		}
		if a.CloseInput && s.screen != nil {
			return Snapshot{}, errors.New("terminal input cannot be half-closed; send a terminal key or close the session")
		}
		done := make(chan error, 1)
		go func() {
			var e error
			if a.Input != "" {
				_, e = io.WriteString(s.p.in, a.Input)
			}
			if e == nil && a.CloseInput {
				e = s.p.in.Close()
			}
			done <- e
		}()
		timer := time.NewTimer(3 * time.Second)
		defer timer.Stop()
		select {
		case e = <-done:
		case <-ctx.Done():
			e = ctx.Err()
			s.stop("input cancelled")
		case <-timer.C:
			e = errors.New("stdin write timed out")
			s.stop("stdin timeout")
		}
		if e != nil {
			s.stop("stdin failed")
			<-s.done
			return Snapshot{}, e
		}
	}
	if e = s.await(ctx, a.WaitMS); e != nil {
		return Snapshot{}, e
	}
	return s.snapshot(a.StdoutOffset, a.StderrOffset), nil
}
func (s *session) await(ctx context.Context, ms int) error {
	if ms == 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(time.Duration(ms) * time.Millisecond)
	defer timer.Stop()
	select {
	case <-s.done:
		return nil
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (s *session) stop(reason string) {
	s.mu.Lock()
	if !s.finished && s.reason == "" {
		s.reason = reason
	}
	s.mu.Unlock()
	_ = s.p.kill()
}
func (s *session) watch(maximum, idle time.Duration) {
	timer := time.NewTimer(maximum)
	defer timer.Stop()
	tick := time.NewTicker(min(idle, time.Second))
	defer tick.Stop()
	for {
		select {
		case <-s.done:
			return
		case <-timer.C:
			s.stop("timeout")
			return
		case <-tick.C:
			s.mu.Lock()
			stale := time.Since(s.last) >= idle
			s.mu.Unlock()
			if stale {
				s.stop("idle timeout")
				return
			}
		}
	}
}
func (s *session) snapshot(out, errout int64) Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	a := Snapshot{SessionID: s.id, PID: s.p.pid, Running: !s.finished, Reason: s.reason, TTY: s.screen != nil}
	a.Stdout, a.StdoutOffset, a.StdoutDropped = s.stdout.read(out)
	a.Stderr, a.StderrOffset, a.StderrDropped = s.stderr.read(errout)
	if s.finished {
		code := s.code
		a.ExitCode = &code
	}
	if s.screen != nil {
		x := s.screen.Snapshot()
		a.Screen = &x
	}
	return a
}
func (m *Manager) CloseSession(ctx context.Context, id string) (Snapshot, error) {
	s, e := m.get(id)
	if e != nil {
		return Snapshot{}, e
	}
	s.stop("closed")
	select {
	case <-s.done:
		return s.snapshot(0, 0), nil
	case <-ctx.Done():
		return Snapshot{}, ctx.Err()
	}
}
func (m *Manager) List() []Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pruneLocked()
	out := make([]Snapshot, 0, len(m.sessions))
	for _, s := range m.sessions {
		s.mu.Lock()
		x := Snapshot{SessionID: s.id, PID: s.p.pid, Running: !s.finished, Reason: s.reason, TTY: s.screen != nil, StdoutOffset: s.stdout.end, StderrOffset: s.stderr.end}
		if s.finished {
			code := s.code
			x.ExitCode = &code
		}
		s.mu.Unlock()
		out = append(out, x)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SessionID < out[j].SessionID })
	return out
}
func (m *Manager) pruneLocked() {
	var completed []*session
	for _, s := range m.sessions {
		s.mu.Lock()
		if s.finished {
			completed = append(completed, s)
		}
		s.mu.Unlock()
	}
	sort.Slice(completed, func(i, j int) bool { return completed[i].created.After(completed[j].created) })
	for _, s := range completed[min(len(completed), m.cfg.Limits.RetainedSessions):] {
		delete(m.sessions, s.id)
	}
}
func (m *Manager) Close() {
	m.mu.Lock()
	m.closed = true
	all := make([]*session, 0, len(m.sessions))
	for _, s := range m.sessions {
		all = append(all, s)
	}
	m.mu.Unlock()
	for _, s := range all {
		s.stop("server closed")
	}
	for _, s := range all {
		<-s.done
	}
}
func validSize(c, r int) bool { return c >= 20 && c <= 300 && r >= 5 && r <= 100 }

type ring struct {
	data []byte
	max  int
	end  int64
}

func (r *ring) write(b []byte) {
	r.end += int64(len(b))
	if len(b) >= r.max {
		r.data = append(r.data[:0], b[len(b)-r.max:]...)
		return
	}
	over := len(r.data) + len(b) - r.max
	if over > 0 {
		copy(r.data, r.data[over:])
		r.data = r.data[:len(r.data)-over]
	}
	r.data = append(r.data, b...)
}
func (r *ring) read(offset int64) (string, int64, int64) {
	start := r.end - int64(len(r.data))
	lost := max(0, start-offset)
	offset = max(start, min(offset, r.end))
	return strings.ToValidUTF8(string(r.data[offset-start:]), "�"), r.end, lost
}
