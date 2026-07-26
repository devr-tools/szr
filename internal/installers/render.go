package installers

import (
	"fmt"
	"os"
)

func Render(target Target, options Options) (Plan, error) {
	paths, err := detectRenderPaths(target, options)
	if err != nil {
		return Plan{}, err
	}
	if options.Binary != "" {
		paths.Binary = options.Binary
	}

	switch target {
	case TargetCodex:
		return renderCodex(paths), nil
	case TargetClaude:
		return renderClaude(paths), nil
	case TargetCursor:
		return renderCursor(paths), nil
	case TargetGemini:
		return renderGemini(paths), nil
	case TargetShell:
		return renderShell(paths), nil
	default:
		return Plan{}, fmt.Errorf("unknown target %q", target)
	}
}

func RenderAll(options Options) ([]Plan, error) {
	targets := Targets()
	plans := make([]Plan, 0, len(targets))
	for _, target := range targets {
		plan, err := Render(target, options)
		if err != nil {
			return nil, err
		}
		plans = append(plans, plan)
	}
	return plans, nil
}

func detectRenderPaths(target Target, options Options) (Paths, error) {
	homeDir := options.HomeDir
	if homeDir == "" {
		var err error
		homeDir, err = os.UserHomeDir()
		if err != nil {
			return Paths{}, err
		}
	}

	switch target {
	case TargetClaude, TargetCursor, TargetGemini:
		return DetectClaudeGlobalPaths(homeDir)
	case TargetCodex:
		if options.Global {
			return DetectClaudeGlobalPaths(homeDir)
		}
		repoPaths, err := DetectPaths(options.RepoRoot)
		if err != nil {
			return Paths{}, err
		}
		globalPaths, err := DetectClaudeGlobalPaths(homeDir)
		if err != nil {
			return Paths{}, err
		}
		repoPaths.CodexDir = globalPaths.CodexDir
		repoPaths.CodexSZRFile = globalPaths.CodexSZRFile
		return repoPaths, nil
	default:
		return DetectPaths(options.RepoRoot)
	}
}
