package runtime

import "fmt"

type Manager[T any] struct {
	docs     map[string]T
	order    []string
	activeID string
	clone    func(T) T
	idOf     func(T) string
}

func NewManager[T any](clone func(T) T, idOf func(T) string) *Manager[T] {
	return &Manager[T]{
		docs:  make(map[string]T),
		clone: clone,
		idOf:  idOf,
	}
}

func (m *Manager[T]) Create(value T) {
	id := m.idOf(value)
	if _, exists := m.docs[id]; !exists {
		m.order = append(m.order, id)
	}
	m.docs[id] = m.clone(value)
	m.activeID = id
}

// Replace stores value under its own id, cloning it, while preserving the
// existing iteration order and every other stored value. If the id is not yet
// present the value is appended to the order (re-inserting a previously removed
// entry at the end). The active id is left unchanged; callers that want the
// replaced value to become active must call SetActiveID separately.
func (m *Manager[T]) Replace(value T) error {
	id := m.idOf(value)
	if id == "" {
		return fmt.Errorf("value id is required")
	}
	if _, exists := m.docs[id]; !exists {
		m.order = append(m.order, id)
	}
	m.docs[id] = m.clone(value)
	return nil
}

// IDs returns the ids of all stored values in their current iteration order.
// The returned slice is a copy and is safe for the caller to retain or mutate.
func (m *Manager[T]) IDs() []string {
	return append([]string(nil), m.order...)
}

func (m *Manager[T]) ReplaceActive(value T) error {
	id := m.idOf(value)
	if id == "" {
		return fmt.Errorf("value id is required")
	}
	if m.activeID == "" {
		m.Create(value)
		return nil
	}
	m.docs[m.activeID] = m.clone(value)
	return nil
}

// ReplaceActiveNoClone stores value under the active id WITHOUT cloning it.
// This is an ownership transfer: the caller must hand over an exclusively-owned
// value (e.g. a working copy obtained from Active(), which clones) and must not
// retain or mutate it afterwards — the manager becomes the sole owner. Callers
// that cannot guarantee exclusive ownership must use ReplaceActive instead.
//
// It exists so that snapshot-based commands can install their already-private
// working copy without paying a second deep clone (see the pointer-snapshot
// history design in internal/engine/state.go).
func (m *Manager[T]) ReplaceActiveNoClone(value T) error {
	id := m.idOf(value)
	if id == "" {
		return fmt.Errorf("value id is required")
	}
	if m.activeID == "" {
		m.Create(value)
		return nil
	}
	m.docs[m.activeID] = value
	return nil
}

func (m *Manager[T]) Active() T {
	var zero T
	if m.activeID == "" {
		return zero
	}
	value, ok := m.docs[m.activeID]
	if !ok {
		return zero
	}
	return m.clone(value)
}

// Get returns a clone of the value identified by id.
func (m *Manager[T]) Get(id string) T {
	var zero T
	value, ok := m.docs[id]
	if !ok {
		return zero
	}
	return m.clone(value)
}

func (m *Manager[T]) ActiveMut() T {
	var zero T
	if m.activeID == "" {
		return zero
	}
	return m.docs[m.activeID]
}

func (m *Manager[T]) ActiveID() string {
	return m.activeID
}

func (m *Manager[T]) SetActiveID(id string) error {
	if id == "" {
		m.activeID = ""
		return nil
	}
	if _, ok := m.docs[id]; !ok {
		return fmt.Errorf("document %q not found", id)
	}
	m.activeID = id
	return nil
}

func (m *Manager[T]) Switch(id string) error {
	return m.SetActiveID(id)
}

func (m *Manager[T]) CloseActive() error {
	return m.Close(m.activeID)
}

// Close removes id while preserving the remaining order. When the active
// value is closed, the value immediately to its left becomes active; if there
// is no left neighbor the first value to its right is used.
func (m *Manager[T]) Close(id string) error {
	if id == "" {
		return nil
	}
	if _, ok := m.docs[id]; !ok {
		return fmt.Errorf("document %q not found", id)
	}
	closedIndex := -1
	for i, candidate := range m.order {
		if candidate == id {
			closedIndex = i
			break
		}
	}
	delete(m.docs, id)
	nextOrder := make([]string, 0, len(m.order))
	for _, candidate := range m.order {
		if candidate != id {
			nextOrder = append(nextOrder, candidate)
		}
	}
	m.order = nextOrder
	if len(m.order) == 0 {
		m.activeID = ""
		return nil
	}
	if m.activeID != id {
		return nil
	}
	if closedIndex > 0 {
		m.activeID = m.order[closedIndex-1]
	} else {
		m.activeID = m.order[0]
	}
	return nil
}
