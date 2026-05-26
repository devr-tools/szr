package rewrite

import (
	"path/filepath"
	"strings"
)

type Decision struct {
	Command       string `json:"command,omitempty"`
	Rewrite       string `json:"rewrite,omitempty"`
	Hint          string `json:"hint,omitempty"`
	Reason        string `json:"reason,omitempty"`
	AutoRewrite   bool   `json:"auto_rewrite"`
	WrapMode      string `json:"wrap_mode,omitempty"`
	ProducerOnly  bool   `json:"producer_only"`
	AlreadyRouted bool   `json:"already_routed"`
}

func Analyze(command, binary string) Decision {
	command = strings.TrimSpace(command)
	decision := Decision{Command: command}
	if command == "" {
		return decision
	}

	producer, suffix := splitProducer(command)
	words, ok := shellSplit(producer)
	if !ok || len(words) == 0 {
		return decision
	}
	name := filepath.Base(words[0])
	if isSZRCommand(name, filepath.Base(binary)) {
		decision.AlreadyRouted = true
		return decision
	}
	if hint := hintForCommand(words, binary); hint != "" {
		decision.Hint = hint
		decision.Reason = "wrapper-guidance"
	}
	if !isSafeAutoRewrite(words) {
		return decision
	}

	if suffix != "" {
		decision.AutoRewrite = true
		decision.ProducerOnly = true
		decision.WrapMode = "proxy"
		decision.Reason = "wrap noisy producer inside shell pipeline"
		decision.Rewrite = binary + " proxy " + producer + suffix
		return decision
	}

	decision.AutoRewrite = true
	decision.WrapMode = "direct"
	decision.Reason = "wrap supported command family"
	decision.Rewrite = binary + " " + command
	return decision
}

func Family(command string) string {
	command = strings.TrimSpace(command)
	if command == "" {
		return ""
	}
	producer, _ := splitProducer(command)
	words, ok := shellSplit(producer)
	if !ok || len(words) == 0 {
		return ""
	}
	name := strings.ToLower(filepath.Base(words[0]))
	if len(words) > 1 {
		second := strings.ToLower(words[1])
		switch name {
		case "git":
			if contains(second, "status", "diff", "log", "show") {
				return name + " " + second
			}
		case "go":
			if contains(second, "test", "build", "vet", "list") {
				return name + " " + second
			}
		}
	}
	switch name {
	case "find", "grep", "rg", "terraform", "tofu", "kubectl", "gh", "eslint", "tsc":
		return name
	}
	return name
}

func hintForCommand(words []string, binary string) string {
	if len(words) == 0 {
		return ""
	}
	switch Family(strings.Join(words, " ")) {
	case "git diff":
		return binary + " git diff ... --stat or " + binary + " proxy git diff ... -- path/to/file | head -200"
	case "git status":
		return binary + " git status or " + binary + " proxy git status --short | head -40"
	case "git log":
		return binary + " git log ... or " + binary + " proxy git log ... | head -80"
	case "git show":
		return binary + " git show ... or " + binary + " proxy git show ... | head -200"
	case "find":
		return binary + " find <path> --name ... or " + binary + " run /usr/bin/find ... when exact find flags matter"
	case "grep":
		return binary + " grep <pattern> <path> or " + binary + " rg <pattern> <path>, or " + binary + " run /usr/bin/grep ... when exact grep flags matter"
	}
	return ""
}

func isSafeAutoRewrite(words []string) bool {
	if len(words) == 0 {
		return false
	}
	name := filepath.Base(words[0])
	switch name {
	case "git":
		return len(words) > 1 && contains(words[1], "status", "diff", "log", "show")
	case "go":
		return len(words) > 1 && contains(words[1], "test", "build", "vet", "list")
	case "npm", "pnpm", "yarn", "bun", "pytest", "cargo", "docker", "kubectl", "gh", "uv", "poetry", "pip", "pip3", "ruff", "mypy", "make", "just", "task", "bazel", "ninja", "cmake", "terraform", "tofu", "helm", "gradle", "mvn", "clang-tidy", "clang-format", "bear", "ctest", "diff", "patch", "tree", "rg":
		return true
	case "python", "python3":
		return len(words) > 2 && words[1] == "-m" && contains(words[2], "pytest", "pip", "ruff", "mypy")
	case "cat":
		return len(words) == 2 && !strings.HasPrefix(words[1], "-")
	default:
		return false
	}
}

func splitProducer(command string) (string, string) {
	var quote rune
	escaped := false
	for i, r := range command {
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			}
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			continue
		}
		if r == '|' || r == ';' {
			return strings.TrimSpace(command[:i]), " " + strings.TrimLeft(command[i:], " ")
		}
		if r == '&' && i+1 < len(command) && command[i+1] == '&' {
			return strings.TrimSpace(command[:i]), " " + strings.TrimLeft(command[i:], " ")
		}
	}
	return strings.TrimSpace(command), ""
}

func shellSplit(input string) ([]string, bool) {
	var out []string
	var current strings.Builder
	var quote rune
	escaped := false
	flush := func() {
		if current.Len() == 0 {
			return
		}
		out = append(out, current.String())
		current.Reset()
	}
	for _, r := range input {
		if escaped {
			current.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' && quote != '\'' {
			escaped = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
				continue
			}
			current.WriteRune(r)
			continue
		}
		switch r {
		case '\'', '"':
			quote = r
		case ' ', '\t', '\n':
			flush()
		default:
			current.WriteRune(r)
		}
	}
	if quote != 0 || escaped {
		return nil, false
	}
	flush()
	return out, true
}

func contains(value string, allowed ...string) bool {
	for _, item := range allowed {
		if value == item {
			return true
		}
	}
	return false
}

func isSZRCommand(name, binaryBase string) bool {
	return name == "szr" || (binaryBase != "" && name == binaryBase)
}
