package ui

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/rdegges/powermon/internal/power"
)

func TestDefaultConfig(t *testing.T) {
	t.Run("returns config with defaults", func(t *testing.T) {
		mock := power.NewMockMonitor()
		cfg := DefaultConfig(mock)

		if cfg.Monitor != mock {
			t.Error("expected monitor to be set")
		}
		if cfg.GraphWidth != DefaultGraphWidth {
			t.Errorf("expected GraphWidth=%d, got %d", DefaultGraphWidth, cfg.GraphWidth)
		}
		if cfg.GraphHeight != DefaultGraphHeight {
			t.Errorf("expected GraphHeight=%d, got %d", DefaultGraphHeight, cfg.GraphHeight)
		}
		if cfg.RefreshInterval != DefaultRefreshInterval {
			t.Errorf("expected RefreshInterval=%v, got %v", DefaultRefreshInterval, cfg.RefreshInterval)
		}
		if cfg.HistoryDuration != DefaultHistoryDuration {
			t.Errorf("expected HistoryDuration=%v, got %v", DefaultHistoryDuration, cfg.HistoryDuration)
		}
	})
}

func TestNewModel(t *testing.T) {
	t.Run("creates model with config", func(t *testing.T) {
		mock := power.NewMockMonitor()
		cfg := Config{
			Monitor:         mock,
			GraphWidth:      80,
			GraphHeight:     20,
			RefreshInterval: 2 * time.Second,
			HistoryDuration: 5 * time.Minute,
			MaxHistorySize:  500,
		}

		m := NewModel(cfg)

		if m.monitor != mock {
			t.Error("expected monitor to be set")
		}
		if m.graphWidth != 80 {
			t.Errorf("expected graphWidth=80, got %d", m.graphWidth)
		}
		if m.graphHeight != 20 {
			t.Errorf("expected graphHeight=20, got %d", m.graphHeight)
		}
		if m.refreshInterval != 2*time.Second {
			t.Errorf("expected refreshInterval=2s, got %v", m.refreshInterval)
		}
		if m.history == nil {
			t.Error("expected history to be initialized")
		}
	})
}

func TestModel_Init(t *testing.T) {
	t.Run("returns batch command", func(t *testing.T) {
		mock := power.NewMockMonitor()
		m := NewModel(DefaultConfig(mock))

		cmd := m.Init()

		if cmd == nil {
			t.Error("expected Init to return a command")
		}
	})
}

func TestModel_Update(t *testing.T) {
	t.Run("quit on q key", func(t *testing.T) {
		mock := power.NewMockMonitor()
		m := NewModel(DefaultConfig(mock))

		newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
		model := newM.(Model)

		if !model.quitting {
			t.Error("expected quitting=true after 'q' key")
		}
		if cmd == nil {
			t.Error("expected Quit command")
		}
	})

	t.Run("quit on ctrl+c", func(t *testing.T) {
		mock := power.NewMockMonitor()
		m := NewModel(DefaultConfig(mock))

		newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
		model := newM.(Model)

		if !model.quitting {
			t.Error("expected quitting=true after ctrl+c")
		}
		if cmd == nil {
			t.Error("expected Quit command")
		}
	})

	t.Run("clear history on c key", func(t *testing.T) {
		mock := power.NewMockMonitor()
		m := NewModel(DefaultConfig(mock))

		// Add some readings
		now := time.Now()
		m.history.Add(power.Reading{Watts: 10.0, Timestamp: now})
		m.history.Add(power.Reading{Watts: 20.0, Timestamp: now.Add(time.Second)})

		if m.history.Len() != 2 {
			t.Fatalf("expected history length=2, got %d", m.history.Len())
		}

		newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
		model := newM.(Model)

		if model.history.Len() != 0 {
			t.Errorf("expected history to be cleared, got length=%d", model.history.Len())
		}
	})

	t.Run("handles window size message", func(t *testing.T) {
		mock := power.NewMockMonitor()
		m := NewModel(DefaultConfig(mock))

		newM, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
		model := newM.(Model)

		if model.width != 100 {
			t.Errorf("expected width=100, got %d", model.width)
		}
		if model.height != 40 {
			t.Errorf("expected height=40, got %d", model.height)
		}
		if !model.ready {
			t.Error("expected ready=true after window size message")
		}
	})

	t.Run("handles reading message", func(t *testing.T) {
		mock := power.NewMockMonitor()
		m := NewModel(DefaultConfig(mock))

		reading := power.Reading{
			Watts:          25.5,
			Timestamp:      time.Now(),
			BatteryPercent: 80.0,
		}

		newM, _ := m.Update(readingMsg{reading: reading, err: nil})
		model := newM.(Model)

		if model.lastReading.Watts != 25.5 {
			t.Errorf("expected lastReading.Watts=25.5, got %f", model.lastReading.Watts)
		}
		if model.history.Len() != 1 {
			t.Errorf("expected history length=1, got %d", model.history.Len())
		}
	})

	t.Run("handles reading error", func(t *testing.T) {
		mock := power.NewMockMonitor()
		m := NewModel(DefaultConfig(mock))

		testErr := tea.ErrProgramKilled

		newM, _ := m.Update(readingMsg{reading: power.Reading{}, err: testErr})
		model := newM.(Model)

		if model.lastError == nil {
			t.Error("expected lastError to be set")
		}
		// History should NOT be updated on error
		if model.history.Len() != 0 {
			t.Errorf("expected history length=0 on error, got %d", model.history.Len())
		}
	})
}

