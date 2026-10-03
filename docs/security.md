# Threat model

Mafsil exposes local file and command capabilities to an MCP client. Treat both the client and any model-generated arguments as potentially mistaken or hostile. The operator chooses the OS account, workspace, configuration and capability flags. Keep configuration outside the writable workspace and grant access only to clients you trust.

## Boundaries

| Threat | Control | Remaining boundary |
| --- | --- | --- |
| File path escape | Relative portable paths, `os.Root`, pinned mutation parents | Rooted APIs do not prohibit bind mounts or filesystem changes by privileged actors |
| Symlink, junction, reparse or hard-link alias | Refuse links/reparse paths and multi-link regular files; recheck opened files | A hostile same-user writer is not isolated from the server |
| Stale edits | Required SHA-256 revisions, exact replacement counts and commit rechecks | Another writer can still race after a final check |
| Partial file mutation | Staged inodes, retained originals and rollback | No cross-file crash atomicity or protection against power loss |
| Unbounded requests/output | Frame, admission, file, batch, session and ring-buffer limits | Limits bound resources, not all CPU or filesystem IO latency |
| Unintended command authority | Execution disabled by default; explicit account choice | Enabled commands have full account rights and network access |
| Credential inheritance | Child environment allowlist, no profiles, separate temporary directories | Commands can explicitly read account-accessible files or process state |
| Process descendants | Windows Job before resume; Linux process groups | Linux deliberate daemonization/privilege changes can escape a group |
| Terminal side effects | Passive, bounded screen model; OSC/string payloads ignored | Raw VT output is untrusted; do not replay it into a trusted terminal blindly |
| Installer path/download abuse | Fixed asset names, HTTPS, checksums, no archive extraction, ownership record | Same-origin checksums do not protect a compromised release account |

The file boundary is not a security sandbox for commands. There is no shell-regex guard claiming otherwise. For untrusted code, use a dedicated account and independently enforced OS/container/VM isolation. Mafsil does not create or configure that isolation for you.

## File mutations

Files are bounded regular files. Reads use handles opened through a root, not a validated absolute path reopened later. Linux opens are nonblocking and no-follow; Windows device aliases and stream syntax are rejected. Special files are not data sources.

For replacement, preparation writes and syncs a temporary inode in the pinned parent directory. Linux copies ownership, ordinary mode and bounded extended attributes; privileged set-ID modes and Linux capability attributes are refused for replacement. Windows accepts files owned by the process user or the token's default owner (which can be a group for an elevated account), preserves the original owner and DACL including its protected state, and refuses read-only, encrypted, compressed or alternate-stream files rather than silently dropping their extra state. Failure to preserve ownership aborts preparation. Originals retained for rollback keep their inode metadata.

New targets are published without replacing an existing destination. Existing targets are rechecked and replaced without truncating the original inode. Filesystems that do not support the required hard links fail closed. Network filesystems and exotic filesystem metadata have not been qualified. Use local NTFS on Windows and ordinary local Linux filesystems for initial evaluation.

A batch has a precommit cancellation check. Once commit starts, cancellation cannot safely interrupt it. If an IO operation fails, recovery restores owned changes; an externally modified target is preserved and an incomplete-rollback error identifies retained backups. A crash can leave `.mafsil-stage-*` or `.mafsil-backup-*` files. They are inaccessible through file tools and are not automatically swept. Inspect and recover them explicitly.

The protocol may suppress a reply after cancellation even when a mutation committed. Read back the affected files before retrying. Do not run another writer against those same paths during a transaction. There is no claim of lock coordination across separate server processes.

## Commands and sessions

Windows launches suspended with an explicit environment block and a constrained inherited-handle list for pipes. Assignment to a kill-on-close Job Object is mandatory; failure aborts before the child is resumed. ConPTY uses the same Job lifecycle and dedicated terminal pipes. Normal completion drains output and closes process, Job and pseudoconsole handles.

Linux starts a process group (or a session with a controlling PTY), terminates the ordinary group on exit/cancellation, and sets a parent-death signal for the direct child. This cannot guarantee cleanup of intentionally detached descendants after an abrupt server kill. Run hostile code under a separately enforced process containment mechanism.

An active-session limit is held atomically during launch. Wall-clock and idle timers terminate forgotten sessions. Output rings discard the oldest bytes and disclose lost history. Input operations are serialized; cancellation or timeout while blocked kills the process because an unknown prefix may already have been delivered.

The allowlisted environment is defense against accidental forwarding, not secret isolation. Mafsil stores no API keys, encrypted key blobs, tunnel identities, service logs or global runtime state. Known tunnel environment variables are cleared in stdio startup, and tools receive a fresh environment. Neither file names nor process output are redacted automatically; expose a workspace that contains only data you intend your client to access.

## Protocol and client trust

The server is a local stdio endpoint, not a multi-tenant network service. A connected client can use all advertised capabilities. Tool annotations are descriptive hints, not an approval system. The client must enforce user confirmation appropriate to its own product.

All file content and command output can contain prompt injection. These strings are data, not operator instructions. Do not let a file's text change the configured authority or convince a client to disclose secrets. MFA, OAuth or a secure tunnel does not make an arbitrary shell command harmless.

## Distribution

Installers do not elevate or register persistence. They preserve existing operator data and refuse unknown/modified managed files. They stage and hash candidates before execution, exclude parallel installers, retain originals, and attempt rollback on ordinary failure. Forced termination, full disks and filesystem errors during recovery can require manual repair.

The release workflow uses manual dispatch, requires successful native CI for the selected commit, and verifies candidate hashes and provenance before publication. The `release` environment can use standard GitHub environment protection. Ordinary branch pushes and pull requests cannot create tags or releases through this workflow.

See [validation status](validation.md) for what has actually run. Automated tests are evidence for their scenarios, not a proof of security against arbitrary kernel, filesystem or privileged-adversary behavior.
