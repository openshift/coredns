package test

import (
	"slices"
	"strings"
	"testing"

	"github.com/coredns/caddy/caddyfile"
	"github.com/coredns/coredns/core/dnsserver"
)

// unavailableDirectives identifies recognized upstream directives absent from this build.
func unavailableDirectives(corefile string) ([]string, error) {
	upstreamOnly := []string{"root", "metadata", "geoip", "proxyproto", "quic", "grpc_server", "https", "https3", "timeouts", "multisocket", "nsid", "bind", "debug", "trace", "pprof", "shed", "dnstap", "local", "dns64", "any", "loadbalance", "tsig", "rewrite", "autopath", "acl", "header", "dnssec", "tls", "minimal", "template", "transfer", "hosts", "route53", "azure", "clouddns", "k8s_external", "auto", "secondary", "etcd", "grpc", "erratic", "whoami", "on", "sign", "view", "nomad"}
	blocks, err := caddyfile.Parse("Corefile", strings.NewReader(corefile), nil)
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
	if err == nil && len(unavailable) != 0 {
		t.Skipf("upstream fixture requires directives outside openshift-plugins.cfg: %s", strings.Join(unavailable, ", "))
	}
}

// TestOpenShiftCorefileApplicability checks disabled-directive detection without hiding invalid configurations.
func TestOpenShiftCorefileApplicability(t *testing.T) {
	cases := []struct {
		name, corefile string
		unavailable    []string
	}{
		{"enabled", ".:0 {\nforward . 127.0.0.1:53 {\nforce_tcp\n}\ncache 30\n}", nil},
		{"restored file", ".:0 {\nfile example.org\n}", nil},
		{"restored hosts", ".:0 {\nhosts\n}", nil},
		{"restored template and debug", ".:0 {\ntemplate IN A\ndebug\n}", nil},
		{"enabled metadata", ".:0 {\nmetadata\n}", nil},
		{"restored fixture plugins", ".:0 {\nerratic\nwhoami\n}", nil},
		{"removed", ".:0 {\nrewrite\n}", []string{"rewrite"}},
		{"code only", ".:0 {\ntransfer\n}", []string{"transfer"}},
		{"unknown is not skipped", ".:0 {\nforwad . 127.0.0.1\n}", nil},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := unavailableDirectives(testCase.corefile)
			if err != nil || !slices.Equal(got, testCase.unavailable) {
				t.Fatalf("unavailable directives = %v, %v; want %v", got, err, testCase.unavailable)
			}
		})
	}
}
