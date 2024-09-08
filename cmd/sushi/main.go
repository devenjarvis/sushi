package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/devenjarvis/sushi/internal/hint"
	"github.com/devenjarvis/sushi/internal/prompt"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/sys/unix"
)

var exitError = errors.New("exit")

type errMsg error

type command struct {
	textInput prompt.Model
	hintInput hint.Model
	stdout    string
	stderr    string
}

func NewCommand(commands []string) command {
	ti := prompt.New()
	ti.Placeholder = "Cmd"
	ti.Focus(false)
	ti.Width = 0
	ti.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#3185FC")).Faint(true)
	ti.CursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#3185FC"))
	ti.TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#F0F7F4"))

	return command{
		textInput: ti,
		hintInput: hint.New(commands),
	}
}

type model struct {
	commands    []command
	commandList []string
	currentCmd  int
	viewport    viewport.Model
	ready       bool
	homeDir     string
	err         error
	cmd         []string
	cmdHistory  []string
	historyPos  int
	width       int
	height      int
	cursor      int
	toBottom    bool
}

func initialModel(homeDir string, cmdHistory []string, commands []string) model {

	return model{
		ready:       false,
		toBottom:    false,
		commandList: commands,
		commands:    []command{NewCommand(commands)},
		currentCmd:  0,
		homeDir:     homeDir,
		cmdHistory:  cmdHistory,
		err:         nil,
		historyPos:  0,
	}
}

