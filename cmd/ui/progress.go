package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type progressModel struct {
	progress      float64
	maxProgress   float64
	currentStep   string
	completed     bool
	countdownSecs int
	width         int
}

type progressMsg struct {
	progress float64
	step     string
}

type countdownMsg struct {
	seconds int
}

type completeMsg struct{}

func (m progressModel) Init() tea.Cmd {
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

	if !m.completed {
		// Progress header
		headerStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("86"))
		s.WriteString(headerStyle.Render("Deploying your application..."))
		s.WriteString("\n\n")

		// Current step
		stepStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("245"))
		s.WriteString(stepStyle.Render(m.currentStep))
		s.WriteString("\n\n")
	} else {
		// Completion header
		headerStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("82"))
		s.WriteString(headerStyle.Render("Deployment complete!"))
		s.WriteString("\n\n")
	}

	// Progress bar
	progressPercent := (m.progress / m.maxProgress) * 100
	if progressPercent > 100 {
		progressPercent = 100
	}

	// Build the progress bar with gradient colors
	barWidth := m.width - 10 // Leave space for percentage
	filledWidth := int((progressPercent / 100) * float64(barWidth))

	// Create gradient colors: purple -> blue -> cyan -> green
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

	// Add empty bars for unfilled portion
	emptyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	for i := filledWidth; i < barWidth; i++ {
		bar.WriteString(emptyStyle.Render("░"))
	}

	s.WriteString(bar.String())
	s.WriteString(fmt.Sprintf(" %.0f%%", progressPercent))
	s.WriteString("\n\n")

	if m.completed {
		// Countdown
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