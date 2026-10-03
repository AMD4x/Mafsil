# Install, update and remove

**The commands below become available after the first approved GitHub release.** There are no working v0.1.0 download assets before that release. Until then, follow the source build in the [README](../README.md).

Release users need no Go toolchain. Windows requires PowerShell 7 to run the installer; command tools also use it by default. Linux requires the usual GNU userland (`sh`, `curl`, `sha256sum`, `stat`, `realpath`, `timeout`, `awk`, `grep` and coreutils). Bash is needed only for shell command/session tools.

## Windows amd64

Download the versioned script, inspect it if required by your organization's policy, then run it in PowerShell 7:

```powershell
Invoke-WebRequest 'https://github.com/AMD4x/Mafsil/releases/download/v0.1.0/install.ps1' -OutFile .\install.ps1
pwsh -NoProfile -File .\install.ps1 -Version v0.1.0
```

Default location: `%LOCALAPPDATA%\Programs\Mafsil`. No administrator permission is requested. The installer prints the full executable path. Use that path in MCP client settings, or add it to the current terminal session's PATH yourself if desired. The installer does not change execution policy or persistent PATH.

```powershell
$MafsilDir = Join-Path $env:LOCALAPPDATA 'Programs\Mafsil'
& "$MafsilDir\mafsil.exe" version
& "$MafsilDir\mafsil.exe" init --workspace 'D:\Projects\demo' --output .\mafsil.local.json
& "$MafsilDir\install.ps1" -Action Status
```

To update, disconnect clients, download the installer for the desired published version and rerun it with that explicit `-Version`. To remove:

```powershell
& "$MafsilDir\install.ps1" -Action Uninstall
```

Use `-Destination 'D:\Tools\Mafsil'` consistently for a custom directory, including status and uninstall. A running executable or another process holding managed files can block an update on Windows; close clients and retry. Mafsil's installer does not terminate existing client processes.

## Linux amd64 / arm64

```sh
curl --proto '=https' --proto-redir '=https' -fL \
  https://github.com/AMD4x/Mafsil/releases/download/v0.1.0/install.sh -o install.sh
sh install.sh --version v0.1.0
```

Default location: `$HOME/.local/share/mafsil`. Architecture is detected with `uname -m`.

```sh
"$HOME/.local/share/mafsil/mafsil" version
"$HOME/.local/share/mafsil/mafsil" init --workspace /path/to/workspace --output ./mafsil.local.json
sh "$HOME/.local/share/mafsil/install.sh" --action status
```

To update, disconnect clients and rerun the new version's installer with an explicit `--version`. To remove:

```sh
sh "$HOME/.local/share/mafsil/install.sh" --action uninstall
```

Use `--destination /absolute/install/directory` consistently for a custom installation. No `sudo`, service, shell-profile or persistent PATH changes are performed. Existing processes continue using their old executable until restarted; close clients before an update.

## Integrity and rollback

The installer stages the binary, license, third-party notices and its platform's management script. It checks all their SHA-256 entries before running the candidate's version command or changing the existing installation. No archive is extracted. An installation record contains only version/file hashes, never credentials.

For a separately obtained binary digest, use Windows `-ExpectedSHA256 DIGEST` or Linux `--sha256 DIGEST`. Checksums downloaded from the same release detect corruption, but do not independently authenticate a compromised release account. Binaries are not Authenticode-signed and the first release does not promise a separate signing key.

An update verifies ownership and previous hashes, retains original files, moves the candidate into place, then verifies the new installation. Ordinary failures roll back. Unknown and modified files are preserved by refusing the operation. Concurrent installers are excluded with an exclusive lock. Do not edit managed files during installation.

Configuration and secret files are outside the managed file list. Uninstall removes only verified managed files and leaves unrelated files intact. There is no implicit purge mode. A damaged installation record requires explicit recovery rather than guessing which files belong to Mafsil.

Power loss or forced process termination can leave `.mafsil-install-*` staging/backups and an `.install-lock`. Do not delete these blindly: stop installers/clients, inspect the exact installation and saved originals, restore needed files, and only then remove the specific stale lock. A rollback failure reports the retained backup path. The updater is not a crash-atomic filesystem transaction.

## Offline installation and tests

Place the appropriate raw binary, `SHA256SUMS`, `LICENSE`, `THIRD_PARTY_NOTICES.md`, and platform installer in a local directory. Pass `-BundleDirectory DIR` on Windows or `--bundle DIR` on Linux. The same integrity checks apply and no download occurs.

Development tests use only this offline mode with explicit temporary destinations. Native Windows amd64 and Linux amd64/arm64 installer fixtures have passed in CI; see [validation status](validation.md). Actual release-download installation will be tested after the first approved release exists.
