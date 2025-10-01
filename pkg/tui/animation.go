package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type animationModel struct {
	frame     int
	completed bool
}

type tickMsg struct{}

// ASCII rocket frames for animation
var rocketFrames = []string{
	`
     ^
    /|\
   / | \
  /  |  \
 |   |   |
 |  /_\  |
 |_/   \_|
   |___|
    |||
   /   \
  /_____\
`,
	`
      ^
     /|\
    / | \
   /  |  \
  |   |   |
  |  /_\  |
  |_/   \_|
    |___|
     |||
    /   \
   /_____\
`,
	`
       ^
      /|\
     / | \
    /  |  \
   |   |   |
   |  /_\  |
   |_/   \_|
     |___|
      |||
     /   \
    /_____\
`,
	`
        ^
       /|\
      / | \
     /  |  \
    |   |   |
    |  /_\  |
    |_/   \_|
      |___|
       |||
      /   \
     /_____\
`,
	`
         ^
        /|\
       / | \
      /  |  \
     |   |   |
     |  /_\  |
     |_/   \_|
       |___|
        |||
       /   \
      /_____\
`,
}

// Success messages to cycle through
var successMessages = []string{
	"🚀 Blast off! Your app is live!",
	"✨ Deployment successful!",
	"🎉 Your application is now running!",
	"🌟 Mission accomplished!",
}

func (m animationModel) Init() tea.Cmd {
	return tea.Tick(time.Millisecond*200, func(t time.Time) tea.Msg {
		return tickMsg{}
	})
}

func (m animationModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "enter", " ":
			return m, tea.Quit
		}

	case tickMsg:
		m.frame++
		if m.frame >= len(rocketFrames)*3 { // Show each frame 3 times
			m.completed = true
			return m, tea.Quit
		}
		return m, tea.Tick(time.Millisecond*200, func(t time.Time) tea.Msg {
			return tickMsg{}
		})
	}

	return m, nil
}

func (m animationModel) View() string {
	var s strings.Builder

	// Success message
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("82")).
		Align(lipgloss.Center).
		Width(60)

	msgIndex := (m.frame / 3) % len(successMessages)
	s.WriteString(headerStyle.Render(successMessages[msgIndex]))
	s.WriteString("\n")

	// Rocket animation
	rocketStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("226")). // Yellow
		Align(lipgloss.Center).
		Width(60)

	frameIndex := (m.frame / 3) % len(rocketFrames)
	rocket := rocketFrames[frameIndex]

	// Add some sparkles around the rocket
	sparkles := getSparkles(m.frame)
	rocketWithSparkles := addSparkles(rocket, sparkles)

	s.WriteString(rocketStyle.Render(rocketWithSparkles))

	// Footer message
	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245")).
		Align(lipgloss.Center).
		Width(60).
		MarginTop(2)

	s.WriteString("\n")
	s.WriteString(footerStyle.Render("Press any key to continue..."))

	return s.String()
}

func getSparkles(frame int) []string {
	sparklePatterns := [][]string{
		{"✨", "⭐", "🌟", "💫"},
		{"⭐", "🌟", "💫", "✨"},
		{"🌟", "💫", "✨", "⭐"},
		{"💫", "✨", "⭐", "🌟"},
	}

	patternIndex := (frame / 2) % len(sparklePatterns)
	return sparklePatterns[patternIndex]
}

func addSparkles(rocket string, sparkles []string) string {
	lines := strings.Split(rocket, "\n")

	// Add sparkles to random positions around the rocket
	if len(lines) > 3 {
		// Add sparkles around the rocket body
		sparklePositions := []int{1, 3, 5, 7}

		for i, pos := range sparklePositions {
			if pos < len(lines) && i < len(sparkles) {
				// Add sparkle to the left and right of the rocket
				lines[pos] = sparkles[i] + " " + lines[pos] + " " + sparkles[i]
			}
		}
	}

	return strings.Join(lines, "\n")
}

func ShowRocketAnimation() error {
	m := animationModel{}

	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// ShowQuickSuccess shows a simple success message without animation for non-terminal environments
func ShowQuickSuccess() {
	successStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("82"))

	fmt.Println()
	fmt.Println(successStyle.Render("🚀 Deployment successful!"))
	fmt.Println("Your application is now live!")
	fmt.Println()
}