# Security policy

Security fixes target the latest published version. A report about an older version is welcome if the issue still applies.

Use [Security → Report a vulnerability](https://github.com/AMD4x/Mafsil/security/advisories/new) to send a private report. Private vulnerability reporting is enabled for this repository. Do not open a public issue containing exploit details, private files, keys or identifiers. If that private channel is unavailable, contact the maintainer through an existing private channel before sending sensitive material; this project does not claim a monitored security email address.

Include the Mafsil version, OS/architecture, enabled capabilities, a minimal synthetic reproduction, expected behavior and observed impact. Use fixture directories and placeholder data. Do not test against another person's machine or expose a real MCP endpoint to demonstrate a report.

Enabling execution grants the server account's command authority. Access outside the workspace through an enabled command is documented behavior, not an execution sandbox escape. File-tool path escape, unintended credential forwarding, resource-limit bypass and process/installer ownership failures are security-relevant.

See [the threat model](docs/security.md) and [validation status](docs/validation.md). For ordinary bugs or setup questions, use the [public issue forms](https://github.com/AMD4x/Mafsil/issues/new/choose).
