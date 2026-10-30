# Real-core integration verification — 2026-10-30

This section records the earlier baseline; subsequent changes and their verification are listed separately below.

Windows amd64, Go 1.27.3, Fyne v2.8.2, LLVM MinGW 20261020. Independent core commit `1592207a98f0c516e8f99eea49e169f4bcae692f`; official AmneziaWG `v3.1.20260828` commit `b5928efb6ca19f0153958460c3d141f04abc5c2e`. Only ephemeral synthetic test keys and a userspace test peer were used; the user's server and profile were not used.

The core startup initialization race detected by GitHub CI was fixed at the pinned commit without changing the official engine or cryptography. The final full core suite passed after that fix, including 100 cycles, deterministic legacy-server DNS retry and the actual product-supervisor lab. Targeted race, startup offset, no-UDP-during-validation and close-before-config checks passed. See the core repository test results for timings and scope.

Passed:

- Core: real handshake, in-tunnel DNS, SOCKS5 domain CONNECT, HTTP forward and CONNECT, negative direct-fallback case, peer loss, port-conflict handling and child EOF cleanup. A normal 100-cycle connect/handshake/probe/disconnect stress passed before the later relay-idle correction; targeted real network, process and race tests passed again after that correction. See the core repository `TEST_RESULTS.md` for exact scope.
- Product session controller → `RealSupervisor` → separately built core → synthetic userspace AWG peer: handshake, SOCKS5/HTTP requests and process/listener shutdown, including a targeted race run. Configuration and ephemeral keys went through inherited stdin, never environment/arguments/files. Independently built core also passed the product's `TestBuiltCoreControlIntegration`.
- GUI/product: `go test -tags ci ./...`, `go vet -tags ci ./...`, and `go test -race -tags ci ./...`; owner-death Job Object and DPAPI CurrentUser tests included. UI port-selection tests cover loopback-only addresses, numeric limits and port conflicts; without saved autoconnect, port values reset to defaults on restart.
- Endpoint import accepts numeric IPs and valid ASCII DNS hostnames without resolving them during import; syntactic rejection, CRLF and surrounding-whitespace compatibility, vet and race tests passed. The separate VPN endpoint bootstrap may use the system resolver; destination DNS remains in the tunnel.
- Windows DACL tests check descriptor control, ACE type/count, mask and binary SID equality, including SDDL aliases and unsafe-descriptor regressions; production access rights were not weakened.
- Opt-in startup/reconnect additions: full product tests, vet and race passed. Autostart tests used isolated HKCU test keys, never the real Run entry; preferences used temporary files with DPAPI/DACL. Session tests covered bounded retries, cancellation, late factory, failed Close and stale status. Real sleep/resume and a user-selected server remain untested.
- Core `go vet`, `go mod verify`, targeted race, parser and IPC fuzz smoke passed. 100 cycles were not run under race; long-term handle/heap growth was not measured.
- Snapshot comparison of Windows routes, adapters, DNS, WinINET proxy and WinHTTP proxy before/after a synthetic core lab was identical. Windows displayed a firewall permission prompt for `core.test`; no permission was accepted and no firewall rule was changed by this work.
- `govulncheck` v1.1.4 after updating GUI `golang.org/x/image` to v0.43.0 found no reachable vulnerabilities. It still reported 11 vulnerabilities in imported packages and 12 in required modules without a detected call path. Core scan found no reachable vulnerabilities, with 2 in imported packages and 36 in required modules without a detected call path. These are scanner results, not a guarantee of absence of vulnerabilities.
- Visible real-mode window smoke: title and profile/probe fields, refusal to connect without a profile, close-to-tray and second-launch restore. That run preceded the final dependency rebuild and last deletion-race fix. Full DPI, accessibility and graceful tray Quit acceptance remain outstanding.

The binary is unsigned. Hash pinning detects mismatch between the GUI and adjacent core but does not authenticate an unsigned build. Notices for this exact dependency graph and toolchain are in `THIRD_PARTY_NOTICES.md` and `third_party/licenses/`; the owner selected MIT for the independent product and core. At the time of this baseline no public binary release had been made.

---

# Independent demo verification — 2026-10-30

Environment: Windows amd64, Go 1.27.3; local LLVM MinGW 20261020 UCRT x86_64 C compiler. Fyne v2.8.2 is the GitHub latest non-prerelease checked during implementation. No global Go/Codex settings were changed by this implementation.

Passed:

- `go mod tidy` resolved the original independent module and generated dependency checksums in `go.sum`.
- `gofmt` on `cmd` and `internal`.
- `go test -tags ci ./...`: application instance activation, protocol bounds/version/duplicate fields, mock non-connectivity, supervised child handshake and 100 connect/disconnect exchanges, idempotent close, hash/path rejection, storage separation/rename/delete/schema errors/encryption failures, restrictive import and actual Windows CurrentUser DPAPI synthetic roundtrip/corruption rejection.
- `go vet -tags ci ./...`.
- Normal Windows GUI build: `go build -trimpath -ldflags="-H=windowsgui" -o dist/AwgLocalProxy-demo.exe ./cmd/awg-local-proxy`.
- Independent network-free fixture: `go build -trimpath -o dist/awg-mock-core.exe ./cmd/awg-mock-core`.
- Import fuzz smoke: `go test ./internal/profiles -run '^$' -fuzz '^FuzzValidateImport$' -fuzztime 3s -parallel 2` — 33,137 executions, pass.
- Protocol fuzz smoke: `go test ./internal/coreclient -run '^$' -fuzz '^FuzzReadMessage$' -fuzztime 3s -parallel 2` — 40,447 executions, pass.

Artifacts (unsigned demo only):

| File | SHA-256 |
|---|---|
| `dist/AwgLocalProxy-demo.exe` | `9f31cbad52778b8225d65c8de265f2d2c0635617d10fc9496aeebc8e2f193545` |
| `dist/awg-mock-core.exe` | `3f1fa711978bf06f96520c24290903509d97eaee9cb7fa29af04481eff513f0c` |

Not run by the implementation worker: manual visible GUI/tray/DPI/accessibility acceptance, real GUI crash cleanup, cross-user DPAPI denial, network setting snapshots, race detector, vulnerability/license release audit, real core/server/handshake/SOCKS5/HTTP/DNS tunnel tests. DPAPI and instance tests ran as Windows unit tests; they do not establish full Windows acceptance. Tests use synthetic data only. No real configuration was requested or transmitted.

At the time of this earlier demo, integration of the originally proposed upstream was blocked by its unresolved license; the independent core was selected later. Headless `ci` tests and a successful Windows compile are not evidence of a working VPN.

## Windows visual smoke (2026-10-30)

The built GUI opened on Windows and displayed its explicit demo warning and disabled proxy addresses. The demo action changed the label to indicate that no tunnel exists; stopping returned it to the stopped state. After terminating the GUI process started for this check, its owned mock child exited. This is a narrow smoke check, not a tray, graceful-exit, DPI or accessibility acceptance.

## Import and lifecycle changes — 2026-10-30

Synthetic tests exercise Amnezia VPN envelope parsing, bounds, metadata stripping and malformed input rejection. No actual exported user profile was used; the tests do not establish compatibility with every export variant. The independent core tests exercise AWG 3.1 header protection, padding, numeric ranges and actual userspace peer traffic using generated test keys.

Windows power registration/unregistration and synthetic suspend/resume events are tested automatically, including reconnect opt-in and cancellation. Physical suspend/resume was not tested at the owner's request; asynchronous notification does not prove that the child stops before hardware enters sleep. The actual user's server and profile were not used for development tests.