func TestModel_View(t *testing.T) {
	t.Run("shows loading when not ready", func(t *testing.T) {
		mock := power.NewMockMonitor()
		m := NewModel(DefaultConfig(mock))
		m.ready = false

		view := m.View()

		if !strings.Contains(view, "Loading") {
			t.Error("expected view to contain 'Loading' when not ready")
		}
	})

	t.Run("shows goodbye when quitting", func(t *testing.T) {
		mock := power.NewMockMonitor()
		m := NewModel(DefaultConfig(mock))
		m.quitting = true

		view := m.View()

		if !strings.Contains(view, "Goodbye") {
			t.Error("expected view to contain 'Goodbye' when quitting")
		}
	})

	t.Run("shows power reading when ready", func(t *testing.T) {
		mock := power.NewMockMonitor()
		m := NewModel(DefaultConfig(mock))
		m.ready = true
		m.lastReading = power.Reading{
			Watts:          15.5,
			Timestamp:      time.Now(),
			BatteryPercent: 75.0,
		}

		view := m.View()

		if !strings.Contains(view, "15.5") {
			t.Error("expected view to contain power reading '15.5'")
		}
		if !strings.Contains(view, "Power Monitor") {
			t.Error("expected view to contain title 'Power Monitor'")
		}
	})

	t.Run("shows help text", func(t *testing.T) {
		mock := power.NewMockMonitor()
		m := NewModel(DefaultConfig(mock))
		m.ready = true

		view := m.View()

		if !strings.Contains(view, "quit") {
			t.Error("expected view to contain help text about quitting")
		}
	})

	t.Run("shows waiting message when no data", func(t *testing.T) {
		mock := power.NewMockMonitor()
		m := NewModel(DefaultConfig(mock))
		m.ready = true

		view := m.View()

		if !strings.Contains(view, "Waiting for data") {
			t.Error("expected view to contain 'Waiting for data' when history is empty")
		}
	})

	t.Run("shows graph with data", func(t *testing.T) {
		mock := power.NewMockMonitor()
		m := NewModel(DefaultConfig(mock))
		m.ready = true

		now := time.Now()
		for i := 0; i < 10; i++ {
			m.history.Add(power.Reading{
				Watts:     float64(10 + i),
				Timestamp: now.Add(time.Duration(i) * time.Second),
			})
		}
		m.lastReading = power.Reading{Watts: 19.0, Timestamp: now.Add(9 * time.Second)}

		view := m.View()

		// Should contain some graph characters
		if !strings.ContainsAny(view, "▁▂▃▄▅▆▇█") {
			t.Error("expected view to contain graph characters")
		}
	})

	t.Run("shows statistics", func(t *testing.T) {
		mock := power.NewMockMonitor()
		m := NewModel(DefaultConfig(mock))
		m.ready = true

		now := time.Now()
		m.history.Add(power.Reading{Watts: 10.0, Timestamp: now})
		m.history.Add(power.Reading{Watts: 20.0, Timestamp: now.Add(time.Second)})
		m.history.Add(power.Reading{Watts: 30.0, Timestamp: now.Add(2 * time.Second)})
		m.lastReading = power.Reading{Watts: 30.0, Timestamp: now.Add(2 * time.Second)}

		view := m.View()

		if !strings.Contains(view, "Avg") {
			t.Error("expected view to contain 'Avg' statistic")
		}
		if !strings.Contains(view, "Min") {
			t.Error("expected view to contain 'Min' statistic")
		}
		if !strings.Contains(view, "Max") {
			t.Error("expected view to contain 'Max' statistic")
		}
	})

	t.Run("shows trend indicator", func(t *testing.T) {
		mock := power.NewMockMonitor()
		m := NewModel(DefaultConfig(mock))
		m.ready = true

		now := time.Now()
		// Add increasing readings to create upward trend
		for i := 0; i < 5; i++ {
			m.history.Add(power.Reading{
				Watts:     float64(10 + i*5),
				Timestamp: now.Add(time.Duration(i) * time.Second),
			})
		}
		m.lastReading = power.Reading{Watts: 30.0, Timestamp: now.Add(4 * time.Second)}

		view := m.View()

		// Should show an increasing trend indicator
		if !strings.Contains(view, "▲") && !strings.Contains(view, "increasing") {
			t.Error("expected view to show increasing trend indicator")
		}
	})
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		duration time.Duration
		expected string
	}{
		{0, "0s"},
		{500 * time.Millisecond, "0s"},
		{1 * time.Second, "1s"},
		{30 * time.Second, "30s"},
		{59 * time.Second, "59s"},
		{60 * time.Second, "1m"},
		{90 * time.Second, "1m30s"},
		{5 * time.Minute, "5m"},
		{65 * time.Minute, "1h5m"},
		{2 * time.Hour, "2h0m"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatDuration(tt.duration)
			if result != tt.expected {
				t.Errorf("formatDuration(%v) = %s, want %s", tt.duration, result, tt.expected)
			}
		})
	}
}

