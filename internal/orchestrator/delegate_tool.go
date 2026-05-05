package orchestrator

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	"charm.land/fantasy"
)

const DelegateToolName = "delegate"

//go:embed delegate_tool.md
var delegateDescription []byte

// DelegateParams are the LLM-provided parameters for the delegate tool.
type DelegateParams struct {
	Target  string `json:"target" description:"Name of the agent to delegate to (e.g. 'reviewer', 'architect')"`
	Task    string `json:"task" description:"A clear description of the task for the target agent"`
	Context string `json:"context,omitempty" description:"Optional additional context to pass to the target agent"`
}

// NewDelegateTool creates a fantasy.AgentTool that delegates tasks to other
// agents via the Orchestrator.
func NewDelegateTool(orch *Orchestrator, callerAgent string) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		DelegateToolName,
		strings.TrimSpace(string(delegateDescription)),
		func(ctx context.Context, params DelegateParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.Target == "" {
				return fantasy.NewTextErrorResponse("target agent name is required"), nil
			}
			if params.Task == "" {
				return fantasy.NewTextErrorResponse("task description is required"), nil
			}
			if params.Target == callerAgent {
				return fantasy.NewTextErrorResponse("cannot delegate to self"), nil
			}

			targetAgent, ok := orch.GetAgent(params.Target)
			if !ok {
				available := listAvailableAgents(orch, callerAgent)
				return fantasy.NewTextErrorResponse(
					fmt.Sprintf("agent %q not found. Available agents: %s", params.Target, available),
				), nil
			}

			req := DelegateRequest{
				FromAgent: callerAgent,
				ToAgent:   params.Target,
				Task:      params.Task,
				SessionID: targetAgent.SessionID,
				Context:   params.Context,
			}

			result, err := orch.Delegate(ctx, req)
			if err != nil {
				return fantasy.NewTextErrorResponse("delegation error: " + err.Error()), nil
			}

			if !result.Success {
				return fantasy.NewTextErrorResponse(
					fmt.Sprintf("delegation to %q failed: %s", params.Target, result.Error),
				), nil
			}

			summary := fmt.Sprintf(
				"Agent %q completed the task in %s.\n\nResult:\n%s",
				params.Target, result.Duration.Round(100*result.Duration/100), result.Result,
			)
			return fantasy.NewTextResponse(summary), nil
		},
	)
}

func listAvailableAgents(orch *Orchestrator, exclude string) string {
	agents := orch.ListAgents()
	var names []string
	for _, a := range agents {
		if a.Name != exclude {
			names = append(names, fmt.Sprintf("%s (%s)", a.Name, a.Role))
		}
	}
	if len(names) == 0 {
		return "(none)"
	}
	return strings.Join(names, ", ")
}
