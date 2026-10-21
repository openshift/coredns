package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// localName restricts manifest directive and local package names to safe identifiers.
var localName = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// externalPackage restricts external imports to safe Go package path characters.
var externalPackage = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._/-]*$`)

// exceptions retains infrastructure and code/test dependencies without enabling their directives.
var exceptions = []string{"pkg", "test", "deprecated", "import", "dnstap", "etcd", "transfer", "bind"}

// configuration contains the manifest and its ordered outputs and retained directories.
type configuration struct {
	data        []byte
	directives  []string
	imports     []string
	directories []string
}

// main selects plugin pruning or read-only verification.
func main() {
	mode := flag.String("mode", "verify", "prune or verify the OpenShift plugin configuration")
	flag.Parse()
	if err := manage(".", *mode); err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(1)
	}
}

// readConfiguration validates the manifest and required directories before any pruning.
func readConfiguration(root string) (configuration, error) {
	config := configuration{directories: slices.Clone(exceptions)}
	pluginRoot, err := os.Lstat(filepath.Join(root, "plugin"))
	if err != nil {
		return config, err
	}
	if !pluginRoot.IsDir() {
		return config, fmt.Errorf("plugin root must be a directory, not a symlink or file")
	}
	data, err := os.ReadFile(filepath.Join(root, "openshift-plugins.cfg"))
	if err != nil {
		return config, err
	}
	config.data = data
	packages := map[string]bool{}
	for index, line := range strings.Split(string(data), "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name, path, ok := strings.Cut(line, ":")
		if !ok || !localName.MatchString(name) || line != strings.TrimSpace(line) || strings.Contains(path, ":") {
			return config, fmt.Errorf("openshift-plugins.cfg:%d: expected name:package", index+1)
		}
		if slices.Contains(config.directives, name) || packages[path] {
			return config, fmt.Errorf("openshift-plugins.cfg:%d: duplicate directive or package", index+1)
		}
		packages[path] = true
		config.directives = append(config.directives, name)
		if localName.MatchString(path) {
			config.directories = append(config.directories, path)
			config.imports = append(config.imports, "github.com/coredns/coredns/plugin/"+path)
		} else {
			if !externalPackage.MatchString(path) || !strings.Contains(path, "/") || !strings.Contains(strings.Split(path, "/")[0], ".") {
				return config, fmt.Errorf("openshift-plugins.cfg:%d: invalid package %q", index+1, path)
			}
			for _, segment := range strings.Split(path, "/") {
				if segment == "" || segment == "." || segment == ".." {
					return config, fmt.Errorf("openshift-plugins.cfg:%d: invalid package path %q", index+1, path)
				}
			}
			config.imports = append(config.imports, path)
		}
	}
	if len(config.directives) == 0 {
		return config, fmt.Errorf("openshift-plugins.cfg: empty plugin set")
	}
	for _, directory := range config.directories {
		info, err := os.Lstat(filepath.Join(root, "plugin", directory))
		if err != nil {
			return config, fmt.Errorf("required plugin/%s is missing: %w", directory, err)
		}
		if !info.IsDir() {
			return config, fmt.Errorf("required plugin/%s must be a directory, not a symlink or file", directory)
		}
	}
	return config, nil
}

// manage prunes unused plugin directories or verifies the tree against the manifest.
func manage(root, mode string) error {
	if mode != "prune" && mode != "verify" {
		return fmt.Errorf("unknown mode %q", mode)
	}
	config, err := readConfiguration(root)
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(filepath.Join(root, "plugin"))
	if err != nil {
		return err
	}
	var stale []string
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("plugin/%s is a symlink; refusing plugin management", entry.Name())
		}
		if entry.IsDir() && !slices.Contains(config.directories, entry.Name()) {
			stale = append(stale, entry.Name())
		}
	}
	if mode == "prune" {
		for _, directory := range stale {
			if err := os.RemoveAll(filepath.Join(root, "plugin", directory)); err != nil {
				return err
			}
		}
		return os.WriteFile(filepath.Join(root, "plugin.cfg"), config.data, 0o644)
	}
	generated, err := os.ReadFile(filepath.Join(root, "plugin.cfg"))
	if err != nil {
		return err
	}
	if !bytes.Equal(config.data, generated) {
		return fmt.Errorf("plugin.cfg differs from openshift-plugins.cfg; run make -f Makefile.ocp generate-plugins")
	}
	if len(stale) > 0 {
		return fmt.Errorf("stale plugin directories: %s; run make -f Makefile.ocp generate-plugins", strings.Join(stale, ", "))
	}
	return verifyGenerated(root, config)
}

// verifyGenerated checks generated imports and directive order against the manifest.
func verifyGenerated(root string, config configuration) error {
	file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(root, "core/plugin/zplugin.go"), nil, 0)
	if err != nil {
		return err
	}
	var imports []string
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			return err
		}
		if spec.Name == nil || spec.Name.Name != "_" {
			return fmt.Errorf("core/plugin/zplugin.go: expected blank plugin import")
		}
		imports = append(imports, path)
	}
	slices.Sort(imports)
	slices.Sort(config.imports)
	if !slices.Equal(imports, config.imports) {
		return fmt.Errorf("core/plugin/zplugin.go imports differ from openshift-plugins.cfg; regenerate plugins")
	}
	file, err = parser.ParseFile(token.NewFileSet(), filepath.Join(root, "core/dnsserver/zdirectives.go"), nil, 0)
	if err != nil {
		return err
	}
	var directives []string
	valid := true
	ast.Inspect(file, func(node ast.Node) bool {
		spec, ok := node.(*ast.ValueSpec)
		if !ok || len(spec.Names) != 1 || spec.Names[0].Name != "Directives" || len(spec.Values) != 1 {
			return true
		}
		literal, ok := spec.Values[0].(*ast.CompositeLit)
		if !ok {
			valid = false
			return false
		}
		for _, element := range literal.Elts {
			value, ok := element.(*ast.BasicLit)
			if !ok || value.Kind != token.STRING {
				valid = false
				return false
			}
			text, err := strconv.Unquote(value.Value)
			if err == nil {
				directives = append(directives, text)
			}
		}
		return false
	})
	if !valid || !slices.Equal(directives, config.directives) {
		return fmt.Errorf("core/dnsserver/zdirectives.go order differs from openshift-plugins.cfg; regenerate plugins")
	}
	return nil
}
