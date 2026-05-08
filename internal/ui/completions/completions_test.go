package completions

import (
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/require"
)

func TestFilterPrefersExactBasenameStem(t *testing.T) {
	t.Parallel()

	c := New(lipgloss.NewStyle(), lipgloss.NewStyle(), lipgloss.NewStyle())
	c.SetItems([]SkillCompletionValue{
		{Name: "search", Path: "internal/ui/chat/search.go"},
		{Name: "user", Path: "internal/ui/chat/user.go"},
	}, nil)

	c.Filter("user")

	filtered := c.filtered
	require.NotEmpty(t, filtered)
	first, ok := filtered[0].(*CompletionItem)
	require.True(t, ok)
	require.Contains(t, first.Text(), "user")
	require.NotEmpty(t, first.match.MatchedIndexes)
}

func TestFilterPrefersBasenamePrefix(t *testing.T) {
	t.Parallel()

	c := New(lipgloss.NewStyle(), lipgloss.NewStyle(), lipgloss.NewStyle())
	c.SetItems([]SkillCompletionValue{
		{Name: "mcp-tool", Path: "internal/ui/chat/mcp.go"},
		{Name: "chat-helper", Path: "internal/ui/model/chat.go"},
	}, nil)

	c.Filter("chat")

	filtered := c.filtered
	require.NotEmpty(t, filtered)
	first, ok := filtered[0].(*CompletionItem)
	require.True(t, ok)
	require.Contains(t, first.Text(), "chat")
	require.NotEmpty(t, first.match.MatchedIndexes)
}

func TestNamePriorityTier(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		path     string
		query    string
		wantTier int
	}{
		{
			name:     "exact stem",
			path:     "internal/ui/chat/user.go",
			query:    "user",
			wantTier: tierExactName,
		},
		{
			name:     "basename prefix",
			path:     "internal/ui/model/chat.go",
			query:    "chat.g",
			wantTier: tierPrefixName,
		},
		{
			name:     "path segment exact",
			path:     "internal/ui/chat/mcp.go",
			query:    "chat",
			wantTier: tierPathSegment,
		},
		{
			name:     "fallback",
			path:     "internal/ui/chat/search.go",
			query:    "user",
			wantTier: tierFallback,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := namePriorityTier(tt.path, tt.query)
			require.Equal(t, tt.wantTier, got)
		})
	}
}

func TestFilterPrefersPathSegmentExact(t *testing.T) {
	t.Parallel()

	c := New(lipgloss.NewStyle(), lipgloss.NewStyle(), lipgloss.NewStyle())
	c.SetItems([]SkillCompletionValue{
		{Name: "xychat", Path: "internal/ui/model/xychat.go"},
		{Name: "mcp", Path: "internal/ui/chat/mcp.go"},
	}, nil)

	c.Filter("chat")

	filtered := c.filtered
	require.NotEmpty(t, filtered)
	first, ok := filtered[0].(*CompletionItem)
	require.True(t, ok)
	require.Contains(t, first.Text(), "chat")
}

func TestSetItemsIncludesAddFileItem(t *testing.T) {
	t.Parallel()

	c := New(lipgloss.NewStyle(), lipgloss.NewStyle(), lipgloss.NewStyle())
	c.SetItems([]SkillCompletionValue{
		{Name: "my-skill", Path: "/tmp/SKILL.md"},
	}, nil)

	require.True(t, c.HasItems())
	first, ok := c.allItems[0].(*CompletionItem)
	require.True(t, ok)
	require.Equal(t, "Add File ...", first.Text())
}
