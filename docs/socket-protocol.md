# Agent IPC Socket Protocol

LugoTUI provides an optional Unix Domain Socket interface using a **JSON Lines** protocol, designed for programmatic UI inspection, external test automation, and bi-directional integration with AI agents.

---

## 1. Connection & Transport Security

- **Socket Creation:** Configured via `--socket /path/to/socket.sock` in the CLI or `Options.SocketPath` in Go.
- **Filesystem Permissions:** The socket is created with strict file permissions **`0600`** (read/write only by the creating user). Existing socket paths are not overwritten.
- **Connection Limits:** Supports up to **4 concurrent connections**.
- **Timeouts:** Enforces **2-second read and write deadlines** per operation.
- **Thread Safety:** Runtime message processing remains strictly serialized on the runtime owner goroutine.
- **Lifecycle:** The managed runner automatically closes all active client connections and unlinks the socket file on shutdown.
- **Low-level Embedding:** Embedders can decode JSON Lines and feed requests directly to `Runtime.AgentReply(data []byte) []byte` without running the network listener.
- **Security Notice:** The socket provides no network authentication. Do not expose this socket to untrusted local users or bridge it over unauthenticated networks. While `snapshot` excludes internal model values, it is not a general secret-redaction mechanism.

---

## 2. Protocol Wire Format

Messages are exchanged as single-line JSON objects terminated with a newline (`\n`). Maximum line length is **1 MiB** (excluding the line delimiter).

### Request Methods

#### 1. Snapshot (`snapshot`)
Inspect a flat semantic node list, focus, bounds, and protocol version. The current beta uses the exact case-sensitive field names shown below.
```json
{"method":"snapshot"}
```

Response payload includes:
- `protocol_version`: `1`
- `Revision`: Current integer revision counter. Use this value as lowercase `packet.revision` in an `apply` request.
- `Focus`: ID of the focused node.
- `Nodes`: Flat preorder list, including unnamed nodes. Each entry has `ID`, `Type`, `Bounds` (`X`, `Y`, `W`, `H`), and `Children` (child IDs, or `null` for a leaf). Unnamed IDs can be empty; this is not a complete reconstructable widget tree.

The snapshot does not include widget text, arbitrary properties, or the internal model. `--snapshot` prints this snapshot object directly; the socket wraps it in `{"ok":true,"result":...}`. Do not expect lowercase `revision`, `focus`, or a `tree` field in the snapshot.

#### 2. Apply Mutations (`apply`)
Stage transactional updates against a specific tree revision:
```json
{
  "method": "apply",
  "packet": {
    "revision": 0,
    "operations": [
      {
        "op": "patch",
        "id": "answer",
        "props": { "text": "Hello" }
      }
    ]
  }
}
```
Supported operations in the packet:
- `"op": "patch"`: Updates specified properties of node `id`.
- `"op": "replace"`: Replaces the entire subtree at node `id` with a new node definition in `tree`, for example `{"op":"replace","id":"answer","tree":{"type":"text","id":"answer","props":{"text":"Hello"}}}`. Node definitions use lowercase `type`, `id`, `props`, and optional `children`; they are not snapshot `Nodes` entries.

#### 3. Dispatch Message (`message`)
Trigger a named Lua action with an optional payload dictionary:
```json
{
  "method": "message",
  "action": "agent.text",
  "payload": { "text": "fragment" }
}
```

#### 4. Drain Host Events (`events`)
Retrieve queued events emitted by Lua via `host.dispatch`:
```json
{"method":"events"}
```
*Note:* The `events` call destructively drains the internal event queue (at-most-once delivery). If `OnEvent` is configured in Go, it consumes host events exclusively, leaving the socket queue empty.

---

## 3. Responses & Error Codes

Every response is a JSON object with a boolean `ok` field. Clients must branch on the machine-readable `code` field rather than parsing human-readable `error` strings.

### Success Response
```json
{"ok": true, "result": { ... }}
```

### Error Response
```json
{
  "ok": false,
  "code": "validation_error",
  "error": "node with id 'answer' not found"
}
```

### Complete Error Code Reference

| Error Code (`code`) | Meaning & Typical Cause |
| --- | --- |
| `invalid_json` | Malformed JSON syntax, incomplete lines, or trailing garbage characters. |
| `unknown_command` | Unrecognized `"method"` value (must be `snapshot`, `apply`, `message`, or `events`). |
| `validation_error` | Unknown fields or operations, invalid tree structure, unsupported node properties, or invalid action payload. |
| `revision_conflict` | The `revision` supplied in the `apply` packet does not match the runtime's current `Revision()`. Client must fetch a new snapshot. |
| `lua_error` | Execution of the target Lua action failed or exceeded the 50 ms execution deadline. |
| `task_limit_exceeded` | The engine's pending task limit (32 concurrent workers) has been reached. |
| `internal_error` | Unclassified runtime or serialization failure. |
