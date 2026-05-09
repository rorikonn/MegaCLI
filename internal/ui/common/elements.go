package common

import (
	"cmp"
	"fmt"
	"image/color"
	"strconv"
	"strings"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/megacli/megacli/internal/agent/hyper"
	"github.com/megacli/megacli/internal/config"
	"github.com/megacli/megacli/internal/home"
	"github.com/megacli/megacli/internal/ui/styles"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// PrettyPath formats a file path with home directory shortening and applies
// muted styling.
func PrettyPath(t *styles.Styles, path string, width int) string {
	formatted := home.Short(path)
	return t.Sidebar.WorkingDir.Width(width).Render(formatted)
}

// FormatReasoningEffort formats a reasoning effort level for display.
func FormatReasoningEffort(effort string) string {
	if effort == "xhigh" {
		return "X-High"
	}
	return cases.Title(language.English).String(effort)
}

// ModelContextInfo contains token usage and cost information for a model.
type ModelContextInfo struct {
	ContextUsed         int64
	ModelContext        int64
	Cost                float64
	InputTokens         int64
	OutputTokens        int64
	CacheCreationTokens int64
	CacheReadTokens     int64
}

// ModelDisplayName builds a human-readable model name with reasoning suffix.
// For models with effort-based reasoning: "claude-opus-4-7-thinking-high".
// For models with manual thinking: "claude-opus-4-5-thinking".
// When reasoning is off: just the model name.
func ModelDisplayName(model catwalk.Model, cfg config.SelectedModel) string {
	name := model.Name
	if name == "" {
		return ""
	}

	if !model.CanReason {
		return name
	}

	effort := cfg.ReasoningEffort

	// User explicitly disabled reasoning.
	if effort == "none" {
		return name
	}

	if effort == "" && !cfg.Think {
		if len(model.ReasoningLevels) > 0 && model.DefaultReasoningEffort != "" {
			return name + "-thinking-" + model.DefaultReasoningEffort
		}
		return name
	}
	if effort == "on" || (cfg.Think && effort == "") {
		return name + "-thinking"
	}
	return name + "-thinking-" + effort
}

// ModelInfo renders model information including name, provider, reasoning
// settings, and optional context usage/cost.
func ModelInfo(t *styles.Styles, modelName, smallModelName, providerName string, context *ModelContextInfo, width int, hyperCredits *int) string {
	modelIcon := t.ModelInfo.Icon.Render(styles.ModelIcon)
	modelName = t.ModelInfo.Name.Render(modelName)

	// Build first line with model name and optionally provider on the same line
	var firstLine string
	if providerName != "" {
		providerInfo := t.ModelInfo.Provider.Render(fmt.Sprintf("via %s", providerName))
		modelWithProvider := fmt.Sprintf("%s %s %s", modelIcon, modelName, providerInfo)

		// Check if it fits on one line
		if lipgloss.Width(modelWithProvider) <= width {
			firstLine = modelWithProvider
		} else {
			// If it doesn't fit, put provider on next line
			firstLine = fmt.Sprintf("%s %s", modelIcon, modelName)
		}
	} else {
		firstLine = fmt.Sprintf("%s %s", modelIcon, modelName)
	}

	parts := []string{firstLine}

	// If provider didn't fit on first line, add it as second line
	if providerName != "" && !strings.Contains(firstLine, "via") {
		providerInfo := fmt.Sprintf("via %s", providerName)
		parts = append(parts, t.ModelInfo.ProviderFallback.Render(providerInfo))
	}

	if smallModelName != "" {
		smallLine := fmt.Sprintf("%s %s", styles.ModelIcon, smallModelName)
		parts = append(parts, t.ModelInfo.Reasoning.Render(smallLine))
	}

	if context != nil {
		formattedInfo := formatTokensAndCost(t, context.ContextUsed, context.ModelContext, context.Cost)
		parts = append(parts, lipgloss.NewStyle().PaddingLeft(2).Render(formattedInfo))

		detailLine := formatTokenDetails(t, context)
		if detailLine != "" {
			parts = append(parts, lipgloss.NewStyle().PaddingLeft(2).Render(detailLine))
		}
	}

	if providerName == hyper.DisplayName && hyperCredits != nil {
		hcInfo := t.ModelInfo.HypercreditIcon.Render(styles.HypercreditIcon)
		hcInfo += " "
		hcInfo += t.ModelInfo.HypercreditText.Render(fmt.Sprintf("%s Hypercredits", FormatCredits(*hyperCredits)))
		parts = append(parts, "", hcInfo)
	}

	return lipgloss.NewStyle().Width(width).Render(
		lipgloss.JoinVertical(lipgloss.Left, parts...),
	)
}

// formatTokensAndCost formats token usage as current/max context with a
// percentage indicator.
func formatTokensAndCost(t *styles.Styles, tokens, contextWindow int64, _ float64) string {
	current := compactTokens(tokens)
	max := compactTokens(contextWindow)

	percentage := (float64(tokens) / float64(contextWindow)) * 100

	contextInfo := t.ModelInfo.TokenCount.Render(fmt.Sprintf("%s/%s", current, max))
	formattedPercentage := t.ModelInfo.TokenPercentage.Render(fmt.Sprintf("%d%%", int(percentage)))
	result := fmt.Sprintf("%s %s", formattedPercentage, contextInfo)
	if percentage > 80 {
		result = fmt.Sprintf("%s %s", styles.LSPWarningIcon, result)
	}

	return result
}

// formatTokenDetails renders per-line breakdown of Input/Output/Cache tokens.
func formatTokenDetails(t *styles.Styles, ctx *ModelContextInfo) string {
	if ctx.InputTokens == 0 && ctx.OutputTokens == 0 {
		return ""
	}

	dim := t.ModelInfo.TokenCount
	label := t.ModelInfo.TokenPercentage

	lines := []string{
		label.Render("Input   ") + dim.Render(compactTokens(ctx.InputTokens)),
		label.Render("Output  ") + dim.Render(compactTokens(ctx.OutputTokens)),
	}

	if ctx.CacheCreationTokens > 0 || ctx.CacheReadTokens > 0 {
		lines = append(lines,
			label.Render("C.Hit   ")+dim.Render(compactTokens(ctx.CacheReadTokens)),
			label.Render("C.Write ")+dim.Render(compactTokens(ctx.CacheCreationTokens)),
		)
	}

	return strings.Join(lines, "\n")
}

func compactTokens(n int64) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

// FormatCredits formats an integer with comma separators for thousands.
func FormatCredits(n int) string {
	s := strconv.FormatInt(int64(n), 10)
	if n < 1000 {
		return s
	}
	// Calculate how many digits before the first comma.
	firstGroup := len(s) % 3
	if firstGroup == 0 {
		firstGroup = 3
	}
	var b []byte
	for i := 0; i < len(s); i++ {
		if i > 0 && i == firstGroup {
			b = append(b, ',')
			firstGroup += 3
		}
		b = append(b, s[i])
	}
	return string(b)
}

// StatusOpts defines options for rendering a status line with icon, title,
// description, and optional extra content.
type StatusOpts struct {
	Icon             string // if empty no icon will be shown
	Title            string
	TitleColor       color.Color
	Description      string
	DescriptionColor color.Color
	ExtraContent     string // additional content to append after the description
}

// Status renders a status line with icon, title, description, and extra
// content. The description is truncated if it exceeds the available width.
func Status(t *styles.Styles, opts StatusOpts, width int) string {
	icon := opts.Icon
	title := opts.Title
	description := opts.Description

	titleColor := cmp.Or(opts.TitleColor, t.Resource.DefaultTitleFg)
	descriptionColor := cmp.Or(opts.DescriptionColor, t.Resource.DefaultDescFg)

	title = t.Resource.RowTitleBase.Foreground(titleColor).Render(title)

	if description != "" {
		extraContentWidth := lipgloss.Width(opts.ExtraContent)
		if extraContentWidth > 0 {
			extraContentWidth += 1
		}
		description = ansi.Truncate(description, width-lipgloss.Width(icon)-lipgloss.Width(title)-2-extraContentWidth, "…")
		description = t.Resource.RowDescBase.Foreground(descriptionColor).Render(description)
	}

	var content []string
	if icon != "" {
		content = append(content, icon)
	}
	content = append(content, title)
	if description != "" {
		content = append(content, description)
	}
	if opts.ExtraContent != "" {
		content = append(content, opts.ExtraContent)
	}

	return strings.Join(content, " ")
}

// Section renders a section header with a title and a horizontal line filling
// the remaining width.
func Section(t *styles.Styles, text string, width int, info ...string) string {
	char := styles.SectionSeparator
	length := lipgloss.Width(text) + 1
	remainingWidth := width - length

	var infoText string
	if len(info) > 0 {
		infoText = strings.Join(info, " ")
		if len(infoText) > 0 {
			infoText = " " + infoText
			remainingWidth -= lipgloss.Width(infoText)
		}
	}

	text = t.Section.Title.Render(text)
	if remainingWidth > 0 {
		text = text + " " + t.Section.Line.Render(strings.Repeat(char, remainingWidth)) + infoText
	}
	return text
}

// DialogTitle renders a dialog title with a decorative line filling the
// remaining width.
func DialogTitle(t *styles.Styles, title string, width int, fromColor, toColor color.Color) string {
	char := "╱"
	length := lipgloss.Width(title) + 1
	remainingWidth := width - length
	if remainingWidth > 0 {
		lines := strings.Repeat(char, remainingWidth)
		lines = styles.ApplyForegroundGrad(t.Dialog.TitleLineBase, lines, fromColor, toColor)
		title = title + " " + lines
	}
	return title
}
