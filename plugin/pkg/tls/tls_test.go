package tls

import (
	"crypto/tls"
	"path/filepath"
	"testing"

	"github.com/coredns/coredns/plugin/test"
)

func getPEMFiles(t *testing.T) (cert, key, ca string) {
	t.Helper()
	tempDir, err := test.WritePEMFiles(t)
	if err != nil {
		t.Fatalf("Could not write PEM files: %s", err)
	}

	cert = filepath.Join(tempDir, "cert.pem")
	key = filepath.Join(tempDir, "key.pem")
	ca = filepath.Join(tempDir, "ca.pem")

	return
}

func TestNewTLSConfig(t *testing.T) {
	cert, key, ca := getPEMFiles(t)
	_, err := NewTLSConfig(cert, key, ca)
	if err != nil {
		t.Errorf("Failed to create TLSConfig: %s", err)
	}
}

func TestNewTLSClientConfig(t *testing.T) {
	_, _, ca := getPEMFiles(t)

	_, err := NewTLSClientConfig(ca)
	if err != nil {
		t.Errorf("Failed to create TLSConfig: %s", err)
	}
}

func TestNewTLSConfigFromArgs(t *testing.T) {
	cert, key, ca := getPEMFiles(t)

	_, err := NewTLSConfigFromArgs()
	if err != nil {
		t.Errorf("Failed to create TLSConfig: %s", err)
	}

	c, err := NewTLSConfigFromArgs(ca)
	if err != nil {
		t.Errorf("Failed to create TLSConfig: %s", err)
	}
	if c.RootCAs == nil {
		t.Error("RootCAs should not be nil when one arg passed")
	}

	c, err = NewTLSConfigFromArgs(cert, key)
	if err != nil {
		t.Errorf("Failed to create TLSConfig: %s", err)
	}
	if c.RootCAs != nil {
		t.Error("RootCAs should be nil when two args passed")
	}
	if len(c.Certificates) != 1 {
		t.Error("Certificates should have a single entry when two args passed")
	}
	args := []string{cert, key, ca}
	c, err = NewTLSConfigFromArgs(args...)
	if err != nil {
		t.Errorf("Failed to create TLSConfig: %s", err)
	}
	if c.RootCAs == nil {
		t.Error("RootCAs should not be nil when three args passed")
	}
	if len(c.Certificates) != 1 {
		t.Error("Certificates should have a single entry when three args passed")
	}
}

func TestNewTLSConfigFromArgsWithRoot(t *testing.T) {
	cert, key, ca := getPEMFiles(t)
	tempDir := t.TempDir()

	root := tempDir
	args := []string{cert, key, ca}
	for i := range args {
		if !filepath.IsAbs(args[i]) && root != "" {
			args[i] = filepath.Join(root, args[i])
		}
	}
	c, err := NewTLSConfigFromArgs(args...)
	if err != nil {
		t.Errorf("Failed to create TLSConfig: %s", err)
	}
	if c.RootCAs == nil {
		t.Error("RootCAs should not be nil when three args passed")
	}
	if len(c.Certificates) != 1 {
		t.Error("Certificates should have a single entry when three args passed")
	}
}

func TestSetTLSDefaults(t *testing.T) {
	cert, key, ca := getPEMFiles(t)

	c, err := NewTLSConfig(cert, key, ca)
	if err != nil {
		t.Fatalf("Failed to create TLSConfig: %s", err)
	}

	if c.MinVersion != tls.VersionTLS12 {
		t.Errorf("Expected MinVersion to be TLS 1.2, got %d", c.MinVersion)
	}
	if c.MaxVersion != tls.VersionTLS13 {
		t.Errorf("Expected MaxVersion to be TLS 1.3, got %d", c.MaxVersion)
	}
	if len(c.CipherSuites) != 6 {
		t.Errorf("Expected 6 CipherSuites, got %d", len(c.CipherSuites))
	}

	expectedCurves := []tls.CurveID{
		tls.X25519MLKEM768,
		tls.X25519,
		tls.CurveP256,
		tls.CurveP384,
	}
	if len(c.CurvePreferences) != len(expectedCurves) {
		t.Fatalf("Expected %d CurvePreferences, got %d", len(expectedCurves), len(c.CurvePreferences))
	}
	for i, curve := range expectedCurves {
		if c.CurvePreferences[i] != curve {
			t.Errorf("CurvePreferences[%d] = %v, want %v", i, c.CurvePreferences[i], curve)
		}
	}
}

func TestNewHTTPSTransport(t *testing.T) {
	_, _, ca := getPEMFiles(t)

	cc, err := NewTLSClientConfig(ca)
	if err != nil {
		t.Errorf("Failed to create TLSConfig: %s", err)
	}

	tr := NewHTTPSTransport(cc)
	if tr == nil {
		t.Errorf("Failed to create https transport with cc")
	}

	tr = NewHTTPSTransport(nil)
	if tr == nil {
		t.Errorf("Failed to create https transport without cc")
	}
}
