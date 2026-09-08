package core

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/mark3labs/mcp-go/server/servertest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smart-mcp-proxy/mcpproxy-go/internal/config"
)

func newTestToolsUpstream(t *testing.T) *mcpserver.MCPServer {
	t.Helper()
	opts := []mcpserver.ServerOption{mcpserver.WithToolCapabilities(true)}
	srv := mcpserver.NewMCPServer("test-upstream-tools", "0.0.1", opts...)
	srv.AddTool(
		mcp.NewTool("echo_tool",
			mcp.WithDescription("Original upstream description"),
			mcp.WithString("message", mcp.Required(), mcp.Description("Message to echo")),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return mcp.NewToolResultText("echo"), nil
		},
	)
	return srv
}

func TestClient_ListTools_ToolOverridesAndHash(t *testing.T) {
	upstream := newTestToolsUpstream(t)
	testServer := servertest.NewTestStreamableHTTPServer(upstream)
	defer testServer.Close()

	c := connectedTestClient(t, testServer.URL, nil)

	// Initial tools without overrides
	tools, err := c.ListTools(context.Background())
	require.NoError(t, err)
	require.Len(t, tools, 1)
	origHash := tools[0].Hash
	assert.Equal(t, "echo_tool", tools[0].Name)
	assert.Equal(t, "Original upstream description", tools[0].Description)
	assert.Empty(t, tools[0].OriginalDescription)

	// Apply tool override via SetToolOverrides
	readOnly := true
	overrides := map[string]*config.ToolOverride{
		"echo_tool": {
			Description: "Overridden description",
			Annotations: &config.ToolAnnotations{
				ReadOnlyHint: &readOnly,
			},
		},
	}
	c.SetToolOverrides(overrides)
	// Published override state must be immutable: callers retain their map.
	overrides["echo_tool"].Description = "Mutated after publish"
	readOnly = false

	toolsOverridden, err := c.ListTools(context.Background())
	require.NoError(t, err)
	require.Len(t, toolsOverridden, 1)
	assert.Equal(t, "Overridden description", toolsOverridden[0].Description)
	assert.Equal(t, "Original upstream description", toolsOverridden[0].OriginalDescription)
	require.NotNil(t, toolsOverridden[0].Annotations)
	require.NotNil(t, toolsOverridden[0].Annotations.ReadOnlyHint)
	assert.True(t, *toolsOverridden[0].Annotations.ReadOnlyHint)
	assert.NotEqual(t, origHash, toolsOverridden[0].Hash, "Hash must change when description or annotations are overridden")

	// Reset tool override
	c.SetToolOverrides(nil)
	toolsReset, err := c.ListTools(context.Background())
	require.NoError(t, err)
	require.Len(t, toolsReset, 1)
	assert.Equal(t, "Original upstream description", toolsReset[0].Description)
	assert.Empty(t, toolsReset[0].OriginalDescription)
	assert.Equal(t, origHash, toolsReset[0].Hash, "Hash should return to original after reset")
}

func TestClientSetToolOverridesSnapshotsInput(t *testing.T) {
	readOnly := true
	overrides := map[string]*config.ToolOverride{
		"tool": {
			Description: "published description",
			Annotations: &config.ToolAnnotations{
				ReadOnlyHint: &readOnly,
			},
		},
	}
	c := &Client{}
	c.SetToolOverrides(overrides)

	// The caller still owns its input and may reuse or mutate it after publish.
	overrides["tool"].Description = "mutated description"
	readOnly = false

	desc, annotations, custom := resolveToolOverride(c.toolOverrides.Load(), "tool", "upstream", nil)
	assert.True(t, custom)
	assert.Equal(t, "published description", desc)
	require.NotNil(t, annotations)
	require.NotNil(t, annotations.ReadOnlyHint)
	assert.True(t, *annotations.ReadOnlyHint)
}
