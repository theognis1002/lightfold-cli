package sequential

import (
	"fmt"
	"lightfold/pkg/providers/digitalocean"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle     = lipgloss.NewStyle().Background(lipgloss.Color("#01FAC6")).Foreground(lipgloss.Color("#030303")).Bold(true).Padding(0, 1, 0)
	focusedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#01FAC6")).Bold(true)
	helpStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	errorStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	progressStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
	completedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("46"))
)

type StepType string

const (
	StepTypeText     StepType = "text"
	StepTypePassword StepType = "password"
	StepTypeSSHKey   StepType = "ssh_key"
	StepTypeConfirm  StepType = "confirm"
	StepTypeSelect   StepType = "select"
)

type Step struct {
	ID           string
	Title        string
	Description  string
	Type         StepType
	Value        string
	Placeholder  string
	Required     bool
	Validate     func(string) error
	Options      []string
	OptionLabels []string // Display labels for each option (for StepTypeSelect)
	OptionDescs  []string // Descriptions for each option (for StepTypeSelect)
	Cursor       int      // Current cursor position for StepTypeSelect
}

type FlowModel struct {
	Title       string
	Steps       []Step
	CurrentStep int
	History     []int
	Completed   bool
	Cancelled   bool
	Error       error
	StepStates  map[int]Step
	SSHHandlers map[int]*SSHKeyHandler
	ProjectName string
}

func NewFlow(title string, steps []Step) *FlowModel {
	stepStates := make(map[int]Step)
	sshHandlers := make(map[int]*SSHKeyHandler)

	for i, step := range steps {
		stepStates[i] = step
		if step.Type == StepTypeSSHKey {
			sshHandlers[i] = NewSSHKeyHandler("")
		}
	}

	return &FlowModel{
		Title:       title,
		Steps:       steps,
		CurrentStep: 0,
		History:     []int{},
		StepStates:  stepStates,
		SSHHandlers: sshHandlers,
	}
}

func (m *FlowModel) SetProjectName(projectName string) {
	m.ProjectName = projectName
	for _, handler := range m.SSHHandlers {
		handler.ProjectName = projectName
	}
}

func (m FlowModel) Init() tea.Cmd {
	return nil
}

func (m FlowModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyPress(msg)
	}
	return m, nil
}

func (m FlowModel) View() string {
	if m.Cancelled {
		return "Configuration cancelled.\n"
	}

	if m.Completed {
		return m.renderCompleted()
	}

	return m.renderCurrentStep()
}

func (m FlowModel) handleKeyPress(msg tea.KeyMsg) (FlowModel, tea.Cmd) {
	currentStep := m.getCurrentStep()

	switch msg.String() {
	case "ctrl+c", "esc":
		m.Cancelled = true
		return m, tea.Quit

	case "enter":
		return m.handleEnter()

	case "backspace":
		return m.handleBackspace()

	case "left":
		return m.goBack()

	case "up":
		if currentStep.Type == StepTypeSelect {
			return m.moveSelectCursor(-1)
		}

	case "down":
		if currentStep.Type == StepTypeSelect {
			return m.moveSelectCursor(1)
		}

	case "right", "tab":
		if len(currentStep.Options) > 0 && currentStep.Type != StepTypeSelect {
			return m.cycleOption()
		}

	case "ctrl+v":
		switch currentStep.Type {
		case StepTypeText, StepTypePassword:
			return m.handleTextInput(msg.String())
		case StepTypeSSHKey:
			return m.handleSSHKeyInput(msg.String())
		}

	default:
		switch currentStep.Type {
		case StepTypeText, StepTypePassword:
			return m.handleTextInput(msg.String())
		case StepTypeSSHKey:
			return m.handleSSHKeyInput(msg.String())
		}
	}

	return m, nil
}

