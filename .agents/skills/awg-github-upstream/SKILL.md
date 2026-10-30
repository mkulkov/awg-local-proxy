---
name: awg-github-upstream
description: "Use for AWG Local Proxy licensing review, creation or verification of its public GitHub fork and independent product repository, remotes, upstream synchronization and dependency provenance."
---

# License gate and two-repository maintenance

## Mandatory order

Read CODEX_INSTRUCTIONS.md sections 2–3 and 9. Recheck the exact source commit and license BEFORE repository creation. Public visibility, GitHub license metadata and dependency licensing are not sufficient permission to modify or redistribute a project.

## Workflow

1. Verify authenticated identity, requested owner, existing repositories, branch state and local files. Never print tokens. Do not change the owner or delete an existing repository to resolve a conflict.
2. Inspect tree, LICENSE/COPYING/NOTICE, README and file-level headers at the exact commit. Record evidence in docs/LICENSE_AUDIT.md. A missing license blocks derivative integration/distribution, not all independent GUI development.
3. A native unchanged GitHub fork may be created under GitHub's stated fork rights. Do not add your own license to upstream code. Verify fork=true, parent=synacker/amnezia-proxy and public visibility after the operation.
4. The product repository mkulkov/awg-local-proxy must be independently created with fork=false, public visibility and only original product files. Keep mock mode explicit while the component is blocked.
5. Use safe gh/API operations and check exit codes. Do not replace a native fork with clone-and-push. Do not blindly clone over an existing nonempty project directory.
6. Use origin for the user's fork and upstream for synacker/amnezia-proxy. Keep local clones separate. No destructive reset or force push; preserve the upstream history.
7. After permission is established, maintain a small component patch branch. Fetch upstream, inspect licensing/API changes, run core/contract tests and propose a reviewable update. Do not auto-merge upstream into releases.
8. Pin full commit hashes and license evidence in core.lock.json. Separate product licensing from dependency obligations; two processes do not magically remove those obligations.
9. Stage explicit files, inspect diff, scan for secrets before push. Keep real VPN profiles, test captures and personal correspondence out of public repositories.
10. Draft any upstream license request, but do not send it or open an upstream PR without separate authorization for that communication.

## Report

Return actual repository URLs, owner/visibility/fork/parent evidence, local remotes, exact source commit, license status and blocked actions. Never present intended URLs as created resources.
