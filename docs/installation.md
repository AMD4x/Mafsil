# Install, update and remove

Installing Mafsil and choosing a workspace are separate steps. Installation manages Mafsil's own files; it never creates, moves or selects a project folder.

## Install the latest stable release

These commands fetch the official bootstrap from this repository and execute it. The bootstrap resolves one stable release, verifies its installer against `SHA256SUMS`, and runs that versioned installer. The installer verifies the binary and other managed files before changing an installation.

**Windows x64 — PowerShell 7**

```powershell
irm https://raw.githubusercontent.com/AMD4x/Mafsil/main/install.ps1 -ErrorAction Stop | iex
```

**Linux x64 / ARM64 — Bash**

```bash
(set -o pipefail; curl --proto '=https' -fsSL https://raw.githubusercontent.com/AMD4x/Mafsil/main/install.sh | sh)
```

The Linux command uses a subshell with `pipefail` so a failed download fails the command without changing your terminal settings. The complete bootstrap is parsed before installation begins. If you prefer to inspect the bootstrap first, use the download-and-run commands under [specific versions](#install-a-specific-version).

Runtime binaries need no Go installation. Windows requires PowerShell 7. Linux requires Bash for the command above, plus `sh`, `curl`, `sha256sum`, `stat`, `realpath`, `timeout`, `awk`, `grep`, `sed` and the usual coreutils.

## One Mafsil directory; your workspace stays separate

| Platform | Default Mafsil directory |
| --- | --- |
| Windows | `%LOCALAPPDATA%\Programs\Mafsil` |
| Linux | `$HOME/.local/share/mafsil` |

The default locations remain compatible with existing installations. The directory contains the executable, management script, license/notices and installation record. The recommended setup also stores your `config.json` there; an optional `scratch` subdirectory can hold terminal session data when execution is enabled.

Your workspace is an **existing folder you choose**, on any supported local drive/location. It stays where it is. Keep the Mafsil directory outside that workspace. No particular drive letter or sample project directory is required.

Follow [Choose an existing workspace](../README.md#choose-an-existing-workspace) once after installation. `init` refuses to overwrite an existing configuration. A custom installation uses the same layout: substitute your actual Mafsil directory in the setup commands.

## Update

1. Close MCP clients using Mafsil so their processes release the executable.
2. Run the **same install command** again. It selects the latest stable release and updates the existing default installation.
3. Restart your MCP clients. Keep the same configuration and executable paths.

For a custom location, use the same explicit destination again; see below. Your configuration, scratch data and unrelated files are not managed or removed by the updater. The bootstrap does not assume that its own script version is the newest application version: it resolves the published stable release each time.

There is no background update service, automatic restart, persistent PATH change, or administrator/root elevation. A running executable can block an update on Windows; the installer reports the failure rather than terminating your clients.

## Install a specific version

Use a published `vMAJOR.MINOR.PATCH` tag. The example below selects the existing `v0.1.0` release; replace that tag deliberately when selecting another published version.

**Windows**

```powershell
Invoke-WebRequest https://raw.githubusercontent.com/AMD4x/Mafsil/main/install.ps1 -OutFile .\mafsil-install.ps1 -ErrorAction Stop
pwsh -NoProfile -File .\mafsil-install.ps1 -Version v0.1.0
```

For a custom directory, pass `-Destination` to that command, for example `-Destination (Join-Path $HOME 'Apps\Mafsil')`. Updates must use that same destination.

**Linux**

```bash
curl --proto '=https' -fsSLo mafsil-install.sh https://raw.githubusercontent.com/AMD4x/Mafsil/main/install.sh
sh mafsil-install.sh --version v0.1.0
```

For a custom directory, add `--destination "$HOME/Apps/Mafsil"`. Use the same destination when updating. These are application installation locations, not workspace examples.

## Status and uninstall

For a custom installation, change the first variable to your actual Mafsil directory. Run only the operation you need.

**Windows**

```powershell
$MafsilHome = Join-Path $env:LOCALAPPDATA 'Programs\Mafsil'
& "$MafsilHome\mafsil.exe" version
& "$MafsilHome\install.ps1" -Action Status -Destination "$MafsilHome"
```

```powershell
$MafsilHome = Join-Path $env:LOCALAPPDATA 'Programs\Mafsil'
& "$MafsilHome\install.ps1" -Action Uninstall -Destination "$MafsilHome"
```

**Linux**

```bash
MAFSIL_HOME="$HOME/.local/share/mafsil"
"$MAFSIL_HOME/mafsil" version
sh "$MAFSIL_HOME/install.sh" --action status --destination "$MAFSIL_HOME"
```

```bash
MAFSIL_HOME="$HOME/.local/share/mafsil"
sh "$MAFSIL_HOME/install.sh" --action uninstall --destination "$MAFSIL_HOME"
```

Uninstall removes only verified managed files. It preserves `config.json`, scratch data and unrelated files, so the directory may remain. There is no implicit purge mode.

## Integrity and recovery

Checksums downloaded from the same release detect corruption and mismatched files; they do not independently authenticate a compromised publishing account. Binaries are not Authenticode-signed. The versioned release installers accept an independently obtained binary digest through Windows `-ExpectedSHA256 DIGEST` or Linux `--sha256 DIGEST`.

An update verifies the existing installation record and hashes, stages the selected release, retains originals and checks the replacement before committing. Ordinary failures roll back. Unknown or modified managed files are preserved by refusing the operation. Concurrent installers are excluded with an exclusive lock. Keep configuration and personal files out of the managed executable/license/script filenames.

Power loss or forced termination can leave `.mafsil-install-*` staging/backups and an `.install-lock`. Stop clients/installers, inspect the exact installation and saved originals, and recover needed files before removing a specific stale lock. A failed rollback reports the retained backup path. Updates are not crash-atomic filesystem transactions.

## Offline installation

Obtain the raw binary, `SHA256SUMS`, `LICENSE`, `THIRD_PARTY_NOTICES.md` and the platform's **versioned release installer** from one release. Run that installer with its explicit version and a local bundle directory:

```powershell
pwsh -NoProfile -File .\bundle\install.ps1 -Version v0.1.0 -BundleDirectory .\bundle
```

```sh
sh ./bundle/install.sh --version v0.1.0 --bundle ./bundle
```

The root bootstraps select/download releases and require network access; offline mode belongs to the release's installer. See [validation status](validation.md) for native tests, injected-failure fixtures and real HTTPS checks. No tests install Mafsil for use on the local development machine.
