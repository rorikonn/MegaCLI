package agent

import (
	"context"
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/megacli/megacli/internal/agent/prompt"
	"github.com/megacli/megacli/internal/config"
)

//go:embed templates
var templateFS embed.FS

// Individual embeds kept for backward-compatible helpers.
//
//go:embed templates/coder.md.tpl
var coderPromptTmpl []byte

//go:embed templates/task.md.tpl
var taskPromptTmpl []byte

//go:embed templates/plan.md.tpl
var planPromptTmpl []byte

//go:embed templates/initialize.md.tpl
var initializePromptTmpl []byte

// promptForAgent builds a Prompt for the given agent config.
// Resolution order:
//  1. PromptFile (disk path) if set.
//  2. Prompt (built-in template name) looked up in the embedded FS.
//  3. Fallback to coder.md.tpl.
func promptForAgent(agentCfg config.Agent, opts ...prompt.Option) (*prompt.Prompt, error) {
	name := agentCfg.Prompt
	if name == "" {
		name = agentCfg.ID
	}

	// PromptFile takes precedence: load from disk.
	if agentCfg.PromptFile != "" {
		data, err := os.ReadFile(agentCfg.PromptFile)
		if err != nil {
			return nil, fmt.Errorf("reading prompt file %s: %w", agentCfg.PromptFile, err)
		}
		return prompt.NewPrompt(name, string(data), opts...)
	}

	// Look up built-in template by name.
	tmplPath := filepath.ToSlash(fmt.Sprintf("templates/%s.md.tpl", name))
	data, err := templateFS.ReadFile(tmplPath)
	if err == nil {
		return prompt.NewPrompt(name, string(data), opts...)
	}

	// Fallback to coder template.
	return prompt.NewPrompt("coder", string(coderPromptTmpl), opts...)
}

func coderPrompt(opts ...prompt.Option) (*prompt.Prompt, error) {
	return prompt.NewPrompt("coder", string(coderPromptTmpl), opts...)
}

func taskPrompt(opts ...prompt.Option) (*prompt.Prompt, error) {
	return prompt.NewPrompt("task", string(taskPromptTmpl), opts...)
}

func InitializePrompt(cfg *config.ConfigStore) (string, error) {
	systemPrompt, err := prompt.NewPrompt("initialize", string(initializePromptTmpl))
	if err != nil {
		return "", err
	}
	return systemPrompt.Build(context.Background(), "", "", cfg)
}
