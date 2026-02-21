package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1).
			Bold(true)

	tokenStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#5A5A5A")).
			Padding(0, 1)

	keyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#3C3C3C")).
			Padding(0, 1).
			MarginRight(1)

	descStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A0A0A0")).
			MarginRight(3)

	gitignoreFileStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#909090")).
				MarginRight(3)

	binFileStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#909090")).
			MarginRight(3)

	emptyDirStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#909090")).
			MarginRight(3)

	textFileStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FAFAFA")).
			MarginRight(3)

	selectedFileStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#B8BB26")).
				MarginRight(3).Bold(true)

	someSelectedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#ffa000")).
				MarginRight(3)
)

type model struct {
	root         *FileNode
	visibleNodes []*FileNode
	cursor       int
	viewport     viewport.Model
	ready        bool
	height       int
	width        int
	helpMode     bool
	selectedMode bool

	quitting bool

	undoStack []Action
	redoStack []Action

	keymaps map[string]Keymap

	keySeq          string
	filteredKeymaps map[string]Keymap
}

type Keymap struct {
	keys   string
	cmdKey string
}

const gotoBottomCmdKey = "gotoBottom"
const gotoTopCmdKey = "gotoTop"
const closeHelpCmdKey = "closeHelp"
const toggleHelpCmdKey = "toggleHelp"
const toggleDirectoryCmdKey = "toggleDirectory"
const toggleAllCmdKey = "toggleAll"
const toggleFileCmdKey = "toggleFile"
const redoCmdKey = "redo"
const undoCmdKey = "undo"
const moveUpCmdKey = "moveUp"
const moveDownCmdKey = "moveDown"
const quitCmdKey = "quit"

var defaultKeymaps = []Keymap{
	{keys: "j", cmdKey: moveDownCmdKey},
	{keys: "<down>", cmdKey: moveDownCmdKey},
	{keys: "k", cmdKey: moveUpCmdKey},
	{keys: "<up>", cmdKey: moveUpCmdKey},
	{keys: "u", cmdKey: undoCmdKey},
	{keys: "<ctrl+r>", cmdKey: redoCmdKey},
	{keys: " ", cmdKey: toggleFileCmdKey},
	{keys: "s", cmdKey: toggleFileCmdKey},
	{keys: "a", cmdKey: toggleAllCmdKey},
	{keys: "<enter>", cmdKey: toggleDirectoryCmdKey},
	{keys: "?", cmdKey: toggleHelpCmdKey},
	{keys: "<esc>", cmdKey: closeHelpCmdKey},
	{keys: "gg", cmdKey: gotoTopCmdKey},
	{keys: "G", cmdKey: gotoBottomCmdKey},
	{keys: "q", cmdKey: quitCmdKey},
	{keys: "ZZ", cmdKey: quitCmdKey},
}

type CmdFunc func(model) model

var commandRegistry = map[string]CmdFunc{
	moveDownCmdKey:        model.moveDown,
	moveUpCmdKey:          model.moveUp,
	undoCmdKey:            model.performUndo,
	redoCmdKey:            model.performRedo,
	toggleFileCmdKey:      model.toggleCurrentFile,
	toggleAllCmdKey:       model.toggleAllFiles,
	toggleDirectoryCmdKey: model.toggleDirectory,
	toggleHelpCmdKey:      model.toggleHelp,
	closeHelpCmdKey:       model.closeHelp,
	gotoTopCmdKey:         model.gotoTop,
	gotoBottomCmdKey:      model.gotoBottom,
	quitCmdKey:            model.quit,
}

func filterKeymap(originalMap map[string]Keymap, prefix string) map[string]Keymap {

	filteredMap := make(map[string]Keymap)

	for key, cmd := range originalMap {

		if strings.HasPrefix(key, prefix) {

			filteredMap[key] = cmd
		}
	}

	return filteredMap
}

func initKeymaps() map[string]Keymap {
	//TODO: add parameter for config
	var keymaps = defaultKeymaps

	fastLookupMap := make(map[string]Keymap)
	for _, item := range keymaps {
		fastLookupMap[item.keys] = item
	}
	return fastLookupMap
}

func initialModel(startPath string) model {
	path, err := filepath.Abs(startPath)
	if err != nil {

		path, _ = os.Getwd()
	}
	root, _ := buildFileTree(path)

	loadState(root, path)
	var keymaps = initKeymaps()

	return model{
		root:            root,
		visibleNodes:    flattenVisible(root),
		cursor:          0,
		keymaps:         keymaps,
		keySeq:          "",
		filteredKeymaps: keymaps,
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) renderContent() string {
	var s strings.Builder

	for i, node := range m.visibleNodes {

		emptyDir := node.IsDir && len(node.Children) == 0
		notEmptyDir := node.IsDir && len(node.Children) > 0

		icon := ""
		if notEmptyDir {
			if node.Expanded {
				icon = "▼"
			} else {
				icon = "▶"
			}
		}
		addon := ""
		if node.IsBinary {
			addon = "(bin)"
		}
		dirAddon := ""
		if node.IsDir {
			dirAddon = "/"
		}

		indent := strings.Repeat("  ", node.Depth)

		line := fmt.Sprintf("%s %s %s%s %s", indent, icon, node.Name, dirAddon, addon)

		style := binFileStyle

		if node.Selected {
			style = selectedFileStyle
		} else if node.SomeSelected {
			style = someSelectedStyle
		} else if !node.IsBinary && !emptyDir {
			style = textFileStyle
		}

		if m.cursor == i {
			style = style.Width(m.viewport.Width).Background(lipgloss.Color("#444"))
		}
		line = style.Render(line)

		s.WriteString(line + "\n")
	}
	return s.String()
}

func keyMsgToKeyStr(msg string) string {
	if len(msg) == 1 {
		return msg
	}
	return "<" + msg + ">"
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	log.Printf("%+v", msg)

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.height = msg.Height
		m.width = msg.Width
		headerHeight := 1
		footerHeight := 1
		if m.helpMode {
			footerHeight = 10
		}
		verticalMargin := headerHeight + footerHeight

		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-verticalMargin)
			m.viewport.YPosition = headerHeight
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - verticalMargin
		}

	case tea.KeyMsg:
		keyStr := keyMsgToKeyStr(msg.String())
		log.Printf("Pressed '%s'", keyStr)
		m.keySeq = m.keySeq + keyStr
		m.filteredKeymaps = filterKeymap(m.keymaps, m.keySeq)
		log.Printf("keySeq '%s'", m.keySeq)
		log.Printf("filteredKeymaps '%+v'", m.filteredKeymaps)

		if len(m.filteredKeymaps) == 0 {
			log.Printf("Reset keySeq")
			m.keySeq = ""
			m.filteredKeymaps = m.keymaps
		} else {
			val, ok := m.filteredKeymaps[m.keySeq]
			if ok {
				fn, ok := commandRegistry[val.cmdKey]
				if ok {
					m = fn(m)
					log.Printf("Running command '%s'", val.cmdKey)
				} else {
					log.Printf("Command key '%s' not found in command registry", val.cmdKey)
				}
				m.keySeq = ""
				m.filteredKeymaps = m.keymaps
			}
		}
	}
	if m.quitting {
		return m, tea.Quit
	}

	m.viewport.SetContent(m.renderContent())

	return m, nil
}

