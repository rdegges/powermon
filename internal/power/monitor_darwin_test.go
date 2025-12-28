//go:build darwin

package power

import (
	"context"
	"testing"
	"time"
)

func TestDarwinMonitor_Name(t *testing.T) {
	m := NewDarwinMonitor()
	name := m.Name()
	// Name should be one of the valid names based on system configuration
	validNames := []string{"macOS-battery", "macOS-desktop", "macOS-powermetrics"}
	valid := false
	for _, validName := range validNames {
		if name == validName {
			valid = true
			break
		}
	}
	if !valid {
		t.Errorf("expected name to be one of %v, got '%s'", validNames, name)
	}
}

func TestDarwinMonitor_IsSupported(t *testing.T) {
	m := NewDarwinMonitor()
	// On macOS, pmset should always be available
	if !m.IsSupported() {
		t.Error("expected IsSupported=true on macOS")
	}
}

func TestDarwinMonitor_ParsePmset(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantBattery  bool
		wantPercent  float64
		wantCharging bool
	}{
		{
			name: "battery power with percentage",
			input: `Now drawing from 'Battery Power'
 -InternalBattery-0 (id=1234567)	75%; discharging; 3:45 remaining present: true`,
			wantBattery:  true,
			wantPercent:  75.0,
			wantCharging: false,
		},
		{
			name: "AC power charging",
			input: `Now drawing from 'AC Power'
 -InternalBattery-0 (id=1234567)	85%; charging; 1:00 remaining present: true`,
			wantBattery:  false,
			wantPercent:  85.0,
			wantCharging: true,
		},
		{
			name: "AC power fully charged",
			input: `Now drawing from 'AC Power'
 -InternalBattery-0 (id=1234567)	100%; charged; present: true`,
			wantBattery:  false,
			wantPercent:  100.0,
			wantCharging: false,
		},
		{
			name: "battery not charging",
			input: `Now drawing from 'Battery Power'
 -InternalBattery-0 (id=1234567)	50%; not charging; present: true`,
			wantBattery:  true,
			wantPercent:  50.0,
			wantCharging: false,
		},
		{
			name: "low battery",
			input: `Now drawing from 'Battery Power'
 -InternalBattery-0 (id=1234567)	5%; discharging; (no estimate) present: true`,
			wantBattery:  true,
			wantPercent:  5.0,
			wantCharging: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewDarwinMonitor()
			reading := Reading{}
			m.parsePmset(tt.input, &reading)

			if reading.IsOnBattery != tt.wantBattery {
				t.Errorf("IsOnBattery = %v, want %v", reading.IsOnBattery, tt.wantBattery)
			}
			if reading.BatteryPercent != tt.wantPercent {
				t.Errorf("BatteryPercent = %f, want %f", reading.BatteryPercent, tt.wantPercent)
			}
			if reading.IsCharging != tt.wantCharging {
				t.Errorf("IsCharging = %v, want %v", reading.IsCharging, tt.wantCharging)
			}
		})
	}
}

func TestDarwinMonitor_Read(t *testing.T) {
	m := NewDarwinMonitor()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	reading, err := m.Read(ctx)
	if err != nil {
		t.Logf("Read returned error (may be expected on some systems): %v", err)
	}

	// Basic sanity checks
	if reading.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}

	// Source should match monitor name
	if reading.Source != m.Name() {
		t.Errorf("expected source '%s', got '%s'", m.Name(), reading.Source)
	}

	// Battery percent should be between 0-100 or -1 if not available
	if reading.BatteryPercent != -1 && (reading.BatteryPercent < 0 || reading.BatteryPercent > 100) {
		t.Errorf("invalid battery percent: %f", reading.BatteryPercent)
	}

	// Watts should be non-negative
	if reading.Watts < 0 {
		t.Errorf("expected non-negative watts, got %f", reading.Watts)
	}

	// Log the reading for debugging
	t.Logf("Power reading: Watts=%.2f, Battery=%.1f%%, OnBattery=%v, Charging=%v, Source=%s",
		reading.Watts, reading.BatteryPercent, reading.IsOnBattery, reading.IsCharging, reading.Source)
}

func TestNewMonitor_Darwin(t *testing.T) {
	m := NewMonitor()
	if m == nil {
		t.Fatal("NewMonitor returned nil")
	}
	if _, ok := m.(*DarwinMonitor); !ok {
		t.Errorf("expected *DarwinMonitor, got %T", m)
	}
}

func TestDarwinMonitor_ParsePowermetrics(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected float64
	}{
		{
			name: "combined power in mW",
			input: `*** Sampled system activity (100.00ms elapsed) ***
Combined Power (CPU + GPU + ANE): 5432 mW`,
			expected: 5.432,
		},
		{
			name: "package power",
			input: `CPU Power: 1234 mW
Package Power: 8500 mW`,
			expected: 8.5,
		},
		{
			name: "sum of CPU GPU ANE",
			input: `CPU Power: 3000 mW
GPU Power: 2000 mW
ANE Power: 500 mW`,
			expected: 5.5,
		},
		{
			name:     "CPU only",
			input:    `CPU Power: 4200 mW`,
			expected: 4.2,
		},
		{
			name:     "no power data",
			input:    `Some other output without power info`,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewDarwinMonitor()
			result := m.parsePowermetrics(tt.input)

			// Allow small floating point differences
			diff := result - tt.expected
			if diff < 0 {
				diff = -diff
			}
			if diff > 0.001 {
				t.Errorf("parsePowermetrics() = %f, want %f", result, tt.expected)
			}
		})
	}
}

func TestDarwinMonitor_HasBattery(t *testing.T) {
	m := NewDarwinMonitor()
	// Just verify the method exists and returns a bool
	_ = m.HasBattery()
}

func TestDarwinMonitor_NeedsSudo(t *testing.T) {
	m := NewDarwinMonitor()
	// Just verify the method exists and returns a bool
	_ = m.NeedsSudo()
}

// BenchmarkDarwinMonitor_Read benchmarks the Read operation
func BenchmarkDarwinMonitor_Read(b *testing.B) {
	m := NewDarwinMonitor()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = m.Read(ctx)
	}
}
