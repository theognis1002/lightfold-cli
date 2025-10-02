package tui

import (
	"context"
	"fmt"
	"lightfold/pkg/deploy"
	"strings"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type progressModel struct {
	progress      float64
	maxProgress   float64
	currentStep   string
	stepHistory   []stepHistoryItem
	completed     bool
	countdownSecs int
	width         int
	orchestrator  *deploy.Orchestrator
	ctx           context.Context
	err           error
	program       *tea.Program
}

type stepHistoryItem struct {
	name        string
	description string
	status      string // "in_progress", "success", "error"
}

type progressMsg struct {
	progress float64
	step     string
}

type stepUpdateMsg struct {
	step deploy.DeploymentStep
}

type countdownMsg struct {
	seconds int
}

type completeMsg struct{}

type deployResultMsg struct {
	result *deploy.DeploymentResult
	err    error
}

func (m progressModel) Init() tea.Cmd {
	if m.orchestrator != nil {
		// Use real deployment with orchestrator
		return m.startDeployment()
	}
	// Fallback to mock progress
	return tea.Batch(
		m.startProgress(),
		tea.Tick(time.Second, func(t time.Time) tea.Msg {
			return countdownMsg{seconds: m.countdownSecs - 1}
		}),
	)
}

func (m progressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}

	case stepUpdateMsg:
		// Update progress
		m.progress = float64(msg.step.Progress)
		m.currentStep = msg.step.Description

		// Mark previous step as successful if we're moving forward
		if len(m.stepHistory) > 0 && m.stepHistory[len(m.stepHistory)-1].status == "in_progress" {
			m.stepHistory[len(m.stepHistory)-1].status = "success"
		}

		// Add new step to history
		m.stepHistory = append(m.stepHistory, stepHistoryItem{
			name:        msg.step.Name,
			description: msg.step.Description,
			status:      "in_progress",
		})

		// Check if deployment is complete
		if msg.step.Progress >= 100 {
			// Mark last step as successful
			if len(m.stepHistory) > 0 {
				m.stepHistory[len(m.stepHistory)-1].status = "success"
			}
			// Don't set completed yet - wait for deployResultMsg
		}

		return m, nil

	case progressMsg:
		m.progress = msg.progress
		m.currentStep = msg.step

		if m.progress >= m.maxProgress {
			m.completed = true
			m.countdownSecs = 3
			return m, tea.Tick(time.Second, func(t time.Time) tea.Msg {
				return countdownMsg{seconds: m.countdownSecs - 1}
			})
		}

		return m, m.continueProgress()

	case deployResultMsg:
		if msg.err != nil {
			m.err = msg.err
			m.completed = true
			// Mark last step as failed
			if len(m.stepHistory) > 0 && m.stepHistory[len(m.stepHistory)-1].status == "in_progress" {
				m.stepHistory[len(m.stepHistory)-1].status = "error"
			}
			return m, tea.Quit
		}

		m.completed = true
		m.progress = m.maxProgress
		m.currentStep = "Deployment complete!"
		// Mark last step as successful
		if len(m.stepHistory) > 0 && m.stepHistory[len(m.stepHistory)-1].status == "in_progress" {
			m.stepHistory[len(m.stepHistory)-1].status = "success"
		}
		m.countdownSecs = 3
		return m, tea.Tick(time.Second, func(t time.Time) tea.Msg {
			return countdownMsg{seconds: m.countdownSecs - 1}
		})

	case countdownMsg:
		m.countdownSecs = msg.seconds
		if m.countdownSecs <= 0 {
			return m, tea.Quit
		}
		return m, tea.Tick(time.Second, func(t time.Time) tea.Msg {
			return countdownMsg{seconds: m.countdownSecs - 1}
		})

	case completeMsg:
		return m, tea.Quit
	}

	return m, nil
}

