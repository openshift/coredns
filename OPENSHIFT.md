# OpenShift CoreDNS fork

This maintainer guide describes the plugin-selection and dependency-reduction
mechanism and the plugin lifecycle: adding, removing, and rebasing plugins.
It explains source/module/vendor ordering, not an agent implementation plan
or a record of development steps.

The primary goal is to reduce vendored dependencies, not local source lines.
Carry-patch maintainability and reviewability also matter: removing a plugin
when that forces substantial changes to retained implementations or tests adds
undue maintenance burden. Avoid deleting code when doing so increases the
maintenance burden for retained code. Local line-count reduction is secondary.
Keep file and hosts so upstream file-backed fixtures and cache regressions can
remain unchanged. Enable erratic and whoami so retained forwarding, reload,
and other fixtures use upstream plugins rather than custom DNS responders.
Enable template for synthesized file-upstream responses and debug for retained
multi-zone fixtures, preserving their original implementations and helpers.
Enable metadata so the retained cache stale-serving race regression can run
with its original logging and direct server helper rather than an applicability skip.

## Plugin source of truth

`openshift-plugins.cfg` is the authoritative ordered `name:package` manifest.
`plugin.cfg`, `core/plugin/zplugin.go`, and `core/dnsserver/zdirectives.go`
are generated from it. `prometheus:metrics` deliberately maps a directive to
a differently named local package. The external `ocp_dnsnameresolver` package
is kept at its pinned Go module version. Generation clears `COREDNS_PLUGINS`
so environment-only registrations cannot change this set.

```sh
make -f Makefile.ocp generate-plugins
make -f Makefile.ocp verify-plugins
```

Generation validates the manifest and retained directories before pruning,
copies the manifest to `plugin.cfg`, removes unused plugin implementations
and their tests, regenerates Go registration files (and CHAOS owners), and
runs `go mod tidy`. It NEVER vendors. Verification is read-only and reports
manifest mismatch, stale/missing directories, stale imports, or directive
order. `make check` and therefore builds/tests verify the configuration;
`make gen` runs the same generator. A second generation must produce no diff.

## Retained directory exceptions

Keep root `plugin/*.go` and infrastructure directories `pkg`, `test`,
`deprecated`, and `import`. The shared exception list is in
`internal/openshiftplugins/main.go`. These additional directories are code
or test dependencies, not enabled Corefile directives:

| Directory | Importing code |
| --- | --- |
| `dnstap` | forward implementation and tests |
| `etcd` | Kubernetes `etcd/msg` imports |
| `transfer` | file and Kubernetes transfer code and tests |
| `bind` | `core/dnsserver/example_test.go` |

Dependency directories retain their implementation and applicable tests; the
disabled dnstap directive's setup test is explicitly removed. Some dependencies
therefore remain even without an enabled runtime directive.
Do not add exceptions merely to enable more runtime plugins.
The metadata, debug, template, hosts, erratic, and whoami packages are enabled through the manifest,
not directory exceptions; their original implementations and tests are kept.

## Add, remove, or rebase a plugin

For additions, first restore any previously pruned implementation from the
matching upstream CoreDNS version. Edit the manifest, preserving middleware
order, and update documented code/test dependency exceptions when necessary.
Run generation and verification, then separately run `go mod vendor`.
For removals, remove the manifest entry and follow the same sequence. Review
Go package imports including retained tests before removing an exception.

During an upstream rebase, preserve this order:

1. Apply upstream changes and preceding OpenShift carry commits, including
   the external resolver and dependency pins.
2. Apply the `UPSTREAM: <carry>:` dependency-reduction change, run
   `make -f Makefile.ocp generate-plugins`, and commit the reduced source,
   generated registration, and tidy metadata WITHOUT vendor files.
3. Recreate the separate vendor carry with `go mod vendor` and commit only
   `vendor/`. Never replay the old full vendor payload as the reduced result.
4. Replay reports and remaining carries, then run the repository checks.

Both carry commit messages must state that reduction precedes vendoring.
Historical rebase reports are not evidence of validation of a later plugin
change or consumer approval.

