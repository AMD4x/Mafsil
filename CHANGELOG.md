# Changelog

## Installer and documentation updates — 2026-10-03

- Stable one-command bootstrap entrypoints select the latest published release and verify its installer before execution. Running the same command again updates an existing installation.
- Installation, workspace setup and MCP connection are separate steps. Workspace prompts use an existing folder chosen by the user, with no assumed drive or project path.
- Keep configuration alongside Mafsil, outside the workspace; updates and uninstall preserve user configuration and scratch data.
- Native bootstrap checks cover corrupted downloads, release selection, README setup and updates. Upgrade fixtures exercise replacement from a simulated older executable.

## 0.1.0 — 2026-10-03

First public release.

- Workspace-scoped MCP file tools with read-only defaults and separately enabled mutations/execution.
- Conditional exact edits, creates, moves and deletes with bounded preparation and rollback.
- Shared process/session tools with Windows Job Objects and ConPTY, Linux process groups and PTY.
- Bounded output replay, stdin cancellation, session timeouts and terminal resizing.
- Modern and legacy MCP compatibility through the official Go SDK.
- Per-user terminal installers, verified raw binaries, update rollback and conservative uninstall.
- Original light/dark identity, editable vector assets and Windows executable resources.
- Native Windows amd64, Linux amd64 and Linux arm64 CI with race checks, security scans, packaged binary tests and installer fixtures.
- Structured bug, setup-help and feature-request forms, with a separate private security reporting channel.
- Exact-source, exact-artifact release approval with verified downloads and native installer validation.
