package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"

	"github.com/AMD4x/Mafsil/internal/config"
	"github.com/AMD4x/Mafsil/internal/server"
	"github.com/AMD4x/Mafsil/internal/workspace"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var version = "0.1.0-dev"
var commit = "unknown"

func main() {
	if e := run(os.Args[1:], os.Stdout, os.Stderr); e != nil {
		fmt.Fprintln(os.Stderr, "mafsil:", e)
		os.Exit(1)
	}
}
func run(args []string, out, errout io.Writer) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" {
		fmt.Fprintln(out, `Mafsil — workspace tools over MCP

  mafsil init --workspace ABSOLUTE_DIRECTORY --output CONFIG.json
  mafsil doctor --config CONFIG.json
  mafsil serve --config CONFIG.json
  mafsil version

init creates a read-only configuration without overwriting an existing file.
serve uses stdio; stdout contains only MCP messages. There is no network listener.
Enable allowWrite / allowExec explicitly in your configuration as needed.
Execution has your account's authority and is not a security sandbox.
Install, update and uninstall: see docs/installation.md.`)
		return nil
	}
	if args[0] == "version" || args[0] == "--version" {
		if len(args) != 1 {
			return errors.New("version accepts no arguments")
		}
		fmt.Fprintf(out, "Mafsil %s (%s) %s/%s\n", version, commit, runtime.GOOS, runtime.GOARCH)
		return nil
	}
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	fs.SetOutput(errout)
	configPath := fs.String("config", "", "configuration file")
	root := fs.String("workspace", "", "absolute workspace directory (init)")
	dest := fs.String("output", "mafsil.local.json", "new configuration path (init)")
	if e := fs.Parse(args[1:]); e != nil {
		if errors.Is(e, flag.ErrHelp) {
			return nil
		}
		return e
	}
	if fs.NArg() != 0 {
		return errors.New("unexpected positional arguments")
	}
	switch args[0] {
	case "init":
		if *configPath != "" {
			return errors.New("init uses --output, not --config")
		}
		c := config.Default(*root)
		if e := c.Validate(); e != nil {
			return e
		}
		w, e := workspace.Open(c.Workspace, c.Limits.FileBytes, false)
		if e != nil {
			return e
		}
		w.Close()
		b, e := json.MarshalIndent(c, "", "  ")
		if e != nil {
			return e
		}
		f, e := os.OpenFile(*dest, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			return e
		}
		_, e = f.Write(append(b, '\n'))
		if e == nil {
			e = f.Sync()
		}
		closeErr := f.Close()
		if e == nil {
			e = closeErr
		}
		if e != nil {
			return e
		}
		fmt.Fprintln(out, "Created read-only configuration:", *dest)
		return nil
	case "doctor", "serve":
		if *root != "" || *dest != "mafsil.local.json" {
			return errors.New("workspace/output flags are only valid with init")
		}
		if *configPath == "" {
			return errors.New("--config is required; run mafsil init first")
		}
		c, e := config.Load(*configPath)
		if e != nil {
			return e
		}
		a, e := server.New(c, version)
		if e != nil {
			return e
		}
		defer a.Close()
		if args[0] == "doctor" {
			if c.AllowExec {
				shell := c.Shell
				if shell == "" {
					shell = "bash"
					if runtime.GOOS == "windows" {
						shell = "pwsh.exe"
					}
				}
				if _, e = exec.LookPath(shell); e != nil {
					return fmt.Errorf("execution shell unavailable: %w", e)
				}
				if !filepath.IsAbs(c.ScratchDirectory) {
					return errors.New("scratchDirectory must be absolute")
				}
			}
			return json.NewEncoder(out).Encode(map[string]any{"ok": true, "version": version, "os": runtime.GOOS, "architecture": runtime.GOARCH, "allowWrite": c.AllowWrite, "allowExec": c.AllowExec, "transport": "stdio", "networkProbePerformed": false})
		}
		// A tunnel runtime may launch the stdio server. Drop its known control
		// variables immediately; tool children additionally use an allowlist.
		for _, key := range []string{"CONTROL_PLANE_API_KEY", "CONTROL_PLANE_TUNNEL_ID", "MCP_COMMAND"} {
			_ = os.Unsetenv(key)
		}
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()
		return a.Run(ctx, &mcp.StdioTransport{MaxLineLength: server.MaxMessageBytes})
	default:
		return fmt.Errorf("unknown command %q; run mafsil help", args[0])
	}
}
