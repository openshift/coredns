package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// snapshot records fixture contents, modes and links, including retained directories.
func snapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	result := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		value := fmt.Sprint(info.Mode())
		if info.Mode().IsRegular() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			value += string(data)
		} else if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			value += target
		}
		result[path] = value
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

// TestRejectModuleLocalImports checks that qualified local aliases cannot prune their source.
func TestRejectModuleLocalImports(t *testing.T) {
	for _, manifest := range []string{
		"cache:github.com/coredns/coredns/plugin/cache\n",
		"cache:cache\nother:github.com/coredns/coredns/plugin/cache\n",
		"cache:github.com/coredns/coredns/plugin/cache\nother:cache\n",
	} {
		t.Run(manifest, func(t *testing.T) {
			root := fixture(t)
			writeFixture(t, root, "plugin/cache/sentinel", "selected source")
			writeFixture(t, root, "outside-sentinel", "unchanged")
			writeFixture(t, root, "plugin.cfg", "previous configuration")
			writeFixture(t, root, "openshift-plugins.cfg", manifest)
			before := snapshot(t, root)
			if err := manage(root, "prune"); err == nil || !strings.Contains(err.Error(), "short local") {
				t.Fatalf("expected qualified-local rejection, got %v", err)
			}
			if !reflect.DeepEqual(before, snapshot(t, root)) {
				t.Fatal("invalid manifest mutated fixture")
			}
		})
	}
}

// TestRejectUnsafeConfiguration checks destinations before any stale plugin is pruned.
func TestRejectUnsafeConfiguration(t *testing.T) {
	for _, kind := range []string{"symlink", "dangling symlink", "directory"} {
		t.Run(kind, func(t *testing.T) {
			root := fixture(t)
			outside := t.TempDir()
			writeFixture(t, outside, "sentinel", "outside checkout")
			destination := filepath.Join(root, "plugin.cfg")
			var err error
			if kind == "directory" {
				err = os.Mkdir(destination, 0o755)
			} else {
				target := filepath.Join(outside, "sentinel")
				if kind == "dangling symlink" {
					target = filepath.Join(outside, "missing")
				}
				err = os.Symlink(target, destination)
			}
			if err != nil {
				t.Fatal(err)
			}
			before, outsideBefore := snapshot(t, root), snapshot(t, outside)
			if err := manage(root, "prune"); err == nil || !strings.Contains(err.Error(), "regular file") {
				t.Fatalf("expected unsafe destination rejection, got %v", err)
			}
			if !reflect.DeepEqual(before, snapshot(t, root)) || !reflect.DeepEqual(outsideBefore, snapshot(t, outside)) {
				t.Fatal("unsafe destination mutated checkout or outside fixture")
			}
		})
	}
}

// TestConfigurationReplacement checks permissions, linked inodes and temporary-file cleanup.
func TestConfigurationReplacement(t *testing.T) {
	for _, kind := range []string{"missing", "regular", "hardlink"} {
		t.Run(kind, func(t *testing.T) {
			root := fixture(t)
			outside := t.TempDir()
			writeFixture(t, outside, "sentinel", "outside checkout")
			destination := filepath.Join(root, "plugin.cfg")
			wantMode := os.FileMode(0o644)
			if kind == "regular" {
				writeFixture(t, root, "plugin.cfg", "old")
			} else if kind == "hardlink" {
				if err := os.Link(filepath.Join(outside, "sentinel"), destination); err != nil {
					t.Fatal(err)
				}
			}
			if kind != "missing" {
				wantMode = 0o640
				if err := os.Chmod(destination, wantMode); err != nil {
					t.Fatal(err)
				}
			}
			outsideBefore := snapshot(t, outside)
			for range 2 {
				if err := manage(root, "prune"); err != nil {
					t.Fatal(err)
				}
				if err := manage(root, "verify"); err != nil {
					t.Fatal(err)
				}
				info, err := os.Lstat(destination)
				if err != nil || info.Mode().Perm() != wantMode {
					t.Fatalf("destination mode: %v, %v", info, err)
				}
				matches, err := filepath.Glob(filepath.Join(root, ".plugin.cfg-*"))
				if err != nil || len(matches) != 0 {
					t.Fatalf("temporary debris: %v, %v", matches, err)
				}
			}
			if !reflect.DeepEqual(outsideBefore, snapshot(t, outside)) {
				t.Fatal("linked outside file changed")
			}
		})
	}
}

