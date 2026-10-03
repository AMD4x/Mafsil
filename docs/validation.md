# Validation status

Mafsil is validated on native Windows amd64, Linux amd64 and Linux arm64 runners. CI covers the server, file and process behavior, packaging, installers and protocol integration before release artifacts are published.

## Current release

Mafsil **v0.1.0** was published on 2026-10-03. Its native CI run completed successfully on all three targets, and the release workflow verified the published assets before running install, update and uninstall checks against the public HTTPS downloads.

- [v0.1.0 release](https://github.com/AMD4x/Mafsil/releases/tag/v0.1.0)
- [Native CI run](https://github.com/AMD4x/Mafsil/actions/runs/37112896496)
- [Release verification run](https://github.com/AMD4x/Mafsil/actions/runs/37113683988)

## What CI validates

| Area | Coverage | Known limits |
| --- | --- | --- |
| Windows amd64 | Go unit/integration and race tests, process trees, PowerShell state, pipes, ConPTY input/resize, packaged binary and installer fixtures | Not every Windows version or filesystem configuration is individually qualified |
| Linux amd64 | Go unit/integration and race tests, PTY/process behavior, permissions/xattrs, special files, packaged binary and installer fixtures | Not every distribution or filesystem is individually qualified |
| Linux arm64 | Native ARM64 runtime, race, packaging and installer checks | Results apply to the tested hosted environment, not every ARM64 system |
| MCP | Protocol fixtures, official SDK client and compiled stdio/EOF checks | Third-party client behavior can differ |
| Files | Native metadata, links, revision checks, failure recovery and cancellation cases | Exotic/network filesystems and hostile concurrent writers remain outside the guarantee |
| Installer | Offline transaction fixtures plus public HTTPS install, update and uninstall checks | Crash recovery can still require manual repair |
| Distribution | Build metadata, binary headers, manifest/checksum validation and public asset verification | Binaries are not independently code-signed |
| Input resilience | Malformed frames, cancellation/backpressure fixtures and bounded fuzzing | Fuzzing is not exhaustive |
| Static/security | `go vet`, Staticcheck and `govulncheck` | Vulnerability results reflect the database available when CI runs |
| Tunnel | Local stdio side of the integration | Live hosted-account/tunnel connectivity is not part of the automated suite |

Tests use temporary directories and isolated fixtures. Installer validation does not register background services or persistent startup integration. The [CI workflow](../.github/workflows/ci.yml) repeats the relevant checks for new commits, and the [release workflow](../.github/workflows/release.yml) verifies the selected CI artifacts again before and after publication.