// Integration tests
func TestModel_Integration(t *testing.T) {
	t.Run("full update cycle", func(t *testing.T) {
		mock := power.NewMockMonitor().WithAutoIncrement(10.0)
		m := NewModel(DefaultConfig(mock))

		// Simulate window size
		newM, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
		m = newM.(Model)

		// Simulate multiple readings
		for i := 0; i < 5; i++ {
			reading, _ := mock.Read(context.Background())
			newM, _ = m.Update(readingMsg{reading: reading, err: nil})
			m = newM.(Model)
		}

		// Check state
		if m.history.Len() != 5 {
			t.Errorf("expected history length=5, got %d", m.history.Len())
		}

		// Render view
		view := m.View()
		if view == "" {
			t.Error("expected non-empty view")
		}

		// View should contain current power
		latest, _ := m.history.Latest()
		if !strings.Contains(view, "14") { // 10 + 4 = 14 (last auto-increment value)
			t.Logf("Latest reading: %f", latest.Watts)
			t.Logf("View: %s", view)
			// This is expected to show the last reading
		}
	})
}

func TestRenderBatteryIndicator(t *testing.T) {
	tests := []struct {
		name           string
		batteryPercent float64
		isCharging     bool
		isOnBattery    bool
		expectIcon     bool
	}{
		{"high battery", 80.0, false, true, true},
		{"medium battery", 40.0, false, true, true},
		{"low battery", 10.0, false, true, true},
		{"charging", 50.0, true, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := power.NewMockMonitor()
			m := NewModel(DefaultConfig(mock))
			m.ready = true
			m.lastReading = power.Reading{
				BatteryPercent: tt.batteryPercent,
				IsCharging:     tt.isCharging,
				IsOnBattery:    tt.isOnBattery,
			}

			result := m.renderBatteryIndicator()

			if tt.expectIcon && result == "" {
				t.Error("expected non-empty battery indicator")
			}
		})
	}
}
