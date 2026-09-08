package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveToolOverride(t *testing.T) {
	readOnlyTrue := true
	readOnlyFalse := false
	destrTrue := true

	sc := &ServerConfig{
		ToolOverrides: map[string]*ToolOverride{
			"search_issues": {
				Description: "Custom search description",
				Annotations: &ToolAnnotations{
					ReadOnlyHint: &readOnlyTrue,
				},
			},
			"delete_all": {
				Annotations: &ToolAnnotations{
					DestructiveHint: &destrTrue,
					ReadOnlyHint:    &readOnlyFalse,
				},
			},
		},
	}

	t.Run("overridden description and annotation", func(t *testing.T) {
		desc, ann, isCustom := sc.ResolveToolOverride("search_issues", "Original desc", nil)
		assert.True(t, isCustom)
		assert.Equal(t, "Custom search description", desc)
		assert.NotNil(t, ann)
		assert.Equal(t, &readOnlyTrue, ann.ReadOnlyHint)
	})

	t.Run("overridden annotations only preserves original description", func(t *testing.T) {
		desc, ann, isCustom := sc.ResolveToolOverride("delete_all", "Original delete desc", nil)
		assert.True(t, isCustom)
		assert.Equal(t, "Original delete desc", desc)
		assert.NotNil(t, ann)
		assert.Equal(t, &destrTrue, ann.DestructiveHint)
		assert.Equal(t, &readOnlyFalse, ann.ReadOnlyHint)
	})

	t.Run("non-overridden tool returns upstream defaults", func(t *testing.T) {
		upstreamAnn := &ToolAnnotations{ReadOnlyHint: &readOnlyTrue}
		desc, ann, isCustom := sc.ResolveToolOverride("unrelated_tool", "Upstream desc", upstreamAnn)
		assert.False(t, isCustom)
		assert.Equal(t, "Upstream desc", desc)
		assert.Equal(t, upstreamAnn, ann)
	})

	t.Run("nil server config or nil overrides", func(t *testing.T) {
		var nilSc *ServerConfig
		desc, ann, isCustom := nilSc.ResolveToolOverride("any", "Original", nil)
		assert.False(t, isCustom)
		assert.Equal(t, "Original", desc)
		assert.Nil(t, ann)
	})
}
