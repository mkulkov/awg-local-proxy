# AWG Local Proxy control protocol v1

This independently authored protocol is a proposed future core interface. Upstream does not implement it. The demo implements a deliberately restricted subset and never accepts a real configuration or opens network listeners.

Transport: inherited stdin/stdout pipes, one UTF-8 JSON object per line, maximum 65536 bytes excluding newline. No TCP control socket. Closing stdin terminates the mock process. stdout contains protocol only; errors must never contain credentials. Unknown envelope properties, unsupported versions and oversized input terminate the session. IDs are nonempty strings up to 64 bytes.

Envelope: `{"protocol_version":1,"type":"get_status","id":"1","payload":{}}`.

Commands: `hello`, `validate_config`, `connect`, `disconnect`, `get_status`, `shutdown`. Demo requires an empty object payload for every command. `validate_config` always returns `error` with `real_config_not_supported_in_demo`; no real client profile is accepted. Unknown commands return `unsupported_command`. Responses echo IDs; `hello` returns protocol/core versions, source identity and capabilities; connect/disconnect/get_status return `status`; shutdown acknowledges and exits. Errors contain a stable code and no input reflection.

Status has `state`, `demo`, `listener_ready`, `tunnel_verified`, optional `handshake_at` and a safe `message`. State values: Stopped, Starting, Connecting, Connected, Degraded, Reconnecting, Stopping, Error. Connected requires an actual listener, confirmed handshake and successful explicitly configured tunnel probe. A demo may never claim Connected, listener readiness, handshake or tunnel verification. In demo, connect goes to Degraded with a visible licensing explanation; disconnect goes to Stopped.

Future real core: connect/validate_config carry a bounded configuration only through stdin; never argv/env/files/clipboard. Asynchronous state/error/counter events require a separate typed reader and request correlation; not yet implemented by this request/response demo. Counters must distinguish TCP payload and encrypted transport. No destination or key material may appear in replies.

The supervisor only launches its own absolute executable as a separate `--mock-core` child, comparing its bytes to the GUI executable's SHA-256. This detects a mismatched demo file; it is not a signed supply-chain trust guarantee. The child has no network package or proxy implementation. EOF handles normal parent death. Real-core activation additionally requires provenance, a reviewed binary lock, Windows Job Object kill-on-close, configuration validation and licensed core support.
