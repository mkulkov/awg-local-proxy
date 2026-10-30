---
name: awg-release-ci
description: "Use for AWG Local Proxy Go and Windows build pipelines, pinned core artifacts, reproducibility, GitHub Actions hardening, dependency checks and release packaging. Not for ordinary GUI or network feature implementation."
---

# Build, CI and release integrity

## Inputs and gates

Read CODEX_INSTRUCTIONS.md sections 9–11, docs/LICENSE_AUDIT.md and the actual source tree. A release gate is passed by evidence, not a TODO checkbox. Before permission, build the original GUI/mock only; do not fetch or bundle unlicensed core artifacts.

## Workflow

1. Pin supported Go/Fyne/module versions and the exact permitted core commit. Use a clean checkout with no absolute local go.mod replace. Record toolchain and module graph.
2. Build GUI and core separately. Windows GUI may require CGO and a C toolchain at build time; verify runtime DLL dependencies on a clean Windows account. Do not assume a Linux cross-compile proves a runnable Windows build.
3. Run gofmt checks, go vet, unit/contract tests, targeted integration, fuzz smoke and race tests on supported targets. Report unsupported/runners-not-available rather than silently skipping with success.
4. Use Windows runners for applicable GUI/DPAPI/lifecycle tests. Headless tests do not prove tray/DPI/accessibility; include a manual checklist.
5. Pin Actions by full SHA, default to contents:read, grant release write only where needed. Never combine untrusted PR execution and privileged secrets. Do not use the user's production VPN profile in CI.
6. Scan dependencies for vulnerabilities and licenses, generate SBOM/notices and scan secrets. Keep credentials, user logs, configs, captures and dumps out of artifacts.
7. Verify the core lock, IPC compatibility and source/binary correspondence. SHA256SUMS checks integrity but is not source authenticity on its own; distinguish checksum, trusted provenance and Authenticode signing.
8. Create a portable ZIP with GUI, permitted core, docs and required notices. Do not add install-time download of an unreviewed latest binary. Automatic updater is outside MVP.
9. Test the release archive from a clean directory/user profile without Go installed. Verify no UAC, adapters, route/DNS/proxy changes and reliable shutdown.
10. Publish only after license/security/compatibility gates pass. If unsigned, state it; never disable Windows protections to eliminate SmartScreen warnings. Document rollback and the exact component versions.

## Output

Provide artifact paths, checksums, tool versions, workflow results, license gate, test evidence and remaining manual checks. Never label a mock-only preview a functioning VPN release.
