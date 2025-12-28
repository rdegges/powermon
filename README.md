# ⚡ PowerMon

[![CI](https://github.com/rdegges/powermon/actions/workflows/ci.yml/badge.svg)](https://github.com/rdegges/powermon/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/rdegges/powermon)](https://goreportcard.com/report/github.com/rdegges/powermon)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A cross-platform CLI application that displays real-time power consumption with a fancy, interactive terminal UI.

![Power Monitor GIF](https://via.placeholder.com/600x400/1a1a2e/7D56F4?text=Power+Monitor)

## Features

- 📊 **Real-time power monitoring** - See current power consumption in watts
- 📈 **Interactive sparkline graph** - Visual trend of power usage over time
- 🔋 **Battery status** - Shows battery percentage, charging status, and power source
- 📉 **Trend analysis** - Indicates if power consumption is increasing, decreasing, or stable
- 📐 **Statistics** - Min, max, and average power consumption
- 🖥️ **Cross-platform** - Works on macOS, Linux, and Windows

## Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/rdegges/powermon.git
cd powermon

# Build
go build -o powermon ./cmd/powermon

# Run
./powermon
```

### Using Go Install

```bash
go install github.com/rdegges/powermon/cmd/powermon@latest
```

## Usage

```bash
# Run with default settings
powermon

# Custom refresh interval (e.g., every 500ms)
powermon -interval 500ms

# Longer history window (e.g., 5 minutes)
powermon -history 5m

# Show version
powermon -version

# Desktop Macs (Mac mini, iMac, Mac Studio, Mac Pro)
# Requires sudo for power monitoring
sudo powermon
```

### Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `q` | Quit the application |
| `c` | Clear history and reset the graph |
| `Ctrl+C` | Quit the application |

## Command-Line Options

| Option | Default | Description |
|--------|---------|-------------|
| `-interval` | `1s` | Refresh interval for power readings |
| `-history` | `2m` | How long to keep readings for the graph |
| `-version` | - | Show version information |

## Platform Support

### macOS 🍎

Works on both laptops and desktop Macs (Mac mini, iMac, Mac Studio, Mac Pro).

#### Laptops (MacBook)
Uses `pmset` and `ioreg` to read battery and power information.
- Battery percentage and charging status from `pmset -g batt`
- Power consumption (watts) from `ioreg -rn AppleSmartBattery`

#### Desktop Macs
Desktop Macs don't have batteries, so power monitoring requires `sudo` to access `powermetrics`:

```bash
# Run with sudo for power data on desktop Macs
sudo powermon
```

This uses Apple's `powermetrics` tool to read CPU, GPU, and ANE power consumption. Without sudo, the app will run but show 0W with a helpful tip.

### Linux 🐧

Reads power information from the sysfs filesystem (`/sys/class/power_supply/`).

**Data Sources:**
- Battery capacity from `/sys/class/power_supply/BAT*/capacity`
- Power consumption from `/sys/class/power_supply/BAT*/power_now`
- Charging status from `/sys/class/power_supply/BAT*/status`

### Windows 🪟

Uses PowerShell and WMI queries to read power information.

**Data Sources:**
- Battery status from `Win32_Battery` WMI class
- Power consumption from `BatteryStatus` WMI namespace

## Development

### Prerequisites

- Go 1.21 or later

### Project Structure

```
powermon/
├── cmd/
│   └── powermon/
│       └── main.go          # CLI entry point
├── internal/
│   ├── power/
│   │   ├── power.go         # Core types and history
│   │   ├── power_test.go    # Core tests
│   │   ├── mock_monitor.go  # Mock for testing
│   │   ├── monitor_darwin.go   # macOS implementation
│   │   ├── monitor_linux.go    # Linux implementation
│   │   └── monitor_windows.go  # Windows implementation
│   └── ui/
│       ├── model.go         # Terminal UI model
│       └── model_test.go    # UI tests
├── go.mod
├── go.sum
└── README.md
```

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with verbose output
go test ./... -v

# Run tests with coverage
go test ./... -cover

# Run benchmarks
go test ./internal/power -bench=.
```

### Building for Different Platforms

```bash
# macOS (Apple Silicon)
GOOS=darwin GOARCH=arm64 go build -o powermon-darwin-arm64 ./cmd/powermon

# macOS (Intel)
GOOS=darwin GOARCH=amd64 go build -o powermon-darwin-amd64 ./cmd/powermon

# Linux
GOOS=linux GOARCH=amd64 go build -o powermon-linux-amd64 ./cmd/powermon

# Windows
GOOS=windows GOARCH=amd64 go build -o powermon-windows-amd64.exe ./cmd/powermon
```

## Architecture

The application is designed with clean separation of concerns:

1. **Power Package** (`internal/power/`)
   - `Monitor` interface for platform abstraction
   - `History` for tracking readings over time with pruning
   - Platform-specific implementations with build tags

2. **UI Package** (`internal/ui/`)
   - Bubble Tea model for the interactive terminal UI
   - Lipgloss styles for beautiful rendering
   - Responsive design that adapts to terminal size

3. **Main** (`cmd/powermon/`)
   - CLI argument parsing
   - Application initialization and lifecycle

## Dependencies

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - Terminal UI framework
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) - Style definitions
- [Bubbles](https://github.com/charmbracelet/bubbles) - UI components

## License

MIT License - see LICENSE file for details.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

