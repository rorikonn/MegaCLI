// Package setup implements the first-run configuration flow for MegaCli.
// It ensures a global config exists, the API key is set, and models are
// selected before entering the normal CLI loop.
package setup

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/megacli/megacli/internal/config"
)

const (
	defaultProviderID   = "athenai"
	defaultProviderName = "AthenAI"
	defaultBaseURL      = "https://athenai.mihoyo.com"
	defaultProviderType = "anthropic"
	modelsEndpoint      = "https://athenai.mihoyo.com/v1/models"
	apiKeyField         = "api_key"
	apiKeyPrefix        = "Bearer "
)

// templateConfig is the minimal config written on first run.
// It intentionally omits models — those are populated interactively.
var templateConfig = map[string]any{
	"providers": map[string]any{
		defaultProviderID: map[string]any{
			"name":     defaultProviderName,
			"type":     defaultProviderType,
			"base_url": defaultBaseURL,
			"api_key":  "",
		},
	},
	"options": map[string]any{
		"disable_default_providers": true,
	},
}

// EnsureGlobalConfig checks if the global config file exists.
// If not, creates the directory and writes a default template.
// Returns the path to the global config file.
func EnsureGlobalConfig() (string, error) {
	cfgPath := config.GlobalConfig()
	if _, err := os.Stat(cfgPath); err == nil {
		return cfgPath, nil
	}

	dir := filepath.Dir(cfgPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("failed to create config directory %s: %w", dir, err)
	}

	data, err := json.MarshalIndent(templateConfig, "", "  ")
	if err != nil {
		return "", err
	}
	data = append(data, '\n')

	if err := os.WriteFile(cfgPath, data, 0o644); err != nil {
		return "", fmt.Errorf("failed to write default config: %w", err)
	}

	fmt.Printf("Created default config at %s\n", cfgPath)
	return cfgPath, nil
}

// ForcePromptAPIKey always prompts the user for a new API key,
// regardless of whether one is already configured. Overwrites the
// existing key in the config file.
func ForcePromptAPIKey(cfgPath string) (string, error) {
	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		return "", err
	}

	// Show existing key if present
	var cfgMap map[string]any
	if err := json.Unmarshal(raw, &cfgMap); err != nil {
		return "", fmt.Errorf("failed to parse config: %w", err)
	}
	existingKey := extractAPIKey(cfgMap)
	existingRaw := strings.TrimPrefix(existingKey, apiKeyPrefix)
	if existingRaw != "" && !strings.HasPrefix(existingRaw, "$") {
		masked := existingRaw
		if len(masked) > 8 {
			masked = masked[:4] + strings.Repeat("*", len(masked)-8) + masked[len(masked)-4:]
		}
		fmt.Printf("\nCurrent API Key: %s\n", masked)
	}

	fmt.Print("Enter new API Key (leave empty to keep current): ")

	var input string
	if _, err := fmt.Scanln(&input); err != nil && err.Error() != "unexpected newline" {
		return "", fmt.Errorf("failed to read API key: %w", err)
	}
	input = strings.TrimSpace(input)
	input = strings.TrimPrefix(input, apiKeyPrefix)

	// If empty, keep existing key
	if input == "" {
		if existingRaw != "" && !strings.HasPrefix(existingRaw, "$") {
			return existingRaw, nil
		}
		return "", fmt.Errorf("API key cannot be empty")
	}

	if err := writeAPIKey(cfgPath, raw, input); err != nil {
		return "", err
	}

	fmt.Println("API Key saved to config.")
	return input, nil
}

// ReadOrPromptAPIKey reads the API key from config. If empty, prompts
// the user to enter one and writes it back to the config file.
// Returns the raw key (without "Bearer " prefix) for use in API calls.
func ReadOrPromptAPIKey(cfgPath string) (string, error) {
	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		return "", err
	}

	var cfgMap map[string]any
	if err := json.Unmarshal(raw, &cfgMap); err != nil {
		return "", fmt.Errorf("failed to parse config: %w", err)
	}

	storedKey := extractAPIKey(cfgMap)
	rawKey := strings.TrimPrefix(storedKey, apiKeyPrefix)

	// If it's an env var reference, try to resolve it
	if strings.HasPrefix(rawKey, "$") {
		envName := strings.TrimPrefix(rawKey, "$")
		if v := os.Getenv(envName); v != "" {
			return v, nil
		}
		// env var not set — fall through to prompt
	}

	if rawKey != "" && !strings.HasPrefix(rawKey, "$") {
		return rawKey, nil
	}

	fmt.Print("\nAthenAI API Key not found.\n")
	fmt.Print("Enter your API Key: ")

	var input string
	if _, err := fmt.Scanln(&input); err != nil {
		return "", fmt.Errorf("failed to read API key: %w", err)
	}
	input = strings.TrimSpace(input)
	input = strings.TrimPrefix(input, apiKeyPrefix)
	if input == "" {
		return "", fmt.Errorf("API key cannot be empty")
	}

	if err := writeAPIKey(cfgPath, raw, input); err != nil {
		return "", err
	}

	fmt.Println("API Key saved to config.")
	return input, nil
}

