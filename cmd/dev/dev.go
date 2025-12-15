package dev

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/creack/pty"
	"github.com/spf13/cobra"
)

// TUI Messages
type logMsg struct {
	serviceName string
	line        string
}

type serviceStatusMsg struct {
	serviceName string
	status      string // "started", "stopped", "error"
	err         error
}

// TUI Model
type model struct {
	services    []Service
	activeTab   int
	viewports   map[string]viewport.Model
	logChannels map[string]chan string
	ptyMasters  map[string]*os.File
	ctx         context.Context
	cancel      context.CancelFunc
	ready       bool
	width       int
	height      int
}

// Styles
var (
	tabStyle = lipgloss.NewStyle().Margin(0).
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("236")).
			Padding(0)

	activeTabStyle = lipgloss.NewStyle().Margin(0).
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("69")).
			Padding(0).
			Bold(true)

	viewportStyle = lipgloss.NewStyle().
			BorderForeground(lipgloss.Color("62")).
			Padding(1)
)

// TUI Methods
func (m model) Init() tea.Cmd {
	// Start listening for logs from all services
	cmds := make([]tea.Cmd, len(m.services))
	for i, svc := range m.services {
		cmds[i] = listenForLogs(svc.Name, m.logChannels[svc.Name])
	}
	return tea.Batch(cmds...)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.cancel()
			return m, tea.Quit
		case "shift+right":
			m.activeTab = min(m.activeTab+1, len(m.services)-1)
		case "shift+left":
			m.activeTab = max(m.activeTab-1, 0)
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Update viewports with new size - reserve more space for UI elements
		viewportHeight := m.height - 6 // Reserve space for tabs, borders, and instructions
		viewportWidth := m.width - 6   // Reserve space for viewport borders

		for name, vp := range m.viewports {
			vp.Width = viewportWidth
			vp.Height = viewportHeight
			m.viewports[name] = vp
		}

		// Update pty sizes for nested TUIs
		for serviceName, ptmx := range m.ptyMasters {
			if ptmx != nil {
				pty.Setsize(ptmx, &pty.Winsize{
					Rows: uint16(viewportHeight),
					Cols: uint16(viewportWidth),
				})
			}
			_ = serviceName // Use serviceName to avoid unused variable
		}

		if !m.ready {
			m.ready = true
		}

	case logMsg:
		// Add log line to the appropriate viewport, preserving ANSI sequences
		if vp, exists := m.viewports[msg.serviceName]; exists {
			currentContent := vp.View()
			if currentContent != "" {
				currentContent += "\n"
			}
			// Preserve ANSI sequences by not re-formatting the line
			formattedLine := fmt.Sprintf("%s > %s", msg.serviceName, msg.line)
			currentContent += formattedLine
			vp.SetContent(currentContent)
			vp.GotoBottom()
			m.viewports[msg.serviceName] = vp
		}

		// Continue listening for more logs
		return m, listenForLogs(msg.serviceName, m.logChannels[msg.serviceName])

	case serviceStatusMsg:
		// Add status message to the appropriate viewport
		if vp, exists := m.viewports[msg.serviceName]; exists {
			content := vp.View()
			if content != "" {
				content += "\n"
			}
			statusLine := fmt.Sprintf("[%s] Service %s", time.Now().Format("15:04:05"), msg.status)
			if msg.err != nil {
				statusLine += fmt.Sprintf(": %v", msg.err)
			}
			content += statusLine
			vp.SetContent(content)
			vp.GotoBottom()
			m.viewports[msg.serviceName] = vp
		}
	}

	// Update the active viewport
	if len(m.services) > 0 {
		activeService := m.services[m.activeTab].Name
		if vp, exists := m.viewports[activeService]; exists {
			var cmd tea.Cmd
			vp, cmd = vp.Update(msg)
			m.viewports[activeService] = vp
			return m, cmd
		}
	}

	return m, nil
}

func (m model) View() string {
	if !m.ready {
		return "Loading..."
	}

	if len(m.services) == 0 {
		return "No services found"
	}

	// Render tabs with proper spacing
	var tabs []string
	for i, svc := range m.services {
		style := tabStyle
		if i == m.activeTab {
			style = activeTabStyle
		}
		tabs = append(tabs, style.Render(svc.Name))
	}

	// Join tabs horizontally and ensure they fit within terminal width
	tabsView := lipgloss.JoinHorizontal(lipgloss.Top, tabs...)
	if lipgloss.Width(tabsView) > m.width {
		// If tabs are too wide, truncate or adjust
		tabsView = tabsView[:m.width-3] + "..."
	}

	// Render active viewport
	activeService := m.services[m.activeTab].Name
	var viewportView string
	if vp, exists := m.viewports[activeService]; exists {
		viewportView = viewportStyle.Render(vp.View())
	}

	// Instructions
	instructions := " Press shift+←/→ to switch tabs • Press q or ctrl+c to quit"
	instructionsStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Italic(true)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		tabsView,
		viewportView,
		instructionsStyle.Render(instructions),
	)
}

func listenForLogs(serviceName string, logChan <-chan string) tea.Cmd {
	return func() tea.Msg {
		line := <-logChan
		return logMsg{serviceName: serviceName, line: line}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func RegisterCommand(rootCmd *cobra.Command) {
	runCmd := &cobra.Command{
		Use:   "dev",
		Short: "Run your project locally",
		Run:   runDev,
	}
	rootCmd.AddCommand(runCmd)
}

func runDev(cmd *cobra.Command, args []string) {

	parsedConfig := parseConfig("runtime.toml")
	if len(parsedConfig.Services) == 0 {
		fmt.Println("No services found")
		return
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Initialize TUI model
	m := model{
		services:    parsedConfig.Services,
		activeTab:   0,
		viewports:   make(map[string]viewport.Model),
		logChannels: make(map[string]chan string),
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
		m.logChannels[svc.Name] = make(chan string, 100)
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

func runServicesWithTUI(ctx context.Context, services []Service, logChannels map[string]chan string, ptyMasters map[string]*os.File) {
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

func runServiceWithTUI(ctx context.Context, svc Service, logChan chan<- string, ptyMasters map[string]*os.File) {
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
		logChan <- fmt.Sprintf("❌ Failed to start: %v", err)
		return
	}

	// Store the pty master for window resize handling
	ptyMasters[svc.Name] = ptmx

	// Set initial terminal size for nested TUIs
	pty.Setsize(ptmx, &pty.Winsize{Rows: 50, Cols: 120})

	logChan <- "✅ Service started"

	// Stream logs from the pseudo-terminal
	go streamPtyToChannel(ptmx, logChan)

	// Wait for command to finish
	cmd.Wait()
	logChan <- "🔴 Service stopped"

	// Clean up
	ptmx.Close()
	delete(ptyMasters, svc.Name)
}

func streamPtyToChannel(ptmx *os.File, logChan chan<- string) {
	scanner := bufio.NewScanner(ptmx)
	for scanner.Scan() {
		select {
		case logChan <- scanner.Text():
		default:
			// Channel is full, skip this log line to avoid blocking
		}
	}
}
