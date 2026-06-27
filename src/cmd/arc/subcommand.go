package main

import (
	"fmt"
	"io"

	"github.com/takezoh/agent-reactor/client/cli"
	"github.com/takezoh/agent-reactor/platform/appid"
)

type commandKind int

const (
	commandKindCLI commandKind = iota
	commandKindDaemon
	commandKindRoost
)

func classifyCommand(args []string) commandKind {
	if len(args) == 0 {
		return commandKindRoost
	}
	if isCoordinatorFlag(args[0]) {
		return commandKindRoost
	}
	if args[0] == "--tui" {
		return commandKindDaemon
	}
	if isHelpCommand(args[0]) {
		return commandKindCLI
	}
	if cli.Has(args[0]) {
		return commandKindCLI
	}
	return commandKindCLI
}

func runCommand(args []string, stdout io.Writer) error {
	if len(args) == 0 || isCoordinatorFlag(args[0]) {
		return runCoordinatorFn()
	}
	if isHelpCommand(args[0]) {
		printUsage(stdout)
		return nil
	}
	if args[0] == "--tui" {
		redirectStderr()
		return runTUI(args[1:])
	}
	handled, err := cli.Dispatch(args)
	if handled {
		return err
	}
	return fmt.Errorf("unknown command: %s (run `%s help` for usage)", args[0], appid.ClientBin)
}

var tuiHandlers = map[string]func([]string) error{
	"header":   func(_ []string) error { return runHeaderTUIFn() },
	"main":     func(_ []string) error { return runMainTUIFn() },
	"sessions": func(_ []string) error { return runSessionListFn() },
	"log":      func(_ []string) error { return runLogViewerFn() },
	"palette":  func(args []string) error { return runPaletteFn(args) },
}

func runTUI(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("unknown tui: missing subcommand")
	}
	h, ok := tuiHandlers[args[0]]
	if !ok {
		return fmt.Errorf("unknown tui: %s", args[0])
	}
	return h(args[1:])
}

func printUsage(w io.Writer) {
	fmt.Fprintf(w, "%s - AI agent session manager\n", appid.ClientBin)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintf(w, "  %s          Start or attach to the %s daemon session\n", appid.ClientBin, appid.ClientBin)
	for _, pair := range cli.RegisteredHelp() {
		fmt.Fprintf(w, "  %s %-8s %s\n", appid.ClientBin, pair[0], pair[1])
	}
	fmt.Fprintf(w, "  %s help     Show this help message\n", appid.ClientBin)
}

func isHelpCommand(arg string) bool {
	return arg == "-h" || arg == "--help" || arg == "help"
}
