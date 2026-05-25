package globutil

import "testing"

func TestCompileGlob(t *testing.T) {
	cases := []struct {
		pattern string
		input   string
		match   bool
	}{
		{"foo", "foo", true},
		{"foo", "foobar", false},
		{"foo*", "foobar", true},
		{"*bar", "foobar", true},
		{"foo*bar", "foo123bar", true},
		{"foo*bar", "fooXbarY", false},
		{"a.b", "a.b", true},
		{"a.b", "aXb", false},
		{"*", "anything\nwith newline", true},
	}
	for _, c := range cases {
		re, err := CompileGlob(c.pattern)
		if err != nil {
			t.Fatalf("compile %q: %v", c.pattern, err)
		}
		if got := re.MatchString(c.input); got != c.match {
			t.Errorf("%q match %q = %v, want %v", c.pattern, c.input, got, c.match)
		}
	}
}
