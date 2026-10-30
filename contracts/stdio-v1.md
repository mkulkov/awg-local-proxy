# AWG Local Proxy control protocol v1

This original protocol is transported over inherited stdin/stdout as one UTF-8 NDJSON object per line, maximum 65536 bytes excluding newline. stdout contains protocol only. The core rejects malformed frames, unknown or differently cased properties, duplicate properties, unsupported versions and oversized messages. No TCP control listener is used. EOF closes the child and its resources. Errors return stable codes and never echo input.

Envelope: `{"protocol_version":1,"type":"get_status","id":"1","payload":{}}`. IDs are nonempty strings at most 64 bytes.

Real-core commands:

- `hello` with `{}` returns `hello` containing protocol/core versions, identity and capabilities.
- `validate_config` accepts `{"config":"..."}` only over stdin and returns `validated` or `error`.
- `connect` accepts `config`, `socks_address`, `http_address` (empty disables HTTP), `probe_url` and `timeout_seconds`, and returns `status` or `error`. Config is never passed through command-line arguments, environment, temporary plaintext files or logs.
- `get_status`, `check_tunnel` and `disconnect` accept `{}` and return `status`. `check_tunnel` repeats the explicitly configured probe; `get_status` sends no network probe. `shutdown` returns an acknowledgement and exits.

Status includes `state`, `demo`, `listener_ready`, `tunnel_verified`, optional `handshake_at`, `socks_address`, `http_address`, and a safe `message`. `Connected` requires actual listener readiness, a real handshake, and success of the user-specified tunnel probe. Degraded means a failure was observed; no direct network fallback is attempted. A past successful probe does not establish continuous reachability.

Demo mode accepts empty payloads only, uses its own process, never opens proxy ports and cannot claim a handshake or tunnel. The real supervisor checks a build-pinned executable hash and Windows Job Object. Hash matching alone does not establish binary authenticity. Events and counters are not yet part of this implementation.
