package askuser

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/megacli/megacli/internal/csync"
	"github.com/megacli/megacli/internal/pubsub"
)

// Question represents a single question with multiple-choice options.
type Question struct {
	Content string   `json:"content"`
	Options []string `json:"options"`
}

// AskUserRequest is published via pub/sub to trigger the UI ask mode.
type AskUserRequest struct {
	ID        string     `json:"id"`
	SessionID string     `json:"session_id"`
	Questions []Question `json:"questions"`
}

// AskUserResponse carries the collected answers back to the blocking tool.
type AskUserResponse struct {
	Answers   []string `json:"answers"`
	Cancelled bool     `json:"cancelled"`
}

// Service manages ask_user requests, blocking the calling tool goroutine
// until the UI collects all answers.
type Service interface {
	pubsub.Subscriber[AskUserRequest]
	Request(ctx context.Context, questions []Question, sessionID string) (AskUserResponse, error)
	Respond(requestID string, answers []string)
	Cancel(requestID string)
}

type service struct {
	*pubsub.Broker[AskUserRequest]
	pendingRequests *csync.Map[string, chan AskUserResponse]
}

// NewService creates a new ask_user service.
func NewService() Service {
	return &service{
		Broker:          pubsub.NewBroker[AskUserRequest](),
		pendingRequests: csync.NewMap[string, chan AskUserResponse](),
	}
}

// Request publishes questions to the UI and blocks until answers are
// collected or the context is cancelled.
func (s *service) Request(ctx context.Context, questions []Question, sessionID string) (AskUserResponse, error) {
	req := AskUserRequest{
		ID:        uuid.NewString(),
		SessionID: sessionID,
		Questions: questions,
	}

	respCh := make(chan AskUserResponse, 1)
	s.pendingRequests.Set(req.ID, respCh)
	defer s.pendingRequests.Del(req.ID)

	s.Publish(pubsub.CreatedEvent, req)

	select {
	case <-ctx.Done():
		return AskUserResponse{}, ctx.Err()
	case resp := <-respCh:
		if resp.Cancelled {
			return resp, errors.New("user cancelled")
		}
		return resp, nil
	}
}

// Respond unblocks a pending request with the provided answers.
func (s *service) Respond(requestID string, answers []string) {
	respCh, ok := s.pendingRequests.Get(requestID)
	if ok {
		respCh <- AskUserResponse{Answers: answers}
	}
}

// Cancel unblocks a pending request indicating the user cancelled.
func (s *service) Cancel(requestID string) {
	respCh, ok := s.pendingRequests.Get(requestID)
	if ok {
		respCh <- AskUserResponse{Cancelled: true}
	}
}
