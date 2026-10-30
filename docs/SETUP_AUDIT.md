# Setup audit

- 2026-10-30: the supplied offline installer was inspected and run against this project without `-Force`; its manifest checks verified all 17 payload files. Global Codex settings were not changed.
- All five project-local skills appeared in the updated Codex session skill catalogue: `awg-go-netstack`, `awg-windows-fyne`, `awg-security-tests`, `awg-github-upstream`, `awg-release-ci`. Read their full instructions only for the relevant task.
- `git` 2.45.1, `gh` 2.96.0, PowerShell 7.6.5 and Codex CLI 0.165.0 were available; authenticated GitHub owner was verified as `mkulkov` without recording credentials.
- No Go or C compiler was found on PATH or in the checked standard locations. Official Go 1.27.3 Windows amd64 archive and llvm-mingw 20261020 UCRT archive were downloaded to ignored project-local `.local/toolchains` and checked against their publishers' SHA-256 values. These tools were not added to the global PATH.
- Verified API properties: `mkulkov/amnezia-proxy` is public, fork=true, parent=`synacker/amnezia-proxy`; `mkulkov/awg-local-proxy` is public and fork=false. Local fork HEAD matched audited upstream commit and the working tree was clean at setup.

Toolchain archives are development dependencies, not product release artifacts. See `LICENSE_AUDIT.md` and `IMPLEMENTATION.md` for remaining license and validation gates.
