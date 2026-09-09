## ocp_dnsnameresolver carry instructions

Downstream keeps the external `ocp_dnsnameresolver` plugin. Upstream v1.14.4 does not ship it, so we must reapply the carry after the merge.

### Files to edit
- `plugin.cfg`: add `ocp_dnsnameresolver:github.com/openshift/coredns-ocp-dnsnameresolver` before `cache`.
- `core/plugin/zplugin.go` and `core/dnsserver/zdirectives.go`: regenerated outputs that pick up the plugin entry.
- `go.mod` / `go.sum`: add the released module version for `github.com/openshift/coredns-ocp-dnsnameresolver`.
- `vendor/modules.txt` and `vendor/github.com/openshift/coredns-ocp-dnsnameresolver/**`: repopulated via `go mod vendor`.

### Commands
1. Ensure the desired plugin version is set in `go.mod` (`go get github.com/openshift/coredns-ocp-dnsnameresolver@<tag>`).
2. Run `go generate coredns.go` to refresh `zplugin.go` / `zdirectives.go`. (Downstream `make check` does this as part of the pipeline.)
3. Run `GOFLAGS=-mod=vendor go mod vendor` to repopulate `vendor/`.
4. Stage `plugin.cfg`, the regenerated Go files, `go.mod`, `go.sum`, `vendor/modules.txt`, and the vendored plugin tree together.

### Ordering requirement
`ocp_dnsnameresolver` must remain immediately before `cache` in `plugin.cfg`; the generator preserves this ordering in the generated files.

### k8s dependency note (v1.14.4 rebase)
The `ocp_dnsnameresolver` plugin at `01fb3d1` requires k8s v0.36.2 (there is no k8s v0.35.x release of the plugin).
Go MVS pulls CoreDNS k8s deps from v0.35.4 to v0.36.2. This is acceptable — k8s v0.36.2 is backwards-compatible.

### Validation
- `GOFLAGS=-mod=vendor go test ./plugin/...` to ensure registrations compile.
- `GOFLAGS=-mod=vendor go test -count=1 ./...` for full test suite.
