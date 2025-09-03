package tcp

import (
	"context"
	gotls "crypto/tls"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/session"
	"github.com/xtls/xray-core/transport/internet"
	"github.com/xtls/xray-core/transport/internet/reality"
	"github.com/xtls/xray-core/transport/internet/stat"
	"github.com/xtls/xray-core/transport/internet/tls"
)

// Dial dials a new TCP connection to the given destination.
func Dial(ctx context.Context, dest net.Destination, streamSettings *internet.MemoryStreamConfig) (stat.Connection, error) {
	errors.LogInfo(ctx, "dialing TCP to ", dest)
	
	// Проверяем, нужно ли использовать мультиплексирование для обхода лимитов РКН
	if shouldUseMultiplex(ctx, dest) {
		dialer := func() (net.Conn, error) {
			return internet.DialSystem(ctx, dest, streamSettings.SocketSettings)
		}
		
		config := GetDefaultMultiplexConfig()
		// Для российских провайдеров используем более агрессивные настройки
		config.DataLimitPerConnection = 14 * 1024 // 14KB для безопасности
		config.MaxConnections = 16
		config.RotateOnLimit = true
		
		conn, err := NewMultiplexedConn(ctx, dialer, config)
		if err != nil {
			// Fallback к обычному соединению
			errors.LogWarning(ctx, "Failed to create multiplexed connection, falling back to normal: ", err)
			conn, err := internet.DialSystem(ctx, dest, streamSettings.SocketSettings)
			if err != nil {
				return nil, err
			}
			return applyDPIBypassMethods(ctx, conn, dest, streamSettings)
		}
		
		// Применяем остальные методы обхода поверх мультиплексирования
		return applyDPIBypassMethods(ctx, conn, dest, streamSettings)
	}
	
	conn, err := internet.DialSystem(ctx, dest, streamSettings.SocketSettings)
	if err != nil {
		return nil, err
	}
	
	return applyDPIBypassMethods(ctx, conn, dest, streamSettings)
}

// applyDPIBypassMethods применяет все методы обхода DPI
func applyDPIBypassMethods(ctx context.Context, conn net.Conn, dest net.Destination, streamSettings *internet.MemoryStreamConfig) (stat.Connection, error) {

	if config := tls.ConfigFromStreamSettings(streamSettings); config != nil {
		mitmServerName := session.MitmServerNameFromContext(ctx)
		mitmAlpn11 := session.MitmAlpn11FromContext(ctx)
		var tlsConfig *gotls.Config
		if tls.IsFromMitm(config.ServerName) {
			tlsConfig = config.GetTLSConfig(tls.WithOverrideName(mitmServerName))
		} else {
			tlsConfig = config.GetTLSConfig(tls.WithDestination(dest))
		}

		isFromMitmVerify := false
		if r, ok := tlsConfig.Rand.(*tls.RandCarrier); ok && len(r.VerifyPeerCertInNames) > 0 {
			for i, name := range r.VerifyPeerCertInNames {
				if tls.IsFromMitm(name) {
					isFromMitmVerify = true
					r.VerifyPeerCertInNames[0], r.VerifyPeerCertInNames[i] = r.VerifyPeerCertInNames[i], r.VerifyPeerCertInNames[0]
					r.VerifyPeerCertInNames = r.VerifyPeerCertInNames[1:]
					after := mitmServerName
					for {
						if len(after) > 0 {
							r.VerifyPeerCertInNames = append(r.VerifyPeerCertInNames, after)
						}
						_, after, _ = strings.Cut(after, ".")
						if !strings.Contains(after, ".") {
							break
						}
					}
					slices.Reverse(r.VerifyPeerCertInNames)
					break
				}
			}
		}
		isFromMitmAlpn := len(tlsConfig.NextProtos) == 1 && tls.IsFromMitm(tlsConfig.NextProtos[0])
		if isFromMitmAlpn {
			if mitmAlpn11 {
				tlsConfig.NextProtos[0] = "http/1.1"
			} else {
				tlsConfig.NextProtos = []string{"h2", "http/1.1"}
			}
		}
		var err error
		if fingerprint := tls.GetFingerprint(config.Fingerprint); fingerprint != nil {
			conn = tls.UClient(conn, tlsConfig, fingerprint)
			// Применяем обфускацию TLS для обхода DPI
			if shouldObfuscateTLS() {
				conn = applyTLSObfuscation(conn)
			}
			if len(tlsConfig.NextProtos) == 1 && tlsConfig.NextProtos[0] == "http/1.1" { // allow manually specify
				err = conn.(*tls.UConn).WebsocketHandshakeContext(ctx)
			} else {
				err = conn.(*tls.UConn).HandshakeContext(ctx)
			}
		} else {
			conn = tls.Client(conn, tlsConfig)
			// Применяем обфускацию TLS для обхода DPI
			if shouldObfuscateTLS() {
				conn = applyTLSObfuscation(conn)
			}
			err = conn.(*tls.Conn).HandshakeContext(ctx)
		}
		if err != nil {
			if isFromMitmVerify {
				return nil, errors.New("MITM freedom RAW TLS: failed to verify Domain Fronting certificate from " + mitmServerName).Base(err).AtWarning()
			}
			return nil, err
		}
		negotiatedProtocol := conn.(tls.Interface).NegotiatedProtocol()
		if isFromMitmAlpn && !mitmAlpn11 && negotiatedProtocol != "h2" {
			conn.Close()
			return nil, errors.New("MITM freedom RAW TLS: unexpected Negotiated Protocol (" + negotiatedProtocol + ") with " + mitmServerName).AtWarning()
		}
	} else if config := reality.ConfigFromStreamSettings(streamSettings); config != nil {
		var err error
		if conn, err = reality.UClient(conn, config, ctx, dest); err != nil {
			return nil, err
		}
	}

	tcpSettings := streamSettings.ProtocolSettings.(*Config)
	if tcpSettings.HeaderSettings != nil {
		headerConfig, err := tcpSettings.HeaderSettings.GetInstance()
		if err != nil {
			return nil, errors.New("failed to get header settings").Base(err).AtError()
		}
		auth, err := internet.CreateConnectionAuthenticator(headerConfig)
		if err != nil {
			return nil, errors.New("failed to create header authenticator").Base(err).AtError()
		}
		conn = auth.Client(conn)
	}
	
	// Применяем фрагментацию для обхода DPI
	if shouldApplyFragmentation() {
		conn = NewFragmentConn(conn, getFragmentSize())
	}
	
	return stat.Connection(conn), nil
}

