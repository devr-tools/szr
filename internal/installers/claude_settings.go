package installers

import (
	"encoding/json"
	"fmt"
	"strings"
)

func mergeClaudeSettings(existing string, hookCommand string) (string, bool, error) {
	root := map[string]any{}
	if strings.TrimSpace(existing) != "" {
		if err := json.Unmarshal([]byte(existing), &root); err != nil {
			return "", false, fmt.Errorf("parse Claude settings.json: %w", err)
		}
	}

	hooks, err := mapField(root, "hooks")
	if err != nil {
		return "", false, err
	}
	preToolUse, err := sliceField(hooks, "PreToolUse")
	if err != nil {
		return "", false, err
	}
	if containsClaudeHook(preToolUse, hookCommand) {
		rendered, marshalErr := marshalClaudeSettings(root)
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
	hooks["PreToolUse"] = preToolUse
	root["hooks"] = hooks

	rendered, err := marshalClaudeSettings(root)
	if err != nil {
		return "", false, err
	}
	return rendered, true, nil
}

func pruneClaudeSettings(existing string, hookCommand string) (string, bool, error) {
	if strings.TrimSpace(existing) == "" {
		return "", false, nil
	}

	root := map[string]any{}
	if err := json.Unmarshal([]byte(existing), &root); err != nil {
		return "", false, fmt.Errorf("parse Claude settings.json: %w", err)
	}

	hooks, ok := root["hooks"]
	if !ok {
		return existing, false, nil
	}
	hooksMap, ok := hooks.(map[string]any)
	if !ok {
		return "", false, fmt.Errorf("parse Claude settings.json: hooks must be an object")
	}

	preToolUse, ok := hooksMap["PreToolUse"]
	if !ok {
		return existing, false, nil
	}
	preToolUseSlice, ok := preToolUse.([]any)
	if !ok {
		return "", false, fmt.Errorf("parse Claude settings.json: hooks.PreToolUse must be an array")
	}

	filtered := make([]any, 0, len(preToolUseSlice))
	changed := false
	for _, entry := range preToolUseSlice {
		entryMap, ok := entry.(map[string]any)
		if !ok {
			filtered = append(filtered, entry)
			continue
		}
		if entryContainsClaudeHook(entryMap, hookCommand) {
			changed = true
			continue
		}
		filtered = append(filtered, entry)
	}
	if !changed {
		return existing, false, nil
	}

	if len(filtered) == 0 {
		delete(hooksMap, "PreToolUse")
	} else {
		hooksMap["PreToolUse"] = filtered
	}
	if len(hooksMap) == 0 {
		delete(root, "hooks")
	} else {
		root["hooks"] = hooksMap
	}

	rendered, err := marshalClaudeSettings(root)
	if err != nil {
		return "", false, err
	}
	return rendered, true, nil
}

func mapField(root map[string]any, key string) (map[string]any, error) {
	value, ok := root[key]
	if !ok {
		return map[string]any{}, nil
	}
	asMap, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("parse Claude settings.json: %s must be an object", key)
	}
	return asMap, nil
}

func sliceField(root map[string]any, key string) ([]any, error) {
	value, ok := root[key]
	if !ok {
		return []any{}, nil
	}
	asSlice, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("parse Claude settings.json: %s must be an array", key)
	}
	return asSlice, nil
}

func containsClaudeHook(entries []any, hookCommand string) bool {
	for _, entry := range entries {
		entryMap, ok := entry.(map[string]any)
		if ok && entryContainsClaudeHook(entryMap, hookCommand) {
			return true
		}
	}
	return false
}

func entryContainsClaudeHook(entry map[string]any, hookCommand string) bool {
	value, ok := entry["hooks"]
	if !ok {
		return false
	}
	hooks, ok := value.([]any)
	if !ok {
		return false
	}
	for _, hook := range hooks {
		hookMap, ok := hook.(map[string]any)
		if !ok {
			continue
		}
		command, _ := hookMap["command"].(string)
		if command == hookCommand {
			return true
		}
	}
	return false
}

func marshalClaudeSettings(root map[string]any) (string, error) {
	return marshalAgentJSON(root, "Claude settings.json")
}

func marshalAgentJSON(root map[string]any, label string) (string, error) {
	data, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return "", fmt.Errorf("serialize %s: %w", label, err)
	}
	return string(data) + "\n", nil
}

func filterHookEntries(entries []any, hookCommand string) ([]any, bool) {
	filtered := make([]any, 0, len(entries))
	changed := false
	for _, entry := range entries {
		entryMap, ok := entry.(map[string]any)
		if !ok {
			filtered = append(filtered, entry)
			continue
		}
		if entryContainsClaudeHook(entryMap, hookCommand) {
			changed = true
			continue
		}
		filtered = append(filtered, entry)
	}
	return filtered, changed
}
