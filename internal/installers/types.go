package installers

import "os"

type Target string

const (
	TargetCodex  Target = "codex"
	TargetClaude Target = "claude-code"
	TargetCursor Target = "cursor"
	TargetGemini Target = "gemini"
)

type Strategy string

const (
	StrategyWrite Strategy = "write"
	StrategyMerge Strategy = "merge"
)

type Options struct {
	RepoRoot string
	Binary   string
}

type Paths struct {
	RepoRoot      string
	Binary        string
	HookDir       string
	HookFile      string
	InstallDir    string
	CursorRuleDir string
}

type File struct {
	Path        string
	Content     string
	Mode        os.FileMode
	Strategy    Strategy
	Marker      string
	Description string
}

type Plan struct {
	Target      Target
	Title       string
	Files       []File
	ManualSteps []string
	Paths       Paths
}

func Targets() []Target {
	return []Target{
		TargetCodex,
		TargetClaude,
		TargetCursor,
		TargetGemini,
	}
}