func (m FlowModel) handleEnter() (FlowModel, tea.Cmd) {
	currentStep := m.getCurrentStep()

	if currentStep.Type == StepTypeSelect {
		// For select type, confirm the current cursor selection
		if currentStep.Cursor >= 0 && currentStep.Cursor < len(currentStep.Options) {
			currentStep.Value = currentStep.Options[currentStep.Cursor]
		} else if currentStep.Required {
			m.Error = fmt.Errorf("please select an option")
			return m, nil
		}
	} else if currentStep.Type == StepTypeSSHKey {
		if handler, exists := m.SSHHandlers[m.CurrentStep]; exists {
			if err := handler.ProcessInput(currentStep.Value); err != nil {
				m.Error = err
				return m, nil
			}

			if handler.GetFilePath() == "" {
				m.Error = fmt.Errorf("please provide a valid SSH key")
				return m, nil
			}
		}
	} else {
		if currentStep.Required && currentStep.Value == "" {
			m.Error = fmt.Errorf("this field is required")
			return m, nil
		}

		if currentStep.Validate != nil {
			if err := currentStep.Validate(currentStep.Value); err != nil {
				m.Error = err
				return m, nil
			}
		}
	}

	m.Error = nil

	m.StepStates[m.CurrentStep] = currentStep

	if m.CurrentStep >= len(m.Steps)-1 {
		m.Completed = true
		return m, tea.Quit
	}

	m.History = append(m.History, m.CurrentStep)
	m.CurrentStep++

	if m.CurrentStep < len(m.Steps) && m.Steps[m.CurrentStep].ID == "size" {
		m.updateSizeStepIfNeeded()
	}

	return m, nil
}

func (m FlowModel) handleBackspace() (FlowModel, tea.Cmd) {
	currentStep := m.getCurrentStep()

	switch currentStep.Type {
	case StepTypeText, StepTypePassword:
		if len(currentStep.Value) > 0 {
			currentStep.Value = currentStep.Value[:len(currentStep.Value)-1]
			m.StepStates[m.CurrentStep] = currentStep
			m.Error = nil
		}
	case StepTypeSSHKey:
		return m.handleSSHKeyBackspace()
	}

	return m, nil
}

func (m FlowModel) goBack() (FlowModel, tea.Cmd) {
	if len(m.History) == 0 {
		return m, nil
	}

	prevStep := m.History[len(m.History)-1]
	m.History = m.History[:len(m.History)-1]
	m.CurrentStep = prevStep

	m.Error = nil

	return m, nil
}

func (m FlowModel) cycleOption() (FlowModel, tea.Cmd) {
	currentStep := m.getCurrentStep()
	if len(currentStep.Options) == 0 {
		return m, nil
	}

	currentIndex := -1
	for i, option := range currentStep.Options {
		if option == currentStep.Value {
			currentIndex = i
			break
		}
	}

	nextIndex := (currentIndex + 1) % len(currentStep.Options)
	currentStep.Value = currentStep.Options[nextIndex]

	m.StepStates[m.CurrentStep] = currentStep

	return m, nil
}

func (m FlowModel) moveSelectCursor(delta int) (FlowModel, tea.Cmd) {
	currentStep := m.getCurrentStep()
	if len(currentStep.Options) == 0 {
		return m, nil
	}

	newCursor := currentStep.Cursor + delta
	if newCursor < 0 {
		newCursor = 0
	} else if newCursor >= len(currentStep.Options) {
		newCursor = len(currentStep.Options) - 1
	}

	currentStep.Cursor = newCursor
	m.StepStates[m.CurrentStep] = currentStep

	return m, nil
}

func (m FlowModel) handleTextInput(key string) (FlowModel, tea.Cmd) {
	currentStep := m.getCurrentStep()

	if key != "backspace" && key != "enter" && key != "left" && key != "right" &&
		key != "up" && key != "down" && key != "tab" && key != "esc" &&
		key != "ctrl+c" && key != "ctrl+v" {
		if len(key) == 1 {
			currentStep.Value += key
		} else if len(key) > 1 {
			filtered := ""
			for _, r := range key {
				if r >= 32 && r <= 126 || r >= 160 {
					filtered += string(r)
				}
			}
			if filtered != "" {
				currentStep.Value += filtered
			}
		}
	}

	m.StepStates[m.CurrentStep] = currentStep
	m.Error = nil

	return m, nil
}

func (m FlowModel) handleSSHKeyInput(key string) (FlowModel, tea.Cmd) {
	currentStep := m.getCurrentStep()

	handler, exists := m.SSHHandlers[m.CurrentStep]
	if !exists {
		return m.handleTextInput(key)
	}

	if key != "backspace" && key != "enter" && key != "left" && key != "right" &&
		key != "up" && key != "down" && key != "tab" && key != "esc" &&
		key != "ctrl+c" && key != "ctrl+v" {
		if len(key) == 1 {
			currentStep.Value += key
		} else if len(key) > 1 {
			filtered := ""
			for _, r := range key {
				if r >= 32 && r <= 126 || r == '\n' || r == '\r' || r >= 160 {
					filtered += string(r)
				}
			}
			if filtered != "" {
				currentStep.Value += filtered
			}
		}

		if err := handler.ProcessInput(currentStep.Value); err != nil {
		}
	}

	m.StepStates[m.CurrentStep] = currentStep
	m.Error = nil

	return m, nil
}

