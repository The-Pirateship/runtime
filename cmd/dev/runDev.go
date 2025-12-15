package dev

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
	"github.com/creack/pty"
	"github.com/spf13/cobra"

	tea "github.com/charmbracelet/bubbletea"
)

func listenForLogs(serviceName string, logChan <-chan logMsg) tea.Cmd {
	return func() tea.Msg {
		return <-logChan
	}
}

func runServicesWithTUI(ctx context.Context, services []Service, logChannels map[string]chan logMsg, ptyMasters map[string]*os.File) {
	var wg sync.WaitGroup

	for _, svc := range services {
		wg.Add(1)
		go func(s Service) {
			defer wg.Done()
			runServiceWithTUI(ctx, s, logChannels[s.Name], ptyMasters)
		}(svc)
	}

	wg.Wait()

	// Close all log channels and pty masters when done
	for _, ch := range logChannels {
		close(ch)
	}
	for _, ptmx := range ptyMasters {
		if ptmx != nil {
			ptmx.Close()
		}
	}
}

func runServiceWithTUI(ctx context.Context, svc Service, logChan chan<- logMsg, ptyMasters map[string]*os.File) {
	parts := strings.Fields(svc.Command)
	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	cmd.Dir = svc.Path

	// Set environment variables to ensure color output
	cmd.Env = append(os.Environ(),
		"FORCE_COLOR=1",
		"TERM=xterm-256color",
		"COLORTERM=truecolor",
	)

	// Create a pseudo-terminal to preserve colors and TUI behavior
	ptmx, err := pty.Start(cmd)
	if err != nil {
		logChan <- logMsg{serviceName: svc.Name, line: fmt.Sprintf("❌ Failed to start: %v", err), isStderr: true}
		return
	}

	// Store the pty master for window resize handling
	ptyMasters[svc.Name] = ptmx

	// Set initial terminal size for nested TUIs
	pty.Setsize(ptmx, &pty.Winsize{Rows: 50, Cols: 120})

	logChan <- logMsg{serviceName: svc.Name, line: "✅ Service started", isStderr: false}

	// Stream logs from the pseudo-terminal
	go streamPtyToChannel(ptmx, logChan, svc.Name)

	// Wait for command to finish
	cmd.Wait()
	logChan <- logMsg{serviceName: svc.Name, line: "🔴 Service stopped", isStderr: false}

	// Clean up
	ptmx.Close()
	delete(ptyMasters, svc.Name)
}

func streamPtyToChannel(ptmx *os.File, logChan chan<- logMsg, serviceName string) {
	scanner := bufio.NewScanner(ptmx)
	// Regex to detect clear screen commands
	clearRegex := regexp.MustCompile(`\x1b\[2J|\x1b\[H\x1b\[2J|\x1b\[3J`)

	for scanner.Scan() {
		line := scanner.Text()

		// Check if this line contains a clear screen command
		if clearRegex.MatchString(line) {
			// Send a special clear command instead of the raw escape sequences
			select {
			case logChan <- logMsg{serviceName: serviceName, line: "___CLEAR_SCREEN___", isStderr: false}:
			default:
				// Channel is full, skip this command
			}
		} else {
			// Normal log line, send as is (assume stdout for PTY output)
			select {
			case logChan <- logMsg{serviceName: serviceName, line: line, isStderr: false}:
			default:
				// Channel is full, skip this log line to avoid blocking
			}
		}
	}
}

func runDev(cmd *cobra.Command, args []string) {

	parsedConfig := parseConfig("runtime.toml")
	if len(parsedConfig.Services) == 0 {
		fmt.Println("No services found")
		return
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Initialize the data model for our TUI
	m := model{
		services:    parsedConfig.Services,
		activeTab:   0,
		viewports:   make(map[string]viewport.Model),
		logChannels: make(map[string]chan logMsg),
		ptyMasters:  make(map[string]*os.File),
		ctx:         ctx,
		cancel:      cancel,
	}

	// Create viewports and log channels for each service
	for _, svc := range parsedConfig.Services {
		vp := viewport.New(100, 25) // Initial size, will be updated on first WindowSizeMsg
		vp.SetContent(fmt.Sprintf("Starting %s...", svc.Name))
		// Enable ANSI processing in the viewport
		vp.Style = lipgloss.NewStyle() // Reset any styles that might interfere
		m.viewports[svc.Name] = vp
		m.logChannels[svc.Name] = make(chan logMsg, 100)
	}

	// Start services in background
	go runServicesWithTUI(ctx, parsedConfig.Services, m.logChannels, m.ptyMasters)

	// Start TUI
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running TUI: %v\n", err)
	}

	cancel()
}
