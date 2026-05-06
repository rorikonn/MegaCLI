package dialog

import (
	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/megacli/megacli/internal/ui/common"
	"github.com/megacli/megacli/internal/ui/list"
)

// AgentsID is the identifier for the agent selection dialog.
const AgentsID = "agents"

// Agents represents an agent selection dialog.
type Agents struct {
	com    *common.Common
	keyMap struct {
		Select,
		UpDown,
		Next,
		Previous,
		Close key.Binding
	}

	currentAgent    string
	availableAgents []string

	help  help.Model
	input textinput.Model
	list  *list.FilterableList
}

var _ Dialog = (*Agents)(nil)

// NewAgents creates a new agent selection dialog.
func NewAgents(com *common.Common, currentAgent string, availableAgents []string) (*Agents, error) {
	a := &Agents{
		com:             com,
		currentAgent:    currentAgent,
		availableAgents: availableAgents,
	}

	h := help.New()
	h.Styles = com.Styles.DialogHelpStyles()
	a.help = h

	a.list = list.NewFilterableList()
	a.list.Focus()
	a.list.SetSelected(0)

	a.input = textinput.New()
	a.input.SetVirtualCursor(false)
	a.input.Placeholder = "Type to filter"
	a.input.SetStyles(com.Styles.TextInput)
	a.input.Focus()

	a.keyMap.Select = key.NewBinding(
		key.WithKeys("enter", "ctrl+y"),
		key.WithHelp("enter", "confirm"),
	)
	a.keyMap.UpDown = key.NewBinding(
		key.WithKeys("up", "down"),
		key.WithHelp("↑/↓", "choose"),
	)
	a.keyMap.Next = key.NewBinding(
		key.WithKeys("down"),
	)
	a.keyMap.Previous = key.NewBinding(
		key.WithKeys("up", "ctrl+p"),
	)
	closeKey := CloseKey
	closeKey.SetHelp("esc", "cancel")
	a.keyMap.Close = closeKey

	a.setAgentItems()

	return a, nil
}

// ID implements Dialog.
func (a *Agents) ID() string {
	return AgentsID
}

// HandleMsg implements Dialog.
func (a *Agents) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, a.keyMap.Close):
			return ActionClose{}
		case key.Matches(msg, a.keyMap.Previous):
			a.list.Focus()
			if a.list.IsSelectedFirst() {
				a.list.SelectLast()
			} else {
				a.list.SelectPrev()
			}
			a.list.ScrollToSelected()
		case key.Matches(msg, a.keyMap.Next):
			a.list.Focus()
			if a.list.IsSelectedLast() {
				a.list.SelectFirst()
			} else {
				a.list.SelectNext()
			}
			a.list.ScrollToSelected()
		case key.Matches(msg, a.keyMap.Select):
			if selectedItem := a.list.SelectedItem(); selectedItem != nil {
				if item, ok := selectedItem.(*CommandItem); ok && item != nil {
					return item.Action()
				}
			}
		default:
			var cmd tea.Cmd
			a.input, cmd = a.input.Update(msg)
			value := a.input.Value()
			a.list.SetFilter(value)
			a.list.ScrollToTop()
			a.list.SetSelected(0)
			return ActionCmd{cmd}
		}
	}
	return nil
}

// Cursor returns the cursor position relative to the dialog.
func (a *Agents) Cursor() *tea.Cursor {
	return InputCursor(a.com.Styles, a.input.Cursor())
}

// Draw implements Dialog.
func (a *Agents) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := a.com.Styles
	width := max(0, min(defaultDialogMaxWidth, area.Dx()-t.Dialog.View.GetHorizontalBorderSize()))
	height := max(0, min(defaultDialogHeight, area.Dy()-t.Dialog.View.GetVerticalBorderSize()))

	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()
	heightOffset := t.Dialog.Title.GetVerticalFrameSize() + titleContentHeight +
		t.Dialog.InputPrompt.GetVerticalFrameSize() + inputContentHeight +
		t.Dialog.HelpView.GetVerticalFrameSize() +
		t.Dialog.View.GetVerticalFrameSize()

	a.input.SetWidth(max(0, innerWidth-t.Dialog.InputPrompt.GetHorizontalFrameSize()-1))

	a.list.SetSize(innerWidth, height-heightOffset)
	a.help.SetWidth(innerWidth)

	rc := NewRenderContext(t, width)
	rc.Title = "Switch Agent"
	inputView := t.Dialog.InputPrompt.Render(a.input.View())
	rc.AddPart(inputView)
	listView := t.Dialog.List.Height(a.list.Height()).Render(a.list.Render())
	rc.AddPart(listView)
	rc.Help = a.help.View(a)

	view := rc.Render()

	cur := a.Cursor()
	DrawCenterCursor(scr, area, view, cur)
	return cur
}

// ShortHelp implements help.KeyMap.
func (a *Agents) ShortHelp() []key.Binding {
	return []key.Binding{
		a.keyMap.UpDown,
		a.keyMap.Select,
		a.keyMap.Close,
	}
}

// FullHelp implements help.KeyMap.
func (a *Agents) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{a.keyMap.Select, a.keyMap.Next, a.keyMap.Previous},
		{a.keyMap.Close},
	}
}

func (a *Agents) setAgentItems() {
	items := make([]list.FilterableItem, 0, len(a.availableAgents))
	for _, name := range a.availableAgents {
		label := name
		if name == a.currentAgent {
			label = name + " (current)"
		}
		items = append(items, NewCommandItem(
			a.com.Styles,
			"agent_"+name,
			label,
			"",
			ActionSwitchAgent{AgentID: name},
		))
	}
	a.list.SetItems(items...)
	a.list.SetFilter("")
	a.list.ScrollToTop()
	a.list.SetSelected(0)
	a.input.SetValue("")
}
