#!/bin/bash

# Test script for DPI evasion features
# This script tests various anti-DPI techniques

echo "🔍 DPI Evasion Testing Script"
echo "=============================="
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test function
test_feature() {
    local name=$1
    local command=$2
    
    echo -n "Testing $name... "
    
    if eval $command > /dev/null 2>&1; then
        echo -e "${GREEN}✓ PASSED${NC}"
        return 0
    else
        echo -e "${RED}✗ FAILED${NC}"
        return 1
    fi
}

# Check if Xray is running
echo "1. Checking Xray status..."
if pgrep -x "xray" > /dev/null; then
    echo -e "${GREEN}✓ Xray is running${NC}"
else
    echo -e "${YELLOW}⚠ Xray is not running${NC}"
fi

# Test configuration validity
echo ""
echo "2. Testing configuration..."
if [ -f /workspace/xray_patched ]; then
    /workspace/xray_patched test -config /workspace/client_config_dpi_evasion.json > /dev/null 2>&1
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✓ Client config is valid${NC}"
    else
        echo -e "${RED}✗ Client config has errors${NC}"
    fi
    
    /workspace/xray_patched test -config /workspace/server_config_dpi_evasion.json > /dev/null 2>&1
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✓ Server config is valid${NC}"
    else
        echo -e "${RED}✗ Server config has errors${NC}"
    fi
fi

# Test DPI evasion features
echo ""
echo "3. Testing DPI evasion features..."

# Check for fragmentation support
echo -n "   Fragment support: "
if grep -q "dialerProxy.*fragment" /workspace/client_config_dpi_evasion.json; then
    echo -e "${GREEN}✓ Enabled${NC}"
else
    echo -e "${RED}✗ Disabled${NC}"
fi

# Check for uTLS fingerprint
echo -n "   uTLS fingerprint: "
if grep -q "fingerprint.*chrome\|firefox" /workspace/client_config_dpi_evasion.json; then
    echo -e "${GREEN}✓ Configured${NC}"
else
    echo -e "${RED}✗ Not configured${NC}"
fi

# Check for mux
echo -n "   Multiplexing: "
if grep -q '"mux".*"enabled".*true' /workspace/client_config_dpi_evasion.json; then
    echo -e "${GREEN}✓ Enabled${NC}"
else
    echo -e "${RED}✗ Disabled${NC}"
fi

# Check for non-standard ports
echo -n "   Non-standard ports: "
if grep -q "port.*8443\|2053\|2083" /workspace/client_config_dpi_evasion.json; then
    echo -e "${GREEN}✓ Using alternative ports${NC}"
else
    echo -e "${YELLOW}⚠ Using standard ports${NC}"
fi

# Network connectivity test (if proxy is running)
echo ""
echo "4. Network connectivity test..."
if nc -z 127.0.0.1 1080 2>/dev/null; then
    echo -e "${GREEN}✓ SOCKS5 proxy is accessible${NC}"
    
    # Test actual connectivity through proxy
    echo -n "   Testing proxy connectivity: "
    timeout 5 curl -x socks5://127.0.0.1:1080 https://www.google.com -o /dev/null -s
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✓ Proxy works${NC}"
    else
        echo -e "${RED}✗ Proxy connection failed${NC}"
    fi
else
    echo -e "${YELLOW}⚠ SOCKS5 proxy not running${NC}"
fi

# Performance test
echo ""
echo "5. Performance indicators..."
echo -n "   Binary size: "
if [ -f /workspace/xray_patched ]; then
    size=$(ls -lh /workspace/xray_patched | awk '{print $5}')
    echo "$size"
fi

# Summary
echo ""
echo "================================"
echo "Summary:"
echo ""

# Count successes
total_tests=7
passed_tests=0

[ -f /workspace/xray_patched ] && ((passed_tests++))
grep -q "dialerProxy.*fragment" /workspace/client_config_dpi_evasion.json && ((passed_tests++))
grep -q "fingerprint.*chrome\|firefox" /workspace/client_config_dpi_evasion.json && ((passed_tests++))
grep -q '"mux".*"enabled".*true' /workspace/client_config_dpi_evasion.json && ((passed_tests++))
grep -q "port.*8443\|2053\|2083" /workspace/client_config_dpi_evasion.json && ((passed_tests++))

echo "Tests passed: $passed_tests/$total_tests"

if [ $passed_tests -eq $total_tests ]; then
    echo -e "${GREEN}✅ All DPI evasion features are properly configured!${NC}"
elif [ $passed_tests -ge 5 ]; then
    echo -e "${YELLOW}⚠ Most features configured, but some optimizations missing${NC}"
else
    echo -e "${RED}❌ Configuration needs improvement${NC}"
fi

echo ""
echo "Recommendations:"
echo "1. Always use 'dialerProxy': 'fragment' in sockopt"
echo "2. Set 'fingerprint' to 'chrome' or 'firefox'"
echo "3. Enable mux with concurrency 8-16"
echo "4. Use ports like 8443, 2053, 2083 instead of 443"
echo "5. Consider using Reality protocol for better evasion"