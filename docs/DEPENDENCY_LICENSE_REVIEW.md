# Preliminary dependency license review — 2026-10-30

`go list -deps` for the Windows builds found 26 external GUI modules and 8 core modules. `go mod verify` passed in both modules. Module and file inventory and copies of license texts were prepared under `%TEMP%/AwgDependencyAudit-20261030-c0c22661`; this is evidence for a later notices package, not a release artifact.

- Official AmneziaWG v3.1.20260828 and the linked Wintun Go bindings: MIT. The selected netstack path uses a userspace device. The dependency graph does not itself prove installation or operation of a Windows TUN driver; Wintun DLL/driver distribution was not audited.
- gVisor: Apache-2.0. GUI dependencies include MIT, BSD, ISC, Apache-2.0 and dual Unlicense/BSD conditions. A root license alone does not cover bundled assets.
- Fyne includes font assets with separate licenses: Noto Sans/Inter SIL OFL 1.1, EmojiOneColor MIT, and DejaVu/Bitstream/Arev notices. Include the complete corresponding texts and the Noto font metadata copyright `Copyright 2015 Google Inc. All Rights Reserved.` when distributing binaries.
- Native GLFW uses zlib terms; HarfBuzz has its own MIT notice. GLFW Windows vendored `dinput.h`/`xinput.h` contain LGPL-2.1-or-later headers. Inspection found no static or inline functions and at most two-line macros; review LGPL 2.1 section 5 for the exact build and preserve relevant notices and license text. This is not a legal opinion.
- The MinGW-w64 runtime requires its runtime notice when statically linked. Include the relevant Go and compiler runtime notices as well.

Before public distribution, generate and review complete THIRD_PARTY_NOTICES and license texts for the exact binary and toolchain, verify actual linked runtime and relevant source/notice obligations, perform vulnerability review, and review compliance for the exact release. On 2026-10-30 the owner selected MIT for the independent product and core; the original synacker proxy remains excluded. This inventory alone does not establish release readiness.
