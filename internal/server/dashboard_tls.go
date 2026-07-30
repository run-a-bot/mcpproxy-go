package server

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"go.uber.org/zap"

	"github.com/smart-mcp-proxy/mcpproxy-go/internal/config"
)

func newDashboardMTLSServer(
	cfg *config.DashboardTLSConfig,
	handler http.Handler,
	logger *zap.Logger,
) (*http.Server, net.Listener, error) {
	if cfg == nil || !cfg.Enabled {
		return nil, nil, nil
	}

	certificate, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
	if err != nil {
		return nil, nil, fmt.Errorf("load dashboard TLS certificate: %w", err)
	}
	clientCAPEM, err := os.ReadFile(cfg.ClientCAFile)
	if err != nil {
		return nil, nil, fmt.Errorf("read dashboard client CA: %w", err)
	}
	clientCAs := x509.NewCertPool()
	if !clientCAs.AppendCertsFromPEM(clientCAPEM) {
		return nil, nil, fmt.Errorf("parse dashboard client CA %q", cfg.ClientCAFile)
	}

	rawListener, err := net.Listen("tcp", cfg.Listen)
	if err != nil {
		return nil, nil, fmt.Errorf("listen for dashboard mTLS on %s: %w", cfg.Listen, err)
	}

	tlsConfig := &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{certificate},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    clientCAs,
		NextProtos:   []string{"h2", "http/1.1"},
	}
	server := &http.Server{
		Addr:              cfg.Listen,
		Handler:           handler,
		ReadHeaderTimeout: 60 * time.Second,
		ReadTimeout:       120 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       180 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	logger.Info("Dashboard mTLS listener configured",
		zap.String("address", rawListener.Addr().String()),
		zap.String("client_ca_file", cfg.ClientCAFile))
	return server, tls.NewListener(rawListener, tlsConfig), nil
}
