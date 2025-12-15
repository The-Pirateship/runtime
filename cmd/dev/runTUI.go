package dev

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
	"github.com/creack/pty"

	tea "github.com/charmbracelet/bubbletea"
)

// The bubble tea tui system works on the ELM architecture model, here we have a core "model"
// This model holds all the state for our TUI application

// TUI Model
type model struct {
	services    []Service
	activeTab   int
	viewports   map[string]viewport.Model
	logChannels map[string]chan logMsg
	ptyMasters  map[string]*os.File
	ctx         context.Context
	cancel      context.CancelFunc
	ready       bool
	width       int
	height      int
}

// This is how the model is initialized at the start of the program
func (m model) Init() tea.Cmd {
	// Start listening for logs from all services
	cmds := make([]tea.Cmd, len(m.services))
	for i, svc := range m.services {
		cmds[i] = listenForLogs(svc.Name, m.logChannels[svc.Name])
	}
	return tea.Batch(cmds...)
}

// Helper function to find service by name
func (m model) findServiceByName(name string) *Service {
	for _, svc := range m.services {
		if svc.Name == name {
			return &svc
		}
	}
	return nil
}

// TUI Messages for updates to the model/view
type logMsg struct {
	serviceName string
	line        string
	isStderr    bool
}

type serviceStatusMsg struct {
	serviceName string
	status      string // "started", "stopped", "error"
	err         error
}

// Handle updates to model based on incoming messages
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	// handling keystrokes
	case tea.KeyMsg:
		switch msg.String() {

		// quitting the application
		case "ctrl+c", "q":
			m.cancel()
			return m, tea.Quit

		// switching tabs
		case "shift+right":
			m.activeTab = min(m.activeTab+1, len(m.services)-1)
		case "shift+left":
			m.activeTab = max(m.activeTab-1, 0)
		default:
			// Forward all other keystrokes to the active service's PTY
			if len(m.services) > 0 {
				activeService := m.services[m.activeTab].Name
				if ptmx, exists := m.ptyMasters[activeService]; exists && ptmx != nil {
					keyBytes := keyMsgToBytes(msg)
					if keyBytes != nil {
						ptmx.Write(keyBytes)
					}
				}
			}
		}

	// resizing the terminal window
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Update viewports with new size - make tab bar truly flush to bottom
		viewportHeight := m.height - 1 // Reserve exactly 1 line for tab bar at bottom
		viewportWidth := m.width - 4   // Reserve space for viewport borders

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
		// Handle clear screen commands specially
		if msg.line == "___CLEAR_SCREEN___" {
			// Clear only the viewport content for this service
			if vp, exists := m.viewports[msg.serviceName]; exists {
				vp.SetContent("")
				m.viewports[msg.serviceName] = vp
			}
		} else {
			// Add log line to the appropriate viewport, preserving ANSI sequences
			if vp, exists := m.viewports[msg.serviceName]; exists {
				currentContent := vp.View()
				if currentContent != "" {
					currentContent += "\n"
				}
				// Preserve ANSI sequences by not re-formatting the line
				var formattedLine string
				if service := m.findServiceByName(msg.serviceName); service != nil {
					// Apply colored background to service name
					serviceNameStyle := lipgloss.NewStyle().
						Background(lipgloss.Color(service.Color)).
						Foreground(lipgloss.Color("#FFFFFF")). // White text for contrast
						Padding(0, 1).
						Bold(true)

					// Create half-width box - dark green for stdout, red for stderr
					var boxColor lipgloss.Color
					if msg.isStderr {
						boxColor = lipgloss.Color("196") // Bright red (ANSI 256)
					} else {
						boxColor = lipgloss.Color("28") // Dark green (ANSI 256)
					}

					boxStyle := lipgloss.NewStyle().
						Foreground(boxColor)

					coloredServiceName := serviceNameStyle.Render(msg.serviceName)
					coloredBox := boxStyle.Render("▌")
					formattedLine = fmt.Sprintf("%s%s %s", coloredServiceName, coloredBox, msg.line)
				} else {
					// Fallback if service not found
					formattedLine = fmt.Sprintf("%s %s", msg.serviceName, msg.line)
				}
				currentContent += formattedLine
				vp.SetContent(currentContent)
				vp.GotoBottom()
				m.viewports[msg.serviceName] = vp
			}
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

// Styles
var (
	// Vim-style inactive tab: dark background, light foreground, no margins
	tabStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("238")).
			Foreground(lipgloss.Color("252")).
			Padding(0, 2).
			Margin(0).                                                 // Ensure no margins
			Border(lipgloss.NormalBorder(), false, true, false, true). // Only right border
			BorderForeground(lipgloss.Color("235"))

	// Vim-style active tab: bright background, dark foreground, no margins
	activeTabStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("255")).
			Foreground(lipgloss.Color("16")).
			Bold(true).
			Padding(0, 2).
			Margin(0).                                                 // Ensure no margins
			Border(lipgloss.NormalBorder(), false, true, false, true). // Only right border
			BorderForeground(lipgloss.Color("235"))

	// Tab bar background to fill remaining width with navigation text
	tabBarFillStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("238")).
			Foreground(lipgloss.Color("250")). // Light text for visibility
			Italic(true).
			Align(lipgloss.Right). // Right-align the text
			Margin(0).
			Padding(0, 1) // Small padding for text readability

	// Tab divider: subtle separator
	dividerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("236"))

	viewportStyle = lipgloss.NewStyle().
			BorderForeground(lipgloss.Color("62")).
			Padding(1).
			Margin(0) // Ensure no margins that could create gaps
)

