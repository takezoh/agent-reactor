// Package procio holds the process-wide stdout/stderr writers for library and
// subprocess use. main initializes these once based on commandKind so that
// library code never needs to know whether it runs under a CLI, coordinator,
// or TUI child process.
package procio

import (
	"io"
	"os"
)

var (
	stdout io.Writer = os.Stdout
	stderr io.Writer = os.Stderr
)

func UseTerminal() { stdout, stderr = os.Stdout, os.Stderr }

func UseLogFile(f *os.File) { stdout, stderr = f, f }

func Stdout() io.Writer { return stdout }

func Stderr() io.Writer { return stderr }
