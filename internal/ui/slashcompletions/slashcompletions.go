package slashcompletions

import (
	"slices"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/ordered"
	"github.com/megacli/megacli/internal/ui/list"
)

const (
	minHeight = 1
	maxHeight = 10
	minWidth  = 10
	maxWidth  = 100
)


// SlashCompletions represents the slash command completions popup.
type SlashCompletions struct {
	width  int
	height int

	open  bool
	query string

	keyMap KeyMap
	list   *list.FilterableList

	normalStyle  lipgloss.Style
	focusedStyle lipgloss.Style
	matchStyle   lipgloss.Style

	allItems []list.FilterableItem
	filtered []list.FilterableItem
}

// New creates a new slash completions component.
func New(normalStyle, focusedStyle, matchStyle lipgloss.Style) *SlashCompletions {
	l := list.NewFilterableList()
	l.SetGap(0)
	l.SetReverse(true)

	return &SlashCompletions{
		keyMap:       DefaultKeyMap(),
		list:         l,
		normalStyle:  normalStyle,
		focusedStyle: focusedStyle,
		matchStyle:   matchStyle,
	}
}

// SetStyles updates the styles used when rendering items.
func (sc *SlashCompletions) SetStyles(normalStyle, focusedStyle, matchStyle lipgloss.Style) {
	sc.normalStyle = normalStyle
	sc.focusedStyle = focusedStyle
	sc.matchStyle = matchStyle
}

// IsOpen returns whether the popup is open.
func (sc *SlashCompletions) IsOpen() bool {
	return sc.open
}

// Size returns the visible size of the popup.
func (sc *SlashCompletions) Size() (width, height int) {
	visible := len(sc.filtered)
	return sc.width, min(visible, sc.height)
}

// Open opens the slash completions with the given items.
func (sc *SlashCompletions) Open(items []*SlashItem) {
	fitems := make([]list.FilterableItem, 0, len(items))
	for _, item := range items {
		fitems = append(fitems, item)
	}

	sc.open = true
	sc.query = ""
	sc.allItems = fitems
	sc.filtered = append([]list.FilterableItem(nil), fitems...)
	sc.list.SetItems(sc.filtered...)
	sc.list.SetFilter("")
	sc.list.Focus()

	sc.width = maxWidth
	sc.height = ordered.Clamp(len(fitems), int(minHeight), int(maxHeight))
	sc.list.SetSize(sc.width, sc.height)
	sc.list.SelectFirst()
	sc.list.ScrollToSelected()

	sc.updateSize()
}

// Close closes the popup.
func (sc *SlashCompletions) Close() {
	sc.open = false
}

// Filter filters the items by query.
func (sc *SlashCompletions) Filter(query string) {
	if !sc.open {
		return
	}

	if query == sc.query {
		return
	}

	sc.query = query
	sc.applyFilter(query)
	sc.updateSize()
}

func (sc *SlashCompletions) applyFilter(query string) {
	if query == "" {
		sc.filtered = append([]list.FilterableItem(nil), sc.allItems...)
		sc.list.SetItems(sc.filtered...)
		sc.list.SelectFirst()
		sc.list.ScrollToSelected()
		return
	}

	sc.list.SetItems(sc.allItems...)
	sc.list.SetFilter(query)
	raw := sc.list.FilteredItems()
	filtered := make([]list.FilterableItem, 0, len(raw))
	for _, item := range raw {
		filterable, ok := item.(list.FilterableItem)
		if !ok {
			continue
		}
		filtered = append(filtered, filterable)
	}

	queryLower := strings.ToLower(strings.TrimSpace(query))
	slices.SortStableFunc(filtered, func(a, b list.FilterableItem) int {
		return priorityTier(a.Filter(), queryLower) - priorityTier(b.Filter(), queryLower)
	})
	sc.filtered = filtered
	sc.list.SetItems(sc.filtered...)
	sc.list.SelectFirst()
	sc.list.ScrollToSelected()
}

func priorityTier(text, queryLower string) int {
	textLower := strings.ToLower(text)
	if textLower == queryLower {
		return 0
	}
	if strings.HasPrefix(textLower, queryLower) {
		return 1
	}
	return 2
}

func (sc *SlashCompletions) updateSize() {
	items := sc.filtered
	width := 0
	for _, item := range items {
		s := item.(interface{ Text() string }).Text()
		width = max(width, ansi.StringWidth(s))
	}
	sc.width = ordered.Clamp(width+2, int(minWidth), int(maxWidth))
	sc.height = ordered.Clamp(len(items), int(minHeight), int(maxHeight))
	sc.list.SetSize(sc.width, sc.height)
}

// HasItems returns whether there are visible items.
func (sc *SlashCompletions) HasItems() bool {
	return len(sc.filtered) > 0
}

// KeyMap returns the key bindings for external matching.
func (sc *SlashCompletions) KeyMap() KeyMap {
	return sc.keyMap
}

// Select selects the current item and returns its action, or nil if nothing
// is selected.
func (sc *SlashCompletions) Select() any {
	if !sc.open || len(sc.filtered) == 0 {
		return nil
	}

	selected := sc.list.Selected()
	if selected < 0 || selected >= len(sc.filtered) {
		selected = 0
	}

	item, ok := sc.filtered[selected].(*SlashItem)
	if !ok {
		return nil
	}

	sc.open = false
	return item.Action()
}

// MoveUp moves the selection up.
func (sc *SlashCompletions) MoveUp() {
	sc.selectPrev()
}

// MoveDown moves the selection down.
func (sc *SlashCompletions) MoveDown() {
	sc.selectNext()
}

func (sc *SlashCompletions) selectPrev() {
	if len(sc.filtered) == 0 {
		return
	}
	if !sc.list.SelectPrev() {
		sc.list.WrapToEnd()
	}
	sc.list.ScrollToSelected()
}

func (sc *SlashCompletions) selectNext() {
	if len(sc.filtered) == 0 {
		return
	}
	if !sc.list.SelectNext() {
		sc.list.WrapToStart()
	}
	sc.list.ScrollToSelected()
}


// Render renders the completions popup.
func (sc *SlashCompletions) Render() string {
	if !sc.open {
		return ""
	}

	if len(sc.filtered) == 0 {
		return ""
	}

	return sc.list.List.Render()
}
