# Validation status

**Mafsil v0.1.0 was published and verified on 2026-10-03.** Native validation passed in [CI run 37112896496](https://github.com/AMD4x/Mafsil/actions/runs/37112896496), at release source commit `888b97c719ca61f20ee5a4f8219db772ad65d8a2`. All three native jobs and the bounded fuzzing job succeeded. The [approved release run](https://github.com/AMD4x/Mafsil/actions/runs/37113683988) then verified the published assets and passed actual HTTPS installation, idempotent update and uninstall on Windows amd64, Linux amd64 and Linux arm64.

The [public release](https://github.com/AMD4x/Mafsil/releases/tag/v0.1.0) contains the 12 approved assets. The published `SHA256SUMS` file has SHA-256 `058e7ed8c6673b561ab778e30759be6feffdd994a64498f5ea495526f4b081a5`; its entries and GitHub's asset digests match the reviewed CI artifacts. Public installer and checksum URLs were also downloaded without authentication and matched byte-for-byte.

This is evidence for the tested source and hosted environments. Subsequent commits require their own checks; a future release must bind its source, CI run and manifest to a new approval.

| Area | Executed evidence | Remaining limits |
| --- | --- | --- |
| Windows amd64 | Local tests plus Windows Server 2025 native CI: Go unit/integration and race tests, process trees, PowerShell state, pipes, ConPTY input/resize, packaged binary and installer fixtures | Other Windows versions and filesystem configurations are not individually qualified |
| Linux amd64 | Ubuntu 24.04 native CI: Go unit/integration and race tests, PTY/process behavior, permissions/xattrs, special files, packaged binary and installer fixtures | Other distributions and filesystems are not individually qualified |
| Linux arm64 | Native ARM64 Ubuntu 24.04 CI with the same runtime, race, packaging and installer checks | No claim of validation on personal remote devices or every ARM64 system |
| MCP | Raw protocol fixtures for all five revisions, official SDK client, and compiled native binary stdio/EOF checks on each target | Broader third-party-client integration |
| Files | Real NTFS owner/DACL/protection, alternate streams, hard links and junctions; native Linux metadata, links, FIFO rejection and cancellation cases | Exotic/network filesystems and concurrent hostile external writers |
| Failure recovery | Injected IO failures during real edits and installer publication; precommit/postcommit cancellation, no-clobber and rollback fixtures | Crash/power-loss recovery remains manual and is not claimed atomic |
| Installer | Ten offline fixture tests per native target; actual published HTTPS install/status/idempotent update/uninstall on all three targets in temporary directories | Upgrade from an older public version is not applicable to the first release; crash recovery remains manual |
| Distribution | Native CI artifacts, seven assembly integrity/approval fixtures, binary headers and CLI/protocol; published bytes checked against approved hashes; Windows metadata and embedded icon images match the approved source | Binaries are not independently code-signed; same-origin hashes do not protect a compromised publishing account |
| Input resilience | Malformed frames, cancellation and unread-response backpressure fixtures; two 30-second Linux amd64 fuzz runs for paths and terminal sequences | Bounded fuzzing is not exhaustive |
| Static/security | `go vet`, Staticcheck v0.8.1 and govulncheck v1.8.0 passed on all three native targets | Vulnerability results reflect the database at execution time |
| Tunnel | Standard stdio side of the integration | No real credentials, hosted account, live tunnel or remote user device was used |

Local Windows race detection was unavailable because no compatible C compiler was installed. No compiler was installed globally; race detection instead passed in the hosted native matrix. Earlier Linux cross-compilation was preparation only and is distinct from the native runs above.

Installer tests use explicit temporary destinations. Pre-release fixtures use offline candidate files; post-release checks download actual public assets over HTTPS on GitHub-hosted runners. They do not register services, scheduled tasks, Registry entries, PATH changes or startup integration. Forced failures are labeled as fault injection; actual IO and rollback still run against the filesystem. No real installation was performed on the local development machine.

Go 1.27.0 and Python 3.12 were used in CI. A passed vulnerability scan is not a permanent security guarantee. The [CI workflow](../.github/workflows/ci.yml) repeats the checks for changes; the [release workflow](../.github/workflows/release.yml) separately verifies approved artifact provenance and actual HTTPS installation on all three targets.
