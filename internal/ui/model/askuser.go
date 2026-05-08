package model

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/megacli/megacli/internal/askuser"
	"github.com/megacli/megacli/internal/ui/styles"
)

// askUserState holds the runtime state while the user is answering questions.
type askUserState struct {
	request askuser.AskUserRequest
	current int      // Current question index.
	answers []string // One per question, "" = unanswered.

	// highlight is the currently pointed-to option (arrow cursor position).
	// -1 means no option highlighted (free-form input mode).
	highlight int

	// savedPlaceholder is the textarea placeholder before entering ask mode.
	savedPlaceholder string
}

func newAskUserState(req askuser.AskUserRequest, currentPlaceholder string) *askUserState {
	return &askUserState{
		request:          req,
		current:          0,
		answers:          make([]string, len(req.Questions)),
		highlight:        0,
		savedPlaceholder: currentPlaceholder,
	}
}

func (s *askUserState) currentQuestion() askuser.Question {
	if s.current >= 0 && s.current < len(s.request.Questions) {
		return s.request.Questions[s.current]
	}
	return askuser.Question{}
}

func (s *askUserState) allAnswered() bool {
	for _, a := range s.answers {
		if strings.TrimSpace(a) == "" {
			return false
		}
	}
	return true
}

// nextUnanswered returns the index of the next unanswered question after
// the current one, wrapping around. Returns -1 if all are answered.
func (s *askUserState) nextUnanswered() int {
	n := len(s.answers)
	for i := 1; i <= n; i++ {
		idx := (s.current + i) % n
		if strings.TrimSpace(s.answers[idx]) == "" {
			return idx
		}
	}
	return -1
}

// askUserPanelHeight returns the number of terminal lines the panel occupies.
func askUserPanelHeight(s *askUserState) int {
	if s == nil {
		return 0
	}
	// Border top (1) + progress (1) + question (1) + border bottom (1) = 4.
	height := 4
	q := s.currentQuestion()
	if len(q.Options) > 0 {
		height += len(q.Options)
	}
	return height
}

// renderAskUserProgress renders the progress indicator as small squares.
// Current question = themed color, answered = filled square (gray),
// unanswered = hollow square (gray).
func renderAskUserProgress(sty *styles.Styles, s *askUserState) string {
	as := &sty.AskUser
	var blocks []string
	for i := range s.request.Questions {
		switch {
		case i == s.current:
			if strings.TrimSpace(s.answers[i]) != "" {
				blocks = append(blocks, as.Selected.Render("■"))
			} else {
				blocks = append(blocks, as.Selected.Render("□"))
			}
		case strings.TrimSpace(s.answers[i]) != "":
			blocks = append(blocks, as.Progress.Render("■"))
		default:
			blocks = append(blocks, as.Progress.Render("□"))
		}
	}
	return strings.Join(blocks, " ")
}

// renderAskUserPanel renders the question panel displayed above the editor.
func renderAskUserPanel(sty *styles.Styles, s *askUserState, width int) string {
	if s == nil || len(s.request.Questions) == 0 {
		return ""
	}

	q := s.currentQuestion()
	as := &sty.AskUser

	var inner []string

	progress := renderAskUserProgress(sty, s)
	inner = append(inner, progress)

	questionText := as.Question.Render(q.Content)
	inner = append(inner, questionText)

	if len(q.Options) > 0 {
		for i, opt := range q.Options {
			var icon string
			if i == s.highlight {
				icon = as.Selected.Render("● ")
			} else {
				icon = as.Shortcut.Render("○ ")
			}
			num := as.Shortcut.Render(fmt.Sprintf("%d.", i+1))
			optText := as.Option.Render(opt)
			if i == s.highlight {
				optText = as.Selected.Render(opt)
			}
			inner = append(inner, fmt.Sprintf("%s%s %s", icon, num, optText))
		}
	}

	content := strings.Join(inner, "\n")
	panelWidth := width - as.Border.GetHorizontalFrameSize()
	return as.Border.Width(panelWidth).Render(content)
}

const (
	askUserPlaceholderWithOptions = "Type custom answer or use ↑↓ to select, Enter to confirm"
	askUserPlaceholderOpenEnded   = "Type your answer, Enter to confirm"
)

func askUserPlaceholderForQuestion(q askuser.Question) string {
	if len(q.Options) > 0 {
		return askUserPlaceholderWithOptions
	}
	return askUserPlaceholderOpenEnded
}

