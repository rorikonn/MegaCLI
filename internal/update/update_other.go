//go:build !windows

package update

func replaceBinaryWindows(_, _ string) error {
	// This function is only called on Windows via runtime.GOOS check.
	panic("replaceBinaryWindows called on non-Windows platform")
}
