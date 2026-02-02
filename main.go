package main

import (
	"log"
)

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
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
				Foreground(lipgloss.Color("#6F6")).
				MarginRight(3)

	someSelectedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#ffa000")).
				MarginRight(3)
)

type FileNode struct {
	Name     string
	Path     string
	IsDir    bool
	IsBinary bool
	Size     int64
	Children []*FileNode
	Parent   *FileNode

	Expanded     bool
	Selected     bool
	SomeSelected bool
	Depth        int
}

func (n *FileNode) SetSelectParentFromChild(selected bool) {
	if n.Parent == nil {
		return
	}

	if selected == true {
		if n.Parent.Selected == false {

			allSelected := true
			for _, node := range n.Parent.Children {

				emptyDir := node.IsDir && len(node.Children) == 0
				if node.IsBinary || emptyDir {
					continue
				}
				if node.Selected == false {
					allSelected = false
					break
				}
			}
			if allSelected {
				n.Parent.SomeSelected = false
				n.Parent.Selected = true
			} else {
				n.Parent.SomeSelected = true
			}
			n.Parent.SetSelectParentFromChild(selected)
		}
	} else {
		if n.Parent.Selected == true || n.Parent.SomeSelected == true {

			allNotSelected := true
			for _, node := range n.Parent.Children {
				emptyDir := node.IsDir && len(node.Children) == 0
				if node.IsBinary || emptyDir {
					continue
				}
				if node.Selected == true {
					allNotSelected = false
					break
				}
			}
			if allNotSelected {
				n.Parent.Selected = false
				n.Parent.SomeSelected = false
			} else {
				n.Parent.SomeSelected = true
				n.Parent.Selected = false
			}
		}
		n.Parent.SetSelectParentFromChild(selected)
	}

}

func (n *FileNode) SetSelected(selected bool) {
	emptyDir := n.IsDir && len(n.Children) == 0
	if n.IsBinary || emptyDir {
		return
	}
	n.Selected = selected
	for _, child := range n.Children {
		child.SetSelected(selected)
	}
	n.SetSelectParentFromChild(selected)
}

func (n *FileNode) ToggleExpand() {
	if n.IsDir {
		n.Expanded = !n.Expanded
	}
}

func buildFileTree(rootPath string) (*FileNode, error) {
	root := &FileNode{
		Name:     rootPath,
		Path:     rootPath,
		IsDir:    true,
		Expanded: true,
		Depth:    0,
	}

	err := filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if path == rootPath {
			return nil
		}

		if strings.Contains(path, ".git") || strings.Contains(path, ".punjado") || strings.Contains(path, "node_modules") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		parentDir := filepath.Dir(path)
		parent := findNode(root, parentDir)

		if parent != nil {
			info, _ := d.Info()
			var size int64
			var isBin bool
			if info != nil {
				size = info.Size()
				if !d.IsDir() {
					isBin = isBinaryFile(path)
				}
			}
			node := &FileNode{
				Name:     d.Name(),
				Path:     path,
				Size:     size,
				IsBinary: isBin,
				IsDir:    d.IsDir(),
				Parent:   parent,
				Depth:    parent.Depth + 1,
				Expanded: true,
			}
			parent.Children = append(parent.Children, node)
		}
		return nil
	})

	return root, err
}

func findNode(root *FileNode, path string) *FileNode {
	if root.Path == path {
		return root
	}
	for _, child := range root.Children {
		found := findNode(child, path)
		if found != nil {
			return found
		}
	}
	return nil
}

func flattenVisible(root *FileNode) []*FileNode {
	var result []*FileNode

	var traverse func(n *FileNode)
	traverse = func(n *FileNode) {
		result = append(result, n)
		if n.IsDir && n.Expanded {
			for _, child := range n.Children {
				traverse(child)
			}
		}
	}

	if root.Expanded {
		for _, child := range root.Children {
			traverse(child)
		}
	}

	log.Println("flattenVisible len: ", len(result))
	names := []string{}
	for _, n := range result {
		names = append(names, n.Name)
	}
	return result
}

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
}

