package hint

import (
	"fmt"
	"sort"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lithammer/fuzzysearch/fuzzy"
)

type Model struct {
	active      bool
	cursor      int
	matchString string
	choices     []string
	hints       []string
	selected    string
}

func New(choices []string) Model {
	return Model{
		active:   false,
		cursor:   0,
		choices:  choices,
		hints:    []string{},
		selected: "",
	}
}

func (m *Model) GetChoice() string {
	return m.hints[m.cursor]
}

func (m *Model) SetActive(active bool) {
	m.active = active
}

func (m *Model) UpdateHintOptions(value string) {
	if value != m.matchString {
		m.matchString = value
		m.hints = []string{}
		// Check for hint
		if len(value) > 0 {
			// Find matches
			matches := fuzzy.RankFind(string(value), m.choices)

			// Show hint if available
			if len(matches) > 0 {
				m.hints = []string{}
				sort.Sort(matches)
				for i, match := range matches {
					if i <= 4 && match.Distance <= (len(value)+1) {
						m.hints = append(m.hints, match.Target)
					} else {
						break
					}
				}
			}
		}

		m.cursor = 0
	}

}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyLeft:
			if m.cursor > 0 {
				m.cursor--
			}
		case tea.KeyRight:
			if m.cursor < len(m.hints)-1 {
				m.cursor++
			}
		case tea.KeyDown:
			m.active = true
		case tea.KeyUp:
			m.active = false
		case tea.KeyTab:
			m.selected = m.choices[m.cursor]
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m Model) View() string {
	var s string
	var selectedColor string

	if m.active {
		selectedColor = "205"
	} else {
		selectedColor = "237"
	}
	selectedText := lipgloss.NewStyle().Background(lipgloss.Color(selectedColor)).Inline(true).Render
	unSelectedText := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Inline(true).Render

	for i, hint := range m.hints {
		cursor := " "
		if m.cursor == i {
			cursor = " "
			s += selectedText(fmt.Sprintf("%s%s ", cursor, hint))
		} else {
			s += unSelectedText(fmt.Sprintf("%s%s ", cursor, hint))
		}
	}

	return s
}
