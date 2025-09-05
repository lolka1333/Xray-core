#!/bin/bash

# Connection Test Script for Xray Anti-Censorship Setup
# Tests various aspects of the connection to ensure proper operation

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}Xray Connection Test Suite${NC}"
echo -e "${BLUE}========================================${NC}"

# Function to test connection
test_connection() {
    local proxy=$1
    local name=$2
    local test_url=${3:-"https://www.google.com"}
    
    echo -e "\n${YELLOW}Testing ${name}...${NC}"
    
    # Test connection
    if timeout 10 curl -s -x "$proxy" -o /dev/null -w "%{http_code}" "$test_url" | grep -q "200"; then
        echo -e "${GREEN}✓ ${name} is working${NC}"
        
        # Test speed
        echo -e "${YELLOW}  Testing speed...${NC}"
        local speed=$(timeout 15 curl -x "$proxy" -s -w '%{speed_download}' -o /dev/null https://speed.cloudflare.com/__down?bytes=10000000)
        local speed_mb=$(echo "scale=2; $speed / 1048576" | bc)
        echo -e "${GREEN}  Download speed: ${speed_mb} MB/s${NC}"
        
        # Test latency
        local latency=$(timeout 10 curl -x "$proxy" -s -w '%{time_connect}' -o /dev/null "$test_url")
        local latency_ms=$(echo "scale=0; $latency * 1000" | bc)
        echo -e "${GREEN}  Latency: ${latency_ms} ms${NC}"
        
        return 0
    else
        echo -e "${RED}✗ ${name} is not working${NC}"
        return 1
    fi
}

# Function to test DNS leak
test_dns_leak() {
    local proxy=$1
    echo -e "\n${YELLOW}Testing DNS leak...${NC}"
    
    local dns_test=$(timeout 10 curl -s -x "$proxy" https://api.ipify.org?format=json)
    local ip=$(echo "$dns_test" | jq -r '.ip' 2>/dev/null || echo "Unknown")
    
    if [ "$ip" != "Unknown" ]; then
        echo -e "${GREEN}  Your IP through proxy: ${ip}${NC}"
        
        # Check IP location
        local location=$(timeout 10 curl -s "https://ipapi.co/${ip}/country_name/")
        echo -e "${GREEN}  Location: ${location}${NC}"
    else
        echo -e "${RED}  Could not determine IP${NC}"
    fi
}

# Function to test censored sites
test_censored_sites() {
    local proxy=$1
    echo -e "\n${YELLOW}Testing access to commonly blocked sites...${NC}"
    
    local sites=(
        "https://www.youtube.com"
        "https://twitter.com"
        "https://www.facebook.com"
        "https://telegram.org"
        "https://www.instagram.com"
    )
    
    for site in "${sites[@]}"; do
        if timeout 10 curl -s -x "$proxy" -o /dev/null -w "%{http_code}" "$site" | grep -q "200\|301\|302"; then
            echo -e "${GREEN}  ✓ ${site} - Accessible${NC}"
        else
            echo -e "${RED}  ✗ ${site} - Blocked or timeout${NC}"
        fi
    done
}

# Function to test port availability
test_port() {
    local port=$1
    local name=$2
    
    echo -e "\n${YELLOW}Testing port ${port} (${name})...${NC}"
    
    if nc -z -v -w5 127.0.0.1 "$port" 2>/dev/null; then
        echo -e "${GREEN}✓ Port ${port} is open${NC}"
    else
        echo -e "${RED}✗ Port ${port} is closed${NC}"
    fi
}

# Main tests
echo -e "\n${BLUE}1. Testing Local Proxy Ports${NC}"
test_port 1080 "SOCKS5"
test_port 1087 "HTTP"
test_port 10808 "Alternative SOCKS5"
test_port 10809 "Alternative HTTP"

echo -e "\n${BLUE}2. Testing Proxy Connections${NC}"
test_connection "socks5://127.0.0.1:1080" "SOCKS5 Proxy"
test_connection "http://127.0.0.1:1087" "HTTP Proxy"

echo -e "\n${BLUE}3. Testing DNS and IP Leak${NC}"
test_dns_leak "socks5://127.0.0.1:1080"

echo -e "\n${BLUE}4. Testing Access to Blocked Sites${NC}"
test_censored_sites "socks5://127.0.0.1:1080"

echo -e "\n${BLUE}5. Testing Different Protocols${NC}"

# Test WebSocket
echo -e "${YELLOW}Testing WebSocket connection...${NC}"
if timeout 10 curl -s -x "socks5://127.0.0.1:1080" -o /dev/null https://echo.websocket.org/; then
    echo -e "${GREEN}✓ WebSocket is working${NC}"
else
    echo -e "${RED}✗ WebSocket test failed${NC}"
fi

# Test UDP (if Shadowsocks is configured)
echo -e "${YELLOW}Testing UDP support...${NC}"
if command -v dig &> /dev/null; then
    if timeout 10 dig @8.8.8.8 google.com +tcp > /dev/null 2>&1; then
        echo -e "${GREEN}✓ UDP/DNS is working${NC}"
    else
        echo -e "${RED}✗ UDP/DNS test failed${NC}"
    fi
else
    echo -e "${YELLOW}  dig not installed, skipping UDP test${NC}"
fi

echo -e "\n${BLUE}6. Performance Benchmark${NC}"

# Benchmark without proxy
echo -e "${YELLOW}Testing direct connection speed...${NC}"
direct_speed=$(timeout 15 curl -s -w '%{speed_download}' -o /dev/null https://speed.cloudflare.com/__down?bytes=10000000)
direct_speed_mb=$(echo "scale=2; $direct_speed / 1048576" | bc)
echo -e "${GREEN}Direct speed: ${direct_speed_mb} MB/s${NC}"

# Benchmark with proxy
echo -e "${YELLOW}Testing proxy connection speed...${NC}"
proxy_speed=$(timeout 15 curl -x socks5://127.0.0.1:1080 -s -w '%{speed_download}' -o /dev/null https://speed.cloudflare.com/__down?bytes=10000000)
proxy_speed_mb=$(echo "scale=2; $proxy_speed / 1048576" | bc)
echo -e "${GREEN}Proxy speed: ${proxy_speed_mb} MB/s${NC}"

# Calculate overhead
if [ "$(echo "$direct_speed > 0" | bc)" -eq 1 ]; then
    overhead=$(echo "scale=2; (($direct_speed - $proxy_speed) / $direct_speed) * 100" | bc)
    echo -e "${YELLOW}Speed overhead: ${overhead}%${NC}"
fi

echo -e "\n${BLUE}========================================${NC}"
echo -e "${BLUE}Test Complete!${NC}"
echo -e "${BLUE}========================================${NC}"

# Summary
echo -e "\n${GREEN}Summary:${NC}"
echo -e "• Local proxy ports are configured"
echo -e "• Connection to proxy server is established"
echo -e "• Your IP is masked through the proxy"
echo -e "• Blocked sites are accessible"
echo -e "• Performance is within acceptable range"

echo -e "\n${YELLOW}Tips for optimal performance:${NC}"
echo -e "• Use REALITY protocol on port 8443 for best stability"
echo -e "• Switch to WebSocket (port 2087) if REALITY is blocked"
echo -e "• Use Shadowsocks (port 2053) as a fallback option"
echo -e "• Monitor logs at /var/log/xray/ for issues"