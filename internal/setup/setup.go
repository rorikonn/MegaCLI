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

// modelTypeDetect returns the provider type best suited for a model ID.
// Returns the provider type string ("anthropic" or "openai-compat").
func modelTypeDetect(modelID string) string {
	lower := strings.ToLower(modelID)
	// Anthropic models
	if strings.Contains(lower, "claude") || strings.Contains(lower, "anthropic") {
		return "anthropic"
	}
	// OpenAI models
	if strings.Contains(lower, "gpt") || strings.Contains(lower, "o1") ||
		strings.Contains(lower, "o3") || strings.Contains(lower, "o4") ||
		strings.Contains(lower, "openai") {
		return "openai-compat"
	}
	// DeepSeek models
	if strings.Contains(lower, "deepseek") {
		return "openai-compat"
	}
	// Gemini models
	if strings.Contains(lower, "gemini") {
		return "openai-compat"
	}
	// Default to openai-compat for unknown models
	return "openai-compat"
}

// providerIDForType returns a provider key for the given type.
func providerIDForType(ptype string) string {
	if ptype == "anthropic" {
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

// WriteModelsToConfig writes the selected models into the global config,
// splitting them across anthropic and openai-compat providers based on model type.
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

	// Group models by provider type.
	anthropicModels := []map[string]any{}
	openAICompatModels := []map[string]any{}
	for _, m := range models {
		def := map[string]any{
			"id":                   m.ID,
			"name":                 m.ID,
			"context_window":       200000,
			"default_max_tokens":   16384,
			"supports_attachments": true,
		}
		if modelTypeDetect(m.ID) == "anthropic" {
			anthropicModels = append(anthropicModels, def)
		} else {
			openAICompatModels = append(openAICompatModels, def)
		}
	}

	// Write anthropic models under the anthropic provider.
	if len(anthropicModels) > 0 {
		provider, _ := providers[defaultProviderID].(map[string]any)
		if provider == nil {
			provider = make(map[string]any)
		}
		provider["models"] = anthropicModels
		providers[defaultProviderID] = provider
	}

	// Write openai-compat models under the openai-compat provider.
	if len(openAICompatModels) > 0 {
		provider, _ := providers[defaultOpenAICompatProvider].(map[string]any)
		if provider == nil {
			provider = make(map[string]any)
		}
		provider["models"] = openAICompatModels
		providers[defaultOpenAICompatProvider] = provider
	}

	cfgMap["providers"] = providers

	// Build model selections using the correct provider per model.
	modelSelections := map[string]any{}
	if len(models) > 0 {
		firstModel := models[0]
		firstProvider := providerIDForType(modelTypeDetect(firstModel.ID))
		modelSelections["large"] = map[string]any{
			"provider": firstProvider,
			"model":    firstModel.ID,
		}
		modelSelections["small"] = map[string]any{
			"provider": firstProvider,
			"model":    firstModel.ID,
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