// ModelInfo represents a model returned by the /v1/models endpoint.
type ModelInfo struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	OwnedBy string `json:"owned_by"`
}

type modelsResponse struct {
	Data []ModelInfo `json:"data"`
}

// FetchModels queries the /v1/models endpoint and returns available models.
func FetchModels(apiKey string) ([]ModelInfo, error) {
	url := modelsEndpoint

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to query models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("models API returned %d: %s", resp.StatusCode, string(body))
	}

	var result modelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode models response: %w", err)
	}

	return result.Data, nil
}

// HasModelsConfigured checks if the provider already has models defined.
func HasModelsConfigured(cfgPath string) (bool, error) {
	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		return false, err
	}
	var cfgMap map[string]any
	if err := json.Unmarshal(raw, &cfgMap); err != nil {
		return false, err
	}

	providers, ok := cfgMap["providers"].(map[string]any)
	if !ok {
		return false, nil
	}
	provider, ok := providers[defaultProviderID].(map[string]any)
	if !ok {
		return false, nil
	}
	models, ok := provider["models"].([]any)
	return ok && len(models) > 0, nil
}

// WriteModelsToConfig writes the selected models into the global config
// and sets the large/small model selections.
func WriteModelsToConfig(cfgPath string, models []ModelInfo) error {
	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		return err
	}

	var cfgMap map[string]any
	if err := json.Unmarshal(raw, &cfgMap); err != nil {
		return err
	}

	providers, _ := cfgMap["providers"].(map[string]any)
	if providers == nil {
		providers = make(map[string]any)
	}
	provider, _ := providers[defaultProviderID].(map[string]any)
	if provider == nil {
		provider = make(map[string]any)
	}

	modelDefs := make([]map[string]any, len(models))
	for i, m := range models {
		modelDefs[i] = map[string]any{
			"id":                   m.ID,
			"name":                 m.ID,
			"context_window":       200000,
			"default_max_tokens":   16384,
			"supports_attachments": true,
		}
	}
	provider["models"] = modelDefs
	providers[defaultProviderID] = provider
	cfgMap["providers"] = providers

	// Auto-select first model as both large and small
	if len(models) > 0 {
		cfgMap["models"] = map[string]any{
			"large": map[string]any{
				"provider": defaultProviderID,
				"model":    models[0].ID,
			},
			"small": map[string]any{
				"provider": defaultProviderID,
				"model":    models[0].ID,
			},
		}
	}

	data, err := json.MarshalIndent(cfgMap, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(cfgPath, data, 0o644)
}

func extractAPIKey(cfgMap map[string]any) string {
	providers, ok := cfgMap["providers"].(map[string]any)
	if !ok {
		return ""
	}
	provider, ok := providers[defaultProviderID].(map[string]any)
	if !ok {
		return ""
	}
	key, _ := provider[apiKeyField].(string)
	return key
}

func writeAPIKey(cfgPath string, raw []byte, apiKey string) error {
	var cfgMap map[string]any
	if err := json.Unmarshal(raw, &cfgMap); err != nil {
		return err
	}

	providers, _ := cfgMap["providers"].(map[string]any)
	if providers == nil {
		providers = make(map[string]any)
	}
	provider, _ := providers[defaultProviderID].(map[string]any)
	if provider == nil {
		provider = make(map[string]any)
	}

	provider[apiKeyField] = apiKeyPrefix + apiKey
	providers[defaultProviderID] = provider
	cfgMap["providers"] = providers

	data, err := json.MarshalIndent(cfgMap, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(cfgPath, data, 0o644)
}
