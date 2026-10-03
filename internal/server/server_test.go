package server

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AMD4x/Mafsil/internal/config"
	"github.com/AMD4x/Mafsil/internal/workspace"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestServerHelper(t *testing.T) {
	for _, a := range os.Args {
		if a == "mafsil-sleep-fixture" {
			time.Sleep(30 * time.Second)
			os.Exit(0)
		}
	}
}
func testApp(t *testing.T, write, exec bool) *App {
	t.Helper()
	root := t.TempDir()
	c := config.Default(root)
	c.AllowWrite = write
	c.AllowExec = exec
	c.ScratchDirectory = filepath.Join(t.TempDir(), "scratch")
	a, e := New(c, "test")
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(a.Close)
	return a
}

type peer struct {
	conn net.Conn
	enc  *json.Encoder
	dec  *json.Decoder
	done chan error
}

func rawPeer(t *testing.T, a *App) *peer {
	t.Helper()
	s, c := net.Pipe()
	p := &peer{conn: c, enc: json.NewEncoder(c), dec: json.NewDecoder(c), done: make(chan error, 1)}
	c.SetDeadline(time.Now().Add(15 * time.Second))
	go func() {
		p.done <- a.Run(context.Background(), &mcp.IOTransport{Reader: s, Writer: s, MaxLineLength: MaxMessageBytes})
	}()
	t.Cleanup(func() {
		c.Close()
		select {
		case <-p.done:
		case <-time.After(5 * time.Second):
			t.Error("server leaked")
		}
	})
	return p
}
func (p *peer) send(t *testing.T, id any, method string, params any) {
	t.Helper()
	m := map[string]any{"jsonrpc": "2.0", "method": method, "params": params}
	if id != nil {
		m["id"] = id
	}
	if e := p.enc.Encode(m); e != nil {
		t.Fatal(e)
	}
}
func (p *peer) recv(t *testing.T) map[string]any {
	t.Helper()
	var m map[string]any
	if e := p.dec.Decode(&m); e != nil {
		t.Fatal(e)
	}
	return m
}
func legacy(t *testing.T, p *peer, version string) {
	p.send(t, 1, "initialize", map[string]any{"protocolVersion": version, "capabilities": map[string]any{}, "clientInfo": map[string]any{"name": "fixture", "version": "1"}})
	m := p.recv(t)
	r, ok := m["result"].(map[string]any)
	if !ok || r["protocolVersion"] != version {
		t.Fatalf("negotiation: %+v", m)
	}
	p.send(t, nil, "notifications/initialized", map[string]any{})
}
func modern() map[string]any {
	return map[string]any{"io.modelcontextprotocol/protocolVersion": "2026-07-28", "io.modelcontextprotocol/clientCapabilities": map[string]any{}, "io.modelcontextprotocol/clientInfo": map[string]any{"name": "fixture", "version": "1"}}
}
func TestProtocolVersionsAndCapabilities(t *testing.T) {
	for _, version := range mcp.SupportedProtocolVersions() {
		t.Run(version, func(t *testing.T) {
			a := testApp(t, false, false)
			p := rawPeer(t, a)
			params := map[string]any{}
			if version == "2026-07-28" {
				params["_meta"] = modern()
				p.send(t, 1, "server/discover", params)
				m := p.recv(t)
				if m["error"] != nil || m["result"].(map[string]any)["resultType"] != "complete" {
					t.Fatal(m)
				}
			} else {
				legacy(t, p, version)
			}
			p.send(t, 2, "tools/list", params)
			m := p.recv(t)
			tools := m["result"].(map[string]any)["tools"].([]any)
			if len(tools) != 3 {
				t.Fatalf("unsafe default tools: %+v", tools)
			}
			for _, v := range tools {
				tool := v.(map[string]any)
				if tool["annotations"].(map[string]any)["readOnlyHint"] != true {
					t.Fatal("read-only hint missing")
				}
			}
		})
	}
}
func TestSDKClientFileRoundTrip(t *testing.T) {
	a := testApp(t, true, false)
	s, c := net.Pipe()
	done := make(chan error, 1)
	go func() {
		done <- a.Run(context.Background(), &mcp.IOTransport{Reader: s, Writer: s, MaxLineLength: MaxMessageBytes})
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client := mcp.NewClient(&mcp.Implementation{Name: "Mafsil test client", Version: "1"}, nil)
	cs, e := client.Connect(ctx, &mcp.IOTransport{Reader: c, Writer: c}, nil)
	if e != nil {
		t.Fatal(e)
	}
	defer func() { cs.Close(); <-done }()
	call := func(name string, args any) *mcp.CallToolResult {
		t.Helper()
		r, e := cs.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
		if e != nil {
			t.Fatal(e)
		}
		return r
	}
	r := call("edit_files", map[string]any{"changes": []any{map[string]any{"operation": "write", "path": "a.txt", "expectedSha256": "missing", "content": "héllo\r\n"}}})
	if r.IsError {
		t.Fatalf("create: %+v", r.Content)
	}
	r = call("read_file", map[string]any{"path": "a.txt"})
	if r.IsError {
		t.Fatal(r)
	}
	var body map[string]any
	if e = json.Unmarshal([]byte(r.Content[0].(*mcp.TextContent).Text), &body); e != nil {
		t.Fatal(e)
	}
	if body["text"] != "héllo\r\n" || body["sha256"] != workspace.Hash([]byte("héllo\r\n")) {
		t.Fatal(body)
	}
	for _, args := range []any{map[string]any{"path": "a.txt", "surprise": true}, map[string]any{"path": "../escape"}, map[string]any{"path": "a.txt", "offset": 2}, map[string]any{"path": "a.txt", "limit": -1}} {
		if r = call("read_file", args); !r.IsError {
			t.Fatalf("bad input accepted: %+v", args)
		}
	}
	for _, name := range []string{"exec_command", "not_a_tool"} {
		r, e := cs.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: map[string]any{}})
		if e == nil && !r.IsError {
			t.Fatalf("unavailable tool %s accepted", name)
		}
	}
}
func TestCancellationRemainsResponsive(t *testing.T) {
	a := testApp(t, false, true)
	p := rawPeer(t, a)
	legacy(t, p, "2025-11-25")
	p.send(t, 2, "tools/call", map[string]any{"name": "exec_command", "arguments": map[string]any{"program": os.Args[0], "args": []string{"-test.run=TestServerHelper", "mafsil-sleep-fixture"}, "waitMs": 30000, "timeoutSeconds": 30}})
	deadline := time.Now().Add(3 * time.Second)
	for len(a.Processes.List()) == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	p.send(t, 3, "ping", map[string]any{})
	r := p.recv(t)
	if r["id"] != float64(3) {
		t.Fatalf("ping blocked: %+v", r)
	}
	p.send(t, nil, "notifications/cancelled", map[string]any{"requestId": 2})
	p.send(t, 4, "ping", map[string]any{})
	for {
		r = p.recv(t)
		if r["id"] == float64(4) {
			break
		}
		if r["id"] == float64(2) && r["error"] == nil {
			if result, ok := r["result"].(map[string]any); ok && result["isError"] != true {
				t.Fatal("late successful response after cancellation")
			}
		}
	}
	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		all := a.Processes.List()
		running := false
		for _, s := range all {
			running = running || s.Running
		}
		if !running {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("cancelled process survived")
}
func TestFrameLimitClosesTransport(t *testing.T) {
	a := testApp(t, false, false)
	p := rawPeer(t, a)
	finished := make(chan error, 1)
	go func() { _, e := p.conn.Write([]byte(strings.Repeat("x", MaxMessageBytes+10) + "\n")); finished <- e }()
	select {
	case <-p.done:
		p.done <- nil
	case <-time.After(5 * time.Second):
		t.Fatal("oversize input did not close")
	}
	p.conn.Close()
	<-finished
}
func TestUnreadResponsesRemainBounded(t *testing.T) {
	p := rawPeer(t, testApp(t, false, false))
	legacy(t, p, "2025-11-25")
	// Deliberately stop reading while sending quick calls. Finished handlers
	// still consume admission until their responses reach the client.
	for id := 2; id <= 18; id++ {
		if e := p.enc.Encode(map[string]any{"jsonrpc": "2.0", "id": id, "method": "ping", "params": map[string]any{}}); e != nil {
			break
		}
	}
	select {
	case <-p.done:
		p.done <- nil
	case <-time.After(5 * time.Second):
		t.Fatal("unread response queue was not bounded")
	}
}
