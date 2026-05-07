package megatool

import (
	"bufio"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"charm.land/fantasy"
)

// ShowFileInnerMetadata is set on the inner ToolResponse.Metadata so the
// TUI can render line-range collapse indicators and choose between code
// highlighting and markdown rendering.
type ShowFileInnerMetadata struct {
	TotalLines int `json:"total_lines"`
	Offset     int `json:"offset"`
	Limit      int `json:"limit"`
}

const ShowFileToolName = "show_file"

//go:embed show_file.md
var showFileDescription []byte

type ShowFileParams struct {
	FilePath string `json:"file_path"`
	Offset   int    `json:"offset,omitempty"`
	Limit    int    `json:"limit,omitempty"`
}

const (
	defaultShowLimit = 200
	maxShowFileSize  = 512 * 1024 // 512KB
)

type ShowFileTool struct {
	workingDir string
	opts       fantasy.ProviderOptions
}

func NewShowFileTool(workingDir string) *ShowFileTool {
	return &ShowFileTool{workingDir: workingDir}
}

func (t *ShowFileTool) Info() fantasy.ToolInfo {
	return fantasy.ToolInfo{
		Name:        ShowFileToolName,
		Description: strings.TrimSpace(string(showFileDescription)),
		Parameters: map[string]any{
			"file_path": map[string]any{
				"type":        "string",
				"description": "The absolute or relative path to the file to display to the user",
			},
			"offset": map[string]any{
				"type":        "integer",
				"description": "Line number to start from (1-based, default 1)",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "Maximum number of lines to show (default 200)",
			},
		},
		Required: []string{"file_path"},
	}
}

func (t *ShowFileTool) ProviderOptions() fantasy.ProviderOptions {
	return t.opts
}

func (t *ShowFileTool) SetProviderOptions(opts fantasy.ProviderOptions) {
	t.opts = opts
}

func (t *ShowFileTool) Run(ctx context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
	var params ShowFileParams
	if err := json.Unmarshal([]byte(call.Input), &params); err != nil {
		return fantasy.NewTextErrorResponse("invalid parameters: " + err.Error()), nil
	}

	if params.FilePath == "" {
		return fantasy.NewTextErrorResponse("file_path is required"), nil
	}
	if params.Limit <= 0 {
		params.Limit = defaultShowLimit
	}
	if params.Offset <= 0 {
		params.Offset = 1
	}

	absPath := params.FilePath
	if !filepath.IsAbs(absPath) {
		absPath = filepath.Join(t.workingDir, absPath)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return fantasy.NewTextErrorResponse(fmt.Sprintf("file not found: %s", params.FilePath)), nil
	}
	if info.IsDir() {
		return fantasy.NewTextErrorResponse(fmt.Sprintf("%s is a directory, not a file", params.FilePath)), nil
	}
	if info.Size() > maxShowFileSize {
		return fantasy.NewTextErrorResponse(fmt.Sprintf("file too large (%d bytes, max %d)", info.Size(), maxShowFileSize)), nil
	}

	f, err := os.Open(absPath)
	if err != nil {
		return fantasy.NewTextErrorResponse("failed to open file: " + err.Error()), nil
	}
	defer f.Close()

	var allLines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		allLines = append(allLines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return fantasy.NewTextErrorResponse("error reading file: " + err.Error()), nil
	}

	if len(allLines) == 0 {
		return fantasy.NewTextResponse("File is empty or offset is beyond file length."), nil
	}

	resp := fantasy.NewTextResponse(strings.Join(allLines, "\n"))
	resp = fantasy.WithResponseMetadata(resp, ShowFileInnerMetadata{
		TotalLines: len(allLines),
		Offset:     params.Offset,
		Limit:      params.Limit,
	})

	return resp, nil
}

func (t *ShowFileTool) Mode() ResponseMode {
	return ModeDisplayOnly
}

func (t *ShowFileTool) LLMSummary(result fantasy.ToolResponse) string {
	return "File content has been displayed to the user. Do not describe or summarize the file content. Proceed with your next action if any."
}
