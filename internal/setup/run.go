package setup

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/megacli/megacli/internal/config"
)

var (
	bannerStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14"))
	infoStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	okStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	errStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
)

// Run executes the full first-run setup flow:
//  1. Ensure global config exists
//  2. Read or prompt for API key
//  3. Fetch and select models if none configured
//
// Returns true if setup completed and the CLI should proceed normally.
func Run() (bool, error) {
	fmt.Println(bannerStyle.Render("\n  MegaCli — First-Run Setup\n"))

	// Step 1: global config
	cfgPath, err := EnsureGlobalConfig()
	if err != nil {
		return false, err
	}
	fmt.Println(okStyle.Render("  ✓ ") + infoStyle.Render("Config: "+cfgPath))

	// Step 2: API key
	apiKey, err := ReadOrPromptAPIKey(cfgPath)
	if err != nil {
		return false, err
	}
	masked := apiKey
	if len(masked) > 8 {
		masked = masked[:4] + strings.Repeat("*", len(masked)-8) + masked[len(masked)-4:]
	}
	fmt.Println(okStyle.Render("  ✓ ") + infoStyle.Render("API Key: "+masked))

	// Step 3: models
	hasModels, err := HasModelsConfigured(cfgPath)
	if err != nil {
		return false, err
	}

	if !hasModels {
		fmt.Println(infoStyle.Render("\n  Fetching available models from " + modelsEndpoint + " ..."))

		models, err := FetchModels(apiKey)
		if err != nil {
			fmt.Println(errStyle.Render("  ✗ Failed to fetch models: " + err.Error()))
			return false, err
		}

		if len(models) == 0 {
			fmt.Println(errStyle.Render("  ✗ No models available from the API"))
			return false, fmt.Errorf("no models returned by API")
		}

		fmt.Printf("  Found %d models. Select which to enable:\n\n", len(models))

		selected, err := RunModelPicker(models)
		if err != nil {
			return false, err
		}

		// Let user pick large / small model
		large, small, err := runModelRolePicker(selected)
		if err != nil {
			return false, err
		}

		if err := WriteModelsToConfig(cfgPath, selected); err != nil {
			return false, fmt.Errorf("failed to save models: %w", err)
		}

		if err := writeModelSelections(cfgPath, large, small); err != nil {
			return false, fmt.Errorf("failed to save model selections: %w", err)
		}

		fmt.Printf("\n"+okStyle.Render("  ✓ ")+"Enabled %d models (large: %s, small: %s)\n", len(selected), large, small)
	} else {
		fmt.Println(okStyle.Render("  ✓ ") + infoStyle.Render("Models already configured"))
	}

	fmt.Println(okStyle.Render("\n  Setup complete! Starting MegaCli...\n"))
	return true, nil
}

// NeedsSetup returns true if the first-run setup should be triggered.
func NeedsSetup() bool {
	cfgPath := config.GlobalConfig()
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		return true
	}

	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		return true
	}

	var cfgMap map[string]any
	if err := json.Unmarshal(raw, &cfgMap); err != nil {
		return true
	}

	apiKey := extractAPIKey(cfgMap)
	if apiKey == "" {
		return true
	}
	if strings.HasPrefix(apiKey, "$") {
		envName := strings.TrimPrefix(apiKey, "$")
		if os.Getenv(envName) == "" {
			return true
		}
	}

	hasModels, _ := HasModelsConfigured(cfgPath)
	return !hasModels
}

// --- role picker: select large / small model ---

type rolePickerModel struct {
	models  []string
	cursor  int
	stage   int // 0 = picking large, 1 = picking small
	large   string
	small   string
	done    bool
	aborted bool
}

func newRolePickerModel(models []string) rolePickerModel {
	return rolePickerModel{models: models}
}

func (m rolePickerModel) Init() tea.Cmd {
	return nil
}

func (m rolePickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.aborted = true
			m.done = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.models)-1 {
				m.cursor++
			}
		case "enter":
			if m.stage == 0 {
				m.large = m.models[m.cursor]
				m.stage = 1
				m.cursor = 0
			} else {
				m.small = m.models[m.cursor]
				m.done = true
				return m, tea.Quit
			}
		}
	}
	return m, nil
}

func (m rolePickerModel) View() tea.View {
	if m.done {
		return tea.NewView("")
	}

	var sb strings.Builder
	label := "Select the PRIMARY (large) model"
	if m.stage == 1 {
		label = fmt.Sprintf("Large: %s — Now select the SMALL (fast) model", m.large)
	}
	sb.WriteString(titleStyle.Render(label))
	sb.WriteString("\n")

	for i, name := range m.models {
		cursor := "  "
		if i == m.cursor {
			cursor = cursorStyle.Render("▸ ")
		}
		sb.WriteString(fmt.Sprintf("%s%s\n", cursor, name))
	}

	sb.WriteString(helpStyle.Render("↑/↓ move • enter select • q abort"))
	return tea.NewView(sb.String())
}

func runModelRolePicker(selected []ModelInfo) (large, small string, err error) {
	if len(selected) == 1 {
		return selected[0].ID, selected[0].ID, nil
	}

	names := make([]string, len(selected))
	for i, m := range selected {
		names[i] = m.ID
	}

	m := newRolePickerModel(names)
	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return "", "", err
	}

	result := finalModel.(rolePickerModel)
	if result.aborted {
		return "", "", fmt.Errorf("model role selection aborted")
	}

	return result.large, result.small, nil
}

func writeModelSelections(cfgPath, large, small string) error {
	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		return err
	}

	var cfgMap map[string]any
	if err := json.Unmarshal(raw, &cfgMap); err != nil {
		return err
	}

	cfgMap["models"] = map[string]any{
		"large": map[string]any{
			"provider": defaultProviderID,
			"model":    large,
		},
		"small": map[string]any{
			"provider": defaultProviderID,
			"model":    small,
		},
	}

	data, err := json.MarshalIndent(cfgMap, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(cfgPath, data, 0o644)
}
