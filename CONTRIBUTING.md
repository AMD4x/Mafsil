# Contributing

## Report a problem or propose a change

Use the [issue chooser](https://github.com/AMD4x/Mafsil/issues/new/choose) for bugs, setup help and feature requests. Check existing issues first. A useful bug report includes the output of `mafsil version`, OS/architecture, MCP client, enabled capabilities, and minimal steps using disposable sample files. Share only sanitized logs and configuration excerpts. Security reports belong in the [private vulnerability channel](https://github.com/AMD4x/Mafsil/security/advisories/new).

For larger changes, open an issue describing the problem and proposed behavior before implementing it. Keep pull requests focused and describe what changed, why, and which checks actually ran. A small documentation correction can go directly to a pull request.

## Develop and validate

Use Go 1.27.0 or later and Python 3.12+ for the verification/packaging scripts. Windows native terminal tests require PowerShell 7 and Windows with ConPTY (Windows 10 1809+; current Windows versions are recommended). Linux native terminal tests require Bash and `/dev/ptmx`.

Keep build outputs, caches and fixtures outside the source tree. Select `GOCACHE`, `GOMODCACHE`, `GOTMPDIR` and `TEMP`/`TMPDIR` when a workspace requires it. Go tools may maintain telemetry counters in their user-config directory; redirect `APPDATA` or `XDG_CONFIG_HOME` for isolated development. Do not change global tool configuration to run project tests.

```sh
python scripts/verify.py --security
python scripts/verify.py --race
```

Race detection requires a supported C compiler. A missing compiler is not a successful race check. The CI matrix requires native race tests on each target. Unit tests, actual process/PTY tests and protocol fixtures run without production credentials or remote machines.

To fuzz the bounded terminal parser and portable path policy:

```sh
go test ./internal/terminal -fuzz=FuzzScreen -fuzztime=30s -parallel=2
go test ./internal/workspace -fuzz=FuzzName -fuzztime=30s -parallel=2
```

Prepare a local candidate in an empty directory outside the source tree:

```sh
python scripts/package.py --version v0.1.0 --output ../mafsil-candidate
python tests/distribution_test.py --bundle ../mafsil-candidate
python tests/installer_test.py --bundle ../mafsil-candidate -v
```

The package script cross-builds all three targets by default. `--target windows/amd64`, `linux/amd64` or `linux/arm64` selects a target. Building is not native testing. Windows icon/version resources use `goversioninfo v1.7.0` in staging; no `.syso` is written to the checkout.

Installer tests always use offline bundles and disposable explicit destinations. Never replace them with a real local install, service registration, persistent PATH/Registry change or a test on a private remote host. Linux installer tests run natively in Linux CI.

For dependencies, update pinned versions deliberately, run `go mod tidy`, regenerate notices with `python scripts/licenses.py`, and rerun security checks. The direct runtime dependencies are the official MCP SDK (protocol compatibility and schema validation) and `golang.org/x/sys` (native handles, PTY and file metadata). The complete notices include transitive linked modules and the Go runtime. Build-only resource tools are not linked into Mafsil.

Changes to file or process behavior need focused regression tests for failure paths as well as success. Preserve byte-level file semantics, process ownership, bounded resources and truthful errors. Do not add a convenience bypass that silently expands authority. Describe platform differences explicitly.

Brand geometry lives in `assets/brand/generate.py` and editable SVG masters; `assets/brand/render.cjs` exports high-resolution PNGs and optical ICO frames. See the [asset guide](assets/brand/README.md). Maintain light/dark contrast, check the real 16–64 px icon frames, and inspect both diagram layouts. No font files are bundled. PNG and ICO exports are intentional distribution assets.

Publication and release are separate maintainer decisions. The [release runbook](docs/releasing.md) documents exact-commit validation and the manual release gate.
