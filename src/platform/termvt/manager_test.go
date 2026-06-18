package termvt

import (
	"reflect"
	"testing"
)

func TestManagerCreateGetList(t *testing.T) {
	m := NewManager()
	defer m.CloseAll()

	if _, err := m.Create("a", Spec{Argv: []string{"sleep", "2"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Create("b", Spec{Argv: []string{"sleep", "2"}}); err != nil {
		t.Fatal(err)
	}
	if _, ok := m.Get("a"); !ok {
		t.Fatal("expected session a")
	}
	if got := m.List(); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Fatalf("List() = %v, want [a b]", got)
	}
}

func TestManagerCreateDuplicate(t *testing.T) {
	m := NewManager()
	defer m.CloseAll()

	if _, err := m.Create("dup", Spec{Argv: []string{"sleep", "2"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Create("dup", Spec{Argv: []string{"sleep", "2"}}); err == nil {
		t.Fatal("expected duplicate-id error")
	}
}

func TestManagerRemove(t *testing.T) {
	m := NewManager()
	defer m.CloseAll()

	sess, err := m.Create("x", Spec{Argv: []string{"sleep", "5"}})
	if err != nil {
		t.Fatal(err)
	}
	_, ch := sess.Subscribe()
	if err := m.Remove("x"); err != nil {
		t.Fatal(err)
	}
	if _, ok := m.Get("x"); ok {
		t.Fatal("session x still present after Remove")
	}
	// Remove closed the session → the subscriber sees EventExit then close.
	waitFor(t, ch, func(ev Event) bool { return ev.Kind == EventExit })

	if err := m.Remove("missing"); err == nil {
		t.Fatal("expected error removing missing session")
	}
}
