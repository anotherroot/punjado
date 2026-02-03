package main

import (
	"fmt"
	"os"
)

func main() {

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

	case "status":
		HandleStatus(params, flags)

	default:
		fmt.Printf("Command '%s' not recognised..\n", subcommand)
	}
}
