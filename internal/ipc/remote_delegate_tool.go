package ipc

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	"charm.land/fantasy"
)

const RemoteDelegateToolName = "remote_delegate"

//go:embed remote_delegate_tool.md
var remoteDelegateDescription []byte

// RemoteDelegateParams are the LLM-provided parameters for remote delegation.
type RemoteDelegateParams struct {
	InstancePID int    `json:"instance_pid" description:"PID of the target MegaCli instance"`
	TargetAgent string `json:"target_agent" description:"Name of the agent on the remote instance"`
	Task        string `json:"task" description:"Task description for the remote agent"`
	Context     string `json:"context,omitempty" description:"Optional additional context"`
}

// NewRemoteDelegateTool creates a tool that delegates tasks to agents on
// other running MegaCli instances.
func NewRemoteDelegateTool(manager *Manager, callerAgent string) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		RemoteDelegateToolName,
		strings.TrimSpace(string(remoteDelegateDescription)),
		func(ctx context.Context, params RemoteDelegateParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.InstancePID == 0 {
				return fantasy.NewTextErrorResponse("instance_pid is required"), nil
			}
			if params.TargetAgent == "" {
				return fantasy.NewTextErrorResponse("target_agent is required"), nil
			}
			if params.Task == "" {
				return fantasy.NewTextErrorResponse("task is required"), nil
			}

			instances := manager.Instances()
			var found bool
			for _, inst := range instances {
				if inst.PID == params.InstancePID {
					found = true
					break
				}
			}
			if !found {
				return fantasy.NewTextErrorResponse(
					fmt.Sprintf("instance PID %d not found. Available instances: %s",
						params.InstancePID, listInstances(instances)),
				), nil
			}

			req := DelegateRequest{
				TargetAgent: params.TargetAgent,
				Task:        params.Task,
				Context:     params.Context,
				CallerAgent: callerAgent,
			}

			resp, err := manager.DelegateRemote(params.InstancePID, req)
			if err != nil {
				return fantasy.NewTextErrorResponse("remote delegation failed: " + err.Error()), nil
			}

			if !resp.Success {
				return fantasy.NewTextErrorResponse(
					fmt.Sprintf("remote agent %q on instance %d failed: %s",
						params.TargetAgent, params.InstancePID, resp.Error),
				), nil
			}

			summary := fmt.Sprintf(
				"Remote agent %q on instance PID %d completed the task in %dms.\n\nResult:\n%s",
				params.TargetAgent, params.InstancePID, resp.Duration, resp.Result,
			)
			return fantasy.NewTextResponse(summary), nil
		},
	)
}

func listInstances(instances []InstanceInfo) string {
	if len(instances) == 0 {
		return "(none)"
	}
	var parts []string
	for _, inst := range instances {
		parts = append(parts, fmt.Sprintf("PID %d (%s, agents: %s)",
			inst.PID, inst.CWD, strings.Join(inst.Agents, ",")))
	}
	return strings.Join(parts, "; ")
}
