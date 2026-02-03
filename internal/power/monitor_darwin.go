//go:build darwin

package power

import (
	"bytes"
	"context"
	"math"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Pre-compiled regexes for parsing ioreg output (performance optimization)
var (
	instantAmperageRe = regexp.MustCompile(`"InstantAmperage"\s*=\s*(\d+)`)
	voltageRe         = regexp.MustCompile(`"Voltage"\s*=\s*(\d+)`)
	amperageRe        = regexp.MustCompile(`"Amperage"\s*=\s*(\d+)`)
	designCapacityRe  = regexp.MustCompile(`"DesignCapacity"\s*=\s*(\d+)`)
	currentCapacityRe = regexp.MustCompile(`"CurrentCapacity"\s*=\s*(\d+)`)
	batteryPercentRe  = regexp.MustCompile(`(\d+)%`)
	// powermetrics output parsing (for desktop Macs)
	cpuPowerRe      = regexp.MustCompile(`CPU Power:\s*([\d.]+)\s*mW`)
	gpuPowerRe      = regexp.MustCompile(`GPU Power:\s*([\d.]+)\s*mW`)
	anePowerRe      = regexp.MustCompile(`ANE Power:\s*([\d.]+)\s*mW`)
	combinedPowerRe = regexp.MustCompile(`Combined Power.*?:\s*([\d.]+)\s*mW`)
	packagePowerRe  = regexp.MustCompile(`Package Power:\s*([\d.]+)\s*mW`)
	// Power telemetry (system load / input power) from ioreg
	systemPowerInRe = regexp.MustCompile(`"SystemPowerIn"\s*=\s*(\d+)`)
	systemLoadRe    = regexp.MustCompile(`"SystemLoad"\s*=\s*(\d+)`)
	systemCurrentInRe = regexp.MustCompile(`"SystemCurrentIn"\s*=\s*(\d+)`)
	systemVoltageInRe = regexp.MustCompile(`"SystemVoltageIn"\s*=\s*(\d+)`)
	batteryPowerRe  = regexp.MustCompile(`"BatteryPower"\s*=\s*(\d+)`)
)

// DarwinMonitor reads power information on macOS using system utilities.
type DarwinMonitor struct {
	hasBattery      bool
	hasRoot         bool
	checkedBattery  bool
	usePowermetrics bool
}

// NewDarwinMonitor creates a new macOS power monitor.
func NewDarwinMonitor() *DarwinMonitor {
	m := &DarwinMonitor{}
	m.detectCapabilities()
	return m
}

// detectCapabilities checks what power monitoring methods are available.
func (m *DarwinMonitor) detectCapabilities() {
	// Check if we have a battery
	cmd := exec.Command("ioreg", "-rn", "AppleSmartBattery")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err == nil && strings.Contains(out.String(), "AppleSmartBattery") {
		m.hasBattery = true
	}
	m.checkedBattery = true

	// Check if we have root privileges (needed for powermetrics on desktops)
	m.hasRoot = os.Geteuid() == 0

	// Use powermetrics if we're on a desktop (no battery) and have root
	m.usePowermetrics = !m.hasBattery && m.hasRoot
}

// Name returns the name of this monitor.
func (m *DarwinMonitor) Name() string {
	if m.usePowermetrics {
		return "macOS-powermetrics"
	}
	if !m.hasBattery {
		return "macOS-desktop"
	}
	return "macOS-battery"
}

// IsSupported checks if power monitoring is available on this system.
func (m *DarwinMonitor) IsSupported() bool {
	// Always supported on macOS - we have fallbacks
	_, err := exec.LookPath("pmset")
	return err == nil
}

// HasBattery returns true if the system has a battery.
func (m *DarwinMonitor) HasBattery() bool {
	return m.hasBattery
}

// NeedsSudo returns true if power monitoring would benefit from sudo.
func (m *DarwinMonitor) NeedsSudo() bool {
	return !m.hasBattery && !m.hasRoot
}

// Read returns the current power consumption reading.
func (m *DarwinMonitor) Read(ctx context.Context) (Reading, error) {
	reading := Reading{
		Timestamp:      time.Now(),
		BatteryPercent: -1, // Default to not available
		Source:         m.Name(),
	}

	// Desktop Mac with root access: use powermetrics
	if m.usePowermetrics {
		return m.readFromPowermetrics(ctx, reading)
	}

	// Get battery info from pmset
	pmsetData, err := m.runPmset(ctx)
	if err != nil {
		return reading, err
	}
	m.parsePmset(pmsetData, &reading)

	// If no battery, we can't get power data without sudo
	if !m.hasBattery {
		// Return reading with 0 watts - UI will show helpful message
		return reading, nil
	}

	// Get ioreg data once (avoid duplicate calls)
	ioregData, err := m.runIoreg(ctx)
	if err != nil {
		return reading, nil
	}

	// Get power consumption from ioreg (Apple Silicon and Intel with power metrics)
	watts := m.parseWattsFromIoreg(ioregData)
	if watts > 0 {
		reading.Watts = watts
	} else {
		// Fallback: estimate based on battery discharge if available
		watts = m.estimateWattsFromIoreg(ioregData)
		if watts > 0 {
			reading.Watts = watts
		}
	}

	return reading, nil
}

// readFromPowermetrics reads power data using powermetrics (requires root).
func (m *DarwinMonitor) readFromPowermetrics(ctx context.Context, reading Reading) (Reading, error) {
	// Run powermetrics for a single sample
	cmd := exec.CommandContext(ctx, "powermetrics",
		"-n", "1", // Single sample
		"-i", "100", // 100ms sample interval
		"--samplers", "cpu_power",
		"-f", "text",
	)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// Fall back to no data
		return reading, nil
	}

	output := out.String()
	reading.Watts = m.parsePowermetrics(output)

	return reading, nil
}

