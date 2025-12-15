package dev

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
	ctx         context.Context
	cancel      context.CancelFunc
	ready       bool
	width       int
	height      int
}

// Styles
var (
	tabStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("236")).
			Padding(0, 1)

	activeTabStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("69")).
			Padding(0, 1).
			Bold(true)

	viewportStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
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
		viewportHeight := m.height - 8 // Reserve space for tabs, borders, and instructions
		viewportWidth := m.width - 6   // Reserve space for viewport borders
		
		for name, vp := range m.viewports {
			vp.Width = viewportWidth
			vp.Height = viewportHeight
			m.viewports[name] = vp
		}

		if !m.ready {
			m.ready = true
		}

	case logMsg:
		// Add log line to the appropriate viewport
		if vp, exists := m.viewports[msg.serviceName]; exists {
			content := vp.View()
			if content != "" {
				content += "\n"
			}
			content += fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), msg.line)
			vp.SetContent(content)
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
	instructions := "Press shift+←/→ to switch tabs • Press q or ctrl+c to quit"
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
		ctx:         ctx,
		cancel:      cancel,
	}

	// Create viewports and log channels for each service
	for _, svc := range parsedConfig.Services {
		vp := viewport.New(100, 25) // Initial size, will be updated on first WindowSizeMsg
		vp.SetContent(fmt.Sprintf("Starting %s...", svc.Name))
		m.viewports[svc.Name] = vp
		m.logChannels[svc.Name] = make(chan string, 100)
	}

	// Start services in background
	go runServicesWithTUI(ctx, parsedConfig.Services, m.logChannels)

	// Start TUI
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running TUI: %v\n", err)
	}

	cancel()
}

func runServicesWithTUI(ctx context.Context, services []Service, logChannels map[string]chan string) {
	var wg sync.WaitGroup

	for _, svc := range services {
		wg.Add(1)
		go func(s Service) {
			defer wg.Done()
			runServiceWithTUI(ctx, s, logChannels[s.Name])
		}(svc)
	}

	wg.Wait()

	// Close all log channels when done
	for _, ch := range logChannels {
		close(ch)
	}
}

func runServiceWithTUI(ctx context.Context, svc Service, logChan chan<- string) {
	parts := strings.Fields(svc.Command)
	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	cmd.Dir = svc.Path

	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		logChan <- fmt.Sprintf("❌ Failed to start: %v", err)
		return
	}

	logChan <- "✅ Service started"

	// Stream logs
	var wg sync.WaitGroup
	wg.Add(2)

	go streamLogsToChannel(&wg, stdout, logChan)
	go streamLogsToChannel(&wg, stderr, logChan)

	wg.Wait()
	cmd.Wait()

	logChan <- "🔴 Service stopped"
}

func streamLogsToChannel(wg *sync.WaitGroup, reader io.ReadCloser, logChan chan<- string) {
	defer wg.Done()

	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		select {
		case logChan <- scanner.Text():
		default:
			// Channel is full, skip this log line to avoid blocking
		}
	}
}
