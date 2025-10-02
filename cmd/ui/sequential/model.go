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
)

type Step struct {
	ID          string
	Title       string
	Description string
	Type        StepType
	Value       string
	Placeholder string
	Required    bool
	Validate    func(string) error
	Options     []string
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

	case "right", "tab":
		if len(currentStep.Options) > 0 {
			return m.cycleOption()
		}

	case "ctrl+v":
		if currentStep.Type == StepTypeText || currentStep.Type == StepTypePassword {
			return m.handleTextInput(msg.String())
		} else if currentStep.Type == StepTypeSSHKey {
			return m.handleSSHKeyInput(msg.String())
		}

	default:
		if currentStep.Type == StepTypeText || currentStep.Type == StepTypePassword {
			return m.handleTextInput(msg.String())
		} else if currentStep.Type == StepTypeSSHKey {
			return m.handleSSHKeyInput(msg.String())
		}
	}

	return m, nil
}

func (m FlowModel) handleEnter() (FlowModel, tea.Cmd) {
	currentStep := m.getCurrentStep()

	if currentStep.Type == StepTypeSSHKey {
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

	if currentStep.Type == StepTypeText || currentStep.Type == StepTypePassword {
		if len(currentStep.Value) > 0 {
			currentStep.Value = currentStep.Value[:len(currentStep.Value)-1]
			m.StepStates[m.CurrentStep] = currentStep
			m.Error = nil
		}
	} else if currentStep.Type == StepTypeSSHKey {
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

func (m FlowModel) renderConfirmInput(step Step) string {
	return "Press Enter to confirm"
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
		help += " • ←: Previous step"
	}
	help += " • Backspace: Delete character • Ctrl+V: Paste • Esc: Cancel"

	return helpStyle.Render(help)
}

func (m FlowModel) renderCompleted() string {
	s := titleStyle.Render("✅ "+m.Title+" Complete") + "\n\n"
	s += "Configuration saved successfully!\n"
	return s
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
