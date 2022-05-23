package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strings"

	"sushi/internal/prompt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type tickMsg struct{}
type errMsg error

type model struct {
	textInput  prompt.Model
	homeDir    string
	err        error
	cmd        []string
	cmdHistory []string
	historyPos int
}

func initialModel(homeDir string, cmdHistory []string) model {
	ti := prompt.New()
	ti.Placeholder = "Cmd"
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 0
	ti.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#3185FC"))
	ti.CursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#3185FC"))
	ti.TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#F0F7F4"))

	return model{
		textInput:  ti,
		homeDir:    homeDir,
		cmdHistory: cmdHistory,
		err:        nil,
		historyPos: 0,
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
			m.cmd = parseInput(m.homeDir, input)
			return m, tea.Quit
		case tea.KeyUp:
			if len(m.textInput.Value()) == 0 && len(m.cmdHistory) > 0 {
				m.historyPos = 1
				m.textInput.SetValue(m.cmdHistory[len(m.cmdHistory)-m.historyPos])
			} else if m.historyPos > 0 && m.historyPos < len(m.cmdHistory) {
				m.historyPos++
				m.textInput.SetValue(m.cmdHistory[len(m.cmdHistory)-m.historyPos])
			}
		case tea.KeyDown:
			if m.historyPos > 1 {
				m.historyPos--
				m.textInput.SetValue(m.cmdHistory[len(m.cmdHistory)-m.historyPos])
			} else if m.historyPos == 1 {
				m.textInput.SetValue("")
			}
		case tea.KeyCtrlC:
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

func prependString(array []string, val string) []string {
	array = append(array, "")
	copy(array[1:], array)
	array[0] = val
	return array
}

func initHistory(sushiHistoryPath string) ([]string, error) {

	if _, err := os.Stat(sushiHistoryPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// sushi history file does not exist
			_, create_err := os.Create(sushiHistoryPath)
			if create_err != nil {
				return nil, create_err
			}
		} else {
			return nil, err
		}

	} else {
		var cmdHistory []string
		// load history
		file, err := os.Open(sushiHistoryPath)
		if err != nil {
			return nil, err
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)

		for scanner.Scan() {
			cmdHistory = append(cmdHistory, scanner.Text())
		}

		if err := scanner.Err(); err != nil {
			return nil, err
		} else {
			return cmdHistory, err
		}
	}

	return nil, nil
}

func initialize(homeDir string) ([]string, error) {
	// Create history file
	sushiHistoryPath := fmt.Sprintf("%s/.sushi_history", homeDir)

	cmdHistory, err := initHistory(sushiHistoryPath)
	if err != nil {
		return nil, err
	}

	// Create config file
	sushiConfigPath := fmt.Sprintf("%s/.sushi_config", homeDir)

	if _, err = os.Stat(sushiConfigPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// sushi config file does not exist
			_, create_err := os.Create(sushiConfigPath)
			if create_err != nil {
				return cmdHistory, create_err
			}
		} else {
			return cmdHistory, err
		}
	} else {
		// run config
	}

	return cmdHistory, nil
}

func appendHistory(home_dir string, command string, cmdHistory []string) []string {
	if len(command) > 0 {
		if len(cmdHistory) == 0 || cmdHistory[len(cmdHistory)-1] != command {
			// Write to in-memory command history
			cmdHistory = append(cmdHistory, command)

			// Write to disk
			sushiHistoryPath := fmt.Sprintf("%s/.sushi_history", home_dir)
			f, err := os.OpenFile(sushiHistoryPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
			if err != nil {
				panic(err)
			}

			defer f.Close()

			if _, err = f.WriteString(fmt.Sprintln(command)); err != nil {
				panic(err)
			}
		}
	}

	return cmdHistory
}

func parseInput(homeDir string, input string) []string {
	// Sustitute home directory
	input = strings.ReplaceAll(input, "~", homeDir)

	// Split on space to build command
	cmd := strings.Split(input, " ")

	return cmd
}

func main() {
	usr, _ := user.Current()
	homeDir := usr.HomeDir

	cmdHistory, init_err := initialize(homeDir)
	if init_err != nil {
		fmt.Println("Initialization Error:", init_err)
	} else {
		for {
			p := tea.NewProgram(initialModel(homeDir, cmdHistory))
			m, err := p.StartReturningModel()
			if err != nil {
				fmt.Println("Oh no:", err)
				os.Exit(1)
			}

			if m, ok := m.(model); ok && len(m.cmd) > 0 {
				if err := execCmd(m.cmd); err != nil {
					fmt.Println(err)
				} else {
					cmdHistory = appendHistory(m.homeDir, m.textInput.Value(), cmdHistory)
				}
			}
		}
	}
}
