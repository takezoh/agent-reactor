//go:build integration

package vt_test

// TestOscPipeVsVtEmulator verifies that OSC sequences survive the
// pipe-pane → VT emulator path.
//
// Run with: cd src && go test -tags=integration ./driver/vt/ -run TestOscPipeVsVtEmulator -v

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/takezoh/agent-roost/driver/vt"
)

func TestOscPipeVsVtEmulator(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not found in PATH")
	}

	session := "roost-pipe-osc-verify"
	bufPath := t.TempDir() + "/pipe.buf"

	cleanup := func() { exec.Command("tmux", "kill-session", "-t", session).Run() } //nolint:errcheck
	cleanup()
	t.Cleanup(cleanup)

	run := func(args ...string) string {
		out, err := exec.Command("tmux", args...).Output()
		if err != nil {
			t.Fatalf("tmux %v: %v", args, err)
		}
		return string(out)
	}

	run("new-session", "-d", "-s", session, "-x", "200", "-y", "50", "bash", "--norc")
	pane := session + ":0.0"

	run("pipe-pane", "-t", pane, "cat >> "+bufPath)
	time.Sleep(100 * time.Millisecond)

	type pipeCase struct {
		name   string
		cmd    int
		printf string
	}

	cases := []pipeCase{
		{"OSC 0 (window title)", 0, `'\x1b]0;pipetitle0\x1b\\'`},
		{"OSC 2 (window title)", 2, `'\x1b]2;pipetitle2\x1b\\'`},
		{"OSC 9 (notification)", 9, `'\x1b]9;pipe9body\x1b\\'`},
		{"OSC 99 (notification)", 99, `'\x1b]99;d=pipe99title:p=pipe99body\x1b\\'`},
		{"OSC 777 (notification)", 777, `'\x1b]777;notify;pipe777title;pipe777body\x1b\\'`},
		{"OSC 133;B (prompt input)", 133, `'\x1b]133;B\x1b\\'`},
	}

	for _, tc := range cases {
		run("send-keys", "-t", pane, fmt.Sprintf("printf %s", tc.printf), "Enter")
	}
	time.Sleep(300 * time.Millisecond)
	run("pipe-pane", "-t", pane)

	raw, err := os.ReadFile(bufPath)
	if err != nil {
		t.Fatalf("read buf: %v", err)
	}
	t.Logf("pipe-pane buffer: %d bytes", len(raw))

	var notifs []vt.OscNotification
	var promptEvents []vt.PromptEvent
	var titles []string

	term := vt.New(200, 50)
	term.OnOscNotification = func(n vt.OscNotification) { notifs = append(notifs, n) }
	term.OnPromptEvent = func(e vt.PromptEvent) { promptEvents = append(promptEvents, e) }
	term.OnWindowTitle = func(_ int, title string) { titles = append(titles, title) }

	if err := term.Feed(raw); err != nil {
		t.Fatalf("Feed: %v", err)
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			marker := fmt.Sprintf("\x1b]%d;", tc.cmd)
			survivedPipe := strings.Contains(string(raw), marker)

			var detected bool
			switch tc.cmd {
			case 0, 2:
				detected = len(titles) > 0
			case 9, 99, 777:
				for _, n := range notifs {
					if n.Cmd == tc.cmd {
						detected = true
					}
				}
			case 133:
				for _, pe := range promptEvents {
					if pe.Phase == vt.PromptPhaseInput {
						detected = true
					}
				}
			}

			t.Logf("survived pipe-pane      : %v", survivedPipe)
			t.Logf("detected by VT emulator : %v", detected)
			if survivedPipe && detected {
				t.Logf("RESULT: pipe preserves OSC %d — VT emulator detects it", tc.cmd)
			} else if survivedPipe {
				t.Logf("RESULT: pipe preserves OSC %d — VT emulator missed it", tc.cmd)
			} else {
				t.Logf("RESULT: pipe drops OSC %d", tc.cmd)
			}
		})
	}
}
