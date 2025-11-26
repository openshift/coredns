package request

import (
	"fmt"
	"testing"

	"github.com/coredns/coredns/plugin/test"

	"github.com/miekg/dns"
)

// mockResponseWriter implements dns.ResponseWriter interface for testing
type mockResponseWriter struct {
	test.ResponseWriter
	lastMsg *dns.Msg
}

func (m *mockResponseWriter) WriteMsg(msg *dns.Msg) error {
	m.lastMsg = msg
	return nil
}

func TestScrubWriter(t *testing.T) {
	req := new(dns.Msg)
	req.SetQuestion("example.com.", dns.TypeA)
	req.SetEdns0(4096, true)

	mock := &mockResponseWriter{}
	sw := NewScrubWriter(req, mock)

	// Create a large response message
	resp := new(dns.Msg)
	resp.SetReply(req)

	// Add a lot of records to make it large
	for i := 1; i < 100; i++ {
		resp.Answer = append(resp.Answer, test.A(
			fmt.Sprintf("example.com. 10 IN A 10.0.0.%d", i)))
	}

	// Write the message through ScrubWriter
	err := sw.WriteMsg(resp)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Verify that ScrubWriter called methods properly
	if mock.lastMsg == nil {
		t.Fatalf("Expected WriteMsg to be called with a message")
	}
}
