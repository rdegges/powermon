// Package ui provides the terminal user interface for power monitoring.
package ui

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/rdegges/powermon/internal/power"
)

const (
	// DefaultGraphWidth is the default width of the power graph in characters.
	DefaultGraphWidth = 60
	// DefaultGraphHeight is the default height of the power graph in characters.
	DefaultGraphHeight = 12
	// DefaultRefreshInterval is the default interval between power readings.
	DefaultRefreshInterval = 1 * time.Second
	// DefaultHistoryDuration is how long to keep readings for the graph.
	DefaultHistoryDuration = 2 * time.Minute
)

// Colors and styles
var (
	// Title style
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4")).
			MarginBottom(1)

	// Box style for the main display
	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#7D56F4")).
			Padding(1, 2)

	// Current power display
	powerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#00FF00"))

	// Stats labels
	labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888"))

	// Stats values
	valueStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF"))

	// Trend indicators
	trendUpStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FF5555"))

	trendDownStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#55FF55"))

	trendStableStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFFF55"))

	// Graph colors
	graphBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7D56F4"))

	graphAxisStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#555555"))

	// Battery indicator colors
	batteryHighStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#55FF55"))

	batteryMedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFF55"))

	batteryLowStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FF5555"))

	// Error style
	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF5555"))

	// Help style
	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#555555")).
			MarginTop(1)
)

// tickMsg is sent periodically to trigger power reading updates.
type tickMsg time.Time

// readingMsg contains a new power reading.
type readingMsg struct {
	reading power.Reading
	err     error
}

// Model represents the UI state.
type Model struct {
	monitor         power.Monitor
	history         *power.History
	spinner         spinner.Model
	width           int
	height          int
	graphWidth      int
	graphHeight     int
	refreshInterval time.Duration
	lastReading     power.Reading
	lastError       error
	quitting        bool
	ready           bool
	needsSudo       bool // True if running on desktop Mac without sudo
}

// Config holds configuration options for the UI.
type Config struct {
	Monitor         power.Monitor
	GraphWidth      int
	GraphHeight     int
	RefreshInterval time.Duration
	HistoryDuration time.Duration
	MaxHistorySize  int
}

// DefaultConfig returns a Config with default values.
func DefaultConfig(monitor power.Monitor) Config {
	return Config{
		Monitor:         monitor,
		GraphWidth:      DefaultGraphWidth,
		GraphHeight:     DefaultGraphHeight,
		RefreshInterval: DefaultRefreshInterval,
		HistoryDuration: DefaultHistoryDuration,
		MaxHistorySize:  300, // 5 minutes at 1s intervals
	}
}

// SudoChecker is an optional interface for monitors that may need sudo.
type SudoChecker interface {
	NeedsSudo() bool
}

// NewModel creates a new UI model with the given configuration.
func NewModel(cfg Config) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4"))

	// Check if monitor needs sudo for full functionality
	var needsSudo bool
	if checker, ok := cfg.Monitor.(SudoChecker); ok {
		needsSudo = checker.NeedsSudo()
	}

	return Model{
		monitor:         cfg.Monitor,
		history:         power.NewHistory(cfg.MaxHistorySize, cfg.HistoryDuration),
		spinner:         s,
		graphWidth:      cfg.GraphWidth,
		graphHeight:     cfg.GraphHeight,
		refreshInterval: cfg.RefreshInterval,
		needsSudo:       needsSudo,
	}
}

// Init initializes the model and starts the tick timer.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.tickCmd(),
	)
}

// tickCmd returns a command that sends a tick message after the refresh interval.
func (m Model) tickCmd() tea.Cmd {
	return tea.Tick(m.refreshInterval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// readPowerCmd returns a command that reads power and returns a readingMsg.
func (m Model) readPowerCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		reading, err := m.monitor.Read(ctx)
		return readingMsg{reading: reading, err: err}
	}
}

