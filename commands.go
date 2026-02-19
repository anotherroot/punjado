package main

import (
)


type Action struct {
	Undo func()
	Redo func()
}

func (m model) commit(action Action) model {
	action.Redo() 
	
	m.undoStack = append(m.undoStack, action)
	m.redoStack = nil 
	
	saveState(m.root, m.root.Path)
	return m
}

func (m model) performUndo() model {
	if len(m.undoStack) == 0 { return m }

	lastIdx := len(m.undoStack) - 1
	action := m.undoStack[lastIdx]
	m.undoStack = m.undoStack[:lastIdx]

	action.Undo()
	m.redoStack = append(m.redoStack, action)
	
	saveState(m.root, m.root.Path)
	return m
}

func (m model) performRedo() model {
	if len(m.redoStack) == 0 { return m }

	lastIdx := len(m.redoStack) - 1
	action := m.redoStack[lastIdx]
	m.redoStack = m.redoStack[:lastIdx]

	action.Redo()
	m.undoStack = append(m.undoStack, action)
	
	saveState(m.root, m.root.Path) 
	return m
}

func (m model) toggleCurrentFile() model {
	node := m.visibleNodes[m.cursor]
	if node.IsBinary {
		return m 
	}

	prevState := node.Selected
	newState := !prevState

	action := Action{
		Undo: func() { node.SetSelected(prevState) },
		Redo: func() { node.SetSelected(newState) },
	}

	return m.commit(action)
}

func (m model) toggleAllFiles() model {
	allSelected := true
	for _, node := range m.visibleNodes {
		emptyDir := node.IsDir && len(node.Children) == 0
		if node.IsBinary || emptyDir { continue }
		if !node.Selected {
			allSelected = false
			break
		}
	}
	targetState := !allSelected

	prevStates := make(map[*FileNode]bool)
	for _, node := range m.visibleNodes {
		prevStates[node] = node.Selected
	}

	action := Action{
		Undo: func() {
			for node, oldState := range prevStates {
				node.SetSelected(oldState)
			}
		},
		Redo: func() {
			for node := range prevStates {
				emptyDir := node.IsDir && len(node.Children) == 0
				if node.IsBinary || emptyDir { continue }
				node.SetSelected(targetState)
			}
		},
	}

	return m.commit(action)
}

func (m model) moveUp() model {
	if m.cursor > 0 {
		m.cursor--
	}
	if m.cursor < m.viewport.YOffset {
		m.viewport.SetYOffset(m.cursor)
	}
	return m
}

func (m model) moveDown() model {
	if m.cursor < len(m.visibleNodes)-1 {
		m.cursor++
	}
	if m.cursor >= m.viewport.YOffset+m.viewport.Height {
		m.viewport.SetYOffset(m.viewport.YOffset + 1)
	}
	return m
}

func (m model) pageUp() model {
	m.viewport.PageUp()
	m.cursor = max(m.cursor-m.viewport.Height, 0)
	return m
}

func (m model) pageDown() model {
	m.viewport.PageDown()
	m.cursor = min(m.cursor+m.viewport.Height, len(m.visibleNodes)-1)
	return m
}

func (m model) gotoTop() model {
	m.cursor = 0
	m.viewport.GotoTop()
	return m
}

func (m model) gotoBottom() model {
	m.cursor = len(m.visibleNodes) - 1
	m.viewport.GotoBottom()
	return m
}

func (m model) toggleHelp() model {
	m.helpMode = !m.helpMode
	headerHeight := 1
	footerHeight := 1
	if m.helpMode {
		footerHeight = 10
	}
	m.viewport.Height = m.height - headerHeight - footerHeight
	return m
}

func (m model) closeHelp() model {
	if m.helpMode {
		m.helpMode = false
		headerHeight := 1
		footerHeight := 2
		m.viewport.Height = m.height - headerHeight - footerHeight
	}
	return m
}

func (m model) toggleDirectory() model {
	node := m.visibleNodes[m.cursor]
	if node.IsDir {
		node.ToggleExpand()
		m.visibleNodes = flattenVisible(m.root)
	}
	return m
}

func (m model) toggleExpandAll() model {
	allNodesExpanded := true
	for _, node := range m.visibleNodes {
		if !node.Expanded && node.IsDir {
			allNodesExpanded = false
			break
		}
	}

	for _, node := range m.visibleNodes {
		if node.IsDir {
			node.Expanded = !allNodesExpanded
		}
	}
	m.visibleNodes = flattenVisible(m.root)
	return m
}
