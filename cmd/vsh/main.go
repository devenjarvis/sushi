package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type tickMsg struct{}
type errMsg error

type model struct {
	textInput textinput.Model
	err       error
}

func initialModel() model {
	ti := textinput.New()
	ti.Placeholder = "Cmd"
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 20
	ti.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	ti.CursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	ti.TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	return model{
		textInput: ti,
		err:       nil,
	}
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			execInput(m.textInput.Value())
			//m.textInput.SetCursorMode(textinput.CursorHide)
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

type editorFinishedMsg struct{ err error }

func execInput(input string) error {
	// Remove newline
	input = strings.TrimSuffix(input, "\n")

	args := strings.Split(input, " ")

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

	// Prepare command to execute
	cmd := exec.Command(args[0], args[1:]...)

	// Set correct output device
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	//Execute the command
	return cmd.Run()
}

func main() {
	// reader := bufio.NewReader(os.Stdin)
	for {
		p := tea.NewProgram(initialModel())
		_, err := p.StartReturningModel()
		if err != nil {
			fmt.Println("Oh no:", err)
			os.Exit(1)
		}
	}
}
