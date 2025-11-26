## Downstream vendoring strategy

We continue to vendor dependencies so downstream builds do not rely on network access.

### Regenerating vendor after the rebase
1. Ensure `go.mod` and `go.sum` reflect the desired dependency set (run `go mod tidy` if needed once conflicts are resolved).
2. Run `GOFLAGS=-mod=vendor go mod vendor` (or export `GOFLAGS=-mod=vendor` globally) to repopulate `vendor/`.
3. Re-apply the downstream ginkgo shim (`vendor/github.com/onsi/ginkgo/v2/ginkgo/build/build_command.go`) if it is dropped by `go mod vendor`.
4. Stage the entire `vendor/` tree along with `go.mod` / `go.sum` updates in a single carry commit.

### Build tooling expectations
- Dockerfiles (`Dockerfile.openshift`, `Dockerfile.openshift.rhel7`, etc.) must keep `GO111MODULE=on` and `GOFLAGS=-mod=vendor` so container builds consume the vendored tree.
- Any CI/Make targets that compile CoreDNS should set `GOFLAGS=-mod=vendor` (unless the buildroot already enforces it).

### .gitignore adjustments
- Keep vendor tracked by removing the upstream `vendor/` ignore entry; see `carry_consolidation/gitignore.patch`.
- Ignore build outputs only: `query.log`, `Corefile`, `*.swp`, `/coredns`, `coredns.exe`, `/build/`, `release/`.
