// Package display provides a panel for rendering DisplayOnly tool output.
// Content shown here is visible to the user but NOT sent to the LLM.
package display

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/megacli/megacli/internal/megatool"
)

// Model manages the display panel state.
type Model struct {
	events   []megatool.DisplayEvent
	maxItems int
	width    int
	height   int
	scroll   int
	visible  bool
	styles   Styles
}

// Styles for the display panel.
type Styles struct {
	Border   lipgloss.Style
	Title    lipgloss.Style
	ToolName lipgloss.Style
	Content  lipgloss.Style
	Empty    lipgloss.Style
}

// DefaultStyles returns the default display panel styles.
func DefaultStyles() Styles {
	return Styles{
		Border: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("6")).
			Padding(0, 1),
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("14")).
			MarginBottom(1),
		ToolName: lipgloss.NewStyle().
			Foreground(lipgloss.Color("3")).
			Bold(true),
		Content: lipgloss.NewStyle().
			Foreground(lipgloss.Color("7")),
		Empty: lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")).
			Italic(true),
	}
}

// New creates a new display panel.
func New() Model {
	return Model{
		maxItems: 5,
		styles:   DefaultStyles(),
	}
}

// SetSize updates the panel dimensions.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// SetVisible controls panel visibility.
func (m *Model) SetVisible(v bool) {
	m.visible = v
}

// IsVisible returns whether the panel is visible.
func (m Model) IsVisible() bool {
	return m.visible
}

// Push adds a new display event to the panel and makes it visible.
func (m *Model) Push(event megatool.DisplayEvent) {
	m.events = append(m.events, event)
	if len(m.events) > m.maxItems {
		m.events = m.events[len(m.events)-m.maxItems:]
	}
	m.visible = true
	m.scroll = len(m.events) - 1
}

// Clear removes all events.
func (m *Model) Clear() {
	m.events = nil
	m.scroll = 0
}

// View renders the display panel.
func (m Model) View() string {
	if !m.visible || m.width < 10 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(m.styles.Title.Render("Display"))
	sb.WriteString("\n")

	if len(m.events) == 0 {
		sb.WriteString(m.styles.Empty.Render("  No content to display"))
	} else {
		idx := m.scroll
		if idx >= len(m.events) {
			idx = len(m.events) - 1
		}
		evt := m.events[idx]

		header := fmt.Sprintf("[%s] (%d/%d)", evt.ToolName, idx+1, len(m.events))
		sb.WriteString(m.styles.ToolName.Render(header))
		sb.WriteString("\n")

		contentLines := strings.Split(evt.Content, "\n")
		maxLines := m.height - 5
		if maxLines < 3 {
			maxLines = 3
		}
		if len(contentLines) > maxLines {
			contentLines = contentLines[:maxLines]
			contentLines = append(contentLines, fmt.Sprintf("... (%d more lines)", len(strings.Split(evt.Content, "\n"))-maxLines))
		}
		sb.WriteString(m.styles.Content.Render(strings.Join(contentLines, "\n")))
	}

	content := sb.String()
	return m.styles.Border.
		Width(m.width - 2).
		MaxHeight(m.height).
		Render(content)
}

// ScrollUp moves to the previous display event.
func (m *Model) ScrollUp() {
	if m.scroll > 0 {
		m.scroll--
	}
}

// ScrollDown moves to the next display event.
func (m *Model) ScrollDown() {
	if m.scroll < len(m.events)-1 {
		m.scroll++
	}
}