// ownersGenerationBounds reserves watchdog time for pipe cleanup without expiring short budgets.
func ownersGenerationBounds(now, deadline time.Time, limited bool) (time.Time, time.Duration, error) {
	if !limited {
		deadline = now.Add(5 * time.Minute)
	}
	remaining := deadline.Sub(now)
	reserve := min(10*time.Second, remaining/4)
	if reserve < 2*time.Nanosecond {
		return time.Time{}, 0, fmt.Errorf("insufficient owners generation budget: %s", remaining)
	}
	return deadline.Add(-reserve), min(5*time.Second, reserve/2), nil
}

// TestOwnersGenerationBounds checks watchdog and pipe-wait budgets including rounding boundaries.
func TestOwnersGenerationBounds(t *testing.T) {
	now := time.Unix(1700000000, 0)
	for _, testCase := range []struct {
		name        string
		remaining   time.Duration
		limited     bool
		wantCommand time.Duration
		wantWait    time.Duration
		wantError   bool
	}{
		{"long", 2 * time.Minute, true, 110 * time.Second, 5 * time.Second, false},
		{"ten seconds", 10 * time.Second, true, 7500 * time.Millisecond, 1250 * time.Millisecond, false},
		{"one second", time.Second, true, 750 * time.Millisecond, 125 * time.Millisecond, false},
		{"minimum", 8 * time.Nanosecond, true, 6 * time.Nanosecond, time.Nanosecond, false},
		{"rounding", 7 * time.Nanosecond, true, 0, 0, true},
		{"expired", -time.Second, true, 0, 0, true},
		{"zero", 0, true, 0, 0, true},
		{"fallback", 0, false, 290 * time.Second, 5 * time.Second, false},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			deadline := now.Add(testCase.remaining)
			commandDeadline, waitDelay, err := ownersGenerationBounds(now, deadline, testCase.limited)
			if (err != nil) != testCase.wantError {
				t.Fatalf("bounds error: %v", err)
			}
			if testCase.wantError {
				return
			}
			if commandDeadline.Sub(now) != testCase.wantCommand || waitDelay != testCase.wantWait {
				t.Fatalf("command budget %s, wait %s", commandDeadline.Sub(now), waitDelay)
			}
			if !testCase.limited {
				deadline = now.Add(5 * time.Minute)
			}
			reserve := deadline.Sub(commandDeadline)
			if !commandDeadline.After(now) || waitDelay <= 0 || waitDelay > reserve/2 || reserve > 10*time.Second {
				t.Fatalf("unsafe command deadline %v, reserve %s, wait %s", commandDeadline, reserve, waitDelay)
			}
		})
	}
}

// TestOwnersGenerationCanceledParent checks that a future child deadline never revives cancellation.
func TestOwnersGenerationCanceledParent(t *testing.T) {
	parent, cancelParent := context.WithCancel(t.Context())
	cancelParent()
	deadline, waitDelay, err := ownersGenerationBounds(time.Now(), time.Time{}, false)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithDeadline(parent, deadline)
	defer cancel()
	command := exec.CommandContext(ctx, "go", "generate", "coredns.go")
	command.Dir = t.TempDir()
	command.WaitDelay = waitDelay
	output, err := command.CombinedOutput()
	if !errors.Is(err, context.Canceled) || len(output) != 0 || command.Process != nil {
		t.Fatalf("canceled parent launched command: %v, %s", err, output)
	}
}

