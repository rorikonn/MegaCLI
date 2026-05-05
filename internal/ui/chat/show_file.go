package chat

import (
	"encoding/json"
	"fmt"

	"github.com/megacli/megacli/internal/fsext"
	"github.com/megacli/megacli/internal/megatool"
	"github.com/megacli/megacli/internal/message"
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

	body := toolOutputCodeContent(sty, params.FilePath, content, params.Offset, cappedWidth, opts.ExpandedContent)
	return joinToolParts(header, body)
}
