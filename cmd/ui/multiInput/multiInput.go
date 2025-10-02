package multiInput

import (
	"fmt"
	"lightfold/cmd/steps"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	focusedStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("#01FAC6")).Bold(true)
	titleStyle            = lipgloss.NewStyle().Background(lipgloss.Color("#01FAC6")).Foreground(lipgloss.Color("#030303")).Bold(true).Padding(0, 1, 0)
	selectedItemStyle     = lipgloss.NewStyle().PaddingLeft(1).Foreground(lipgloss.Color("170")).Bold(true)
	selectedItemDescStyle = lipgloss.NewStyle().PaddingLeft(1).Foreground(lipgloss.Color("170"))
	descriptionStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#40BDA3"))
)

type Selection struct {
	Choice string
}

func (s *Selection) Update(value string) {
	s.Choice = value
}

type model struct {
	cursor      int
	choices     []steps.Item
	selected    map[int]struct{}
	choice      *Selection
	header      string
	exit        *bool
	multiSelect bool
}

func (m model) Init() tea.Cmd {
	return nil
}

func InitialModelMulti(choices []steps.Item, selection *Selection, header string, exitPtr *bool) model {
	return model{
		choices:     choices,
		selected:    make(map[int]struct{}),
		choice:      selection,
		header:      titleStyle.Render(header),
		exit:        exitPtr,
		multiSelect: false,
	}
}

func InitialModelMultiSelect(choices []steps.Item, selection *Selection, header string, exitPtr *bool) model {
	return model{
		choices:     choices,
		selected:    make(map[int]struct{}),
		choice:      selection,
		header:      titleStyle.Render(header),
		exit:        exitPtr,
		multiSelect: true,
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			if m.exit != nil {
				*m.exit = true
			}
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.choices)-1 {
				m.cursor++
			}
		case "enter":
			if m.multiSelect {
				if len(m.selected) > 0 {
					for selectedKey := range m.selected {
						m.choice.Update(m.choices[selectedKey].Title)
						m.cursor = selectedKey
						break
					}
					return m, tea.Quit
				} else {
					m.selected[m.cursor] = struct{}{}
				}
			} else {
				m.choice.Update(m.choices[m.cursor].Title)
				return m, tea.Quit
			}
		case " ":
			if len(m.selected) == 1 && !m.multiSelect {
				m.selected = make(map[int]struct{})
			}
			_, ok := m.selected[m.cursor]
			if ok {
				delete(m.selected, m.cursor)
			} else {
				m.selected[m.cursor] = struct{}{}
			}
		case "y":
			if len(m.selected) == 1 {
				for selectedKey := range m.selected {
					m.choice.Update(m.choices[selectedKey].Title)
					m.cursor = selectedKey
				}
				return m, tea.Quit
			}
		}
	}
	return m, nil
}

func (m model) View() string {
	s := m.header + "\n\n"

	for i, choice := range m.choices {
		cursor := " "
		if m.cursor == i {
			cursor = focusedStyle.Render(">")
			choice.Title = selectedItemStyle.Render(choice.Title)
			choice.Desc = selectedItemDescStyle.Render(choice.Desc)
		}

		checked := " "
		if m.multiSelect {
			if _, ok := m.selected[i]; ok {
				checked = focusedStyle.Render("X")
			}
		} else {
			if m.cursor == i {
				checked = focusedStyle.Render("X")
			}
		}

		title := focusedStyle.Render(choice.Title)
		description := descriptionStyle.Render(choice.Desc)

		s += fmt.Sprintf("%s [%s] %s\n%s\n\n", cursor, checked, title, description)
	}

	if m.multiSelect {
		s += fmt.Sprintf("Press %s to select, %s to confirm choice, %s to exit.\n\n",
			focusedStyle.Render("space"), focusedStyle.Render("enter/y"), focusedStyle.Render("esc/q"))
	} else {
		s += fmt.Sprintf("Press %s to confirm choice, %s to exit.\n\n",
			focusedStyle.Render("enter"), focusedStyle.Render("esc/q"))
	}
	return s
}

func ShowMenu(choices []steps.Item, header string) (string, error) {
	selection := &Selection{}
	exit := false

	m := InitialModelMulti(choices, selection, header, &exit)

	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return "", fmt.Errorf("error running menu: %w", err)
	}

	final := finalModel.(model)
	if exit && final.choice.Choice == "" {
		return "", fmt.Errorf("selection cancelled")
	}

	return final.choice.Choice, nil
}

func ShowMultiSelectMenu(choices []steps.Item, header string) (string, error) {
	selection := &Selection{}
	exit := false

	m := InitialModelMultiSelect(choices, selection, header, &exit)

	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return "", fmt.Errorf("error running multi-select menu: %w", err)
	}

	final := finalModel.(model)
	if exit && final.choice.Choice == "" {
		return "", fmt.Errorf("selection cancelled")
	}

	return final.choice.Choice, nil
}