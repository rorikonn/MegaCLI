package slashcompletions

import (
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/megacli/megacli/internal/ui/list"
	"github.com/rivo/uniseg"
	"github.com/sahilm/fuzzy"
)

// SlashItem represents a selectable item in the slash completions list.
type SlashItem struct {
	text    string
	action  any
	match   fuzzy.Match
	focused bool
	cache   map[int]string

	normalStyle  lipgloss.Style
	focusedStyle lipgloss.Style
	matchStyle   lipgloss.Style
}

// NewSlashItem creates a new slash completion item.
func NewSlashItem(text string, action any, normalStyle, focusedStyle, matchStyle lipgloss.Style) *SlashItem {
	return &SlashItem{
		text:         text,
		action:       action,
		normalStyle:  normalStyle,
		focusedStyle: focusedStyle,
		matchStyle:   matchStyle,
	}
}

// Text returns the display text.
func (s *SlashItem) Text() string {
	return s.text
}

// Action returns the action to dispatch on selection.
func (s *SlashItem) Action() any {
	return s.action
}

// Filter implements [list.FilterableItem].
func (s *SlashItem) Filter() string {
	return s.text
}

// SetMatch implements [list.MatchSettable].
func (s *SlashItem) SetMatch(m fuzzy.Match) {
	s.cache = nil
	s.match = m
}

// SetFocused implements [list.Focusable].
func (s *SlashItem) SetFocused(focused bool) {
	if s.focused != focused {
		s.cache = nil
	}
	s.focused = focused
}

// Render implements [list.Item].
func (s *SlashItem) Render(width int) string {
	if s.cache == nil {
		s.cache = make(map[int]string)
	}

	cached, ok := s.cache[width]
	if ok {
		return cached
	}

	text := s.text
	innerWidth := width - 2
	if ansi.StringWidth(text) > innerWidth {
		text = ansi.Truncate(text, innerWidth, "…")
	}

	style := s.normalStyle
	mStyle := s.matchStyle.Background(style.GetBackground())
	if s.focused {
		style = s.focusedStyle
		mStyle = s.matchStyle.Background(style.GetBackground())
	}

	content := style.Padding(0, 1).Width(width).Render(text)

	if len(s.match.MatchedIndexes) > 0 {
		var ranges []lipgloss.Range
		for _, rng := range matchedRanges(s.match.MatchedIndexes) {
			start, stop := bytePosToVisibleCharPos(text, rng)
			ranges = append(ranges, lipgloss.NewRange(start+1, stop+2, mStyle))
		}
		content = lipgloss.StyleRanges(content, ranges...)
	}

	s.cache[width] = content
	return content
}

// matchedRanges converts a list of match indexes into contiguous ranges.
func matchedRanges(in []int) [][2]int {
	if len(in) == 0 {
		return [][2]int{}
	}
	current := [2]int{in[0], in[0]}
	if len(in) == 1 {
		return [][2]int{current}
	}
	var out [][2]int
	for i := 1; i < len(in); i++ {
		if in[i] == current[1]+1 {
			current[1] = in[i]
		} else {
			out = append(out, current)
			current = [2]int{in[i], in[i]}
		}
	}
	out = append(out, current)
	return out
}

// bytePosToVisibleCharPos converts byte positions to visible character
// positions.
func bytePosToVisibleCharPos(str string, rng [2]int) (int, int) {
	bytePos, byteStart, byteStop := 0, rng[0], rng[1]
	pos, start, stop := 0, 0, 0
	gr := uniseg.NewGraphemes(str)
	for byteStart > bytePos {
		if !gr.Next() {
			break
		}
		bytePos += len(gr.Str())
		pos += max(1, gr.Width())
	}
	start = pos
	for byteStop > bytePos {
		if !gr.Next() {
			break
		}
		bytePos += len(gr.Str())
		pos += max(1, gr.Width())
	}
	stop = pos
	return start, stop
}

// Ensure SlashItem implements the required interfaces.
var (
	_ list.Item           = (*SlashItem)(nil)
	_ list.FilterableItem = (*SlashItem)(nil)
	_ list.MatchSettable  = (*SlashItem)(nil)
	_ list.Focusable      = (*SlashItem)(nil)
)
