package installers

import (
	"encoding/json"
	"fmt"
	"strings"
)

func mergeGeminiSettings(existing string, hookCommand string) (string, bool, error) {
	root := map[string]any{}
	if strings.TrimSpace(existing) != "" {
		if err := json.Unmarshal([]byte(existing), &root); err != nil {
			return "", false, fmt.Errorf("parse Gemini settings.json: %w", err)
		}
	}

	hooks, err := mapField(root, "hooks")
	if err != nil {
		return "", false, fmt.Errorf("parse Gemini settings.json: %w", err)
	}
	beforeTool, err := sliceField(hooks, "BeforeTool")
	if err != nil {
		return "", false, fmt.Errorf("parse Gemini settings.json: %w", err)
	}
	if containsClaudeHook(beforeTool, hookCommand) {
		rendered, marshalErr := marshalAgentJSON(root, "Gemini settings.json")
		return rendered, false, marshalErr
	}

	beforeTool = append(beforeTool, map[string]any{
		"matcher": "run_shell_command",
		"hooks": []any{
			map[string]any{
				"type":    "command",
				"command": hookCommand,
			},
		},
	})
	hooks["BeforeTool"] = beforeTool
	root["hooks"] = hooks

	rendered, err := marshalAgentJSON(root, "Gemini settings.json")
	if err != nil {
		return "", false, err
	}
	return rendered, true, nil
}

func pruneGeminiSettings(existing string, hookCommand string) (string, bool, error) {
	if strings.TrimSpace(existing) == "" {
		return "", false, nil
	}
	root := map[string]any{}
	if err := json.Unmarshal([]byte(existing), &root); err != nil {
		return "", false, fmt.Errorf("parse Gemini settings.json: %w", err)
	}
	hooks, ok := root["hooks"]
	if !ok {
		return existing, false, nil
	}
	hooksMap, ok := hooks.(map[string]any)
	if !ok {
		return "", false, fmt.Errorf("parse Gemini settings.json: hooks must be an object")
	}
	beforeTool, ok := hooksMap["BeforeTool"]
	if !ok {
		return existing, false, nil
	}
	beforeToolSlice, ok := beforeTool.([]any)
	if !ok {
		return "", false, fmt.Errorf("parse Gemini settings.json: hooks.BeforeTool must be an array")
	}
	filtered, changed := filterHookEntries(beforeToolSlice, hookCommand)
	if !changed {
		return existing, false, nil
	}
	if len(filtered) == 0 {
		delete(hooksMap, "BeforeTool")
	} else {
		hooksMap["BeforeTool"] = filtered
	}
	if len(hooksMap) == 0 {
		delete(root, "hooks")
	} else {
		root["hooks"] = hooksMap
	}
	rendered, err := marshalAgentJSON(root, "Gemini settings.json")
	if err != nil {
		return "", false, err
	}
	return rendered, true, nil
}
