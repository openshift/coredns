package test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/coredns/caddy/caddyfile"
	"github.com/coredns/coredns/core/dnsserver"
)

// TestOpenShiftOperatorCorefileSnapshotParsing is a directive-recognition tripwire
// for a hand-maintained, adapted snapshot, not operator-compatibility proof.
// Comparison baseline verified for this annotation (not the original copy revision):
// openshift/cluster-dns-operator, pkg/operator/controller/controller_dns_configmap.go,
// corefileTemplate at 9d18bdd71092bd9eaad426c6ec42b719c3a992e9 (2026-10-16 UTC).
// The fixture substitutes one example zone/upstream, cluster domain, TTLs, log
// class, lameduck duration and resolver namespaces; flattens template branches;
// abbreviates the revisioned CA path; omits generated server-name comments; and
// changes whitespace/option ordering. It combines TLS forwarding and force_tcp
// syntactically without exercising operator rendering or plugin argument checks.
// Review this snapshot against operator template changes and update its baseline
// and adaptations deliberately: there is no automatic drift detection here.
// Only caddyfile parsing with the retained directive list is checked, not plugin
// setup, startup/runtime, TLS test-fixture compatibility or cluster acceptance.
func TestOpenShiftOperatorCorefileSnapshotParsing(t *testing.T) {
	for _, resolver := range []bool{false, true} {
		t.Run(fmt.Sprintf("resolver=%t", resolver), func(t *testing.T) {
			resolverConfig := ""
			if resolver {
				resolverConfig = "ocp_dnsnameresolver {\n namespaces openshift-ingress openshift-ingress-operator\n}\n"
			}
			corefile := `example.org:5353 {
prometheus 127.0.0.1:9153
forward . tls://192.0.2.1 {
 policy round_robin
 force_tcp
 tls_servername dns.example.org
 tls /etc/pki/example-ca.crt
}

errors
log . {
 class error
}
bufsize 1232
cache 900 {
 denial 9984 30
}
` + resolverConfig + `}
.:5353 {
bufsize 1232
errors
log . {
 class error
}
health {
 lameduck 20s
}
ready
kubernetes cluster.local in-addr.arpa ip6.arpa {
 pods insecure
 fallthrough in-addr.arpa ip6.arpa
}
prometheus 127.0.0.1:9153
forward . /etc/resolv.conf {
 policy sequential
}
cache 900 {
 denial 9984 30
}
reload
` + resolverConfig + `}
hostname.bind:5353 {
chaos
}`
			if _, err := caddyfile.Parse("operator Corefile", strings.NewReader(corefile), dnsserver.Directives); err != nil {
				t.Fatalf("operator directives are incompatible: %v", err)
			}
		})
	}
}
