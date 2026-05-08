package agent

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"strings"

	"charm.land/fantasy"

	"github.com/megacli/megacli/internal/agent/prompt"
	"github.com/megacli/megacli/internal/agent/tools"
	"github.com/megacli/megacli/internal/config"
)

//go:embed templates/agent_tool.md
var agentToolDescription []byte

// AgentParams are the LLM-provided parameters for the agent tool.
type AgentParams struct {
	Prompt string `json:"prompt" description:"The task for the agent to perform"`
	Target string `json:"target,omitempty" description:"Name of the subagent to use. If omitted, defaults to 'task'. Use 'available_subagents' in the tool description to see options."`
}

const (
	AgentToolName = "agent"
)

// buildSubAgents creates SessionAgent instances for all configured subagents
// and stores them in c.subAgents.
func (c *coordinator) buildSubAgents(ctx context.Context) error {
	c.subAgents = make(map[string]SessionAgent)
	for id, agentCfg := range c.agentDefs {
		if agentCfg.EffectiveRole() != config.AgentRoleSubagent {
			continue
		}
		if agentCfg.Disabled {
			continue
		}
		p, err := promptForAgent(agentCfg, prompt.WithWorkingDir(c.cfg.WorkingDir()))
		if err != nil {
			return fmt.Errorf("building prompt for subagent %q: %w", id, err)
		}
		agent, err := c.buildAgent(ctx, p, agentCfg, true)
		if err != nil {
			return fmt.Errorf("building subagent %q: %w", id, err)
		}
		c.subAgents[id] = agent
	}
	return nil
}

// agentToolDescription returns the tool description with dynamically
// appended subagent list.
func (c *coordinator) agentToolDesc() string {
	base := tools.FirstLineDescription(agentToolDescription)
	if len(c.subAgents) <= 1 {
		return base
	}
	var sb strings.Builder
	sb.WriteString(base)
	sb.WriteString("\n\n<available_subagents>\n")
	for id, agentCfg := range c.agentDefs {
		if agentCfg.EffectiveRole() != config.AgentRoleSubagent || agentCfg.Disabled {
			continue
		}
		desc := agentCfg.Description
		if desc == "" {
			desc = agentCfg.Name
		}
		fmt.Fprintf(&sb, "- %s: %s\n", id, desc)
	}
	sb.WriteString("</available_subagents>")
	return sb.String()
}

func (c *coordinator) agentTool(ctx context.Context) (fantasy.AgentTool, error) {
	if err := c.buildSubAgents(ctx); err != nil {
		return nil, err
	}

	// Ensure at least the task subagent exists (backward compat).
	if _, ok := c.subAgents[config.AgentTask]; !ok {
		agentCfg, ok := c.agentDefs[config.AgentTask]
		if !ok {
			return nil, errors.New("task agent not configured")
		}
		p, err := taskPrompt(prompt.WithWorkingDir(c.cfg.WorkingDir()))
		if err != nil {
			return nil, err
		}
		agent, err := c.buildAgent(ctx, p, agentCfg, true)
		if err != nil {
			return nil, err
		}
		c.subAgents[config.AgentTask] = agent
	}

	return fantasy.NewParallelAgentTool(
		AgentToolName,
		c.agentToolDesc(),
		func(ctx context.Context, params AgentParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.Prompt == "" {
				return fantasy.NewTextErrorResponse("prompt is required"), nil
			}

			target := params.Target
			if target == "" {
				target = config.AgentTask
			}

			agent, ok := c.subAgents[target]
			if !ok {
				var available []string
				for id := range c.subAgents {
					available = append(available, id)
				}
				return fantasy.NewTextErrorResponse(
					fmt.Sprintf("subagent %q not found. Available: %s", target, strings.Join(available, ", ")),
				), nil
			}

			sessionID := tools.GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.ToolResponse{}, errors.New("session id missing from context")
			}

			agentMessageID := tools.GetMessageFromContext(ctx)
			if agentMessageID == "" {
				return fantasy.ToolResponse{}, errors.New("agent message id missing from context")
			}

			agentCfg := c.agentDefs[target]
			return c.runSubAgent(ctx, subAgentParams{
				Agent:          agent,
				SessionID:      sessionID,
				AgentMessageID: agentMessageID,
				ToolCallID:     call.ID,
				Prompt:         params.Prompt,
				SessionTitle:   "New Agent Session",
				ModelType:      agentCfg.Model,
			})
		}), nil
}
