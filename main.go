package main

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/atotto/clipboard"
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

func readConfig(dir string) map[string]bool {
	m := make(map[string]bool)
	// Construct the full path: dir/.punjado
	path := filepath.Join(dir, ".punjado")

	data, _ := os.ReadFile(path)
	for _, line := range strings.Split(string(data), "\n") {
		if s := strings.TrimSpace(line); s != "" {
			m[s] = true
		}
	}
	return m
}

func writeConfig(dir string, m map[string]bool) {
	var lines []string
	for k := range m {
		lines = append(lines, k)
	}

	path := filepath.Join(dir, ".punjado")
	os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644)
}

func VarifyFlags(userFlags map[string]string, allowedList []string) {
	// 1. Create a "Set" of allowed flags for fast lookup
	// We use a map[string]bool because checking a map is instant
	allowed := make(map[string]bool)
	for _, flagName := range allowedList {
		allowed[flagName] = true
	}

	// 2. Iterate over the flags the USER actually provided
	for flagName := range userFlags {
		// 3. Check if the user's flag exists in our allowed list
		if !allowed[flagName] {
			fmt.Printf("Error: unknown flag '--%s'\n", flagName)
			fmt.Printf("Valid flags for this command are: %v\n", allowedList)
			os.Exit(1) // Exit the program immediately
		}
	}
}

func GetFlag(flags map[string]string, key, defaultVal string) string {
	if val, ok := flags[key]; ok {
		return val
	}
	return defaultVal
}

func HasFlag(flags map[string]string, key string) bool {
	if _, ok := flags[key]; ok {
		return true
	}
	return false
}

func HandleRun(params []string, flags map[string]string) {
	VarifyFlags(flags, []string{"help", "stdout", "dir"})

	if HasFlag(flags, "help") {
		HandleHelp(params, flags)
		return
	}

	dir := GetFlag(flags, "dir", ".")
	RunTUI(dir)
}

func HandleOpen(params []string, flags map[string]string) {
	VarifyFlags(flags, []string{"help", "stdout", "dir"})

	if HasFlag(flags, "help") {
		fmt.Printf("Help for add....")
		return
	}

	dir := GetFlag(flags, "dir", ".")
	RunTUI(dir)

}

func HandleAdd(params []string, flags map[string]string) {
	VarifyFlags(flags, []string{"help", "dir"})

	if HasFlag(flags, "help") {
		fmt.Printf("Help for add....")
		return
	}

	dir := GetFlag(flags, "dir", ".")

	config := readConfig(dir)
	for _, f := range params {
		clean := filepath.Clean(f)
		config[clean] = true
		fmt.Printf("Added: %s\n", clean)
	}
	writeConfig(dir, config)
}

func HandleRemove(params []string, flags map[string]string) {
	VarifyFlags(flags, []string{"help", "dir"})

	if HasFlag(flags, "help") {
		fmt.Printf("Help for remove....")
		return
	}

	dir := GetFlag(flags, "dir", ".")

	config := readConfig(dir)
	for _, f := range params {
		clean := filepath.Clean(f)
		delete(config, clean)
		fmt.Printf("Removed: %s\n", clean)
	}
	writeConfig(dir, config)
}

func HandleCopy(params []string, flags map[string]string) {
	VarifyFlags(flags, []string{"help", "dir", "stdout"})

	if HasFlag(flags, "help") {
		fmt.Printf("Help for copy....")
		return
	}

	dir := GetFlag(flags, "dir", ".")

	useStdOut := HasFlag(flags, "stdout")

	config := readConfig(dir)
	var sb strings.Builder
	for path := range config {
		sb.WriteString(fmt.Sprintf("\n--- FILE: %s ---\n", path))
		content, err := os.ReadFile(path)
		if err != nil {
			sb.WriteString(fmt.Sprintf("(Error reading file: %v)\n", err))
		} else {
			sb.Write(content)
		}
		sb.WriteString("\n")
	}
	finalText := sb.String()
	if useStdOut {
		fmt.Print(finalText)
	} else {
		err := clipboard.WriteAll(finalText)
		if err != nil {
			fmt.Println("Error copying to clipboard (install xclip/wl-copy on Linux):", err)
			os.Exit(1)
		}
		fmt.Printf("Copied %d files to clipboard!\n", len(config))
	}
}

func HandleProfile(params []string, flags map[string]string) {
	VarifyFlags(flags, []string{"help", "dir"})

	if HasFlag(flags, "help") {
		fmt.Printf("Help for profile....")
		return
	}

	// dir := GetFlag(flags, "dir", ".")

	fmt.Printf("Error: HandleProfile not implemented")
}

func HandleToggle(params []string, flags map[string]string) {
	VarifyFlags(flags, []string{"help", "dir"})

	if HasFlag(flags, "help") {
		fmt.Printf("Help for toggle....")
		return
	}

	dir := GetFlag(flags, "dir", ".")

	if len(params) < 1 {
		fmt.Println("Usage: punjado toggle <file>")
		return
	}
	config := readConfig(dir)
	file := filepath.Clean(params[0])

	if config[file] {
		delete(config, file)
		fmt.Printf("Removed: %s\n", file)
	} else {
		config[file] = true
		fmt.Printf("Added: %s\n", file)
	}
	writeConfig(dir, config)
}

