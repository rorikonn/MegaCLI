package agent

import (
	"context"
	"fmt"

	"charm.land/fantasy"
)

// AllowedToolsFunc returns the current agent's allowed tool names.
// A nil return means all tools are allowed.
type AllowedToolsFunc func() []string

// gatedTool wraps a fantasy.AgentTool to check whether the current
// agent allows this tool at execution time. The tool definition (schema)
// is always included in API calls so that tool definitions never change
// across agent switches, preserving prompt cache.
type gatedTool struct {
	inner     fantasy.AgentTool
	allowedFn AllowedToolsFunc
}

func newGatedTool(inner fantasy.AgentTool, allowedFn AllowedToolsFunc) *gatedTool {
	return &gatedTool{inner: inner, allowedFn: allowedFn}
}

func (g *gatedTool) Info() fantasy.ToolInfo                   { return g.inner.Info() }
func (g *gatedTool) ProviderOptions() fantasy.ProviderOptions { return g.inner.ProviderOptions() }
func (g *gatedTool) SetProviderOptions(opts fantasy.ProviderOptions) {
	g.inner.SetProviderOptions(opts)
}

func (g *gatedTool) Run(ctx context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
	allowed := g.allowedFn()
	if allowed != nil {
		found := false
		for _, name := range allowed {
			if name == call.Name {
				found = true
				break
			}
		}
		if !found {
			return fantasy.NewTextErrorResponse(
				fmt.Sprintf("Tool %q is not available for the current agent. Switch to an agent that supports this tool, or ask the user for help.", call.Name),
			), nil
		}
	}
	return g.inner.Run(ctx, call)
}
