package test

import (
	"net"
	"testing"
)

// pickPort returns a free TCP port on 127.0.0.1 and closes the probe listener.
func pickPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("probe listen failed: %v", err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}
