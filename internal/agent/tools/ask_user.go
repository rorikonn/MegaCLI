package tools

import (
	"context"
	_ "embed"
	"fmt"
	"regexp"
	"strings"

	"charm.land/fantasy"
	"github.com/megacli/megacli/internal/askuser"
)

// listPatternRe matches numbered lists (1. / 2) / 3.) or bullet points
// (- / *) at the start of a line. Used to reject open-ended questions
// whose content looks like selectable options.
var listPatternRe = regexp.MustCompile(`(?m)^\s*(\d+[.)]\s|[-*]\s)`)

// embeddedSubOptionsRe matches lettered sub-options (A) / B) / - A) etc.)
// embedded inside question content. This detects the anti-pattern where
// multiple sub-questions with their own choices are crammed into one
// question.
var embeddedSubOptionsRe = regexp.MustCompile(`(?m)^\s*-?\s*[A-Z]\)\s`)

// combinedOptionLabelRe matches combined option labels like "1A, 2B, 3A"
// which indicate the LLM combined multiple independent questions into one.
var combinedOptionLabelRe = regexp.MustCompile(`^\d+[A-Z](?:,\s*\d+[A-Z])+$`)

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
				if len(q.Options) == 0 && listPatternRe.MatchString(q.Content) {
					return fantasy.NewTextErrorResponse(fmt.Sprintf(
						"question %d: open-ended questions must not contain numbered or bulleted lists "+
							"in their content — split them into separate Question objects instead", i+1,
					)), nil
				}
				if len(q.Options) > 0 && embeddedSubOptionsRe.MatchString(q.Content) {
					return fantasy.NewTextErrorResponse(fmt.Sprintf(
						"question %d: content contains embedded sub-options (A/B/C). "+
							"Each question must ask ONE thing — split multiple sub-questions into "+
							"separate Question objects, each with its own options array", i+1,
					)), nil
				}
				if len(q.Options) > 0 && hasCombinedOptionLabels(q.Options) {
					return fantasy.NewTextErrorResponse(fmt.Sprintf(
						"question %d: options contain combined labels (e.g. '1A, 2B, 3A'). "+
							"Each question must be independent — split into separate Question objects, "+
							"each with its own self-descriptive options", i+1,
					)), nil
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

// hasCombinedOptionLabels returns true if the majority of options look
// like combined labels referencing multiple sub-questions (e.g.
// "1A, 2B, 3A").
func hasCombinedOptionLabels(options []string) bool {
	if len(options) == 0 {
		return false
	}
	matches := 0
	for _, o := range options {
		if combinedOptionLabelRe.MatchString(strings.TrimSpace(o)) {
			matches++
		}
	}
	return matches > len(options)/2
}