// Update handles messages and updates the model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "c":
			m.history.Clear()
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Adjust graph size based on terminal size
		m.graphWidth = min(DefaultGraphWidth, msg.Width-20)
		m.graphHeight = min(DefaultGraphHeight, msg.Height-15)
		m.ready = true
		return m, nil

	case tickMsg:
		return m, tea.Batch(m.readPowerCmd(), m.tickCmd())

	case readingMsg:
		m.lastError = msg.err
		if msg.err == nil {
			m.lastReading = msg.reading
			m.history.Add(msg.reading)
		}
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

// View renders the UI.
func (m Model) View() string {
	if m.quitting {
		return "Goodbye!\n"
	}

	if !m.ready {
		return fmt.Sprintf("%s Loading...\n", m.spinner.View())
	}

	var b strings.Builder

	// Title
	b.WriteString(titleStyle.Render("⚡ Power Monitor"))
	b.WriteString("\n\n")

	// Current power reading
	b.WriteString(m.renderCurrentPower())
	b.WriteString("\n\n")

	// Power graph
	b.WriteString(m.renderGraph())
	b.WriteString("\n\n")

	// Statistics
	b.WriteString(m.renderStats())
	b.WriteString("\n")

	// Error display
	if m.lastError != nil {
		b.WriteString("\n")
		b.WriteString(errorStyle.Render(fmt.Sprintf("⚠ Error: %v", m.lastError)))
		b.WriteString("\n")
	}

	// Sudo hint for desktop Macs
	if m.needsSudo && m.lastReading.Watts == 0 {
		b.WriteString("\n")
		b.WriteString(labelStyle.Render("💡 Tip: Run with sudo for power data on desktop Macs:"))
		b.WriteString("\n")
		b.WriteString(valueStyle.Render("   sudo powermon"))
		b.WriteString("\n")
	}

	// Help
	b.WriteString(helpStyle.Render("Press 'q' to quit • 'c' to clear history"))

	return boxStyle.Render(b.String())
}

// renderCurrentPower renders the current power consumption display.
func (m Model) renderCurrentPower() string {
	var b strings.Builder

	// Current watts
	watts := m.lastReading.Watts
	wattsStr := fmt.Sprintf("%.1f W", watts)
	b.WriteString(powerStyle.Render(wattsStr))

	// Trend indicator
	trend := m.history.Trend()
	trendStr := ""
	if trend > 0.5 {
		trendStr = trendUpStyle.Render(" ▲ increasing")
	} else if trend < -0.5 {
		trendStr = trendDownStyle.Render(" ▼ decreasing")
	} else {
		trendStr = trendStableStyle.Render(" ● stable")
	}
	b.WriteString("  " + trendStr)

	// Battery indicator
	if m.lastReading.BatteryPercent >= 0 {
		b.WriteString("  ")
		b.WriteString(m.renderBatteryIndicator())
	}

	return b.String()
}

// renderBatteryIndicator renders the battery status.
func (m Model) renderBatteryIndicator() string {
	pct := m.lastReading.BatteryPercent

	// Choose style based on battery level
	var style lipgloss.Style
	var icon string
	if pct >= 60 {
		style = batteryHighStyle
		icon = "🔋"
	} else if pct >= 20 {
		style = batteryMedStyle
		icon = "🔋"
	} else {
		style = batteryLowStyle
		icon = "🪫"
	}

	status := ""
	if m.lastReading.IsCharging {
		status = " ⚡"
	} else if m.lastReading.IsOnBattery {
		status = " ↓"
	}

	return fmt.Sprintf("%s %s%s", icon, style.Render(fmt.Sprintf("%.0f%%", pct)), status)
}