Review upstream changes under `test/` against the previous upstream baseline,
including files deleted by this carry. For example, use
`git diff --name-status OLD_UPSTREAM NEW_UPSTREAM -- test/` and then inspect
the changed functions, imports, Corefiles, and assertions. A filename alone
does not establish which plugin a test covers. If upstream adds a retained
plugin or core regression to a deleted file, restore that test and its required
helpers or reconsider retaining the whole file. For mixed files, reconcile
partial deletions function by function and review any changed shared fixtures.
Modify/delete conflicts are useful audit cues, not complete detection: new
files and cleanly applied patches also need review. There is no automated
guarantee that rebasing preserves newly added retained-behavior coverage.

## Consumer compatibility and validation

The cluster-dns-operator Corefile uses reload, bufsize, ready, health,
prometheus, errors, log, chaos, conditional ocp_dnsnameresolver, cache,
kubernetes, and forward. `cancel` and `loop` are conservative retentions
from the existing OpenShift selection, not requirements established by that
operator template.
Forwarding TLS options do not require the separate server-side `tls`
directive; `hostname.bind` uses CHAOS, not the `bind` directive.

On-Prem Networking and all impacted image consumers must review the final
set before it is finalized. No such signoff is asserted here. Other
consumers may require additional directives; obtain actual requirements
rather than treating the operator template as a complete consumer inventory.

Delete integration tests whose subject is exclusively a removed plugin or an
unsupported transport; do not retain useless tests merely to add skip rules.
Judge mixed coverage by the intended assertions, not the presence of an empty
contrast control or an incidental responder. Preserve actual mixed retained-plugin
and security regressions when a removed directive is only a fixture dependency.
Those fixtures report explicit per-Corefile applicability
skips, not passing behavior coverage, and do not add test-only directives to
the production registry. Existing retained-plugin integration tests continue
to exercise their applicable behavior.
Invalid-Corefile security regressions still execute without applicability
filtering. README examples are checked individually against the runtime set.
Retained plugin unit tests and core embedding examples remain intact.

Only recognized upstream-only directives can cause an applicability skip;
unknown directives and malformed enabled-plugin configurations still fail.
Cache TTL/EDNS integration cases retain their upstream file-backed fixtures.
The metadata-backed cache regression uses its original direct startup helper
and all original assertions; metadata is enabled. The original whoami/erratic forwarding and reload fixtures
and hosts-backed file-upstream fixtures are restored without custom responders.
The separate file-upstream error fixture uses the restored template plugin;
file-upstream tests now use their original server helpers without guards.
The debug multi-zone fixture also uses its original helper without a guard.
Tests for unsupported QUIC, HTTP/3, gRPC, DoH, and server TLS transports, etcd integration,
ACL/cache interaction, and rewrite/auto-specific metrics are removed; their
production transport implementations are not removed. Multisocket tests are
removed, including their empty contrast case and incidental whoami responder;
`test/multisocket_test.go` retains only `pickPort`, used by reload tests.
The disabled dnstap directive setup tests are removed while its other package
tests and implementation remain. The large-AXFR test is deleted because the
transfer directive is disabled; its zone was generated privately inside the test,
so no shared fixture assets are removed. Transfer implementation remains required
by file and Kubernetes. No startup applicability wrappers remain; README examples
retain per-example filtering. Skipped behavior is not claimed as executed coverage.
The operator fixture in `test/openshift_operator_test.go`
is derived from cluster-dns-operator's `controller_dns_configmap.go` template
and exercises directive parsing with the resolver enabled and disabled.
This is a parsing compatibility check, not a duplicate operator E2E suite.
Forwarding and TLS behavior are covered by retained plugin tests and the
cluster-dns-operator E2E suite. Kubernetes API synchronization, resolver-on
serving, and cluster behavior still require cluster validation: the external
resolver initializes with `rest.InClusterConfig`.

Use `make test`, `make gen`, applicable configured race checks, and the
`Dockerfile.ocp` image build. After the separate vendor carry, dependency CI
runs tidy and vendor and requires no diff, including ignored vendor files.
Validate operator-derived Corefiles against the actual reduced binary/image
and obtain applicable cluster E2E results and required human consumer review.
