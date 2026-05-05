package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"charm.land/fantasy"
	"github.com/megacli/megacli/internal/message"
)

// Orchestrator manages multiple named agents and routes messages between them.
// It sits above the Crush Coordinator layer — each managed agent has its own
// Coordinator instance.
type Orchestrator struct {
	mu      sync.RWMutex
	agents  map[string]*ManagedAgent
	primary string

	onAgentEvent func(AgentEvent)
}

// New creates an Orchestrator. The onAgentEvent callback fires whenever an
// agent's status changes (for TUI dashboard updates).
func New(onAgentEvent func(AgentEvent)) *Orchestrator {
	return &Orchestrator{
		agents:       make(map[string]*ManagedAgent),
		onAgentEvent: onAgentEvent,
	}
}

// RegisterAgent adds an agent to the orchestrator. The first agent registered
// with RolePrimary becomes the default target for user prompts.
func (o *Orchestrator) RegisterAgent(agent *ManagedAgent) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if _, exists := o.agents[agent.Name]; exists {
		return fmt.Errorf("agent %q already registered", agent.Name)
	}

	agent.Status = StatusIdle
	o.agents[agent.Name] = agent

	if agent.Role == RolePrimary && o.primary == "" {
		o.primary = agent.Name
	}
	return nil
}

// PrimaryAgent returns the name of the primary agent.
func (o *Orchestrator) PrimaryAgent() string {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.primary
}

// GetAgent returns a managed agent by name.
func (o *Orchestrator) GetAgent(name string) (*ManagedAgent, bool) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	a, ok := o.agents[name]
	return a, ok
}

// ListAgents returns a snapshot of all managed agents.
func (o *Orchestrator) ListAgents() []*ManagedAgent {
	o.mu.RLock()
	defer o.mu.RUnlock()
	list := make([]*ManagedAgent, 0, len(o.agents))
	for _, a := range o.agents {
		list = append(list, a)
	}
	return list
}

// Run sends a prompt to the primary agent. This is the main entry point for
// user-initiated messages.
func (o *Orchestrator) Run(ctx context.Context, sessionID, prompt string, attachments ...message.Attachment) (*fantasy.AgentResult, error) {
	primary := o.PrimaryAgent()
	if primary == "" {
		return nil, fmt.Errorf("no primary agent registered")
	}
	return o.RunAgent(ctx, primary, sessionID, prompt, attachments...)
}

// RunAgent sends a prompt to a specific named agent.
func (o *Orchestrator) RunAgent(ctx context.Context, agentName, sessionID, prompt string, attachments ...message.Attachment) (*fantasy.AgentResult, error) {
	agent, ok := o.GetAgent(agentName)
	if !ok {
		return nil, fmt.Errorf("agent %q not found", agentName)
	}

	o.setStatus(agentName, StatusBusy)
	defer o.setStatus(agentName, StatusIdle)

	return agent.Coordinator.Run(ctx, sessionID, prompt, attachments...)
}

// Delegate sends a task from one agent to another and waits for completion.
// The calling agent is set to "waiting" status while the target executes.
func (o *Orchestrator) Delegate(ctx context.Context, req DelegateRequest) (DelegateResult, error) {
	target, ok := o.GetAgent(req.ToAgent)
	if !ok {
		return DelegateResult{
			FromAgent: req.ToAgent,
			ToAgent:   req.FromAgent,
			Success:   false,
			Error:     fmt.Sprintf("target agent %q not found", req.ToAgent),
		}, nil
	}

	o.setStatus(req.FromAgent, StatusWaiting)
	o.setStatus(req.ToAgent, StatusBusy)

	start := time.Now()

	taskPrompt := req.Task
	if req.Context != "" {
		taskPrompt = fmt.Sprintf("%s\n\nContext:\n%s", req.Task, req.Context)
	}

	result, err := target.Coordinator.Run(ctx, req.SessionID, taskPrompt)

	duration := time.Since(start)
	o.setStatus(req.ToAgent, StatusIdle)
	o.setStatus(req.FromAgent, StatusBusy)

	if err != nil {
		return DelegateResult{
			FromAgent: req.ToAgent,
			ToAgent:   req.FromAgent,
			Success:   false,
			Error:     err.Error(),
			Duration:  duration,
		}, nil
	}

	resultText := ""
	if result != nil {
		resultText = extractResultText(result)
	}

	return DelegateResult{
		FromAgent: req.ToAgent,
		ToAgent:   req.FromAgent,
		Success:   true,
		Result:    resultText,
		Duration:  duration,
	}, nil
}

// Cancel cancels all active requests across all agents.
func (o *Orchestrator) Cancel() {
	o.mu.RLock()
	defer o.mu.RUnlock()
	for _, a := range o.agents {
		a.Coordinator.CancelAll()
	}
}

// CancelAgent cancels the active request for a specific agent.
func (o *Orchestrator) CancelAgent(name string) {
	a, ok := o.GetAgent(name)
	if !ok {
		return
	}
	if a.SessionID != "" {
		a.Coordinator.Cancel(a.SessionID)
	}
}

// IsBusy returns true if any agent is busy.
func (o *Orchestrator) IsBusy() bool {
	o.mu.RLock()
	defer o.mu.RUnlock()
	for _, a := range o.agents {
		if a.Status == StatusBusy || a.Status == StatusWaiting {
			return true
		}
	}
	return false
}

func (o *Orchestrator) setStatus(name string, status AgentStatus) {
	o.mu.Lock()
	agent, ok := o.agents[name]
	if !ok {
		o.mu.Unlock()
		return
	}
	old := agent.Status
	agent.Status = status
	o.mu.Unlock()

	if o.onAgentEvent != nil && old != status {
		o.onAgentEvent(AgentEvent{
			AgentName: name,
			OldStatus: old,
			NewStatus: status,
			SessionID: agent.SessionID,
		})
	}
	slog.Debug("agent status changed", "agent", name, "old", old, "new", status)
}

func extractResultText(result *fantasy.AgentResult) string {
	if result == nil {
		return ""
	}
	var parts []string
	for _, step := range result.Steps {
		if t := step.Content.Text(); t != "" {
			parts = append(parts, t)
		}
	}
	text := strings.Join(parts, "\n")
	if len(text) > 4096 {
		text = text[:4096] + "\n... (truncated)"
	}
	return text
}
