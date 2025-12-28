package power

import (
	"testing"
	"time"
)

func TestReading(t *testing.T) {
	t.Run("creates reading with all fields", func(t *testing.T) {
		now := time.Now()
		r := Reading{
			Watts:          15.5,
			Timestamp:      now,
			IsOnBattery:    true,
			BatteryPercent: 75.0,
			IsCharging:     false,
			Source:         "test",
		}

		if r.Watts != 15.5 {
			t.Errorf("expected Watts=15.5, got %f", r.Watts)
		}
		if !r.Timestamp.Equal(now) {
			t.Errorf("expected Timestamp=%v, got %v", now, r.Timestamp)
		}
		if !r.IsOnBattery {
			t.Error("expected IsOnBattery=true")
		}
		if r.BatteryPercent != 75.0 {
			t.Errorf("expected BatteryPercent=75.0, got %f", r.BatteryPercent)
		}
		if r.IsCharging {
			t.Error("expected IsCharging=false")
		}
		if r.Source != "test" {
			t.Errorf("expected Source=test, got %s", r.Source)
		}
	})
}

func TestNewHistory(t *testing.T) {
	t.Run("creates empty history with correct capacity", func(t *testing.T) {
		h := NewHistory(100, 5*time.Minute)

		if h.Len() != 0 {
			t.Errorf("expected Len()=0, got %d", h.Len())
		}
		if h.maxSize != 100 {
			t.Errorf("expected maxSize=100, got %d", h.maxSize)
		}
		if h.windowSize != 5*time.Minute {
			t.Errorf("expected windowSize=5m, got %v", h.windowSize)
		}
	})
}

func TestHistory_Add(t *testing.T) {
	t.Run("adds reading to empty history", func(t *testing.T) {
		h := NewHistory(100, 5*time.Minute)
		now := time.Now()

		h.Add(Reading{Watts: 10.0, Timestamp: now})

		if h.Len() != 1 {
			t.Errorf("expected Len()=1, got %d", h.Len())
		}
	})

	t.Run("adds multiple readings in order", func(t *testing.T) {
		h := NewHistory(100, 5*time.Minute)
		now := time.Now()

		h.Add(Reading{Watts: 10.0, Timestamp: now})
		h.Add(Reading{Watts: 20.0, Timestamp: now.Add(1 * time.Second)})
		h.Add(Reading{Watts: 30.0, Timestamp: now.Add(2 * time.Second)})

		if h.Len() != 3 {
			t.Errorf("expected Len()=3, got %d", h.Len())
		}

		readings := h.Readings()
		if readings[0].Watts != 10.0 {
			t.Errorf("expected first reading=10.0, got %f", readings[0].Watts)
		}
		if readings[2].Watts != 30.0 {
			t.Errorf("expected last reading=30.0, got %f", readings[2].Watts)
		}
	})

	t.Run("respects max size limit", func(t *testing.T) {
		h := NewHistory(3, 5*time.Minute)
		now := time.Now()

		for i := 0; i < 5; i++ {
			h.Add(Reading{Watts: float64(i * 10), Timestamp: now.Add(time.Duration(i) * time.Second)})
		}

		if h.Len() != 3 {
			t.Errorf("expected Len()=3, got %d", h.Len())
		}

		// Should have the last 3 readings (20, 30, 40)
		readings := h.Readings()
		if readings[0].Watts != 20.0 {
			t.Errorf("expected first reading=20.0, got %f", readings[0].Watts)
		}
	})

	t.Run("prunes old readings outside time window", func(t *testing.T) {
		h := NewHistory(100, 2*time.Second)
		baseTime := time.Now()

		// Add readings at different times
		h.Add(Reading{Watts: 10.0, Timestamp: baseTime})
		h.Add(Reading{Watts: 20.0, Timestamp: baseTime.Add(1 * time.Second)})
		// At 4 seconds, both first (at 0s) and second (at 1s) are outside the 2s window
		h.Add(Reading{Watts: 30.0, Timestamp: baseTime.Add(4 * time.Second)})

		// Both earlier readings should be pruned because they're more than 2 seconds old relative to the newest
		if h.Len() != 1 {
			t.Errorf("expected Len()=1, got %d", h.Len())
		}

		readings := h.Readings()
		if readings[0].Watts != 30.0 {
			t.Errorf("expected first reading=30.0, got %f", readings[0].Watts)
		}
	})
}

