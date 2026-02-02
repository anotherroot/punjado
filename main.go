package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// --- 1. DATA STRUCTURES ---

type FileNode struct {
	Name     string
	Path     string
	IsDir    bool
	Children []*FileNode
	Parent   *FileNode // Back-reference helper

	// State
	Expanded bool
	Selected bool
	Depth    int
}

// Recursive function to select/deselect a node and all its children
func (n *FileNode) SetSelected(selected bool) {
	n.Selected = selected
	for _, child := range n.Children {
		child.SetSelected(selected)
	}
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
		if strings.Contains(path, ".git") || strings.Contains(path, "node_modules") {
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
			node := &FileNode{
				Name:     d.Name(),
				Path:     path,
				IsDir:    d.IsDir(),
				Parent:   parent,
				Depth:    parent.Depth + 1,
				Expanded: false, // Subfolders start closed
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

	return model{
		root:         root,
		visibleNodes: flattenVisible(root),
		cursor:       0,
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.height = msg.Height
		m.width = msg.Width
		if !m.ready {
			// Initialize viewport with the remaining height
			// We reserve 3 lines for header/footer
			m.viewport = viewport.New(msg.Width, msg.Height-3)
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - 3
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
			// Scroll viewport if needed
			if m.cursor < m.viewport.YOffset {
				m.viewport.SetYOffset(m.cursor)
			}

		case "down", "j":
			if m.cursor < len(m.visibleNodes)-1 {
				m.cursor++
			}
			// Scroll viewport if needed
			if m.cursor >= m.viewport.YOffset+m.viewport.Height {
				m.viewport.SetYOffset(m.viewport.YOffset + 1)
			}

		case "enter":
			// Toggle Collapse/Expand
			node := m.visibleNodes[m.cursor]
			if node.IsDir {
				node.ToggleExpand()
				// Re-calculate the list!
				m.visibleNodes = flattenVisible(m.root)
			}

		case " ":
			// Toggle Selection
			node := m.visibleNodes[m.cursor]
			newState := !node.Selected
			node.SetSelected(newState) // This handles recursion automatically!
		}
	}
	return m, nil
}

func (m model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	// Render the list string
	var s strings.Builder

	for i, node := range m.visibleNodes {
		// 1. Cursor
		cursor := " "
		if m.cursor == i {
			cursor = ">"
		}

		// 2. Checkbox
		check := "[ ]"
		if node.Selected {
			check = "[x]"
		}

		// 3. Icon (Folder vs File)
		icon := "📄"
		if node.IsDir {
			if node.Expanded {
				icon = "📂"
			} else {
				icon = "📁"
			}
		}

		// 4. Indentation
		indent := strings.Repeat("  ", node.Depth)

		// 5. Build Line
		line := fmt.Sprintf("%s %s %s%s %s", cursor, check, indent, icon, node.Name)

		// Highlight the active line
		if m.cursor == i {
			line = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render(line)
		}

		s.WriteString(line + "\n")
	}

	m.viewport.SetContent(s.String())

	header := "Punjado Tree View (Space: Select, Enter: Open/Close)\n"
	return fmt.Sprintf("%s%s", header, m.viewport.View())
}

func main() {
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
