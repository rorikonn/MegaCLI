package ipc

import (
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/megacli/megacli/internal/orchestrator"
)

// Manager coordinates the IPC server, client connections, and instance registry.
type Manager struct {
	server *Server
	orch   *orchestrator.Orchestrator
	info   InstanceInfo

	mu        sync.RWMutex
	instances []InstanceInfo

	stopRefresh chan struct{}
}

// NewManager creates a new IPC manager.
func NewManager(orch *orchestrator.Orchestrator) *Manager {
	return &Manager{
		orch:        orch,
		stopRefresh: make(chan struct{}),
	}
}

// Start initializes the IPC server, registers this instance, and begins
// periodic discovery of other instances.
func (m *Manager) Start(name, cwd string, agentNames []string) error {
	m.server = NewServer(m.orch)
	port, err := m.server.Start()
	if err != nil {
		return err
	}

	m.info = InstanceInfo{
		PID:       os.Getpid(),
		Port:      port,
		CWD:       cwd,
		Agents:    agentNames,
		Name:      name,
		StartTime: time.Now(),
	}

	if err := Register(m.info); err != nil {
		slog.Warn("failed to register instance", "error", err)
	}

	go m.refreshLoop()
	return nil
}

// Stop shuts down the IPC server and unregisters this instance.
func (m *Manager) Stop() {
	close(m.stopRefresh)
	if m.server != nil {
		m.server.Stop()
	}
	Unregister(m.info.PID)
}

// Instances returns a snapshot of all discovered remote instances.
func (m *Manager) Instances() []InstanceInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]InstanceInfo, len(m.instances))
	copy(out, m.instances)
	return out
}

// Info returns this instance's registration info.
func (m *Manager) Info() InstanceInfo {
	return m.info
}

// Server returns the underlying IPC server.
func (m *Manager) Server() *Server {
	return m.server
}

// DelegateRemote sends a task to an agent on a remote instance.
func (m *Manager) DelegateRemote(targetPID int, req DelegateRequest) (*DelegateResponse, error) {
	m.mu.RLock()
	var target *InstanceInfo
	for i := range m.instances {
		if m.instances[i].PID == targetPID {
			target = &m.instances[i]
			break
		}
	}
	m.mu.RUnlock()

	if target == nil {
		return nil, &InstanceNotFoundError{PID: targetPID}
	}

	client := NewClientFromInstance(*target)
	req.CallerPID = m.info.PID
	return client.Delegate(req)
}

// QueryRemoteStatus queries a remote instance's status.
func (m *Manager) QueryRemoteStatus(targetPID int) (*StatusResponse, error) {
	m.mu.RLock()
	var target *InstanceInfo
	for i := range m.instances {
		if m.instances[i].PID == targetPID {
			target = &m.instances[i]
			break
		}
	}
	m.mu.RUnlock()

	if target == nil {
		return nil, &InstanceNotFoundError{PID: targetPID}
	}

	client := NewClientFromInstance(*target)
	return client.QueryStatus()
}

func (m *Manager) refreshLoop() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	m.refresh()

	for {
		select {
		case <-ticker.C:
			m.refresh()
		case <-m.stopRefresh:
			return
		}
	}
}

func (m *Manager) refresh() {
	instances := Discover(m.info.PID)
	m.mu.Lock()
	m.instances = instances
	m.mu.Unlock()
}

// InstanceNotFoundError indicates a remote instance is no longer available.
type InstanceNotFoundError struct {
	PID int
}

func (e *InstanceNotFoundError) Error() string {
	return fmt.Sprintf("instance with PID %d not found", e.PID)
}
