package stream

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/takezoh/agent-roost/client/state"
)

func TestExtractThreadID(t *testing.T) {
	if got := extractThreadID([]byte(`{"threadId":"t1"}`)); got != "t1" {
		t.Errorf("flat: %q", got)
	}
	if got := extractThreadID([]byte(`{"thread":{"id":"t2"}}`)); got != "t2" {
		t.Errorf("nested: %q", got)
	}
	if got := extractThreadID([]byte(`bad`)); got != "" {
		t.Errorf("bad: %q", got)
	}
	if got := extractThreadID([]byte(`{}`)); got != "" {
		t.Errorf("empty: %q", got)
	}
}

func TestExtractTurnID(t *testing.T) {
	if got := extractTurnID([]byte(`{"turnId":"tu1"}`)); got != "tu1" {
		t.Errorf("flat: %q", got)
	}
	if got := extractTurnID([]byte(`{"turn":{"id":"tu2"}}`)); got != "tu2" {
		t.Errorf("nested: %q", got)
	}
	if got := extractTurnID([]byte(`bad`)); got != "" {
		t.Errorf("bad: %q", got)
	}
}

func TestExtractThreadCWD(t *testing.T) {
	if got := extractThreadCWD([]byte(`{"cwd":"/p"}`)); got != "/p" {
		t.Errorf("flat: %q", got)
	}
	if got := extractThreadCWD([]byte(`{"thread":{"cwd":"/q"}}`)); got != "/q" {
		t.Errorf("nested: %q", got)
	}
	if got := extractThreadCWD([]byte(`bad`)); got != "" {
		t.Errorf("bad: %q", got)
	}
}

func TestExtractText(t *testing.T) {
	if got := extractText([]byte(`{"text":"hi"}`)); got != "hi" {
		t.Errorf("text: %q", got)
	}
	if got := extractText([]byte(`{"delta":"d"}`)); got != "d" {
		t.Errorf("delta: %q", got)
	}
	if got := extractText([]byte(`{"item":{"text":"ti"}}`)); got != "ti" {
		t.Errorf("item.text: %q", got)
	}
	if got := extractText([]byte(`{"item":{"content":"c"}}`)); got != "c" {
		t.Errorf("item.content: %q", got)
	}
	if got := extractText([]byte(`bad`)); got != "" {
		t.Errorf("bad: %q", got)
	}
}

func TestNestedString(t *testing.T) {
	if got := nestedString([]byte(`{"x":"a"}`), "x"); got != "a" {
		t.Errorf("flat: %q", got)
	}
	if got := nestedString([]byte(`{"item":{"x":"b"}}`), "x"); got != "b" {
		t.Errorf("nested: %q", got)
	}
	if got := nestedString([]byte(`bad`), "x"); got != "" {
		t.Errorf("bad: %q", got)
	}
}

func TestExtractThreadStatus(t *testing.T) {
	raw := []byte(`{"threadId":"t1","status":{"type":"active","activeFlags":["waitingOnApproval"]}}`)
	st, wa, tid := extractThreadStatus(raw)
	if st != "active" || !wa || tid != "t1" {
		t.Errorf("got st=%q wa=%v tid=%q", st, wa, tid)
	}
	raw2 := []byte(`{"thread":{"id":"t2","status":{"type":"idle"}}}`)
	st, wa, tid = extractThreadStatus(raw2)
	if st != "idle" || wa || tid != "t2" {
		t.Errorf("nested: %q %v %q", st, wa, tid)
	}
	// invalid JSON must not panic
	_, _, _ = extractThreadStatus([]byte(`bad`))
	st, _, _ = extractThreadStatus([]byte(`{"threadId":"t"}`))
	if st != "" {
		t.Errorf("no status: %q", st)
	}
}

func TestThreadStatusEventsActive(t *testing.T) {
	raw := json.RawMessage(`{"threadId":"t","status":{"type":"active"}}`)
	out, st, wa := threadStatusEvents(raw, "t", "", false)
	if st != "active" || wa {
		t.Errorf("st=%q wa=%v", st, wa)
	}
	if len(out) != 1 || out[0].kind != state.SubsystemTurnStarted {
		t.Errorf("expected TurnStarted, got %+v", out)
	}
}

func TestThreadStatusEventsActiveApproval(t *testing.T) {
	raw := json.RawMessage(`{"threadId":"t","status":{"type":"active","activeFlags":["waitingOnApproval"]}}`)
	out, _, _ := threadStatusEvents(raw, "t", "idle", false)
	hasTurnStart, hasApproval := false, false
	for _, e := range out {
		if e.kind == state.SubsystemTurnStarted {
			hasTurnStart = true
		}
		if e.kind == state.SubsystemApprovalRequested {
			hasApproval = true
		}
	}
	if !hasTurnStart || !hasApproval {
		t.Errorf("missing events: %+v", out)
	}
}

