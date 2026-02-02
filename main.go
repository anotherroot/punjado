package main

import (
	"fmt"
	"os"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// 1. DATA STRUCTURES
// We define a custom struct for our items
type item struct {
	title    string
	desc     string // We can show file size here later
	selected bool
}

// These three methods make our struct satisfy the list.Item interface
func (i item) Title() string {
	// The Trick: We change the visual title based on state
	if i.selected {
		return i.title
	}
	return i.title
}
func (i item) Description() string { return i.desc }
func (i item) FilterValue() string { return i.title } // What can we search for?

// 2. MODEL
type model struct {
	list     list.Model
	selected map[string]struct{} // Keep track of filenames
}

func initialModel() model {
	// Create some dummy files
	files := []string{"README.md", "main.go", "flake.nix", "src/utils.go", "go.mod", "go.sum", ".gitignore"}

	// Convert strings to our 'item' struct
	items := []list.Item{}
	for _, file := range files {
		items = append(items, item{title: file, desc: "File"})
	}

	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = false
	delegate.SetSpacing(0)
	l := list.New(items, delegate, 0, 0)
	l.Title = "Punjado File Picker"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	// l.DisableQuitKeybindings() // Optional: if you want to handle q yourself logic fully

	return model{
		list:     l,
		selected: make(map[string]struct{}),
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

// 3. UPDATE
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	// Handle Window Resizing (Crucial for the list to look right)
	case tea.WindowSizeMsg:
		h, v := docStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)

	// Handle Keys
	case tea.KeyMsg:
		// Don't intercept keys if the user is typing in the filter box
		if m.list.FilterState() == list.Filtering {
			break
		}

		switch msg.String() {
		case "space":
			// Get the item the cursor is on
			selectedItem, ok := m.list.SelectedItem().(item)
			if ok {
				// Toggle the selection in our map
				// (We use the title as the key for now)
				fileName := selectedItem.title
				if _, exists := m.selected[fileName]; exists {
					delete(m.selected, fileName)
					selectedItem.selected = false
				} else {
					m.selected[fileName] = struct{}{}
					selectedItem.selected = true
				}

				// Update the item in the list so the title changes from [ ] to [x]
				// We need to find the index of the item we just modified
				index := m.list.Index()
				m.list.SetItem(index, selectedItem)
			}
			return m, nil
		}
	}

	// Forward all other events to the list component (scrolling, filtering, etc.)
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// 4. VIEW
var docStyle = lipgloss.NewStyle().Margin(1, 2)

func (m model) View() string {
	return docStyle.Render(m.list.View())
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v", err)
		os.Exit(1)
	}
}
