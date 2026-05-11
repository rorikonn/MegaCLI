package tools

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEmbeddedSubOptionsRe(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		match   bool
	}{
		{
			name:    "lettered sub-option A)",
			content: "Some question\n  A) First choice\n  B) Second choice",
			match:   true,
		},
		{
			name:    "dash prefixed sub-option",
			content: "Question text\n  - A) Option one\n  - B) Option two",
			match:   true,
		},
		{
			name:    "no sub-options",
			content: "Which approach do you prefer?",
			match:   false,
		},
		{
			name:    "lowercase a) not matched",
			content: "This has a) lowercase which is fine",
			match:   false,
		},
		{
			name:    "A) at line start",
			content: "A) Direct start",
			match:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.match, embeddedSubOptionsRe.MatchString(tt.content))
		})
	}
}

func TestCombinedOptionLabelRe(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		label string
		match bool
	}{
		{name: "typical combined", label: "1A, 2A, 3A", match: true},
		{name: "two items", label: "1A, 2B", match: true},
		{name: "no spaces", label: "1A,2B,3C", match: true},
		{name: "single label", label: "1A", match: false},
		{name: "descriptive option", label: "Use caching for speed", match: false},
		{name: "number only", label: "1, 2, 3", match: false},
		{name: "other text", label: "其他组合（请说明）", match: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.match, combinedOptionLabelRe.MatchString(tt.label))
		})
	}
}

func TestHasCombinedOptionLabels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		options []string
		result  bool
	}{
		{
			name:    "all combined labels",
			options: []string{"1A, 2A, 3A", "1A, 2B, 3A", "1B, 2A, 3B"},
			result:  true,
		},
		{
			name:    "majority combined",
			options: []string{"1A, 2A", "1B, 2B", "其他组合"},
			result:  true,
		},
		{
			name:    "no combined labels",
			options: []string{"Use caching", "Use streaming", "Use polling"},
			result:  false,
		},
		{
			name:    "empty options",
			options: []string{},
			result:  false,
		},
		{
			name:    "minority combined",
			options: []string{"Good option", "Another option", "1A, 2B"},
			result:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.result, hasCombinedOptionLabels(tt.options))
		})
	}
}
