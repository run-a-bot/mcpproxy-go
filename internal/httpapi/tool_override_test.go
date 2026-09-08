package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/smart-mcp-proxy/mcpproxy-go/internal/config"
	"github.com/smart-mcp-proxy/mcpproxy-go/internal/contracts"
)

type toolOverrideMockController struct {
	MockServerController
	lastServerName  string
	lastTools       []string
	lastDescription string
	lastHints       *config.ToolAnnotations
	overrideErr     error
	resetErr        error
}

func (m *toolOverrideMockController) SetToolOverrides(serverName string, tools []string, description string, hints *config.ToolAnnotations) error {
	m.lastServerName = serverName
	m.lastTools = tools
	m.lastDescription = description
	m.lastHints = hints
	return m.overrideErr
}

func (m *toolOverrideMockController) ResetToolOverrides(serverName string, tools []string) error {
	m.lastServerName = serverName
	m.lastTools = tools
	return m.resetErr
}

func TestHandleOverrideTools(t *testing.T) {
	logger := zaptest.NewLogger(t).Sugar()
	ctrl := &toolOverrideMockController{}
	srv := NewServer(ctrl, logger, nil)

	t.Run("successful override with hints and description", func(t *testing.T) {
		readOnly := true
		payload := contracts.OverrideToolsRequest{
			ServerName:  "github",
			Tools:       []string{"search_issues", "get_issue"},
			Description: "Custom search",
			Annotations: &config.ToolAnnotations{
				ReadOnlyHint: &readOnly,
			},
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/api/v1/tools/override", bytes.NewReader(body))
		req.Header.Set("X-Request-Source", "socket")
		w := httptest.NewRecorder()

		srv.router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "github", ctrl.lastServerName)
		assert.Equal(t, []string{"search_issues", "get_issue"}, ctrl.lastTools)
		assert.Equal(t, "Custom search", ctrl.lastDescription)
		require.NotNil(t, ctrl.lastHints)
		assert.Equal(t, &readOnly, ctrl.lastHints.ReadOnlyHint)
	})

	t.Run("missing server_name fails with 400", func(t *testing.T) {
		payload := contracts.OverrideToolsRequest{
			Tools: []string{"search_issues"},
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/api/v1/tools/override", bytes.NewReader(body))
		req.Header.Set("X-Request-Source", "socket")
		w := httptest.NewRecorder()

		srv.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("empty tools fails with 400", func(t *testing.T) {
		payload := contracts.OverrideToolsRequest{
			ServerName: "github",
			Tools:      []string{},
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/api/v1/tools/override", bytes.NewReader(body))
		req.Header.Set("X-Request-Source", "socket")
		w := httptest.NewRecorder()

		srv.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("controller error returns 500", func(t *testing.T) {
		ctrl.overrideErr = errors.New("storage failure")
		defer func() { ctrl.overrideErr = nil }()

		payload := contracts.OverrideToolsRequest{
			ServerName: "github",
			Tools:      []string{"search_issues"},
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/api/v1/tools/override", bytes.NewReader(body))
		req.Header.Set("X-Request-Source", "socket")
		w := httptest.NewRecorder()

		srv.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestHandleResetToolOverrides(t *testing.T) {
	logger := zaptest.NewLogger(t).Sugar()
	ctrl := &toolOverrideMockController{}
	srv := NewServer(ctrl, logger, nil)

	t.Run("successful reset", func(t *testing.T) {
		payload := contracts.ResetToolOverridesRequest{
			ServerName: "jira",
			Tools:      []string{"create_issue"},
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/api/v1/tools/reset-override", bytes.NewReader(body))
		req.Header.Set("X-Request-Source", "socket")
		w := httptest.NewRecorder()

		srv.router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "jira", ctrl.lastServerName)
		assert.Equal(t, []string{"create_issue"}, ctrl.lastTools)
	})

	t.Run("missing server_name fails with 400", func(t *testing.T) {
		payload := contracts.ResetToolOverridesRequest{
			Tools: []string{"create_issue"},
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/api/v1/tools/reset-override", bytes.NewReader(body))
		req.Header.Set("X-Request-Source", "socket")
		w := httptest.NewRecorder()

		srv.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("empty tools fails with 400", func(t *testing.T) {
		payload := contracts.ResetToolOverridesRequest{
			ServerName: "jira",
			Tools:      []string{},
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/api/v1/tools/reset-override", bytes.NewReader(body))
		req.Header.Set("X-Request-Source", "socket")
		w := httptest.NewRecorder()

		srv.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("controller error returns 500", func(t *testing.T) {
		ctrl.resetErr = errors.New("reset failure")
		defer func() { ctrl.resetErr = nil }()

		payload := contracts.ResetToolOverridesRequest{
			ServerName: "jira",
			Tools:      []string{"create_issue"},
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/api/v1/tools/reset-override", bytes.NewReader(body))
		req.Header.Set("X-Request-Source", "socket")
		w := httptest.NewRecorder()

		srv.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestConvertGenericToolsToTyped_PreservesOriginalDescription(t *testing.T) {
	generic := []map[string]interface{}{
		{
			"name":                 "search_repos",
			"server_name":          "github",
			"description":          "Custom description",
			"original_description": "Upstream original description",
			"usage":                5,
		},
	}

	typed := contracts.ConvertGenericToolsToTyped(generic)
	require.Len(t, typed, 1)
	assert.Equal(t, "search_repos", typed[0].Name)
	assert.Equal(t, "Custom description", typed[0].Description)
	assert.Equal(t, "Upstream original description", typed[0].OriginalDescription)
}
