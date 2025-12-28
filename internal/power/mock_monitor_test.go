package power

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestMockMonitor(t *testing.T) {
	t.Run("implements Monitor interface", func(t *testing.T) {
		var _ Monitor = NewMockMonitor()
	})

	t.Run("default returns reading with base watts", func(t *testing.T) {
		m := NewMockMonitor()
		ctx := context.Background()

		reading, err := m.Read(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if reading.Watts != 10.0 {
			t.Errorf("expected Watts=10.0, got %f", reading.Watts)
		}
	})

	t.Run("returns predefined readings in sequence", func(t *testing.T) {
		now := time.Now()
		m := NewMockMonitor().WithReadings(
			Reading{Watts: 5.0, Timestamp: now},
			Reading{Watts: 15.0, Timestamp: now.Add(time.Second)},
			Reading{Watts: 25.0, Timestamp: now.Add(2 * time.Second)},
		)
		ctx := context.Background()

		r1, _ := m.Read(ctx)
		if r1.Watts != 5.0 {
			t.Errorf("expected first reading=5.0, got %f", r1.Watts)
		}

		r2, _ := m.Read(ctx)
		if r2.Watts != 15.0 {
			t.Errorf("expected second reading=15.0, got %f", r2.Watts)
		}

		r3, _ := m.Read(ctx)
		if r3.Watts != 25.0 {
			t.Errorf("expected third reading=25.0, got %f", r3.Watts)
		}

		// Should wrap around
		r4, _ := m.Read(ctx)
		if r4.Watts != 5.0 {
			t.Errorf("expected fourth reading to wrap to 5.0, got %f", r4.Watts)
		}
	})

	t.Run("returns error when configured", func(t *testing.T) {
		expectedErr := errors.New("test error")
		m := NewMockMonitor().WithError(expectedErr)
		ctx := context.Background()

		_, err := m.Read(ctx)
		if !errors.Is(err, expectedErr) {
			t.Errorf("expected error %v, got %v", expectedErr, err)
		}
	})

	t.Run("reports supported status correctly", func(t *testing.T) {
		m := NewMockMonitor()
		if !m.IsSupported() {
			t.Error("expected IsSupported=true by default")
		}

		m.WithSupported(false)
		if m.IsSupported() {
			t.Error("expected IsSupported=false after configuration")
		}
	})

	t.Run("tracks read count", func(t *testing.T) {
		m := NewMockMonitor()
		ctx := context.Background()

		if m.ReadCount() != 0 {
			t.Errorf("expected initial ReadCount=0, got %d", m.ReadCount())
		}

		for i := 0; i < 5; i++ {
			_, _ = m.Read(ctx)
		}

		if m.ReadCount() != 5 {
			t.Errorf("expected ReadCount=5, got %d", m.ReadCount())
		}
	})

	t.Run("auto increment generates increasing values", func(t *testing.T) {
		m := NewMockMonitor().WithAutoIncrement(10.0)
		ctx := context.Background()

		r1, _ := m.Read(ctx)
		r2, _ := m.Read(ctx)
		r3, _ := m.Read(ctx)

		if r1.Watts != 10.0 {
			t.Errorf("expected first reading=10.0, got %f", r1.Watts)
		}
		if r2.Watts != 11.0 {
			t.Errorf("expected second reading=11.0, got %f", r2.Watts)
		}
		if r3.Watts != 12.0 {
			t.Errorf("expected third reading=12.0, got %f", r3.Watts)
		}
	})

	t.Run("reset clears state", func(t *testing.T) {
		expectedErr := errors.New("test error")
		m := NewMockMonitor().WithError(expectedErr)
		ctx := context.Background()

		_, _ = m.Read(ctx)
		_, _ = m.Read(ctx)

		m.Reset()

		if m.ReadCount() != 0 {
			t.Errorf("expected ReadCount=0 after reset, got %d", m.ReadCount())
		}

		// Error should be cleared
		_, err := m.Read(ctx)
		if err != nil {
			t.Errorf("expected no error after reset, got %v", err)
		}
	})
}

// TestMonitorInterface ensures the interface is properly defined
func TestMonitorInterface(t *testing.T) {
	t.Run("Monitor interface has required methods", func(t *testing.T) {
		var m Monitor = NewMockMonitor()

		// These should compile, verifying the interface
		_ = m.Name()
		_ = m.IsSupported()
		_, _ = m.Read(context.Background())
	})
}