func HandleGit(params []string, flags map[string]string) {
	VarifyFlags(flags, []string{"help", "dir"})

	if HasFlag(flags, "help") {
		fmt.Printf("Help for toggle....")
		return
	}

	dir := GetFlag(flags, "dir", ".")

	cmd := exec.Command("git", "status", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		fmt.Println("Error running git status. Is this a git repo?")
		os.Exit(1)
	}
	config := readConfig(dir)
	lines := strings.Split(string(output), "\n")
	count := 0
	for _, line := range lines {
		if len(line) < 4 {
			continue
		}
		path := strings.TrimSpace(line[3:])
		path = filepath.Clean(path)
		if !config[path] {
			config[path] = true
			fmt.Printf("Git file added: %s\n", path)
			count++
		}
	}
	writeConfig(dir, config)
	fmt.Printf("Successfully added %d files from git status.\n", count)
}

func HandleList(params []string, flags map[string]string) {
	VarifyFlags(flags, []string{"help", "dir"})

	if HasFlag(flags, "help") {
		fmt.Printf("Help for toggle....")
		return
	}

	dir := GetFlag(flags, "dir", ".")

	config := readConfig(dir)
	for file := range config {
		fmt.Println(file)
	}
}

func HandleHelp(params []string, flags map[string]string) {
	fmt.Println(`Punjado - Context Manager

Usage:
  punjado [path]        Open TUI in directory
  punjado open [path]   Open TUI in directory
  punjado add <files>   Add files to context
  punjado remove <files> Remove files from context
  punjado toggle <file> Toggle file context
  punjado list          List selected files
  punjado copy          Copy context to clipboard (flags: --std)
  punjado git           Add all changed git files`)
}

type Flag struct {
	Long         string
	Short        byte
	HasParameter bool
}

func ParseArgs() (string, []string, map[string]string, error) {
	validFlags := []Flag{
		{
			Long:         "dir",
			Short:        'd',
			HasParameter: true,
		},
		{
			Long:         "stdout",
			Short:        0,
			HasParameter: false,
		},
		{
			Long:         "version",
			Short:        0,
			HasParameter: false,
		},
		{
			Long:         "help",
			Short:        0,
			HasParameter: false,
		},
	}

	flagHasParameter := make(map[string]bool)
	shortFlagHasParameter := make(map[byte]bool)
	shortFlagToLongFlag := make(map[byte]string)

	for _, f := range validFlags {
		flagHasParameter[f.Long] = f.HasParameter

		if f.Short != 0 {
			shortFlagHasParameter[f.Short] = f.HasParameter
			shortFlagToLongFlag[f.Short] = f.Long
		}
	}

	exe := os.Args[0]

	cmds := []string{}
	flags := make(map[string]string)

	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		if arg[0:2] == "--" {
			arg = arg[2:]
			if val, ok := flagHasParameter[arg]; ok {
				flagVal := ""
				if val {
					if len(os.Args) <= i+1 {
						return "", nil, nil, fmt.Errorf("Flag '%s' requires a parameter", arg)
					}
					flagVal = os.Args[i+1]
					i++
				}
				flags[arg] = flagVal
			} else {
				return "", nil, nil, fmt.Errorf("Flag '%s' is not recognised", arg)
			}
		} else if arg[0] == '-' {
			arg = arg[1:]
			for i := 0; i < len(arg); i++ {
				c := arg[i]
				if val, ok := shortFlagHasParameter[c]; ok {
					flagVal := ""
					if val {
						if len(os.Args) <= i+1 {
							return "", nil, nil, fmt.Errorf("Short flag '-%c' requires a parameter", c)
						}
						flagVal = os.Args[i+1]
						i++
					}
					flags[shortFlagToLongFlag[c]] = flagVal
				} else {
					return "", nil, nil, fmt.Errorf("Short flag '-%c' is not recognised", c)
				}

			}
		} else {
			cmds = append(cmds, arg)
		}
	}

	return exe, cmds, flags, nil

}

func main() {
	f, err := tea.LogToFile("debug.log", "debug")
	if err != nil {
		fmt.Println("fatal:", err)
		os.Exit(1)
	}
	defer f.Close()

	_, cmds, flags, err := ParseArgs()
	if err != nil {
		fmt.Println("fatal:", err)
		os.Exit(1)
	}

	if len(cmds) == 0 && len(flags) == 0 {
		HandleRun(cmds, flags)
		return
	}
	subcommand := cmds[0]
	params := cmds[1:]

	switch subcommand {
	case "open":
		HandleOpen(params, flags)

	case "add":
		HandleAdd(params, flags)

	case "list":
		HandleList(params, flags)

	case "remove":
		HandleRemove(params, flags)

	case "git":
		HandleGit(params, flags)

	case "toggle":
		HandleToggle(params, flags)

	case "profile":
		HandleProfile(params, flags)

	case "copy":
		HandleCopy(params, flags)

	case "help":
		HandleHelp(params, flags)

	default:
		// If argument is a path, open TUI
		if _, err := os.Stat(os.Args[1]); err == nil {
			RunTUI(os.Args[1])
		} else {
			HandleHelp(params, flags)
		}
	}
}
