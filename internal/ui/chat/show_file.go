package chat

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/megacli/megacli/internal/fsext"
	"github.com/megacli/megacli/internal/megatool"
	"github.com/megacli/megacli/internal/message"
	"github.com/megacli/megacli/internal/stringext"
	"github.com/megacli/megacli/internal/ui/common"
	"github.com/megacli/megacli/internal/ui/styles"
)

type ShowFileToolMessageItem struct {
	*baseToolMessageItem
}

var _ ToolMessageItem = (*ShowFileToolMessageItem)(nil)

func NewShowFileToolMessageItem(
	sty *styles.Styles,
	toolCall message.ToolCall,
	result *message.ToolResult,
	canceled bool,
) ToolMessageItem {
	return newBaseToolMessageItem(sty, toolCall, result, &ShowFileToolRenderContext{}, canceled)
}

type ShowFileToolRenderContext struct{}

func isMarkdownFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".md" || ext == ".markdown"
}

func (s *ShowFileToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	cappedWidth := cappedMessageWidth(width)
	if opts.IsPending() {
		return pendingTool(sty, "Show File", opts.Anim, opts.Compact)
	}

	var params megatool.ShowFileParams
	if err := json.Unmarshal([]byte(opts.ToolCall.Input), &params); err != nil {
		return toolErrorContent(sty, &message.ToolResult{Content: "Invalid parameters"}, cappedWidth)
	}

	file := fsext.PrettyPath(params.FilePath)
	toolParams := []string{file}
	if params.Limit != 0 {
		toolParams = append(toolParams, "limit", fmt.Sprintf("%d", params.Limit))
	}
	if params.Offset != 0 {
		toolParams = append(toolParams, "offset", fmt.Sprintf("%d", params.Offset))
	}

	header := toolHeader(sty, opts.Status, "Show File", cappedWidth, opts.Compact, toolParams...)
	if opts.Compact {
		return header
	}

	if earlyState, ok := toolEarlyStateContent(sty, opts, cappedWidth); ok {
		return joinToolParts(header, earlyState)
	}

	if !opts.HasResult() {
		return header
	}

	var meta megatool.DisplayMetadata
	content := ""
	if err := json.Unmarshal([]byte(opts.Result.Metadata), &meta); err == nil && meta.Content != "" {
		content = meta.Content
	}

	if content == "" {
		return header
	}

	var inner megatool.ShowFileInnerMetadata
	if meta.InnerMetadata != "" {
		_ = json.Unmarshal([]byte(meta.InnerMetadata), &inner)
	}

	body := showFileBorderedContent(sty, params.FilePath, content, inner, cappedWidth, opts.ExpandedContent)
	return joinToolParts(header, body)
}

// showFileBorderedContent renders the file content inside a bordered frame
// with an optional title and collapsed-range indicators.
func showFileBorderedContent(
	sty *styles.Styles,
	filePath string,
	content string,
	inner megatool.ShowFileInnerMetadata,
	width int,
	expanded bool,
) string {
	content = stringext.NormalizeSpace(content)
	allLines := strings.Split(content, "\n")
	totalLines := len(allLines)

	if inner.TotalLines > 0 {
		totalLines = inner.TotalLines
	}
	offset := inner.Offset
	if offset <= 0 {
		offset = 1
	}
	limit := inner.Limit
	if limit <= 0 {
		limit = totalLines
	}

	// Determine the visible range (1-based inclusive).
	rangeStart := offset
	rangeEnd := min(offset+limit-1, totalLines)

	// Select lines to display.
	var displayLines []string
	if expanded {
		displayLines = allLines
	} else {
		startIdx := rangeStart - 1
		endIdx := min(rangeEnd, len(allLines))
		if startIdx < 0 {
			startIdx = 0
		}
		if startIdx < len(allLines) {
			displayLines = allLines[startIdx:endIdx]
		}
	}

	// Border eats some width: left border (2) + paddingLeft (1).
	borderStyle := sty.Tool.ShowFileBorder
	borderHPad := borderStyle.GetHorizontalBorderSize() + borderStyle.GetHorizontalPadding()
	innerWidth := width - toolBodyLeftPaddingTotal - borderHPad
	if innerWidth < 20 {
		innerWidth = 20
	}

	var bodyParts []string

	// Collapsed indicator above.
	linesAbove := rangeStart - 1
	if !expanded && linesAbove > 0 {
		hint := sty.Tool.ShowFileCollapsed.Render(
			fmt.Sprintf("… %d lines above …", linesAbove))
		bodyParts = append(bodyParts, hint)
	}

	// Render file content.
	if isMarkdownFile(filePath) {
		bodyParts = append(bodyParts, renderShowFileMarkdown(sty, strings.Join(displayLines, "\n"), innerWidth))
	} else {
		lineOffset := 0
		if !expanded {
			lineOffset = rangeStart - 1
		}
		bodyParts = append(bodyParts, renderShowFileCode(sty, filePath, displayLines, lineOffset, innerWidth))
	}

	// Collapsed indicator below.
	linesBelow := totalLines - rangeEnd
	if !expanded && linesBelow > 0 {
		hint := sty.Tool.ShowFileCollapsed.Render(
			fmt.Sprintf("… %d lines below …", linesBelow))
		bodyParts = append(bodyParts, hint)
	}

	innerContent := strings.Join(bodyParts, "\n")

	// Build the title line.
	prettyFile := fsext.PrettyPath(filePath)
	title := sty.Tool.ShowFileTitle.Render(prettyFile)

	bordered := borderStyle.
		Width(width - toolBodyLeftPaddingTotal).
		Render(innerContent)

	result := lipgloss.JoinVertical(lipgloss.Left, title, bordered)
	return sty.Tool.Body.Render(result)
}

// renderShowFileMarkdown renders content through glamour.
func renderShowFileMarkdown(sty *styles.Styles, content string, width int) string {
	if width > maxTextWidth {
		width = maxTextWidth
	}
	renderer := common.MarkdownRenderer(sty, width)
	rendered, err := renderer.Render(content)
	if err != nil {
		return content
	}
	return strings.TrimRight(rendered, "\n")
}

// renderShowFileCode renders content with syntax highlighting and line numbers.
func renderShowFileCode(sty *styles.Styles, path string, lines []string, offset, width int) string {
	bg := sty.Tool.ContentCodeBg
	highlighted, _ := common.SyntaxHighlight(sty, strings.Join(lines, "\n"), path, bg)
	highlightedLines := strings.Split(highlighted, "\n")

	maxLineNumber := len(lines) + offset
	maxDigits := getDigits(maxLineNumber)
	numFmt := fmt.Sprintf("%%%dd", maxDigits)

	codeWidth := width - maxDigits

	var out []string
	for i, ln := range highlightedLines {
		lineNum := sty.Tool.ContentLineNumber.Render(fmt.Sprintf(numFmt, i+1+offset))
		ln = ansi.Truncate(ln, codeWidth-sty.Tool.ContentCodeLine.GetHorizontalPadding(), "…")
		codeLine := sty.Tool.ContentCodeLine.
			Width(codeWidth).
			Render(ln)
		out = append(out, lipgloss.JoinHorizontal(lipgloss.Left, lineNum, codeLine))
	}

	return strings.Join(out, "\n")
}
