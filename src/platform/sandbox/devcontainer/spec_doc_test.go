package devcontainer

import (
	"encoding/json"
	"reflect"
	"testing"
)

func rawMessages(t *testing.T, kv map[string]any) map[string]json.RawMessage {
	t.Helper()
	out := make(map[string]json.RawMessage, len(kv))
	for k, v := range kv {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("marshal %s: %v", k, err)
		}
		out[k] = b
	}
	return out
}

func TestExtractString(t *testing.T) {
	cases := []struct {
		name  string
		extra map[string]any
		key   string
		want  string
	}{
		{"present", map[string]any{"k": "value"}, "k", "value"},
		{"absent", map[string]any{}, "k", ""},
		{"wrong type (number)", map[string]any{"k": 42}, "k", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractString(rawMessages(t, tc.extra), tc.key)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestExtractStrings(t *testing.T) {
	cases := []struct {
		name  string
		extra map[string]any
		key   string
		want  []string
	}{
		{"present", map[string]any{"k": []string{"a", "b"}}, "k", []string{"a", "b"}},
		{"empty array", map[string]any{"k": []string{}}, "k", []string{}},
		{"absent", map[string]any{}, "k", nil},
		{"wrong type (string)", map[string]any{"k": "single"}, "k", nil},
		{"wrong type (number)", map[string]any{"k": 42}, "k", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractStrings(rawMessages(t, tc.extra), tc.key)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestExtractPostCreate(t *testing.T) {
	cases := []struct {
		name string
		raw  any
		want []string
	}{
		{"string form is wrapped as bash -lc", "echo hi", []string{"bash", "-lc", "echo hi"}},
		{"array form is passed through", []string{"./setup.sh", "--quick"}, []string{"./setup.sh", "--quick"}},
		{"empty string yields nil", "", nil},
		// Note: json numbers and bools are rejected → nil
		{"number form rejected", 42, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			extra := map[string]any{}
			if tc.raw != nil {
				extra["postCreateCommand"] = tc.raw
			}
			got := extractPostCreate(rawMessages(t, extra))
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestExtractPostCreate_Absent(t *testing.T) {
	if got := extractPostCreate(map[string]json.RawMessage{}); got != nil {
		t.Errorf("absent postCreateCommand must return nil, got %v", got)
	}
}

func TestCloneEnv(t *testing.T) {
	src := map[string]string{"A": "1", "B": "2"}
	dst := cloneEnv(src)
	if !reflect.DeepEqual(src, dst) {
		t.Errorf("cloneEnv copy mismatch: %v vs %v", src, dst)
	}
	// Mutate the copy and verify the source is untouched.
	dst["A"] = "changed"
	dst["C"] = "new"
	if src["A"] != "1" {
		t.Errorf("source mutated through clone: %v", src)
	}
	if _, ok := src["C"]; ok {
		t.Errorf("source got new key from clone: %v", src)
	}
}

func TestCloneEnv_Nil(t *testing.T) {
	got := cloneEnv(nil)
	if got == nil {
		t.Errorf("cloneEnv(nil) must return non-nil empty map for safe writes")
	}
	if len(got) != 0 {
		t.Errorf("cloneEnv(nil) got %v, want empty", got)
	}
}
