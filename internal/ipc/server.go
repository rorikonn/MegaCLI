package ipc

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/rpc"
	"net/rpc/jsonrpc"
	"os"
	"sync"
	"time"

	"github.com/megacli/megacli/internal/orchestrator"
)

// Server exposes the local MegaCli instance's capabilities over JSON-RPC/TCP.
type Server struct {
	orch     *orchestrator.Orchestrator
	listener net.Listener
	port     int
	done     chan struct{}
	wg       sync.WaitGroup

	subscribers   map[chan RemoteEvent]struct{}
	subscribersMu sync.Mutex
}

// NewServer creates an IPC server backed by the given orchestrator.
func NewServer(orch *orchestrator.Orchestrator) *Server {
	return &Server{
		orch:        orch,
		done:        make(chan struct{}),
		subscribers: make(map[chan RemoteEvent]struct{}),
	}
}

// Start begins listening on a random local TCP port.
func (s *Server) Start() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("failed to listen: %w", err)
	}
	s.listener = ln
	s.port = ln.Addr().(*net.TCPAddr).Port

	svc := &rpcService{server: s}
	rpcServer := rpc.NewServer()
	if err := rpcServer.RegisterName("MegaCli", svc); err != nil {
		ln.Close()
		return 0, err
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		for {
			conn, err := ln.Accept()
			if err != nil {
				select {
				case <-s.done:
					return
				default:
					slog.Debug("ipc accept error", "error", err)
					continue
				}
			}
			go rpcServer.ServeCodec(jsonrpc.NewServerCodec(conn))
		}
	}()

	slog.Info("IPC server started", "port", s.port)
	return s.port, nil
}

// Port returns the port the server is listening on.
func (s *Server) Port() int {
	return s.port
}

// Stop shuts down the server and cleans up.
func (s *Server) Stop() {
	close(s.done)
	if s.listener != nil {
		s.listener.Close()
	}
	s.wg.Wait()

	s.subscribersMu.Lock()
	for ch := range s.subscribers {
		close(ch)
		delete(s.subscribers, ch)
	}
	s.subscribersMu.Unlock()
}

// Broadcast sends an event to all local subscribers.
func (s *Server) Broadcast(event RemoteEvent) {
	s.subscribersMu.Lock()
	defer s.subscribersMu.Unlock()
	for ch := range s.subscribers {
		select {
		case ch <- event:
		default:
		}
	}
}

// rpcService implements the JSON-RPC methods.
type rpcService struct {
	server *Server
}

func (svc *rpcService) Discover(_ *DiscoverRequest, reply *DiscoverResponse) error {
	agents := svc.server.orch.ListAgents()
	remoteAgents := make([]RemoteAgent, len(agents))
	for i, a := range agents {
		remoteAgents[i] = RemoteAgent{
			Name:   a.Name,
			Role:   string(a.Role),
			Status: string(a.Status),
		}
	}

	reply.Instance = InstanceInfo{
		PID:       os.Getpid(),
		Port:      svc.server.port,
		Agents:    agentNames(agents),
		StartTime: time.Now(),
	}
	reply.Agents = remoteAgents
	return nil
}

func (svc *rpcService) Delegate(req *DelegateRequest, reply *DelegateResponse) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	target, ok := svc.server.orch.GetAgent(req.TargetAgent)
	if !ok {
		reply.Success = false
		reply.Error = fmt.Sprintf("agent %q not found", req.TargetAgent)
		return nil
	}

	svc.server.Broadcast(RemoteEvent{
		Type:      EventDelegateStarted,
		Source:    req.CallerPID,
		AgentName: req.TargetAgent,
	})

	start := time.Now()
	delegateReq := orchestrator.DelegateRequest{
		FromAgent: req.CallerAgent,
		ToAgent:   req.TargetAgent,
		Task:      req.Task,
		SessionID: target.SessionID,
		Context:   req.Context,
	}

	result, err := svc.server.orch.Delegate(ctx, delegateReq)
	duration := time.Since(start)

	if err != nil {
		reply.Success = false
		reply.Error = err.Error()
		reply.Duration = duration.Milliseconds()
		return nil
	}

	reply.Success = result.Success
	reply.Result = result.Result
	reply.Error = result.Error
	reply.Duration = duration.Milliseconds()

	svc.server.Broadcast(RemoteEvent{
		Type:      EventDelegateCompleted,
		Source:    req.CallerPID,
		AgentName: req.TargetAgent,
	})

	return nil
}

func (svc *rpcService) QueryStatus(_ *StatusRequest, reply *StatusResponse) error {
	agents := svc.server.orch.ListAgents()
	remoteAgents := make([]RemoteAgent, len(agents))
	for i, a := range agents {
		remoteAgents[i] = RemoteAgent{
			Name:   a.Name,
			Role:   string(a.Role),
			Status: string(a.Status),
		}
	}

	reply.Instance = InstanceInfo{
		PID:       os.Getpid(),
		Port:      svc.server.port,
		Agents:    agentNames(agents),
		StartTime: time.Now(),
	}
	reply.Agents = remoteAgents
	reply.Busy = svc.server.orch.IsBusy()
	return nil
}

func agentNames(agents []*orchestrator.ManagedAgent) []string {
	names := make([]string, len(agents))
	for i, a := range agents {
		names[i] = a.Name
	}
	return names
}
