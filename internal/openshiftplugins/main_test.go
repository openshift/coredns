package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fixture creates a minimal plugin tree for isolated reducer tests.
func fixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, directory := range append(exceptions, "metrics", "unused") {
		if err := os.MkdirAll(filepath.Join(root, "plugin", directory), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeFixture(t, root, "openshift-plugins.cfg", "prometheus:metrics\nocp_dnsnameresolver:github.com/openshift/coredns-ocp-dnsnameresolver\n")
	writeFixture(t, root, "core/plugin/zplugin.go", "package plugin\nimport (_ \"github.com/coredns/coredns/plugin/metrics\"; _ \"github.com/openshift/coredns-ocp-dnsnameresolver\")\n")
	writeFixture(t, root, "core/dnsserver/zdirectives.go", "package dnsserver\nvar Directives = []string{\"prometheus\", \"ocp_dnsnameresolver\"}\n")
	writeFixture(t, root, "vendor/sentinel", "unchanged")
	return root
}

// writeFixture writes a fixture file and creates parent directories as needed.
func writeFixture(t *testing.T, root, name, data string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestPruneAndVerify checks pruning, verification, and repeated-run stability.
func TestPruneAndVerify(t *testing.T) {
	root := fixture(t)
	for range 2 {
		if err := manage(root, "prune"); err != nil {
			t.Fatal(err)
		}
		if err := manage(root, "verify"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "plugin/unused")); !os.IsNotExist(err) {
		t.Fatalf("unused plugin remains: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "vendor/sentinel"))
	if err != nil || string(data) != "unchanged" {
		t.Fatalf("vendor changed: %s, %v", data, err)
	}
}

// TestRejectInvalidManifestBeforePruning checks that invalid entries cannot trigger deletion.
func TestRejectInvalidManifestBeforePruning(t *testing.T) {
	cases := []string{"", "broken", "prometheus:metrics\nprometheus:metrics\n", "prometheus:../metrics\n", "prometheus:github.com/../metrics\n", " prometheus:metrics\n", "prometheus:missing\n"}
	for _, manifest := range cases {
		t.Run(manifest, func(t *testing.T) {
			root := fixture(t)
			writeFixture(t, root, "openshift-plugins.cfg", manifest)
			if err := manage(root, "prune"); err == nil {
				t.Fatal("accepted invalid manifest")
			}
			if _, err := os.Stat(filepath.Join(root, "plugin/unused")); err != nil {
				t.Fatalf("pruned before validation: %v", err)
			}
		})
	}
}

// TestVerificationFailures checks diagnostics for stale configuration, outputs, and directories.
func TestVerificationFailures(t *testing.T) {
	cases := []struct{ name, path, data, message string }{
		{"manifest mismatch", "plugin.cfg", "different", "plugin.cfg differs"},
		{"stale directory", "plugin/stale/sentinel", "", "stale plugin directories"},
		{"stale imports", "core/plugin/zplugin.go", "package plugin", "imports differ"},
		{"stale order", "core/dnsserver/zdirectives.go", "package dnsserver\nvar Directives = []string{\"ocp_dnsnameresolver\", \"prometheus\"}", "order differs"},
		{"nonliteral element", "core/dnsserver/zdirectives.go", "package dnsserver\nvar Directives = []string{\"prometheus\", \"ocp_dnsnameresolver\", extra}", "order differs"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			root := fixture(t)
			if err := manage(root, "prune"); err != nil {
				t.Fatal(err)
			}
			writeFixture(t, root, testCase.path, testCase.data)
			if err := manage(root, "verify"); err == nil || !strings.Contains(err.Error(), testCase.message) {
				t.Fatalf("expected %q, got %v", testCase.message, err)
			}
			data, err := os.ReadFile(filepath.Join(root, testCase.path))
			if err != nil || string(data) != testCase.data {
				t.Fatal("verification mutated its input")
			}
		})
	}
}

// TestMissingDependency checks that required retained directories cannot be missing.
func TestMissingDependency(t *testing.T) {
	root := fixture(t)
	if err := os.Remove(filepath.Join(root, "plugin/bind")); err != nil {
		t.Fatal(err)
	}
	if _, err := readConfiguration(root); err == nil || !strings.Contains(err.Error(), "plugin/bind") {
		t.Fatalf("expected missing dependency error, got %v", err)
	}
}

// TestRejectSymlink checks that plugin management rejects symlinked directories.
func TestRejectSymlink(t *testing.T) {
	root := fixture(t)
	if err := os.Symlink(t.TempDir(), filepath.Join(root, "plugin/link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := manage(root, "prune"); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("expected symlink error, got %v", err)
	}
}

// TestRejectSymlinkRoot checks that a symlinked plugin root cannot escape the source tree.
func TestRejectSymlinkRoot(t *testing.T) {
	root := fixture(t)
	external := t.TempDir()
	writeFixture(t, external, "stale/sentinel", "outside checkout")
	if err := os.Rename(filepath.Join(root, "plugin"), filepath.Join(root, "original-plugin")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(root, "plugin")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := manage(root, "prune"); err == nil || !strings.Contains(err.Error(), "plugin root") {
		t.Fatalf("expected unsafe root error, got %v", err)
	}
	data, err := os.ReadFile(filepath.Join(external, "stale/sentinel"))
	if err != nil || string(data) != "outside checkout" {
		t.Fatalf("external content changed: %v", err)
	}
}