func initialModel(startPath string) model {
	path, err := filepath.Abs(startPath)
	if err != nil {

		path, _ = os.Getwd()
	}
	root, _ := buildFileTree(path)

	loadState(root, path)

	return model{
		root:         root,
		visibleNodes: flattenVisible(root),
		cursor:       0,
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
		switch msg.String() {

		case "?":
			m.helpMode = !m.helpMode
			headerHeight := 1
			footerHeight := 1
			if m.helpMode {
				footerHeight = 10
			}
			m.viewport.Height = m.height - headerHeight - footerHeight
		case "esc":
			if m.helpMode {
				m.helpMode = !m.helpMode
				headerHeight := 1
				footerHeight := 2
				if m.helpMode {
					footerHeight = 10
				}
				m.viewport.Height = m.height - headerHeight - footerHeight
			}
		case "ctrl+c", "q":
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

			if m.cursor < m.viewport.YOffset {
				m.viewport.SetYOffset(m.cursor)
			}

		case "down", "j":
			if m.cursor < len(m.visibleNodes)-1 {
				m.cursor++
			}

			if m.cursor >= m.viewport.YOffset+m.viewport.Height {
				m.viewport.SetYOffset(m.viewport.YOffset + 1)
			}

		case "enter":
			node := m.visibleNodes[m.cursor]
			if node.IsDir {
				node.ToggleExpand()
				m.visibleNodes = flattenVisible(m.root)
			}
		case "ctrl+d":
			m.viewport.PageDown()
			m.cursor = min(m.cursor+m.viewport.Height, len(m.visibleNodes)-1)
		case "ctrl+u":
			m.viewport.PageUp()
			m.cursor = max(m.cursor-m.viewport.Height, 0)
		case "g":
			m.cursor = 0
			m.viewport.GotoTop()
		case "G":
			m.cursor = len(m.visibleNodes) - 1
			m.viewport.GotoBottom()
		case "a":
			allNodesSelected := true
			for _, node := range m.visibleNodes {

				emptyDir := node.IsDir && len(node.Children) == 0
				if node.IsBinary || emptyDir {
					continue
				}
				if node.Selected != true {
					allNodesSelected = false
					break
				}
			}

			for _, node := range m.visibleNodes {
				emptyDir := node.IsDir && len(node.Children) == 0
				if node.IsBinary || emptyDir {
					continue
				}
				if allNodesSelected {
					node.SetSelected(false)
				} else {
					node.SetSelected(true)
				}
			}
		case "T":
			allNodesExpanded := true
			for _, node := range m.visibleNodes {
				if node.Expanded != true && node.IsDir {
					allNodesExpanded = false
					break
				}
			}

			for _, node := range m.visibleNodes {
				if node.IsDir {
					if allNodesExpanded {
						node.Expanded = false
					} else {
						node.Expanded = true
					}
				}
			}

			m.visibleNodes = flattenVisible(m.root)
		case "y":

		case "S":
			m.selectedMode = !m.selectedMode

		case " ", "s":
			node := m.visibleNodes[m.cursor]
			if node.IsBinary {
				return m, nil
			}
			newState := !node.Selected
			node.SetSelected(newState)
		}
	}

	saveState(m.root, m.root.Path)

	m.viewport.SetContent(m.renderContent())

	return m, nil
}

func (m model) ToClipboard() {

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
	// Define styles
	colStyle := lipgloss.NewStyle().MarginRight(4).Foreground(lipgloss.Color("#A0A0A0"))
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FAFAFA")).MarginBottom(1)

	// Create a dedicated style for the separator line (Subtle Grey)
	separatorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#626262"))

	// Helper to format a single command group
	group := func(title string, keys ...string) string {
		lines := []string{titleStyle.Render(title)}
		for i := 0; i < len(keys); i += 2 {
			k := keyStyle.Render(keys[i])
			desc := descStyle.Render(keys[i+1])
			lines = append(lines, fmt.Sprintf("%s %s", k, desc))
		}
		return colStyle.Render(strings.Join(lines, "\n"))
	}

	// Create columns (same as before)
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

	// --- NEW LOGIC: Create the Full-Width Header ---

	prefix := "─── Help "

	// Calculate how many dashes we need
	// We use m.width (saved in Update) to know the terminal size
	dashCount := m.width - len([]rune(prefix)) // Use []rune for accurate length

	// Safety check: avoid negative count if screen is tiny
	if dashCount < 0 {
		dashCount = 0
	}

	// Build the line: "--- Help " + "--------------------"
	headerLine := separatorStyle.Render(prefix + strings.Repeat("─", dashCount) + "\n")

	// Combine: [Newline] + [Header Line] + [Columns]
	columns := lipgloss.JoinHorizontal(lipgloss.Top, col1, col2, col3)
	return "\n" + headerLine + "\n" + columns + "\n"
}