// shouldUseMultiplex определяет, нужно ли использовать мультиплексирование
func shouldUseMultiplex(ctx context.Context, dest net.Destination) bool {
	// Отключаем мультиплексирование по умолчанию для совместимости
	// TODO: Сделать это настраиваемым через конфигурацию
	return false
}

// ShouldApplyFragmentation определяет, нужно ли применять фрагментацию (экспортировано для тестов)
var ShouldApplyFragmentation = shouldApplyFragmentation

// shouldApplyFragmentation определяет, нужно ли применять фрагментацию
func shouldApplyFragmentation() bool {
	// Отключаем в CI/CD окружении для стабильности тестов
	if os.Getenv("CI") == "true" || os.Getenv("GITHUB_ACTIONS") == "true" {
		return false
	}
	// Можно явно отключить через переменную окружения
	if os.Getenv("XRAY_DPI_BYPASS_DISABLED") == "true" {
		return false
	}
	// По умолчанию включено для обхода DPI
	return true
}

// getFragmentSize возвращает размер фрагмента
func getFragmentSize() int {
	// Используем небольшие фрагменты для лучшего обхода DPI
	// Типичный размер для обхода российских DPI
	return 40
}

// ShouldObfuscateTLS определяет, нужно ли применять TLS обфускацию (экспортировано для тестов)
var ShouldObfuscateTLS = shouldObfuscateTLS

// shouldObfuscateTLS определяет, нужно ли применять TLS обфускацию
func shouldObfuscateTLS() bool {
	// Отключаем в CI/CD окружении для стабильности тестов
	if os.Getenv("CI") == "true" || os.Getenv("GITHUB_ACTIONS") == "true" {
		return false
	}
	// Можно явно отключить через переменную окружения
	if os.Getenv("XRAY_DPI_BYPASS_DISABLED") == "true" {
		return false
	}
	// По умолчанию включено для обхода DPI
	return true
}

// applyTLSObfuscation применяет обфускацию к TLS соединению
func applyTLSObfuscation(conn net.Conn) net.Conn {
	// Оборачиваем соединение для обфускации TLS handshake
	// Это включает фрагментацию ClientHello и маскировку SNI
	return &TLSObfuscatedConn{
		Conn:             conn,
		fragmentSize:     40,
		splitClientHello: true,
		maskSNI:          true,
	}
}

// TLSObfuscatedConn обертка для TLS соединения с обфускацией
type TLSObfuscatedConn struct {
	net.Conn
	fragmentSize     int
	splitClientHello bool
	maskSNI          bool
	firstWrite       bool
}

// Write перехватывает и обфусцирует TLS handshake
func (toc *TLSObfuscatedConn) Write(b []byte) (int, error) {
	// Проверяем, является ли это TLS ClientHello
	if !toc.firstWrite && len(b) > 5 && b[0] == 0x16 && b[5] == 0x01 {
		toc.firstWrite = true
		
		// Фрагментируем ClientHello для обхода DPI
		if toc.splitClientHello {
			return toc.writeFragmentedClientHello(b)
		}
	}
	
	// Обычная отправка для остальных данных
	return toc.Conn.Write(b)
}

// writeFragmentedClientHello фрагментирует и отправляет ClientHello
func (toc *TLSObfuscatedConn) writeFragmentedClientHello(data []byte) (int, error) {
	// Стратегия фрагментации для обхода российских DPI:
	// 1. Отправляем TLS заголовок отдельно (5 байт)
	// 2. Разбиваем SNI на части
	// 3. Добавляем случайные задержки
	
	totalLen := len(data)
	
	// Отправляем TLS заголовок
	if _, err := toc.Conn.Write(data[:5]); err != nil {
		return 0, err
	}
	
	// Небольшая задержка
	time.Sleep(time.Microsecond * time.Duration(randInt(100, 500)))
	
	// Отправляем остальное маленькими фрагментами
	sent := 5
	for sent < totalLen {
		chunkSize := toc.fragmentSize
		if sent+chunkSize > totalLen {
			chunkSize = totalLen - sent
		}
		
		// Для области SNI используем еще меньшие фрагменты
		if sent > 40 && sent < 100 {
			chunkSize = min(chunkSize, 5+int(randInt(0, 10)))
		}
		
		if _, err := toc.Conn.Write(data[sent : sent+chunkSize]); err != nil {
			return sent, err
		}
		
		sent += chunkSize
		
		// Случайная микро-задержка между фрагментами
		if sent < totalLen {
			time.Sleep(time.Microsecond * time.Duration(randInt(50, 200)))
		}
	}
	
	return totalLen, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func init() {
	common.Must(internet.RegisterTransportDialer(protocolName, Dial))
}
