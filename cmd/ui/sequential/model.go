// Package sequential provides a step-by-step input flow system
// that allows users to navigate forward and backward through
// configuration steps.
package sequential

import (
	"fmt"

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

// StepType defines the type of input for a step
type StepType string

const (
	StepTypeText     StepType = "text"
	StepTypePassword StepType = "password"
	StepTypeSSHKey   StepType = "ssh_key"
	StepTypeConfirm  StepType = "confirm"
)

// Step represents a single step in the sequential flow
type Step struct {
	ID          string
	Title       string
	Description string
	Type        StepType
	Value       string
	Placeholder string
	Required    bool
	Validate    func(string) error
	Options     []string // For choice-based steps
}

// FlowModel manages the sequential flow of steps
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

// NewFlow creates a new sequential flow
func NewFlow(title string, steps []Step) *FlowModel {
	stepStates := make(map[int]Step)
	sshHandlers := make(map[int]*SSHKeyHandler)

	for i, step := range steps {
		stepStates[i] = step
		// Initialize SSH key handlers for SSH key steps
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

// SetProjectName sets the project name for SSH key file naming
func (m *FlowModel) SetProjectName(projectName string) {
	m.ProjectName = projectName
	// Update all SSH handlers with the project name
	for _, handler := range m.SSHHandlers {
		handler.ProjectName = projectName
	}
}

// Init implements tea.Model
func (m FlowModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model
func (m FlowModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyPress(msg)
	}
	return m, nil
}

// View implements tea.Model
func (m FlowModel) View() string {
	if m.Cancelled {
		return "Configuration cancelled.\n"
	}

	if m.Completed {
		return m.renderCompleted()
	}

	return m.renderCurrentStep()
}

// handleKeyPress processes keyboard input
func (m FlowModel) handleKeyPress(msg tea.KeyMsg) (FlowModel, tea.Cmd) {
	currentStep := m.getCurrentStep()

	switch msg.String() {
	case "ctrl+c", "esc":
		m.Cancelled = true
		return m, tea.Quit

	case "enter":
		return m.handleEnter()

	case "backspace", "left":
		return m.goBack()

	case "right", "tab":
		// For multi-option steps, cycle through options
		if len(currentStep.Options) > 0 {
			return m.cycleOption()
		}

	default:
		// Handle text input for text-based steps
		if currentStep.Type == StepTypeText || currentStep.Type == StepTypePassword {
			return m.handleTextInput(msg.String())
		} else if currentStep.Type == StepTypeSSHKey {
			return m.handleSSHKeyInput(msg.String())
		}
	}

	return m, nil
}

// handleEnter processes the Enter key
func (m FlowModel) handleEnter() (FlowModel, tea.Cmd) {
	currentStep := m.getCurrentStep()

	// Special handling for SSH key steps
	if currentStep.Type == StepTypeSSHKey {
		if handler, exists := m.SSHHandlers[m.CurrentStep]; exists {
			// Process the final SSH key value
			if err := handler.ProcessInput(currentStep.Value); err != nil {
				m.Error = err
				return m, nil
			}

			// Ensure we have a valid file path
			if handler.GetFilePath() == "" {
				m.Error = fmt.Errorf("please provide a valid SSH key")
				return m, nil
			}
		}
	} else {
		// Standard validation for non-SSH steps
		if currentStep.Required && currentStep.Value == "" {
			m.Error = fmt.Errorf("this field is required")
			return m, nil
		}

		// Run custom validation if provided
		if currentStep.Validate != nil {
			if err := currentStep.Validate(currentStep.Value); err != nil {
				m.Error = err
				return m, nil
			}
		}
	}

	// Clear any previous errors
	m.Error = nil

	// Save current step state
	m.StepStates[m.CurrentStep] = currentStep

	// Check if this is the last step
	if m.CurrentStep >= len(m.Steps)-1 {
		m.Completed = true
		return m, tea.Quit
	}

	// Move to next step
	m.History = append(m.History, m.CurrentStep)
	m.CurrentStep++

	return m, nil
}

// goBack navigates to the previous step
func (m FlowModel) goBack() (FlowModel, tea.Cmd) {
	if len(m.History) == 0 {
		// If no history, cancel the flow
		m.Cancelled = true
		return m, tea.Quit
	}

	// Get previous step from history
	prevStep := m.History[len(m.History)-1]
	m.History = m.History[:len(m.History)-1]
	m.CurrentStep = prevStep

	// Clear any errors
	m.Error = nil

	return m, nil
}

// cycleOption cycles through available options for the current step
func (m FlowModel) cycleOption() (FlowModel, tea.Cmd) {
	currentStep := m.getCurrentStep()
	if len(currentStep.Options) == 0 {
		return m, nil
	}

	// Find current option index
	currentIndex := -1
	for i, option := range currentStep.Options {
		if option == currentStep.Value {
			currentIndex = i
			break
		}
	}

	// Cycle to next option
	nextIndex := (currentIndex + 1) % len(currentStep.Options)
	currentStep.Value = currentStep.Options[nextIndex]

	// Update step state
	m.StepStates[m.CurrentStep] = currentStep

	return m, nil
}

// handleTextInput processes text input
func (m FlowModel) handleTextInput(key string) (FlowModel, tea.Cmd) {
	currentStep := m.getCurrentStep()

	if key == "backspace" {
		if len(currentStep.Value) > 0 {
			currentStep.Value = currentStep.Value[:len(currentStep.Value)-1]
		}
	} else if len(key) == 1 {
		currentStep.Value += key
	}

	// Update step state
	m.StepStates[m.CurrentStep] = currentStep
	// Clear errors on input
	m.Error = nil

	return m, nil
}

// handleSSHKeyInput processes SSH key input with the SSH key handler
func (m FlowModel) handleSSHKeyInput(key string) (FlowModel, tea.Cmd) {
	currentStep := m.getCurrentStep()

	// Get SSH handler for this step
	handler, exists := m.SSHHandlers[m.CurrentStep]
	if !exists {
		// Fall back to text input if no handler
		return m.handleTextInput(key)
	}

	// Handle text input for SSH keys
	if key == "backspace" {
		if len(currentStep.Value) > 0 {
			currentStep.Value = currentStep.Value[:len(currentStep.Value)-1]
		}
	} else if len(key) == 1 {
		currentStep.Value += key

		// Process the input through the SSH handler
		if err := handler.ProcessInput(currentStep.Value); err != nil {
			// Don't set error immediately, user might still be typing
			// Error will be checked on Enter
		}
	}

	// Update step state
	m.StepStates[m.CurrentStep] = currentStep
	// Clear errors on input
	m.Error = nil

	return m, nil
}

// getCurrentStep returns the current step with any updates
func (m *FlowModel) getCurrentStep() Step {
	if stepState, exists := m.StepStates[m.CurrentStep]; exists {
		return stepState
	}
	return m.Steps[m.CurrentStep]
}

// renderCurrentStep renders the current step
func (m FlowModel) renderCurrentStep() string {
	currentStep := m.getCurrentStep()

	var s string

	// Title
	s += titleStyle.Render(m.Title) + "\n\n"

	// Progress indicator
	s += m.renderProgress() + "\n\n"

	// Step title and description
	s += focusedStyle.Render(currentStep.Title) + "\n"
	if currentStep.Description != "" {
		s += currentStep.Description + "\n"
	}
	s += "\n"

	// Input area
	s += m.renderInput(currentStep) + "\n\n"

	// Error message
	if m.Error != nil {
		s += errorStyle.Render("Error: "+m.Error.Error()) + "\n\n"
	}

	// Help text
	s += m.renderHelp()

	return s
}

// renderInput renders the input area for the current step
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

// renderTextInput renders a text input field
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

	// Add cursor
	value += "│"

	return inputStyle.Render(value)
}

// renderSSHKeyInput renders SSH key input with options
func (m FlowModel) renderSSHKeyInput(step Step) string {
	// Get SSH handler for this step
	if handler, exists := m.SSHHandlers[m.CurrentStep]; exists {
		return handler.RenderSSHKeyInput(step.Value)
	}

	// Fall back to text input if no handler
	return m.renderTextInput(step)
}

// renderConfirmInput renders a confirmation step
func (m FlowModel) renderConfirmInput(step Step) string {
	// TODO: Implement confirmation rendering
	return "Press Enter to confirm"
}

// renderProgress shows the current step progress
func (m FlowModel) renderProgress() string {
	progress := fmt.Sprintf("Step %d of %d", m.CurrentStep+1, len(m.Steps))

	// Add breadcrumb of completed steps
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

// renderHelp shows help text for navigation
func (m FlowModel) renderHelp() string {
	help := "Enter: Next"
	if len(m.History) > 0 {
		help += " • ← Backspace: Previous"
	}
	help += " • Esc: Cancel"

	return helpStyle.Render(help)
}

// renderCompleted shows the completion message
func (m FlowModel) renderCompleted() string {
	s := titleStyle.Render("✅ "+m.Title+" Complete") + "\n\n"
	s += "Configuration saved successfully!\n"
	return s
}

// GetResults returns the collected values from all steps
func (m FlowModel) GetResults() map[string]string {
	results := make(map[string]string)
	for i, step := range m.Steps {
		if stepState, exists := m.StepStates[i]; exists {
			// For SSH key steps, get the final file path
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

// GetSSHKeyInfo returns SSH key information for a step
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
