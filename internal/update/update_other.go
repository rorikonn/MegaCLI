//go:build !windows

package update

// ApplyPendingUpdate is a no-op on non-Windows platforms.
// Unix systems can overwrite running binaries directly.
func ApplyPendingUpdate() error {
	return nil
}

func replaceBinaryWindows(_, _ string) error {
	panic("replaceBinaryWindows called on non-Windows platform")
}
