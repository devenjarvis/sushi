package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"sushi/internal/prompt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type tickMsg struct{}
type errMsg error

type model struct {
	textInput prompt.Model
	err       error
	cmd       []string
}

func initialModel() model {
	ti := prompt.New()
	ti.Placeholder = "Cmd"
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 0
	ti.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#3185FC"))
	ti.CursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#3185FC"))
	ti.TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#F0F7F4"))

	return model{
		textInput: ti,
		err:       nil,
	}
}

func (m model) Init() tea.Cmd {
	return prompt.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			input := strings.TrimSuffix(m.textInput.Value(), "\n")
			m.cmd = strings.Split(input, " ")
			return m, tea.Quit
		}

	// We handle errors just like any other message
	case errMsg:
		m.err = msg
		return m, nil
	}

	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m model) View() string {
	return fmt.Sprintf(
		m.textInput.View(),
	) + "\n"
}

func execCmd(args []string) error {
	// Check for built-in commands
	switch args[0] {
	case "cd":
		if len(args) < 2 {
			return errors.New("path required")
		}

		return os.Chdir(args[1])
	case "exit":
		os.Exit(0)
	}

	// Make sure command exists
	_, err := exec.LookPath(args[0])
	if err != nil {
		errMsg := fmt.Sprintf("didn't find '%s'", args[0])
		return errors.New(errMsg)
	} else {
		// Prepare command to execute
		cmd := exec.Command(args[0], args[1:]...)

		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		//Execute the command
		return cmd.Run()
	}
}

func main() {
	for {
		p := tea.NewProgram(initialModel())
		m, err := p.StartReturningModel()
		if err != nil {
			fmt.Println("Oh no:", err)
			os.Exit(1)
		}

		if m, ok := m.(model); ok && len(m.cmd) > 0 {
			if err := execCmd(m.cmd); err != nil {
				fmt.Println(err)
			}
		}
	}
}
