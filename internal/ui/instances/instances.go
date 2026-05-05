// Package instances renders a panel showing other running MegaCli instances
// discovered via the IPC registry.
package instances

import (
	"fmt"
	"path/filepath"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/megacli/megacli/internal/ipc"
)

// Model holds the instances panel state.
type Model struct {
	instances []ipc.InstanceInfo
	selected  int
	width     int
	height    int
	visible   bool
	styles    Styles
}

// Styles for the instances panel.
type Styles struct {
	Border   lipgloss.Style
	Title    lipgloss.Style
	Instance lipgloss.Style
	Selected lipgloss.Style
	Agent    lipgloss.Style
	Path     lipgloss.Style
	Empty    lipgloss.Style
}

// DefaultStyles returns the default instance panel styles.
func DefaultStyles() Styles {
	return Styles{
		Border: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("5")).
			Padding(0, 1),
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("13")).
			MarginBottom(1),
		Instance: lipgloss.NewStyle().
			Foreground(lipgloss.Color("7")),
		Selected: lipgloss.NewStyle().
			Foreground(lipgloss.Color("14")).
			Bold(true),
		Agent: lipgloss.NewStyle().
			Foreground(lipgloss.Color("3")),
		Path: lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")),
		Empty: lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")).
			Italic(true),
	}
}

// New creates a new instances panel.
func New() Model {
	return Model{
		styles: DefaultStyles(),
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

// Update refreshes the instance list from the IPC manager.
func (m *Model) Update(instances []ipc.InstanceInfo) {
	m.instances = instances
	if m.selected >= len(m.instances) {
		m.selected = max(0, len(m.instances)-1)
	}
}

// Selected returns the currently selected instance, if any.
func (m Model) Selected() (ipc.InstanceInfo, bool) {
	if m.selected < len(m.instances) {
		return m.instances[m.selected], true
	}
	return ipc.InstanceInfo{}, false
}

// SelectNext moves selection down.
func (m *Model) SelectNext() {
	if m.selected < len(m.instances)-1 {
		m.selected++
	}
}

// SelectPrev moves selection up.
func (m *Model) SelectPrev() {
	if m.selected > 0 {
		m.selected--
	}
}

// View renders the instances panel.
func (m Model) View() string {
	if !m.visible || m.width < 10 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(m.styles.Title.Render("Instances"))
	sb.WriteString("\n")

	if len(m.instances) == 0 {
		sb.WriteString(m.styles.Empty.Render("  No other instances found"))
	} else {
		for i, inst := range m.instances {
			style := m.styles.Instance
			prefix := "  "
			if i == m.selected {
				style = m.styles.Selected
				prefix = "▸ "
			}

			dir := filepath.Base(inst.CWD)
			agents := strings.Join(inst.Agents, ", ")
			line := fmt.Sprintf("%sPID %d", prefix, inst.PID)
			sb.WriteString(style.Render(line))
			sb.WriteString("\n")
			sb.WriteString(m.styles.Path.Render(fmt.Sprintf("    %s", dir)))
			sb.WriteString("\n")
			sb.WriteString(m.styles.Agent.Render(fmt.Sprintf("    [%s]", agents)))
			sb.WriteString("\n")
		}
	}

	content := sb.String()
	return m.styles.Border.
		Width(m.width - 2).
		MaxHeight(m.height).
		Render(content)
}
