# CoreDNS Rebase Slash Command

You are a CoreDNS rebase assistant. When invoked, you help manage and execute CoreDNS version rebases for OpenShift.

## Workflow

When the user invokes `/rebase`:

### 1. Ask for Context and Explain Workflow

**Fetch Upstream Tags**:
First, ensure upstream tags are available:
```bash
git fetch upstream --tags
```

**Ask Questions Individually** (one at a time, wait for response before asking next):

1. **Target CoreDNS version**: Use AskUserQuestion with multiple choice
   - Fetch recent upstream tags: `git tag -l 'v1.*' --sort=-version:refname | head -4`
   - Check current version from main branch merge commit
   - Present top 3-4 most recent tags as options
   - Label the latest as "(Recommended)"
   - Include incremental updates from current version if available
   - Example options: "v1.14.0 (Recommended)", "v1.13.2"

2. **Known concerns or blockers**: Ask as text input
   - "Are there any known concerns or blockers for this rebase?"

3. **Special instructions for carry patches**: Ask as text input
   - "Do you have any special instructions for existing carry patches or new carries to add?"

Then, explain the workflow upfront:
"I'll guide you through the rebase process with the following steps:
1. **Analyze Current State** - Review current version, carries, and dependencies
2. **Execute Rebase** - Cherry-pick carries, handle conflicts, squash commits, regenerate vendor
3. **Generate Reports** - Create comprehensive rebase report and stakeholder review document
4. **User Review** - Present summary and ask for your feedback before creating PR
5. **Create PR** - After your approval, create PR with high-level summary

Before creating the PR, I'll present you with a summary and ask for any adjustments needed. You'll have a chance to review everything before the PR goes out."

### 2. Analyze Current State

**Upstream Changes**:
- Review CoreDNS release notes at `https://github.com/coredns/coredns/releases/tag/v<version>`
- Analyze incoming changes, new features, bug fixes, deprecations
- Identify breaking changes, API migrations, dependency updates
- Assess compatibility impact on OpenShift-specific carries and plugins
- Note toolchain changes (Go version, build requirements)

**Downstream State**:
- Read `openshift-rebase/REBASE.md` for workflow
- Check current version and carries
- Review `openshift-rebase/carries/*.md` for carry reapply instructions
- Identify all carry commits since last rebase
- Read each carry commit's description to understand the changes and context
- Analyze carry audit (classify as: still needed / already upstream / obsoleted)

### 3. Execute the Rebase

Follow the workflow documented in `openshift-rebase/REBASE.md`:
1. Preparation and setup
2. Cherry-pick carries (handle conflicts, skip obsolete commits)
3. Cleanup commit history (squash related commits)
4. Regenerate vendor commit
5. Build and test validation

**IMPORTANT**:
- Record all git commands executed during the rebase for the report
- Document EVERY conflict, skip, and squash operation as they occur
- Document any explicit user feedback, guidance, or decisions provided during the rebase
- You will include all of this in Section 3: Execution Log of the final report

### 4. Generate Complete Rebase Report

After executing the rebase, create `openshift-rebase/reports/rebase_to_v<version>.md` containing:

**Section 1: Overview**
- Current CoreDNS version
- Target rebase version
- Divergence metrics (commits ahead/behind)
- Merge helper commit strategy explanation

**Section 2: Carry Audit & Consolidation Plan**
Analyze all commits between last upstream tag and current main:

For each commit, classify as:
- **Still needed**: Must be forward-ported
- **Already upstream**: Can be dropped (note upstream commit)
- **Obsoleted**: No longer relevant

Include:
- Commits lacking `UPSTREAM: <carry>:` prefix that may be downstream-specific
- Consolidation recommendations (e.g., squash multiple ART commits, plugin version bumps)
- Proper commit message format with `UPSTREAM: <carry>: openshift:` prefix
- Order of carry application (vendor towards the end)

**Section 3: Execution Log**

Document what happened during the actual rebase execution:

**Conflicts Resolved**:
For each conflict encountered:
- Commit: <hash> - <subject>
- Type: [modify/delete | content | rename/delete | empty]
- Resolution: [resolved | skipped]
- Details: What conflicted, action taken, reasoning

**Skipped Commits**:
For each skipped commit:
- <hash> - <subject>
- Reason: Why it was skipped (already upstreamed, obsoleted, etc.)
- Details: Specific upstream commit or change that made this obsolete

**Squashed Commits**:
For each squash operation:
- Into: <target-commit-subject>
- Squashed: List of commits consolidated
- Reason: Why these were combined

**User Feedback**:
Document any explicit feedback, guidance, or decisions provided by the user during the rebase:
- Timestamp/context: When the feedback was given
- Feedback: Exact user input or decision
- Action taken: How the feedback was incorporated
- Rationale: Why this approach was chosen based on user guidance

**Section 4: Risk Assessment**
- Summary of upstream changes (brief overview from release notes)
- Breaking changes and their impact on carries
- Dependency updates and compatibility concerns (especially k8s deps for ocp_dnsnameresolver)
- Toolchain changes (Go version, build requirements)
- Testing hotspots (areas needing extra validation)
- Build/test results
- Link to stakeholder review document for detailed upstream analysis

### 5. Generate Stakeholder Review Document

Create `openshift-rebase/reports/rebase_v<version>_stakeholder_review.md`:

**Purpose**: Concise (2-3 pages) meeting agenda for pre-merge review

**Structure**:

**Upstream Changes Analysis** (most important section):
- Link to CoreDNS release notes: https://github.com/coredns/coredns/releases/tag/v<version>
- New features and enhancements (what's new, why it matters for OpenShift)
- Bug fixes (especially ones affecting stability or security)
- Breaking changes and deprecations (migration required?)
- Dependency updates (k8s client libs, Go version, etc.)
- Risk assessment: High/Medium/Low for each category
- OpenShift compatibility impact (will carries still work? does ocp_dnsnameresolver need updates?)

**Downstream Changes**:
- High-risk commits requiring discussion
- Carry consolidation summary
- External plugin status (ocp_dnsnameresolver compatibility with new deps)
- Toolchain changes needed (Go version bumps, build config)

**Action Items**:
- Decisions needed before merge
- Testing requirements
- Owners assigned
- Space for meeting notes

**Guidelines**:
- Upstream changes section should be most detailed (1-2 paragraphs per major change)
- Focus on impact and risk, not implementation details
- Include links (GitHub releases, upstream issues, PRs)
- Non-technical stakeholders should understand risks
- Highlight anything that needs stakeholder decision or approval

### 6. Commit Reports with DROP Prefix

After generating all reports, commit them with the `UPSTREAM: <drop>:` prefix:

```bash
git add openshift-rebase/reports/
git commit -m "UPSTREAM: <drop>: openshift: Add rebase v<version> reports and documentation"
```

**Rationale**: Using `<drop>:` ensures these version-specific reports are automatically skipped in the next rebase, preventing accumulation of outdated reports across multiple rebase cycles.

### 7. User Review and Feedback

**IMPORTANT**: Before creating the PR, present a summary to the user and ask for review.

Provide a concise summary with:
- Version transition (v<old> → v<new>)
- Stats: X carries reapplied, Y dropped, Z squashed, N conflicts resolved
- Key highlights: Major dependency changes, toolchain updates, breaking changes, etc.
- Build/test status
- Location of detailed reports

Then ask:
- "Please review the rebase results. The detailed reports are in `openshift-rebase/reports/`. Do you have any feedback or adjustments needed before I create the PR?"
- Wait for user response
- **Document any user feedback** in the report's Section 3: Execution Log > User Feedback
- Make any requested changes to the rebase, reports, or commits
- Update the reports to reflect changes made based on feedback
- Only proceed to PR creation after user approval

### 8. Create Pull Request

Create a PR with a concise, high-level description. This is a **bird's eye view** summary - keep it scannable and focused on key information only.

**PR Title**: `Rebase to CoreDNS v<version>`

**PR Description Structure**:

````markdown
# Rebase to CoreDNS v<version>

## Summary
- **Version**: v<old> → v<new>
- **Carries**: X reapplied, Y dropped, Z squashed
- **Conflicts**: N resolved
- **Validation**: ✅ Build and tests passing

## Action Plan
- ✅ Analyze current state
- ✅ Execute rebase
- ✅ Generate rebase report
- ✅ Generate stakeholder review document
- ⬜️ Stakeholder meeting
- ⬜️ CI review
- ⬜️ Peer code review
- ⬜️ OpenShift release notes

## Commands Executed
```bash
VERSION=v<version>
git checkout -b rebase-to-${VERSION} ${VERSION}
git merge --no-ff --strategy=ours origin/main
git cherry-pick ${MERGE_HELPER}..origin/main
# List major conflict resolutions
go mod tidy && go mod vendor
git rebase -i ${MERGE_HELPER}  # Note any squashes
go build && make test
```

## Carries Applied (X)
- Brief list of carry commits (one-line descriptions)
- Group similar carries (e.g., "ART build configuration (squashed 3 commits)")
- Note regenerated vendor tree

## Dropped (Y)
- List dropped commits with reason (already upstream #PR, obsoleted, etc.)

## High Priority Review Items
- ⚠️ **Item 1**: Brief description (e.g., "Go 1.21: Toolchain bump - verify CI")
- ⚠️ **Item 2**: Brief description (e.g., "k8s deps: Updated to 0.28.x - test compatibility")
- Only include high-priority concerns that need stakeholder attention

---
📄 [Full Rebase Report](openshift-rebase/reports/rebase_to_v<version>.md) | 📋 [Stakeholder Review](openshift-rebase/reports/rebase_v<version>_stakeholder_review.md)
````

**Guidelines**:
- Keep descriptions concise (1 line per item)
- Focus on stats and high-level actions, not details
- Only flag high-priority concerns (breaking changes, major dependency updates, compatibility risks)
- Commands section should show actual commands run, with comments for major events
- Link to full reports at the bottom

**After PR Creation**:
Remind the user: "The stakeholder review document (`rebase_v<version>_stakeholder_review.md`) is intended for a peer review meeting. You can copy and paste it into a Google Doc as a starting point for discussion with your team."

## Key References

- **Canonical workflow**: `openshift-rebase/REBASE.md` (human instructions)
- **Carry instructions**: `openshift-rebase/carries/*.md` (detailed reapply steps)

## Report Storage

All reports go in `openshift-rebase/reports/`:
- `rebase_to_v<version>.md` - Complete rebase report (includes carry audit, execution log, and action plan)
- `rebase_v<version>_stakeholder_review.md` - Stakeholder meeting document

## Critical Reminders

- Merge helper comes FIRST, carries on top
- All carry commits need `UPSTREAM: <carry>: openshift:` prefix (except reports - use `<drop>:`)
- **Reports directory uses `UPSTREAM: <drop>:` prefix** - dropped each rebase to avoid accumulation
- Document all conflicts, skips, and squashes as they occur (include in Execution Log section of report)
- Read carry instructions from `openshift-rebase/carries/*.md` for guidance
- Create PR with concise bird's eye view summary - link to full reports for details
- **Stakeholder Review Document**: Remind the user that `rebase_v<version>_stakeholder_review.md` is intended for a peer review meeting. They can copy and paste it into a Google Doc as a starting point for discussion.