// parsePowermetrics extracts power consumption from powermetrics output.
func (m *DarwinMonitor) parsePowermetrics(output string) float64 {
	var totalWatts float64

	// Try to find Combined Power first (most accurate for total system)
	if matches := combinedPowerRe.FindStringSubmatch(output); len(matches) >= 2 {
		if mw, err := strconv.ParseFloat(matches[1], 64); err == nil {
			return mw / 1000.0 // Convert mW to W
		}
	}

	// Try Package Power (common on Apple Silicon)
	if matches := packagePowerRe.FindStringSubmatch(output); len(matches) >= 2 {
		if mw, err := strconv.ParseFloat(matches[1], 64); err == nil {
			return mw / 1000.0 // Convert mW to W
		}
	}

	// Otherwise, sum CPU + GPU + ANE power
	if matches := cpuPowerRe.FindStringSubmatch(output); len(matches) >= 2 {
		if mw, err := strconv.ParseFloat(matches[1], 64); err == nil {
			totalWatts += mw / 1000.0
		}
	}

	if matches := gpuPowerRe.FindStringSubmatch(output); len(matches) >= 2 {
		if mw, err := strconv.ParseFloat(matches[1], 64); err == nil {
			totalWatts += mw / 1000.0
		}
	}

	if matches := anePowerRe.FindStringSubmatch(output); len(matches) >= 2 {
		if mw, err := strconv.ParseFloat(matches[1], 64); err == nil {
			totalWatts += mw / 1000.0
		}
	}

	return totalWatts
}

// runPmset executes pmset -g batt and returns output.
func (m *DarwinMonitor) runPmset(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "pmset", "-g", "batt")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return out.String(), nil
}

// parsePmset parses pmset -g batt output to extract battery information.
func (m *DarwinMonitor) parsePmset(output string, reading *Reading) {
	lines := strings.Split(output, "\n")

	// First line usually indicates power source
	if len(lines) > 0 {
		firstLine := strings.ToLower(lines[0])
		reading.IsOnBattery = strings.Contains(firstLine, "battery power")
	}

	// Look for battery percentage and charging status
	// Example: "-InternalBattery-0 (id=...)  75%; charging; 1:30 remaining"
	for _, line := range lines {
		if strings.Contains(line, "InternalBattery") || strings.Contains(line, "%") {
			matches := batteryPercentRe.FindStringSubmatch(line)
			if len(matches) >= 2 {
				if pct, err := strconv.ParseFloat(matches[1], 64); err == nil {
					reading.BatteryPercent = pct
				}
			}
			lineLower := strings.ToLower(line)
			// Check for explicit "discharging" or "not charging" first
			if strings.Contains(lineLower, "discharging") || strings.Contains(lineLower, "not charging") {
				reading.IsCharging = false
			} else if strings.Contains(lineLower, "charging") {
				// Only set charging if we didn't find discharging
				reading.IsCharging = true
			}
		}
	}
}

// runIoreg executes ioreg and returns output for AppleSmartBattery.
func (m *DarwinMonitor) runIoreg(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "ioreg", "-rn", "AppleSmartBattery")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return out.String(), nil
}

