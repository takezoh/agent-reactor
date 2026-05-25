package gemini

import (
	"fmt"
	"os"
	"path/filepath"
)

// Run dispatches Gemini subcommands.
func Run(args []string) error {
	if len(args) == 0 {
		printHelp()
		return fmt.Errorf("gemini: missing subcommand")
	}
	switch args[0] {
	case "setup":
		return RunSetup()
	case "help", "-h", "--help":
		printHelp()
		return nil
	default:
		fmt.Fprintf(os.Stderr, "roost gemini: unknown subcommand: %s\n", args[0])
		printHelp()
		return fmt.Errorf("gemini: unknown subcommand: %s", args[0])
	}
}

func resolveSettingsPath() (string, error) {
	if p := os.Getenv("GEMINI_SETTINGS_PATH"); p != "" {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".gemini", "settings.json"), nil
}

func printHelp() {
	fmt.Print(`Usage: roost gemini <command>

Commands:
  setup    Register roost hooks in ~/.gemini/settings.json
  help     Show this help message
`)
}

// RunSetup registers roost hooks and MCP server in Gemini's settings.
func RunSetup() error {
	settingsPath, err := resolveSettingsPath()
	if err != nil {
		return err
	}
	roostPath, _ := os.Executable()
	if resolved, err := filepath.EvalSymlinks(roostPath); err == nil {
		roostPath = resolved
	}
	events, err := RegisterHooks(settingsPath, roostPath)
	if err != nil {
		return err
	}
	if len(events) == 0 {
		fmt.Println("Hooks already registered")
	} else {
		fmt.Printf("Registered events: %v\n", events)
		fmt.Printf("  Settings: %s\n", settingsPath)
	}
	added, err := RegisterMCPServer(settingsPath, roostPath)
	if err != nil {
		return err
	}
	if added {
		fmt.Printf("Registered MCP server: roost-peers\n")
		fmt.Printf("  Settings: %s\n", settingsPath)
	} else {
		fmt.Println("MCP server roost-peers already registered")
	}
	return nil
}
