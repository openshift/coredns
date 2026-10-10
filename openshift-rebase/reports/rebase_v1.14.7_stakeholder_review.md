# CoreDNS v1.14.7 Rebase — Stakeholder Review

## Upstream Changes Analysis

**Release notes**: https://github.com/coredns/coredns/releases/tag/v1.14.7
**Commits**: 631 between v1.13.1 and v1.14.7 (216 of those between v1.14.4 and v1.14.7)

### New Features and Enhancements

| Feature | Risk | Notes |
|---------|------|-------|
| Forward plugin hostname resolution (#7923) | **Medium** | Carried over from v1.14.4 analysis — TO endpoints can use hostnames instead of IPs. Needs e2e validation in OpenShift. |
| DoH support for forward (#8004) | **Medium** | New transport for outbound forwarding. Opt-in, but new code path worth exercising. |
| ACME DNS-01 certificate management in `plugin/tls` (#8310) | **Medium** | New certificate lifecycle feature. Not used by default OpenShift config, but changes the TLS plugin's surface area. Related: upstream also relocated `tls` in `plugin.cfg` — it now runs just after `dnssec` (previously ran near `proxyproto`/`quic`, before most of the chain), specifically so `dnssec` can sign ACME challenge records before authoritative backends run. This changes where in the request pipeline the `tls` plugin executes. |
| `source_address` directive for forward (#8011) | **Low** | Lets forward bind a specific source address. Opt-in. |
| Secondary catalog member zones (#8230, #8288) | **Low** | Extends `secondary`/catalog support. Opt-in. |
| `prefer_positive` / configurable stale-TTL cache policies (#8378, #9411/#8411) | **Medium** | Changes cache staleness behavior when configured. Same family of change flagged as medium risk in the v1.14.4 review. |
| New `shed` plugin — UDP overload protection (#8312) | **Low** | New opt-in plugin, not in the default OpenShift plugin chain. |
| DoQ/DoH3 connection-level concurrency limits (#8213, #e0a8eb8a4) | **Low** | Hardening for QUIC/HTTP3 listeners. |
| `topology-aware headless services` for kubernetes plugin (#8388) | **Medium** | New Kubernetes-plugin feature touching service topology; worth a compatibility pass given OpenShift's own topology handling. |

### Bug Fixes

| Fix | Risk | Notes |
|-----|------|-------|
| Cache: preserve AD bit / monotonic time / SOA-less NODATA handling (#8438, #c0adbae99, #e073d1c05) | **Medium** | Several correctness fixes to caching semantics since v1.14.4 — same plugin flagged as medium risk previously. |
| File: multi-primary AXFR zone contamination, DNAME loops, wildcard/empty-non-terminal fixes (#3a52659cb, #ea2eea57a, #d74404f8a) | **Medium** | Multiple correctness fixes in zone transfer/file handling. |
| Kubernetes: AXFR panic on multiple nsAddrs, Pod.DeepCopyObject label copy (#7df923825, #bc74540f5) | **Medium** | Crash/correctness fixes in the kubernetes plugin, which OpenShift depends on directly. |
| Forward: incorrect failover counter reset, UDP forwarding block on malformed datagram, cap default connect attempts (#4ebf66a74, #989a188d1, #0fa6c6679) | **Medium** | Reliability fixes in the forward plugin's retry/failover logic. |
| ACL: autopath bypass, cached answers reaching blocked clients (#18a58b9e8, #1ba14119d) | **High** | Security-relevant ACL bypass fixes — worth explicit sign-off before merge. |
| proxyproto: nil deref on malformed packets (#60a439dd4) | **Low** | Already covered in the v1.14.4 review; still present as of v1.14.7. |

### Breaking Changes and Deprecations

| Change | Risk | Notes |
|--------|------|-------|
| Continued stricter config validation | **Medium** | Same trend flagged in the v1.14.4 review (`any`, `local`, `chaos`, `health`, `log`, `ready`, `trace`, `dnstap` reject unknown block options). No new plugins added to the list since v1.14.4. |
| TLS defaults (Go crypto/tls defaults) | **Low** | Was a downstream carry (#8227) as of v1.14.4; now upstream natively in v1.14.7 (`bc4343b08`) — the carry is dropped in this rebase. |

### Dependency Updates

| Dependency | v1.14.4 | v1.14.7 | Final (after carries) | Risk |
|------------|---------|---------|------------------------|------|
| k8s.io/api, apimachinery, client-go | v0.35.4 | v0.35.4 | v0.36.2 (MVS from ocp_dnsnameresolver, unchanged reasoning from v1.14.4) | **High** |
| google.golang.org/grpc | v1.81.1 | v1.83.0 | v1.83.0 | **Low** |
| golang.org/x/net | v0.55.0 | v0.57.0 | v0.57.0 | **Low** |
| Go minimum version (go.mod) | 1.26.3 (downstream carry) | 1.25.0 (upstream go.mod; `.go-version` is 1.26.6) | 1.26.6 (alignment carry) | **Medium** |

## Downstream Changes

### Carry Consolidation Summary

- **8 carries reapplied**: OWNERS, disable dependabot, `.ci-operator.yaml` + `Dockerfile.ocp` build/image config, track vendor, make test + GOTOOLCHAIN=local, Go 1.26 alignment, ocp_dnsnameresolver, vendor tree
- **2 previously-missing carries recovered**: `.ci-operator.yaml` and `Dockerfile.ocp` — both ART-managed, OpenShift-only files whose founding commits (2021) predate the `v1.13.1` carry-audit window, so both were silently dropped by the `-s ours` merge helper on every rebase, including the original v1.14.4 attempt (#197). Missing `.ci-operator.yaml` broke `ci/prow/unit`/`ci/prow/verify-deps`; missing `Dockerfile.ocp` broke `ci/prow/images`. Both failures were present on #197 since 2026-08-06.
- **2 more latent CI-breaking bugs found and fixed after pushing** (same root cause pattern — pre-existing gaps, not regressions from this rebase): `.dockerignore` was never deleted (OpenShift's `upstream/main` deletes it; leaving it in place makes `Dockerfile.ocp`'s from-source build copy nothing, breaking `ci/prow/images` even after the fix above), and `.gitignore`'s `coredns` entry was unanchored, silently excluding `vendor/github.com/coredns/caddy` (a required dependency) from every "update vendor" carry ever produced in this fork — the actual cause of `ci/prow/unit` and `ci/prow/verify-deps` failing.
- **2 PR #197 review comments (@davidesalerno) addressed**: README.md's stated Go minimum now matches go.mod (1.26.6); `.circleci/config.yml`'s `K8S_VERSION` now matches the bumped client-go version (v1.36.2).
- **1 carry dropped versus the v1.14.4 attempt**: TLS defaults (#8227) — now upstream natively as of v1.14.7
- **3 CVE bumps dropped** (superseded by v1.14.7, same as the v1.14.4 review): gRPC, x/net, expr
- **2 ART image-sync commits skipped**: only their version-tag bumps are skipped (bot will resubmit after merge) — the `.ci-operator.yaml`/`Dockerfile.ocp` files they touch are reapplied as carries, not dropped
- **2 OWNERS commits squashed** into 1

### External Plugin Status

`ocp_dnsnameresolver` stays pinned at `01fb3d1` (2026-07-21), unchanged from the v1.14.4 rebase — it still requires k8s v0.36.2 (no v0.35.x release exists). Same accepted tradeoff as before; no new compatibility work needed since v1.14.4 already absorbed this bump.

### Toolchain Changes

- `go.mod`'s `go` directive moves to 1.26.6 to match v1.14.7's own `.go-version` (previously 1.26.3 for v1.14.4).
- `GOTOOLCHAIN=local` carry unchanged — still prevents auto-download in disconnected builds.
- **Action needed**: confirm the OpenShift builder image ships Go ≥1.26.6 before merge. This is a higher bar than the 1.26.3 the v1.14.4 rebase required — worth explicit verification rather than assuming the image already moved.

## Action Items

| Item | Owner | Status |
|------|-------|--------|
| Verify OpenShift CI/builder images support Go 1.26.6 (higher bar than the 1.26.3 used for v1.14.4) | Team | ⬜️ |
| Review ACL bypass fixes (#18a58b9e8, #1ba14119d) for OpenShift relevance | Team | ⬜️ |
| Validate kubernetes plugin topology-aware headless service change against OpenShift's own topology handling | Team | ⬜️ |
| Re-confirm k8s v0.36.2 compatibility (carried over from v1.14.4 sign-off, unchanged) | Team | ⬜️ |
| Run OpenShift e2e DNS test suite | Team | ⬜️ |

## Meeting Notes

_(Space for notes during stakeholder review meeting)_
