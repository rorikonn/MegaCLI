package agent

import (
	"context"
	"log/slog"
	"time"

	"charm.land/fantasy"
)

// loggedTool wraps a fantasy.AgentTool to emit structured log lines around
// every tool execution. This makes it trivial to diagnose hangs and
// measure tool latency from the application log alone.
type loggedTool struct {
	inner fantasy.AgentTool
}

func newLoggedTool(inner fantasy.AgentTool) *loggedTool {
	return &loggedTool{inner: inner}
}

// wrapToolsWithLogging returns a tool slice with each entry wrapped in a
// loggedTool that emits slog INFO on execution start/finish and WARN on
// context cancellation.
func wrapToolsWithLogging(tools []fantasy.AgentTool) []fantasy.AgentTool {
	out := make([]fantasy.AgentTool, len(tools))
	for i, tool := range tools {
		out[i] = newLoggedTool(tool)
	}
	return out
}

func (l *loggedTool) Info() fantasy.ToolInfo {
	return l.inner.Info()
}

func (l *loggedTool) ProviderOptions() fantasy.ProviderOptions {
	return l.inner.ProviderOptions()
}

func (l *loggedTool) SetProviderOptions(opts fantasy.ProviderOptions) {
	l.inner.SetProviderOptions(opts)
}

func (l *loggedTool) Run(ctx context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
	name := call.Name
	id := call.ID

	slog.Info("Tool call started",
		"tool", name,
		"tool_call_id", id,
	)

	start := time.Now()
	resp, err := l.inner.Run(ctx, call)
	elapsed := time.Since(start)

	if err != nil {
		if ctx.Err() != nil {
			slog.Warn("Tool call cancelled",
				"tool", name,
				"tool_call_id", id,
				"elapsed", elapsed.Truncate(time.Millisecond).String(),
				"error", err,
			)
		} else {
			slog.Error("Tool call failed",
				"tool", name,
				"tool_call_id", id,
				"elapsed", elapsed.Truncate(time.Millisecond).String(),
				"error", err,
			)
		}
		return resp, err
	}

	slog.Info("Tool call completed",
		"tool", name,
		"tool_call_id", id,
		"elapsed", elapsed.Truncate(time.Millisecond).String(),
	)
	return resp, nil
}
