//go:build windows

package power

import (
	"bytes"
	"context"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// WindowsMonitor reads power information on Windows using WMI/PowerShell.
type WindowsMonitor struct{}

// NewWindowsMonitor creates a new Windows power monitor.
func NewWindowsMonitor() *WindowsMonitor {
	return &WindowsMonitor{}
}

// Name returns the name of this monitor.
func (m *WindowsMonitor) Name() string {
	return "windows-wmi"
}

// IsSupported checks if power monitoring is available on this system.
func (m *WindowsMonitor) IsSupported() bool {
	_, err := exec.LookPath("powershell")
	return err == nil
}

// Read returns the current power consumption reading.
func (m *WindowsMonitor) Read(ctx context.Context) (Reading, error) {
	reading := Reading{
		Timestamp:      time.Now(),
		BatteryPercent: -1,
		Source:         m.Name(),
	}

	// Get battery status using PowerShell
	batteryInfo, err := m.getBatteryInfo(ctx)
	if err == nil {
		m.parseBatteryInfo(batteryInfo, &reading)
	}

	// Get power consumption estimate
	if watts, err := m.getEstimatedWatts(ctx); err == nil {
		reading.Watts = watts
	}

	return reading, nil
}

// getBatteryInfo gets battery information via PowerShell/WMI.
func (m *WindowsMonitor) getBatteryInfo(ctx context.Context) (string, error) {
	script := `
		$battery = Get-WmiObject Win32_Battery
		if ($battery) {
			Write-Output "BatteryStatus=$($battery.BatteryStatus)"
			Write-Output "EstimatedChargeRemaining=$($battery.EstimatedChargeRemaining)"
			Write-Output "DesignCapacity=$($battery.DesignCapacity)"
			Write-Output "FullChargeCapacity=$($battery.FullChargeCapacity)"
		}
		$power = Get-WmiObject Win32_PowerMeter -ErrorAction SilentlyContinue
		if ($power) {
			Write-Output "CurrentReading=$($power.CurrentReading)"
		}
	`
	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-Command", script)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return out.String(), nil
}

// parseBatteryInfo parses the PowerShell output.
func (m *WindowsMonitor) parseBatteryInfo(output string, reading *Reading) {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "BatteryStatus":
			// 1 = Discharging, 2 = AC, 3-5 = various charging states
			if status, err := strconv.Atoi(value); err == nil {
				reading.IsOnBattery = status == 1
				reading.IsCharging = status >= 2 && status <= 5 && status != 2
			}
		case "EstimatedChargeRemaining":
			if pct, err := strconv.ParseFloat(value, 64); err == nil {
				reading.BatteryPercent = pct
			}
		case "CurrentReading":
			// Power meter reading in milliwatts
			if mw, err := strconv.ParseFloat(value, 64); err == nil && mw > 0 {
				reading.Watts = mw / 1000.0
			}
		}
	}
}

// getEstimatedWatts tries to estimate power consumption.
func (m *WindowsMonitor) getEstimatedWatts(ctx context.Context) (float64, error) {
	// Try to get power consumption from battery discharge rate
	script := `
		$battery = Get-WmiObject -Class BatteryStatus -Namespace root\wmi -ErrorAction SilentlyContinue
		if ($battery) {
			Write-Output "DischargeRate=$($battery.DischargeRate)"
			Write-Output "Voltage=$($battery.Voltage)"
		}
		# Also try Win32_Battery
		$bat2 = Get-WmiObject Win32_Battery -ErrorAction SilentlyContinue
		if ($bat2) {
			Write-Output "EstimatedRunTime=$($bat2.EstimatedRunTime)"
			Write-Output "DesignVoltage=$($bat2.DesignVoltage)"
		}
	`
	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-Command", script)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return 0, err
	}

	output := out.String()
	var dischargeRate, voltage float64

	// Parse discharge rate (in mW)
	drRe := regexp.MustCompile(`DischargeRate=(\d+)`)
	if matches := drRe.FindStringSubmatch(output); len(matches) >= 2 {
		if v, err := strconv.ParseFloat(matches[1], 64); err == nil {
			dischargeRate = v / 1000.0 // Convert mW to W
		}
	}

	// Parse voltage (in mV)
	vRe := regexp.MustCompile(`Voltage=(\d+)`)
	if matches := vRe.FindStringSubmatch(output); len(matches) >= 2 {
		if v, err := strconv.ParseFloat(matches[1], 64); err == nil {
			voltage = v / 1000.0 // Convert mV to V
		}
	}

	if dischargeRate > 0 {
		return dischargeRate, nil
	}

	// If we have voltage but no discharge rate, we can't calculate watts
	_ = voltage

	return 0, nil
}

// NewMonitor creates the appropriate monitor for this platform.
func NewMonitor() Monitor {
	return NewWindowsMonitor()
}