func TestHistory_Readings(t *testing.T) {
	t.Run("returns copy of readings", func(t *testing.T) {
		h := NewHistory(100, 5*time.Minute)
		now := time.Now()

		h.Add(Reading{Watts: 10.0, Timestamp: now})

		readings := h.Readings()
		readings[0].Watts = 999.0 // Modify the copy

		// Original should be unchanged
		original := h.Readings()
		if original[0].Watts != 10.0 {
			t.Errorf("expected original unchanged at 10.0, got %f", original[0].Watts)
		}
	})

	t.Run("returns empty slice for empty history", func(t *testing.T) {
		h := NewHistory(100, 5*time.Minute)

		readings := h.Readings()
		if len(readings) != 0 {
			t.Errorf("expected empty slice, got %d elements", len(readings))
		}
	})
}

func TestHistory_Latest(t *testing.T) {
	t.Run("returns latest reading", func(t *testing.T) {
		h := NewHistory(100, 5*time.Minute)
		now := time.Now()

		h.Add(Reading{Watts: 10.0, Timestamp: now})
		h.Add(Reading{Watts: 20.0, Timestamp: now.Add(1 * time.Second)})
		h.Add(Reading{Watts: 30.0, Timestamp: now.Add(2 * time.Second)})

		latest, ok := h.Latest()
		if !ok {
			t.Error("expected ok=true")
		}
		if latest.Watts != 30.0 {
			t.Errorf("expected Watts=30.0, got %f", latest.Watts)
		}
	})

	t.Run("returns false for empty history", func(t *testing.T) {
		h := NewHistory(100, 5*time.Minute)

		_, ok := h.Latest()
		if ok {
			t.Error("expected ok=false for empty history")
		}
	})
}

func TestHistory_Average(t *testing.T) {
	t.Run("calculates correct average", func(t *testing.T) {
		h := NewHistory(100, 5*time.Minute)
		now := time.Now()

		h.Add(Reading{Watts: 10.0, Timestamp: now})
		h.Add(Reading{Watts: 20.0, Timestamp: now.Add(1 * time.Second)})
		h.Add(Reading{Watts: 30.0, Timestamp: now.Add(2 * time.Second)})

		avg := h.Average()
		if avg != 20.0 {
			t.Errorf("expected average=20.0, got %f", avg)
		}
	})

	t.Run("returns 0 for empty history", func(t *testing.T) {
		h := NewHistory(100, 5*time.Minute)

		avg := h.Average()
		if avg != 0 {
			t.Errorf("expected average=0, got %f", avg)
		}
	})

	t.Run("handles single reading", func(t *testing.T) {
		h := NewHistory(100, 5*time.Minute)
		now := time.Now()

		h.Add(Reading{Watts: 25.5, Timestamp: now})

		avg := h.Average()
		if avg != 25.5 {
			t.Errorf("expected average=25.5, got %f", avg)
		}
	})
}

func TestHistory_Min(t *testing.T) {
	t.Run("finds minimum value", func(t *testing.T) {
		h := NewHistory(100, 5*time.Minute)
		now := time.Now()

		h.Add(Reading{Watts: 20.0, Timestamp: now})
		h.Add(Reading{Watts: 5.0, Timestamp: now.Add(1 * time.Second)})
		h.Add(Reading{Watts: 30.0, Timestamp: now.Add(2 * time.Second)})

		min := h.Min()
		if min != 5.0 {
			t.Errorf("expected min=5.0, got %f", min)
		}
	})

	t.Run("returns 0 for empty history", func(t *testing.T) {
		h := NewHistory(100, 5*time.Minute)

		min := h.Min()
		if min != 0 {
			t.Errorf("expected min=0, got %f", min)
		}
	})
}

func TestHistory_Max(t *testing.T) {
	t.Run("finds maximum value", func(t *testing.T) {
		h := NewHistory(100, 5*time.Minute)
		now := time.Now()

		h.Add(Reading{Watts: 20.0, Timestamp: now})
		h.Add(Reading{Watts: 50.0, Timestamp: now.Add(1 * time.Second)})
		h.Add(Reading{Watts: 30.0, Timestamp: now.Add(2 * time.Second)})

		max := h.Max()
		if max != 50.0 {
			t.Errorf("expected max=50.0, got %f", max)
		}
	})

	t.Run("returns 0 for empty history", func(t *testing.T) {
		h := NewHistory(100, 5*time.Minute)

		max := h.Max()
		if max != 0 {
			t.Errorf("expected max=0, got %f", max)
		}
	})
}

