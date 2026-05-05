// Package dashboard renders a sidebar showing all managed agents and
// their current status. It integrates with the Orchestrator to provide
// real-time agent state visibility.
package dashboard

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/megacli/megacli/internal/orchestrator"
)

// Model holds the dashboard state.
type Model struct {
	agents []agentEntry
	width  int
	height int
	styles Styles
}

type agentEntry struct {
	Name   string
	Role   orchestrator.AgentRole
	Status orchestrator.AgentStatus
}

// Styles for the dashboard panel.
type Styles struct {
	Border      lipgloss.Style
	Title       lipgloss.Style
	AgentName   lipgloss.Style
	StatusIdle  lipgloss.Style
	StatusBusy  lipgloss.Style
	StatusWait  lipgloss.Style
	RolePrimary lipgloss.Style
}

// DefaultStyles returns the default dashboard styles.
func DefaultStyles() Styles {
	return Styles{
		Border: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("8")).
			Padding(0, 1),
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("12")).
			MarginBottom(1),
		AgentName: lipgloss.NewStyle().
			Bold(true),
		StatusIdle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")),
		StatusBusy: lipgloss.NewStyle().
			Foreground(lipgloss.Color("3")).
			Bold(true),
		StatusWait: lipgloss.NewStyle().
			Foreground(lipgloss.Color("5")),
		RolePrimary: lipgloss.NewStyle().
			Foreground(lipgloss.Color("2")),
	}
}

// New creates a new dashboard model.
func New() Model {
	return Model{
		styles: DefaultStyles(),
	}
}

// SetSize updates the dashboard dimensions.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// UpdateAgents refreshes the agent list from the orchestrator.
func (m *Model) UpdateAgents(orch *orchestrator.Orchestrator) {
	managed := orch.ListAgents()
	m.agents = make([]agentEntry, len(managed))
	for i, a := range managed {
		m.agents[i] = agentEntry{
			Name:   a.Name,
			Role:   a.Role,
			Status: a.Status,
		}
	}
}

// View renders the dashboard panel.
func (m Model) View() string {
	if m.width < 10 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(m.styles.Title.Render("Agents"))
	sb.WriteString("\n")

	for _, a := range m.agents {
		statusIcon := m.renderStatus(a.Status)
		name := m.styles.AgentName.Render(a.Name)
		if a.Role == orchestrator.RolePrimary {
			name += " " + m.styles.RolePrimary.Render("★")
		}

		line := fmt.Sprintf(" %s %s", statusIcon, name)
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	if len(m.agents) == 0 {
		sb.WriteString(" (no agents)")
	}

	content := sb.String()
	return m.styles.Border.
		Width(m.width - 2).
		MaxHeight(m.height).
		Render(content)
}

func (m Model) renderStatus(status orchestrator.AgentStatus) string {
	switch status {
	case orchestrator.StatusBusy:
		return m.styles.StatusBusy.Render("●")
	case orchestrator.StatusWaiting:
		return m.styles.StatusWait.Render("◐")
	default:
		return m.styles.StatusIdle.Render("○")
	}
}
