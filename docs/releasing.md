# Release process

Mafsil releases are published from artifacts produced by native CI for a specific source commit. The release workflow verifies the selected CI run and the assembled SHA-256 manifest before publishing anything.

## Prepare a release

1. Update `VERSION`, `CHANGELOG.md` and the matching release notes under `docs/releases/`.
2. Run the local verification commands documented in [CONTRIBUTING.md](../CONTRIBUTING.md).
3. Push the release commit and wait for the `CI` workflow to pass on that exact commit.

CI builds native candidates for Windows amd64, Linux amd64 and Linux arm64. It also runs unit/integration tests, race detection, static and vulnerability checks, package validation, installer fixtures and bounded fuzzing.

## Verify release artifacts

Download the three `candidate-*` artifacts from the successful CI run and assemble them in an empty directory:

```sh
python scripts/assemble.py \
  --input ../ci-candidates \
  --output ../mafsil-release \
  --commit <40-character-commit-sha> \
  --version v<version>
```

The assembler verifies target metadata, common files and every candidate checksum before producing the final `SHA256SUMS`. Record the SHA-256 digest of that assembled `SHA256SUMS` file; the release workflow uses it to ensure that the published payload matches the files you verified.

Expected release contents are the three native binaries, platform installers, license/notices, three build-info files, `DEPENDENCIES.json` and `SHA256SUMS`.

## Publish

Run the manual **Release** workflow from `main` with:

- `source_commit`: the exact commit that passed CI.
- `ci_run_id`: the successful `CI` workflow run for that commit.
- `manifest_sha256`: the SHA-256 digest of the assembled `SHA256SUMS`.

The workflow reads the release version from `VERSION`, verifies that the selected CI run succeeded on `main` for the supplied commit, downloads its artifacts, reassembles them, checks the manifest digest and creates the matching GitHub release using `docs/releases/v<version>.md`.

The `release` environment can use normal GitHub environment protection if desired. Ordinary pushes, pull requests and tag pushes do not publish releases.

## Post-release verification

After publication, the workflow downloads the public release assets and checks their hashes. Native Windows amd64, Linux amd64 and Linux arm64 jobs then exercise the published binary plus the real HTTPS install, update and uninstall path in temporary directories.

If CI artifacts have expired, rerun CI on the intended source commit and verify the newly generated artifacts before publishing. Any change to source, version, release notes or artifact bytes requires a fresh verification pass.
