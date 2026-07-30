package config

import "testing"

func TestApplyTLSEnvOverridesDashboardListener(t *testing.T) {
	t.Setenv("MCPPROXY_DASHBOARD_TLS_ENABLED", "true")
	t.Setenv("MCPPROXY_DASHBOARD_TLS_LISTEN", "0.0.0.0:8443")
	t.Setenv("MCPPROXY_DASHBOARD_TLS_CERT_FILE", "/tls/tls.crt")
	t.Setenv("MCPPROXY_DASHBOARD_TLS_KEY_FILE", "/tls/tls.key")
	t.Setenv("MCPPROXY_DASHBOARD_TLS_CLIENT_CA_FILE", "/tls/ca.crt")

	cfg := &Config{}
	applyTLSEnvOverrides(cfg)

	if cfg.DashboardTLS == nil {
		t.Fatal("DashboardTLS is nil")
	}
	if !cfg.DashboardTLS.Enabled ||
		cfg.DashboardTLS.Listen != "0.0.0.0:8443" ||
		cfg.DashboardTLS.CertFile != "/tls/tls.crt" ||
		cfg.DashboardTLS.KeyFile != "/tls/tls.key" ||
		cfg.DashboardTLS.ClientCAFile != "/tls/ca.crt" {
		t.Fatalf("unexpected dashboard TLS config: %#v", cfg.DashboardTLS)
	}
}
