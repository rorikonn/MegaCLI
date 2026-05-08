// Package setup implements the first-run configuration flow for MegaCli.
// It ensures a global config exists, the API key is set, and models are
// selected before entering the normal CLI loop.
package setup

import (
	_ "embed"
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

//go:embed model_presets.json
var modelPresetsJSON []byte

const (
	defaultProviderID           = "athenai"
	defaultProviderName         = "AthenAI"
	defaultBaseURL              = "https://athenai.mihoyo.com"
	defaultProviderType         = "anthropic"
	defaultOpenAICompatProvider = "athenai-openai-compat"
	defaultOpenAICompatType     = "openai-compat"
	modelsEndpoint              = "https://athenai.mihoyo.com/v1/models"
	apiKeyField                 = "api_key"
	apiKeyPrefix                = "Bearer "
)

// templateProvider holds model definitions and advanced model IDs for a
// single provider within the setup template.
type templateProvider struct {
	Models         []map[string]any `json:"models"`
	AdvancedModels []string         `json:"advanced_models"`
}

// setupTemplate is the embedded JSON template that defines all known model
// configurations, their ordering, and default parameters.
type setupTemplate struct {
	ProviderOrder        []string                    `json:"provider_order"`
	Providers            map[string]templateProvider `json:"providers"`
	DefaultContextWindow int64                       `json:"default_context_window"`
	DefaultMaxTokens     int64                       `json:"default_max_tokens"`
}

// detectProvider returns the provider ID for a given model name.
func detectProvider(modelID string) string {
	lower := strings.ToLower(modelID)
	if strings.Contains(lower, "claude") || strings.Contains(lower, "anthropic") {
		return defaultProviderID
	}
	return defaultOpenAICompatProvider
}

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
		defaultOpenAICompatProvider: map[string]any{
			"name":     defaultProviderName + " (OpenAI Compat)",
			"type":     defaultOpenAICompatType,
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

// HasModelsConfigured checks if any provider already has models defined.
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
	for _, pid := range []string{defaultProviderID, defaultOpenAICompatProvider} {
		provider, ok := providers[pid].(map[string]any)
		if !ok {
			continue
		}
		models, ok := provider["models"].([]any)
		if ok && len(models) > 0 {
			return true, nil
		}
	}
	return false, nil
}

// WriteModelsToConfig writes models into the global config using the embedded
// template. Template models are filtered against the API's available list,
// then any remaining API models not in the template are appended with
// default parameters (fallback). The first non-advanced model in template
// order becomes the default.
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

	// Load embedded template.
	var tpl setupTemplate
	if err := json.Unmarshal(modelPresetsJSON, &tpl); err != nil {
		return fmt.Errorf("failed to parse model presets template: %w", err)
	}

	// Build API available model ID set (case-insensitive).
	available := make(map[string]bool, len(models))
	for _, m := range models {
		available[strings.ToLower(m.ID)] = true
	}

	// Build set of model IDs covered by template.
	templateCovered := make(map[string]bool)
	for _, p := range tpl.Providers {
		for _, m := range p.Models {
			if id, ok := m["id"].(string); ok {
				templateCovered[strings.ToLower(id)] = true
			}
		}
	}

	// Filter template models: keep only those available in API.
	for pid, provider := range tpl.Providers {
		var filtered []map[string]any
		for _, m := range provider.Models {
			id, _ := m["id"].(string)
			if available[strings.ToLower(id)] {
				filtered = append(filtered, m)
			}
		}
		var filteredAdv []string
		for _, id := range provider.AdvancedModels {
			if available[strings.ToLower(id)] {
				filteredAdv = append(filteredAdv, id)
			}
		}
		provider.Models = filtered
		provider.AdvancedModels = filteredAdv
		tpl.Providers[pid] = provider
	}

	// Fallback: append API models not covered by template.
	for _, m := range models {
		if templateCovered[strings.ToLower(m.ID)] {
			continue
		}
		pid := detectProvider(m.ID)
		p := tpl.Providers[pid]
		p.Models = append(p.Models, map[string]any{
			"id":                 m.ID,
			"name":               m.ID,
			"context_window":     tpl.DefaultContextWindow,
			"default_max_tokens": tpl.DefaultMaxTokens,
		})
		tpl.Providers[pid] = p
	}

	// Write providers to config.
	for pid, provider := range tpl.Providers {
		existing, _ := providers[pid].(map[string]any)
		if existing == nil {
			existing = make(map[string]any)
		}
		existing["models"] = provider.Models
		if len(provider.AdvancedModels) > 0 {
			existing["advanced_models"] = provider.AdvancedModels
		}
		providers[pid] = existing
	}
	cfgMap["providers"] = providers

	// Select default model: first non-advanced model in template order.
	modelSelections := map[string]any{}
	for _, pid := range tpl.ProviderOrder {
		provider := tpl.Providers[pid]
		advSet := make(map[string]bool, len(provider.AdvancedModels))
		for _, id := range provider.AdvancedModels {
			advSet[id] = true
		}
		for _, m := range provider.Models {
			id, _ := m["id"].(string)
			if !advSet[id] {
				modelSelections["large"] = map[string]any{
					"provider": pid,
					"model":    id,
				}
				modelSelections["small"] = map[string]any{
					"provider": pid,
					"model":    id,
				}
				break
			}
		}
		if len(modelSelections) > 0 {
			break
		}
	}
	cfgMap["models"] = modelSelections

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
	for _, pid := range []string{defaultProviderID, defaultOpenAICompatProvider} {
		provider, ok := providers[pid].(map[string]any)
		if !ok {
			continue
		}
		key, _ := provider[apiKeyField].(string)
		if key != "" {
			return key
		}
	}
	return ""
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

	// Write API key to both providers.
	for _, pid := range []string{defaultProviderID, defaultOpenAICompatProvider} {
		provider, _ := providers[pid].(map[string]any)
		if provider == nil {
			provider = make(map[string]any)
		}
		provider[apiKeyField] = apiKeyPrefix + apiKey
		providers[pid] = provider
	}

	cfgMap["providers"] = providers

	data, err := json.MarshalIndent(cfgMap, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(cfgPath, data, 0o644)
}
