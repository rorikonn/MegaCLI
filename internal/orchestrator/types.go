package orchestrator

import (
	"time"

	"github.com/megacli/megacli/internal/agent"
	"github.com/megacli/megacli/internal/message"
)

// AgentRole determines how an agent participates in orchestration.
type AgentRole string

const (
	// RolePrimary is the default user-facing agent. User prompts route here first.
	RolePrimary AgentRole = "primary"
	// RoleSecondary agents only activate via delegation from another agent.
	RoleSecondary AgentRole = "secondary"
)

// AgentStatus represents the current state of a managed agent.
type AgentStatus string

const (
	StatusIdle    AgentStatus = "idle"
	StatusBusy    AgentStatus = "busy"
	StatusWaiting AgentStatus = "waiting"
)

// ManagedAgent wraps a Crush Coordinator with orchestration metadata.
type ManagedAgent struct {
	Name        string
	Role        AgentRole
	Status      AgentStatus
	Coordinator agent.Coordinator
	SessionID   string
}

// DelegateRequest represents a task being handed from one agent to another.
type DelegateRequest struct {
	FromAgent string
	ToAgent   string
	Task      string
	SessionID string
	Context   string
}

// DelegateResult is returned when a delegated task completes.
type DelegateResult struct {
	FromAgent string
	ToAgent   string
	Success   bool
	Result    string
	Error     string
	Duration  time.Duration
}

// AgentEvent is published when agent status changes.
type AgentEvent struct {
	AgentName string
	OldStatus AgentStatus
	NewStatus AgentStatus
	SessionID string
}

// AgentConfig defines a single agent from the MegaCli config file.
type AgentConfig struct {
	Name                 string   `json:"name"`
	Role                 string   `json:"role"`
	Model                string   `json:"model,omitempty"`
	Tools                []string `json:"tools,omitempty"`
	SystemPromptTemplate string   `json:"system_prompt_template,omitempty"`
}

// DelegateResultToAttachment converts a DelegateResult into a message
// Attachment that can be passed back to the calling agent.
func DelegateResultToAttachment(result DelegateResult) message.Attachment {
	content := result.Result
	if !result.Success {
		content = "Delegation failed: " + result.Error
	}
	return message.Attachment{
		FilePath: "delegate:" + result.FromAgent,
		MimeType: "text/plain",
		Content:  []byte(content),
	}
}
