# Validation status

This document separates implementation evidence from deployment claims. Status is recorded during local preparation on **2026-10-03**. No GitHub Actions run or release is claimed by this local report.

| Area | Locally executed | Remaining validation |
| --- | --- | --- |
| Windows amd64 | Go unit/integration tests; real process trees, PowerShell state, pipes, ConPTY input/resize and terminal output | Native hosted-runner CI and race detector |
| Linux amd64 | Source and OS-specific test cross-compilation; binary/header inspection | Native Go tests, PTY/process behavior, permissions/xattrs, installer and race checks in CI |
| Linux arm64 | Source and OS-specific test cross-compilation; binary/header inspection | Native ARM64 Go tests, PTY/process behavior, permissions/xattrs, installer and race checks in CI |
| MCP | Raw protocol fixtures for all five revisions; official SDK client; compiled Windows binary stdio/EOF tests | Broader third-party-client integration |
| Files | Real NTFS fixtures for revisions, no-clobber, Unicode, mixed endings, hard links, junctions, DACL preservation and alternate streams | Native Linux metadata and special-file tests |
| Failure recovery | Injected IO failure during real file edits and installer publication; precommit/postcommit cancellation cases | Crash/power-loss recovery remains manual and is not claimed atomic |
| Installer | Windows offline fixture install/status/update/uninstall, checksums, ownership, lock exclusion, reparse rejection and rollback | Native Linux fixtures; real release download URLs after publication |
| Distribution | All target binaries built; Windows icon/version resources; checksums, headers, native CLI/protocol and five release-assembly integrity/provenance fixtures | Native CI artifact production and actual published download verification |
| Input resilience | Bounded Windows fuzz smoke runs for paths and terminal sequences; malformed frames, cancellation and unread-response backpressure fixtures | Longer native CI fuzzing |
| Static/security | `go vet`, Staticcheck and govulncheck run locally | CI repeats these checks against its current vulnerability database |
| Tunnel | Standard stdio side of the integration | No real credentials, hosted account, live tunnel or remote user device was used |

Local Windows race detection is unavailable because no compatible C compiler is installed. No compiler was installed globally. The hosted CI matrix requires race detection; compile-only output is never a substitute.

Installer fixtures do not register services, scheduled tasks, Registry entries, PATH changes or startup integration. They use explicit temporary directories and offline candidate files. Tests using a forced failure are labeled as fault injection; actual IO and rollback still run against the filesystem.

Go 1.27.0, PowerShell 7.6.6 and Python 3.12 were used locally. Pinned development checks are Staticcheck v0.8.1 and govulncheck v1.8.0. A vulnerability scan result reflects the database at execution time and is not a permanent security guarantee.

The [CI workflow](../.github/workflows/ci.yml) is prepared for standard Windows amd64, Linux amd64 and native Linux arm64 runners. After publication, replace this pending status only with observed results at the relevant commit. Live upstream-account tests require an appropriate separately authorized environment.
