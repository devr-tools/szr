package installers

import "os"

type Target string

const (
	TargetCodex  Target = "codex"
	TargetClaude Target = "claude-code"
	TargetCursor Target = "cursor"
	TargetGemini Target = "gemini"
	TargetShell  Target = "shell"
)

type Strategy string

const (
	StrategyWrite               Strategy = "write"
	StrategyMerge               Strategy = "merge"
	StrategyDelete              Strategy = "delete"
	StrategyUnmerge             Strategy = "unmerge"
	StrategyClaudeSettingsMerge Strategy = "claude-settings-merge"
	StrategyClaudeSettingsPrune Strategy = "claude-settings-prune"
	StrategyCursorHooksMerge    Strategy = "cursor-hooks-merge"
	StrategyCursorHooksPrune    Strategy = "cursor-hooks-prune"
	StrategyGeminiSettingsMerge Strategy = "gemini-settings-merge"
	StrategyGeminiSettingsPrune Strategy = "gemini-settings-prune"
)

type Options struct {
	RepoRoot string
	Binary   string
	HomeDir  string
	Global   bool
}

type Paths struct {
	RepoRoot       string
	Binary         string
	HookDir        string
	HookFile       string
	InstallDir     string
	CursorRuleDir  string
	ClaudeDir      string
	ClaudeMDFile   string
	ClaudeSZRFile  string
	ClaudeConfig   string
	CodexDir       string
	CodexSZRFile   string
	CursorDir      string
	CursorConfig   string
	CursorHookDir  string
	CursorHookFile string
	GeminiDir      string
	GeminiConfig   string
	GeminiHookDir  string
	GeminiHookFile string
	Global         bool
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
		TargetShell,
	}
}
