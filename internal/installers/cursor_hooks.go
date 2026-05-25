package installers

import (
	"encoding/json"
	"fmt"
	"strings"
)

func mergeCursorHooks(existing string, hookCommand string) (string, bool, error) {
	root := map[string]any{}
	if strings.TrimSpace(existing) != "" {
		if err := json.Unmarshal([]byte(existing), &root); err != nil {
			return "", false, fmt.Errorf("parse Cursor hooks.json: %w", err)
		}
	}

	preToolUse, err := sliceField(root, "preToolUse")
	if err != nil {
		return "", false, fmt.Errorf("parse Cursor hooks.json: %w", err)
	}
	if containsClaudeHook(preToolUse, hookCommand) {
		rendered, marshalErr := marshalAgentJSON(root, "Cursor hooks.json")
		return rendered, false, marshalErr
	}

	preToolUse = append(preToolUse, map[string]any{
		"matcher": "Bash",
		"hooks": []any{
			map[string]any{
				"type":    "command",
				"command": hookCommand,
			},
		},
	})
	root["preToolUse"] = preToolUse
	rendered, err := marshalAgentJSON(root, "Cursor hooks.json")
	if err != nil {
		return "", false, err
	}
	return rendered, true, nil
}

func pruneCursorHooks(existing string, hookCommand string) (string, bool, error) {
	if strings.TrimSpace(existing) == "" {
		return "", false, nil
	}
	root := map[string]any{}
	if err := json.Unmarshal([]byte(existing), &root); err != nil {
		return "", false, fmt.Errorf("parse Cursor hooks.json: %w", err)
	}
	preToolUse, ok := root["preToolUse"]
	if !ok {
		return existing, false, nil
	}
	preToolUseSlice, ok := preToolUse.([]any)
	if !ok {
		return "", false, fmt.Errorf("parse Cursor hooks.json: preToolUse must be an array")
	}
	filtered, changed := filterHookEntries(preToolUseSlice, hookCommand)
	if !changed {
		return existing, false, nil
	}
	if len(filtered) == 0 {
		delete(root, "preToolUse")
	} else {
		root["preToolUse"] = filtered
	}
	rendered, err := marshalAgentJSON(root, "Cursor hooks.json")
	if err != nil {
		return "", false, err
	}
	return rendered, true, nil
}
