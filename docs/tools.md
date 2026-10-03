# Tool contracts

All file paths are slash-separated and relative to the operator's workspace. Absolute paths, `..`, Windows device names, alternate-data-stream syntax, links/reparse points, and `.mafsil-*` internal names are rejected. Files must be regular and have exactly one hard link. Names with control characters or characters invalid on Windows are not accepted on either platform.

Tools are advertised only when their capability is enabled. Their schemas are generated and validated by the MCP SDK. Operational failures are MCP tool errors (`isError: true`); protocol failures use JSON-RPC errors. Results provide structured content and a JSON text fallback.

## Inspect

`server_info {}` reports the version, OS/architecture, capability flags and limits. It omits usernames, machine names, credentials and absolute workspace paths.

```json
{"path":"src","limit":100,"offset":0}
```

`list_directory` returns `entries`, `truncated` and `nextOffset`. Entries report name, kind and size. Follow `nextOffset` to page through a stable directory. Enumeration order comes from the filesystem; concurrent directory edits may cause repeated or skipped entries. Blocked links are labeled, not traversed.

```json
{"path":"src/main.go","format":"text","offset":0,"limit":4096}
```

`read_file` returns the complete-file `sha256`, file size, text, `nextOffset` and `truncated`. Offsets and limits count bytes. An offset must be a UTF-8 boundary, and the returned text never ends in a partial character. The whole file must fit `fileBytes`; oversized files are rejected rather than sampled under an unreliable revision hash. Text containing NUL or invalid UTF-8 requires `format: "resource"`.

`format: "image"` returns native MCP image content for PNG, JPEG, GIF or WebP detected from bytes. `format: "resource"` returns an embedded resource with a `mafsil://workspace/…` URI. Both have a 2 MiB content limit. A client's support determines whether embedded resources appear as downloadable attachments. These URIs do not create a public download endpoint.

## Edit

`create_directory {"path":"src/new"}` creates exactly one directory; parents must exist. It does not recursively create or delete trees.

`edit_files` accepts a `changes` array. Each path, including a move destination, can occur only once per batch. Read existing files first and send their exact hash. Use `"missing"` only for a new file.

Create a file:

```json
{
  "changes": [{
    "operation": "write",
    "path": "notes.txt",
    "expectedSha256": "missing",
    "content": "One line\n"
  }]
}
```

Replace exact text without normalizing unrelated bytes:

```json
{
  "changes": [{
    "operation": "edit",
    "path": "notes.txt",
    "expectedSha256": "<64-character hash from read_file>",
    "edits": [{"old":"One line", "new":"A revised line", "count":1}]
  }]
}
```

Each `count` is the exact number of occurrences expected at that step. Ambiguity is an error. Edits within a file are applied in array order. To replace a whole existing file, use `operation: "write"`, its hash and `content`.

Use `operation: "delete"` with the source hash to remove one regular file. Use `operation: "move"` with the source hash and `destination` to move to an absent destination. A move can include exact edits. A move without edits also accepts binary content.

The structured patch format avoids a second text-patch parser, makes revision requirements explicit and preserves mixed line endings and unterminated final lines. It is not unified diff or the `*** Begin Patch` format.

All operations are prepared and rechecked before commit. IO failure triggers rollback; concurrent external changes can prevent a safe rollback, in which case the error identifies retained backups. Once commit starts, cancellation does not interrupt it. A cancelled request or broken connection is never proof that the files remained unchanged. Reread them before retrying. See [transaction limits](security.md#file-mutations).

## Execute

```json
{"command":"go test ./...","cwd":".","waitMs":1000,"timeoutSeconds":120}
```

`exec_command` requires exactly one form:

- `command`: a script in the configured PowerShell 7 or Bash shell.
- `program` and optional `args`: an absolute executable path and a literal argument vector.
- `interactive: true`: a persistent shell; requires `tty: true`.

Commands use no shell profiles. A script's own shell semantics determine its exit code; Mafsil does not rewrite sequences or invent per-statement statuses. Explicit program arguments bypass shell interpolation. User input still needs careful quoting when deliberately constructing a shell script.

`cwd` is the starting directory, not a containment boundary. Executable selection and commands can reach outside the workspace with the server account's rights. File-tool permissions do not restrict command behavior.

`tty: true` creates a native terminal. Optional dimensions default to 100×30; allowed dimensions are 20–300 columns and 5–100 rows. Native terminal streams merge stderr into stdout. Non-terminal processes retain stdout and stderr separately.

`waitMs` defaults to zero (return promptly), with a maximum of 30,000. Every command returns a `sessionId`, `running`, `pid`, retained stdout/stderr and offsets. Completed sessions include an exit code. Timeout, idle expiry, explicit close and input failure also set a reason.

## Continue or close a session

```json
{
  "sessionId":"<returned id>",
  "input":"hello\n",
  "waitMs":500,
  "stdoutOffset":123,
  "stderrOffset":0
}
```

`session_io` writes input exactly: it never adds a newline. Empty input polls. For a terminal, send raw keys such as `"\u001b[B"` (down arrow) or `"\r"` (Enter). `columns` and `rows` resize a live terminal. `closeInput: true` closes a pipe's stdin after any supplied input; terminal half-close is rejected.

Offsets returned in one response are the next byte positions to request. Omit offsets to replay all retained output. If a buffer has rolled over, `stdoutDropped`/`stderrDropped` identify bytes no longer available. UTF-8 fragments in arbitrary process output are replaced for JSON display; offsets still count the original bytes.

A screen snapshot is bounded, plain-text and approximate: common cursor movement, erase, scrolling, alternate-screen and split UTF-8 sequences work. Color, full grapheme width, terminal queries and rich terminal graphics are not emulated. Raw VT output remains available. Do not execute OSC sequences from tool output in a trusted terminal.

Cancelling a poll leaves the session running. Cancelling a blocked input write terminates it, because delivery may be partial. Input writes are serialized and time out after three seconds. `list_sessions {}` lists status; `close_session {"sessionId":"…"}` terminates the managed tree and returns final output. Do not use unrelated PIDs for cleanup.
