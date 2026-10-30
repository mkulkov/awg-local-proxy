---
name: awg-go-netstack
description: "Use for AWG Local Proxy Go core, SOCKS5/HTTP forwarding, configuration parsing, userspace netstack, DNS, IPC or AmneziaWG compatibility. Do not use for UI-only or repository administration tasks."
---

# Go userspace proxy and AmneziaWG integration

## Scope and inputs

Read the applicable AGENTS files, CODEX_INSTRUCTIONS.md sections 4–6 and docs/LICENSE_AUDIT.md. Identify whether the task targets the product repository or the sibling component fork. Check the license gate before altering upstream-derived code. If blocked, implement only original contracts, state machines and MockCore, without copying upstream implementations.

## Workflow

1. Inspect go.mod/go.sum, the exact core/AWG commits and existing tests. Do not equate Go module major version with protocol version.
2. State the invariant changed by this task and write a focused failing test. Keep changes narrow and reviewable.
3. Keep destination TCP dialing and destination DNS inside the injected netstack dialer/resolver. Never fall back to host networking or default HTTP transports. The outer UDP endpoint and explicitly documented endpoint bootstrap DNS are separate paths.
4. Bind proxy listeners only to approved loopback addresses. Reject wildcard/public addresses and port conflicts. SOCKS5 BIND/UDP ASSOCIATE must fail explicitly unless separately implemented and tested.
5. Propagate context cancellation and deadlines through parsing, dialing, relay and shutdown. Bound input sizes, buffers, connections and retries; implement TCP half-close intentionally.
6. Keep config values compatible with the pinned engine. Validate key lengths, field support, duplicate keys, MTU, endpoint, peers and AllowedIPs. Never execute wg-quick hooks or install host routes. Unknown fields require explicit errors or reviewed supported behavior.
7. Bound .vpn/base64/zlib/JSON decoding. Treat client configs as untrusted and secrets. Do not retain administrative Amnezia exports.
8. For managed mode use versioned bounded NDJSON over inherited pipes. Protocol-only stdout, sanitized stderr. No config in args/env/temp files. EOF must shut down the child.
9. Report listeners-ready, handshake-confirmed and successful tunneled probe separately. Device.Up is not proof of connection. MockCore must never provide direct proxy access.
10. Run formatting, go vet, targeted tests, race where supported and parser/protocol fuzzing. Verify cancellation, disconnect, failed handshake, port reuse and absence of goroutine leaks.

## Output

Return changed files, pinned versions, actual test commands/results, uncovered scenarios and the smallest next technical blocker. Never claim real-server compatibility from a mock test. For changes to packet handling, include a network-path assertion that direct host dialing is impossible for destinations.

## References

Use official AmneziaWG source at the chosen commit, Go documentation and the project's source links. Do not copy arbitrary upstream snippets across the product/component boundary. This is an original project workflow, not an imported upstream skill.
