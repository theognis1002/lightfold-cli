// Package multiInput provides functions that
// help define and draw a multi-input step
package multiInput

import (
	"fmt"
	"lightfold/cmd/steps"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// go-blueprint inspired color scheme
var (
	focusedStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("#01FAC6")).Bold(true)
	titleStyle            = lipgloss.NewStyle().Background(lipgloss.Color("#01FAC6")).Foreground(lipgloss.Color("#030303")).Bold(true).Padding(0, 1, 0)
	selectedItemStyle     = lipgloss.NewStyle().PaddingLeft(1).Foreground(lipgloss.Color("170")).Bold(true)
	selectedItemDescStyle = lipgloss.NewStyle().PaddingLeft(1).Foreground(lipgloss.Color("170"))
	descriptionStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#40BDA3"))
)

// A Selection represents a choice made in a multiInput step
type Selection struct {
	Choice string
}

// Update changes the value of a Selection's Choice
func (s *Selection) Update(value string) {
	s.Choice = value
}

// A multiInput.model contains the data for the multiInput step.
//
// It has the required methods that make it a bubbletea.Model
type model struct {
	cursor      int
	choices     []steps.Item
	selected    map[int]struct{}
	choice      *Selection
	header      string
	exit        *bool
	multiSelect bool // Whether this is a multi-select or single-select menu
}

func (m model) Init() tea.Cmd {
	return nil
}

// InitialModelMulti initializes a multiInput step with
// the given data
func InitialModelMulti(choices []steps.Item, selection *Selection, header string, exitPtr *bool) model {
	return model{
		choices:     choices,
		selected:    make(map[int]struct{}),
		choice:      selection,
		header:      titleStyle.Render(header),
		exit:        exitPtr,
		multiSelect: false, // Default to single-select
	}
}

// InitialModelMultiSelect initializes a multi-select step
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

// Update is called when "things happen", it checks for
// important keystrokes to signal when to quit, change selection,
// and confirm the selection.
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
				// Multi-select mode: Enter confirms if something is selected
				if len(m.selected) > 0 {
					// Confirm selection (use first selected item for now)
					for selectedKey := range m.selected {
						m.choice.Update(m.choices[selectedKey].Title)
						m.cursor = selectedKey
						break
					}
					return m, tea.Quit
				} else {
					// Nothing selected, select current item
					m.selected[m.cursor] = struct{}{}
				}
			} else {
				// Single-select mode: Enter immediately selects and confirms
				m.choice.Update(m.choices[m.cursor].Title)
				return m, tea.Quit
			}
		case " ":
			// Space toggles selection in both modes
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
			// Y confirms selection in both modes
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

// View is called to draw the multiInput step
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
		if _, ok := m.selected[i]; ok {
			checked = focusedStyle.Render("X")
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

// ShowMenu displays a single-select menu with the given choices and returns the selected option.
// Users can press Enter to directly select and confirm their choice.
// Use ShowMultiSelectMenu for multi-select scenarios.
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

// ShowMultiSelectMenu displays a multi-select menu with the given choices.
// Users must press Space to select items and Enter/y to confirm their choices.
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