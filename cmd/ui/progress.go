package tui

import (
	"context"
	"fmt"
	"lightfold/pkg/config"
	"lightfold/pkg/deploy"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type progressModel struct {
	progress          float64
	maxProgress       float64
	currentStep       string
	stepHistory       []stepHistoryItem
	completed         bool
	countdownSecs     int
	width             int
	orchestrator      *deploy.Orchestrator
	ctx               context.Context
	err               error
	result            *deploy.DeploymentResult
	program           *tea.Program
	spinner           spinner.Model
	skipInit          bool
	completionMessage string
}

type stepHistoryItem struct {
	name        string
	description string
	status      string
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
	if m.skipInit {
		return m.spinner.Tick
	}
	if m.orchestrator != nil {
		return tea.Batch(
			m.startDeployment(),
			m.spinner.Tick,
		)
	}
	return tea.Batch(
		m.startProgress(),
		m.spinner.Tick,
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

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case stepUpdateMsg:
		if msg.step.Progress == -1 {
			fmt.Println(msg.step.Description)
			return m, nil
		}

		m.progress = float64(msg.step.Progress)
		m.currentStep = msg.step.Description

		if len(m.stepHistory) > 0 && m.stepHistory[len(m.stepHistory)-1].status == "in_progress" {
			m.stepHistory[len(m.stepHistory)-1].status = "success"
		}

		m.stepHistory = append(m.stepHistory, stepHistoryItem{
			name:        msg.step.Name,
			description: msg.step.Description,
			status:      "in_progress",
		})

		if msg.step.Progress >= 100 {
			if len(m.stepHistory) > 0 {
				m.stepHistory[len(m.stepHistory)-1].status = "success"
			}
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
			if len(m.stepHistory) > 0 && m.stepHistory[len(m.stepHistory)-1].status == "in_progress" {
				m.stepHistory[len(m.stepHistory)-1].status = "error"
			}
			return m, tea.Quit
		}

		m.result = msg.result
		m.completed = true
		m.progress = m.maxProgress
		if m.completionMessage != "" {
			m.currentStep = m.completionMessage
		} else {
			m.currentStep = "Deployment complete!"
		}
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

	s.WriteString("\n")

	if !m.completed {
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
			completionMsg := "Deployment complete!"
			if m.completionMessage != "" {
				completionMsg = m.completionMessage
			}
			s.WriteString(headerStyle.Render(completionMsg))
			s.WriteString("\n\n")
		}
	}

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
				icon = m.spinner.View()
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
			step = "Building app..."
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
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))

	m := progressModel{
		progress:    0,
		maxProgress: 100,
		width:       60,
		spinner:     s,
	}

	p := tea.NewProgram(m)
	_, err := p.Run()
	return err
}

func ShowDeploymentProgressWithOrchestrator(ctx context.Context, orchestrator *deploy.Orchestrator) error {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))

	m := progressModel{
		progress:     0,
		maxProgress:  100,
		width:        60,
		orchestrator: orchestrator,
		ctx:          ctx,
		spinner:      s,
	}

	p := tea.NewProgram(m)
	m.program = p

	orchestrator.SetProgressCallback(func(step deploy.DeploymentStep) {
		if p != nil {
			p.Send(stepUpdateMsg{step: step})
		}
	})

	finalModel, err := p.Run()
	if err != nil {
		return err
	}

	if final, ok := finalModel.(progressModel); ok && final.err != nil {
		return final.err
	}

	return nil
}

func ShowConfigurationProgressWithOrchestrator(ctx context.Context, orchestrator *deploy.Orchestrator, providerCfg config.ProviderConfig) error {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))

	m := progressModel{
		progress:          0,
		maxProgress:       100,
		width:             60,
		orchestrator:      orchestrator,
		ctx:               ctx,
		spinner:           s,
		skipInit:          true,
		completionMessage: "Configuration complete!",
	}

	p := tea.NewProgram(m)
	m.program = p

	orchestrator.SetProgressCallback(func(step deploy.DeploymentStep) {
		if p != nil {
			p.Send(stepUpdateMsg{step: step})
		}
	})

	go func() {
		result, err := orchestrator.ConfigureServer(ctx, providerCfg)
		p.Send(deployResultMsg{result: result, err: err})
	}()

	finalModel, err := p.Run()
	if err != nil {
		return err
	}

	if final, ok := finalModel.(progressModel); ok && final.err != nil {
		return final.err
	}

	return nil
}

func ShowProvisioningProgressWithOrchestrator(ctx context.Context, orchestrator *deploy.Orchestrator) (*deploy.DeploymentResult, error) {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))

	m := progressModel{
		progress:          0,
		maxProgress:       100,
		width:             60,
		orchestrator:      orchestrator,
		ctx:               ctx,
		spinner:           s,
		skipInit:          true,
		completionMessage: "Provisioning complete!",
	}

	p := tea.NewProgram(m)
	m.program = p

	orchestrator.SetProgressCallback(func(step deploy.DeploymentStep) {
		if p != nil {
			p.Send(stepUpdateMsg{step: step})
		}
	})

	go func() {
		result, err := orchestrator.Deploy(ctx)
		p.Send(deployResultMsg{result: result, err: err})
	}()

	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}

	if final, ok := finalModel.(progressModel); ok {
		if final.err != nil {
			return nil, final.err
		}
		return final.result, nil
	}

	return nil, fmt.Errorf("unexpected model type")
}
