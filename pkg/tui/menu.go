package tui

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	TargetDigitalOcean = "digitalocean"
	TargetS3           = "s3"
)

type choice struct {
	name        string
	value       string
	description string
}

var deploymentTargets = []choice{
	{name: "DigitalOcean", value: TargetDigitalOcean, description: "Deploy to a DigitalOcean droplet"},
	{name: "S3 (Static)", value: TargetS3, description: "Deploy static sites to AWS S3"},
}

type menuModel struct {
	choices  []choice
	cursor   int
	selected string
	quitting bool
}

func (m menuModel) Init() tea.Cmd {
	return nil
}

func (m menuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.cursor < len(m.choices)-1 {
				m.cursor++
			}

		case "enter", " ":
			m.selected = m.choices[m.cursor].value
			m.quitting = true
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m menuModel) View() string {
	var s strings.Builder

	// Header
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86"))

	s.WriteString(headerStyle.Render("What deployment target would you like?"))
	s.WriteString("\n\n")

	// Choices
	for i, choice := range m.choices {
		cursor := " "
		if m.cursor == i {
			cursor = "▶"
		}

		checked := " "
		if m.cursor == i {
			checked = "●"
		}

		// Style the current selection
		choiceStyle := lipgloss.NewStyle()
		if m.cursor == i {
			choiceStyle = choiceStyle.Foreground(lipgloss.Color("86"))
		}

		line := fmt.Sprintf("%s [%s] %s", cursor, checked, choice.name)
		s.WriteString(choiceStyle.Render(line))
		s.WriteString("\n")

		// Add description for current selection
		if m.cursor == i {
			descStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("243")).
				MarginLeft(4)
			s.WriteString(descStyle.Render(choice.description))
			s.WriteString("\n")
		}
	}

	s.WriteString("\n")

	// Help text
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))
	s.WriteString(helpStyle.Render("Use ↑/↓ to navigate, Enter to select, q to quit"))

	return s.String()
}

func ShowDeploymentMenu() (string, error) {
	m := menuModel{
		choices: deploymentTargets,
	}

	// Create a new program
	p := tea.NewProgram(m, tea.WithAltScreen())

	// Run the program
	finalModel, err := p.Run()
	if err != nil {
		return "", fmt.Errorf("error running menu: %w", err)
	}

	// Check if user quit without selecting
	final := finalModel.(menuModel)
	if final.quitting && final.selected == "" {
		return "", fmt.Errorf("deployment cancelled")
	}

	return final.selected, nil
}

// Helper function to check if we're in a terminal (for testing/CI)
func IsTerminal() bool {
	return isTerminal(os.Stdout) && isTerminal(os.Stdin)
}

func isTerminal(w io.Writer) bool {
	if f, ok := w.(*os.File); ok {
		// Check if it's a terminal and has a TERM environment variable
		// Also check that we're not in a non-interactive environment
		if os.Getenv("CI") != "" || os.Getenv("TERM") == "dumb" {
			return false
		}
		return f.Fd() == 1 && os.Getenv("TERM") != ""
	}
	return false
}