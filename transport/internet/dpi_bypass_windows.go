//go:build windows

package internet

// GetDefaultDPIBypassOptions возвращает оптимальные настройки для обхода DPI на Windows
// Используем минимальные настройки для Windows, чтобы избежать проблем совместимости
func GetDefaultDPIBypassOptions() *DPIBypassOptions {
	return &DPIBypassOptions{
		TCPNoDelay:     false,  // Отключаем даже TCP_NODELAY для максимальной совместимости
		TCPQuickAck:    false,  // Не поддерживается на Windows
		TCPKeepAlive:   false,  // Отключаем, чтобы не вмешиваться в WebSocket
		TCPUserTimeout: 0,      // Не поддерживается на Windows
		FragmentSize:   0,      // Отключаем фрагментацию на Windows
		FragmentDelay:  0,      // Без задержек
		TTLFake:        0,      // Отключаем TTL манипуляции на Windows
		TTLReal:        0,      // Не меняем TTL
		TCPFastOpen:    false,  // Не поддерживается на Windows
		TCPSyncookie:   false,  // Не используется
		WindowSize:     0,      // Не меняем размер окна на Windows
		MSS:            0,      // Не меняем MSS на Windows
	}
}

// setSockoptInt is a platform-specific wrapper for setting socket options
func setSockoptInt(fd uintptr, level, opt, value int) error {
	// On Windows, skip all DPI bypass socket options to avoid compatibility issues
	// These options can interfere with WebSocket and other protocols
	return nil
}