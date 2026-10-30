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

Real-core integration/release remains blocked by the upstream license. Headless `ci` tests and a successful Windows compile are not evidence of a working VPN.

## Windows visual smoke (2026-10-30)

The built GUI opened on Windows and displayed its explicit demo warning and disabled proxy addresses. The demo action changed the label to indicate that no tunnel exists; stopping returned it to the stopped state. After terminating the GUI process started for this check, its owned mock child exited. This is a narrow smoke check, not a tray, graceful-exit, DPI or accessibility acceptance.