func (m progressModel) View() string {
	var s strings.Builder

	// Header
	if !m.completed {
		headerStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("86"))
		s.WriteString(headerStyle.Render("Deploying your application..."))
		s.WriteString("\n\n")
	} else {
		var headerStyle lipgloss.Style
		if m.err != nil {
			headerStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("196"))
			s.WriteString(headerStyle.Render("Deployment failed!"))
			s.WriteString("\n\n")
			errorStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("203"))
			s.WriteString(errorStyle.Render(fmt.Sprintf("Error: %v", m.err)))
			s.WriteString("\n\n")
		} else {
			headerStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("82"))
			s.WriteString(headerStyle.Render("Deployment complete!"))
			s.WriteString("\n\n")
		}
	}

	// Step history
	if len(m.stepHistory) > 0 {
		successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
		inProgressStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("226"))
		errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
		descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))

		for _, step := range m.stepHistory {
			var icon string
			var style lipgloss.Style
			switch step.status {
			case "success":
				icon = "✓"
				style = successStyle
			case "in_progress":
				icon = "⋯"
				style = inProgressStyle
			case "error":
				icon = "✗"
				style = errorStyle
			}
			s.WriteString(style.Render(fmt.Sprintf("%s ", icon)))
			s.WriteString(descStyle.Render(step.description))
			s.WriteString("\n")
		}
		s.WriteString("\n")
	}

	// Progress bar
	progressPercent := (m.progress / m.maxProgress) * 100
	if progressPercent > 100 {
		progressPercent = 100
	}

	barWidth := m.width - 10
	filledWidth := int((progressPercent / 100) * float64(barWidth))

	colors := []string{"129", "63", "39", "33", "45", "51", "50", "49", "48", "47", "46", "82"}

	var bar strings.Builder
	for i := 0; i < filledWidth; i++ {
		colorIndex := int(float64(i) / float64(barWidth) * float64(len(colors)))
		if colorIndex >= len(colors) {
			colorIndex = len(colors) - 1
		}

		colorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors[colorIndex]))
		bar.WriteString(colorStyle.Render("█"))
	}

	emptyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	for i := filledWidth; i < barWidth; i++ {
		bar.WriteString(emptyStyle.Render("░"))
	}

	s.WriteString(bar.String())
	s.WriteString(fmt.Sprintf(" %.0f%%", progressPercent))
	s.WriteString("\n\n")

	// Countdown or current step
	if m.completed && m.err == nil {
		countdownStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("245"))
		s.WriteString(countdownStyle.Render(fmt.Sprintf("Exiting in %d seconds...", m.countdownSecs)))
	}

	return s.String()
}

func (m progressModel) startProgress() tea.Cmd {
	return func() tea.Msg {
		return progressMsg{
			progress: 10,
			step:     "Connecting to server...",
		}
	}
}

func (m progressModel) continueProgress() tea.Cmd {
	return tea.Tick(time.Millisecond*500, func(t time.Time) tea.Msg {
		newProgress := m.progress + 15

		var step string
		switch {
		case newProgress <= 25:
			step = "Building application..."
		case newProgress <= 50:
			step = "Uploading files..."
		case newProgress <= 75:
			step = "Installing dependencies..."
		case newProgress <= 90:
			step = "Starting services..."
		default:
			step = "Finalizing deployment..."
		}

		return progressMsg{
			progress: newProgress,
			step:     step,
		}
	})
}

func (m progressModel) startDeployment() tea.Cmd {
	return func() tea.Msg {
		result, err := m.orchestrator.Deploy(m.ctx)
		return deployResultMsg{result: result, err: err}
	}
}

func ShowDeploymentProgress() error {
	m := progressModel{
		progress:    0,
		maxProgress: 100,
		width:       60,
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func ShowDeploymentProgressWithOrchestrator(ctx context.Context, orchestrator *deploy.Orchestrator) error {
	m := progressModel{
		progress:     0,
		maxProgress:  100,
		width:        60,
		orchestrator: orchestrator,
		ctx:          ctx,
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	m.program = p

	// Set progress callback on orchestrator to send updates to the UI
	orchestrator.SetProgressCallback(func(step deploy.DeploymentStep) {
		if p != nil {
			p.Send(stepUpdateMsg{step: step})
		}
	})

	finalModel, err := p.Run()
	if err != nil {
		return err
	}

	// Check for deployment errors
	if final, ok := finalModel.(progressModel); ok && final.err != nil {
		return final.err
	}

	return nil
}