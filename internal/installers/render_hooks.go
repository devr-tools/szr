package installers

import (
	"fmt"
	"path/filepath"
	"strings"
)

func sharedHookFile(paths Paths) File {
	return File{
		Path:        paths.HookFile,
		Content:     renderSharedHookScript(paths.HookFile, paths.Binary),
		Mode:        0o755,
		Strategy:    StrategyWrite,
		Description: "pre-command reminder hook",
	}
}

func renderHookScript(binary string) string {
	return fmt.Sprintf(`#!/bin/sh
set -eu

if [ "$#" -eq 0 ]; then
  exit 0
fi

if [ "$1" = "szr" ]; then
  exit 0
fi

hint=$(%s rewrite --binary %q --format hint --command "$*" 2>/dev/null || true)
if [ -n "$hint" ]; then
  printf 'szr hint: prefer %%s\n' "$hint" >&2
fi
`, binary, binary)
}

func renderSharedHookScript(path, binary string) string {
	switch filepath.Base(path) {
	case "szr-rewrite.sh":
		if strings.Contains(filepath.ToSlash(path), "/.cursor/") {
			return renderCursorHookScript(binary)
		}
		if strings.Contains(filepath.ToSlash(path), "/.gemini/") {
			return renderGeminiHookScript(binary)
		}
		return renderClaudeGlobalHookScript(binary)
	default:
		return renderHookScript(binary)
	}
}

func renderClaudeGlobalHookScript(binary string) string {
	return fmt.Sprintf(`#!/bin/sh
set -eu

%s rewrite --binary %q --hook claude || true
`, binary, binary)
}

func renderCursorHookScript(binary string) string {
	return fmt.Sprintf(`#!/bin/sh
set -eu

%s rewrite --binary %q --hook cursor || printf '{}'
`, binary, binary)
}

func renderGeminiHookScript(binary string) string {
	return fmt.Sprintf(`#!/bin/sh
set -eu

%s rewrite --binary %q --hook gemini || printf '{"decision":"allow"}'
`, binary, binary)
}
