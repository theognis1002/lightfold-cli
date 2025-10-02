package detection

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
)

type model struct {
	detection detector.Detection
	confirmed bool
	quitting  bool
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit
		case "y", "Y", "enter":
			m.confirmed = true
			m.quitting = true
			return m, tea.Quit
		case "n", "N", "esc":
			m.quitting = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m model) View() string {
	var s strings.Builder

	// Title
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
		content.WriteString("\n")
	}

	// Build plan
	if len(m.detection.BuildPlan) > 0 {
		content.WriteString(focusedStyle.Render("Build plan:"))
		content.WriteString("\n")
		for i, cmd := range m.detection.BuildPlan {
			content.WriteString(fmt.Sprintf("  %s ", successStyle.Render(fmt.Sprintf("%d.", i+1))))
			content.WriteString(descriptionStyle.Render(cmd))
			content.WriteString("\n")
		}
		content.WriteString("\n")
	}

	// Run plan
	if len(m.detection.RunPlan) > 0 {
		content.WriteString(focusedStyle.Render("Run plan:"))
		content.WriteString("\n")
		for i, cmd := range m.detection.RunPlan {
			content.WriteString(fmt.Sprintf("  %s ", successStyle.Render(fmt.Sprintf("%d.", i+1))))
			content.WriteString(descriptionStyle.Render(cmd))
			content.WriteString("\n")
		}
	}

	s.WriteString(frameworkBox.Render(content.String()))
	s.WriteString("\n\n")

	// Prompt
	s.WriteString(focusedStyle.Render("Would you like to configure deployment for this project?"))
	s.WriteString("\n\n")
	s.WriteString(helpStyle.Render("Press "))
	s.WriteString(focusedStyle.Render("y"))
	s.WriteString(helpStyle.Render(" to continue, "))
	s.WriteString(focusedStyle.Render("n"))
	s.WriteString(helpStyle.Render(" to skip, or "))
	s.WriteString(focusedStyle.Render("q"))
	s.WriteString(helpStyle.Render(" to quit"))

	return s.String()
}

// ShowDetectionResults displays the detection results and asks if user wants to configure deployment
func ShowDetectionResults(detection detector.Detection) (bool, error) {
	m := model{
		detection: detection,
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return false, fmt.Errorf("error showing detection results: %w", err)
	}

	final := finalModel.(model)
	if final.quitting && !final.confirmed {
		return false, nil // User chose not to configure deployment
	}

	return final.confirmed, nil
}
