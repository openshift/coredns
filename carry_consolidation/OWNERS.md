# OWNERS carry instructions

Our downstream fork keeps its own `OWNERS` file so the OpenShift DNS/APISnoop
teams can self-serve reviews and hook into the component automation. Upstream
CoreDNS does not ship this metadata, so every rebase needs to restore it.

## Files to stage together
- `OWNERS` – the downstream file shown below (unchanged from
  `a82419240`/`c6cbe9feb`).
- `.ci-operator.yaml`, `Dockerfile.ocp`, `Dockerfile.openshift` – carried in the
  automation metadata commit, but listed here so reviewers know why the same set
  of people remain approvers.

## Reapply steps
1. Copy the snippet into the repo root as `OWNERS` (overwriting any upstream
   placeholder):

```yaml
approvers:
  - knobunc
  - Miciah
  - frobware
  - candita
  - rfredette
  - alebedev87
  - gcs278
  - rikatz

component: DNS

features:
  - comments
  - reviewers
  - aliases
  - branches
  - exec

aliases:
  - |
    /plugin: (.*) -> /label add: plugin/$1
  - |
    /approve -> /lgtm
  - |
    /wai -> /label add: works as intended
  - |
    /release: (.*) -> /exec: /opt/bin/release-coredns $1
```

2. Run `git add OWNERS` (or `git checkout --theirs OWNERS` when resolving
   conflicts).
3. Commit as part of `UPSTREAM: <carry>: openshift: restore automation metadata`
   so the OWNERS/automation files travel together and reviewers have a single
   place to look.

Historical context: this file originated in commits like `a82419240` and was
last reviewed by @Miciah and @alebedev87. Keeping the instructions in Markdown
instead of raw patch form makes it easier to see what the carry reintroduces
without having to consult the archived rebase report.
