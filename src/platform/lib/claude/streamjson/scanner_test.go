package streamjson

import (
	"strings"
	"testing"
)

func TestScanner_Basic(t *testing.T) {
	input := strings.Join([]string{
		`{"type":"system","subtype":"init","session_id":"s1"}`,
		`{"type":"assistant","message":{"content":[{"type":"text","text":"hi"}]}}`,
		`{"type":"result","subtype":"success","result":"done","is_error":false,"usage":{"input_tokens":5,"output_tokens":3}}`,
	}, "\n")

	sc := NewScanner(strings.NewReader(input))

	if !sc.Scan() {
		t.Fatal("Scan 1 returned false")
	}
	if _, ok := sc.Event().(SystemInit); !ok {
		t.Errorf("event 1: got %T, want SystemInit", sc.Event())
	}

	if !sc.Scan() {
		t.Fatal("Scan 2 returned false")
	}
	if _, ok := sc.Event().(AssistantMessage); !ok {
		t.Errorf("event 2: got %T, want AssistantMessage", sc.Event())
	}

	if !sc.Scan() {
		t.Fatal("Scan 3 returned false")
	}
	if _, ok := sc.Event().(Result); !ok {
		t.Errorf("event 3: got %T, want Result", sc.Event())
	}

	if sc.Scan() {
		t.Error("Scan 4 should return false at EOF")
	}
	if err := sc.Err(); err != nil {
		t.Errorf("unexpected io error: %v", err)
	}
}

func TestScanner_SkipsBadLines(t *testing.T) {
	input := strings.Join([]string{
		``,                    // empty line
		`   `,                 // whitespace
		`not json at all {{{`, // malformed
		`{"type":"system","subtype":"init","session_id":"s1"}`,
		`{"type":"result","subtype":"success","is_error":false,"usage":{"input_tokens":1,"output_tokens":1}}`,
	}, "\n")

	sc := NewScanner(strings.NewReader(input))

	var events []Event
	for sc.Scan() {
		events = append(events, sc.Event())
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("unexpected io error: %v", err)
	}

	if len(events) != 2 {
		t.Fatalf("got %d events, want 2", len(events))
	}
	if _, ok := events[0].(SystemInit); !ok {
		t.Errorf("events[0]: got %T, want SystemInit", events[0])
	}
	if sc.Skipped() < 1 {
		t.Errorf("Skipped() = %d, want ≥1 (malformed line)", sc.Skipped())
	}
}

func TestScanner_LargeLine(t *testing.T) {
	// Synthesise an assistant message whose text exceeds the default 64 KiB
	// bufio.Scanner limit to verify the 64 MiB buffer is in effect.
	bigText := strings.Repeat("x", 128*1024) // 128 KiB
	line := `{"type":"assistant","message":{"content":[{"type":"text","text":"` + bigText + `"}]}}`

	sc := NewScanner(strings.NewReader(line))
	if !sc.Scan() {
		t.Fatalf("Scan returned false; err=%v", sc.Err())
	}
	got, ok := sc.Event().(AssistantMessage)
	if !ok {
		t.Fatalf("got %T, want AssistantMessage", sc.Event())
	}
	if len(got.Text) != len(bigText) {
		t.Errorf("Text length = %d, want %d", len(got.Text), len(bigText))
	}
}

func TestScanner_UnknownTypesPassThrough(t *testing.T) {
	input := strings.Join([]string{
		`{"type":"debug","x":1}`,
		`{"type":"system","subtype":"init","session_id":"s2"}`,
	}, "\n")

	sc := NewScanner(strings.NewReader(input))

	if !sc.Scan() {
		t.Fatal("Scan 1 returned false")
	}
	if _, ok := sc.Event().(Unknown); !ok {
		t.Errorf("event 1: got %T, want Unknown", sc.Event())
	}

	if !sc.Scan() {
		t.Fatal("Scan 2 returned false")
	}
	if _, ok := sc.Event().(SystemInit); !ok {
		t.Errorf("event 2: got %T, want SystemInit", sc.Event())
	}
}