func (m model) Init() tea.Cmd {
	return prompt.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyRunes:
			m.toBottom = true
		case tea.KeyEnter:
			input := strings.TrimSuffix(m.commands[m.currentCmd].textInput.Value(), "\n")
			m.cmd = parseInput(m.homeDir, input)
			m.commands[m.currentCmd].textInput.Blur()
			m.commands[m.currentCmd].hintInput.Clear()
			m.commands[m.currentCmd].hintInput.Blur()
			if stdout, stderr, err := execCmd(m.cmd); err != nil {
				if err == exitError {
					return m, tea.Quit
				} else {
					m.commands[m.currentCmd].stderr = err.Error()
				}

			} else {
				m.commands[m.currentCmd].stdout = stdout.String()
				m.commands[m.currentCmd].stderr = stderr.String()
			}
			// Store command in history
			m.cmdHistory = appendHistory(m.homeDir, input, m.cmdHistory)
			// Add a new command
			m.commands = append(m.commands, NewCommand(m.commandList))
			m.currentCmd += 1
			m.toBottom = true

		case tea.KeyUp:
			if m.commands[m.currentCmd].hintInput.Focused() {
				m.commands[m.currentCmd].hintInput.Blur()
				m.commands[m.currentCmd].textInput.Focus(true)
			} else {
				if len(m.commands[m.currentCmd].textInput.Value()) == 0 && len(m.cmdHistory) > 0 {
					m.historyPos = 1
					m.commands[m.currentCmd].textInput.SetValue(m.cmdHistory[len(m.cmdHistory)-m.historyPos])
				} else if m.historyPos > 0 && m.historyPos < len(m.cmdHistory) {
					m.historyPos++
					m.commands[m.currentCmd].textInput.SetValue(m.cmdHistory[len(m.cmdHistory)-m.historyPos])
				}
			}
		case tea.KeyRight:
			if !m.commands[m.currentCmd].hintInput.Focused() {
				if m.commands[m.currentCmd].textInput.Cursor() == len(m.commands[m.currentCmd].textInput.Value()) {
					m.commands[m.currentCmd].textInput.Blur()
					m.commands[m.currentCmd].hintInput.Focus()
				}
			}
		case tea.KeyLeft:
			if m.commands[m.currentCmd].hintInput.Focused() {
				if m.commands[m.currentCmd].hintInput.GetCursor() == 0 {
					m.commands[m.currentCmd].hintInput.Blur()
					m.commands[m.currentCmd].textInput.Focus(true)
				}
			}

		case tea.KeyDown:
			if m.historyPos > 1 {
				m.historyPos--
				m.commands[m.currentCmd].textInput.SetValue(m.cmdHistory[len(m.cmdHistory)-m.historyPos])
			} else if m.historyPos == 1 {
				m.commands[m.currentCmd].textInput.SetValue("")
			} else if m.historyPos == 0 {
				if !m.commands[m.currentCmd].hintInput.Focused() {
					m.commands[m.currentCmd].hintInput.Focus()
					m.commands[m.currentCmd].textInput.Blur()
				}
			}
		case tea.KeyBackspace:
			if m.commands[m.currentCmd].hintInput.Focused() {
				m.commands[m.currentCmd].hintInput.Blur()
				m.commands[m.currentCmd].textInput.Focus(false)
			}
		case tea.KeyTab: // Accept hint
			m.commands[m.currentCmd].hintInput.Blur()
			m.commands[m.currentCmd].textInput.SetValue(m.commands[m.currentCmd].hintInput.GetChoice())
			m.commands[m.currentCmd].textInput.SetCursor(len(m.commands[m.currentCmd].textInput.Value()))
			m.commands[m.currentCmd].textInput.Focus(true)
		case tea.KeyCtrlC:
			if m.commands[m.currentCmd].hintInput.Focused() {
				m.commands[m.currentCmd].hintInput.Blur()
				m.commands[m.currentCmd].textInput.Focus(true)
			}
			m.commands[m.currentCmd].textInput.SetValue("")
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		if !m.ready {
			m.viewport = viewport.Model{
				Width:           msg.Width,
				Height:          msg.Height,
				MouseWheelDelta: 1,
			}
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height
			//m.fixViewport(true)
		}

	// We handle errors just like any other message
	case errMsg:
		m.err = msg
		return m, nil
	}

	m.commands[m.currentCmd].textInput, cmd = m.commands[m.currentCmd].textInput.Update(msg)
	cmds = append(cmds, cmd)
	m.commands[m.currentCmd].hintInput.UpdateHintOptions(m.commands[m.currentCmd].textInput.Value())
	m.commands[m.currentCmd].hintInput, cmd = m.commands[m.currentCmd].hintInput.Update(msg)
	cmds = append(cmds, cmd)

	m.SetContent(m.width)
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (c command) View(width int) string {
	promptStyle := lipgloss.NewStyle().Width(width - 2).MarginTop(1).BorderStyle(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#56EEF4")).Render
	errorStyle := lipgloss.NewStyle().Width(width - 2).MarginTop(1).BorderStyle(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#DB162F")).Render
	successStyle := lipgloss.NewStyle().Width(width - 2).MarginTop(1).BorderStyle(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#C7EF00")).Render

	if len(c.stdout) == 0 && len(c.stderr) == 0 {
		return promptStyle(lipgloss.JoinVertical(lipgloss.Left, c.textInput.View(), c.hintInput.View()))
	} else if len(c.stdout) > 0 {
		if len(c.stderr) > 0 {
			return errorStyle(lipgloss.JoinVertical(lipgloss.Left, c.textInput.View(), c.stdout, c.stderr))
		} else {
			return successStyle(lipgloss.JoinVertical(lipgloss.Left, c.textInput.View(), c.stdout))
		}
	} else {
		return errorStyle(lipgloss.JoinVertical(lipgloss.Left, c.textInput.View(), c.stderr))
	}
}

func (m *model) SetContent(width int) {
	var b strings.Builder

	for i := range m.commands {
		b.WriteString(m.commands[i].View(width))
	}

	m.viewport.SetContent(b.String())

	if m.toBottom {
		m.viewport.GotoBottom()
		m.toBottom = false
	}
}

func (m model) View() string {
	return m.viewport.View()
}

func execCmd(args []string) (bytes.Buffer, bytes.Buffer, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

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
				return stdout, stderr, errors.New("path required")
			}

			return stdout, stderr, os.Chdir(args[1])
		case "exit":
			return stdout, stderr, exitError
		}

		// Make sure command exists
		_, err := exec.LookPath(args[0])
		if err != nil {
			errMsg := fmt.Sprintf("didn't find '%s'", args[0])
			return stdout, stderr, errors.New(errMsg)
		} else {
			// Prepare command to execute
			cmd := exec.Command(args[0], args[1:]...)

			// cmd.Stdout = os.Stdout
			// cmd.Stderr = os.Stderr
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			//Execute the command
			if backgroundProcess {
				return stdout, stderr, cmd.Start()
			} else {
				cmd.Stdin = os.Stdin
				return stdout, stderr, cmd.Run()
			}
		}
	} else {
		return stdout, stderr, nil
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

	if len(input) > 0 {
		// Make sure background commands have a space
		if input[len(input)-1] == '&' && input[len(input)-2] != ' ' {
			input = fmt.Sprintf("%s%s", input[:len(input)-1], " &")
		}

		// Split on space to build command
		cmd := strings.Split(input, " ")

		return cmd
	} else {
		return []string{}
	}
}

func main() {
	usr, _ := user.Current()
	homeDir := usr.HomeDir

	cmdHistory, commands, init_err := initialize(homeDir)
	if init_err != nil {
		fmt.Println("Initialization Error:", init_err)
	} else {
		p := tea.NewProgram(initialModel(homeDir, cmdHistory, commands), tea.WithAltScreen(), tea.WithMouseCellMotion())

		_, err := p.Run()
		if err != nil {
			fmt.Println("Oh no:", err)
			os.Exit(1)
		}
	}
}
