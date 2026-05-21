package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"szr/internal/config"
	"szr/internal/filters"
)

func (a *App) runLS(args []string) int {
	root := "."
	if len(args) > 0 {
		root = args[0]
	}

	var paths []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		paths = append(paths, path)
		if info.IsDir() && len(strings.Split(filepath.Clean(path), string(filepath.Separator)))-len(strings.Split(filepath.Clean(root), string(filepath.Separator))) > 2 {
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "szr: %v\n", err)
		return 1
	}

	fmt.Println(filters.BuildTree(paths, root))
	return 0
}

func (a *App) runRead(cfg config.Config, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "szr: read requires a file")
		return 2
	}

	level := "none"
	lineNumbers := false
	maxLines := adjustCountForReasoningMode(cfg.ReasoningBudgetMode, cfg.MaxPreviewLines)
	files := []string{}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-l", "--level":
			i++
			if i >= len(args) {
				fmt.Fprintln(os.Stderr, "szr: missing value for --level")
				return 2
			}
			level = args[i]
		case "-n", "--line-numbers":
			lineNumbers = true
		case "--max-lines":
			i++
			if i >= len(args) {
				fmt.Fprintln(os.Stderr, "szr: missing value for --max-lines")
				return 2
			}
			fmt.Sscanf(args[i], "%d", &maxLines)
		default:
			files = append(files, args[i])
		}
	}

	for idx, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "szr: %v\n", err)
			return 1
		}
		if idx > 0 {
			fmt.Println()
		}
		if len(files) > 1 {
			fmt.Printf("== %s ==\n", file)
		}
		fmt.Println(filters.ReadLevel(data, level, lineNumbers, maxLines))
	}
	return 0
}

func (a *App) runGrep(cfg config.Config, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "szr: grep requires a pattern")
		return 2
	}

	pattern := args[0]
	searchPath := "."
	extra := []string{}
	if len(args) > 1 {
		searchPath = args[1]
		if len(args) > 2 {
			extra = args[2:]
		}
	}

	rgArgs := append([]string{"-n", "--no-heading", pattern, searchPath}, extra...)
	cmd := exec.Command("rg", rgArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			fmt.Fprintf(os.Stderr, "szr: %v\n", err)
			return 1
		}
		if exitErr.ExitCode() > 1 {
			fmt.Fprintln(os.Stderr, string(output))
			return exitErr.ExitCode()
		}
	}

	fmt.Println(filters.GroupRipgrep(string(output), adjustCountForReasoningMode(cfg.ReasoningBudgetMode, cfg.MaxMatchGroups)))
	return 0
}

func adjustCountForReasoningMode(mode string, value int) int {
	if value <= 0 {
		return value
	}
	switch mode {
	case config.ReasoningBudgetAgent:
		scaled := (value*3 + 3) / 4
		if scaled < 4 {
			return 4
		}
		return scaled
	default:
		return value
	}
}

func (a *App) runJSON(args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "szr: json requires a file")
		return 2
	}

	data, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "szr: %v\n", err)
		return 1
	}
	fmt.Println(filters.RenderJSONStructure(data))
	return 0
}

func (a *App) runLog(args []string) int {
	var data []byte
	var err error
	if len(args) == 0 {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(args[0])
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "szr: %v\n", err)
		return 1
	}

	fmt.Println(filters.ScannerDedupe(data))
	return 0
}
