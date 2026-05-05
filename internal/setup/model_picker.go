package setup

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")).MarginBottom(1)
	itemStyle     = lipgloss.NewStyle().PaddingLeft(2)
	selectedStyle = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("10"))
	cursorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	helpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).MarginTop(1)
	checkOn       = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).SetString("[✓]")
	checkOff      = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).SetString("[ ]")
)

type modelItem struct {
	info     ModelInfo
	selected bool
}

type pickerModel struct {
	items   []modelItem
	cursor  int
	done    bool
	aborted bool
}

func newPickerModel(models []ModelInfo) pickerModel {
	items := make([]modelItem, len(models))
	for i, m := range models {
		items[i] = modelItem{info: m, selected: true}
	}
	return pickerModel{items: items}
}

func (m pickerModel) Init() tea.Cmd {
	return nil
}

func (m pickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.aborted = true
			m.done = true
			return m, tea.Quit
		case "enter":
			m.done = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		case "space", " ":
			m.items[m.cursor].selected = !m.items[m.cursor].selected
		case "a":
			allSelected := true
			for _, item := range m.items {
				if !item.selected {
					allSelected = false
					break
				}
			}
			for i := range m.items {
				m.items[i].selected = !allSelected
			}
		}
	}
	return m, nil
}

func (m pickerModel) View() tea.View {
	if m.done {
		return tea.NewView("")
	}

	var sb strings.Builder
	sb.WriteString(titleStyle.Render("Select models to enable"))
	sb.WriteString("\n")

	for i, item := range m.items {
		cursor := "  "
		if i == m.cursor {
			cursor = cursorStyle.Render("▸ ")
		}

		check := checkOff.String()
		if item.selected {
			check = checkOn.String()
		}

		style := itemStyle
		if item.selected {
			style = selectedStyle
		}

		name := item.info.ID
		if item.info.OwnedBy != "" {
			name += fmt.Sprintf("  (%s)", item.info.OwnedBy)
		}

		sb.WriteString(fmt.Sprintf("%s%s %s\n", cursor, check, style.Render(name)))
	}

	sb.WriteString(helpStyle.Render("↑/↓ move • space toggle • a all/none • enter confirm • q abort"))
	return tea.NewView(sb.String())
}

// RunModelPicker displays an interactive checkbox list for model selection.
// All models start as selected. Returns the selected models.
func RunModelPicker(models []ModelInfo) ([]ModelInfo, error) {
	if len(models) == 0 {
		return nil, fmt.Errorf("no models available")
	}

	m := newPickerModel(models)
	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}

	result := finalModel.(pickerModel)
	if result.aborted {
		return nil, fmt.Errorf("model selection aborted")
	}

	var selected []ModelInfo
	for _, item := range result.items {
		if item.selected {
			selected = append(selected, item.info)
		}
	}

	if len(selected) == 0 {
		return nil, fmt.Errorf("at least one model must be selected")
	}

	return selected, nil
}
