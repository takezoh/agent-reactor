package view

import (
	"encoding/json"
	"testing"
)

func TestStatusStringRoundTrip(t *testing.T) {
	cases := []struct {
		s    Status
		name string
		sym  string
	}{
		{StatusRunning, "running", "status.running"},
		{StatusWaiting, "waiting", "status.waiting"},
		{StatusIdle, "idle", "status.idle"},
		{StatusStopped, "stopped", "status.stopped"},
		{StatusPending, "pending", "status.pending"},
	}
	for _, c := range cases {
		if got := c.s.String(); got != c.name {
			t.Errorf("String(%d) = %q, want %q", c.s, got, c.name)
		}
		if got := c.s.SymbolKey(); got != c.sym {
			t.Errorf("SymbolKey(%d) = %q, want %q", c.s, got, c.sym)
		}
		parsed, ok := ParseStatus(c.name)
		if !ok || parsed != c.s {
			t.Errorf("ParseStatus(%q) = %d,%v, want %d,true", c.name, parsed, ok, c.s)
		}
	}
}

func TestStatusUnknown(t *testing.T) {
	var bad Status = 99
	if got := bad.String(); got != "unknown" {
		t.Errorf("unknown String = %q", got)
	}
	if got := bad.SymbolKey(); got != "" {
		t.Errorf("unknown SymbolKey = %q", got)
	}
	if _, ok := ParseStatus("nope"); ok {
		t.Errorf("ParseStatus(nope) ok = true")
	}
}

func TestStatusJSON(t *testing.T) {
	b, err := json.Marshal(StatusWaiting)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `"waiting"` {
		t.Errorf("marshal = %s", b)
	}
	var s Status
	if err := json.Unmarshal([]byte(`"idle"`), &s); err != nil {
		t.Fatal(err)
	}
	if s != StatusIdle {
		t.Errorf("unmarshal got %v", s)
	}
	if err := json.Unmarshal([]byte(`"bogus"`), &s); err == nil {
		t.Errorf("expected error for unknown status")
	}
	if err := json.Unmarshal([]byte(`123`), &s); err == nil {
		t.Errorf("expected error for non-string")
	}
}

func TestHostTag(t *testing.T) {
	tag := HostTag()
	if tag.Text != "host" || tag.Foreground == "" || tag.Background == "" {
		t.Errorf("HostTag malformed: %+v", tag)
	}
}

func TestViewJSONOmitEmpty(t *testing.T) {
	v := View{Card: Card{Title: "x"}}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var back View
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if back.Card.Title != "x" {
		t.Errorf("round trip lost title: %+v", back)
	}
}

// View.Status must always be present on the wire. Status is an int (iota)
// where StatusRunning == 0, so an omitempty tag would silently drop the
// field for every running session and the web client would render it as
// "unknown". Round-trip a running View and assert the JSON contains the
// status key.
func TestViewJSONStatusAlwaysEmitted(t *testing.T) {
	for _, s := range []Status{StatusRunning, StatusWaiting, StatusIdle, StatusStopped, StatusPending} {
		v := View{Card: Card{Title: "x"}, Status: s}
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("marshal %v: %v", s, err)
		}
		var obj map[string]any
		if err := json.Unmarshal(b, &obj); err != nil {
			t.Fatalf("unmarshal %v: %v", s, err)
		}
		got, ok := obj["status"]
		if !ok {
			t.Errorf("status field missing for %v; json=%s", s, b)
			continue
		}
		if got != s.String() {
			t.Errorf("status field = %q, want %q (status %v)", got, s.String(), s)
		}
	}
}

