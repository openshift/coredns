## Toolchain Management Strategy

Downstream OpenShift builds occur in disconnected/offline environments and cannot download Go toolchains from the network. This carry commit ensures builds use whatever Go version ART provides in builder images.

### Carry Commit Strategy

**This is an unconditional, permanent carry commit.** Always apply it during every rebase.

```
UPSTREAM: <carry>: openshift: set GOTOOLCHAIN=local for offline builds
```

### The Problem

Upstream CoreDNS sets:
```makefile
export GOTOOLCHAIN = go$(GOLANG_VERSION)
```

This expands to something like `GOTOOLCHAIN = go1.25.2` based on `.go-version`. When the Go toolchain in ART's builder image is older (e.g., 1.24.6), Go attempts to download the newer version from the network. In disconnected ART builds, this download fails.

### The Solution

Set `GOTOOLCHAIN = local` to tell Go: "Use whatever Go version is installed locally, do NOT attempt network downloads."

### Reapply Instructions

Edit `Makefile`:
```makefile
export GOSUMDB = sum.golang.org
export GOTOOLCHAIN = local
```

Create carry commit:
```bash
git add Makefile
git commit -m "UPSTREAM: <carry>: openshift: set GOTOOLCHAIN=local for offline builds

ART builds occur in disconnected environments and cannot download Go
toolchains from the network. Setting GOTOOLCHAIN=local ensures builds
use whatever Go version is provided in the builder image."
```

### What NOT to do

**Do NOT downgrade `.go-version`** to match ART builder versions. Keep `.go-version` at upstream's value. ART automation updates `Dockerfile.ocp` based on builder images, but does NOT touch `.go-version`. These files can (and do) diverge - that's expected and fine.

### Validation

```bash
# Verify build works
make

# Verify tests pass
make test
```

### Related Files

- `Makefile` - Where `GOTOOLCHAIN` is set
- `.go-version` - Upstream's Go requirement (leave at upstream value)
- `Dockerfile.ocp` - ART's builder image reference (ART automation controls this)
