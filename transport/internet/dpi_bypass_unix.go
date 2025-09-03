//go:build !windows

package internet

import "syscall"

// GetDefaultDPIBypassOptions возвращает оптимальные настройки для обхода российских DPI на Unix-системах
// На Linux и других Unix-системах используем полный набор техник обхода
func GetDefaultDPIBypassOptions() *DPIBypassOptions {
	return &DPIBypassOptions{
		TCPNoDelay:     true,
		TCPQuickAck:    true,
		TCPKeepAlive:   true,
		TCPUserTimeout: 30000, // 30 секунд
		FragmentSize:   40,
		FragmentDelay:  100,
		TTLFake:        3,  // Низкий TTL для первых пакетов (не дойдут до сервера)
		TTLReal:        64, // Нормальный TTL
		TCPFastOpen:    true,
		TCPSyncookie:   false,
		WindowSize:     65535,
		MSS:            1360, // Меньше стандартного для лучшей фрагментации
	}
}

// setSockoptInt is a platform-specific wrapper for setting socket options
func setSockoptInt(fd uintptr, level, opt, value int) error {
	return syscall.SetsockoptInt(int(fd), level, opt, value)
}