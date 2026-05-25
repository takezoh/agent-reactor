package codex

import (
	"fmt"
	"os"
	"path/filepath"
)

// Run dispatches Codex subcommands.
func Run(args []string) error {
	if len(args) == 0 {
		printHelp()
		return fmt.Errorf("codex: missing subcommand")
	}
	switch args[0] {
	case "setup":
		return RunSetup()
	case "help", "-h", "--help":
		printHelp()
		return nil
	default:
		fmt.Fprintf(os.Stderr, "roost codex: unknown subcommand: %s\n", args[0])
		printHelp()
		return fmt.Errorf("codex: unknown subcommand: %s", args[0])
	}
}

func printHelp() {
	fmt.Print(`Usage: roost codex <command>

Commands:
  setup    Register roost MCP server in ~/.codex/mcp.json
  help     Show this help message
`)
}

// RunSetup registers roost MCP server in Codex's config file.
func RunSetup() error {
	configDir, err := getConfigDir()
	if err != nil {
		return err
	}
	mcpPath := filepath.Join(configDir, "mcp.json")

	roostPath, _ := os.Executable()
	if resolved, err := filepath.EvalSymlinks(roostPath); err == nil {
		roostPath = resolved
	}
	added, err := RegisterMCPServer(mcpPath, roostPath)
	if err != nil {
		return err
	}
	if added {
		fmt.Printf("Registered MCP server: roost-peers\n")
		fmt.Printf("  MCP:    %s\n", mcpPath)
	} else {
		fmt.Println("MCP server roost-peers already registered")
	}
	return nil
}

func getConfigDir() (string, error) {
	if dir := os.Getenv("CODEX_CONFIG_DIR"); dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".codex"), nil
}
