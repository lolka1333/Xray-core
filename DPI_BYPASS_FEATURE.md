# DPI Bypass Feature for Russian RKN Blocking

This implementation adds a new DPI (Deep Packet Inspection) bypass method to Xray Core specifically designed to circumvent Russian RKN (Roskomnadzor) blocking techniques.

## Problem Description

The Russian censor (RKN) has introduced a new blocking method that works as follows:
- When a client connects to a server via TCP using HTTPS and TLS 1.3
- If the server IP is "suspicious" (located outside Russia, owned by foreign data centers)
- If data received from server exceeds 15-20KB within one TCP connection
- The connection is "frozen" - TCP packets stop arriving after the size limit

## Solution

This implementation fragments data transmission across multiple TCP connections, ensuring each connection stays below the 15-20KB threshold that triggers blocking.

## Features

### 1. WebSocket Transport Fragmentation
- Fragments data into configurable chunks (default 15KB)
- Uses multiple WebSocket connections to distribute data
- Adds configurable delays between fragments to avoid detection

### 2. SplitHTTP/XHTTP Transport Fragmentation
- Similar fragmentation for SplitHTTP protocol
- Creates multiple HTTP POST requests to stay under limits
- Optimized for packet-up and stream modes

## Configuration

### WebSocket Client Configuration

```json
{
  "streamSettings": {
    "network": "ws",
    "wsSettings": {
      "path": "/websocket",
      "enable_fragmentation": true,
      "fragment_size": 15,         // Size in KB (15-20KB recommended)
      "fragment_interval": 10      // Interval in milliseconds
    }
  }
}
```

### SplitHTTP Client Configuration

```json
{
  "streamSettings": {
    "network": "splithttp",
    "splithttpSettings": {
      "path": "/splithttp",
      "enable_fragmentation": true,
      "fragment_size": 15,         // Size in KB
      "fragment_interval": 10,     // Interval in milliseconds
      "scMaxEachPostBytes": {
        "from": 100000,
        "to": 200000
      }
    }
  }
}
```

## Configuration Parameters

### enable_fragmentation
- Type: `boolean`
- Default: `false`
- Description: Enables or disables the DPI bypass fragmentation feature

### fragment_size
- Type: `uint32`
- Default: `15` (KB)
- Range: `5-20` (KB)
- Description: Size of each data fragment. Should be set between 15-20KB to avoid RKN detection

### fragment_interval
- Type: `uint32`
- Default: `10` (milliseconds)
- Description: Delay between sending fragments. Helps avoid pattern detection

## Implementation Details

### WebSocket Implementation
1. **Connection Pool**: Maintains multiple WebSocket connections
2. **Data Fragmentation**: Splits data into chunks before transmission
3. **Round-Robin**: Distributes fragments across connections
4. **Automatic Switching**: Creates new connections when size limit approached

### SplitHTTP Implementation
1. **Writer Pool**: Multiple HTTP writers for parallel uploads
2. **POST Fragmentation**: Each POST request stays under size limit
3. **Interval Management**: Configurable delays between requests
4. **Connection Reuse**: Efficient connection management with pooling

## Performance Considerations

1. **Latency**: Small increase due to fragmentation and delays
2. **Throughput**: May be reduced due to multiple connections
3. **Resource Usage**: Slightly higher due to connection pooling
4. **Stability**: More resilient to DPI-based blocking

## Recommendations

1. **Fragment Size**: Use 15KB for maximum compatibility
2. **Interval**: 10-20ms provides good balance
3. **Mode Selection**: 
   - WebSocket: Better for real-time applications
   - SplitHTTP: Better for bulk transfers
4. **TLS Settings**: Use TLS 1.3 with proper SNI configuration

## Testing

To test if your ISP has these restrictions:
1. Disable VPN
2. Try downloading large files from foreign servers
3. Monitor if connections freeze after ~15-20KB

## Compatibility

- Works with existing Xray Core configurations
- Backward compatible - can be disabled if not needed
- Compatible with all proxy protocols (VLESS, VMess, Trojan)

## Security Notes

1. This feature is designed specifically for DPI bypass
2. Does not compromise encryption or security
3. May increase detectability due to connection patterns
4. Should be used only when necessary

## Example Use Cases

1. **Streaming Services**: Bypass throttling of video streams
2. **File Downloads**: Download large files without interruption
3. **Web Browsing**: Improved stability for HTTPS sites
4. **API Access**: Reliable access to foreign APIs

## Troubleshooting

### Connection Drops
- Reduce `fragment_size` to 10-12KB
- Increase `fragment_interval` to 20-30ms

### Slow Performance
- Increase `fragment_size` to 18-20KB if stable
- Reduce `fragment_interval` to 5-10ms

### High Resource Usage
- Limit connection pool size in advanced settings
- Use SplitHTTP instead of WebSocket for efficiency

## Future Improvements

1. Adaptive fragment sizing based on network conditions
2. Dynamic interval adjustment
3. Intelligent connection pooling
4. Pattern randomization for better evasion

## Contributing

This feature is experimental and improvements are welcome. Please test in your environment and report issues or suggestions.