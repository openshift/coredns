package forward

import (
	"context"
	"errors"
	"testing"

	"github.com/coredns/caddy"
	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/pkg/transport"
	"github.com/coredns/coredns/plugin/test"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
)

func TestProxy(t *testing.T) {
	s := dnstest.NewServer(func(w dns.ResponseWriter, r *dns.Msg) {
		ret := new(dns.Msg)
		ret.SetReply(r)
		ret.Answer = append(ret.Answer, test.A("example.org. IN A 127.0.0.1"))
		w.WriteMsg(ret)
	})
	defer s.Close()

	c := caddy.NewTestController("dns", "forward . "+s.Addr)
	fs, err := parseForward(c)
	f := fs[0]
	if err != nil {
		t.Errorf("Failed to create forwarder: %s", err)
	}
	f.OnStartup()
	defer f.OnShutdown()

	m := new(dns.Msg)
	m.SetQuestion("example.org.", dns.TypeA)
	rec := dnstest.NewRecorder(&test.ResponseWriter{})

	if _, err := f.ServeDNS(context.TODO(), rec, m); err != nil {
		t.Fatal("Expected to receive reply, but didn't")
	}
	if x := rec.Msg.Answer[0].Header().Name; x != "example.org." {
		t.Errorf("Expected %s, got %s", "example.org.", x)
	}
}

func TestProxyTLSFail(t *testing.T) {
	// This is an udp/tcp test server, so we shouldn't reach it with TLS.
	s := dnstest.NewServer(func(w dns.ResponseWriter, r *dns.Msg) {
		ret := new(dns.Msg)
		ret.SetReply(r)
		ret.Answer = append(ret.Answer, test.A("example.org. IN A 127.0.0.1"))
		w.WriteMsg(ret)
	})
	defer s.Close()

	c := caddy.NewTestController("dns", "forward . tls://"+s.Addr)
	fs, err := parseForward(c)
	f := fs[0]
	if err != nil {
		t.Errorf("Failed to create forwarder: %s", err)
	}
	f.OnStartup()
	defer f.OnShutdown()

	m := new(dns.Msg)
	m.SetQuestion("example.org.", dns.TypeA)
	rec := dnstest.NewRecorder(&test.ResponseWriter{})

	if _, err := f.ServeDNS(context.TODO(), rec, m); err == nil {
		t.Fatal("Expected *not* to receive reply, but got one")
	}
}

func TestProtocolSelection(t *testing.T) {
	p := NewProxy("bad_address", transport.DNS)

	stateUDP := request.Request{W: &test.ResponseWriter{}, Req: new(dns.Msg)}
	stateTCP := request.Request{W: &test.ResponseWriter{TCP: true}, Req: new(dns.Msg)}
	ctx := context.TODO()

	go func() {
		p.Connect(ctx, stateUDP, options{})
		p.Connect(ctx, stateUDP, options{forceTCP: true})
		p.Connect(ctx, stateUDP, options{preferUDP: true})
		p.Connect(ctx, stateUDP, options{preferUDP: true, forceTCP: true})
		p.Connect(ctx, stateTCP, options{})
		p.Connect(ctx, stateTCP, options{forceTCP: true})
		p.Connect(ctx, stateTCP, options{preferUDP: true})
		p.Connect(ctx, stateTCP, options{preferUDP: true, forceTCP: true})
	}()

	for i, exp := range []string{"udp", "tcp", "udp", "tcp", "tcp", "tcp", "udp", "tcp"} {
		proto := <-p.transport.dial
		p.transport.ret <- nil
		if proto != exp {
			t.Errorf("Unexpected protocol in case %d, expected %q, actual %q", i, exp, proto)
		}
	}
}

func TestCoreDNSOverflow(t *testing.T) {
	s := dnstest.NewServer(func(w dns.ResponseWriter, r *dns.Msg) {
		ret := new(dns.Msg)
		ret.SetReply(r)

		answers := []dns.RR{
			test.A("example.org. IN A 127.0.0.1"),
			test.A("example.org. IN A 127.0.0.2"),
			test.A("example.org. IN A 127.0.0.3"),
			test.A("example.org. IN A 127.0.0.4"),
			test.A("example.org. IN A 127.0.0.5"),
			test.A("example.org. IN A 127.0.0.6"),
			test.A("example.org. IN A 127.0.0.7"),
			test.A("example.org. IN A 127.0.0.8"),
			test.A("example.org. IN A 127.0.0.9"),
			test.A("example.org. IN A 127.0.0.10"),
			test.A("example.org. IN A 127.0.0.11"),
			test.A("example.org. IN A 127.0.0.12"),
			test.A("example.org. IN A 127.0.0.13"),
			test.A("example.org. IN A 127.0.0.14"),
			test.A("example.org. IN A 127.0.0.15"),
			test.A("example.org. IN A 127.0.0.16"),
			test.A("example.org. IN A 127.0.0.17"),
			test.A("example.org. IN A 127.0.0.18"),
			test.A("example.org. IN A 127.0.0.19"),
			test.A("example.org. IN A 127.0.0.20"),
		}
		ret.Answer = answers
		w.WriteMsg(ret)
	})
	defer s.Close()

	p := NewProxy(s.Addr, "TestCoreDNSOverflow")
	f := New()
	f.SetProxy(p)
	defer f.OnShutdown()

	// Test different connection modes
	testConnection := func(proto string, options options, expectTruncated bool) {
		t.Helper()

		queryMsg := new(dns.Msg)
		queryMsg.SetQuestion("example.org.", dns.TypeA)

		recorder := dnstest.NewRecorder(&test.ResponseWriter{})
		request := request.Request{Req: queryMsg, W: recorder}

		response, err := p.Connect(context.Background(), request, options)
		if err != nil {
			t.Errorf("Failed to connect to testdnsserver: %s", err)
		}

		if response.Truncated != expectTruncated {
			t.Errorf("Expected truncated response for %s, but got TC flag %v", proto, response.Truncated)
		}
	}

	// Test PreferUDP, expect truncated response
	testConnection("PreferUDP", options{preferUDP: true}, true)

	// Test ForceTCP, expect no truncated response
	testConnection("ForceTCP", options{forceTCP: true}, false)

	// Test No options specified, expect truncated response
	testConnection("NoOptionsSpecified", options{}, true)

	// Test both TCP and UDP provided, expect no truncated response
	testConnection("BothTCPAndUDP", options{preferUDP: true, forceTCP: true}, false)
}

func TestShouldTruncateResponse(t *testing.T) {
	testCases := []struct {
		testname string
		err      error
		expected bool
	}{
		{"BadAlgorithm", dns.ErrAlg, false},
		{"BufferSizeTooSmall", dns.ErrBuf, true},
		{"OverflowUnpackingA", errors.New("overflow unpacking a"), true},
		{"OverflowingHeaderSize", errors.New("overflowing header size"), true},
		{"OverflowpackingA", errors.New("overflow packing a"), true},
		{"ErrSig", dns.ErrSig, false},
	}

	for _, tc := range testCases {
		t.Run(tc.testname, func(t *testing.T) {
			result := shouldTruncateResponse(tc.err)
			if result != tc.expected {
				t.Errorf("For testname '%v', expected %v but got %v", tc.testname, tc.expected, result)
			}
		})
	}
}
