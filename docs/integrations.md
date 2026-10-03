# MCP clients and optional tunnels

## Local clients

Configure your client to launch the absolute Mafsil executable with:

```text
serve --config /absolute/path/to/Mafsil/config.json
```

Use separate command/argument fields where the client supports them. Do not put shell syntax into an executable-path field. The client should retain stdin/stdout pipes and terminate the child cleanly on disconnect. stdout is exclusively MCP; diagnostic startup errors go to stderr.

Mafsil uses the official Go SDK's supported revisions: `2026-07-28`, `2025-11-25`, `2025-06-18`, `2025-03-26` and `2024-11-05`. Modern clients use per-request metadata and `server/discover`; older clients use `initialize`. Tool schemas and capability gating are identical across revisions.

## OpenAI Secure MCP Tunnel

The optional integration runs the **official `tunnel-client`** as the transport owner and Mafsil as its stdio server. Mafsil does not bundle the tunnel runtime or implement its control plane. The [official guide](https://developers.openai.com/api/docs/guides/secure-mcp-tunnels) documents current availability, organization/workspace permissions and authentication.

1. Obtain the client from the download linked by Platform tunnel settings or its [official release page](https://github.com/openai/tunnel-client/releases/latest).
2. Follow its credential-handling guidance. Keep the runtime key out of command arguments, Mafsil configuration and repositories.
3. Use `tunnel-client help quickstart` and a local stdio profile. The child command is the absolute Mafsil binary followed by `serve --config` and the absolute configuration file.
4. Run the upstream profile's doctor/readiness check before selecting the tunnel in the supported OpenAI product.

For an upstream client exposing the documented profile CLI, a Linux-shaped example is:

```sh
tunnel-client init --sample sample_mcp_stdio_local --profile mafsil \
  --tunnel-id '<YOUR_TUNNEL_ID>' \
  --mcp-command '"/absolute/path/to/Mafsil/mafsil" serve --config "/absolute/path/to/Mafsil/config.json"'
tunnel-client doctor --profile mafsil --explain
tunnel-client run --profile mafsil
```

On Windows, provide equivalent absolute Windows paths inside the quoted command and follow the installed client's parsing rules. Inspect its help for version-specific options; this repository does not store a profile with real identifiers or keys.

The upstream client opens an outbound HTTPS connection; Mafsil itself listens on no port. Authentication and reachability are the tunnel client's responsibility. A public GitHub project does not make each user's local MCP endpoint a public plugin endpoint.

The standard stdio behavior is tested locally with protocol fixtures and the official SDK client. Real tunnel credentials, hosted account permissions, upstream readiness and end-to-end OpenAI product connectivity have **not** been exercised. Those checks require a separately authorized operator environment. No live tunnel is used in the test suite.

## Other remote adapters

A standards-compatible adapter can launch the same stdio command. It must provide suitable authentication, authorization, message limits and process cleanup. Do not expose an unauthenticated stdio-to-HTTP bridge to the network. Adapter deployment and service management are deliberately outside Mafsil's installer.
