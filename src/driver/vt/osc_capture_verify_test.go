//go:build integration

package vt_test

// TestOscCapturePaneVsVtEmulator verifies which OSC sequences survive
// tmux capture-pane -e and whether the VT emulator can detect them.
//
// Run with: cd src && go test -tags=integration ./driver/vt/ -run TestOscCapturePaneVsVtEmulator -v

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/takezoh/agent-roost/driver/vt"
)

type oscCase struct {
	name   string
	cmd    int
	printf string
}

func TestOscCapturePaneVsVtEmulator(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not found in PATH")
	}

	session := "roost-osc-verify"
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

	cases := []oscCase{
		{"OSC 0 (window title)", 0, `'\x1b]0;testtitle0\x1b\\'`},
		{"OSC 2 (window title)", 2, `'\x1b]2;testtitle2\x1b\\'`},
		{"OSC 9 (notification)", 9, `'\x1b]9;osc9body\x1b\\'`},
		{"OSC 99 (notification)", 99, `'\x1b]99;d=osc99title:p=osc99body\x1b\\'`},
		{"OSC 777 (notification)", 777, `'\x1b]777;notify;osc777title;osc777body\x1b\\'`},
		{"OSC 133;B (prompt input)", 133, `'\x1b]133;B\x1b\\'`},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			run("send-keys", "-t", pane, fmt.Sprintf("printf %s", tc.printf), "Enter")
			time.Sleep(200 * time.Millisecond)

			raw := run("capture-pane", "-p", "-e", "-t", pane, "-S", "-50")
			marker := fmt.Sprintf("\x1b]%d;", tc.cmd)
			survivedRaw := strings.Contains(raw, marker)

			var notifs []vt.OscNotification
			var promptEvents []vt.PromptEvent
			var titles []string
			term := vt.New(200, 50)
			term.OnOscNotification = func(n vt.OscNotification) { notifs = append(notifs, n) }
			term.OnPromptEvent = func(e vt.PromptEvent) { promptEvents = append(promptEvents, e) }
			term.OnWindowTitle = func(_ int, title string) { titles = append(titles, title) }
			if err := term.Feed([]byte(raw)); err != nil {
				t.Fatalf("Feed: %v", err)
			}

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

			t.Logf("survived capture-pane -e : %v", survivedRaw)
			t.Logf("detected by VT emulator  : %v", detected)
			if !survivedRaw {
				t.Logf("RESULT: capture-pane drops OSC %d", tc.cmd)
			} else if detected {
				t.Logf("RESULT: capture-pane preserves OSC %d — VT emulator detects it", tc.cmd)
			} else {
				t.Logf("RESULT: capture-pane preserves OSC %d — VT emulator missed it", tc.cmd)
			}
		})
	}
}