// enterAskMode activates ask mode on the UI and forces focus to the
// editor so that digit/arrow shortcuts reach handleAskUserKeyPress.
func (m *UI) enterAskMode(req askuser.AskUserRequest) {
	if len(req.Questions) == 0 {
		return
	}
	m.askUser = newAskUserState(req, m.textarea.Placeholder)
	m.textarea.Placeholder = askUserPlaceholderForQuestion(req.Questions[0])
	m.textarea.Reset()

	if m.focus != uiFocusEditor {
		m.focus = uiFocusEditor
		m.textarea.Focus() //nolint:errcheck
		m.chat.Blur()
	}

	m.updateLayoutAndSize()
}

// exitAskMode deactivates ask mode and restores the textarea.
func (m *UI) exitAskMode() {
	if m.askUser == nil {
		return
	}
	m.textarea.Placeholder = m.askUser.savedPlaceholder
	m.textarea.Reset()
	m.askUser = nil
	m.updateLayoutAndSize()
}

// confirmAskUser confirms the current highlight or typed answer and advances.
func (m *UI) confirmAskUser() {
	if m.askUser == nil {
		return
	}
	q := m.askUser.currentQuestion()

	// If textarea has text, use that as custom answer.
	if value := strings.TrimSpace(m.textarea.Value()); value != "" {
		m.askUser.answers[m.askUser.current] = value
		m.textarea.Reset()
		m.advanceAskUser()
		return
	}

	// Otherwise use the highlighted option.
	if len(q.Options) > 0 && m.askUser.highlight >= 0 && m.askUser.highlight < len(q.Options) {
		m.askUser.answers[m.askUser.current] = q.Options[m.askUser.highlight]
		m.advanceAskUser()
	}
}

// advanceAskUser moves to the next unanswered question or submits if all
// are answered.
func (m *UI) advanceAskUser() {
	if m.askUser == nil {
		return
	}
	if m.askUser.allAnswered() {
		m.com.Workspace.AskUserRespond(m.askUser.request.ID, m.askUser.answers)
		m.exitAskMode()
		return
	}
	next := m.askUser.nextUnanswered()
	if next >= 0 {
		m.askUser.current = next
		m.askUser.highlight = 0
		m.textarea.Placeholder = askUserPlaceholderForQuestion(
			m.askUser.request.Questions[next],
		)
	}
	m.textarea.Reset()
}

// askUserOptionIndex returns the 0-based option index for a digit shortcut
// keystroke, or -1 if the key doesn't match.
func askUserOptionIndex(ks string) int {
	switch ks {
	case "1", "alt+1":
		return 0
	case "2", "alt+2":
		return 1
	case "3", "alt+3":
		return 2
	case "4", "alt+4":
		return 3
	case "5", "alt+5":
		return 4
	case "6", "alt+6":
		return 5
	case "7", "alt+7":
		return 6
	case "8", "alt+8":
		return 7
	case "9", "alt+9":
		return 8
	case "0", "alt+0":
		return 9
	}
	return -1
}

// handleAskUserKeyPress processes key events when in ask mode. Returns
// true if the key was consumed and should not propagate further.
// Note: Escape is handled separately at the top of handleKeyPressMsg.
func (m *UI) handleAskUserKeyPress(msg tea.KeyPressMsg) (bool, tea.Cmd) {
	if m.askUser == nil {
		return false, nil
	}

	q := m.askUser.currentQuestion()
	ks := msg.String()

	// Only intercept digit keys when textarea is empty (not typing).
	if m.textarea.Value() == "" {
		if idx := askUserOptionIndex(ks); idx >= 0 && idx < len(q.Options) {
			if m.askUser.highlight == idx {
				// Same key pressed again — confirm.
				m.confirmAskUser()
			} else {
				m.askUser.highlight = idx
			}
			return true, nil
		}
	}

	switch ks {
	// Up = move highlight up.
	case "up":
		if len(q.Options) > 0 && m.askUser.highlight > 0 {
			m.askUser.highlight--
		}
		return true, nil

	// Down = move highlight down.
	case "down":
		if len(q.Options) > 0 && m.askUser.highlight < len(q.Options)-1 {
			m.askUser.highlight++
		}
		return true, nil

	// Left / Alt+[ = previous question.
	case "left", "alt+[":
		if m.askUser.current > 0 {
			m.askUser.current--
			m.askUser.highlight = 0
			m.textarea.Placeholder = askUserPlaceholderForQuestion(
				m.askUser.request.Questions[m.askUser.current],
			)
			m.textarea.Reset()
		}
		return true, nil

	// Right / Alt+] = next question.
	case "right", "alt+]":
		if m.askUser.current < len(m.askUser.request.Questions)-1 {
			m.askUser.current++
			m.askUser.highlight = 0
			m.textarea.Placeholder = askUserPlaceholderForQuestion(
				m.askUser.request.Questions[m.askUser.current],
			)
			m.textarea.Reset()
		}
		return true, nil
	}

	return false, nil
}
