package termvt

import (
	"testing"
)

// TestSessionExitCodeSuccess verifies a clean exit reports code 0 and exited=true.
func TestSessionExitCodeSuccess(t *testing.T) {
	s, err := NewSession(Spec{Argv: []string{"bash", "-c", "exit 0"}})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	_, ch := s.Subscribe()
	waitFor(t, ch, func(ev Event) bool { return ev.Kind == EventExit })

	code, exited := s.ExitCode()
	if !exited {
		t.Fatal("ExitCode() exited = false after EventExit, want true")
	}
	if code != 0 {
		t.Fatalf("ExitCode() code = %d, want 0", code)
	}
}

// TestSessionExitCodeNonZero verifies a non-zero exit propagates the exit code.
func TestSessionExitCodeNonZero(t *testing.T) {
	s, err := NewSession(Spec{Argv: []string{"bash", "-c", "exit 7"}})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	_, ch := s.Subscribe()
	waitFor(t, ch, func(ev Event) bool { return ev.Kind == EventExit })

	code, exited := s.ExitCode()
	if !exited {
		t.Fatal("ExitCode() exited = false after EventExit, want true")
	}
	if code != 7 {
		t.Fatalf("ExitCode() code = %d, want 7", code)
	}
}

// TestSessionExitCodeAlive verifies a live session reports exited=false.
func TestSessionExitCodeAlive(t *testing.T) {
	s, err := NewSession(Spec{Argv: []string{"sleep", "5"}})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	if _, exited := s.ExitCode(); exited {
		t.Fatal("ExitCode() exited = true for a live session, want false")
	}
}

// TestStripSGRTail pins the SGR-stripping trailing-line helper: it returns the
// last n lines with all CSI ... m (SGR) sequences removed.
func TestStripSGRTail(t *testing.T) {
	cases := []struct {
		name string
		in   string
		n    int
		want string
	}{
		{
			name: "strips SGR colour codes",
			in:   "\x1b[31mred\x1b[0m\x1b[1mbold\x1b[0m",
			n:    1,
			want: "redbold",
		},
		{
			name: "keeps trailing n lines",
			in:   "one\ntwo\nthree\nfour",
			n:    2,
			want: "three\nfour",
		},
		{
			name: "n larger than line count returns all",
			in:   "a\nb",
			n:    10,
			want: "a\nb",
		},
		{
			name: "strips SGR across multiple lines",
			in:   "\x1b[32mgreen\x1b[0m\n\x1b[33myellow\x1b[0m",
			n:    2,
			want: "green\nyellow",
		},
		{
			name: "non-positive n returns empty",
			in:   "a\nb\nc",
			n:    0,
			want: "",
		},
		{
			name: "trailing blank lines preserved within window",
			in:   "x\ny\n",
			n:    2,
			want: "y\n",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := stripSGRTail(c.in, c.n); got != c.want {
				t.Errorf("stripSGRTail(%q, %d) = %q, want %q", c.in, c.n, got, c.want)
			}
		})
	}
}