func TestThreadStatusEventsApprovalResolved(t *testing.T) {
	raw := json.RawMessage(`{"threadId":"t","status":{"type":"active"}}`)
	out, _, _ := threadStatusEvents(raw, "t", "active", true)
	found := false
	for _, e := range out {
		if e.kind == state.SubsystemApprovalResolved {
			found = true
		}
	}
	if !found {
		t.Errorf("expected ApprovalResolved, got %+v", out)
	}
}

func TestThreadStatusEventsIdle(t *testing.T) {
	raw := json.RawMessage(`{"threadId":"t","status":{"type":"idle"}}`)
	out, _, _ := threadStatusEvents(raw, "t", "active", false)
	if len(out) != 1 || out[0].kind != state.SubsystemTurnCompleted {
		t.Errorf("expected TurnCompleted, got %+v", out)
	}
}

func TestThreadStatusEventsForeignThread(t *testing.T) {
	raw := json.RawMessage(`{"threadId":"other","status":{"type":"active"}}`)
	out, _, _ := threadStatusEvents(raw, "self", "", false)
	if out != nil {
		t.Errorf("foreign thread should be filtered, got %+v", out)
	}
}

func TestItemLifecycleEvents(t *testing.T) {
	startCmd := json.RawMessage(`{"item":{"type":"commandExecution","itemId":"i1","command":"ls","cwd":"/p"}}`)
	out := itemLifecycleEvents("item/started", startCmd, "t")
	if len(out) != 1 || out[0].kind != state.SubsystemToolStarted || out[0].payload.Tool.Name != "command" {
		t.Errorf("cmd start: %+v", out)
	}
	endCmd := json.RawMessage(`{"item":{"type":"commandExecution","itemId":"i1","command":"ls","error":"oops"}}`)
	out = itemLifecycleEvents("item/completed", endCmd, "t")
	if len(out) != 1 || out[0].kind != state.SubsystemToolCompleted || out[0].payload.Tool.Error != "oops" {
		t.Errorf("cmd end: %+v", out)
	}
	fileStart := json.RawMessage(`{"item":{"type":"fileChange","itemId":"i2","path":"/x"}}`)
	out = itemLifecycleEvents("item/started", fileStart, "t")
	if len(out) != 1 || out[0].payload.Tool.Name != "file_change" {
		t.Errorf("file start: %+v", out)
	}
	out = itemLifecycleEvents("item/completed", fileStart, "t")
	if len(out) != 1 {
		t.Errorf("file end: %+v", out)
	}
	if out := itemLifecycleEvents("unknown", fileStart, "t"); out != nil {
		t.Errorf("unknown method should return nil: %+v", out)
	}
}

func TestSummarizePlan(t *testing.T) {
	if got := summarizePlan([]byte(`{"summary":"s"}`)); got != "s" {
		t.Errorf("summary: %q", got)
	}
	got := summarizePlan([]byte(`{"items":[{"step":"a","status":"done"},{"step":"b","status":"pending"}]}`))
	if !strings.Contains(got, "a done") || !strings.Contains(got, "b pending") {
		t.Errorf("items: %q", got)
	}
	if summarizePlan([]byte(`bad`)) != "" {
		t.Errorf("bad")
	}
}

func TestSummarizeDiff(t *testing.T) {
	got := summarizeDiff([]byte(`{"paths":["a","b"]}`))
	if got != "a, b" {
		t.Errorf("got %q", got)
	}
	if summarizeDiff([]byte(`{}`)) != "" {
		t.Errorf("empty")
	}
	if summarizeDiff([]byte(`bad`)) != "" {
		t.Errorf("bad")
	}
}

func TestApprovalFromParams(t *testing.T) {
	a := approvalFromParams("item/commandExecution/requestApproval", []byte(`{"itemId":"i","command":"c","reason":"r"}`), true)
	if a.Kind != "command" || a.Command != "c" || !a.AutoApprove {
		t.Errorf("%+v", a)
	}
	b := approvalFromParams("item/fileChange/requestApproval", []byte(`{"path":"/p"}`), false)
	if b.Kind != "file_change" || b.Path != "/p" {
		t.Errorf("%+v", b)
	}
}

func TestAppendHistory(t *testing.T) {
	var h []state.SubsystemTurn
	for range 10 {
		appendHistory(&h, "user", "msg")
	}
	if len(h) != 6 {
		t.Errorf("len = %d", len(h))
	}
	appendHistory(&h, "user", "   ")
	if len(h) != 6 {
		t.Errorf("whitespace should be ignored")
	}
}

func TestFirstNonEmpty(t *testing.T) {
	if got := firstNonEmpty("", "  ", "x"); got != "x" {
		t.Errorf("got %q", got)
	}
	if got := firstNonEmpty("", ""); got != "" {
		t.Errorf("all empty: %q", got)
	}
}

func TestItemType(t *testing.T) {
	if got := itemType([]byte(`{"item":{"type":"x"}}`)); got != "x" {
		t.Errorf("got %q", got)
	}
}
