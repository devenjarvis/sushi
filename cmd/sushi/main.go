package main

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"

	"sushi/internal/hint"
	"sushi/internal/prompt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/sys/unix"
)

type tickMsg struct{}
type errMsg error
type switchMsg struct{}

const (
	textInput = iota
	hintInput = iota
)

type model struct {
	textInput   prompt.Model
	hintInput   hint.Model
	activeInput int
	homeDir     string
	err         error
	cmd         []string
	cmdHistory  []string
	historyPos  int
	width       int
	height      int
}

func initialModel(homeDir string, cmdHistory []string, commands []string) model {
	ti := prompt.New()
	ti.Placeholder = "Cmd"
	ti.Focus(false)
	ti.CharLimit = 156
	ti.Width = 0
	ti.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#3185FC")).Faint(true)
	ti.CursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#3185FC"))
	ti.TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#F0F7F4"))

	return model{
		textInput:  ti,
		hintInput:  hint.New(commands),
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
			m.hintInput.Clear()
			m.textInput.Blur()
			return m, tea.Quit
		case tea.KeyUp:
			if m.hintInput.Focused() {
				m.hintInput.Blur()
				m.textInput.Focus(true)
			} else {
				if len(m.textInput.Value()) == 0 && len(m.cmdHistory) > 0 {
					m.historyPos = 1
					m.textInput.SetValue(m.cmdHistory[len(m.cmdHistory)-m.historyPos])
				} else if m.historyPos > 0 && m.historyPos < len(m.cmdHistory) {
					m.historyPos++
					m.textInput.SetValue(m.cmdHistory[len(m.cmdHistory)-m.historyPos])
				}
			}
		case tea.KeyRight:
			if !m.hintInput.Focused() {
				if m.textInput.Cursor() == len(m.textInput.Value()) {
					m.textInput.Blur()
					m.hintInput.Focus()
				}
			}
		case tea.KeyLeft:
			if m.hintInput.Focused() {
				if m.hintInput.GetCursor() == 0 {
					m.hintInput.Blur()
					m.textInput.Focus(true)
				}
			}

		case tea.KeyDown:
			if m.historyPos > 1 {
				m.historyPos--
				m.textInput.SetValue(m.cmdHistory[len(m.cmdHistory)-m.historyPos])
			} else if m.historyPos == 1 {
				m.textInput.SetValue("")
			} else if m.historyPos == 0 {
				if !m.hintInput.Focused() {
					m.hintInput.Focus()
					m.textInput.Blur()
				}
			}
		case tea.KeyBackspace:
			if m.hintInput.Focused() {
				m.hintInput.Blur()
				m.textInput.Focus(false)
			}
		case tea.KeyTab: // Accept hint
			m.hintInput.Blur()
			m.textInput.SetValue(m.hintInput.GetChoice())
			m.textInput.SetCursor(len(m.textInput.Value()))
			m.textInput.Focus(true)
		case tea.KeyCtrlC:
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	// We handle errors just like any other message
	case errMsg:
		m.err = msg
		return m, nil
	}

	m.textInput, cmd = m.textInput.Update(msg)
	m.hintInput.UpdateHintOptions(m.textInput.Value())
	m.hintInput, cmd = m.hintInput.Update(msg)
	return m, cmd
}

func (m model) View() string {
	promptStyle := lipgloss.NewStyle().Width(m.width - 2).MarginTop(1).BorderStyle(lipgloss.ThickBorder()).BorderLeft(true).BorderForeground(lipgloss.Color("63")).Render

	s := lipgloss.JoinVertical(lipgloss.Left, promptStyle(m.textInput.View()), m.hintInput.View())
	// return fmt.Sprintf(
	// 	m.textInput.View(),
	// ) + "\n"
	return s
}

func execCmd(args []string) error {
	if len(args) > 0 {
		// Check for background process
		backgroundProcess := false

		// Check for background process
		if args[len(args)-1] == "&" {
			backgroundProcess = true
			args = args[:len(args)-1]
		}
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

			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr

			//Execute the command
			if backgroundProcess {
				return cmd.Start()
			} else {
				cmd.Stdin = os.Stdin
				return cmd.Run()
			}
		}
	} else {
		return nil
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

func initialize(homeDir string) ([]string, []string, error) {
	// Create history file
	sushiHistoryPath := fmt.Sprintf("%s/.sushi_history", homeDir)

	cmdHistory, err := initHistory(sushiHistoryPath)
	if err != nil {
		return nil, nil, err
	}

	// Load PATH
	full_path := os.Getenv("PATH")
	split_path := filepath.SplitList(full_path)

	// Initialize with internal list of commands
	commandMap := make(map[string]bool)
	commands := []string{"cd", "exit"}

	for _, path := range split_path {
		filepath.WalkDir(path, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			info, err := d.Info()
			if err == nil {
				// Check if executable
				if info.Mode().Perm()&0111 != 0 {
					// Check if user has access (Unix)
					access_err := unix.Access(path, 0x1)
					if access_err != nil {
						return nil
					} else {
						if _, value := commandMap[d.Name()]; !value {
							commandMap[d.Name()] = true
							commands = append(commands, d.Name())
						}
					}
				}
			}

			return nil
		})
	}

	// Create config file
	sushiConfigPath := fmt.Sprintf("%s/.sushi_config", homeDir)

	if _, err = os.Stat(sushiConfigPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// sushi config file does not exist
			_, create_err := os.Create(sushiConfigPath)
			if create_err != nil {
				return cmdHistory, commands, create_err
			}
		} else {
			return cmdHistory, commands, err
		}
	} else {
		// run config
	}

	return cmdHistory, commands, nil
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

	// Make sure background commands have a space
	if input[len(input)-1] == '&' && input[len(input)-2] != ' ' {
		input = fmt.Sprintf("%s%s", input[:len(input)-1], " &")
	}

	// Split on space to build command
	cmd := strings.Split(input, " ")

	return cmd
}

func main() {
	usr, _ := user.Current()
	homeDir := usr.HomeDir

	cmdHistory, commands, init_err := initialize(homeDir)
	if init_err != nil {
		fmt.Println("Initialization Error:", init_err)
	} else {
		for {
			p := tea.NewProgram(initialModel(homeDir, cmdHistory, commands))
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
