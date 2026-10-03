# Contributing

## Report a problem or propose a change

Use the [issue chooser](https://github.com/AMD4x/Mafsil/issues/new/choose) for bugs, setup help and feature requests. Check existing issues first. A useful bug report includes the output of `mafsil version`, OS/architecture, MCP client, enabled capabilities and minimal reproduction steps using disposable sample files. Share only sanitized logs and configuration excerpts. Security reports belong in the [private vulnerability channel](https://github.com/AMD4x/Mafsil/security/advisories/new).

For larger changes, open an issue describing the problem and proposed behavior before implementing it. Keep pull requests focused and describe what changed, why and which checks ran. Small documentation corrections can go directly to a pull request.

## Development requirements

Use Go 1.27.0 or later and Python 3.12+ for verification and packaging scripts. Windows terminal tests require PowerShell 7 and ConPTY-capable Windows. Linux terminal tests require Bash and `/dev/ptmx`.

Keep generated build outputs and test fixtures outside the source tree. Project tests are designed to use disposable paths and must not depend on production credentials, private remote machines or persistent system changes.

## Verify changes

Run the standard checks:

```sh
python scripts/verify.py --security
python scripts/verify.py --race
```

Race detection requires a supported C compiler. CI runs native race tests on Windows amd64, Linux amd64 and Linux arm64.

For the bounded terminal parser and portable path policy:

```sh
go test ./internal/terminal -fuzz=FuzzScreen -fuzztime=30s -parallel=2
go test ./internal/workspace -fuzz=FuzzName -fuzztime=30s -parallel=2
```

To exercise a packaged candidate locally, use an empty directory outside the source tree:

```sh
python scripts/package.py --version v0.1.0 --output ../mafsil-candidate
python tests/distribution_test.py --bundle ../mafsil-candidate
python tests/installer_test.py --bundle ../mafsil-candidate -v
```

`package.py` cross-builds all three targets by default; `--target windows/amd64`, `linux/amd64` or `linux/arm64` selects one. Cross-building does not replace native runtime testing.

Installer tests use disposable destinations and offline fixtures. The network bootstrap tests use temporary homes and public release downloads. They do not register services, make persistent PATH/Registry changes or require a private host.

## Dependencies and behavior changes

When changing dependencies, update pinned versions deliberately, run `go mod tidy`, regenerate notices with `python scripts/licenses.py`, and rerun security checks.

Changes to file or process behavior should include regression coverage for failure paths as well as success. Preserve byte-level file semantics, process ownership, bounded resources and accurate errors. Platform differences should be documented explicitly.

Brand source files live under `assets/brand/`; see the [asset guide](assets/brand/README.md) before changing logos, icons or diagrams.

See the [release process](docs/releasing.md) for packaging and publication.
