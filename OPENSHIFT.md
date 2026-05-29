# OpenShift CoreDNS Fork

This is the OpenShift fork of [CoreDNS](https://coredns.io). It is deployed
and managed by the
[cluster-dns-operator](https://github.com/openshift/cluster-dns-operator).

## Plugin management

The operator generates the CoreDNS Corefile and only uses a subset of upstream
plugins. Unused plugins are removed to reduce the binary size, dependency
footprint, and attack surface.

The file `openshift-plugins.cfg` is the source of truth for the kept plugin
set. It uses the same `name:package` format as the upstream `plugin.cfg`.

### Regenerating after a rebase

After rebasing onto a new upstream version:

```
make -f Makefile.ocp generate-plugins
```

This will:

1. Overwrite `plugin.cfg` with `openshift-plugins.cfg`
2. Remove plugin directories under `plugin/` that are not needed
3. Regenerate the Go files (`core/plugin/zplugin.go`, `core/dnsserver/zdirectives.go`)
4. Run `go mod tidy` and `go mod vendor`

Review the result with `git diff --stat`, then commit.

### Adding or removing a plugin

Edit `openshift-plugins.cfg`, then run `make -f Makefile.ocp generate-plugins`.

If a kept plugin imports another plugin as a Go package (not as a registered
Corefile directive), that dependency directory must be listed in the `DEP_DIRS`
variable in `Makefile.ocp`. The build will fail with a clear error if a
dependency is missing.

### CI verification

The pipeline should run:

```
make -f Makefile.ocp verify-plugins
```

This checks that `plugin.cfg` matches `openshift-plugins.cfg` and that no
unused plugin directories are present.
