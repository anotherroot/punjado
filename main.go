package main

// logging
import (
	// "github.com/davecgh/go-spew/spew"
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
			Background(lipgloss.Color("#5A5A5A")). // Dark Grey
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

// --- 1. DATA STRUCTURES ---

type FileNode struct {
	Name     string
	Path     string
	IsDir    bool
	IsBinary bool
	Size     int64
	Children []*FileNode
	Parent   *FileNode // Back-reference helper

	// State
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
			//check all children

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
			//check all children

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

// Recursive function to select/deselect a node and all its children
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

// Toggle expansion (only for directories)
func (n *FileNode) ToggleExpand() {
	if n.IsDir {
		n.Expanded = !n.Expanded
	}
}

// --- 2. FILE SYSTEM SCANNER ---

func buildFileTree(rootPath string) (*FileNode, error) {
	root := &FileNode{
		Name:     rootPath,
		Path:     rootPath,
		IsDir:    true,
		Expanded: true, // Start with root open
		Depth:    0,
	}

	// Walk the directory
	err := filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if path == rootPath {
			return nil
		}

		// Simple ignore logic (add your gitignore logic here later)
		if strings.Contains(path, ".git") || strings.Contains(path, ".punjado")|| strings.Contains(path, "node_modules") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Find parent
		// This is a naive implementation (re-traversing from root).
		// For massive repos, a map lookup is faster, but this works fine for <5000 files.
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

// Helper to find a node by path
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

// --- 3. FLATTENER (The Magic) ---
// Turns the tree into a simple list of "Visible Items" based on what is expanded
func flattenVisible(root *FileNode) []*FileNode {
	var result []*FileNode
	// We do not add the root folder itself to the list to save space,
	// or you can add it if you want. Let's add the root's children.

	var traverse func(n *FileNode)
	traverse = func(n *FileNode) {
		result = append(result, n)
		if n.IsDir && n.Expanded {
			for _, child := range n.Children {
				traverse(child)
			}
		}
	}

	// Start with root children so we don't see "." as the first item
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

// --- 4. MODEL ---

type model struct {
	root         *FileNode
	visibleNodes []*FileNode // The currently rendered list
	cursor       int
	viewport     viewport.Model
	ready        bool
	height       int
	width        int
}

func initialModel(startPath string) model {
	path, err := filepath.Abs(startPath)
	if err != nil {
		// Fallback if the path is weird
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
		// 3. Icon
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

		// 4. Indentation
		indent := strings.Repeat("  ", node.Depth)

		// 5. Build Line
		line := fmt.Sprintf("%s %s %s%s %s", indent, icon, node.Name, dirAddon, addon)

		style := binFileStyle

		if node.Selected {
			style = selectedFileStyle
		} else if node.SomeSelected {
			style = someSelectedStyle
		} else if !node.IsBinary && !emptyDir {
			style = textFileStyle
		}

		// Highlight
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
		footerHeight := 1 // Adjust if you add a footer
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
		case "ctrl+c", "q":
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
			// Sync Viewport Scroll
			if m.cursor < m.viewport.YOffset {
				m.viewport.SetYOffset(m.cursor)
			}

		case "down", "j":
			if m.cursor < len(m.visibleNodes)-1 {
				m.cursor++
			}
			// Sync Viewport Scroll
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

		case " ", "s": // Added enter for selection too if you like
			node := m.visibleNodes[m.cursor]
			if node.IsBinary {
				return m, nil
			}
			newState := !node.Selected
			node.SetSelected(newState)
		}
	}

	saveState(m.root, m.root.Path)
	// CRITICAL FIX: Set the content inside Update
	// This saves the "lines" into the viewport model that gets returned
	m.viewport.SetContent(m.renderContent())

	return m, nil
}

func (m model) ToClipboard() {

}

// --- VIEW ---
func (m model) View() string {
	if !m.ready {
		return "Initializing..."
	}
	// View is now dumb: just print the header and the viewport
	return fmt.Sprintf("%s%s%s", m.ViewHeader(), m.viewport.View(), m.ViewFooter())
}

func (m model) ViewHeader() string {
	title := "Punjado"

	// Calculate tokens
	count := m.countSelectedTokens()
	tokenText := fmt.Sprintf("%d tokens", count)

	// Change color if too large (>32k)
	style := tokenStyle
	if count > 32000 {
		style = style.Background(lipgloss.Color("#E03A3E")) // Red warning
	}

	// Calculate space between title and tokens
	// m.width is the total terminal width we saved in Update
	// We subtract the length of our text to know how many spaces to add
	w := m.width - lipgloss.Width(title) - lipgloss.Width(tokenText) - 4 // -4 for padding safety
	if w < 0 {
		w = 0
	}
	spacer := strings.Repeat(" ", w)

	// Render: [Title] [Spacer] [Tokens]
	return headerStyle.Render(title) + spacer + style.Render(tokenText) + "\n"
}

func (m model) ViewFooter() string {
	// Helper to make a key:value pair
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

	// Recursive helper
	var traverse func(n *FileNode)
	traverse = func(n *FileNode) {
		// If it's a file and it's selected, add its size
		if !n.IsDir && n.Selected {
			totalSize += n.Size
		}
		// Always check children (even if parent isn't selected,
		// logic might allow partial selection later)
		for _, child := range n.Children {
			traverse(child)
		}
	}

	traverse(m.root)

	// Heuristic: 1 token ~= 4 bytes
	return int(totalSize / 4)
}

// --- PERSISTENCE ---

func saveState(root *FileNode, rootPath string) {
	var selectedPaths []string

	// 1. Collect all selected paths
	var traverse func(n *FileNode)
	traverse = func(n *FileNode) {
		// We only save specific selected items.
		// If a folder is selected, saving the folder path is enough
		// if your loading logic re-selects children.
		// However, saving LEAF nodes (files) is the safest and most robust way.
		if n.Selected && !n.IsDir {
			// Save relative path to make it portable
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

	// 2. Write to .punjado
	saveFile := filepath.Join(rootPath, ".punjado")
	content := strings.Join(selectedPaths, "\n")
	os.WriteFile(saveFile, []byte(content), 0644)
}

func loadState(root *FileNode, rootPath string) {
	saveFile := filepath.Join(rootPath, ".punjado")
	data, err := os.ReadFile(saveFile)
	if err != nil {
		return // No save file, just start fresh
	}

	// 1. Load paths into a map for fast lookup
	selectedMap := make(map[string]bool)
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			selectedMap[line] = true
		}
	}

	// 2. Walk the tree and re-select
	var traverse func(n *FileNode)
	traverse = func(n *FileNode) {
		rel, _ := filepath.Rel(rootPath, n.Path)
		if selectedMap[rel] {
			// logic: just mark it selected.
			// Your SetSelected logic will handle parents automatically.
			n.SetSelected(true)
		}

		for _, child := range n.Children {
			traverse(child)
		}
	}
	traverse(root)
}

func isBinaryFile(path string) bool {
	// 1. Fast Path: Check common extensions to avoid disk I/O
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".ico", ".webp", // Images
		".pdf", ".zip", ".tar", ".gz", ".7z", ".rar", // Archives
		".exe", ".dll", ".so", ".dylib", ".bin", // Executables
		".mp3", ".mp4", ".wav", ".avi", ".mov": // Media
		return true
	}

	// 2. Slow Path: Read the first 512 bytes
	f, err := os.Open(path)
	if err != nil {
		return false // If we can't read it, assume it's text (or handle error)
	}
	defer f.Close()

	// Read up to 512 bytes
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

func main() {
	f, err := tea.LogToFile("debug.log", "debug")
	if err != nil {
		fmt.Println("fatal:", err)
		os.Exit(1)
	}
	defer f.Close()

	log.Println()
	log.Println("Starting Punjado!!")
	log.Println()

	startPath := "."
	if len(os.Args) > 1 {
		startPath = os.Args[1]
	}
	p := tea.NewProgram(initialModel(startPath), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
}