// Render the TUI view based on the current model state
func (m model) View() string {
	if !m.ready {
		return "Loading..."
	}

	if len(m.services) == 0 {
		return "No services found"
	}

	// Render active viewport first (now at top)
	activeService := m.services[m.activeTab].Name
	var viewportView string
	if vp, exists := m.viewports[activeService]; exists {
		viewportView = viewportStyle.Render(vp.View())
	}

	// Render Vim-style tabs that fill the entire width (at bottom)
	var tabs []string
	for i, svc := range m.services {
		style := tabStyle
		if i == m.activeTab {
			style = activeTabStyle
		}
		tabs = append(tabs, style.Render(svc.Name))
	}

	// Join tabs with no spacing (borders will separate them)
	tabsContent := lipgloss.JoinHorizontal(lipgloss.Top, tabs...)
	tabsContentWidth := lipgloss.Width(tabsContent)

	// Calculate remaining space and fill it with navigation instructions
	var tabsView string
	remainingSpace := m.width - tabsContentWidth
	navText := "shift + ←/→ to switch tabs"

	if remainingSpace > len(navText)+4 { // Ensure enough space for text
		filler := tabBarFillStyle.Width(remainingSpace).Render(navText)
		tabsView = lipgloss.JoinHorizontal(lipgloss.Top, tabsContent, filler)
	} else if remainingSpace > 0 {
		// Not enough space for full text, use shortened version or empty
		shortText := "shift ←/→"
		var filler string
		if remainingSpace > len(shortText)+4 {
			filler = tabBarFillStyle.Width(remainingSpace).Render(shortText)
		} else {
			filler = tabBarFillStyle.Width(remainingSpace).Render("")
		}
		tabsView = lipgloss.JoinHorizontal(lipgloss.Top, tabsContent, filler)
	} else {
		// If tabs are too wide, truncate gracefully
		activeService := m.services[m.activeTab].Name
		tabsView = activeTabStyle.Render(activeService)
		remainingSpace = m.width - lipgloss.Width(tabsView)
		if remainingSpace > len(navText)+4 {
			filler := tabBarFillStyle.Width(remainingSpace).Render(navText)
			tabsView = lipgloss.JoinHorizontal(lipgloss.Top, tabsView, filler)
		} else if remainingSpace > 0 {
			filler := tabBarFillStyle.Width(remainingSpace).Render("")
			tabsView = lipgloss.JoinHorizontal(lipgloss.Top, tabsView, filler)
		}
	}

	// Return with tabs at the bottom (Vim-style)
	return lipgloss.JoinVertical(
		lipgloss.Left,
		viewportView,
		tabsView,
	)
}
