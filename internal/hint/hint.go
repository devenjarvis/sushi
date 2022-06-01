package hint

import (
	"fmt"
	"sort"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lithammer/fuzzysearch/fuzzy"
)

type switchMsg struct{}

type Model struct {
	focus       bool
	activated   bool
	accepted    bool
	cursor      int
	matchString string
	choices     []string
	hints       []string
	selected    string
}

func New(choices []string) Model {
	return Model{
		focus:     false,
		activated: false,
		accepted:  false,
		cursor:    0,
		choices:   choices,
		hints:     []string{},
		selected:  "",
	}
}

func (m *Model) GetCursor() int {
	return m.cursor
}

func (m *Model) GetChoice() string {
	if len(m.hints) > m.cursor {
		return m.hints[m.cursor]
	} else {
		return m.matchString
	}
}

func (m Model) Focused() bool {
	return m.focus
}

func (m *Model) Focus() {
	m.focus = true
	m.activated = true
}

func (m *Model) Blur() {
	m.focus = false
}

func (m *Model) Clear() {
	m.hints = []string{}
}

func (m *Model) AcceptHint() {
	m.accepted = true
}

func (m *Model) UpdateHintOptions(value string) {
	if value != m.matchString {
		m.matchString = value
		m.accepted = false
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
	if !m.focus {
		return m, nil
	}
	if m.activated {
		m.activated = false
		return m, nil
	}

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

	if m.focus {
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
