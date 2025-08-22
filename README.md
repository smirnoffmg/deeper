# Deeper

```text
██████  ███████ ███████ ██████  ███████ ██████
██   ██ ██      ██      ██   ██ ██      ██   ██
██   ██ █████   █████   ██████  █████   ██████
██   ██ ██      ██      ██      ██      ██   ██
██████  ███████ ███████ ██      ███████ ██   ██
```

Deeper is an OSINT (Open Source Intelligence) tool designed to help users gather information from various online sources. The tool operates based on the concept of "traces." Each trace represents a piece of information such as an email, phone number, domain, or username. The tool leverages plugins to follow these traces, discovering new traces along the way. Each plugin specializes in processing a specific type of trace and produces new traces based on the input. This modular approach allows for easy extension and customization of the tool to suit various OSINT needs.

## 🚀 Features

- **Modular Plugin Architecture**: Easy to extend with new plugins
- **Concurrent Processing**: Efficient parallel trace processing
- **Comprehensive Error Handling**: Structured error types and logging
- **Configuration Management**: Environment-based configuration
- **Test Coverage**: Comprehensive unit tests
- **Rate Limiting**: Built-in HTTP rate limiting and retry logic
- **Structured Logging**: JSON-based logging with configurable levels

## 🏗️ Architecture

The project follows SOLID principles and clean architecture patterns:

### Core Components

- **Engine**: Orchestrates the trace processing workflow
- **Processor**: Handles concurrent trace processing through plugins
- **Display**: Manages result presentation and formatting
- **Config**: Centralized configuration management
- **Errors**: Structured error handling and types
- **HTTP**: Shared HTTP client with retry logic and rate limiting

### Plugin System

Plugins implement the `DeeperPlugin` interface and are automatically discovered and registered. Each plugin specializes in processing specific trace types and can generate new traces.

## 📦 Installation

### Prerequisites

- Go 1.22 or higher
- Git

### Build from Source

```bash
# Clone the repository
git clone https://github.com/smirnoffmg/deeper.git
cd deeper

# Install dependencies
make deps

# Build the application
make build

# Run the application
./build/deeper <input>
```

### Quick Start

```bash
# Run with default input
make run

# Run with custom input
make run-custom INPUT=your_input_here

# Run in development mode with hot reload
make dev
```

## ⚙️ Configuration

Deeper uses environment variables for configuration:

| Variable                 | Default      | Description                              |
| ------------------------ | ------------ | ---------------------------------------- |
| `DEEPER_HTTP_TIMEOUT`    | `30s`        | HTTP request timeout                     |
| `DEEPER_MAX_CONCURRENCY` | `10`         | Maximum concurrent operations            |
| `DEEPER_RATE_LIMIT`      | `5`          | Requests per second                      |
| `DEEPER_LOG_LEVEL`       | `info`       | Logging level (debug, info, warn, error) |
| `DEEPER_USER_AGENT`      | `Deeper/1.0` | User agent for HTTP requests             |
| `DEEPER_MAX_RETRIES`     | `3`          | Maximum retry attempts                   |
| `DEEPER_RETRY_DELAY`     | `1s`         | Delay between retries                    |

Example:

```bash
export DEEPER_LOG_LEVEL=debug
export DEEPER_MAX_CONCURRENCY=20
./deeper username
```

## 🧪 Testing

The project includes comprehensive test coverage:

```bash
# Run all tests
make test

# Run tests with coverage report
make test-coverage

# Run tests with race detector
make test-race

# Run benchmarks
make benchmark
```

## 🔧 Development

### Project Structure

```text
deeper/
├── internal/
│   ├── config/          # Configuration management
│   ├── display/         # Result presentation
│   ├── engine/          # Core orchestration
│   ├── entities/        # Data models and validation
│   ├── errors/          # Structured error handling
│   ├── http/            # HTTP client utilities
│   ├── plugins/         # Plugin implementations
│   ├── processor/       # Trace processing logic
│   └── state/           # Global state management
├── docs/                # Documentation and diagrams
├── main.go              # Application entry point
├── Makefile             # Build and development tasks
└── README.md            # This file
```

