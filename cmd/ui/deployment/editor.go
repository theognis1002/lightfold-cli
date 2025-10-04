package deployment

import (
	"fmt"
	"lightfold/pkg/detector"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle        = lipgloss.NewStyle().Background(lipgloss.Color("#01FAC6")).Foreground(lipgloss.Color("#030303")).Bold(true).Padding(0, 1, 0)
	focusedStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#01FAC6")).Bold(true)
	selectedItemStyle = lipgloss.NewStyle().PaddingLeft(1).Foreground(lipgloss.Color("170")).Bold(true)
	descriptionStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#40BDA3"))
	helpStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	successStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("46")).Bold(true)
	errorStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	mutedStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
)

type EditorMode int

const (
	ModeReview EditorMode = iota
	ModeEditBuild
	ModeEditRun
	ModeConfirm
)

type model struct {
	detection     detector.Detection
	buildCommands []string
	runCommands   []string
	mode          EditorMode
	cursor        int
	editingIndex  int
	editBuffer    string
	confirmed     bool
	cancelled     bool
	err           error
}

// ShowDeploymentEditor displays an interactive editor for build/run commands
func ShowDeploymentEditor(detection detector.Detection, buildCmds, runCmds []string) (bool, []string, []string, error) {
	// Use detection plans as defaults if no custom commands provided
	if len(buildCmds) == 0 {
		buildCmds = detection.BuildPlan
	}
	if len(runCmds) == 0 {
		runCmds = detection.RunPlan
	}

	m := model{
		detection:     detection,
		buildCommands: make([]string, len(buildCmds)),
		runCommands:   make([]string, len(runCmds)),
		mode:          ModeReview,
		cursor:        0,
		editingIndex:  -1,
	}

	// Deep copy to avoid mutations
	copy(m.buildCommands, buildCmds)
	copy(m.runCommands, runCmds)

	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return false, nil, nil, fmt.Errorf("error running editor: %w", err)
	}

	final := finalModel.(model)
	if final.cancelled {
		return false, nil, nil, nil
	}

	return final.confirmed, final.buildCommands, final.runCommands, nil
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.mode {
		case ModeReview:
			return m.handleReviewInput(msg)
		case ModeEditBuild:
			return m.handleEditInput(msg, true)
		case ModeEditRun:
			return m.handleEditInput(msg, false)
		case ModeConfirm:
			return m.handleConfirmInput(msg)
		}
	}
	return m, nil
}

func (m model) handleReviewInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q", "esc":
		m.cancelled = true
		return m, tea.Quit
	case "n", "N":
		m.cancelled = true
		return m, tea.Quit
	case "y", "Y", "enter":
		m.mode = ModeEditBuild
		m.cursor = 0
		return m, nil
	}
	return m, nil
}

func (m model) handleEditInput(msg tea.KeyMsg, isBuild bool) (tea.Model, tea.Cmd) {
	commands := &m.buildCommands
	if !isBuild {
		commands = &m.runCommands
	}

	// If currently editing a command
	if m.editingIndex >= 0 {
		switch msg.String() {
		case "esc":
			m.editingIndex = -1
			m.editBuffer = ""
			return m, nil
		case "enter":
			if m.editBuffer != "" {
				(*commands)[m.editingIndex] = m.editBuffer
			}
			m.editingIndex = -1
			m.editBuffer = ""
			return m, nil
		case "backspace":
			if len(m.editBuffer) > 0 {
				m.editBuffer = m.editBuffer[:len(m.editBuffer)-1]
			}
			return m, nil
		default:
			if len(msg.String()) == 1 {
				m.editBuffer += msg.String()
			}
			return m, nil
		}
	}

	// Navigation and actions when not editing
	switch msg.String() {
	case "ctrl+c":
		m.cancelled = true
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(*commands) {
			m.cursor++
		}
	case "e":
		if m.cursor < len(*commands) {
			m.editingIndex = m.cursor
			m.editBuffer = (*commands)[m.cursor]
		}
	case "d":
		if m.cursor < len(*commands) && len(*commands) > 0 {
			*commands = append((*commands)[:m.cursor], (*commands)[m.cursor+1:]...)
			if m.cursor >= len(*commands) && m.cursor > 0 {
				m.cursor--
			}
		}
	case "a":
		// Add new command
		*commands = append(*commands, "")
		m.cursor = len(*commands) - 1
		m.editingIndex = m.cursor
		m.editBuffer = ""
	case "left", "h":
		// Go back to previous mode
		if isBuild {
			m.mode = ModeReview
			m.cursor = 0
		} else {
			m.mode = ModeEditBuild
			m.cursor = 0
		}
	case "right", "l", "enter":
		// Go to next mode
		if isBuild {
			m.mode = ModeEditRun
			m.cursor = 0
		} else {
			m.mode = ModeConfirm
			m.cursor = 0
		}
	}

	return m, nil
}

func (m model) handleConfirmInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q", "esc", "n", "N":
		m.cancelled = true
		return m, tea.Quit
	case "y", "Y", "enter":
		m.confirmed = true
		return m, tea.Quit
	case "left", "h":
		m.mode = ModeEditRun
		m.cursor = 0
		return m, nil
	}
	return m, nil
}

func (m model) View() string {
	if m.cancelled {
		return "Configuration cancelled.\n"
	}

	if m.confirmed {
		return m.renderSuccess()
	}

	switch m.mode {
	case ModeReview:
		return m.renderReview()
	case ModeEditBuild:
		return m.renderEdit(true)
	case ModeEditRun:
		return m.renderEdit(false)
	case ModeConfirm:
		return m.renderConfirm()
	}

	return ""
}