func (m FlowModel) handleSSHKeyBackspace() (FlowModel, tea.Cmd) {
	currentStep := m.getCurrentStep()

	handler, exists := m.SSHHandlers[m.CurrentStep]
	if !exists {
		if len(currentStep.Value) > 0 {
			currentStep.Value = currentStep.Value[:len(currentStep.Value)-1]
		}
		m.StepStates[m.CurrentStep] = currentStep
		m.Error = nil
		return m, nil
	}

	if len(currentStep.Value) > 0 {
		currentStep.Value = currentStep.Value[:len(currentStep.Value)-1]

		if err := handler.ProcessInput(currentStep.Value); err != nil {
		}
	}

	m.StepStates[m.CurrentStep] = currentStep
	m.Error = nil

	return m, nil
}

func (m *FlowModel) getCurrentStep() Step {
	if stepState, exists := m.StepStates[m.CurrentStep]; exists {
		return stepState
	}
	return m.Steps[m.CurrentStep]
}

func (m FlowModel) renderCurrentStep() string {
	currentStep := m.getCurrentStep()

	var s string

	s += titleStyle.Render(m.Title) + "\n\n"

	s += m.renderProgress() + "\n\n"

	s += focusedStyle.Render(currentStep.Title) + "\n"
	if currentStep.Description != "" {
		s += currentStep.Description + "\n"
	}
	s += "\n"

	s += m.renderInput(currentStep) + "\n\n"

	if m.Error != nil {
		s += errorStyle.Render("Error: "+m.Error.Error()) + "\n\n"
	}

	s += m.renderHelp()

	return s
}

func (m FlowModel) renderInput(step Step) string {
	switch step.Type {
	case StepTypeText, StepTypePassword:
		return m.renderTextInput(step)
	case StepTypeSSHKey:
		return m.renderSSHKeyInput(step)
	case StepTypeConfirm:
		return m.renderConfirmInput(step)
	case StepTypeSelect:
		return m.renderSelectInput(step)
	default:
		return "Unknown step type"
	}
}

func (m FlowModel) renderTextInput(step Step) string {
	inputStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#01FAC6")).
		Padding(0, 1).
		Width(50)

	value := step.Value
	if step.Type == StepTypePassword && value != "" {
		value = lipgloss.NewStyle().Foreground(lipgloss.Color("243")).Render(fmt.Sprintf("[%d characters]", len(value)))
	}

	if value == "" && step.Placeholder != "" {
		value = lipgloss.NewStyle().Foreground(lipgloss.Color("243")).Render(step.Placeholder)
	}

	value += "│"

	return inputStyle.Render(value)
}

func (m FlowModel) renderSSHKeyInput(step Step) string {
	if handler, exists := m.SSHHandlers[m.CurrentStep]; exists {
		return handler.RenderSSHKeyInput(step.Value)
	}

	return m.renderTextInput(step)
}

func (m FlowModel) renderConfirmInput(_ Step) string {
	return "Press Enter to confirm"
}

func (m FlowModel) renderSelectInput(step Step) string {
	var s string

	// Viewport configuration for scrollable list
	const maxVisibleItems = 8
	totalItems := len(step.Options)

	// Calculate viewport window
	start := 0
	end := totalItems

	if totalItems > maxVisibleItems {
		// Center the viewport around the cursor
		start = step.Cursor - maxVisibleItems/2
		end = start + maxVisibleItems

		// Adjust if we're at the beginning
		if start < 0 {
			start = 0
			end = maxVisibleItems
		}

		// Adjust if we're at the end
		if end > totalItems {
			end = totalItems
			start = totalItems - maxVisibleItems
			if start < 0 {
				start = 0
			}
		}
	}

	// Show scroll indicator at top if there are items above
	if start > 0 {
		s += lipgloss.NewStyle().Foreground(lipgloss.Color("243")).Render(fmt.Sprintf("   ↑ %d more above...\n\n", start))
	}

	// Render visible items
	for i := start; i < end; i++ {
		// Use label if available, otherwise fall back to option value
		label := step.Options[i]
		if step.OptionLabels != nil && i < len(step.OptionLabels) && step.OptionLabels[i] != "" {
			label = step.OptionLabels[i]
		}

		cursor := "  "
		titleColor := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))

		if i == step.Cursor {
			cursor = focusedStyle.Render(">")
			titleColor = lipgloss.NewStyle().Foreground(lipgloss.Color("170")).Bold(true)
		}

		// Render label + optional description
		if step.OptionDescs != nil && i < len(step.OptionDescs) && step.OptionDescs[i] != "" {
			descColor := lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
			s += fmt.Sprintf("%s %s %s\n", cursor, titleColor.Render(label), descColor.Render("- "+step.OptionDescs[i]))
		} else {
			s += fmt.Sprintf("%s %s\n", cursor, titleColor.Render(label))
		}
	}

	// Show scroll indicator at bottom if there are items below
	if end < totalItems {
		s += "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("243")).Render(fmt.Sprintf("   ↓ %d more below...", totalItems-end))
	}

	return s
}

