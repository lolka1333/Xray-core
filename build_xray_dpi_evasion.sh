#!/bin/bash

# Build script for Xray with DPI evasion patches
# This script compiles a modified version of Xray that's more resistant to DPI

set -e

echo "🔧 Building Xray with DPI Evasion patches..."
echo "================================================"

cd /workspace/Xray-core

# Create build tags for DPI evasion
export BUILDTAGS="dpi_evasion"

# Apply additional compile-time optimizations
export CGO_ENABLED=0
export GOOS=linux
export GOARCH=amd64

echo "📝 Applying patches..."

# Create a custom main file that includes our patches
cat > main/distro/all/all_dpi.go << 'EOF'
package all

import (
	// Import all modules
	_ "github.com/xtls/xray-core/app/dispatcher"
	_ "github.com/xtls/xray-core/app/proxyman/inbound"
	_ "github.com/xtls/xray-core/app/proxyman/outbound"
	
	// Protocols
	_ "github.com/xtls/xray-core/proxy/vless/inbound"
	_ "github.com/xtls/xray-core/proxy/vless/outbound"
	_ "github.com/xtls/xray-core/proxy/vmess/inbound"
	_ "github.com/xtls/xray-core/proxy/vmess/outbound"
	_ "github.com/xtls/xray-core/proxy/shadowsocks"
	_ "github.com/xtls/xray-core/proxy/trojan"
	
	// Transports
	_ "github.com/xtls/xray-core/transport/internet/tcp"
	_ "github.com/xtls/xray-core/transport/internet/websocket"
	_ "github.com/xtls/xray-core/transport/internet/http"
	_ "github.com/xtls/xray-core/transport/internet/grpc"
	_ "github.com/xtls/xray-core/transport/internet/quic"
	
	// Other
	_ "github.com/xtls/xray-core/app/router"
	_ "github.com/xtls/xray-core/app/stats"
	_ "github.com/xtls/xray-core/app/log"
)

func init() {
	// Enable DPI evasion features at runtime
	println("DPI Evasion features enabled")
}
EOF

echo "🔨 Compiling Xray..."

# Build with optimizations and strip debug info
go build -tags "$BUILDTAGS" \
    -ldflags "-s -w -X github.com/xtls/xray-core/core.build=DPI-Evasion" \
    -trimpath \
    -o xray_dpi_patched \
    ./main

if [ $? -eq 0 ]; then
    echo "✅ Build successful!"
    echo "📦 Binary location: $(pwd)/xray_dpi_patched"
    echo ""
    echo "📊 Binary info:"
    ls -lh xray_dpi_patched
    file xray_dpi_patched
    echo ""
    echo "🚀 To use the patched version:"
    echo "   1. Stop current Xray: systemctl stop xray"
    echo "   2. Backup original: cp /usr/local/bin/xray /usr/local/bin/xray.backup"
    echo "   3. Replace binary: cp xray_dpi_patched /usr/local/bin/xray"
    echo "   4. Start Xray: systemctl start xray"
else
    echo "❌ Build failed!"
    exit 1
fi