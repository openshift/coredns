# CoreDNS Rebase Instructions

## Overview

OpenShift's CoreDNS fork incorporates upstream releases while maintaining downstream customizations. This document outlines the rebase process for integrating new upstream versions.

**Note**: These instructions are intended for manual rebases. A Claude Code `/rebase` command is also available that automates much of this workflow, including report generation documentation.

## Rebase Checklist

Before submitting a rebase PR, verify:

- [ ] Branch originates from upstream tag (e.g., `v1.x.y`)
- [ ] Merge helper commit created before carry patches: `git merge --no-ff --strategy=ours origin/main`
- [ ] Carry commits reapplied after merge helper with proper `UPSTREAM: <carry>: openshift:` prefix
- [ ] Vendor commit regenerated (should be towards the end of carry commits)
- [ ] Build succeeds: `go build`
- [ ] Tests pass: `make test`

## Commit Message Format

All downstream carry commits must use:

```
UPSTREAM: <carry>: openshift: <description>
```

For commits that will be dropped in the next rebase:

```
UPSTREAM: <drop>: openshift: <description>
```

For commits that cherry-pick an upstream PR:

```
UPSTREAM: <PR#>: <original commit message>
```

Example: `UPSTREAM: 6354: Fix memory leak in cache plugin`

This convention enables automated carry detection and is standard across OpenShift fork repositories. The prefix indicates how to handle the commit during the next rebase:
- `<carry>`: Must be reapplied/forward-ported
- `<drop>`: Skip this commit (temporary change)
- `<PR#>`: Check if PR is in new upstream version; skip if already included

## Carry Patch Documentation

Detailed reapply instructions for specific carry types are documented in `openshift-rebase/carries/`. These documentation files are co-located in the same commits as the carries they describe, making it easy to reference the instructions when cherry-picking.

## Rebase Workflow

### 1. Preparation

```bash
# Fetch upstream
git remote add upstream https://github.com/coredns/coredns.git  # if needed
git fetch upstream --tags

# Identify latest carries from the most recent rebase
# Find the merge helper commit (looks for "rebase-to-v" pattern in the most recent rebase PR)
# Example: Merge remote-tracking branch 'origin/main' into rebase-to-v1.13.2
# NOTE: This assumes the merge helper is BEFORE carries (as documented below)
MERGE_HELPER=$(git log --oneline --merges --grep="rebase-to-v" -1 --format="%H")
git log --oneline --no-merges ${MERGE_HELPER}..origin/main
```

### 2. Cherry-Pick Carries

```bash
# Set target version (e.g., v1.13.2)
VERSION=v1.x.y

# Create branch from upstream tag
# NOTE: Do not change the branch name format 'rebase-to-v...' as it is used in the
# preparation step above to identify carry patches via the merge helper commit message
git checkout -b rebase-to-${VERSION} ${VERSION}

# IMPORTANT: Create merge helper commit FIRST (this should be the first commit after the tag)
git merge --no-ff --strategy=ours origin/main -m "Merge remote-tracking branch 'origin/main' into rebase-to-${VERSION}"

# Then reapply carry commits ON TOP of the merge helper
# This automatically cherry-picks all commits from the previous rebase
git cherry-pick ${MERGE_HELPER}..origin/main
```

**Note**: Pay attention to the `UPSTREAM:` prefix in each commit message (see Commit Message Format section above) to determine whether to cherry-pick, skip, or check upstream for PR inclusion.

#### Handling Conflicts

Cherry-picking will likely encounter conflicts:

- **File deletion conflicts** (modify/delete) - Deleted files modified upstream
- **Dependency conflicts** - go.mod/go.sum conflicts (resolve then run `go mod tidy && go mod vendor`)
- **Empty commits** - Change already in upstream (skip with `git cherry-pick --skip`)
- **Vendor conflicts** - Renamed/deleted vendor files (re-run `go mod vendor` to clean up)

### 3. Cleanup Commit History

After cherry-picking, squash related commits to simplify the carry history:

Common squash candidates:
- Multiple ART consistency updates → single product build config commit
- Plugin version bumps and configuration changes → initial plugin addition commit
- **Multiple vendor commits → single vendor commit** (REQUIRED - see note below)

Use `git rebase -i ${MERGE_HELPER}` to squash commits. After squashing vendor-related changes, regenerate the vendor tree with `go mod tidy && go mod vendor` and amend the commit.

Review final history with `git log --oneline ${MERGE_HELPER}..HEAD` and verify each commit has proper `UPSTREAM:` prefix.

### 4. Rebuild Generated Files

Regenerate the vendor commit (should be towards the end of carry commits). See `openshift-rebase/carries/vendor_workflow.md` for details.

### 5. Validation

```bash
# Build and test
go build
make test

# Test with vendored modules
GOFLAGS=-mod=vendor go test ./...
```

## Post-Rebase

1. Create PR against `main` branch
2. Document conflict resolutions and skipped commits in PR description
3. Coordinate stakeholder review for high-risk changes
4. After merge, notify relevant teams of version update
