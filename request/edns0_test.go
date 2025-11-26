package request

import (
	"testing"

	"github.com/miekg/dns"
)

func TestSupportedOptions(t *testing.T) {
	tests := []struct {
		name     string
		options  []dns.EDNS0
		expected int
	}{
		{
			name:     "empty options",
			options:  []dns.EDNS0{},
			expected: 0,
		},
		{
			name: "all supported options",
			options: []dns.EDNS0{
				&dns.EDNS0_NSID{},
				&dns.EDNS0_EXPIRE{},
				&dns.EDNS0_COOKIE{},
				&dns.EDNS0_TCP_KEEPALIVE{},
				&dns.EDNS0_PADDING{},
			},
			expected: 5,
		},
		{
			name: "mixed supported and unsupported options",
			options: []dns.EDNS0{
				&dns.EDNS0_NSID{},
				&dns.EDNS0_LOCAL{Code: 65001}, // unsupported code
				&dns.EDNS0_PADDING{},
			},
			expected: 2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := supportedOptions(tc.options)
			if len(result) != tc.expected {
				t.Errorf("Expected %d supported options, got %d", tc.expected, len(result))
			}
		})
	}
}