func (m model) View() string {
	if !m.ready {
		return "Initializing..."
	}
	footer := m.ViewFooter()
	if m.helpMode {
		footer = m.ViewExpandedHelp()
	}

	return fmt.Sprintf("%s%s%s", m.ViewHeader(), m.viewport.View(), footer)
}

func (m model) ViewHeader() string {
	title := "Punjado"

	count := m.countSelectedTokens()
	tokenText := fmt.Sprintf("%d tokens", count)

	style := tokenStyle
	if count > 32000 {
		style = style.Background(lipgloss.Color("#E03A3E"))
	}

	w := m.width - lipgloss.Width(title) - lipgloss.Width(tokenText) - 4
	if w < 0 {
		w = 0
	}
	spacer := strings.Repeat(" ", w)

	return headerStyle.Render(title) + spacer + style.Render(tokenText) + "\n"
}

func (m model) ViewExpandedHelp() string {
	colStyle := lipgloss.NewStyle().MarginRight(4).Foreground(lipgloss.Color("#A0A0A0"))
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FAFAFA")).MarginBottom(1)
	separatorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#626262"))

	group := func(title string, keys ...string) string {
		lines := []string{titleStyle.Render(title)}
		for i := 0; i < len(keys); i += 2 {
			k := keyStyle.Render(keys[i])
			desc := descStyle.Render(keys[i+1])
			lines = append(lines, fmt.Sprintf("%s %s", k, desc))
		}
		return colStyle.Render(strings.Join(lines, "\n"))
	}

	col1 := group("NAVIGATION",
		"k/↑", "Up",
		"j/↓", "Down",
		"u", "Page Up",
		"d", "Page Down",
		"g", "Go to Top",
		"G", "Go to Bottom",
	)

	col2 := group("SELECTION",
		"space", "Toggle Dir",
		"s", "Select File",
		"enter", "Open Dir",
		"a", "Toggle All",
	)

	col3 := group("ACTIONS",
		"T", "Expand All",
		"y", "Copy",
		"q", "Quit",
		"?", "Close Help",
	)

	prefix := "─── Help "
	dashCount := m.width - len([]rune(prefix))
	if dashCount < 0 {
		dashCount = 0
	}
	headerLine := separatorStyle.Render(prefix + strings.Repeat("─", dashCount) + "\n")
	columns := lipgloss.JoinHorizontal(lipgloss.Top, col1, col2, col3)
	return "\n" + headerLine + "\n" + columns + "\n"
}

func (m model) ViewFooter() string {
	key := func(k, desc string) string {
		return keyStyle.Render(k) + descStyle.Render(desc)
	}

	// 1. Build the left side
	leftSide := key("↑/↓", "move") +
		key("space", "toggle dir") +
		key("s", "select") +
		key("q", "quit") +
		key("a", "toggle all") +
		key("y", "to clipboard")

	// 2. Build the right side (the active sequence)
	rightSide := ""
	if m.keySeq != "" {
		// Give it a nice bold/colored style so the user notices it
		seqStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#ffa000")).Bold(true)
		rightSide = seqStyle.Render(m.keySeq)
	}

	// 3. Calculate spacing
	w := m.width - lipgloss.Width(leftSide) - lipgloss.Width(rightSide)
	if w < 0 {
		w = 0
	}
	spacer := strings.Repeat(" ", w)

	return "\n" + leftSide + spacer + rightSide
}

func (m model) countSelectedTokens() int {
	var totalSize int64
	var traverse func(n *FileNode)
	traverse = func(n *FileNode) {
		if !n.IsDir && n.Selected {
			totalSize += n.Size
		}
		for _, child := range n.Children {
			traverse(child)
		}
	}
	traverse(m.root)
	return int(totalSize / 4)
}

func RunTUI(startPath string) {
	if os.Getenv("DEBUG") == "true" {
		f, err := tea.LogToFile("debug.log", "debug")
		if err != nil {
			fmt.Println("fatal:", err)
			os.Exit(1)
		}
		defer f.Close()
	} else {
		log.SetOutput(io.Discard)
	}

	log.Printf("Starting Punjado TUI at '%s'!!", startPath)
	p := tea.NewProgram(initialModel(startPath), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
}
