package power

import (
	"context"
	"sync"
	"time"
)

// MockMonitor is a mock implementation of Monitor for testing.
type MockMonitor struct {
	mu            sync.Mutex
	readings      []Reading
	readIndex     int
	supported     bool
	name          string
	err           error
	readCount     int
	autoIncrement bool
	baseWatts     float64
}

// NewMockMonitor creates a new mock monitor.
func NewMockMonitor() *MockMonitor {
	return &MockMonitor{
		supported: true,
		name:      "mock",
		baseWatts: 10.0,
	}
}

// WithReadings sets the readings that will be returned in sequence.
func (m *MockMonitor) WithReadings(readings ...Reading) *MockMonitor {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.readings = readings
	m.readIndex = 0
	return m
}

// WithSupported sets whether the monitor reports as supported.
func (m *MockMonitor) WithSupported(supported bool) *MockMonitor {
	m.supported = supported
	return m
}

// WithError sets an error to be returned on Read.
func (m *MockMonitor) WithError(err error) *MockMonitor {
	m.err = err
	return m
}

// WithAutoIncrement enables automatic watts incrementing for testing trends.
func (m *MockMonitor) WithAutoIncrement(base float64) *MockMonitor {
	m.autoIncrement = true
	m.baseWatts = base
	return m
}

// Name returns the name of this mock monitor.
func (m *MockMonitor) Name() string {
	return m.name
}

// IsSupported returns whether this monitor is supported.
func (m *MockMonitor) IsSupported() bool {
	return m.supported
}

// Read returns the next reading from the configured sequence.
func (m *MockMonitor) Read(ctx context.Context) (Reading, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.readCount++

	if m.err != nil {
		return Reading{}, m.err
	}

	if len(m.readings) > 0 {
		reading := m.readings[m.readIndex]
		m.readIndex = (m.readIndex + 1) % len(m.readings)
		if reading.Timestamp.IsZero() {
			reading.Timestamp = time.Now()
		}
		return reading, nil
	}

	// Generate a reading if no predefined readings
	reading := Reading{
		Watts:          m.baseWatts,
		Timestamp:      time.Now(),
		IsOnBattery:    false,
		BatteryPercent: 75.0,
		IsCharging:     true,
		Source:         m.name,
	}

	if m.autoIncrement {
		reading.Watts = m.baseWatts + float64(m.readCount-1)
	}

	return reading, nil
}

// ReadCount returns how many times Read was called.
func (m *MockMonitor) ReadCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.readCount
}

// Reset resets the mock state.
func (m *MockMonitor) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.readIndex = 0
	m.readCount = 0
	m.err = nil
}