// parseWattsFromIoreg parses power consumption from ioreg output.
func (m *DarwinMonitor) parseWattsFromIoreg(output string) float64 {
	if watts := m.parseTelemetryWattsFromIoreg(output); watts > 0 {
		return watts
	}

	// Look for InstantAmperage and Voltage to calculate watts
	// Watts = Voltage * Amperage
	var voltage, amperage float64

	// InstantAmperage - stored as unsigned but represents signed value
	// When discharging, it's a large positive number that's actually negative
	if matches := instantAmperageRe.FindStringSubmatch(output); len(matches) >= 2 {
		if v, ok := parseIoregSigned(matches[1]); ok {
			amperage = float64(v) / 1000.0 // Convert mA to A
		}
	}

	// Voltage (in mV)
	if matches := voltageRe.FindStringSubmatch(output); len(matches) >= 2 {
		if v, err := strconv.ParseFloat(matches[1], 64); err == nil {
			voltage = v / 1000.0 // Convert mV to V
		}
	}

	if voltage > 0 && amperage != 0 {
		// Power in watts, use absolute value for display
		watts := voltage * amperage
		if watts < 0 {
			watts = -watts
		}
		return watts
	}

	return 0
}

func (m *DarwinMonitor) parseTelemetryWattsFromIoreg(output string) float64 {
	// Prefer adapter input power when available (AC power).
	if matches := systemPowerInRe.FindStringSubmatch(output); len(matches) >= 2 {
		if v, ok := parseIoregSigned(matches[1]); ok {
			if v != 0 {
				return math.Abs(float64(v)) / 1000.0
			}
		}
	}

	// Fall back to system load (total consumption), available on many Macs.
	if matches := systemLoadRe.FindStringSubmatch(output); len(matches) >= 2 {
		if v, ok := parseIoregSigned(matches[1]); ok {
			if v != 0 {
				return math.Abs(float64(v)) / 1000.0
			}
		}
	}

	// If we have current and voltage in, calculate power.
	if watts := calculateInputPower(output); watts > 0 {
		return watts
	}

	// Last resort: battery power (may be negative when discharging).
	if matches := batteryPowerRe.FindStringSubmatch(output); len(matches) >= 2 {
		if v, ok := parseIoregSigned(matches[1]); ok {
			if v != 0 {
				return math.Abs(float64(v)) / 1000.0
			}
		}
	}

	return 0
}

func calculateInputPower(output string) float64 {
	matchesCurrent := systemCurrentInRe.FindStringSubmatch(output)
	matchesVoltage := systemVoltageInRe.FindStringSubmatch(output)
	if len(matchesCurrent) < 2 || len(matchesVoltage) < 2 {
		return 0
	}

	current, ok := parseIoregSigned(matchesCurrent[1])
	if !ok {
		return 0
	}

	voltage, ok := parseIoregSigned(matchesVoltage[1])
	if !ok {
		return 0
	}

	if current == 0 || voltage == 0 {
		return 0
	}

	// mA * mV = microwatts, convert to watts.
	watts := (float64(current) * float64(voltage)) / 1_000_000.0
	return math.Abs(watts)
}

func parseIoregSigned(value string) (int64, bool) {
	v, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, false
	}

	// If the value looks like a 32-bit two's complement, handle that explicitly.
	if v > math.MaxInt32 && v <= math.MaxUint32 {
		return int64(int32(v)), true
	}

	return int64(v), true
}

// estimateWattsFromIoreg estimates power consumption from ioreg battery data.
func (m *DarwinMonitor) estimateWattsFromIoreg(output string) float64 {
	// Try to calculate from battery capacity and current draw
	var designCapacity, currentCapacity, amperage float64

	// DesignCapacity
	if matches := designCapacityRe.FindStringSubmatch(output); len(matches) >= 2 {
		if v, err := strconv.ParseFloat(matches[1], 64); err == nil {
			designCapacity = v
		}
	}

	// CurrentCapacity
	if matches := currentCapacityRe.FindStringSubmatch(output); len(matches) >= 2 {
		if v, err := strconv.ParseFloat(matches[1], 64); err == nil {
			currentCapacity = v
		}
	}

	// Amperage - stored as unsigned but represents signed value
	if matches := amperageRe.FindStringSubmatch(output); len(matches) >= 2 {
		if v, ok := parseIoregSigned(matches[1]); ok {
			amperage = float64(v) / 1000.0 // Convert mA to A
		}
	}

	// If we have data, try to estimate (assuming ~11.4V typical battery voltage)
	if designCapacity > 0 && currentCapacity > 0 && amperage != 0 {
		// Estimate voltage around 11-12V for typical MacBook battery
		estimatedVoltage := 11.4
		watts := estimatedVoltage * amperage
		if watts < 0 {
			watts = -watts
		}
		return watts
	}

	return 0
}

// NewMonitor creates the appropriate monitor for this platform.
func NewMonitor() Monitor {
	return NewDarwinMonitor()
}
