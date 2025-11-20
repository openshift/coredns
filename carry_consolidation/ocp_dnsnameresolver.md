## ocp_dnsnameresolver carry instructions

Downstream keeps the external `ocp_dnsnameresolver` plugin. Upstream v1.13.1 does not ship it, so we must reapply the carry after the merge.

### Files to edit
- `plugin.cfg`: add `ocp_dnsnameresolver:github.com/openshift/coredns-ocp-dnsnameresolver` before `cache`.
- `core/plugin/zplugin.go` and `core/dnsserver/zdirectives.go`: regenerated outputs that pick up the plugin entry.
- `go.mod` / `go.sum`: add the released module version for `github.com/openshift/coredns-ocp-dnsnameresolver`.
- `vendor/modules.txt` and `vendor/github.com/openshift/coredns-ocp-dnsnameresolver/**`: repopulated via `go mod vendor`.
- Drop the temporary `replace` directive once the plugin release is tagged (track in action plan).

### Commands
1. Ensure the desired plugin version is set in `go.mod` (`go get github.com/openshift/coredns-ocp-dnsnameresolver@<tag>`).
2. Run `go generate coredns.go` to refresh `zplugin.go` / `zdirectives.go`. (Downstream `make check` does this as part of the pipeline.)
3. Run `GOFLAGS=-mod=vendor go mod vendor` to repopulate `vendor/`.
4. Stage `plugin.cfg`, the regenerated Go files, `go.mod`, `go.sum`, `vendor/modules.txt`, and the vendored plugin tree together.

### Ordering requirement
`ocp_dnsnameresolver` must remain immediately before `cache` in `plugin.cfg`; the generator preserves this ordering in the generated files.

### Validation
- `GOFLAGS=-mod=vendor go test ./plugin/...` to ensure registrations compile.
- Execute the targeted plugin smoke tests from the prototype branch once the module tag is published.
