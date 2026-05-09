package common

import (
	"fmt"
	"image"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/atotto/clipboard"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/megacli/megacli/internal/config"
	"github.com/megacli/megacli/internal/ui/styles"
	"github.com/megacli/megacli/internal/ui/util"
	"github.com/megacli/megacli/internal/workspace"
)

// MaxAttachmentSize defines the maximum allowed size for file attachments (5 MB).
const MaxAttachmentSize = int64(5 * 1024 * 1024)

// LargeAttachmentThreshold is the size above which a warning is shown
// to the user when attaching a file (256 KB).
const LargeAttachmentThreshold = int64(256 * 1024)

// FormatFileSize returns a human-readable string for a byte count
// (e.g. "1.5 MB", "320 KB").
func FormatFileSize(size int64) string {
	switch {
	case size >= 1024*1024:
		return fmt.Sprintf("%.1f MB", float64(size)/(1024*1024))
	case size >= 1024:
		return fmt.Sprintf("%.1f KB", float64(size)/1024)
	default:
		return fmt.Sprintf("%d B", size)
	}
}

// AllowedImageTypes defines the permitted image file types.
var AllowedImageTypes = []string{".jpg", ".jpeg", ".png"}

// AttachmentTypeInfo describes a recognized attachment file type.
type AttachmentTypeInfo struct {
	// NeedsImageSupport indicates the model must have SupportsImages
	// enabled to accept this file type.
	NeedsImageSupport bool
}

// AllowedAttachmentTypes maps lowercase file extensions to their attachment
// metadata. Files whose extension appears here are eligible for attachment
// when dragged or pasted; all others are treated as plain path text.
var AllowedAttachmentTypes = map[string]AttachmentTypeInfo{
	".jpg":  {NeedsImageSupport: true},
	".jpeg": {NeedsImageSupport: true},
	".png":  {NeedsImageSupport: true},
	".gif":  {NeedsImageSupport: true},
	".webp": {NeedsImageSupport: true},
	".pdf":  {NeedsImageSupport: true},
}

// LookupAttachmentType checks whether ext (case-insensitive) is a recognized
// attachment type. Returns the type info and true if found.
func LookupAttachmentType(ext string) (AttachmentTypeInfo, bool) {
	info, ok := AllowedAttachmentTypes[strings.ToLower(ext)]
	return info, ok
}

// Common defines common UI options and configurations.
type Common struct {
	Workspace workspace.Workspace
	Styles    *styles.Styles
}

// Config returns the pure-data configuration associated with this [Common] instance.
func (c *Common) Config() *config.Config {
	return c.Workspace.Config()
}

// DefaultCommon returns the default common UI configurations. When the
// workspace has a large model selected, the theme is chosen based on its
// provider; otherwise the default theme is used.
func DefaultCommon(ws workspace.Workspace) *Common {
	s := styles.ThemeForProvider(largeModelProviderID(ws))
	return &Common{
		Workspace: ws,
		Styles:    &s,
	}
}

// largeModelProviderID returns the provider ID of the currently selected
// large model, or the empty string if none is set or the workspace is nil.
func largeModelProviderID(ws workspace.Workspace) string {
	if ws == nil {
		return ""
	}
	cfg := ws.Config()
	if cfg == nil {
		return ""
	}
	return cfg.Models[config.SelectedModelTypeLarge].Provider
}

// IsHyper reports whether the currently selected large model is provided
// by Hyper.
func (c *Common) IsHyper() bool {
	return largeModelProviderID(c.Workspace) == "hyper"
}

// CenterRect returns a new [Rectangle] centered within the given area with the
// specified width and height.
func CenterRect(area uv.Rectangle, width, height int) uv.Rectangle {
	centerX := area.Min.X + area.Dx()/2
	centerY := area.Min.Y + area.Dy()/2
	minX := centerX - width/2
	minY := centerY - height/2
	maxX := minX + width
	maxY := minY + height
	return image.Rect(minX, minY, maxX, maxY)
}

// BottomLeftRect returns a new [Rectangle] positioned at the bottom-left within the given area with the
// specified width and height.
func BottomLeftRect(area uv.Rectangle, width, height int) uv.Rectangle {
	minX := area.Min.X
	maxX := minX + width
	maxY := area.Max.Y
	minY := maxY - height
	return image.Rect(minX, minY, maxX, maxY)
}

// IsFileTooBig checks if the file at the given path exceeds the specified size
// limit.
func IsFileTooBig(filePath string, sizeLimit int64) (bool, error) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return false, fmt.Errorf("error getting file info: %w", err)
	}

	if fileInfo.Size() > sizeLimit {
		return true, nil
	}

	return false, nil
}

// CopyToClipboard copies the given text to the clipboard using both OSC 52
// (terminal escape sequence) and native clipboard for maximum compatibility.
// Returns a command that reports success to the user with the given message.
func CopyToClipboard(text, successMessage string) tea.Cmd {
	return CopyToClipboardWithCallback(text, successMessage, nil)
}

// CopyToClipboardWithCallback copies text to clipboard and executes a callback
// before showing the success message.
// This is useful when you need to perform additional actions like clearing UI state.
func CopyToClipboardWithCallback(text, successMessage string, callback tea.Cmd) tea.Cmd {
	return tea.Sequence(
		tea.SetClipboard(text),
		func() tea.Msg {
			_ = clipboard.WriteAll(text)
			return nil
		},
		callback,
		util.ReportInfo(successMessage),
	)
}
