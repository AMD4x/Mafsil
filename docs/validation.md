# Validation status

Native pre-release validation passed on **2026-10-03** in [CI run 37111395529](https://github.com/AMD4x/Mafsil/actions/runs/37111395529), at source commit `24b027146e0794680edf22075d3e0336a6ccc84f`. All three native jobs and the bounded fuzzing job succeeded. Subsequent commits must pass their own CI before release; the release workflow requires a successful run for the exact approved commit.

This is evidence for the tested source and hosted environments. No public release or working release-download URL is claimed yet.

| Area | Executed evidence | Remaining limits |
| --- | --- | --- |
| Windows amd64 | Local tests plus Windows Server 2025 native CI: Go unit/integration and race tests, process trees, PowerShell state, pipes, ConPTY input/resize, packaged binary and installer fixtures | Other Windows versions and filesystem configurations are not individually qualified |
| Linux amd64 | Ubuntu 24.04 native CI: Go unit/integration and race tests, PTY/process behavior, permissions/xattrs, special files, packaged binary and installer fixtures | Other distributions and filesystems are not individually qualified |
| Linux arm64 | Native ARM64 Ubuntu 24.04 CI with the same runtime, race, packaging and installer checks | No claim of validation on personal remote devices or every ARM64 system |
| MCP | Raw protocol fixtures for all five revisions, official SDK client, and compiled native binary stdio/EOF checks on each target | Broader third-party-client integration |
| Files | Real NTFS owner/DACL/protection, alternate streams, hard links and junctions; native Linux metadata, links, FIFO rejection and cancellation cases | Exotic/network filesystems and concurrent hostile external writers |
| Failure recovery | Injected IO failures during real edits and installer publication; precommit/postcommit cancellation, no-clobber and rollback fixtures | Crash/power-loss recovery remains manual and is not claimed atomic |
| Installer | Ten fixture tests per native target covering install/status/update/uninstall, checksums, ownership, lock exclusion, link rejection and rollback | Actual HTTPS release downloads after an approved release exists |
| Distribution | Native CI rebuilt the approved artwork and Markdown notices into candidates; verified Windows icon/version resources, binary headers, CLI/protocol, payload checksums and assembly provenance fixtures | Public asset-byte and online installation checks after release approval |
| Input resilience | Malformed frames, cancellation and unread-response backpressure fixtures; two 30-second Linux amd64 fuzz runs for paths and terminal sequences | Bounded fuzzing is not exhaustive |
| Static/security | `go vet`, Staticcheck v0.8.1 and govulncheck v1.8.0 passed on all three native targets | Vulnerability results reflect the database at execution time |
| Tunnel | Standard stdio side of the integration | No real credentials, hosted account, live tunnel or remote user device was used |

Local Windows race detection was unavailable because no compatible C compiler was installed. No compiler was installed globally; race detection instead passed in the hosted native matrix. Earlier Linux cross-compilation was preparation only and is distinct from the native runs above.

Installer fixtures use explicit temporary destinations and offline candidate files. They do not register services, scheduled tasks, Registry entries, PATH changes or startup integration. Forced failures are labeled as fault injection; actual IO and rollback still run against the filesystem.

Go 1.27.0 and Python 3.12 were used in CI. A passed vulnerability scan is not a permanent security guarantee. The [CI workflow](../.github/workflows/ci.yml) repeats the checks for changes; the [release workflow](../.github/workflows/release.yml) separately verifies approved artifact provenance and, after publication, actual HTTPS installation on all three targets.
