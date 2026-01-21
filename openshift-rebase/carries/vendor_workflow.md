## Downstream vendoring strategy

We continue to vendor dependencies so downstream builds do not rely on network access.

### Carry Commit Strategy

**IMPORTANT**: The vendor carry commit is cherry-picked like other carries, but must be **regenerated** after cherry-picking because the vendored dependencies change with each upstream version. Run `go mod tidy && go mod vendor` and amend the commit with the updated vendor tree.

**CRITICAL - Only One Vendor Commit**:
There should be **only one** vendor carry commit in the final rebase, positioned **towards the end** of carry commits. If multiple vendor commits exist after cherry-picking (e.g., from intermediate carries or conflicts), squash them into a single vendor commit using `git rebase -i`, then regenerate the vendor tree to capture the complete dependency state.

### Moving Vendor Commit to the End

After cherry-picking all carries and adding any new carries, the vendor commit needs to be at the end before regenerating:

```bash
# Find the merge helper commit
MERGE_HELPER=$(git log --oneline --merges --grep="rebase-to-v" -1 --format="%H")

# Move the vendor commit to be the last commit
git rebase -i ${MERGE_HELPER}
# In the editor, move the "vendor deps" commit line to the bottom of the list
# Save and exit

# Now regenerate and amend the vendor commit
go mod tidy && go mod vendor
git add vendor/ go.mod go.sum
git commit --amend --no-edit
```

**Key principle**: The vendor commit must be at HEAD when you regenerate it, so you can use `git commit --amend`.

### Regenerating vendor after the rebase
1. Ensure `go.mod` and `go.sum` reflect the desired dependency set (run `go mod tidy` if needed once conflicts are resolved).
2. Run `GOFLAGS=-mod=vendor go mod vendor` (or export `GOFLAGS=-mod=vendor` globally) to repopulate `vendor/`.
3. Stage the entire `vendor/` tree along with `go.mod` / `go.sum` updates in a single carry commit with `UPSTREAM: <carry>: openshift:` prefix.

### Build tooling expectations
- Dockerfiles (`Dockerfile.openshift`, `Dockerfile.openshift.rhel7`, etc.) must keep `GO111MODULE=on` and `GOFLAGS=-mod=vendor` so container builds consume the vendored tree.
- Any CI/Make targets that compile CoreDNS should set `GOFLAGS=-mod=vendor` (unless the buildroot already enforces it).

### .gitignore adjustments
- Keep vendor tracked by removing the upstream `vendor/` ignore entry.
- Ignore build outputs only: `query.log`, `Corefile`, `*.swp`, `/coredns`, `coredns.exe`, `/build/`, `release/`.
