package dev

import tea "github.com/charmbracelet/bubbletea"

// getServiceColor returns a background color for a service based on its index
func getServiceColor(index int) string {
	colors := []string{
		"22",  // Darker green
		"23",  // Dark cyan
		"54",  // Dark purple
		"94",  // Dark orange/brown
		"58",  // Darker olive green
	}
	return colors[index%len(colors)]
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

func keyMsgToBytes(msg tea.KeyMsg) []byte {
	switch msg.Type {
	case tea.KeyEnter:
		return []byte{'\r'}
	case tea.KeyBackspace:
		return []byte{'\b'}
	case tea.KeyDelete:
		return []byte{'\x7f'}
	case tea.KeyTab:
		return []byte{'\t'}
	case tea.KeyEsc:
		return []byte{'\x1b'}
	case tea.KeyUp:
		return []byte{'\x1b', '[', 'A'}
	case tea.KeyDown:
		return []byte{'\x1b', '[', 'B'}
	case tea.KeyRight:
		return []byte{'\x1b', '[', 'C'}
	case tea.KeyLeft:
		return []byte{'\x1b', '[', 'D'}
	case tea.KeyHome:
		return []byte{'\x1b', '[', 'H'}
	case tea.KeyEnd:
		return []byte{'\x1b', '[', 'F'}
	case tea.KeyPgUp:
		return []byte{'\x1b', '[', '5', '~'}
	case tea.KeyPgDown:
		return []byte{'\x1b', '[', '6', '~'}
	case tea.KeyCtrlA:
		return []byte{'\x01'}
	case tea.KeyCtrlB:
		return []byte{'\x02'}
	case tea.KeyCtrlD:
		return []byte{'\x04'}
	case tea.KeyCtrlE:
		return []byte{'\x05'}
	case tea.KeyCtrlF:
		return []byte{'\x06'}
	case tea.KeyCtrlG:
		return []byte{'\x07'}
	case tea.KeyCtrlH:
		return []byte{'\x08'}
	case tea.KeyCtrlK:
		return []byte{'\x0b'}
	case tea.KeyCtrlL:
		return []byte{'\x0c'}
	case tea.KeyCtrlN:
		return []byte{'\x0e'}
	case tea.KeyCtrlO:
		return []byte{'\x0f'}
	case tea.KeyCtrlP:
		return []byte{'\x10'}
	case tea.KeyCtrlR:
		return []byte{'\x12'}
	case tea.KeyCtrlS:
		return []byte{'\x13'}
	case tea.KeyCtrlT:
		return []byte{'\x14'}
	case tea.KeyCtrlU:
		return []byte{'\x15'}
	case tea.KeyCtrlV:
		return []byte{'\x16'}
	case tea.KeyCtrlW:
		return []byte{'\x17'}
	case tea.KeyCtrlX:
		return []byte{'\x18'}
	case tea.KeyCtrlY:
		return []byte{'\x19'}
	case tea.KeyCtrlZ:
		return []byte{'\x1a'}
	case tea.KeyRunes:
		return []byte(msg.String())
	default:
		// For regular character input
		if len(msg.String()) == 1 {
			return []byte(msg.String())
		}
		return nil
	}
}
