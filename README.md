<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="assets/brand/logo-dark.svg">
    <img src="assets/brand/logo-light.svg" alt="Mafsil" width="768">
  </picture>
</p>

<p align="center">
  <strong>Your workspace. Connected.</strong><br>
  <a href="#get-started">Get started</a> ·
  <a href="docs/tools.md">Tools</a> ·
  <a href="https://github.com/AMD4x/Mafsil/issues/new/choose">Report a problem</a>
</p>

Mafsil connects an MCP-compatible agent to a workspace on Windows or Linux. Read files, make precise conditional edits, run commands, and work with interactive terminals through a small set of composable tools.

**Read only by default.** You choose the workspace and separately enable file changes and command execution. Mafsil runs as a local stdio process, needs no inbound network port, and starts no background service.

> **Available for Windows x64, Linux x64 and Linux ARM64.** [Download the latest release](https://github.com/AMD4x/Mafsil/releases/latest) or use the installers below. Native CI validates all three targets; see [validation status](docs/validation.md).

## What it does

- Reads bounded UTF-8 text, images and embedded file resources.
- Applies exact, revision-checked edits across multiple files, with prevalidation and rollback on IO failure.
- Runs ordinary commands and interactive shells using native ConPTY on Windows and PTY on Linux.
- Retains bounded process output with replay offsets, cancellation, timeouts, terminal resize and plain-text screen snapshots.
- Speaks MCP `2026-07-28` and the four earlier revisions supported by the official Go SDK.

<picture>
  <source media="(max-width: 600px)" srcset="assets/architecture-mobile.svg">
  <img src="assets/architecture.svg" alt="An MCP client connects to Mafsil. File tools stay in the workspace; command and terminal tools are enabled explicitly and use the account's permissions." width="100%">
</picture>

The name comes from the Arabic **مَفْصِل**, a point where two parts connect.

## Get started

### Install or update Mafsil

Run one command for your platform. It selects the latest stable release and verifies the release installer before running it. Installation does **not** choose a workspace or change your projects.

**Windows — PowerShell 7**

```powershell
irm https://raw.githubusercontent.com/AMD4x/Mafsil/main/install.ps1 -ErrorAction Stop | iex
```

**Linux — Bash, x64 / ARM64**

```bash
(set -o pipefail; curl --proto '=https' -fsSL https://raw.githubusercontent.com/AMD4x/Mafsil/main/install.sh | sh)
```

To update later, close your MCP clients and run the same command again. Your configuration and workspace files are preserved. There is no background updater or persistent PATH change. See the [installation guide](docs/installation.md) for version pinning, custom locations, status and uninstall.

### Choose an existing workspace

Your workspace is **your own existing project folder**, wherever you keep it. No particular drive or folder name is required. The commands below ask for its full path; they do not create or move that folder.

Mafsil keeps its executable and configuration together in its own directory. Keep that directory outside the workspace you expose to an agent.

**Windows — PowerShell 7**

```powershell
$MafsilHome = Join-Path $env:LOCALAPPDATA 'Programs\Mafsil'
$Workspace = Read-Host 'Full path to your existing workspace folder'
& "$MafsilHome\mafsil.exe" init --workspace "$Workspace" --output "$MafsilHome\config.json"
& "$MafsilHome\mafsil.exe" doctor --config "$MafsilHome\config.json"
```

**Linux — Bash**

```bash
MAFSIL_HOME="$HOME/.local/share/mafsil"
read -r -p 'Full path to your existing workspace folder: ' MAFSIL_WORKSPACE
"$MAFSIL_HOME/mafsil" init --workspace "$MAFSIL_WORKSPACE" --output "$MAFSIL_HOME/config.json"
"$MAFSIL_HOME/mafsil" doctor --config "$MAFSIL_HOME/config.json"
```

For a custom installation or source build, set `MafsilHome` / `MAFSIL_HOME` to the directory containing your executable. Run `init` once: it refuses an existing configuration rather than overwriting it. `doctor` checks configuration without running commands or connecting to a network service.

### Connect an MCP client

Use an absolute executable path and an absolute configuration path in your client's stdio MCP settings:

```json
{
  "mcpServers": {
    "mafsil": {
      "command": "/absolute/path/to/Mafsil/mafsil",
      "args": ["serve", "--config", "/absolute/path/to/Mafsil/config.json"]
    }
  }
}
```

Replace both paths with the executable and `config.json` in your Mafsil directory. On Windows use `mafsil.exe` and escape each backslash as `\\` in JSON, or use forward slashes. Some clients use a different settings structure; the command and arguments stay the same.

Start with `server_info`, `list_directory` and `read_file`. The client owns the stdio process lifetime. Disconnecting closes managed sessions.

### Enable the capabilities you need

Edit the generated configuration:

```json
{
  "workspace": "/path/to/workspace",
  "allowWrite": true,
  "allowExec": false
}
```

Omitted limits use conservative defaults. Enable execution only for a trusted client and a suitable user account:

```json
{
  "workspace": "/path/to/workspace",
  "allowWrite": true,
  "allowExec": true,
  "scratchDirectory": "/absolute/path/to/Mafsil/scratch"
}
```

Restart the client connection after changing capabilities. Execution can modify files **outside the workspace**, access the network, and read anything the server's user can read. The filesystem boundary applies to file tools; it is **not an execution sandbox**. Use a dedicated OS account or an independently configured isolation environment when you need stronger separation.

## Tools

| Tool | Purpose | Available when |
| --- | --- | --- |
| `server_info` | Version, capabilities and limits | Always |
| `list_directory` | Bounded, paged directory entries | Always |
| `read_file` | Text, image or embedded resource, with SHA-256 | Always |
| `edit_files` | Conditional write, exact edit, move or delete | `allowWrite` |
| `create_directory` | Create one directory | `allowWrite` |
| `exec_command` | Command, program or interactive shell | `allowExec` |
| `session_io` | Output replay, input, EOF and resize | `allowExec` |
| `list_sessions` | Session status | `allowExec` |
| `close_session` | Stop a managed process tree | `allowExec` |

See [tool contracts and examples](docs/tools.md) and [configuration](docs/configuration.md).

## Installation and updates

The [terminal installers](docs/installation.md) select the latest stable release, download the correct binary and verify its SHA-256. They run without administrator/root elevation, preserve configurations and unrelated files, and roll back ordinary update failures. Re-running the installer updates the managed files. The same script provides status and uninstall commands.

There is no automatic update, service registration, persistent PATH edit, or startup integration. Release binaries carry Windows version/icon resources and are distributed separately for Windows amd64, Linux amd64 and Linux arm64.

## Optional OpenAI integration

Mafsil can be the local stdio server behind the official Secure MCP Tunnel client. Tunnel identity, authentication, upgrades and deployment belong to that client. Mafsil stores no tunnel credentials. See the [integration guide](docs/integrations.md), including the limits of local verification.

## Security boundaries

File tools use Go's traversal-resistant `os.Root`, reject links/reparse points and hard-linked files, and enforce file, request and output limits. Updates require the exact revision you read. Multi-file edits are not crash-atomic; cancellation after commit begins requires a read-back to determine the outcome.

Command children receive an allowlisted environment with dedicated temporary paths. Windows processes join a kill-on-close Job Object before execution. Linux uses process groups; deliberate daemonization or privilege changes can escape that lifecycle mechanism. Screen snapshots are an approximation; raw VT output is also available.

Read the [threat model](docs/security.md) before enabling execution. Report vulnerabilities using [SECURITY.md](SECURITY.md).

## Help and feedback

[Report a bug, ask for help or suggest an improvement](https://github.com/AMD4x/Mafsil/issues/new/choose). The forms help you include your Mafsil version, platform and a small reproduction. Search [existing issues](https://github.com/AMD4x/Mafsil/issues) first, and remove private paths, files and credentials from anything you share.

For vulnerabilities, use [private security reporting](https://github.com/AMD4x/Mafsil/security/advisories/new) instead of a public issue. See [SECURITY.md](SECURITY.md) for the reporting scope.

## Development

### Build from source

Go **1.27.0 or later** is needed to build, not to use a release binary. Windows shell execution uses PowerShell 7; Linux shell execution uses Bash. File tools do not require a shell.

Windows, from the source directory:

```powershell
go build -trimpath -o ..\mafsil-build\mafsil.exe .\cmd\mafsil
& ..\mafsil-build\mafsil.exe version
```

Linux:

```sh
go build -trimpath -o ../mafsil-build/mafsil ./cmd/mafsil
../mafsil-build/mafsil version
```

Then follow [workspace setup](#choose-an-existing-workspace), using the build directory as your Mafsil directory. Building the executable does not configure a workspace.

### Verify changes

```sh
python scripts/verify.py --security
```

Use `--race` where a C compiler is available. [CONTRIBUTING.md](CONTRIBUTING.md) explains fixtures, native tests, fuzzing, packaging and dependency notices. [CI](.github/workflows/ci.yml) runs native Windows amd64, Linux amd64 and Linux arm64 checks; the release workflow requires a separate manual approval and successful CI for the exact source commit.

Mafsil's code and original brand assets use the [MIT license](LICENSE). Dependencies retain the licenses reproduced in [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).
