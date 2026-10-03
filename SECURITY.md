# Security policy

Before the first release, only the development tree exists. After release, security fixes target the latest published version. A report about an older version is welcome if the issue still applies.

Use the repository's **Security → Report a vulnerability** flow when private vulnerability reporting is enabled. Do not open a public issue containing exploit details, private files, keys or identifiers. If that private channel is unavailable, contact the maintainer through an existing private channel before sending sensitive material; this project does not claim a monitored security email address.

Include the Mafsil version, OS/architecture, enabled capabilities, a minimal synthetic reproduction, expected behavior and observed impact. Use fixture directories and placeholder data. Do not test against another person's machine or expose a real MCP endpoint to demonstrate a report.

Enabling execution grants the server account's command authority. Access outside the workspace through an enabled command is documented behavior, not an execution sandbox escape. File-tool path escape, unintended credential forwarding, resource-limit bypass and process/installer ownership failures are security-relevant.

See [the threat model](docs/security.md) and [validation status](docs/validation.md). Private reporting must be configured at publication; it has not been enabled by local development.
