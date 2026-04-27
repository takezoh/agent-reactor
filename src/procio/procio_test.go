package procio

import (
	"os"
	"testing"
)

func TestUseTerminal(t *testing.T) {
	t.Cleanup(UseTerminal)
	UseTerminal()
	if Stdout() != os.Stdout {
		t.Error("Stdout() should be os.Stdout after UseTerminal")
	}
	if Stderr() != os.Stderr {
		t.Error("Stderr() should be os.Stderr after UseTerminal")
	}
}

func TestUseLogFile(t *testing.T) {
	t.Cleanup(UseTerminal)
	f, err := os.CreateTemp(t.TempDir(), "procio-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	UseLogFile(f)
	if Stdout() != f {
		t.Error("Stdout() should be logFile after UseLogFile")
	}
	if Stderr() != f {
		t.Error("Stderr() should be logFile after UseLogFile")
	}
}
