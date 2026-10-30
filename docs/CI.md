# Windows CI

The product workflow runs on Windows Server 2025 using Go 1.27.3. It checks formatting and modules, tests and vets the product, runs the race detector and fuzz smoke, and builds the unbound demo. It separately checks out the independent core at the exact commit in `core.lock.json`, then runs a synthetic end-to-end test through the actual product session controller and supervisor, separate core process and temporary userspace AWG peer. No user configuration, user server or credentials are involved. The optional test helper path is not secret; generated keys travel over inherited stdin only.

Actions are pinned to full commit SHA, checkout credentials are not persisted, `contents: read` is the only permission, and no release or artifact publication is performed. The blocked `synacker/amnezia-proxy` source is not downloaded. The separate core has its own Windows workflow.

A successful run covers the checks in that run; it does not prove compatibility with a user's server, full Windows DPI/tray/accessibility acceptance, cross-user DPAPI behavior, or long-term resource stability. Current evidence and remaining limitations are in [TEST_RESULTS.md](TEST_RESULTS.md).