func (m FlowModel) renderProgress() string {
	progress := fmt.Sprintf("Step %d of %d", m.CurrentStep+1, len(m.Steps))

	var breadcrumb string
	for i := 0; i < len(m.Steps); i++ {
		if i < m.CurrentStep {
			breadcrumb += completedStyle.Render("●")
		} else if i == m.CurrentStep {
			breadcrumb += focusedStyle.Render("●")
		} else {
			breadcrumb += progressStyle.Render("○")
		}
		if i < len(m.Steps)-1 {
			breadcrumb += " "
		}
	}

	return progressStyle.Render(progress) + "  " + breadcrumb
}

func (m FlowModel) renderHelp() string {
	help := "Enter: Next"
	if len(m.History) > 0 {
		help += " • ←: Back"
	}
	help += " • Esc: Cancel"

	return helpStyle.Render(help)
}

func (m FlowModel) renderCompleted() string {
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))

	var content string

	for i, step := range m.Steps {
		if stepState, exists := m.StepStates[i]; exists {
			label := step.Title + ":"
			value := stepState.Value

			// Don't show password values
			if step.Type == StepTypePassword {
				value = "[hidden]"
			}

			// For SSH keys, show just the filename
			if step.Type == StepTypeSSHKey {
				if handler, exists := m.SSHHandlers[i]; exists && handler.GetFilePath() != "" {
					value = handler.GetFilePath()
				}
			}

			if step.Type == StepTypeSelect && value != "" {
				for optIdx, opt := range step.Options {
					if opt == value {
						if step.OptionLabels != nil && optIdx < len(step.OptionLabels) && step.OptionLabels[optIdx] != "" {
							value = step.OptionLabels[optIdx]
						}
						if step.OptionDescs != nil && optIdx < len(step.OptionDescs) && step.OptionDescs[optIdx] != "" {
							value = value + " - " + step.OptionDescs[optIdx]
						}
						break
					}
				}
			}

			if value != "" {
				content += fmt.Sprintf("%s %s\n", label, value)
			}
		}
	}

	mutedBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("245")).
		Padding(0, 1)

	return "\n" + mutedBox.Render(mutedStyle.Render(content)) + "\n"
}

func (m FlowModel) GetResults() map[string]string {
	results := make(map[string]string)
	for i, step := range m.Steps {
		if stepState, exists := m.StepStates[i]; exists {
			if step.Type == StepTypeSSHKey {
				if handler, exists := m.SSHHandlers[i]; exists && handler.GetFilePath() != "" {
					results[step.ID] = handler.GetFilePath()
				} else {
					results[step.ID] = stepState.Value
				}
			} else {
				results[step.ID] = stepState.Value
			}
		} else {
			results[step.ID] = step.Value
		}
	}
	return results
}

func (m FlowModel) GetSSHKeyInfo(stepID string) (filePath, keyName string) {
	for i, step := range m.Steps {
		if step.ID == stepID && step.Type == StepTypeSSHKey {
			if handler, exists := m.SSHHandlers[i]; exists {
				return handler.GetFilePath(), handler.GetKeyName()
			}
		}
	}
	return "", ""
}

func (m *FlowModel) updateSizeStepIfNeeded() {
	results := m.GetResults()

	apiToken, hasToken := results["api_token"]
	region, hasRegion := results["region"]

	if !hasToken || !hasRegion || apiToken == "" || region == "" {
		return
	}

	client := digitalocean.NewClient(apiToken)

	if err := m.UpdateStepWithDynamicSizes("size", client, region); err != nil {
	}
}
