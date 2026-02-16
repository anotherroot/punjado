package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/atotto/clipboard"
)

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

		path := filepath.Join(dir, clean)
		if !FileExists(path) {
			fmt.Printf("Error: File '%s' doesn't exist in directory '%s'", f, dir)
			os.Exit(1)
		}

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
		content, err := os.ReadFile(filepath.Join(dir, path))
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

func HandleStatus(params []string, flags map[string]string) {
	VarifyFlags(flags, []string{"help", "dir"})

	if HasFlag(flags, "help") {
		fmt.Printf("Help for toggle....")
		return
	}

	dir := GetFlag(flags, "dir", ".")

	if len(params) < 1 {
		fmt.Println("Usage: punjado status <file>")
		return
	}

	config := readConfig(dir)
	exists := false
	for file := range config {
		if file == params[0] {
			exists = true
			break
		}
	}
	if exists {
		fmt.Println(1)
	} else {
		fmt.Println(0)
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
