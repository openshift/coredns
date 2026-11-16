package test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/coredns/caddy/caddyfile"
	"github.com/coredns/coredns/core/dnsserver"
)

// unavailableDirectives identifies recognized upstream directives absent from this build.
func unavailableDirectives(corefile string) ([]string, error) {
	upstreamOnly := []string{"root", "metadata", "geoip", "proxyproto", "quic", "grpc_server", "https", "https3", "timeouts", "multisocket", "nsid", "bind", "debug", "trace", "pprof", "shed", "dnstap", "local", "dns64", "any", "loadbalance", "tsig", "rewrite", "autopath", "acl", "header", "dnssec", "tls", "minimal", "template", "transfer", "hosts", "route53", "azure", "clouddns", "k8s_external", "auto", "secondary", "etcd", "grpc", "erratic", "whoami", "on", "sign", "view", "nomad"}
	blocks, err := caddyfile.Parse("Corefile", strings.NewReader(corefile), append(slices.Clone(dnsserver.Directives), upstreamOnly...))
	if err != nil {
		return nil, err
	}
	var unavailable []string
	for _, block := range blocks {
		for name := range block.Tokens {
			if slices.Contains(upstreamOnly, name) && !slices.Contains(dnsserver.Directives, name) && !slices.Contains(unavailable, name) {
				unavailable = append(unavailable, name)
			}
		}
	}
	slices.Sort(unavailable)
	return unavailable, nil
}

// requireOpenShiftCorefile skips only fixtures requiring recognized disabled directives.
func requireOpenShiftCorefile(t testing.TB, corefile string) {
	t.Helper()
	unavailable, err := unavailableDirectives(corefile)
	if err != nil {
		t.Fatal(err)
	}
	if len(unavailable) != 0 {
		t.Skipf("upstream fixture requires directives outside openshift-plugins.cfg: %s", strings.Join(unavailable, ", "))
	}
}

// TestOpenShiftCorefileApplicability checks disabled-directive detection without hiding invalid configurations.
func TestOpenShiftCorefileApplicability(t *testing.T) {
	cases := []struct {
		name, corefile string
		unavailable    []string
		parseError     string
	}{
		{"enabled", ".:0 {\nforward . 127.0.0.1:53 {\nforce_tcp\n}\ncache 30\n}", nil, ""},
		{"restored file", ".:0 {\nfile example.org\n}", nil, ""},
		{"restored hosts", ".:0 {\nhosts\n}", nil, ""},
		{"restored template and debug", ".:0 {\ntemplate IN A\ndebug\n}", nil, ""},
		{"restored TLS listener", "tls://.:0 {\ntls cert.pem key.pem\n}", nil, ""},
		{"enabled metadata", ".:0 {\nmetadata\n}", nil, ""},
		{"restored fixture plugins", ".:0 {\nerratic\nwhoami\n}", nil, ""},
		{"removed", ".:0 {\nrewrite\n}", []string{"rewrite"}, ""},
		{"code only", ".:0 {\ntransfer\n}", []string{"transfer"}, ""},
		{"unknown is not skipped", ".:0 {\nforwad . 127.0.0.1\n}", nil, "Unknown directive 'forwad'"},
		{"disabled then unknown", ".:0 {\nrewrite\nforwad . 127.0.0.1\n}", nil, "Unknown directive 'forwad'"},
		{"unknown then disabled", ".:0 {\nforwad . 127.0.0.1\nrewrite\n}", nil, "Unknown directive 'forwad'"},
		{"enabled then unknown", ".:0 {\ncache\nforwad\n}", nil, "Unknown directive 'forwad'"},
		{"unknown then enabled", ".:0 {\nforwad\ncache\n}", nil, "Unknown directive 'forwad'"},
		{"unknown in second block", ".:0 {\nrewrite\n}\nexample.org:0 {\nforwad\n}", nil, "Unknown directive 'forwad'"},
		{"unknown in first block", ".:0 {\nforwad\n}\nexample.org:0 {\nrewrite\n}", nil, "Unknown directive 'forwad'"},
		{"malformed disabled", ".:0 {\nrewrite {\n", nil, "Unexpected EOF"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := unavailableDirectives(testCase.corefile)
			if testCase.parseError != "" {
				if err == nil || !strings.Contains(err.Error(), testCase.parseError) {
					t.Fatalf("expected %q, got %v", testCase.parseError, err)
				}
				return
			}
			if err != nil || !slices.Equal(got, testCase.unavailable) {
				t.Fatalf("unavailable directives = %v, %v; want %v", got, err, testCase.unavailable)
			}
		})
	}
}

// TestOpenShiftCorefileGate exercises the actual README applicability gate without starting servers.
func TestOpenShiftCorefileGate(t *testing.T) {
	if corefile, child := os.LookupEnv("COREDNS_COREFILE_GATE_INPUT"); child {
		requireOpenShiftCorefile(t, corefile)
		t.Log("APPLICABILITY_ACCEPTED")
		return
	}
	for _, testCase := range []struct {
		name, corefile, diagnostic string
		fail, skip                 bool
	}{
		{"enabled", ".:0 {\ncache\n}", "APPLICABILITY_ACCEPTED", false, false},
		{"disabled", ".:0 {\nrewrite\n}", "outside openshift-plugins.cfg: rewrite", false, true},
		{"unknown", ".:0 {\nforwad\n}", "Unknown directive 'forwad'", true, false},
		{"disabled then unknown", ".:0 {\nrewrite\nforwad\n}", "Unknown directive 'forwad'", true, false},
		{"unknown then disabled", ".:0 {\nforwad\nrewrite\n}", "Unknown directive 'forwad'", true, false},
		{"enabled then unknown", ".:0 {\ncache\nforwad\n}", "Unknown directive 'forwad'", true, false},
		{"unknown then enabled", ".:0 {\nforwad\ncache\n}", "Unknown directive 'forwad'", true, false},
		{"unknown in second block", ".:0 {\nrewrite\n}\nexample.org:0 {\nforwad\n}", "Unknown directive 'forwad'", true, false},
		{"unknown in first block", ".:0 {\nforwad\n}\nexample.org:0 {\nrewrite\n}", "Unknown directive 'forwad'", true, false},
		{"malformed disabled", ".:0 {\nrewrite {\n", "Unexpected EOF", true, false},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestOpenShiftCorefileGate$", "-test.v")
			command.WaitDelay = 5 * time.Second
			command.Env = append(os.Environ(), "COREDNS_COREFILE_GATE_INPUT="+testCase.corefile)
			output, err := command.CombinedOutput()
			text := string(output)
			if ctx.Err() != nil || errors.Is(err, exec.ErrWaitDelay) || (err != nil) != testCase.fail || !strings.Contains(text, testCase.diagnostic) || strings.Contains(text, "--- SKIP:") != testCase.skip {
				t.Fatalf("gate: %v (context %v): %s", err, ctx.Err(), text)
			}
			if (testCase.fail || testCase.skip) && strings.Contains(text, "APPLICABILITY_ACCEPTED") {
				t.Fatalf("invalid or disabled fixture passed gate: %s", text)
			}
		})
	}
}
