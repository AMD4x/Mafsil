# Publication and release runbook

Repository publication and release publication require separate maintainer decisions. A logo approval, local test result, repository creation or successful push does not approve a release.

## Local preparation

Finish source, tests, documentation, identity, package scripts and workflows locally. Keep application binaries outside Git. Verify source hygiene, notices, native tests and candidate contents. Review the actual working tree/history and present a local-readiness report before asking to publish the repository.

The initial version is **0.1.0**: the API is useful but deliberately pre-1.0 while platform/client feedback is gathered. The proposed tag is `v0.1.0`. Do not reuse an unrelated project's history or versions.

## Repository publication

Only after explicit repository-publication approval:

1. Publish the reviewed source to the intended Mafsil repository.
2. Observe the `CI` workflow on that exact commit.
3. Fix actionable failures locally, push fixes and repeat until all required jobs pass.
4. Enable private vulnerability reporting and configure required reviewers on the `release` environment.
5. Keep pre-release README wording accurate; download commands cannot work until assets exist.

The required native jobs are Windows amd64, Linux amd64 and Linux arm64. They include tests, race detection, static/vulnerability checks, source hygiene, package generation, binary/stdio verification and installer fixtures. The separate fuzz job must pass too. Standard GitHub-hosted runners are used; no larger or paid runner labels are required.

Do not treat a cross-compile or an unexecuted test as a native pass. If a genuine external blocker prevents required validation, report it and stop before creating a misleading release.

## Release proposal and approval

After CI passes, prepare a complete proposal using the reviewed [notes](releases/v0.1.0.md), exact source commit and CI run ID, title, asset names and checksums, final install/update commands and known limitations. Assemble the native candidates with `scripts/assemble.py` and record the SHA-256 of the resulting `SHA256SUMS` file itself. That digest binds approval to every reviewed asset. Obtain explicit release approval. A draft release is still a GitHub-side release action and must not be created merely to request review.

Expected v0.1.0 assets:

- `mafsil_v0.1.0_windows_amd64.exe`
- `mafsil_v0.1.0_linux_amd64`
- `mafsil_v0.1.0_linux_arm64`
- `install.ps1`, `install.sh`
- `LICENSE`, `THIRD_PARTY_NOTICES.md`
- `BUILD-INFO_windows_amd64.json`
- `BUILD-INFO_linux_amd64.json`
- `BUILD-INFO_linux_arm64.json`
- `DEPENDENCIES.json`
- `SHA256SUMS`

No independent signing certificate/key is configured. Describe SHA-256 integrity accurately; do not call it independent publisher authentication. GitHub also generates source archives for the tag.

## Publish the approved release

Dispatch the manual **Approved release** workflow from `main` with four reviewed inputs: `source_commit`, `ci_run_id`, `manifest_sha256`, and `approval` set to `publish-v0.1.0`. Its first job verifies that the selected run is the repository's `CI` workflow, succeeded on `main`, and tested the approved commit. The protected `release` environment requires maintainer review.

The workflow downloads artifacts from that exact run, checks every manifest and common file, requires the expected target/version/commit in each build record, and verifies the assembled manifest against the approved digest before copying any release payload. A rerun that changes artifact bytes cannot silently replace reviewed assets. It does not rebuild binaries. `gh release create` creates the approved tag/release only at this stage. Ordinary pushes, pull requests and tag pushes do not trigger this workflow.

After release publication, the workflow downloads all released assets and verifies their bytes. Three standard native runners then exercise the published binary and actual HTTPS installer download/update/uninstall path in temporary fixtures. Require those jobs to succeed, confirm public repository/release accessibility and update README pre-release wording. Real tunnel/account validation is separate and must never use undisclosed credentials or private machines as an implicit test target.

If artifact retention expires (14 days), rerun CI at the approved source commit, reassemble and recheck the assets, and refresh the run ID and manifest digest in the proposal before approval. If an approved proposal's artifact bytes, version, notes, title or asset names materially change, present the revised proposal before publishing it.
