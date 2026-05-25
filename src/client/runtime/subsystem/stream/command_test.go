package stream

import (
	"strings"
	"testing"
)

func TestPrefixWriter(t *testing.T) {
	var sb strings.Builder
	w := newPrefixWriter(&sb, 5)
	n, err := w.Write([]byte("hello world"))
	if err != nil {
		t.Fatal(err)
	}
	if n != 11 {
		t.Errorf("n = %d, expected to count all input", n)
	}
	if sb.String() != "hello" {
		t.Errorf("dst = %q", sb.String())
	}
	// further writes should be ignored
	w.Write([]byte("xx"))
	if sb.String() != "hello" {
		t.Errorf("dst = %q", sb.String())
	}
}
