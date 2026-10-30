---
name: awg-windows-fyne
description: "Use only for AWG Local Proxy Windows desktop UI, Fyne layouts, tray, DPI, process lifecycle and application launch profiles. Do not load for pure networking, license or CI-only changes."
---

# Windows UI and lifecycle

## Inputs

Read applicable AGENTS, CODEX_INSTRUCTIONS.md sections 4, 6–8 and the actual pinned Fyne version. Preserve the existing visual direction; do not import Godot/mobile/web design systems into this Windows utility.

## Workflow

1. Identify the user operation and its real backend state. Keep UI code behind a CoreClient interface; networking and IPC waits run off the UI loop.
2. Use Go + stable Fyne v2 unless an explicit recorded decision changes the stack. Verify Windows tray and build requirements with a small spike. Use the UI dispatch mechanism supported by the pinned Fyne version for background callbacks.
3. Build compact Russian-language views with visible labels, keyboard navigation and readable errors. Prioritize profile, state, connect button and copyable proxy addresses. Do not hide critical actions behind ambiguous icons.
4. Render stopped, connecting, connected, degraded and reconnecting accurately. A running child or open port is not Connected. MockCore always displays a demo warning.
5. Test close-to-tray, tray Show/Connect/Disconnect/Quit, single-instance behavior and complete child shutdown on Quit or parent crash. Avoid killing unrelated processes and avoid stale tray icons/listeners.
6. Keep autostart and auto-connect opt-in and separate. Any startup integration is per-user, reversible and does not request elevation or create a service.
7. The Applications page is a proxy configuration helper, not a process-routing allowlist. Launch profiles contain an absolute executable and an argument array, never an arbitrary shell command. Check browser single-instance/profile behavior; do not modify or terminate an existing user browser silently.
8. Do not set system proxy, global environment, routes, DNS or firewall. For applications without proxy support show a limitation, not an automatic interception promise.
9. Test DPI 100/125/150/200%, light/dark themes, resize, long Russian text, focus order, keyboard-only operation and the actual accessibility support of the chosen toolkit.
10. Capture UI evidence without secrets. Automated headless widget tests complement but do not replace manual Windows tray/DPI/DPAPI verification.

## Output

Provide the UI behavior implemented, visible limitations, screenshots/test evidence when available and manual Windows checks still outstanding. Never describe toolkit availability as completed Windows testing.
