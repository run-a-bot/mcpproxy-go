package server

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smart-mcp-proxy/mcpproxy-go/internal/config"
)

func TestUpdateServerPersistsOAuthClientCredentials(t *testing.T) {
	srv := newConsistencyServer(t)
	seedOAuthUpdateServer(t, srv, &config.ServerConfig{
		Name:     "github",
		URL:      "https://example.com/mcp",
		Protocol: "streamable-http",
		Enabled:  false,
	})

	require.NoError(t, srv.UpdateServer(context.Background(), "github", &config.ServerConfig{
		OAuth: &config.OAuthConfig{
			ClientID:     "client-id",
			ClientSecret: "client-secret",
		},
	}))

	stored, err := srv.runtime.StorageManager().GetUpstreamServer("github")
	require.NoError(t, err)
	require.NotNil(t, stored.OAuth)
	assert.Equal(t, "client-id", stored.OAuth.ClientID)
	assert.Equal(t, "client-secret", stored.OAuth.ClientSecret)

	runtimeConfig := persistedServer(t, srv, "github")
	require.NotNil(t, runtimeConfig.OAuth)
	assert.Equal(t, "client-id", runtimeConfig.OAuth.ClientID)
	assert.Equal(t, "client-secret", runtimeConfig.OAuth.ClientSecret)
}

func TestUpdateServerOAuthPreservesStoredSecret(t *testing.T) {
	srv := newConsistencyServer(t)
	seedOAuthUpdateServer(t, srv, &config.ServerConfig{
		Name:     "github",
		URL:      "https://example.com/mcp",
		Protocol: "streamable-http",
		Enabled:  false,
		OAuth: &config.OAuthConfig{
			ClientID:     "old-client-id",
			ClientSecret: "stored-secret",
		},
	})

	require.NoError(t, srv.UpdateServer(context.Background(), "github", &config.ServerConfig{
		OAuth: &config.OAuthConfig{ClientID: "new-client-id"},
	}))

	stored, err := srv.runtime.StorageManager().GetUpstreamServer("github")
	require.NoError(t, err)
	require.NotNil(t, stored.OAuth)
	assert.Equal(t, "new-client-id", stored.OAuth.ClientID)
	assert.Equal(t, "stored-secret", stored.OAuth.ClientSecret)
}

func seedOAuthUpdateServer(t *testing.T, srv *Server, serverConfig *config.ServerConfig) {
	t.Helper()
	require.NoError(t, srv.runtime.StorageManager().SaveUpstreamServer(serverConfig))

	current := srv.runtime.Config()
	require.NotNil(t, current)
	updated := *current
	updated.Servers = append(append([]*config.ServerConfig(nil), current.Servers...), serverConfig)
	srv.runtime.UpdateConfig(&updated, "")
}
