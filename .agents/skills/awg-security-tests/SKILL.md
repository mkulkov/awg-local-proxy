---
name: awg-security-tests
description: "Use for AWG Local Proxy threat modeling, secret storage, DPAPI, hostile configs, IPC boundaries, DNS leaks, fail-closed behavior and security acceptance tests. Not a general UI styling skill."
---

# Security boundaries and acceptance tests

## Scope

Read CODEX_INSTRUCTIONS.md sections 5–7 and 10. Threat-model the actual implementation; mark planned controls as planned. Separate developer/CI risks from runtime risks.

## Trust boundaries

Model: local application to proxy; GUI to core via inherited pipes; untrusted imported configuration; userspace netstack to outer UDP socket; local encrypted storage; upstream/source to build/release. Loopback is not process identity. DPAPI CurrentUser is not protection from malware with the same user's rights.

## Workflow

1. Enumerate assets: client private keys/PSK, profile metadata, visited destinations, binary integrity and user's unchanged network configuration. Identify feasible attackers without inventing capabilities.
2. Ensure DPAPI uses user scope, restrictive Windows ACLs and atomic writes. Corrupted/undecryptable storage must fail without plaintext fallback. Minimize key copies and lifetime; never promise absolute memory erasure in Go.
3. Audit args, environment, stdout/stderr, exception paths, IPC replies, screenshots, crash reports, git staging and archives for secrets. Administrative Amnezia exports must not be retained wholesale.
4. Validate all inbound data sizes before allocation/decompression. Reject malformed .vpn/INI, decompression bombs, duplicate/unknown critical fields, unexpected peers and executable hooks. Fuzz parsers and SOCKS/HTTP framing.
5. Prove destination dial and DNS use only the tunnel. Test stopped core, loss after connect, wrong keys, unreachable endpoint, NXDOMAIN, IPv6-unavailable and disallowed AllowedIPs. Verify there is no host/DIRECT fallback.
6. Document and isolate outer UDP plus endpoint bootstrap DNS. Distinguish app-side DNS leakage before proxy use from core behavior; do not promise control over WebRTC/QUIC or applications ignoring the proxy.
7. Keep control off public TCP ports. Validate NDJSON schema/version/message sizes; no arbitrary command execution. Ensure EOF and parent death close the child and listeners.
8. Snapshot system routes/adapters/DNS/WinINET/WinHTTP before and after. Attribute unrelated DHCP changes correctly. Test physical egress using a controlled lab rather than a single public IP website.
9. Review loopback access by other local processes/users, resource exhaustion, executable path hijack and package tampering. Authentication may reduce accidental access but is not same-user malware isolation.
10. Run secret scanning and dependency checks. Record findings with concrete file/line evidence, realistic impact, mitigation and a regression test. Do not suppress findings merely to pass CI.

## Deliverables

Write a concise threat model and a reproducible test matrix, listing actual passes/failures/not-run. No production credentials in public CI. Do not publish user logs/pcaps or send issue reports automatically.
