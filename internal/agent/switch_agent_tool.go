package agent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"charm.land/fantasy"

	"github.com/megacli/megacli/internal/agent/tools"
	"github.com/megacli/megacli/internal/askuser"
	"github.com/megacli/megacli/internal/config"
)

const SwitchAgentToolName = "switch_agent"

// SwitchAgentParams are the LLM-provided parameters for the switch_agent tool.
type SwitchAgentParams struct {
	AgentID        string `json:"agent_id" description:"The ID of the agent to switch to (e.g. 'coder', 'planner')."`
	Reason         string `json:"reason" description:"Why the switch is suggested."`
	AcceptResponse string `json:"accept_response" description:"Content returned to you if the user accepts the switch."`
	RejectResponse string `json:"reject_response" description:"Content returned to you if the user rejects the switch."`
}

func (c *coordinator) switchAgentToolDesc() string {
	base := "Suggest switching to a different agent. The user will be asked to confirm. Provide a reason, and specify what content you want returned for both accept and reject outcomes."
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
		fmt.Fprintf(&sb, "- %s: %s\n", id, desc)
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

			if params.AgentID == c.currentAgentName.Get() {
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

			currentName := c.currentAgentName.Get()
			targetName := agentCfg.Name
			if targetName == "" {
				targetName = params.AgentID
			}

			reason := params.Reason
			if reason == "" {
				reason = "No reason provided"
			}

			question := askuser.Question{
				Content: fmt.Sprintf("%s suggests switching to %s\nReason: %s", currentName, targetName, reason),
				Options: []string{"Accept", "Reject"},
			}

			sessionID := tools.GetSessionFromContext(ctx)
			resp, err := c.askUser.Request(ctx, []askuser.Question{question}, sessionID)
			if err != nil {
				r := fantasy.NewTextErrorResponse("User cancelled the switch request.")
				r.StopTurn = true
				return r, nil
			}

			answer := ""
			if len(resp.Answers) > 0 {
				answer = strings.TrimSpace(resp.Answers[0])
			}

			switch answer {
			case "Accept":
				if _, err := c.SwitchAgent(ctx, params.AgentID); err != nil {
					return fantasy.NewTextErrorResponse(fmt.Sprintf("failed to switch agent: %v", err)), nil
				}
				if sessionID != "" {
					if err := c.sessions.UpdateActiveAgent(ctx, sessionID, params.AgentID); err != nil {
						slog.Error("Failed to persist active agent to session", "error", err)
					}
				}
				if params.AcceptResponse != "" {
					return fantasy.NewTextResponse(params.AcceptResponse), nil
				}
				return fantasy.NewTextResponse(fmt.Sprintf("Switched to agent %q. The new agent is now active.", params.AgentID)), nil

			case "Reject":
				if params.RejectResponse != "" {
					return fantasy.NewTextResponse(params.RejectResponse), nil
				}
				return fantasy.NewTextResponse("User rejected the switch. Continuing with current agent."), nil

			default:
				// User typed a custom response instead of selecting an option.
				return fantasy.NewTextResponse(answer), nil
			}
		})
}
