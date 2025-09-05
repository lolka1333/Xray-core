#!/bin/bash

# Comprehensive setup script for Xray with DPI evasion
# This script sets up both client and server with anti-DPI configurations

set -e

echo "🚀 Xray DPI Evasion Setup Script"
echo "================================="
echo ""

# Function to generate UUID
generate_uuid() {
    cat /proc/sys/kernel/random/uuid
}

# Function to generate random password for Shadowsocks
generate_ss_password() {
    openssl rand -base64 16
}

# Function to generate random path
generate_random_path() {
    echo "/$(openssl rand -hex 8)"
}

# Check if running as root
if [ "$EUID" -ne 0 ]; then 
   echo "❌ Please run as root (use sudo)"
   exit 1
fi

# Menu
echo "Select installation type:"
echo "1) Server setup"
echo "2) Client setup"
echo "3) Both (for testing)"
read -p "Enter choice [1-3]: " choice

# Generate credentials
UUID=$(generate_uuid)
SS_PASSWORD=$(generate_ss_password)
WS_PATH=$(generate_random_path)

echo ""
echo "📝 Generated credentials:"
echo "UUID: $UUID"
echo "SS Password: $SS_PASSWORD"
echo "WS Path: $WS_PATH"
echo ""

case $choice in
    1)
        echo "🖥️ Setting up SERVER..."
        
        # Install Xray if not present
        if [ ! -f /usr/local/bin/xray ]; then
            echo "Installing Xray..."
            cp /workspace/xray_patched /usr/local/bin/xray
            chmod +x /usr/local/bin/xray
        fi
        
        # Create config directory
        mkdir -p /usr/local/etc/xray
        
        # Get server IP
        read -p "Enter your server's public IP: " SERVER_IP
        
        # Generate self-signed certificate (for testing)
        echo "Generating TLS certificate..."
        openssl req -x509 -newkey rsa:4096 -keyout /usr/local/etc/xray/key.pem \
            -out /usr/local/etc/xray/cert.pem -days 365 -nodes \
            -subj "/C=US/ST=California/L=Los Angeles/O=Microsoft/CN=www.microsoft.com" 2>/dev/null
        
        # Create server config
        cat > /usr/local/etc/xray/config.json << EOF
{
  "log": {
    "loglevel": "warning"
  },
  "inbounds": [
    {
      "port": 8443,
      "protocol": "vless",
      "settings": {
        "clients": [
          {
            "id": "$UUID",
            "flow": "xtls-rprx-vision"
          }
        ],
        "decryption": "none",
        "fallbacks": [
          {
            "dest": "www.microsoft.com:443"
          }
        ]
      },
      "streamSettings": {
        "network": "tcp",
        "security": "tls",
        "tlsSettings": {
          "certificates": [
            {
              "certificateFile": "/usr/local/etc/xray/cert.pem",
              "keyFile": "/usr/local/etc/xray/key.pem"
            }
          ],
          "alpn": ["h2", "http/1.1"]
        },
        "sockopt": {
          "tcpFastOpen": true,
          "tcpNoDelay": true
        }
      }
    },
    {
      "port": 2053,
      "protocol": "vless",
      "settings": {
        "clients": [
          {
            "id": "$UUID"
          }
        ],
        "decryption": "none"
      },
      "streamSettings": {
        "network": "ws",
        "wsSettings": {
          "path": "$WS_PATH"
        }
      }
    },
    {
      "port": 2083,
      "protocol": "shadowsocks",
      "settings": {
        "method": "2022-blake3-aes-128-gcm",
        "password": "$SS_PASSWORD"
      }
    }
  ],
  "outbounds": [
    {
      "protocol": "freedom",
      "settings": {}
    }
  ]
}
EOF
        
        # Create systemd service
        cat > /etc/systemd/system/xray.service << EOF
[Unit]
Description=Xray Service
After=network.target

[Service]
Type=simple
User=root
ExecStart=/usr/local/bin/xray run -config /usr/local/etc/xray/config.json
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF
        
        # Enable and start service
        systemctl daemon-reload
        systemctl enable xray
        systemctl restart xray
        
        echo "✅ Server setup complete!"
        echo ""
        echo "📋 Server details:"
        echo "IP: $SERVER_IP"
        echo "VLESS port: 8443"
        echo "WebSocket port: 2053"
        echo "Shadowsocks port: 2083"
        ;;
        
    2)
        echo "💻 Setting up CLIENT..."
        
        read -p "Enter server IP: " SERVER_IP
        read -p "Enter UUID: " UUID
        read -p "Enter WS path: " WS_PATH
        read -p "Enter SS password: " SS_PASSWORD
        
        # Create client config
        mkdir -p ~/xray_client
        cat > ~/xray_client/config.json << EOF
{
  "log": {
    "loglevel": "warning"
  },
  "inbounds": [
    {
      "port": 1080,
      "protocol": "socks",
      "settings": {
        "auth": "noauth"
      }
    },
    {
      "port": 8118,
      "protocol": "http",
      "settings": {}
    }
  ],
  "outbounds": [
    {
      "protocol": "vless",
      "settings": {
        "vnext": [
          {
            "address": "$SERVER_IP",
            "port": 8443,
            "users": [
              {
                "id": "$UUID",
                "encryption": "none",
                "flow": "xtls-rprx-vision"
              }
            ]
          }
        ]
      },
      "streamSettings": {
        "network": "tcp",
        "security": "tls",
        "tlsSettings": {
          "serverName": "www.microsoft.com",
          "fingerprint": "chrome",
          "alpn": ["h2", "http/1.1"]
        },
        "sockopt": {
          "dialerProxy": "fragment",
          "tcpFastOpen": true,
          "tcpNoDelay": true,
          "mark": 255
        }
      },
      "mux": {
        "enabled": true,
        "concurrency": 8
      }
    }
  ]
}
EOF
        
        echo "✅ Client setup complete!"
        echo ""
        echo "📋 Client configuration saved to: ~/xray_client/config.json"
        echo "SOCKS5 proxy: 127.0.0.1:1080"
        echo "HTTP proxy: 127.0.0.1:8118"
        echo ""
        echo "To start client:"
        echo "./xray_patched run -config ~/xray_client/config.json"
        ;;
        
    3)
        echo "🔧 Setting up both SERVER and CLIENT..."
        # Implement both setups
        ;;
        
    *)
        echo "Invalid choice"
        exit 1
        ;;
esac

echo ""
echo "🎉 Setup complete!"
echo ""
echo "⚠️ Important notes:"
echo "1. Use fingerprint 'chrome' or 'firefox' in client config"
echo "2. Enable 'dialerProxy': 'fragment' for packet fragmentation"
echo "3. Use non-standard ports (8443, 2053, 2083) instead of 443"
echo "4. Enable mux for traffic mixing"
echo "5. Regularly update configurations"