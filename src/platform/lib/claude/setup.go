package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// RegisterHooks registers roost hooks in Claude's settings.json.
// Returns the list of registered event names.
func RegisterHooks(settingsPath, roostBinary string) ([]string, error) {
	settings, err := readSettings(settingsPath)
	if err != nil {
		return nil, err
	}

	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = make(map[string]any)
	}

	events := []string{
		"SessionStart",
		"SessionEnd",
		"PreToolUse",
		"PostToolUse",
		"PostToolUseFailure",
		"Stop",
		"StopFailure",
		"UserPromptSubmit",
		"PreCompact",
		"PostCompact",
		"Notification",
		"SubagentStart",
		"SubagentStop",
		"TaskCreated",
		"TaskCompleted",
	}
	registered := []string{}
	command := roostBinary + " event claude"

	for _, event := range events {
		if addHookEntry(hooks, event, command) {
			registered = append(registered, event)
		}
	}

	if len(registered) == 0 {
		return nil, nil
	}

	settings["hooks"] = hooks
	return registered, writeSettings(settingsPath, settings)
}

// addHookEntry upserts a roost hook entry for the given event.
// If an existing entry already contains the exact command, it returns false (no-op).
// If an existing entry contains a stale roost command (ends with " event claude"),
// it is replaced in-place. Otherwise a new entry is appended.
func addHookEntry(hooks map[string]any, event, command string) bool {
	entries, _ := hooks[event].([]any)

	for i, e := range entries {
		if hasCommand(e, command) {
			return false // already up to date
		}
		if hasRoostEventCommand(e) {
			// Replace stale roost hook entry with the current command.
			entries[i] = map[string]any{
				"hooks": []any{
					map[string]any{"type": "command", "command": command},
				},
			}
			hooks[event] = entries
			return true
		}
	}

	hooks[event] = append(entries, map[string]any{
		"hooks": []any{
			map[string]any{"type": "command", "command": command},
		},
	})
	return true
}

func hasCommand(entry any, command string) bool {
	m, ok := entry.(map[string]any)
	if !ok {
		return false
	}
	hooksArr, _ := m["hooks"].([]any)
	for _, h := range hooksArr {
		hm, ok := h.(map[string]any)
		if !ok {
			continue
		}
		if hm["command"] == command {
			return true
		}
	}
	return false
}

// hasRoostEventCommand reports whether entry contains a hook command that ends
// with " event claude", identifying it as a (possibly stale) roost hook.
func hasRoostEventCommand(entry any) bool {
	m, ok := entry.(map[string]any)
	if !ok {
		return false
	}
	hooksArr, _ := m["hooks"].([]any)
	for _, h := range hooksArr {
		hm, ok := h.(map[string]any)
		if !ok {
			continue
		}
		cmd, _ := hm["command"].(string)
		if strings.HasSuffix(cmd, " event claude") {
			return true
		}
	}
	return false
}

// RegisterMCPServer writes mcpServers.roost-peers to settings.json.
// Returns true if the entry was newly written, false if already present.
func RegisterMCPServer(settingsPath, roostBinary string) (bool, error) {
	settings, err := readSettings(settingsPath)
	if err != nil {
		return false, err
	}
	mcpServers, _ := settings["mcpServers"].(map[string]any)
	if mcpServers == nil {
		mcpServers = make(map[string]any)
	}
	if _, exists := mcpServers["roost-peers"]; exists {
		return false, nil
	}
	mcpServers["roost-peers"] = map[string]any{
		"command": roostBinary,
		"args":    []any{"peers-mcp"},
	}
	settings["mcpServers"] = mcpServers
	return true, writeSettings(settingsPath, settings)
}

func readSettings(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return make(map[string]any), nil
	}
	if err != nil {
		return nil, err
	}
	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, err
	}
	return settings, nil
}

func writeSettings(path string, settings map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
