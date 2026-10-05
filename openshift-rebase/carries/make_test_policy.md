## Downstream `make test` target

- Keep a single `test` target that the OpenShift ci-operator calls.
- Target must depend on `check` to regenerate `zplugin.go`/`zdirectives.go`.
- Use module-aware testing with vendored deps:

```make
.PHONY: test
test: check
	GOFLAGS=-mod=vendor go test -count=1 ./...
```

- Avoid per-package subshell loops; Go 1.24 handles parallelization itself.
- Ensure CI jobs set `GOFLAGS=-mod=vendor` (either via environment or within the target as shown).
