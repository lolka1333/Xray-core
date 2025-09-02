//go:build windows

package internet

import "syscall"

// setSockoptInt is a platform-specific wrapper for setting socket options
func setSockoptInt(fd uintptr, level, opt, value int) error {
	return syscall.SetsockoptInt(syscall.Handle(fd), level, opt, value)
}