// TestOwnersGeneration exercises the real go:generate directives with optional CHAOS source.
func TestOwnersGeneration(t *testing.T) {
	for _, kind := range []string{"present", "missing", "not directory", "parent not directory", "write error"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			for _, name := range []string{"coredns.go", "directives_generate.go", "owners_generate.go"} {
				data, err := os.ReadFile(filepath.Join("../..", name))
				if err != nil {
					t.Fatal(err)
				}
				writeFixture(t, root, name, string(data))
			}
			writeFixture(t, root, "CODEOWNERS", "* @zebra @alpha\n")
			writeFixture(t, root, "go.mod", "module fixture\n\ngo 1.26.6\n")
			writeFixture(t, root, "plugin.cfg", "")
			for _, directory := range []string{"core/plugin", "core/dnsserver", "plugin"} {
				if err := os.MkdirAll(filepath.Join(root, directory), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			if kind == "present" || kind == "write error" {
				if err := os.Mkdir(filepath.Join(root, "plugin/chaos"), 0o755); err != nil {
					t.Fatal(err)
				}
			} else if kind == "not directory" {
				writeFixture(t, root, "plugin/chaos", "not a directory")
			} else if kind == "parent not directory" {
				if err := os.Remove(filepath.Join(root, "plugin")); err != nil {
					t.Fatal(err)
				}
				writeFixture(t, root, "plugin", "not a directory")
			}
			if kind == "write error" {
				if err := os.Mkdir(filepath.Join(root, "plugin/chaos/zowners.go"), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			deadline, limited := t.Deadline()
			commandDeadline, waitDelay, err := ownersGenerationBounds(time.Now(), deadline, limited)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithDeadline(t.Context(), commandDeadline)
			defer cancel()
			command := exec.CommandContext(ctx, "go", "generate", "coredns.go")
			command.WaitDelay = waitDelay
			command.Dir = root
			command.Env = append(os.Environ(), "COREDNS_PLUGINS=")
			output, err := command.CombinedOutput()
			if ctx.Err() != nil || errors.Is(err, exec.ErrWaitDelay) {
				t.Fatalf("go generate did not complete within its bounds: %v (context %v): %s", err, ctx.Err(), output)
			}
			if kind == "not directory" || kind == "parent not directory" || kind == "write error" {
				if err == nil || !strings.Contains(string(output), "plugin/chaos") {
					t.Fatalf("expected CHAOS filesystem error, got %v: %s", err, output)
				}
				return
			}
			if err != nil {
				t.Fatalf("go generate: %v: %s", err, output)
			}
			if kind == "missing" {
				if _, err := os.Lstat(filepath.Join(root, "plugin/chaos")); !os.IsNotExist(err) {
					t.Fatalf("recreated absent CHAOS: %v", err)
				}
				return
			}
			want := "package chaos\n\n// Owners are all GitHub handlers of all maintainers.\nvar Owners = []string{\"alpha\", \"zebra\"}\n"
			got, err := os.ReadFile(filepath.Join(root, "plugin/chaos/zowners.go"))
			if err != nil || string(got) != want {
				t.Fatalf("present CHAOS output changed: %v", err)
			}
		})
	}
}

// TestConfigurationReplacementFailure checks primary rename errors and best-effort temporary cleanup.
func TestConfigurationReplacementFailure(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "plugin.cfg/sentinel", "unchanged")
	before := snapshot(t, root)
	err := replaceConfiguration(root, []byte("new configuration"), 0o640)
	var linkError *os.LinkError
	if !errors.As(err, &linkError) || linkError.Op != "rename" {
		t.Fatalf("expected primary rename failure, got %v", err)
	}
	if !reflect.DeepEqual(before, snapshot(t, root)) {
		t.Fatal("failed replacement changed destination or left temporary debris")
	}
}

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