// renderGraph renders the power consumption graph.
func (m Model) renderGraph() string {
	readings := m.history.Readings()
	if len(readings) == 0 {
		return graphAxisStyle.Render("Waiting for data...")
	}

	// Calculate min/max for scaling
	minVal := m.history.Min()
	maxVal := m.history.Max()

	// Add padding to range
	rangeVal := maxVal - minVal
	if rangeVal < 1.0 {
		rangeVal = 1.0
	}
	minVal = math.Max(0, minVal-rangeVal*0.1)
	maxVal += rangeVal * 0.1

	// Build the graph
	var lines []string

	// Graph header
	lines = append(lines, graphAxisStyle.Render(fmt.Sprintf("Power (%.1f - %.1f W)", minVal, maxVal)))

	// Create graph rows
	blockChars := []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

	// Sample readings to fit graph width
	numPoints := min(m.graphWidth, len(readings))
	sampledReadings := make([]float64, numPoints)

	if numPoints < len(readings) {
		// Sample evenly
		for i := 0; i < numPoints; i++ {
			idx := i * (len(readings) - 1) / (numPoints - 1)
			sampledReadings[i] = readings[idx].Watts
		}
	} else {
		// Use all readings
		for i := 0; i < len(readings); i++ {
			sampledReadings[i] = readings[i].Watts
		}
	}

	// Build sparkline-style graph
	var graphLine strings.Builder
	for _, val := range sampledReadings {
		// Normalize value to 0-1 range
		normalized := (val - minVal) / (maxVal - minVal)
		if normalized < 0 {
			normalized = 0
		}
		if normalized > 1 {
			normalized = 1
		}

		// Map to block character
		charIdx := int(normalized * float64(len(blockChars)-1))
		graphLine.WriteRune(blockChars[charIdx])
	}

	lines = append(lines, graphBarStyle.Render(graphLine.String()))

	// Time axis
	if len(readings) > 0 {
		oldest := readings[0].Timestamp
		newest := readings[len(readings)-1].Timestamp
		duration := newest.Sub(oldest)
		timeLabel := fmt.Sprintf("← %s ago", formatDuration(duration))
		lines = append(lines, graphAxisStyle.Render(timeLabel))
	}

	return strings.Join(lines, "\n")
}

// renderStats renders the statistics section.
func (m Model) renderStats() string {
	var b strings.Builder

	avg := m.history.Average()
	minVal := m.history.Min()
	maxVal := m.history.Max()

	// Stats row
	b.WriteString(labelStyle.Render("Avg: "))
	b.WriteString(valueStyle.Render(fmt.Sprintf("%.1fW", avg)))
	b.WriteString("  ")
	b.WriteString(labelStyle.Render("Min: "))
	b.WriteString(valueStyle.Render(fmt.Sprintf("%.1fW", minVal)))
	b.WriteString("  ")
	b.WriteString(labelStyle.Render("Max: "))
	b.WriteString(valueStyle.Render(fmt.Sprintf("%.1fW", maxVal)))
	b.WriteString("  ")
	b.WriteString(labelStyle.Render("Samples: "))
	b.WriteString(valueStyle.Render(fmt.Sprintf("%d", m.history.Len())))

	// Power source
	b.WriteString("\n")
	b.WriteString(labelStyle.Render("Source: "))
	if m.lastReading.IsOnBattery {
		b.WriteString(valueStyle.Render("Battery"))
	} else {
		b.WriteString(valueStyle.Render("AC Power"))
	}
	b.WriteString("  ")
	b.WriteString(labelStyle.Render("Monitor: "))
	b.WriteString(valueStyle.Render(m.monitor.Name()))

	return b.String()
}

// formatDuration formats a duration as a human-readable string.
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return "0s"
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		mins := int(d.Minutes())
		secs := int(d.Seconds()) % 60
		if secs > 0 {
			return fmt.Sprintf("%dm%ds", mins, secs)
		}
		return fmt.Sprintf("%dm", mins)
	}
	hours := int(d.Hours())
	mins := int(d.Minutes()) % 60
	return fmt.Sprintf("%dh%dm", hours, mins)
}