func (m model) renderReview() string {
	var s strings.Builder

	s.WriteString(titleStyle.Render("Framework Detection Results"))
	s.WriteString("\n\n")

	// Framework info box
	frameworkBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#01FAC6")).
		Padding(1, 2).
		Width(60)

	var content strings.Builder
	content.WriteString(focusedStyle.Render("Framework: "))
	content.WriteString(selectedItemStyle.Render(m.detection.Framework))
	content.WriteString("\n")

	content.WriteString(focusedStyle.Render("Language: "))
	content.WriteString(selectedItemStyle.Render(m.detection.Language))
	content.WriteString("\n\n")

	// Detection signals
	if len(m.detection.Signals) > 0 {
		content.WriteString(focusedStyle.Render("Detection signals:"))
		content.WriteString("\n")
		for _, signal := range m.detection.Signals {
			content.WriteString(successStyle.Render("  ✓ "))
			content.WriteString(descriptionStyle.Render(signal))
			content.WriteString("\n")
		}
	}

	s.WriteString(frameworkBox.Render(content.String()))
	s.WriteString("\n\n")

	s.WriteString(focusedStyle.Render("Would you like to configure deployment for this project?"))
	s.WriteString("\n\n")
	s.WriteString(helpStyle.Render("Press "))
	s.WriteString(focusedStyle.Render("y"))
	s.WriteString(helpStyle.Render(" to continue and customize deployment, "))
	s.WriteString(focusedStyle.Render("n"))
	s.WriteString(helpStyle.Render(" to skip, or "))
	s.WriteString(focusedStyle.Render("q"))
	s.WriteString(helpStyle.Render(" to quit"))

	return s.String()
}

func (m model) renderEdit(isBuild bool) string {
	var s strings.Builder

	title := "Edit Build Commands"
	commands := m.buildCommands
	if !isBuild {
		title = "Edit Run Commands"
		commands = m.runCommands
	}

	s.WriteString(titleStyle.Render(title))
	s.WriteString("\n\n")

	// Show commands list
	for i, cmd := range commands {
		cursor := "  "
		if i == m.cursor {
			cursor = focusedStyle.Render("> ")
		}

		if m.editingIndex == i {
			// Show editing state
			editBox := lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#01FAC6")).
				Padding(0, 1).
				Width(50)
			s.WriteString(cursor)
			s.WriteString(editBox.Render(m.editBuffer + "│"))
			s.WriteString("\n")
		} else {
			// Show command
			s.WriteString(cursor)
			s.WriteString(descriptionStyle.Render(fmt.Sprintf("%d. %s", i+1, cmd)))
			s.WriteString("\n")
		}
	}

	// Show "add new" option
	cursor := "  "
	if m.cursor == len(commands) {
		cursor = focusedStyle.Render("> ")
	}
	s.WriteString(cursor)
	s.WriteString(mutedStyle.Render("+ Add new command"))
	s.WriteString("\n\n")

	if m.err != nil {
		s.WriteString(errorStyle.Render(fmt.Sprintf("Error: %v", m.err)))
		s.WriteString("\n\n")
	}

	// Help text
	if m.editingIndex >= 0 {
		s.WriteString(helpStyle.Render("Editing: Enter to save • Esc to cancel"))
	} else {
		s.WriteString(helpStyle.Render("↑/↓: Navigate • e: Edit • d: Delete • a: Add • → or Enter: Next • ←: Back • Ctrl+C: Cancel"))
	}

	return s.String()
}

func (m model) renderConfirm() string {
	var s strings.Builder

	s.WriteString(titleStyle.Render("Confirm Deployment Configuration"))
	s.WriteString("\n\n")

	// Show build plan
	s.WriteString(focusedStyle.Render("Build Commands:"))
	s.WriteString("\n")
	for i, cmd := range m.buildCommands {
		s.WriteString(fmt.Sprintf("  %s ", successStyle.Render(fmt.Sprintf("%d.", i+1))))
		s.WriteString(descriptionStyle.Render(cmd))
		s.WriteString("\n")
	}
	s.WriteString("\n")

	// Show run plan
	s.WriteString(focusedStyle.Render("Run Commands:"))
	s.WriteString("\n")
	for i, cmd := range m.runCommands {
		s.WriteString(fmt.Sprintf("  %s ", successStyle.Render(fmt.Sprintf("%d.", i+1))))
		s.WriteString(descriptionStyle.Render(cmd))
		s.WriteString("\n")
	}
	s.WriteString("\n")

	s.WriteString(focusedStyle.Render("Proceed with this configuration?"))
	s.WriteString("\n\n")
	s.WriteString(helpStyle.Render("Press "))
	s.WriteString(focusedStyle.Render("y"))
	s.WriteString(helpStyle.Render(" to confirm, "))
	s.WriteString(focusedStyle.Render("n"))
	s.WriteString(helpStyle.Render(" to cancel, or "))
	s.WriteString(focusedStyle.Render("←"))
	s.WriteString(helpStyle.Render(" to go back"))

	return s.String()
}

func (m model) renderSuccess() string {
	var s strings.Builder

	s.WriteString("\n")
	s.WriteString(successStyle.Render("Deployment configuration saved"))
	s.WriteString("\n\n")

	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("86"))
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))

	s.WriteString(labelStyle.Render("Build Commands:"))
	s.WriteString("\n")
	for i, cmd := range m.buildCommands {
		s.WriteString(fmt.Sprintf("  %d. %s\n", i+1, valueStyle.Render(cmd)))
	}
	s.WriteString("\n")

	s.WriteString(labelStyle.Render("Run Commands:"))
	s.WriteString("\n")
	for i, cmd := range m.runCommands {
		s.WriteString(fmt.Sprintf("  %d. %s\n", i+1, valueStyle.Render(cmd)))
	}
	s.WriteString("\n")

	return s.String()
}
