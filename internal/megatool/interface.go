package megatool

import "charm.land/fantasy"

// ResponseMode controls how a tool's result is routed between the TUI and the LLM.
type ResponseMode int

const (
	// ModeDefault passes the full tool result to both the TUI and the LLM.
	ModeDefault ResponseMode = iota

	// ModeDisplayOnly renders the full result in the TUI but returns only
	// a short summary string to the LLM, saving context window tokens.
	ModeDisplayOnly

	// ModeSilent suppresses TUI rendering entirely. The LLM receives only
	// a summary. Useful for internal bookkeeping tools.
	ModeSilent
)

// MegaTool extends fantasy.AgentTool with response-routing semantics.
type MegaTool interface {
	fantasy.AgentTool

	// Mode returns the routing mode for this tool's responses.
	Mode() ResponseMode

	// LLMSummary produces a compact string that replaces the full tool
	// result in the LLM context when Mode() is not ModeDefault.
	LLMSummary(result fantasy.ToolResponse) string
}

// DisplayEvent is published via PubSub when a ModeDisplayOnly or ModeDefault
// tool wants to show rich content in the TUI.
type DisplayEvent struct {
	ToolName  string
	SessionID string
	MessageID string
	Content   string
	MimeType  string // e.g. "text/plain", "text/markdown"
}

// DisplayMetadata is serialized to JSON and stored in ToolResponse.Metadata
// so that the Chat renderer can extract the full content for inline display
// while the LLM only sees a short summary in ToolResponse.Content.
type DisplayMetadata struct {
	ToolName string `json:"tool_name"`
	Content  string `json:"content"`
}
