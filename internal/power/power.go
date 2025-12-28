// Package power provides cross-platform power consumption monitoring.
package power

import (
	"context"
	"time"
)

// Reading represents a single power consumption measurement.
type Reading struct {
	// Watts is the current power consumption in watts.
	Watts float64

	// Timestamp is when this reading was taken.
	Timestamp time.Time

	// IsOnBattery indicates if the device is running on battery power.
	IsOnBattery bool

	// BatteryPercent is the current battery percentage (0-100), or -1 if not available.
	BatteryPercent float64

	// IsCharging indicates if the battery is currently charging.
	IsCharging bool

	// Source describes where this reading came from (e.g., "macOS-ioreg", "linux-sysfs").
	Source string
}

// Monitor provides power consumption readings.
type Monitor interface {
	// Read returns the current power consumption reading.
	// Returns an error if power information cannot be obtained.
	Read(ctx context.Context) (Reading, error)

	// IsSupported returns true if power monitoring is supported on this system.
	IsSupported() bool

	// Name returns the name of this monitor implementation.
	Name() string
}

// History stores a rolling window of power readings for trend analysis.
type History struct {
	readings   []Reading
	maxSize    int
	windowSize time.Duration
}

// NewHistory creates a new History with the specified maximum size and time window.
func NewHistory(maxSize int, windowSize time.Duration) *History {
	return &History{
		readings:   make([]Reading, 0, maxSize),
		maxSize:    maxSize,
		windowSize: windowSize,
	}
}

// Add adds a new reading to the history, removing old readings outside the time window.
func (h *History) Add(r Reading) {
	// Remove readings outside the time window
	h.prune(r.Timestamp)

	// Add the new reading
	h.readings = append(h.readings, r)

	// If we exceed max size, remove the oldest
	if len(h.readings) > h.maxSize {
		h.readings = h.readings[1:]
	}
}

// prune removes readings that are older than the time window.
func (h *History) prune(now time.Time) {
	cutoff := now.Add(-h.windowSize)
	startIdx := 0
	for i, r := range h.readings {
		if r.Timestamp.After(cutoff) {
			startIdx = i
			break
		}
		startIdx = i + 1
	}
	if startIdx > 0 && startIdx <= len(h.readings) {
		h.readings = h.readings[startIdx:]
	}
}

// Readings returns a copy of all current readings.
func (h *History) Readings() []Reading {
	result := make([]Reading, len(h.readings))
	copy(result, h.readings)
	return result
}

// Len returns the number of readings in history.
func (h *History) Len() int {
	return len(h.readings)
}

// Latest returns the most recent reading, or an empty Reading if history is empty.
func (h *History) Latest() (Reading, bool) {
	if len(h.readings) == 0 {
		return Reading{}, false
	}
	return h.readings[len(h.readings)-1], true
}

// Average returns the average power consumption over the stored readings.
func (h *History) Average() float64 {
	if len(h.readings) == 0 {
		return 0
	}
	var sum float64
	for _, r := range h.readings {
		sum += r.Watts
	}
	return sum / float64(len(h.readings))
}

// Min returns the minimum power reading in the history.
func (h *History) Min() float64 {
	if len(h.readings) == 0 {
		return 0
	}
	minVal := h.readings[0].Watts
	for _, r := range h.readings[1:] {
		if r.Watts < minVal {
			minVal = r.Watts
		}
	}
	return minVal
}

// Max returns the maximum power reading in the history.
func (h *History) Max() float64 {
	if len(h.readings) == 0 {
		return 0
	}
	maxVal := h.readings[0].Watts
	for _, r := range h.readings[1:] {
		if r.Watts > maxVal {
			maxVal = r.Watts
		}
	}
	return maxVal
}

// Trend calculates the trend direction: positive means increasing consumption,
// negative means decreasing, near zero means stable.
// Uses a simple linear regression slope.
func (h *History) Trend() float64 {
	n := len(h.readings)
	if n < 2 {
		return 0
	}

	// Simple linear regression: calculate slope
	var sumX, sumY, sumXY, sumX2 float64
	for i, r := range h.readings {
		x := float64(i)
		y := r.Watts
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	nf := float64(n)
	denominator := nf*sumX2 - sumX*sumX
	if denominator == 0 {
		return 0
	}

	slope := (nf*sumXY - sumX*sumY) / denominator
	return slope
}

// Clear removes all readings from history.
func (h *History) Clear() {
	h.readings = h.readings[:0]
}
