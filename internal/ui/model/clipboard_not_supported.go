//go:build !(darwin || linux || windows) || arm || 386 || ios || android

package model

func readClipboard(clipboardFormat) ([]byte, error) {
	return nil, errClipboardPlatformUnsupported
}

// readClipboardFileDrop is not supported on this platform.
func readClipboardFileDrop() []string {
	return nil
}