func (m model) ViewFooter() string {

	key := func(k, desc string) string {
		return keyStyle.Render(k) + descStyle.Render(desc)
	}

	return "\n" +
		key("↑/↓", "move") +
		key("space", "toggle dir") +
		key("s", "select") +
		key("q", "quit") +
		key("a", "toggle all") +
		key("y", "to clipboard")
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

func saveState(root *FileNode, rootPath string) {
	var selectedPaths []string

	var traverse func(n *FileNode)
	traverse = func(n *FileNode) {

		if n.Selected && !n.IsDir {

			rel, err := filepath.Rel(rootPath, n.Path)
			if err == nil {
				selectedPaths = append(selectedPaths, rel)
			}
		}
		for _, child := range n.Children {
			traverse(child)
		}
	}
	traverse(root)

	saveFile := filepath.Join(rootPath, ".punjado")
	content := strings.Join(selectedPaths, "\n")
	os.WriteFile(saveFile, []byte(content), 0644)
}

func loadState(root *FileNode, rootPath string) {
	saveFile := filepath.Join(rootPath, ".punjado")
	data, err := os.ReadFile(saveFile)
	if err != nil {
		return
	}

	selectedMap := make(map[string]bool)
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			selectedMap[line] = true
		}
	}

	var traverse func(n *FileNode)
	traverse = func(n *FileNode) {
		rel, _ := filepath.Rel(rootPath, n.Path)
		if selectedMap[rel] {

			n.SetSelected(true)
		}

		for _, child := range n.Children {
			traverse(child)
		}
	}
	traverse(root)
}

func isBinaryFile(path string) bool {

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".ico", ".webp",
		".pdf", ".zip", ".tar", ".gz", ".7z", ".rar",
		".exe", ".dll", ".so", ".dylib", ".bin",
		".mp3", ".mp4", ".wav", ".avi", ".mov":
		return true
	}

	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	buf := make([]byte, 512)
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return false
	}
	buf = buf[:n]

	if bytes.IndexByte(buf, 0) != -1 {
		return true
	}

	return false
}

func RunTUI(startPath string) {
	log.Printf("Starting Punjado TUI at '%s'!!", startPath)
	p := tea.NewProgram(initialModel(startPath), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
}

func HandleAdd(fileNames []string) {
	log.Printf("HandleAdd(%+v)", fileNames)
}

func HandleRemove() {
	fmt.Printf("Error: HandleRemove not implemented")
}

func HandleCopy() {
	fmt.Printf("Error: HandleCopy not implemented")
}

func HandleProfile() {
	fmt.Printf("Error: HandleProfile not implemented")
}

func HandleToggle() {
	fmt.Printf("Error: HandleToggle not implemented")
}

func HandleGit() {
	fmt.Printf("Error: HandleGit not implemented")
}

func HandleList() {
	fmt.Printf("Error: HandleList not implemented")
}

func HandleHelp() {
	fmt.Printf("Error: help not implemented")
}

func main() {
	f, err := tea.LogToFile("debug.log", "debug")
	if err != nil {
		fmt.Println("fatal:", err)
		os.Exit(1)
	}
	defer f.Close()

	if len(os.Args) < 2 {
		RunTUI(".")
	}
	log.Printf("os.Args: %+v",os.Args)

	switch os.Args[1] {
	case "open":
		startPath := "."
		if len(os.Args) > 2 {
			startPath = os.Args[2]
		}
		RunTUI(startPath)

	case "add":
		fileNames := os.Args[2:]
		if len(fileNames) < 1 {
			fmt.Printf("No file passed to command")
			return
		}
		HandleAdd(fileNames)

	case "list":
		HandleList()

	case "remove":
		HandleRemove()

	case "git":
		HandleGit()

	case "toggle":
		HandleToggle()

	case "profile":
		HandleProfile()

	case "copy":
		HandleCopy()

	case "help":
		HandleHelp()

	default:
		HandleHelp()

	}
}
