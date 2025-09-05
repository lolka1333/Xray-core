#!/bin/bash

# Xray Server Installation Script with Anti-Censorship Configuration
# Designed for bypassing RKN DPI and TLS 1.3 blocking

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}Xray Anti-Censorship Server Setup${NC}"
echo -e "${GREEN}========================================${NC}"

# Check if running as root
if [[ $EUID -ne 0 ]]; then
   echo -e "${RED}This script must be run as root${NC}"
   exit 1
fi

# Update system
echo -e "${YELLOW}Updating system packages...${NC}"
apt-get update
apt-get upgrade -y

# Install required packages
echo -e "${YELLOW}Installing required packages...${NC}"
apt-get install -y curl wget unzip jq qrencode ufw fail2ban net-tools

# Install Xray
echo -e "${YELLOW}Installing Xray-core...${NC}"
bash -c "$(curl -L https://github.com/XTLS/Xray-install/raw/main/install-release.sh)" @ install

# Generate UUID
UUID=$(cat /proc/sys/kernel/random/uuid)
echo -e "${GREEN}Generated UUID: ${UUID}${NC}"

# Generate REALITY keys
echo -e "${YELLOW}Generating REALITY keys...${NC}"
KEYS=$(xray x25519)
PRIVATE_KEY=$(echo "$KEYS" | grep "Private key:" | cut -d' ' -f3)
PUBLIC_KEY=$(echo "$KEYS" | grep "Public key:" | cut -d' ' -f3)

echo -e "${GREEN}Private Key: ${PRIVATE_KEY}${NC}"
echo -e "${GREEN}Public Key: ${PUBLIC_KEY}${NC}"

# Generate Shadowsocks password
SS_PASSWORD=$(openssl rand -base64 32)
echo -e "${GREEN}Shadowsocks Password: ${SS_PASSWORD}${NC}"

# Get server IP
SERVER_IP=$(curl -s ifconfig.me)
echo -e "${GREEN}Server IP: ${SERVER_IP}${NC}"

# Backup original config if exists
if [ -f /usr/local/etc/xray/config.json ]; then
    cp /usr/local/etc/xray/config.json /usr/local/etc/xray/config.json.bak
fi

# Create Xray configuration
cat > /usr/local/etc/xray/config.json <<EOF
{
  "log": {
    "loglevel": "warning",
    "access": "/var/log/xray/access.log",
    "error": "/var/log/xray/error.log"
  },
  "routing": {
    "domainStrategy": "IPIfNonMatch",
    "rules": [
      {
        "type": "field",
        "domain": [
          "geosite:category-ads-all"
        ],
        "outboundTag": "block"
      }
    ]
  },
  "inbounds": [
    {
      "port": 8443,
      "protocol": "vless",
      "settings": {
        "clients": [
          {
            "id": "${UUID}",
            "flow": "xtls-rprx-vision"
          }
        ],
        "decryption": "none"
      },
      "streamSettings": {
        "network": "tcp",
        "security": "reality",
        "realitySettings": {
          "show": false,
          "dest": "microsoft.com:443",
          "xver": 0,
          "serverNames": [
            "microsoft.com",
            "www.microsoft.com",
            "update.microsoft.com",
            "www.bing.com",
            "edge.microsoft.com"
          ],
          "privateKey": "${PRIVATE_KEY}",
          "shortIds": [
            "6ba85179e30d4fc2"
          ]
        }
      },
      "sniffing": {
        "enabled": true,
        "destOverride": ["http", "tls", "quic"],
        "routeOnly": true
      }
    },
    {
      "port": 2087,
      "protocol": "vless",
      "settings": {
        "clients": [
          {
            "id": "${UUID}"
          }
        ],
        "decryption": "none"
      },
      "streamSettings": {
        "network": "ws",
        "wsSettings": {
          "path": "/ws${RANDOM}",
          "headers": {
            "Host": "www.apple.com"
          }
        }
      }
    },
    {
      "port": 2053,
      "protocol": "shadowsocks",
      "settings": {
        "method": "2022-blake3-aes-256-gcm",
        "password": "${SS_PASSWORD}",
        "network": "tcp,udp"
      }
    }
  ],
  "outbounds": [
    {
      "protocol": "freedom",
      "tag": "direct"
    },
    {
      "protocol": "blackhole",
      "tag": "block"
    }
  ]
}
EOF

# Create log directory
mkdir -p /var/log/xray

