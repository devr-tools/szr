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
	route, ok := autoRewriteCommand(words)
	if !ok {
		return decision
	}
	if !route.structured {
		route.command = producer
	}

	if suffix != "" {
		if route.structured {
			return decision
		}
		decision.AutoRewrite = true
		decision.ProducerOnly = true
		decision.WrapMode = "proxy"
		decision.Reason = "wrap noisy producer inside shell pipeline"
		decision.Rewrite = binary + " proxy " + route.command + suffix
		return decision
	}

	decision.AutoRewrite = true
	decision.WrapMode = "direct"
	if route.structured {
		decision.Reason = "rewrite supported command into structured szr path"
	} else {
		decision.Reason = "wrap supported command family"
	}
	decision.Rewrite = binary + " " + route.command
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
			if second == "ls-files" {
				return "git ls-files"
			}
		case "go":
			if contains(second, "test", "build", "vet", "list") {
				return name + " " + second
			}
		}
	}
	switch name {
	case "find", "grep", "rg", "fd", "ls", "tree", "terraform", "tofu", "kubectl", "gh", "eslint", "tsc":
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
	case "git ls-files":
		return binary + " git ls-files ... or " + binary + " proxy git ls-files ... | head -200"
	case "find":
		return binary + " find <path> --name ... or " + binary + " run /usr/bin/find ... when exact find flags matter"
	case "grep":
		return binary + " grep <pattern> <path> or " + binary + " rg <pattern> <path>, or " + binary + " run /usr/bin/grep ... when exact grep flags matter"
	case "fd":
		return binary + " fd <pattern> <path> or " + binary + " proxy fd ... | head -200"
	case "ls":
		return binary + " ls [path]"
	case "tree":
		return binary + " ls [path]"
	}
	return ""
}

type routePlan struct {
	command    string
	structured bool
}

func autoRewriteCommand(words []string) (routePlan, bool) {
	if len(words) == 0 {
		return routePlan{}, false
	}
	name := strings.ToLower(filepath.Base(words[0]))
	switch name {
	case "git":
		if len(words) > 1 && (contains(words[1], "status", "diff", "log", "show") || words[1] == "ls-files") {
			return routePlan{}, true
		}
	case "go":
		if len(words) > 1 && contains(words[1], "test", "build", "vet", "list") {
			return routePlan{}, true
		}
	case "find":
		args, ok := rewriteFind(words)
		if ok {
			return routePlan{command: "find " + strings.Join(args, " "), structured: true}, true
		}
	case "grep":
		args, ok := rewriteGrep(words)
		if ok {
			return routePlan{command: "grep " + strings.Join(args, " "), structured: true}, true
		}
	case "ls":
		args, ok := rewriteLS(words)
		if ok {
			return routePlan{command: "ls " + strings.Join(args, " "), structured: true}, true
		}
	case "tree":
		args, ok := rewriteTree(words)
		if ok {
			return routePlan{command: "ls " + strings.Join(args, " "), structured: true}, true
		}
	case "fd":
		if isSafeFD(words) {
			return routePlan{}, true
		}
	case "npm", "pnpm", "yarn", "bun", "pytest", "cargo", "docker", "kubectl", "gh", "uv", "poetry", "pip", "pip3", "ruff", "mypy", "make", "just", "task", "bazel", "ninja", "cmake", "terraform", "tofu", "helm", "gradle", "mvn", "clang-tidy", "clang-format", "bear", "ctest", "diff", "patch", "rg":
		return routePlan{}, true
	case "python", "python3":
		if len(words) > 2 && words[1] == "-m" && contains(words[2], "pytest", "pip", "ruff", "mypy") {
			return routePlan{}, true
		}
	case "cat":
		if len(words) == 2 && !strings.HasPrefix(words[1], "-") {
			return routePlan{}, true
		}
	}
	return routePlan{}, false
}

func rewriteFind(words []string) ([]string, bool) {
	if len(words) == 1 {
		return []string{"."}, true
	}

	root := "."
	rootSet := false
	args := []string{}
	for i := 1; i < len(words); i++ {
		word := words[i]
		switch word {
		case "-name", "--name":
			if i+1 >= len(words) {
				return nil, false
			}
			args = append(args, "--name", words[i+1])
			i++
		case "-path", "--path":
			if i+1 >= len(words) {
				return nil, false
			}
			args = append(args, "--path", words[i+1])
			i++
		case "-type", "--type":
			if i+1 >= len(words) || !contains(words[i+1], "f", "d") {
				return nil, false
			}
			args = append(args, "--type", words[i+1])
			i++
		case "-maxdepth", "--max-depth":
			if i+1 >= len(words) || !isDigits(words[i+1]) {
				return nil, false
			}
			args = append(args, "--max-depth", words[i+1])
			i++
		default:
			if strings.HasPrefix(word, "-") || rootSet {
				return nil, false
			}
			root = word
			rootSet = true
		}
	}

	return append([]string{root}, args...), true
}

func rewriteGrep(words []string) ([]string, bool) {
	allowedFlags := map[string]bool{
		"-r":              true,
		"-R":              true,
		"-n":              true,
		"-H":              true,
		"--recursive":     true,
		"--line-number":   true,
		"--with-filename": true,
	}

	positional := []string{}
	for i := 1; i < len(words); i++ {
		word := words[i]
		if word == "--" {
			positional = append(positional, words[i+1:]...)
			break
		}
		if strings.HasPrefix(word, "-") {
			if !isAllowedGrepFlag(word, allowedFlags) {
				return nil, false
			}
			continue
		}
		positional = append(positional, word)
	}

	if len(positional) == 0 || len(positional) > 2 {
		return nil, false
	}
	pattern := positional[0]
	if !isLiteralSearchPattern(pattern) {
		return nil, false
	}
	searchPath := "."
	if len(positional) == 2 {
		searchPath = positional[1]
	}
	return []string{pattern, searchPath}, true
}

func isAllowedGrepFlag(flag string, allowed map[string]bool) bool {
	if allowed[flag] {
		return true
	}
	if !strings.HasPrefix(flag, "-") || strings.HasPrefix(flag, "--") || len(flag) < 3 {
		return false
	}
	for _, r := range flag[1:] {
		if !allowed["-"+string(r)] {
			return false
		}
	}
	return true
}

func rewriteLS(words []string) ([]string, bool) {
	if len(words) == 1 {
		return []string{"."}, true
	}
	if len(words) == 2 && !strings.HasPrefix(words[1], "-") {
		return []string{words[1]}, true
	}
	return nil, false
}

func rewriteTree(words []string) ([]string, bool) {
	if len(words) == 1 {
		return []string{"."}, true
	}
	if len(words) == 2 && !strings.HasPrefix(words[1], "-") {
		return []string{words[1]}, true
	}
	return nil, false
}

func isSafeFD(words []string) bool {
	for i := 1; i < len(words); i++ {
		switch words[i] {
		case "-x", "-X", "--exec", "--exec-batch":
			return false
		}
	}
	return true
}

func isDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func isLiteralSearchPattern(pattern string) bool {
	if pattern == "" || strings.HasPrefix(pattern, "-") {
		return false
	}
	return !strings.ContainsAny(pattern, `\.^$*+?()[]{}|`)
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
