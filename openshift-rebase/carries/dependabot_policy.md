## Dependabot policy for downstream fork

- Upstream CoreDNS keeps `.github/dependabot.yml`. Downstream OpenShift policy disables Dependabot entirely.
- During rebase, delete `.github/dependabot.yml` in the carry commit and avoid reintroducing it.
- No replacement configuration is required; document the disablement in commit message for clarity.
