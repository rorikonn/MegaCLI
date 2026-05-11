package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"charm.land/fantasy"
)

const maxContentLen = 500

// LLMCommLogger writes structured LLM communication logs to a file.
// Only active when running in debug mode. It logs step-level and
// turn-level events for each session.
type LLMCommLogger struct {
	mu  sync.Mutex
	f   *os.File
	enc *json.Encoder
}

// NewLLMCommLogger creates a new LLMCommLogger that writes to
// dataDir/logs/llm_comm.log. Returns nil if the file cannot be
// opened.
func NewLLMCommLogger(dataDir string) *LLMCommLogger {
	logDir := filepath.Join(dataDir, "logs")
	if err := os.MkdirAll(logDir, 0o700); err != nil {
		return nil
	}
	f, err := os.OpenFile(filepath.Join(logDir, "llm_comm.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil
	}
	return &LLMCommLogger{
		f:   f,
		enc: json.NewEncoder(f),
	}
}

func (l *LLMCommLogger) write(record map[string]any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	// Best-effort write; ignore errors so logging never breaks the agent.
	_ = l.enc.Encode(record)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "...[truncated]"
}

// LogTurnStart records the beginning of a user turn.
func (l *LLMCommLogger) LogTurnStart(sessionID, userPrompt, model, provider string, promptLen int, stepNum int) {
	l.write(map[string]any{
		"type":          "turn_start",
		"session_id":    sessionID,
		"timestamp":     time.Now().Format(time.RFC3339),
		"model":         model,
		"provider":      provider,
		"user_input":    truncate(userPrompt, 200),
		"prompt_length": promptLen,
		"step_number":   stepNum,
	})
}

// LogStepRequest records what is being sent to the LLM in a step.
func (l *LLMCommLogger) LogStepRequest(sessionID string, stepNum int, messageCount int, toolNames []string, model string) {
	l.write(map[string]any{
		"type":          "step_request",
		"session_id":    sessionID,
		"timestamp":     time.Now().Format(time.RFC3339),
		"step_number":   stepNum,
		"message_count": messageCount,
		"tools":         toolNames,
		"model":         model,
	})
}

// LogToolCall records a tool invocation from the LLM.
func (l *LLMCommLogger) LogToolCall(sessionID string, tc fantasy.ToolCallContent) {
	l.write(map[string]any{
		"type":              "tool_call",
		"session_id":        sessionID,
		"timestamp":         time.Now().Format(time.RFC3339),
		"tool_call_id":      tc.ToolCallID,
		"tool_name":         tc.ToolName,
		"input":             truncate(tc.Input, maxContentLen),
		"provider_executed": tc.ProviderExecuted,
	})
}

// LogToolResult records the result of a tool execution.
func (l *LLMCommLogger) LogToolResult(sessionID string, tr fantasy.ToolResultContent) {
	resultType := "unknown"
	resultLen := 0
	if text, ok := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentText](tr.Result); ok {
		resultType = "text"
		resultLen = len(text.Text)
	} else if _, ok := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentError](tr.Result); ok {
		resultType = "error"
	} else if _, ok := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentMedia](tr.Result); ok {
		resultType = "media"
	}

	l.write(map[string]any{
		"type":              "tool_result",
		"session_id":        sessionID,
		"timestamp":         time.Now().Format(time.RFC3339),
		"tool_call_id":      tr.ToolCallID,
		"tool_name":         tr.ToolName,
		"result_type":       resultType,
		"result_length":     resultLen,
		"provider_executed": tr.ProviderExecuted,
	})
}

// LogStepResponse records the LLM's response for a step, including
// token usage and finish reason.
func (l *LLMCommLogger) LogStepResponse(sessionID string, stepNum int, result fantasy.StepResult) {
	toolCalls := result.Content.ToolCalls()
	toolCallNames := make([]string, len(toolCalls))
	for i, tc := range toolCalls {
		toolCallNames[i] = tc.ToolName
	}
	toolResults := result.Content.ToolResults()
	toolResultNames := make([]string, len(toolResults))
	for i, tr := range toolResults {
		toolResultNames[i] = tr.ToolName
	}

	l.write(map[string]any{
		"type":          "step_response",
		"session_id":    sessionID,
		"timestamp":     time.Now().Format(time.RFC3339),
		"step_number":   stepNum,
		"finish_reason": string(result.FinishReason),
		"text_length":   len(result.Content.Text()),
		"tool_calls":    toolCallNames,
		"tool_results":  toolResultNames,
		"input_tokens":  result.Usage.InputTokens,
		"output_tokens": result.Usage.OutputTokens,
		"cache_read":    result.Usage.CacheReadTokens,
		"cache_write":   result.Usage.CacheCreationTokens,
		"reasoning":     result.Usage.ReasoningTokens,
		"total_tokens":  result.Usage.TotalTokens,
	})
}

// LogTurnEnd records the end of a user turn with cumulative
// statistics.
func (l *LLMCommLogger) LogTurnEnd(sessionID string, steps int, cumulativeUsage fantasy.Usage, duration time.Duration, toolNames []string) {
	l.write(map[string]any{
		"type":          "turn_end",
		"session_id":    sessionID,
		"timestamp":     time.Now().Format(time.RFC3339),
		"steps":         steps,
		"tool_calls":    toolNames,
		"duration_ms":   duration.Milliseconds(),
		"input_tokens":  cumulativeUsage.InputTokens,
		"output_tokens": cumulativeUsage.OutputTokens,
		"cache_read":    cumulativeUsage.CacheReadTokens,
		"cache_write":   cumulativeUsage.CacheCreationTokens,
		"reasoning":     cumulativeUsage.ReasoningTokens,
		"total_tokens":  cumulativeUsage.TotalTokens,
	})
}

// LogTurnError records a turn that ended with an error.
func (l *LLMCommLogger) LogTurnError(sessionID string, steps int, cumulativeUsage fantasy.Usage, duration time.Duration, toolNames []string, err error) {
	l.write(map[string]any{
		"type":          "turn_error",
		"session_id":    sessionID,
		"timestamp":     time.Now().Format(time.RFC3339),
		"steps":         steps,
		"tool_calls":    toolNames,
		"duration_ms":   duration.Milliseconds(),
		"error":         fmt.Sprintf("%v", err),
		"input_tokens":  cumulativeUsage.InputTokens,
		"output_tokens": cumulativeUsage.OutputTokens,
		"cache_read":    cumulativeUsage.CacheReadTokens,
		"cache_write":   cumulativeUsage.CacheCreationTokens,
		"reasoning":     cumulativeUsage.ReasoningTokens,
		"total_tokens":  cumulativeUsage.TotalTokens,
	})
}

// Close closes the underlying log file.
func (l *LLMCommLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.f.Close()
}
