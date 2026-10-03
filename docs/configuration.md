# Configuration

Pass one explicit JSON file to `mafsil serve --config FILE`. There is no hidden global configuration or environment expansion. Unknown properties and trailing JSON are rejected. Configuration belongs to the operator, outside the agent's writable workspace.

`mafsil init --workspace ABSOLUTE_DIRECTORY --output FILE` creates a read-only configuration using exclusive file creation. `mafsil doctor --config FILE` verifies local configuration without creating runtime state. Its success is not a tunnel connectivity or end-to-end client test.

| Field | Default | Meaning |
| --- | --- | --- |
| `workspace` | Required | Existing absolute directory below a filesystem root; no symlink/reparse ancestors |
| `allowWrite` | `false` | Expose conditional file mutations |
| `allowExec` | `false` | Expose commands and sessions with the current user's authority |
| `shell` | Resolved `pwsh.exe` / `bash` | Optional absolute PowerShell 7 / Bash executable |
| `scratchDirectory` | Required when execution is enabled | Absolute, separate directory for unique per-session temporary state |

One server configuration exposes one workspace. Run another stdio instance for another workspace. Client-supplied MCP roots cannot widen the boundary.

The `limits` object accepts:

| Field | Default | Accepted range |
| --- | --- | --- |
| `fileBytes` | 4 MiB | 1 KiB–16 MiB, complete regular file |
| `readBytes` | 256 KiB | 256 bytes–1 MiB, no larger than `fileBytes` |
| `outputBytes` | 256 KiB | 1 KiB–1 MiB per output stream/session |
| `sessions` | 8 | 1–16 active sessions |
| `retainedSessions` | 16 | 0–64 completed sessions |
| `timeoutSeconds` | 900 | 1–3600, maximum requested process lifetime |
| `idleSeconds` | 300 | 1–3600, maximum time without a session interaction |

The default process timeout is 120 seconds, capped by `timeoutSeconds`. Polling resets the idle timer, but never extends the wall-clock limit. Completed sessions are pruned on the next list/start operation. All sessions are closed when the stdio server shuts down.

Fixed safeguards include an 8 MiB JSON-RPC frame limit, 16 pending requests, 16 active handlers, bounded input rate, 2 MiB image/resource content, 64 KiB stdin input, 32 file operations per batch, and 128 exact replacements per file. Directory listing accepts up to 2,000 entries per page and an offset of up to 50,000. Exceeding the transport flood limits closes the connection.

The child environment copies only `PATH`, `SYSTEMROOT`, `WINDIR`, `COMSPEC`, `PATHEXT`, `LANG` and `LC_ALL`. HOME/USERPROFILE point to the workspace; temporary, XDG, APPDATA and LOCALAPPDATA paths point to that session's scratch directory. TERM and telemetry/update opt-outs are set explicitly. Credential, proxy, SSH-agent, package-manager and shell-startup environment variables are not automatically forwarded. If a task needs credentials, use a separate, deliberately scoped execution environment; do not broaden this allowlist casually.

Mafsil does not encrypt or store secrets. On Linux, generated configuration uses mode 0600; Windows uses the directory's ACL. Place configuration and scratch directories in locations private to the operator. The server is not a multi-user authorization service.
