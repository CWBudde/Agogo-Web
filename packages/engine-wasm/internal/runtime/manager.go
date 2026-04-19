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
	if m.activeID == "" {
		return nil
	}
	delete(m.docs, m.activeID)
	nextOrder := make([]string, 0, len(m.order))
	for _, id := range m.order {
		if id != m.activeID {
			nextOrder = append(nextOrder, id)
		}
	}
	m.order = nextOrder
	if len(m.order) == 0 {
		m.activeID = ""
		return nil
	}
	m.activeID = m.order[len(m.order)-1]
	return nil
}
