package devcontainer

import (
	"reflect"
	"testing"
)

func TestBuildPostCreateArgs_userInjected(t *testing.T) {
	got := buildPostCreateArgs("ctr1", "ubuntu", []string{"bash", "-lc", "echo ok"})
	want := []string{"exec", "-u", "ubuntu", "ctr1", "bash", "-lc", "echo ok"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestBuildPostCreateArgs_emptyUserOmitsFlag(t *testing.T) {
	got := buildPostCreateArgs("ctr1", "", []string{"sh", "-c", "echo ok"})
	want := []string{"exec", "ctr1", "sh", "-c", "echo ok"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
