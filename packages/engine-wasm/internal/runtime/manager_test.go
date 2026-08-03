package runtime

import (
	"reflect"
	"testing"
)

type doc struct {
	id   string
	name string
}

func newDocManager() *Manager[*doc] {
	return NewManager(
		func(d *doc) *doc {
			if d == nil {
				return nil
			}
			clone := *d
			return &clone
		},
		func(d *doc) string {
			if d == nil {
				return ""
			}
			return d.id
		},
	)
}

func TestManagerReplacePreservesOtherEntriesAndOrder(t *testing.T) {
	m := newDocManager()
	m.Create(&doc{id: "a", name: "A"})
	m.Create(&doc{id: "b", name: "B"})
	m.Create(&doc{id: "c", name: "C"})

	// b is not active (c is); replacing it must not disturb the active id.
	if m.ActiveID() != "c" {
		t.Fatalf("active id = %q, want %q", m.ActiveID(), "c")
	}
	if err := m.Replace(&doc{id: "b", name: "B updated"}); err != nil {
		t.Fatalf("Replace(b): %v", err)
	}
	if m.ActiveID() != "c" {
		t.Fatalf("active id after Replace = %q, want %q (must be unchanged)", m.ActiveID(), "c")
	}

	// Order must be preserved.
	if got, want := m.IDs(), []string{"a", "b", "c"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("IDs after Replace = %v, want %v", got, want)
	}

	// The replaced value must reflect the new content; the others untouched.
	if err := m.SetActiveID("b"); err != nil {
		t.Fatalf("SetActiveID(b): %v", err)
	}
	if got := m.Active(); got == nil || got.name != "B updated" {
		t.Fatalf("Active(b) = %+v, want name %q", got, "B updated")
	}
	if err := m.SetActiveID("a"); err != nil {
		t.Fatalf("SetActiveID(a): %v", err)
	}
	if got := m.Active(); got == nil || got.name != "A" {
		t.Fatalf("Active(a) = %+v, want untouched name %q", got, "A")
	}
}

func TestManagerReplaceClonesValue(t *testing.T) {
	m := newDocManager()
	m.Create(&doc{id: "a", name: "A"})

	input := &doc{id: "a", name: "A2"}
	if err := m.Replace(input); err != nil {
		t.Fatalf("Replace: %v", err)
	}
	// Mutating the caller's value after Replace must not affect stored state.
	input.name = "mutated"
	if got := m.ActiveMut(); got.name != "A2" {
		t.Fatalf("stored value = %q, want %q (Replace must clone)", got.name, "A2")
	}
}

func TestManagerReplaceReinsertsMissingID(t *testing.T) {
	m := newDocManager()
	m.Create(&doc{id: "a", name: "A"})
	m.Create(&doc{id: "b", name: "B"})

	// Re-inserting an id that is not present appends it to the order.
	if err := m.Replace(&doc{id: "z", name: "Z"}); err != nil {
		t.Fatalf("Replace(z): %v", err)
	}
	if got, want := m.IDs(), []string{"a", "b", "z"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("IDs after re-insert = %v, want %v", got, want)
	}

	// Re-inserting the same id again must not duplicate the order entry.
	if err := m.Replace(&doc{id: "z", name: "Z2"}); err != nil {
		t.Fatalf("Replace(z) again: %v", err)
	}
	if got, want := m.IDs(), []string{"a", "b", "z"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("IDs after second re-insert = %v, want %v", got, want)
	}
}

func TestManagerReplaceRejectsEmptyID(t *testing.T) {
	m := newDocManager()
	if err := m.Replace(&doc{id: ""}); err == nil {
		t.Fatal("expected Replace to reject an empty id")
	}
}

func TestManagerIDsReturnsCopy(t *testing.T) {
	m := newDocManager()
	m.Create(&doc{id: "a"})
	ids := m.IDs()
	ids[0] = "mutated"
	if got := m.IDs(); got[0] != "a" {
		t.Fatalf("IDs mutation leaked into manager: %v", got)
	}
}

func TestManagerHasAndInspectDoNotClone(t *testing.T) {
	cloneCount := 0
	m := NewManager(
		func(d *doc) *doc {
			cloneCount++
			if d == nil {
				return nil
			}
			clone := *d
			return &clone
		},
		func(d *doc) string {
			if d == nil {
				return ""
			}
			return d.id
		},
	)
	m.Create(&doc{id: "a", name: "A"})
	cloneCount = 0

	if !m.Has("a") || m.Has("missing") {
		t.Fatalf("unexpected Has results: present=%v missing=%v", m.Has("a"), m.Has("missing"))
	}
	var inspected *doc
	if !m.Inspect("a", func(value *doc) { inspected = value }) {
		t.Fatal("Inspect(a) returned false")
	}
	if inspected != m.ActiveMut() {
		t.Fatal("Inspect should expose the stored value without cloning")
	}
	if m.Inspect("missing", func(*doc) {}) {
		t.Fatal("Inspect(missing) returned true")
	}
	if cloneCount != 0 {
		t.Fatalf("Has/Inspect cloned %d values, want 0", cloneCount)
	}
}

func TestManagerReplaceActiveNoCloneStoresSamePointer(t *testing.T) {
	m := newDocManager()
	m.Create(&doc{id: "a", name: "A"})

	// Ownership transfer: the exact value handed in must be stored, no clone.
	transferred := &doc{id: "a", name: "A2"}
	if err := m.ReplaceActiveNoClone(transferred); err != nil {
		t.Fatalf("ReplaceActiveNoClone: %v", err)
	}
	if got := m.ActiveMut(); got != transferred {
		t.Fatalf("stored pointer = %p, want the transferred value %p (must NOT clone)", got, transferred)
	}

	// It must reject values without an id, like ReplaceActive.
	if err := m.ReplaceActiveNoClone(&doc{id: ""}); err == nil {
		t.Fatal("expected ReplaceActiveNoClone to reject an empty id")
	}
}

func TestManagerReplaceActiveNoCloneWithoutActiveCreates(t *testing.T) {
	m := newDocManager()
	if err := m.ReplaceActiveNoClone(&doc{id: "a", name: "A"}); err != nil {
		t.Fatalf("ReplaceActiveNoClone on empty manager: %v", err)
	}
	if m.ActiveID() != "a" {
		t.Fatalf("active id = %q, want %q", m.ActiveID(), "a")
	}
	if got := m.ActiveMut(); got == nil || got.name != "A" {
		t.Fatalf("stored value = %+v, want name A", got)
	}
}
