package tui

import (
	"fmt"
	"lightfold/pkg/config"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type inputField struct {
	label       string
	placeholder string
	value       string
	required    bool
	isPassword  bool
}

type inputModel struct {
	target      string
	fields      []inputField
	currentField int
	completed   bool
	quitting    bool
	result      interface{}
}

func (m inputModel) Init() tea.Cmd {
	return nil
}

func (m inputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.quitting = true
			return m, tea.Quit

		case "enter":
			if m.currentField < len(m.fields)-1 {
				// Move to next field
				m.currentField++
			} else {
				// Complete the form
				m.completed = true
				m.quitting = true

				// Build result based on target
				switch m.target {
				case TargetDigitalOcean:
					m.result = &config.DigitalOceanConfig{
						IP:       m.fields[0].value,
						SSHKey:   m.fields[1].value,
						Username: m.fields[2].value,
					}
				case TargetS3:
					m.result = &config.S3Config{
						Bucket:    m.fields[0].value,
						Region:    m.fields[1].value,
						AccessKey: m.fields[2].value,
						SecretKey: m.fields[3].value,
					}
				}

				return m, tea.Quit
			}

		case "up":
			if m.currentField > 0 {
				m.currentField--
			}

		case "down":
			if m.currentField < len(m.fields)-1 {
				m.currentField++
			}

		case "backspace":
			if len(m.fields[m.currentField].value) > 0 {
				m.fields[m.currentField].value = m.fields[m.currentField].value[:len(m.fields[m.currentField].value)-1]
			}

		default:
			// Add character to current field
			if len(msg.String()) == 1 {
				m.fields[m.currentField].value += msg.String()
			}
		}
	}

	return m, nil
}

func (m inputModel) View() string {
	var s strings.Builder

	// Header
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86"))

	targetName := "DigitalOcean"
	if m.target == TargetS3 {
		targetName = "S3"
	}

	s.WriteString(headerStyle.Render(fmt.Sprintf("Configure %s deployment", targetName)))
	s.WriteString("\n\n")

	// Form fields
	for i, field := range m.fields {
		// Field label
		labelStyle := lipgloss.NewStyle().Bold(true)
		if i == m.currentField {
			labelStyle = labelStyle.Foreground(lipgloss.Color("86"))
		}
		s.WriteString(labelStyle.Render(field.label))
		if field.required {
			s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render(" *"))
		}
		s.WriteString("\n")

		// Input box
		inputStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 1).
			Width(50)

		if i == m.currentField {
			inputStyle = inputStyle.BorderForeground(lipgloss.Color("86"))
		}

		displayValue := field.value
		if field.isPassword && displayValue != "" {
			displayValue = strings.Repeat("*", len(displayValue))
		}

		if displayValue == "" && field.placeholder != "" {
			placeholderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
			displayValue = placeholderStyle.Render(field.placeholder)
		}

		// Add cursor for current field
		if i == m.currentField {
			displayValue += "│"
		}

		s.WriteString(inputStyle.Render(displayValue))
		s.WriteString("\n\n")
	}

	// Help text
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))
	s.WriteString(helpStyle.Render("Use ↑/↓ to navigate fields, Enter to submit, Esc to cancel"))

	return s.String()
}

func ShowDigitalOceanInputs() (*config.DigitalOceanConfig, error) {
	// Default SSH key path
	homeDir, _ := os.UserHomeDir()
	defaultSSHKey := filepath.Join(homeDir, ".ssh", "id_rsa")

	m := inputModel{
		target: TargetDigitalOcean,
		fields: []inputField{
			{
				label:       "Server IP Address",
				placeholder: "192.168.1.100",
				required:    true,
			},
			{
				label:       "SSH Key Path",
				placeholder: defaultSSHKey,
				value:       defaultSSHKey,
				required:    true,
			},
			{
				label:       "Username",
				placeholder: "root",
				value:       "root",
				required:    true,
			},
		},
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("error running input form: %w", err)
	}

	final := finalModel.(inputModel)
	if final.quitting && !final.completed {
		return nil, fmt.Errorf("configuration cancelled")
	}

	if final.result == nil {
		return nil, fmt.Errorf("no configuration provided")
	}

	return final.result.(*config.DigitalOceanConfig), nil
}

func ShowS3Inputs() (*config.S3Config, error) {
	m := inputModel{
		target: TargetS3,
		fields: []inputField{
			{
				label:       "S3 Bucket Name",
				placeholder: "my-static-site",
				required:    true,
			},
			{
				label:       "AWS Region",
				placeholder: "us-east-1",
				value:       "us-east-1",
				required:    true,
			},
			{
				label:       "AWS Access Key (optional)",
				placeholder: "Leave empty to use default credentials",
				required:    false,
			},
			{
				label:       "AWS Secret Key (optional)",
				placeholder: "Leave empty to use default credentials",
				required:    false,
				isPassword:  true,
			},
		},
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("error running input form: %w", err)
	}

	final := finalModel.(inputModel)
	if final.quitting && !final.completed {
		return nil, fmt.Errorf("configuration cancelled")
	}

	if final.result == nil {
		return nil, fmt.Errorf("no configuration provided")
	}

	return final.result.(*config.S3Config), nil
}