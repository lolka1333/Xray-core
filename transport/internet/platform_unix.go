//go:build !windows

package internet

// isWindows returns false on non-Windows platforms
func isWindows() bool {
	return false
}