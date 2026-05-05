package orchestrator

import (
	"fmt"

	"github.com/megacli/megacli/internal/config"
)

// OrchestratorConfig is the top-level orchestrator configuration, loaded
// from the [orchestrator] section of megacli.json.
type OrchestratorConfig struct {
	Agents []AgentConfig `json:"agents"`
}

// Validate checks that the orchestrator config is well-formed.
func (c *OrchestratorConfig) Validate() error {
	if len(c.Agents) == 0 {
		return fmt.Errorf("at least one agent must be configured")
	}

	names := make(map[string]bool)
	hasPrimary := false
	for _, a := range c.Agents {
		if a.Name == "" {
			return fmt.Errorf("agent name is required")
		}
		if names[a.Name] {
			return fmt.Errorf("duplicate agent name: %q", a.Name)
		}
		names[a.Name] = true

		role := AgentRole(a.Role)
		if role != RolePrimary && role != RoleSecondary {
			return fmt.Errorf("agent %q has invalid role %q (must be 'primary' or 'secondary')", a.Name, a.Role)
		}
		if role == RolePrimary {
			if hasPrimary {
				return fmt.Errorf("only one primary agent is allowed, found duplicate at %q", a.Name)
			}
			hasPrimary = true
		}
	}
	if !hasPrimary {
		return fmt.Errorf("exactly one agent must have role 'primary'")
	}
	return nil
}

// DefaultConfig returns the default orchestrator config which mirrors Crush's
// built-in coder + task agent setup.
func DefaultConfig() OrchestratorConfig {
	return OrchestratorConfig{
		Agents: []AgentConfig{
			{
				Name: config.AgentCoder,
				Role: string(RolePrimary),
			},
			{
				Name: config.AgentTask,
				Role: string(RoleSecondary),
			},
		},
	}
}

// MergeWithCrushConfig takes an OrchestratorConfig and ensures that each
// agent has a corresponding entry in the Crush config.Agents map. Missing
// entries are created with sensible defaults.
func MergeWithCrushConfig(orchCfg OrchestratorConfig, crushCfg *config.Config) {
	if crushCfg.Agents == nil {
		crushCfg.SetupAgents()
	}
	for _, ac := range orchCfg.Agents {
		if _, exists := crushCfg.Agents[ac.Name]; !exists {
			crushCfg.Agents[ac.Name] = config.Agent{
				ID:           ac.Name,
				Name:         ac.Name,
				Description:  fmt.Sprintf("Custom agent: %s", ac.Name),
				Model:        config.SelectedModelTypeLarge,
				ContextPaths: crushCfg.Options.ContextPaths,
			}
		}
	}
}
