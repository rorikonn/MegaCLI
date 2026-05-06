package setup

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/megacli/megacli/internal/config"
)

var (
	bannerStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14"))
	infoStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	okStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	errStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
)

// RunForce executes the setup flow unconditionally, overwriting existing
// configuration. Used by the --startup flag to allow re-configuration.
func RunForce() (bool, error) {
	fmt.Println(bannerStyle.Render("\n  MegaCli — Setup (forced)\n"))

	// Step 1: global config
	cfgPath, err := EnsureGlobalConfig()
	if err != nil {
		return false, err
	}
	fmt.Println(okStyle.Render("  ✓ ") + infoStyle.Render("Config: "+cfgPath))

	// Step 2: always prompt for API key (overwrite existing)
	apiKey, err := ForcePromptAPIKey(cfgPath)
	if err != nil {
		return false, err
	}
	masked := apiKey
	if len(masked) > 8 {
		masked = masked[:4] + strings.Repeat("*", len(masked)-8) + masked[len(masked)-4:]
	}
	fmt.Println(okStyle.Render("  ✓ ") + infoStyle.Render("API Key: "+masked))

	// Step 3: always re-fetch models and write config with presets.
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

	fmt.Printf("  Found %d models.\n", len(models))

	if err := WriteModelsToConfig(cfgPath, models); err != nil {
		return false, fmt.Errorf("failed to save models: %w", err)
	}

	fmt.Println(okStyle.Render("  ✓ ") + infoStyle.Render("Models configured"))
	fmt.Println(okStyle.Render("\n  Setup complete! Configuration has been updated.\n"))
	return true, nil
}

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

		fmt.Printf("  Found %d models.\n", len(models))

		if err := WriteModelsToConfig(cfgPath, models); err != nil {
			return false, fmt.Errorf("failed to save models: %w", err)
		}

		fmt.Println(okStyle.Render("  ✓ ") + infoStyle.Render("Models configured"))
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
