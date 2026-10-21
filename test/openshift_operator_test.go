package test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/coredns/caddy/caddyfile"
	"github.com/coredns/coredns/core/dnsserver"
)

// TestOpenShiftOperatorDirectives checks that retained directives parse operator-derived Corefiles with the resolver on and off.
func TestOpenShiftOperatorDirectives(t *testing.T) {
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
