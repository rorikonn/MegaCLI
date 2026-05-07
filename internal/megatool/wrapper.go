package megatool

import (
	"context"
	"encoding/json"

	"charm.land/fantasy"
)

// DisplayHandler is called when a DisplayOnly tool produces output that should
// be rendered in the TUI. The app layer wires this to the PubSub/TUI system.
type DisplayHandler func(event DisplayEvent)

// WrapAll takes a slice of fantasy.AgentTool and wraps any that implement
// MegaTool with the appropriate response-mode interception. Non-MegaTool
// items pass through unchanged.
func WrapAll(tools []fantasy.AgentTool, onDisplay DisplayHandler) []fantasy.AgentTool {
	out := make([]fantasy.AgentTool, len(tools))
	for i, t := range tools {
		if mt, ok := t.(MegaTool); ok && mt.Mode() != ModeDefault {
			out[i] = &wrappedMegaTool{inner: mt, onDisplay: onDisplay}
		} else {
			out[i] = t
		}
	}
	return out
}

type wrappedMegaTool struct {
	inner     MegaTool
	onDisplay DisplayHandler
	opts      fantasy.ProviderOptions
}

func (w *wrappedMegaTool) Info() fantasy.ToolInfo {
	return w.inner.Info()
}

func (w *wrappedMegaTool) ProviderOptions() fantasy.ProviderOptions {
	return w.opts
}

func (w *wrappedMegaTool) SetProviderOptions(opts fantasy.ProviderOptions) {
	w.opts = opts
}

func (w *wrappedMegaTool) Run(ctx context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
	fullResult, err := w.inner.Run(ctx, call)
	if err != nil {
		return fullResult, err
	}

	summary := w.inner.LLMSummary(fullResult)
	resp := fantasy.NewTextResponse(summary)

	if w.inner.Mode() == ModeDisplayOnly {
		meta := DisplayMetadata{
			ToolName:      w.inner.Info().Name,
			Content:       fullResult.Content,
			InnerMetadata: fullResult.Metadata,
		}
		if b, err := json.Marshal(meta); err == nil {
			resp.Metadata = string(b)
		}
	}

	return resp, nil
}
