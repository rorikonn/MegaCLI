package dialog

import (
	"fmt"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/megacli/megacli/internal/fsext"
	"github.com/megacli/megacli/internal/ui/common"
)

// ReviewID is the identifier for the review dialog.
const ReviewID = "review"

// ReviewFile holds the data needed to display a diff for one file.
type ReviewFile struct {
	Path       string
	OldContent string
	NewContent string
	Additions  int
	Deletions  int
}

// Review is a full-screen dialog that shows all file changes in the current
// session as side-by-side diffs.
type Review struct {
	com   *common.Common
	files []ReviewFile

	currentFileIdx int
	yOffset        int
	xOffset        int

	help   help.Model
	keyMap reviewKeyMap
}

type reviewKeyMap struct {
	NextFile key.Binding
	PrevFile key.Binding
	Down     key.Binding
	Up       key.Binding
	Left     key.Binding
	Right    key.Binding
	PageDown key.Binding
	PageUp   key.Binding
	Home     key.Binding
	End      key.Binding
	Close    key.Binding
}

func (k reviewKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		k.PrevFile, k.NextFile, k.Down, k.Up, k.Left, k.Right, k.Close,
	}
}

func (k reviewKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{k.ShortHelp()}
}

var _ Dialog = (*Review)(nil)

// NewReview creates a new Review dialog with the given files.
func NewReview(com *common.Common, files []ReviewFile) *Review {
	h := help.New()
	h.Styles = com.Styles.DialogHelpStyles()

	return &Review{
		com:   com,
		files: files,
		help:  h,
		keyMap: reviewKeyMap{
			NextFile: key.NewBinding(
				key.WithKeys("right", "l", "tab"),
				key.WithHelp("→/l", "next file"),
			),
			PrevFile: key.NewBinding(
				key.WithKeys("left", "h", "shift+tab"),
				key.WithHelp("←/h", "prev file"),
			),
			Down: key.NewBinding(
				key.WithKeys("down", "j"),
				key.WithHelp("↓/j", "scroll down"),
			),
			Up: key.NewBinding(
				key.WithKeys("up", "k"),
				key.WithHelp("↑/k", "scroll up"),
			),
			Left: key.NewBinding(
				key.WithKeys("shift+left", "H"),
				key.WithHelp("shift+←", "scroll left"),
			),
			Right: key.NewBinding(
				key.WithKeys("shift+right", "L"),
				key.WithHelp("shift+→", "scroll right"),
			),
			PageDown: key.NewBinding(
				key.WithKeys("pgdown", "f", "d"),
				key.WithHelp("f/pgdn", "page down"),
			),
			PageUp: key.NewBinding(
				key.WithKeys("pgup", "b", "u"),
				key.WithHelp("b/pgup", "page up"),
			),
			Home: key.NewBinding(
				key.WithKeys("g", "home"),
				key.WithHelp("g", "top"),
			),
			End: key.NewBinding(
				key.WithKeys("G", "end"),
				key.WithHelp("G", "bottom"),
			),
			Close: key.NewBinding(
				key.WithKeys("esc", "alt+esc", "q"),
				key.WithHelp("esc/q", "close"),
			),
		},
	}
}

// ID implements [Dialog].
func (r *Review) ID() string { return ReviewID }

const horizontalScrollAmount = 5

// HandleMsg implements [Dialog].
func (r *Review) HandleMsg(msg tea.Msg) Action {
	km := r.keyMap
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return nil
	}

	switch {
	case key.Matches(keyMsg, km.Close):
		return ActionClose{}
	case key.Matches(keyMsg, km.NextFile):
		if r.currentFileIdx < len(r.files)-1 {
			r.currentFileIdx++
			r.yOffset = 0
			r.xOffset = 0
		}
	case key.Matches(keyMsg, km.PrevFile):
		if r.currentFileIdx > 0 {
			r.currentFileIdx--
			r.yOffset = 0
			r.xOffset = 0
		}
	case key.Matches(keyMsg, km.Down):
		r.yOffset++
	case key.Matches(keyMsg, km.Up):
		r.yOffset = max(0, r.yOffset-1)
	case key.Matches(keyMsg, km.Left):
		r.xOffset = max(0, r.xOffset-horizontalScrollAmount)
	case key.Matches(keyMsg, km.Right):
		r.xOffset += horizontalScrollAmount
	case key.Matches(keyMsg, km.PageDown):
		r.yOffset += 20
	case key.Matches(keyMsg, km.PageUp):
		r.yOffset = max(0, r.yOffset-20)
	case key.Matches(keyMsg, km.Home):
		r.yOffset = 0
	case key.Matches(keyMsg, km.End):
		r.yOffset = 99999
	}

	return nil
}

// Draw implements [Dialog].
func (r *Review) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := r.com.Styles
	width := area.Dx()
	height := area.Dy()

	if len(r.files) == 0 {
		content := t.Dialog.View.Width(width).Render("No changes to review")
		DrawCenter(scr, area, content)
		return nil
	}

	file := r.files[r.currentFileIdx]

	// Header: file name + index indicator.
	relPath := file.Path
	if cwd := r.com.Workspace.WorkingDir(); cwd != "" {
		if rel, err := filepath.Rel(cwd, file.Path); err == nil {
			relPath = rel
		}
	}
	relPath = fsext.PrettyPath(relPath)

	additions := t.Files.Additions.Render(fmt.Sprintf("+%d", file.Additions))
	deletions := t.Files.Deletions.Render(fmt.Sprintf("-%d", file.Deletions))
	indicator := fmt.Sprintf("[%d/%d]", r.currentFileIdx+1, len(r.files))
	headerText := fmt.Sprintf(" %s  %s %s  %s", relPath, additions, deletions, indicator)
	headerText = ansi.Truncate(headerText, width-2, "…")
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Width(width)
	header := headerStyle.Render(headerText)

	// Help bar at the bottom.
	helpView := r.help.View(r.keyMap)

	// Calculate content area.
	headerHeight := lipgloss.Height(header)
	helpHeight := lipgloss.Height(helpView)
	contentHeight := height - headerHeight - helpHeight - 1
	if contentHeight < 1 {
		contentHeight = 1
	}

	// Render the split diff.
	formatter := common.DiffFormatter(t).
		Split().
		Before(file.Path, file.OldContent).
		After(file.Path, file.NewContent).
		Width(width).
		Height(contentHeight).
		YOffset(r.yOffset).
		XOffset(r.xOffset)

	diffContent := formatter.String()

	// Compose the full view.
	parts := []string{
		header,
		diffContent,
		helpView,
	}
	view := strings.Join(parts, "\n")
	view = lipgloss.NewStyle().
		Width(width).
		Height(height).
		Render(view)

	uv.NewStyledString(view).Draw(scr, area)
	return nil
}
