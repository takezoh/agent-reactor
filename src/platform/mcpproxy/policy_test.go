package mcpproxy

import (
	"testing"
)

func TestPolicyCheckTool_allowed(t *testing.T) {
	pol, err := CompilePolicy(
		[]string{"list_log_*", "describe_*"},
		[]string{"delete_*"},
	)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		tool    string
		wantErr bool
	}{
		{"list_log_entries", false},
		{"list_log_groups", false},
		{"describe_cluster", false},
		{"delete_log_group", true}, // denied
		{"create_bucket", true},    // not in allowlist
		{"get_metrics", true},      // not in allowlist
	}
	for _, c := range cases {
		err := pol.CheckTool(c.tool)
		if (err != nil) != c.wantErr {
			t.Errorf("CheckTool(%q) err=%v, wantErr=%v", c.tool, err, c.wantErr)
		}
	}
}

func TestPolicyCheckTool_wildcardAll(t *testing.T) {
	pol, err := CompilePolicy([]string{"read_*", "list_*"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := pol.CheckTool("read_file"); err != nil {
		t.Errorf("read_file should be allowed: %v", err)
	}
	if err := pol.CheckTool("write_file"); err == nil {
		t.Error("write_file should be denied")
	}
}

func TestPolicyCheckTool_emptyAllow(t *testing.T) {
	pol, err := CompilePolicy(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := pol.CheckTool("list_log_entries"); err == nil {
		t.Error("empty allow should reject all tools")
	}
}

func TestPolicyCheckTool_exactMatch(t *testing.T) {
	pol, err := CompilePolicy([]string{"get_status"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := pol.CheckTool("get_status"); err != nil {
		t.Errorf("get_status should be allowed: %v", err)
	}
	if err := pol.CheckTool("get_status_extra"); err == nil {
		t.Error("get_status_extra should not match exact pattern")
	}
}

func TestFilterTools(t *testing.T) {
	pol, err := CompilePolicy(
		[]string{"list_*", "get_*"},
		[]string{"get_secret_*"},
	)
	if err != nil {
		t.Fatal(err)
	}

	input := []string{"list_logs", "get_metrics", "get_secret_key", "delete_logs", "create_alert"}
	got := pol.FilterTools(input)
	want := []string{"list_logs", "get_metrics"}
	if len(got) != len(want) {
		t.Fatalf("FilterTools = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("FilterTools[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
