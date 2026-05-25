package agent

import "testing"

// SPEC §17.5 — session ID follows the <thread_id>-<turn_id> format (§4.2).
func TestSPEC_17_5_SessionIDFormat(t *testing.T) {
	ids := sessionIDs{threadID: "thread-abc", turnID: "turn-1"}
	got := ids.sessionID()
	want := "thread-abc-turn-1"
	if got != want {
		t.Errorf("sessionID() = %q, want %q", got, want)
	}
}
