package main

import (
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
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
// TODO: maybe move into utils?

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
