package rules

type Format string

const (
	FormatUnknown Format = ""
	FormatJSON    Format = "json"
	FormatYAML    Format = "yaml"
)

type File struct {
	Version  int       `json:"version,omitempty"`
	Profiles []Profile `json:"profiles"`
}

type Profile struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Explain     []string `json:"explain,omitempty"`
	Match       Match    `json:"match"`
	Rewrite     Rewrite  `json:"rewrite,omitempty"`
	Render      Render   `json:"render,omitempty"`
}

type Match struct {
	CommandPrefix []string `json:"command_prefix,omitempty"`
	DisplayPrefix []string `json:"display_prefix,omitempty"`
	AllArgs       []string `json:"all_args,omitempty"`
	AnyArgs       []string `json:"any_args,omitempty"`
	ExcludeArgs   []string `json:"exclude_args,omitempty"`
	CwdContains   []string `json:"cwd_contains,omitempty"`
}

type Rewrite struct {
	Mode         string   `json:"mode,omitempty"`
	Args         []string `json:"args,omitempty"`
	SkipIfHasAny []string `json:"skip_if_has_any,omitempty"`
}

type Render struct {
	Mode     string `json:"mode,omitempty"`
	MaxLines int    `json:"max_lines,omitempty"`
}
