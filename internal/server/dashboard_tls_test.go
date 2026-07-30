package server

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/smart-mcp-proxy/mcpproxy-go/internal/config"
)

func TestDashboardMTLSServerRequiresTrustedClientCertificate(t *testing.T) {
	certFile, keyFile, caFile, clientCertificate, roots := dashboardTLSFixture(t)
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	server, listener, err := newDashboardMTLSServer(&config.DashboardTLSConfig{
		Enabled:      true,
		Listen:       "127.0.0.1:0",
		CertFile:     certFile,
		KeyFile:      keyFile,
		ClientCAFile: caFile,
	}, handler, zap.NewNop())
	if err != nil {
		t.Fatalf("newDashboardMTLSServer() error = %v", err)
	}
	t.Cleanup(func() { _ = server.Close() })
	go func() { _ = server.Serve(listener) }()

	address := "https://" + listener.Addr().String()
	withoutCertificate := &http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12},
	}}
	if _, err := withoutCertificate.Get(address); err == nil {
		t.Fatal("request without client certificate unexpectedly succeeded")
	}

	withCertificate := &http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{
			RootCAs:      roots,
			Certificates: []tls.Certificate{clientCertificate},
			MinVersion:   tls.VersionTLS12,
		},
	}}
	response, err := withCertificate.Get(address)
	if err != nil {
		t.Fatalf("request with trusted client certificate failed: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusNoContent)
	}
}

func dashboardTLSFixture(t *testing.T) (string, string, string, tls.Certificate, *x509.CertPool) {
	t.Helper()
	caCertificate, caKey, caPEM := issueDashboardCertificate(t, nil, nil, true, false)
	_, serverKey, serverPEM := issueDashboardCertificate(t, caCertificate, caKey, false, false)
	_, clientKey, clientPEM := issueDashboardCertificate(t, caCertificate, caKey, false, true)

	dir := t.TempDir()
	certFile := filepath.Join(dir, "tls.crt")
	keyFile := filepath.Join(dir, "tls.key")
	caFile := filepath.Join(dir, "ca.crt")
	writeDashboardTLSFile(t, certFile, serverPEM)
	writeDashboardTLSFile(t, keyFile, pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: mustMarshalECPrivateKey(t, serverKey),
	}))
	writeDashboardTLSFile(t, caFile, caPEM)

	clientCertificate, err := tls.X509KeyPair(clientPEM, pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: mustMarshalECPrivateKey(t, clientKey),
	}))
	if err != nil {
		t.Fatalf("parse client key pair: %v", err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caPEM) {
		t.Fatal("append test CA")
	}
	return certFile, keyFile, caFile, clientCertificate, roots
}

func issueDashboardCertificate(
	t *testing.T,
	ca *x509.Certificate,
	caKey *ecdsa.PrivateKey,
	isCA bool,
	isClient bool,
) (*x509.Certificate, *ecdsa.PrivateKey, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               pkix.Name{CommonName: "dashboard-mtls-test"},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(time.Hour),
		BasicConstraintsValid: true,
		IsCA:                  isCA,
		KeyUsage:              x509.KeyUsageDigitalSignature,
	}
	if isCA {
		template.KeyUsage |= x509.KeyUsageCertSign
		ca = template
		caKey = key
	} else if isClient {
		template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	} else {
		template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
		template.IPAddresses = []net.IP{net.ParseIP("127.0.0.1")}
	}
	der, err := x509.CreateCertificate(rand.Reader, template, ca, &key.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	certificate, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse certificate: %v", err)
	}
	return certificate, key, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func mustMarshalECPrivateKey(t *testing.T, key *ecdsa.PrivateKey) []byte {
	t.Helper()
	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshal private key: %v", err)
	}
	return der
}

func writeDashboardTLSFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
