package runtime

type HistoryEntry struct {
	ID          int64
	Description string
	State       string
}

type Command[C any] interface {
	Apply(C) error
	Undo(C) error
	Description() string
}

type SnapshotCommand[C any, S any] struct {
	description string
	before      S
	after       S
	applyFn     func(C) (S, error)
	capture     func(C) S
	restore     func(C, S) error
}

func NewSnapshotCommand[C any, S any](
	description string,
	applyFn func(C) (S, error),
	capture func(C) S,
	restore func(C, S) error,
) *SnapshotCommand[C, S] {
	return &SnapshotCommand[C, S]{
		description: description,
		applyFn:     applyFn,
		capture:     capture,
		restore:     restore,
	}
}

func (c *SnapshotCommand[C, S]) Apply(ctx C) error {
	if c.applyFn != nil {
		before := c.capture(ctx)
		after, err := c.applyFn(ctx)
		if err != nil {
			return err
		}
		c.before = before
		c.after = after
		c.applyFn = nil
		return nil
	}
	return c.restore(ctx, c.after)
}

func (c *SnapshotCommand[C, S]) Undo(ctx C) error {
	return c.restore(ctx, c.before)
}

func (c *SnapshotCommand[C, S]) Description() string {
	return c.description
}

func (c *SnapshotCommand[C, S]) After() S {
	return c.after
}

type groupedCommand[C any, S any] struct {
	description string
	before      S
	after       S
	restore     func(C, S) error
}

func (c *groupedCommand[C, S]) Apply(ctx C) error {
	return c.restore(ctx, c.after)
}

func (c *groupedCommand[C, S]) Undo(ctx C) error {
	return c.restore(ctx, c.before)
}

func (c *groupedCommand[C, S]) Description() string {
	return c.description
}

func (c *groupedCommand[C, S]) After() S {
	return c.after
}

type HistoryStack[C any, S any] struct {
	undo     []Command[C]
	redo     []Command[C]
	maxDepth int
	active   *groupedCommand[C, S]
	capture  func(C) S
	restore  func(C, S) error
	equal    func(S, S) bool
}

func NewHistoryStack[C any, S any](
	maxDepth int,
	capture func(C) S,
	restore func(C, S) error,
	equal func(S, S) bool,
) *HistoryStack[C, S] {
	return &HistoryStack[C, S]{
		maxDepth: maxDepth,
		capture:  capture,
		restore:  restore,
		equal:    equal,
	}
}

func (h *HistoryStack[C, S]) Clone() *HistoryStack[C, S] {
	if h == nil {
		return nil
	}
	return &HistoryStack[C, S]{
		undo:     append([]Command[C](nil), h.undo...),
		redo:     append([]Command[C](nil), h.redo...),
		maxDepth: h.maxDepth,
		capture:  h.capture,
		restore:  h.restore,
		equal:    h.equal,
	}
}

func (h *HistoryStack[C, S]) Execute(ctx C, command Command[C]) error {
	if err := command.Apply(ctx); err != nil {
		return err
	}
	if h.active != nil {
		h.active.after = h.capture(ctx)
		return nil
	}
	h.push(command)
	return nil
}

func (h *HistoryStack[C, S]) BeginTransaction(ctx C, description string) {
	if h.active != nil {
		return
	}
	state := h.capture(ctx)
	h.active = &groupedCommand[C, S]{
		description: description,
		before:      state,
		after:       state,
		restore:     h.restore,
	}
}

func (h *HistoryStack[C, S]) EndTransaction(commit bool) {
	if h.active == nil {
		return
	}
	active := h.active
	h.active = nil
	if !commit || h.equal(active.before, active.after) {
		return
	}
	h.push(active)
}

func (h *HistoryStack[C, S]) push(command Command[C]) {
	h.undo = append(h.undo, command)
	if len(h.undo) > h.maxDepth {
		h.undo = h.undo[len(h.undo)-h.maxDepth:]
	}
	h.redo = h.redo[:0]
}

func (h *HistoryStack[C, S]) Push(command Command[C]) {
	h.push(command)
}

func (h *HistoryStack[C, S]) Undo(ctx C) error {
	if len(h.undo) == 0 {
		return nil
	}
	command := h.undo[len(h.undo)-1]
	h.undo = h.undo[:len(h.undo)-1]
	if err := command.Undo(ctx); err != nil {
		return err
	}
	h.redo = append(h.redo, command)
	return nil
}

func (h *HistoryStack[C, S]) Redo(ctx C) error {
	if len(h.redo) == 0 {
		return nil
	}
	command := h.redo[len(h.redo)-1]
	h.redo = h.redo[:len(h.redo)-1]
	if err := command.Apply(ctx); err != nil {
		return err
	}
	h.undo = append(h.undo, command)
	return nil
}

func (h *HistoryStack[C, S]) Entries() []HistoryEntry {
	entries := make([]HistoryEntry, 0, len(h.undo)+len(h.redo))
	for i, command := range h.undo {
		state := "done"
		if i == len(h.undo)-1 {
			state = "current"
		}
		entries = append(entries, HistoryEntry{
			ID:          int64(i + 1),
			Description: command.Description(),
			State:       state,
		})
	}
	for i := len(h.redo) - 1; i >= 0; i-- {
		command := h.redo[i]
		entries = append(entries, HistoryEntry{
			ID:          int64(len(entries) + 1),
			Description: command.Description(),
			State:       "undone",
		})
	}
	return entries
}

func (h *HistoryStack[C, S]) CurrentIndex() int {
	return len(h.undo)
}

func (h *HistoryStack[C, S]) SnapshotAt(historyIndex int) (S, bool) {
	var zero S
	if historyIndex < 0 || historyIndex > len(h.undo) || historyIndex == 0 {
		return zero, false
	}
	command := h.undo[historyIndex-1]
	afterCarrier, ok := command.(interface{ After() S })
	if !ok {
		return zero, false
	}
	return afterCarrier.After(), true
}

func (h *HistoryStack[C, S]) CanUndo() bool { return len(h.undo) > 0 }
func (h *HistoryStack[C, S]) CanRedo() bool { return len(h.redo) > 0 }

func (h *HistoryStack[C, S]) Clear() {
	h.undo = nil
	h.redo = nil
	h.active = nil
}

func (h *HistoryStack[C, S]) JumpTo(ctx C, historyIndex int) error {
	total := len(h.undo) + len(h.redo)
	if historyIndex < 0 {
		historyIndex = 0
	}
	if historyIndex > total {
		historyIndex = total
	}
	for len(h.undo) > historyIndex {
		if err := h.Undo(ctx); err != nil {
			return err
		}
	}
	for len(h.undo) < historyIndex {
		if err := h.Redo(ctx); err != nil {
			return err
		}
	}
	return nil
}
