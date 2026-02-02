package main

// logging
import (
	// "github.com/davecgh/go-spew/spew"
	"log"
)

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
)

// --- 1. DATA STRUCTURES ---

type FileNode struct {
	Name     string
	Path     string
	IsDir    bool
	Size     int64
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
			info, _ := d.Info()
			var size int64
			if info != nil {
				size = info.Size()
			}
			node := &FileNode{
				Name:     d.Name(),
				Path:     path,
				Size:     size,
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

	log.Println("flattenVisible len: ", len(result))
	names := []string{}
	for _, n := range result {
		names = append(names, n.Name)
	}
	log.Printf("Visible List: %v", names)
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

func (m model) renderContent() string {
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

		// 3. Icon
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
		line := fmt.Sprintf("%s%s%s %s %s", cursor, indent, check, icon, node.Name)

		// Highlight
		if m.cursor == i {
			line = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render(line)
		}

		s.WriteString(line + "\n")
	}
	return s.String()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {

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

		case " ":
			node := m.visibleNodes[m.cursor]
			if node.IsDir {
				node.ToggleExpand()
				m.visibleNodes = flattenVisible(m.root)
			}

		case "a": 
			allNodesSelected := true
			for _, node := range m.visibleNodes {
				if node.Selected != true{
					allNodesSelected = false;
					break;
				}
			}

			
			for _, node := range m.visibleNodes {
				if allNodesSelected{
				node.SetSelected(false)
				} else{
				node.SetSelected(true)
				}
			}

		case "s", "enter": // Added enter for selection too if you like
			node := m.visibleNodes[m.cursor]
			newState := !node.Selected
			node.SetSelected(newState)
		}
	}

	// CRITICAL FIX: Set the content inside Update
	// This saves the "lines" into the viewport model that gets returned
	m.viewport.SetContent(m.renderContent())

	return m, nil
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
    if w < 0 { w = 0 }
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
        key("a", "toggle all")
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