# Configure firewall
echo -e "${YELLOW}Configuring firewall...${NC}"
ufw --force disable
ufw default deny incoming
ufw default allow outgoing
ufw allow ssh
ufw allow 8443/tcp
ufw allow 2087/tcp
ufw allow 2053/tcp
ufw allow 2053/udp
ufw --force enable

# Configure sysctl for better performance
echo -e "${YELLOW}Optimizing network settings...${NC}"
cat > /etc/sysctl.d/99-xray.conf <<EOF
# TCP BBR congestion control
net.core.default_qdisc=fq
net.ipv4.tcp_congestion_control=bbr

# TCP optimization
net.ipv4.tcp_fastopen=3
net.ipv4.tcp_slow_start_after_idle=0
net.ipv4.tcp_mtu_probing=1

# Increase buffer sizes
net.core.rmem_max=134217728
net.core.wmem_max=134217728
net.ipv4.tcp_rmem=4096 87380 134217728
net.ipv4.tcp_wmem=4096 65536 134217728

# Connection tracking
net.netfilter.nf_conntrack_max=1000000
net.nf_conntrack_max=1000000

# Security
net.ipv4.tcp_syncookies=1
net.ipv4.tcp_syn_retries=2
net.ipv4.tcp_synack_retries=2
net.ipv4.tcp_max_syn_backlog=4096
EOF

sysctl -p /etc/sysctl.d/99-xray.conf

# Configure fail2ban for Xray
echo -e "${YELLOW}Configuring fail2ban...${NC}"
cat > /etc/fail2ban/filter.d/xray.conf <<EOF
[Definition]
failregex = rejected.*from\s<HOST>
ignoreregex =
EOF

cat > /etc/fail2ban/jail.d/xray.conf <<EOF
[xray]
enabled = true
port = 8443,2087,2053
filter = xray
logpath = /var/log/xray/error.log
maxretry = 3
findtime = 3600
bantime = 86400
EOF

systemctl restart fail2ban

# Start and enable Xray
echo -e "${YELLOW}Starting Xray service...${NC}"
systemctl restart xray
systemctl enable xray

# Generate client configurations
echo -e "${YELLOW}Generating client configurations...${NC}"

# VLESS + REALITY config
cat > /root/client-reality.json <<EOF
{
  "outbounds": [
    {
      "protocol": "vless",
      "settings": {
        "vnext": [
          {
            "address": "${SERVER_IP}",
            "port": 8443,
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
      }
    }
  ]
}
EOF

# Generate connection strings
VLESS_REALITY="vless://${UUID}@${SERVER_IP}:8443?encryption=none&flow=xtls-rprx-vision&security=reality&sni=microsoft.com&fp=chrome&pbk=${PUBLIC_KEY}&sid=6ba85179e30d4fc2&type=tcp#REALITY-8443"
VLESS_WS="vless://${UUID}@${SERVER_IP}:2087?encryption=none&security=none&type=ws&host=www.apple.com&path=%2Fws${RANDOM}#WS-2087"

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}Installation Complete!${NC}"
echo -e "${GREEN}========================================${NC}"
echo
echo -e "${YELLOW}Connection Information:${NC}"
echo -e "${GREEN}Server IP:${NC} ${SERVER_IP}"
echo
echo -e "${GREEN}VLESS + REALITY (Port 8443):${NC}"
echo "${VLESS_REALITY}"
echo
echo -e "${GREEN}VLESS + WebSocket (Port 2087):${NC}"
echo "${VLESS_WS}"
echo
echo -e "${GREEN}Shadowsocks (Port 2053):${NC}"
echo "Method: 2022-blake3-aes-256-gcm"
echo "Password: ${SS_PASSWORD}"
echo
echo -e "${YELLOW}Client configuration saved to:${NC}"
echo "/root/client-reality.json"
echo
echo -e "${YELLOW}QR Codes:${NC}"
qrencode -t ansiutf8 "${VLESS_REALITY}"
echo
echo -e "${GREEN}Service Status:${NC}"
systemctl status xray --no-pager

# Save credentials to file
cat > /root/xray-credentials.txt <<EOF
Xray Server Credentials
=======================
Server IP: ${SERVER_IP}
UUID: ${UUID}
Public Key: ${PUBLIC_KEY}
Private Key: ${PRIVATE_KEY}
Shadowsocks Password: ${SS_PASSWORD}

VLESS + REALITY (Port 8443):
${VLESS_REALITY}

VLESS + WebSocket (Port 2087):
${VLESS_WS}
EOF

echo
echo -e "${YELLOW}Credentials saved to /root/xray-credentials.txt${NC}"
echo -e "${GREEN}Setup complete! Your server is ready.${NC}"