func TestHistory_Trend(t *testing.T) {
	t.Run("detects increasing trend", func(t *testing.T) {
		h := NewHistory(100, 5*time.Minute)
		now := time.Now()

		// Steadily increasing values
		h.Add(Reading{Watts: 10.0, Timestamp: now})
		h.Add(Reading{Watts: 20.0, Timestamp: now.Add(1 * time.Second)})
		h.Add(Reading{Watts: 30.0, Timestamp: now.Add(2 * time.Second)})
		h.Add(Reading{Watts: 40.0, Timestamp: now.Add(3 * time.Second)})

		trend := h.Trend()
		if trend <= 0 {
			t.Errorf("expected positive trend for increasing values, got %f", trend)
		}
	})

	t.Run("detects decreasing trend", func(t *testing.T) {
		h := NewHistory(100, 5*time.Minute)
		now := time.Now()

		// Steadily decreasing values
		h.Add(Reading{Watts: 40.0, Timestamp: now})
		h.Add(Reading{Watts: 30.0, Timestamp: now.Add(1 * time.Second)})
		h.Add(Reading{Watts: 20.0, Timestamp: now.Add(2 * time.Second)})
		h.Add(Reading{Watts: 10.0, Timestamp: now.Add(3 * time.Second)})

		trend := h.Trend()
		if trend >= 0 {
			t.Errorf("expected negative trend for decreasing values, got %f", trend)
		}
	})

	t.Run("detects stable trend", func(t *testing.T) {
		h := NewHistory(100, 5*time.Minute)
		now := time.Now()

		// Constant values
		h.Add(Reading{Watts: 20.0, Timestamp: now})
		h.Add(Reading{Watts: 20.0, Timestamp: now.Add(1 * time.Second)})
		h.Add(Reading{Watts: 20.0, Timestamp: now.Add(2 * time.Second)})
		h.Add(Reading{Watts: 20.0, Timestamp: now.Add(3 * time.Second)})

		trend := h.Trend()
		if trend != 0 {
			t.Errorf("expected zero trend for constant values, got %f", trend)
		}
	})

	t.Run("returns 0 for empty history", func(t *testing.T) {
		h := NewHistory(100, 5*time.Minute)

		trend := h.Trend()
		if trend != 0 {
			t.Errorf("expected trend=0 for empty history, got %f", trend)
		}
	})

	t.Run("returns 0 for single reading", func(t *testing.T) {
		h := NewHistory(100, 5*time.Minute)
		now := time.Now()

		h.Add(Reading{Watts: 20.0, Timestamp: now})

		trend := h.Trend()
		if trend != 0 {
			t.Errorf("expected trend=0 for single reading, got %f", trend)
		}
	})
}

func TestHistory_Clear(t *testing.T) {
	t.Run("clears all readings", func(t *testing.T) {
		h := NewHistory(100, 5*time.Minute)
		now := time.Now()

		h.Add(Reading{Watts: 10.0, Timestamp: now})
		h.Add(Reading{Watts: 20.0, Timestamp: now.Add(1 * time.Second)})

		h.Clear()

		if h.Len() != 0 {
			t.Errorf("expected Len()=0 after Clear(), got %d", h.Len())
		}
	})
}

// Benchmark tests
func BenchmarkHistory_Add(b *testing.B) {
	h := NewHistory(1000, 5*time.Minute)
	now := time.Now()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		h.Add(Reading{Watts: float64(i % 100), Timestamp: now.Add(time.Duration(i) * time.Millisecond)})
	}
}

func BenchmarkHistory_Average(b *testing.B) {
	h := NewHistory(1000, 5*time.Minute)
	now := time.Now()

	// Pre-fill with data
	for i := 0; i < 1000; i++ {
		h.Add(Reading{Watts: float64(i % 100), Timestamp: now.Add(time.Duration(i) * time.Millisecond)})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = h.Average()
	}
}

func BenchmarkHistory_Trend(b *testing.B) {
	h := NewHistory(1000, 5*time.Minute)
	now := time.Now()

	// Pre-fill with data
	for i := 0; i < 1000; i++ {
		h.Add(Reading{Watts: float64(i % 100), Timestamp: now.Add(time.Duration(i) * time.Millisecond)})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = h.Trend()
	}
}
