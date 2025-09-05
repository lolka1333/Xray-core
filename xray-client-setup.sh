#!/bin/bash

# Xray Client Setup Script for Linux/macOS
# Configures local proxy with anti-censorship settings

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}Xray Client Setup${NC}"
echo -e "${GREEN}========================================${NC}"

# Detect OS
if [[ "$OSTYPE" == "linux-gnu"* ]]; then
    OS="linux"
elif [[ "$OSTYPE" == "darwin"* ]]; then
    OS="macos"
else
    echo -e "${RED}Unsupported OS${NC}"
    exit 1
fi

# Request server details
read -p "Enter server IP: " SERVER_IP
read -p "Enter UUID: " UUID
read -p "Enter Public Key: " PUBLIC_KEY
read -p "Enter port (8443/2087/2053) [8443]: " PORT
PORT=${PORT:-8443}

# Install Xray
if [ "$OS" == "linux" ]; then
    echo -e "${YELLOW}Installing Xray for Linux...${NC}"
    bash -c "$(curl -L https://github.com/XTLS/Xray-install/raw/main/install-release.sh)" @ install
    CONFIG_PATH="/usr/local/etc/xray/config.json"
    SERVICE_CMD="systemctl"
elif [ "$OS" == "macos" ]; then
    echo -e "${YELLOW}Installing Xray for macOS...${NC}"
    if ! command -v brew &> /dev/null; then
        echo -e "${RED}Homebrew not found. Please install Homebrew first.${NC}"
        exit 1
    fi
    brew install xray
    CONFIG_PATH="/usr/local/etc/xray/config.json"
    SERVICE_CMD="brew services"
fi

# Create client configuration
echo -e "${YELLOW}Creating client configuration...${NC}"

if [ "$PORT" == "8443" ]; then
    # REALITY configuration
    cat > "$CONFIG_PATH" <<EOF
{
  "log": {
    "loglevel": "warning"
  },
  "routing": {
    "domainStrategy": "IPIfNonMatch",
    "rules": [
      {
        "type": "field",
        "domain": [
          "geosite:cn",
          "geosite:private",
          "domain:ru",
          "domain:рф",
          "domain:by"
        ],
        "outboundTag": "direct"
      },
      {
        "type": "field",
        "ip": [
          "geoip:cn",
          "geoip:ru",
          "geoip:private"
        ],
        "outboundTag": "direct"
      }
    ]
  },
  "inbounds": [
    {
      "port": 1080,
      "protocol": "socks",
      "settings": {
        "auth": "noauth",
        "udp": true
      }
    },
    {
      "port": 1087,
      "protocol": "http"
    }
  ],
  "outbounds": [
    {
      "protocol": "vless",
      "settings": {
        "vnext": [
          {
            "address": "${SERVER_IP}",
            "port": ${PORT},
            "users": [
              {
                "id": "${UUID}",
                "flow": "xtls-rprx-vision",
                "encryption": "none"
              }
            ]
          }
        ]
      },
      "streamSettings": {
        "network": "tcp",
        "security": "reality",
        "realitySettings": {
          "serverName": "microsoft.com",
          "fingerprint": "chrome",
          "show": false,
          "publicKey": "${PUBLIC_KEY}",
          "shortId": "6ba85179e30d4fc2"
        }
      },
      "tag": "proxy"
    },
    {
      "protocol": "freedom",
      "tag": "direct"
    },
    {
      "protocol": "blackhole",
      "tag": "block"
    }
  ],
  "dns": {
    "servers": [
      "8.8.8.8",
      "1.1.1.1",
      "localhost"
    ]
  }
}
EOF

elif [ "$PORT" == "2087" ]; then
    # WebSocket configuration
    read -p "Enter WebSocket path: " WS_PATH
    cat > "$CONFIG_PATH" <<EOF
{
  "log": {
    "loglevel": "warning"
  },
  "inbounds": [
    {
      "port": 1080,
      "protocol": "socks",
      "settings": {
        "auth": "noauth",
        "udp": true
      }
    },
    {
      "port": 1087,
      "protocol": "http"
    }
  ],
  "outbounds": [
    {
      "protocol": "vless",
      "settings": {
        "vnext": [
          {
            "address": "${SERVER_IP}",
            "port": ${PORT},
            "users": [
              {
                "id": "${UUID}",
                "encryption": "none"
              }
            ]
          }
        ]
      },
      "streamSettings": {
        "network": "ws",
        "wsSettings": {
          "path": "${WS_PATH}",
          "headers": {
            "Host": "www.apple.com"
          }
        }
      }
    },
    {
      "protocol": "freedom",
      "tag": "direct"
    }
  ]
}
EOF
fi

# Start Xray service
echo -e "${YELLOW}Starting Xray client...${NC}"
if [ "$OS" == "linux" ]; then
    systemctl restart xray
    systemctl enable xray
    systemctl status xray --no-pager
elif [ "$OS" == "macos" ]; then
    brew services restart xray
fi

# Configure system proxy (optional)
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}Client Setup Complete!${NC}"
echo -e "${GREEN}========================================${NC}"
echo
echo -e "${YELLOW}Proxy Settings:${NC}"
echo -e "${GREEN}SOCKS5 Proxy:${NC} 127.0.0.1:1080"
echo -e "${GREEN}HTTP Proxy:${NC} 127.0.0.1:1087"
echo
echo -e "${YELLOW}To configure system proxy:${NC}"
if [ "$OS" == "linux" ]; then
    echo "export http_proxy=http://127.0.0.1:1087"
    echo "export https_proxy=http://127.0.0.1:1087"
    echo "export socks_proxy=socks5://127.0.0.1:1080"
elif [ "$OS" == "macos" ]; then
    echo "networksetup -setsocksfirewallproxy Wi-Fi 127.0.0.1 1080"
    echo "networksetup -setwebproxy Wi-Fi 127.0.0.1 1087"
    echo "networksetup -setsecurewebproxy Wi-Fi 127.0.0.1 1087"
fi
echo
echo -e "${GREEN}Test your connection:${NC}"
echo "curl -x socks5://127.0.0.1:1080 https://www.google.com"