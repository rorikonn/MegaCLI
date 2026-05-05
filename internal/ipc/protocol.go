package ipc

// RPC method names for inter-process communication.
const (
	MethodDiscover    = "MegaCli.Discover"
	MethodDelegate    = "MegaCli.Delegate"
	MethodQueryStatus = "MegaCli.QueryStatus"
	MethodSubscribe   = "MegaCli.Subscribe"
)

// DiscoverRequest is sent to find available agents on a remote instance.
type DiscoverRequest struct{}

// DiscoverResponse lists the remote instance's agents and their status.
type DiscoverResponse struct {
	Instance InstanceInfo     `json:"instance"`
	Agents   []RemoteAgent    `json:"agents"`
}

// RemoteAgent describes an agent running on a remote MegaCli instance.
type RemoteAgent struct {
	Name   string `json:"name"`
	Role   string `json:"role"`
	Status string `json:"status"`
}

// DelegateRequest asks a remote instance to run a task on one of its agents.
type DelegateRequest struct {
	TargetAgent string `json:"target_agent"`
	Task        string `json:"task"`
	Context     string `json:"context,omitempty"`
	CallerPID   int    `json:"caller_pid"`
	CallerAgent string `json:"caller_agent"`
}

// DelegateResponse is the result of a remote delegation.
type DelegateResponse struct {
	Success  bool   `json:"success"`
	Result   string `json:"result"`
	Error    string `json:"error,omitempty"`
	Duration int64  `json:"duration_ms"`
}

// StatusRequest queries a remote instance's current state.
type StatusRequest struct{}

// StatusResponse contains the remote instance's full state.
type StatusResponse struct {
	Instance InstanceInfo  `json:"instance"`
	Agents   []RemoteAgent `json:"agents"`
	Busy     bool          `json:"busy"`
}

// EventType identifies the kind of cross-instance event.
type EventType string

const (
	EventAgentStatusChanged EventType = "agent_status_changed"
	EventDelegateStarted    EventType = "delegate_started"
	EventDelegateCompleted  EventType = "delegate_completed"
)

// RemoteEvent is an event from a remote MegaCli instance.
type RemoteEvent struct {
	Type      EventType `json:"type"`
	Source    int       `json:"source_pid"`
	AgentName string   `json:"agent_name,omitempty"`
	Status    string   `json:"status,omitempty"`
	Message   string   `json:"message,omitempty"`
}
