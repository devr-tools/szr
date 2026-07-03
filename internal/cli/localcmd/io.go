package localcmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/devr-tools/szr/internal/config"
	"github.com/devr-tools/szr/internal/filters"
)

// RunLS renders the builtin directory tree for argv shapes the builtin
// supports and reports an error otherwise.
//
// Deprecated: use TryRunLS, which reports whether the builtin handled the
// command so the caller can delegate unsupported argv to the native binary.
func RunLS(rt Runtime, args []string) int {
	code, handled := TryRunLS(rt, args)
	if !handled {
		fmt.Fprintln(rt.Stderr, "szr: ls: unsupported arguments for builtin execution")
		return 2
	}
	return code
}

// TryRunLS renders the builtin directory tree. The second return value reports
// whether the builtin handled the command; when it is false the caller must
// delegate the original argv to the native ls binary (e.g. `ls -la`).
func TryRunLS(rt Runtime, args []string) (int, bool) {
	if !lsBuiltinSupports(args) {
		return 0, false
	}

	root := "."
	if len(args) > 0 {
		root = args[0]
	}

	paths, err := collectTreePaths(root)
	if err != nil {
		fmt.Fprintf(rt.Stderr, "szr: %v\n", err)
		return 1, true
	}

	fmt.Fprintln(rt.Stdout, filters.BuildTree(paths, root))
	return 0, true
}

func collectTreePaths(root string) ([]string, error) {
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
	return paths, err
}

// lsBuiltinSupports reports whether the builtin tree view fully understands
// the argv: at most one path and no flags.
func lsBuiltinSupports(args []string) bool {
	if len(args) > 1 {
		return false
	}
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			return false
		}
	}
	return true
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
	mode := filters.JSONModeStructure
	maxLines := 8
	files := []string{}

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-m", "--mode":
			i++
			if i >= len(args) {
				fmt.Fprintln(rt.Stderr, "szr: missing value for --mode")
				return 2
			}
			mode = args[i]
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

	if mode != filters.JSONModeStructure && mode != filters.JSONModePreview {
		fmt.Fprintf(rt.Stderr, "szr: unsupported json mode %q\n", mode)
		return 2
	}

	if len(files) != 1 {
		fmt.Fprintln(rt.Stderr, "szr: json requires a file")
		return 2
	}

	data, err := os.ReadFile(files[0])
	if err != nil {
		fmt.Fprintf(rt.Stderr, "szr: %v\n", err)
		return 1
	}
	fmt.Fprintln(rt.Stdout, filters.RenderJSON(data, mode, maxLines))
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
