// Package main provides the entry point for the powermon CLI application.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/rdegges/powermon/internal/power"
	"github.com/rdegges/powermon/internal/ui"
)

// These variables are set at build time via ldflags
var (
	version   = "dev"
	buildTime = "unknown"
)

func main() {
	// Parse command-line flags
	showVersion := flag.Bool("version", false, "Show version information")
	refreshInterval := flag.Duration("interval", 1*time.Second, "Refresh interval for power readings")
	historyDuration := flag.Duration("history", 2*time.Minute, "How long to keep readings for the graph")

	flag.Parse()

	if *showVersion {
		fmt.Printf("powermon %s\n", version)
		if buildTime != "unknown" {
			fmt.Printf("Built: %s\n", buildTime)
		}
		os.Exit(0)
	}

	// Create the power monitor
	monitor := power.NewMonitor()

	// Check if power monitoring is supported
	if !monitor.IsSupported() {
		fmt.Fprintf(os.Stderr, "Error: Power monitoring is not supported on this system.\n")
		fmt.Fprintf(os.Stderr, "Monitor: %s\n", monitor.Name())
		os.Exit(1)
	}

	// Create UI configuration
	cfg := ui.Config{
		Monitor:         monitor,
		GraphWidth:      ui.DefaultGraphWidth,
		GraphHeight:     ui.DefaultGraphHeight,
		RefreshInterval: *refreshInterval,
		HistoryDuration: *historyDuration,
		MaxHistorySize:  int(historyDuration.Seconds()/refreshInterval.Seconds()) + 100,
	}

	// Create and run the UI
	model := ui.NewModel(cfg)
	p := tea.NewProgram(model, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running power monitor: %v\n", err)
		os.Exit(1)
	}
}
