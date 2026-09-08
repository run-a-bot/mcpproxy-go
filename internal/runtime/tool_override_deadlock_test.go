package runtime

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/smart-mcp-proxy/mcpproxy-go/internal/config"
)

func TestToolOverrides_DeadlockRegression(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "mcp_config.json")

	initialCfg := config.DefaultConfig()
	initialCfg.Listen = "127.0.0.1:0"
	initialCfg.DataDir = tmpDir
	initialCfg.Servers = []*config.ServerConfig{
		{
			Name:     "git.runabot.de",
			Command:  "echo",
			Protocol: "stdio",
			Enabled:  true,
		},
	}
	require.NoError(t, config.SaveConfig(initialCfg, cfgPath))

	rt, err := New(initialCfg, cfgPath, zap.NewNop())
	require.NoError(t, err)
	defer func() { _ = rt.Close() }()

	require.NoError(t, rt.storageManager.SaveUpstreamServer(initialCfg.Servers[0]))

	// Calling SetToolOverrides previously deadlocked because SetToolOverrides held r.mu.Lock()
	// and called lookupServerConfigForRestart, which attempted r.mu.RLock().
	readOnly := true
	hints := &config.ToolAnnotations{ReadOnlyHint: &readOnly}
	err = rt.SetToolOverrides("git.runabot.de", []string{"search_repos"}, "Search git repositories", hints)
	require.NoError(t, err)

	// Verify persistence in storage
	stored, err := rt.storageManager.GetUpstreamServer("git.runabot.de")
	require.NoError(t, err)
	require.NotNil(t, stored.ToolOverrides)
	override, ok := stored.ToolOverrides["search_repos"]
	require.True(t, ok)
	assert.Equal(t, "Search git repositories", override.Description)
	require.NotNil(t, override.Annotations)
	require.NotNil(t, override.Annotations.ReadOnlyHint)
	assert.True(t, *override.Annotations.ReadOnlyHint)

	// Calling ResetToolOverrides also previously deadlocked
	err = rt.ResetToolOverrides("git.runabot.de", []string{"search_repos"})
	require.NoError(t, err)

	storedAfterReset, err := rt.storageManager.GetUpstreamServer("git.runabot.de")
	require.NoError(t, err)
	assert.Nil(t, storedAfterReset.ToolOverrides)
}
