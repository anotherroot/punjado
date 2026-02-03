package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

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
