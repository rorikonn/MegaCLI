package agent

import (
	"context"
	"fmt"
	"strings"

	"charm.land/fantasy"

	"github.com/megacli/megacli/internal/config"
)

const SwitchAgentToolName = "switch_agent"

// SwitchAgentParams are the LLM-provided parameters for the switch_agent tool.
type SwitchAgentParams struct {
	AgentID string `json:"agent_id" description:"The ID of the agent to switch to (e.g. 'coder', 'plan')."`
}

func (c *coordinator) switchAgentToolDesc() string {
	base := "Switch the active agent. Only primary agents can be switched to. After switching, the current turn ends and the new agent takes over."
	agents := c.AvailableAgents()
	if len(agents) <= 1 {
		return base
	}
	var sb strings.Builder
	sb.WriteString(base)
	sb.WriteString("\n\n<available_agents>\n")
	for _, id := range agents {
		agentCfg, ok := c.agentDefs[id]
		if !ok {
			continue
		}
		desc := agentCfg.Description
		if desc == "" {
			desc = agentCfg.Name
		}
		current := ""
		if id == c.currentAgentName {
			current = " (current)"
		}
		fmt.Fprintf(&sb, "- %s: %s%s\n", id, desc, current)
	}
	sb.WriteString("</available_agents>")
	return sb.String()
}

func (c *coordinator) switchAgentTool() fantasy.AgentTool {
	return fantasy.NewAgentTool(
		SwitchAgentToolName,
		c.switchAgentToolDesc(),
		func(ctx context.Context, params SwitchAgentParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.AgentID == "" {
				return fantasy.NewTextErrorResponse("agent_id is required"), nil
			}

			if params.AgentID == c.currentAgentName {
				return fantasy.NewTextResponse(fmt.Sprintf("Already using agent %q.", params.AgentID)), nil
			}

			agentCfg, ok := c.agentDefs[params.AgentID]
			if !ok {
				return fantasy.NewTextErrorResponse(
					fmt.Sprintf("agent %q not found. Available: %s", params.AgentID, strings.Join(c.AvailableAgents(), ", ")),
				), nil
			}
			if agentCfg.EffectiveRole() != config.AgentRolePrimary {
				return fantasy.NewTextErrorResponse(
					fmt.Sprintf("agent %q is a subagent and cannot be switched to directly", params.AgentID),
				), nil
			}

			if err := c.SwitchAgent(ctx, params.AgentID); err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("failed to switch agent: %v", err)), nil
			}

			resp := fantasy.NewTextResponse(fmt.Sprintf("Switched to agent %q. The new agent is now active.", params.AgentID))
			resp.StopTurn = true
			return resp, nil
		})
}
