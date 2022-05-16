package main

import (
	"fmt"
	"io/ioutil"
	"os"

	"slvck/internal/views"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const useHighPerformanceRenderer = false

type model struct {
	chat  views.ChatModel
	convo views.ConvoModel
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		m.chat.Init(),
		m.convo.Init(),
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	// Handle any top-level messages
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			// Send the quit command. But also notice that we're not returning
			// here because in this example case we also want the nested models to
			// do their updates too.
			cmds = append(cmds, tea.Quit)
		}
	}

	// Handle updates for nested model A
	m.chat, cmd = m.chat.Update(msg)
	cmds = append(cmds, cmd)

	// Handle updates for nested model B
	m.convo, cmd = m.convo.Update(msg)
	cmds = append(cmds, cmd)

	// Return updated model and any new commands
	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	// Render nested model views
	s := lipgloss.JoinHorizontal(0.2, m.convo.View(), m.chat.View())
	return s
}

type item struct {
	title, desc string
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.desc }
func (i item) FilterValue() string { return i.title }

func main() {
	content, err := ioutil.ReadFile("artichoke.md")
	if err != nil {
		fmt.Println("could not load file:", err)
		os.Exit(1)
	}
	chat := views.ChatModel{Content: string(content)}

	items := []list.Item{
		item{title: "Raspberry Pi’s", desc: "I have ’em all over my house"},
		item{title: "Nutella", desc: "It's good on toast"},
		item{title: "Bitter melon", desc: "It cools you down"},
		item{title: "Nice socks", desc: "And by that I mean socks without holes"},
		item{title: "Eight hours of sleep", desc: "I had this once"},
		item{title: "Cats", desc: "Usually"},
		item{title: "Plantasia, the album", desc: "My plants love it too"},
		item{title: "Pour over coffee", desc: "It takes forever to make though"},
		item{title: "VR", desc: "Virtual reality...what is there to say?"},
		item{title: "Noguchi Lamps", desc: "Such pleasing organic forms"},
		item{title: "Linux", desc: "Pretty much the best OS"},
		item{title: "Business school", desc: "Just kidding"},
		item{title: "Pottery", desc: "Wet clay is a great feeling"},
		item{title: "Shampoo", desc: "Nothing like clean hair"},
		item{title: "Table tennis", desc: "It’s surprisingly exhausting"},
		item{title: "Milk crates", desc: "Great for packing in your extra stuff"},
		item{title: "Afternoon tea", desc: "Especially the tea sandwich part"},
		item{title: "Stickers", desc: "The thicker the vinyl the better"},
		item{title: "20° Weather", desc: "Celsius, not Fahrenheit"},
		item{title: "Warm light", desc: "Like around 2700 Kelvin"},
		item{title: "The vernal equinox", desc: "The autumnal equinox is pretty good too"},
		item{title: "Gaffer’s tape", desc: "Basically sticky fabric"},
		item{title: "Terrycloth", desc: "In other words, towel fabric"},
	}

	convo := views.ConvoModel{List: list.New(items, list.NewDefaultDelegate(), 0, 0)}

	m := model{chat: chat, convo: convo}
	p := tea.NewProgram(m, tea.WithAltScreen())

	if err := p.Start(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}
