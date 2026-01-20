# Lotty

Lotty is a Go program that sends command output to Grafana Loki via HTTP API. It supports PTY capabilities, in-memory buffering, and batch sending.

## Features

- **Dual Input Modes**: Supports PTY mode (default) and simple pipe mode
- **In-Memory Buffer**: Uses ring buffer with default 10,000 messages, FIFO discard policy
- **Batch Sending**: Supports time-based triggering (default 900ms) and buffer-full triggering
- **Basic Auth**: Complete Loki HTTP API authentication support
- **Flexible Configuration**: Supports command-line arguments and environment variables
- **Retry Mechanism**: Exponential backoff algorithm, default 3 retries
- **Graceful Shutdown**: Supports SIGINT and SIGTERM signal handling

## Installation

### Build from Source

```bash
git clone https://github.com/yourname/lotty.git
cd lotty
make build
```

### Using Docker

```bash
docker build -t lotty:latest .
docker run -e LOKI_ENDPOINT=http://loki:3100 \
           -e LOKI_USERNAME=admin \
           -e LOKI_PASSWORD=password \
           lotty:latest -- your-command
```

## Configuration

All configuration items support environment variables. Priority: command-line arguments > environment variables > default values

### Environment Variables

| Variable Name | Description | Default Value |
|---------------|-------------|---------------|
| `LOKI_ENDPOINT` | Loki service address | None (required) |
| `LOKI_USERNAME` | Basic Auth username | None |
| `LOKI_PASSWORD` | Basic Auth password | None |
| `LOKI_LABELS` | Loki labels, format key=value | None |
| `BUFFER_SIZE` | Buffer size | 10000 |
| `BATCH_INTERVAL` | Batch sending interval | 900ms |
| `BATCH_SIZE` | Batch size | 1000 |
| `RETRY_COUNT` | Retry count | 3 |
| `INPUT_MODE` | Input mode (pty/pipe) | pty |
| `LOG_LEVEL` | Log level | info |

### Command-Line Arguments

```bash
lotty [flags] -- command [args...]

Flags:
  --config string       Configuration file (default is $HOME/.lotty.yaml)
  --input-mode string   Input mode (pty, pipe) (default "pty")
  --log-level string    Log level (debug, info, warn, error) (default "info")
  -h, --help           Show help information
```

## Usage Examples

### Basic Usage

```bash
# Configure using environment variables
export LOKI_ENDPOINT="http://localhost:3100/loki/api/v1/push"
export LOKI_USERNAME="admin"
export LOKI_PASSWORD="password"
export LOKI_LABELS="service=myapp,env=prod"

# Send ping output to Loki
lotty -- ping google.com
```

### Using Pipe Mode

```bash
# Monitor log files
lotty --input-mode pipe -- tail -f /var/log/app.log
```

### Using PTY Mode (Default)

```bash
# Monitor application output (supports colored output and interactive programs)
lotty -- my-app --arg1 --arg2
```

## Architecture

```
Command Output → PTY/Pipe Capture → Line Parsing → Memory Buffer → Batch Processor → HTTP Client → Loki
```

### Core Components

- **Input Processing**: Supports PTY and pipe modes, captures command stdout and stderr
- **Memory Buffer**: Ring buffer implementation, thread-safe, FIFO discard when full
- **Batch Processor**: Timer-triggered and buffer-full triggered, supports dynamic batch size
- **Loki Client**: HTTP client with Basic Auth support, JSON format requests
- **Retry Mechanism**: Exponential backoff algorithm, configurable maximum retry count

## Development

### Running Tests

```bash
# Run all tests
make test

# Generate test coverage report
make test-coverage
```

### Code Linting

```bash
# Run linter
make lint

# Format code
make fmt
```

### Building

```bash
# Build binary
make build

# Cross-compile
make build-all
```

## Project Structure

```
lotty/
├── cmd/lotty/main.go          # Main entry point
├── internal/
│   ├── config/              # Configuration management
│   ├── input/               # Input processing (pty + reader)
│   ├── buffer/              # Ring buffer
│   ├── loki/                # HTTP client + Auth
│   ├── batcher/             # Batch processor
│   └── retry/               # Retry mechanism
├── pkg/
│   └── types/               # Common types
├── go.mod
├── go.sum
├── Makefile
├── Dockerfile
└── README.md
```

## Performance Metrics

- **Memory Usage**: Depends on buffer size configuration, default ~10MB
- **Throughput**: Depends on batch interval and network latency
- **CPU Usage**: Low, mainly during batch sending and network requests

## Troubleshooting

### Connection Issues

- Check Loki service reachability
- Verify authentication information
- Check network connection status

### Performance Issues

- Monitor memory usage
- Adjust batch size
- Optimize buffer configuration

### PTY Issues

- Check command execution permissions
- Verify terminal environment
- Check process status

## License

MIT License

## Contributing

Pull Requests and Issues are welcome.