### Adding a New Plugin

1. Create a new directory in `internal/plugins/`
2. Implement the `DeeperPlugin` interface:

```go
package your_plugin

import (
    "github.com/smirnoffmg/deeper/internal/entities"
    "github.com/smirnoffmg/deeper/internal/state"
)

const InputTraceType = entities.Username

func init() {
    p := NewPlugin()
    if err := p.Register(); err != nil {
        log.Error().Err(err).Msgf("Failed to register plugin %s", p)
    }
}

type YourPlugin struct{}

func NewPlugin() *YourPlugin {
    return &YourPlugin{}
}

func (p *YourPlugin) Register() error {
    state.RegisterPlugin(InputTraceType, p)
    return nil
}

func (p *YourPlugin) FollowTrace(trace entities.Trace) ([]entities.Trace, error) {
    if trace.Type != InputTraceType {
        return nil, nil
    }
    
    // Your plugin logic here
    var newTraces []entities.Trace
    // ... process trace and generate new traces
    
    return newTraces, nil
}

func (p *YourPlugin) String() string {
    return "YourPlugin"
}
```

3. Import the plugin in `main.go`:

```go
_ "github.com/smirnoffmg/deeper/internal/plugins/your_plugin"
```

### Development Commands

```bash
# Format code
make fmt

# Run linter
make lint

# Generate mocks for testing
make mocks

# Run security scan
make security

# Show all available commands
make help
```

## 🔒 Security

The project includes several security features:

- **Input Validation**: Comprehensive validation of all inputs
- **Rate Limiting**: Prevents abuse of external APIs
- **Error Handling**: Secure error messages without information leakage
- **Security Scanning**: Automated security checks with gosec

## 📊 Performance

- **Concurrent Processing**: Efficient parallel execution
- **Connection Pooling**: Reusable HTTP connections
- **Memory Management**: Proper resource cleanup
- **Batch Processing**: Configurable batch sizes for large datasets

## 🤝 Contributing

We welcome contributions! Please follow these guidelines:

1. **Fork the Repository**: Click the "Fork" button at the top right
2. **Create a Feature Branch**: `git checkout -b feature/your-feature`
3. **Follow Coding Standards**:
   - Run `make fmt` to format code
   - Run `make lint` to check for issues
   - Add tests for new functionality
4. **Commit Your Changes**: Use descriptive commit messages
5. **Push to Your Branch**: `git push origin feature/your-feature`
6. **Create a Pull Request**: Submit for review

### Development Setup

```bash
# Install development tools
make deps

# Set up pre-commit hooks (optional)
cp .git/hooks/pre-commit.sample .git/hooks/pre-commit
chmod +x .git/hooks/pre-commit

# Run tests before committing
make test
```

## 📝 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🆘 Support

- **Issues**: Report bugs and feature requests on GitHub
- **Discussions**: Join community discussions
- **Documentation**: Check the [docs](docs/) directory for detailed documentation

## 🔄 Changelog

### v1.0.0 (Current)

- ✅ Modular plugin architecture
- ✅ Concurrent trace processing
- ✅ Comprehensive error handling
- ✅ Configuration management
- ✅ Test coverage
- ✅ Rate limiting and retry logic
- ✅ Structured logging
- ✅ Security improvements

## 📈 Roadmap

- [ ] CLI framework with subcommands
- [ ] Plugin lifecycle management
- [ ] Metrics and monitoring
- [ ] Database integration for trace storage
- [ ] Web interface
- [ ] API server mode
- [ ] Plugin marketplace
- [ ] Advanced filtering and search
- [ ] Export functionality (JSON, CSV, etc.)
- [ ] Integration with external OSINT tools

---

**Note**: This tool is designed for legitimate OSINT research and security testing. Please ensure you have proper authorization before using it on any systems or networks you don't own.
