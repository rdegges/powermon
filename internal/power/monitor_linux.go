//go:build linux

package power

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	powerSupplyPath = "/sys/class/power_supply"
)

// LinuxMonitor reads power information on Linux from sysfs.
type LinuxMonitor struct {
	batteryPath string
	acPath      string
}

// NewLinuxMonitor creates a new Linux power monitor.
func NewLinuxMonitor() *LinuxMonitor {
	m := &LinuxMonitor{}
	m.detectPowerSupplies()
	return m
}

// detectPowerSupplies finds available power supply paths.
func (m *LinuxMonitor) detectPowerSupplies() {
	entries, err := os.ReadDir(powerSupplyPath)
	if err != nil {
		return
	}

	for _, entry := range entries {
		name := entry.Name()
		typePath := filepath.Join(powerSupplyPath, name, "type")
		typeBytes, err := os.ReadFile(typePath)
		if err != nil {
			continue
		}

		supplyType := strings.TrimSpace(string(typeBytes))
		switch supplyType {
		case "Battery":
			if m.batteryPath == "" {
				m.batteryPath = filepath.Join(powerSupplyPath, name)
			}
		case "Mains", "USB", "USB_PD":
			if m.acPath == "" {
				m.acPath = filepath.Join(powerSupplyPath, name)
			}
		}
	}
}

// Name returns the name of this monitor.
func (m *LinuxMonitor) Name() string {
	return "linux-sysfs"
}

// IsSupported checks if power monitoring is available on this system.
func (m *LinuxMonitor) IsSupported() bool {
	_, err := os.Stat(powerSupplyPath)
	return err == nil && (m.batteryPath != "" || m.acPath != "")
}

// Read returns the current power consumption reading.
func (m *LinuxMonitor) Read(ctx context.Context) (Reading, error) {
	reading := Reading{
		Timestamp:      time.Now(),
		BatteryPercent: -1,
		Source:         m.Name(),
	}

	// Check if we're on battery or AC
	if m.acPath != "" {
		online := m.readFile(filepath.Join(m.acPath, "online"))
		reading.IsOnBattery = online != "1"
	}

	// Read battery information
	if m.batteryPath != "" {
		// Get battery percentage
		capacity := m.readFile(filepath.Join(m.batteryPath, "capacity"))
		if pct, err := strconv.ParseFloat(capacity, 64); err == nil {
			reading.BatteryPercent = pct
		} else {
			// Calculate from energy_now/energy_full or charge_now/charge_full
			reading.BatteryPercent = m.calculateBatteryPercent()
		}

		// Check charging status
		status := strings.ToLower(m.readFile(filepath.Join(m.batteryPath, "status")))
		reading.IsCharging = status == "charging"

		// Calculate watts
		reading.Watts = m.calculateWatts()
	}

	return reading, nil
}

// readFile reads and trims a sysfs file.
func (m *LinuxMonitor) readFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// calculateBatteryPercent calculates battery percentage from energy or charge values.
func (m *LinuxMonitor) calculateBatteryPercent() float64 {
	// Try energy-based calculation first
	energyNow := m.readFile(filepath.Join(m.batteryPath, "energy_now"))
	energyFull := m.readFile(filepath.Join(m.batteryPath, "energy_full"))
	if energyNow != "" && energyFull != "" {
		now, err1 := strconv.ParseFloat(energyNow, 64)
		full, err2 := strconv.ParseFloat(energyFull, 64)
		if err1 == nil && err2 == nil && full > 0 {
			return (now / full) * 100.0
		}
	}

	// Try charge-based calculation
	chargeNow := m.readFile(filepath.Join(m.batteryPath, "charge_now"))
	chargeFull := m.readFile(filepath.Join(m.batteryPath, "charge_full"))
	if chargeNow != "" && chargeFull != "" {
		now, err1 := strconv.ParseFloat(chargeNow, 64)
		full, err2 := strconv.ParseFloat(chargeFull, 64)
		if err1 == nil && err2 == nil && full > 0 {
			return (now / full) * 100.0
		}
	}

	return -1
}

// calculateWatts calculates current power consumption in watts.
func (m *LinuxMonitor) calculateWatts() float64 {
	// Try power_now first (in microwatts)
	powerNow := m.readFile(filepath.Join(m.batteryPath, "power_now"))
	if powerNow != "" {
		if p, err := strconv.ParseFloat(powerNow, 64); err == nil {
			return p / 1000000.0 // Convert µW to W
		}
	}

	// Calculate from voltage and current
	voltageNow := m.readFile(filepath.Join(m.batteryPath, "voltage_now"))
	currentNow := m.readFile(filepath.Join(m.batteryPath, "current_now"))
	if voltageNow != "" && currentNow != "" {
		voltage, err1 := strconv.ParseFloat(voltageNow, 64)
		current, err2 := strconv.ParseFloat(currentNow, 64)
		if err1 == nil && err2 == nil {
			// Both are in microunits
			watts := (voltage * current) / 1000000000000.0 // µV * µA = pW, convert to W
			if watts < 0 {
				watts = -watts
			}
			return watts
		}
	}

	return 0
}

// NewMonitor creates the appropriate monitor for this platform.
func NewMonitor() Monitor {
	return NewLinuxMonitor()
}
