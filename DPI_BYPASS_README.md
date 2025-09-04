# DPI Bypass Feature for Xray Core

## Overview

This implementation adds a new DPI (Deep Packet Inspection) bypass method to Xray Core specifically designed to counter the Russian censorship mechanism that limits TCP data transfer to ~15-20KB per connection for "suspicious" IP addresses.

## How It Works

The DPI bypass feature fragments data into smaller chunks (default: 15KB) to stay below the censorship threshold. Unlike simple TCP packet fragmentation, this implementation:

1. **Fragments at the application layer** - Data is split into configurable chunks before being sent
2. **Works with WebSocket and splitHTTP transports** - Both protocols now support fragmentation
3. **Provides random fragment sizes** - Helps avoid pattern detection
4. **Adds configurable delays** - Spaces out fragments to appear more natural

## Configuration Parameters

The following parameters can be configured in both WebSocket (`wsSettings`) and splitHTTP (`splithttpSettings`):

- `dpiBypassEnabled` (bool): Enable/disable DPI bypass fragmentation
- `dpiFragmentSize` (int32): Fragment size in bytes (default: 15360 = 15KB)
- `dpiFragmentDelay` (int32): Delay between fragments in milliseconds (default: 0)
- `dpiRandomSize` (bool): Use random fragment sizes between min and max
- `dpiMinSize` (int32): Minimum random fragment size (default: 10240 = 10KB)
- `dpiMaxSize` (int32): Maximum random fragment size (default: 20480 = 20KB)

## WebSocket Configuration Example

### Client Configuration
```json
{
  "streamSettings": {
    "network": "ws",
    "wsSettings": {
      "path": "/websocket",
      "dpiBypassEnabled": true,
      "dpiFragmentSize": 15360,
      "dpiFragmentDelay": 10,
      "dpiRandomSize": true,
      "dpiMinSize": 10240,
      "dpiMaxSize": 20480
    }
  }
}
```

### Server Configuration
The server should use the same DPI bypass settings as the client for optimal performance.

## SplitHTTP Configuration Example

### Client Configuration
```json
{
  "streamSettings": {
    "network": "splithttp",
    "splithttpSettings": {
      "path": "/splithttp",
      "mode": "packet-up",
      "scMaxEachPostBytes": {
        "from": 15000,
        "to": 18000
      },
      "dpiBypassEnabled": true,
      "dpiFragmentSize": 15360,
      "dpiFragmentDelay": 10,
      "dpiRandomSize": true,
      "dpiMinSize": 10240,
      "dpiMaxSize": 20480
    }
  }
}
```

## Performance Considerations

1. **Fragmentation overhead**: Splitting data into smaller chunks adds overhead. Use the largest fragment size that works reliably.

2. **Delay settings**: Adding delays between fragments reduces throughput but may improve reliability. Start with 0 and increase if needed.

3. **Random sizes**: Using random fragment sizes helps avoid detection but may impact performance. Enable only if fixed sizes are being blocked.

## Testing

To test if your ISP has these limitations:
1. Disable any VPN/proxy
2. Try downloading a file larger than 20KB from a foreign server
3. If the connection freezes after ~15-20KB, you're affected by this censorship

## Recommendations

1. **Start with default settings**: 15KB fragments with no delay
2. **Adjust based on your ISP**: Some ISPs may have different thresholds
3. **Monitor performance**: Use logs to track connection stability
4. **Use with TLS**: Always use TLS encryption with these transports
5. **Consider splitHTTP**: It's specifically designed for handling fragmented transfers

## Technical Details

The implementation:
- Creates a `FragmentWriter` that wraps the underlying connection
- Splits outgoing data into configured chunk sizes
- Maintains connection state across fragments
- Works transparently with existing Xray protocols

## Compatibility

This feature is compatible with:
- VLESS protocol
- VMess protocol  
- Trojan protocol
- Any other protocol that uses WebSocket or splitHTTP transport

## Limitations

1. **TCP connections only**: This bypass method works for TCP-based transports
2. **Increased latency**: Fragmentation adds latency to data transfer
3. **Not for UDP**: UDP-based transports like QUIC are not affected by this censorship

## Building from Source

```bash
# Clone the repository
git clone https://github.com/xtls/xray-core
cd xray-core

# Build with DPI bypass support
go build -o xray ./main
```

## Example Use Cases

### For users in Russia affected by the 15-20KB limit:
1. Enable DPI bypass with 15KB fragments
2. Use WebSocket or splitHTTP transport
3. Configure TLS with a valid certificate
4. Use VLESS protocol for better performance

### Configuration for maximum reliability:
```json
{
  "dpiBypassEnabled": true,
  "dpiFragmentSize": 12288,  // 12KB - conservative size
  "dpiFragmentDelay": 20,     // 20ms delay
  "dpiRandomSize": true,
  "dpiMinSize": 8192,         // 8KB minimum
  "dpiMaxSize": 15360         // 15KB maximum
}
```

### Configuration for better performance:
```json
{
  "dpiBypassEnabled": true,
  "dpiFragmentSize": 18432,  // 18KB - closer to limit
  "dpiFragmentDelay": 0,      // No delay
  "dpiRandomSize": false     // Fixed size for consistency
}
```

## Troubleshooting

1. **Connection freezes**: Reduce `dpiFragmentSize`
2. **Slow speeds**: Increase `dpiFragmentSize` or reduce `dpiFragmentDelay`
3. **Intermittent blocks**: Enable `dpiRandomSize`
4. **High CPU usage**: Increase `dpiFragmentSize` to reduce fragmentation overhead

## Contributing

If you find issues or have improvements, please submit a pull request or open an issue on the Xray Core repository.