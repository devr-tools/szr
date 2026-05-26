package localcmd

import (
	"fmt"
	"io"
	"os"

	"github.com/devr-tools/szr/internal/config"
	"github.com/devr-tools/szr/internal/filters"
)

func RunLS(rt Runtime, args []string) int {
	root := "."
	if len(args) > 0 {
		root = args[0]
	}

	var paths []string
	err := filepathWalk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		paths = append(paths, path)
		if info.IsDir() && pathDepth(path)-pathDepth(root) > 2 {
			return skipDir()
		}
		return nil
	})
	if err != nil {
		fmt.Fprintf(rt.Stderr, "szr: %v\n", err)
		return 1
	}

	fmt.Fprintln(rt.Stdout, filters.BuildTree(paths, root))
	return 0
}

func RunRead(rt Runtime, cfg config.Config, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(rt.Stderr, "szr: read requires a file")
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
				fmt.Fprintln(rt.Stderr, "szr: missing value for --level")
				return 2
			}
			level = args[i]
		case "-n", "--line-numbers":
			lineNumbers = true
		case "--max-lines":
			i++
			if i >= len(args) {
				fmt.Fprintln(rt.Stderr, "szr: missing value for --max-lines")
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
			fmt.Fprintf(rt.Stderr, "szr: %v\n", err)
			return 1
		}
		if idx > 0 {
			fmt.Fprintln(rt.Stdout)
		}
		if len(files) > 1 {
			fmt.Fprintf(rt.Stdout, "== %s ==\n", file)
		}
		fmt.Fprintln(rt.Stdout, filters.ReadLevel(data, level, lineNumbers, maxLines))
	}
	return 0
}

func RunJSON(rt Runtime, args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(rt.Stderr, "szr: json requires a file")
		return 2
	}

	data, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Fprintf(rt.Stderr, "szr: %v\n", err)
		return 1
	}
	fmt.Fprintln(rt.Stdout, filters.RenderJSONStructure(data))
	return 0
}

func RunLog(rt Runtime, args []string) int {
	var data []byte
	var err error
	if len(args) == 0 {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(args[0])
	}
	if err != nil {
		fmt.Fprintf(rt.Stderr, "szr: %v\n", err)
		return 1
	}

	fmt.Fprintln(rt.Stdout, filters.ScannerDedupe(data))
	return 0
}
