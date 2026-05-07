package tools

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	"charm.land/fantasy"
	"github.com/megacli/megacli/internal/askuser"
)

//go:embed ask_user.md
var askUserDescription []byte

const AskUserToolName = "ask_user"

// AskUserParams defines the parameters for the ask_user tool.
type AskUserParams struct {
	Questions []askuser.Question `json:"questions" description:"List of questions to ask the user"`
}

// NewAskUserTool creates the ask_user tool.
func NewAskUserTool(svc askuser.Service) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		AskUserToolName,
		FirstLineDescription(askUserDescription),
		func(ctx context.Context, params AskUserParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if len(params.Questions) == 0 {
				return fantasy.NewTextErrorResponse("at least one question is required"), nil
			}
			for i, q := range params.Questions {
				if q.Content == "" {
					return fantasy.NewTextErrorResponse(fmt.Sprintf("question %d has empty content", i+1)), nil
				}
				if len(q.Options) > 10 {
					return fantasy.NewTextErrorResponse(fmt.Sprintf("question %d has more than 10 options", i+1)), nil
				}
			}

			sessionID := GetSessionFromContext(ctx)
			resp, err := svc.Request(ctx, params.Questions, sessionID)
			if err != nil {
				r := fantasy.NewTextErrorResponse("User cancelled the questions.")
				r.StopTurn = true
				return r, nil
			}

			var sb strings.Builder
			for i, q := range params.Questions {
				answer := ""
				if i < len(resp.Answers) {
					answer = resp.Answers[i]
				}
				sb.WriteString(fmt.Sprintf("Q%d: %s\nA%d: %s\n\n", i+1, q.Content, i+1, answer))
			}

			return fantasy.NewTextResponse(sb.String()), nil
		})
}
