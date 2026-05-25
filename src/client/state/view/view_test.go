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

func TestConnectorSection(t *testing.T) {
	s := ConnectorSection{Title: "PRs", Items: []ConnectorItem{{Symbol: "*", Title: "t", Meta: "m"}}}
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var back ConnectorSection
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if back.Title != "PRs" || len(back.Items) != 1 || back.Items[0].Title != "t" {
		t.Errorf("round trip failed: %+v", back)
	}
}
