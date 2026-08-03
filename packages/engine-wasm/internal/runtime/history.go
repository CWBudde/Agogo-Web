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

func (c *SnapshotCommand[C, S]) Before() S {
	return c.before
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
	undo            []Command[C]
	redo            []Command[C]
	undoRevisions   []uint64
	redoRevisions   []uint64
	baseRevision    uint64
	currentRevision uint64
	nextRevision    uint64
	maxDepth        int
	active          *groupedCommand[C, S]
	capture         func(C) S
	restore         func(C, S) error
	equal           func(S, S) bool
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
		undo:            append([]Command[C](nil), h.undo...),
		redo:            append([]Command[C](nil), h.redo...),
		undoRevisions:   append([]uint64(nil), h.undoRevisions...),
		redoRevisions:   append([]uint64(nil), h.redoRevisions...),
		baseRevision:    h.baseRevision,
		currentRevision: h.currentRevision,
		nextRevision:    h.nextRevision,
		maxDepth:        h.maxDepth,
		capture:         h.capture,
		restore:         h.restore,
		equal:           h.equal,
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
	// Suppress no-op commands: if the command exposes both its before and after
	// snapshots and they compare equal, applying it changed nothing meaningful,
	// so pushing it would only pollute the history (e.g. renaming a layer to its
	// current name, or a viewport command that resolves to the same view). This
	// mirrors the no-op suppression EndTransaction already performs for grouped
	// commands. Commands that do not expose both snapshots are always pushed.
	if h.equal != nil {
		if before, after, ok := commandSnapshots[C, S](command); ok && h.equal(before, after) {
			return nil
		}
	}
	h.push(command)
	return nil
}

// commandSnapshots extracts a command's before/after snapshots when it exposes
// both accessors (as SnapshotCommand does). It returns ok=false for commands
// that cannot describe their effect via snapshots (e.g. delta commands), which
// are then always pushed.
func commandSnapshots[C any, S any](command Command[C]) (before, after S, ok bool) {
	beforeCarrier, hasBefore := command.(interface{ Before() S })
	afterCarrier, hasAfter := command.(interface{ After() S })
	if !hasBefore || !hasAfter {
		return before, after, false
	}
	return beforeCarrier.Before(), afterCarrier.After(), true
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

// EndTransaction closes the active transaction. With commit=false it only
// discards the grouped history entry — mutations made during the transaction
// REMAIN in the live state. To revert them, use CancelTransaction instead.
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

// CancelTransaction aborts the active transaction and reverts the live state
// to the snapshot recorded by BeginTransaction. Unlike EndTransaction(false) —
// which only discards the grouped history entry and keeps whatever mutations
// happened during the transaction — this undoes those mutations. The undo and
// redo stacks are deliberately left untouched: the restored state is exactly
// the head state they were recorded against, so existing redo entries still
// replay onto the correct base. Calling it with no active transaction is a
// no-op.
func (h *HistoryStack[C, S]) CancelTransaction(ctx C) error {
	if h.active == nil {
		return nil
	}
	if err := h.restore(ctx, h.active.before); err != nil {
		// Leave the transaction active on failure so the caller can retry;
		// discarding it here would silently orphan the live mutations.
		return err
	}
	h.active = nil
	return nil
}

func (h *HistoryStack[C, S]) push(command Command[C]) {
	h.nextRevision++
	revision := h.nextRevision
	h.undo = append(h.undo, command)
	h.undoRevisions = append(h.undoRevisions, revision)
	if len(h.undo) > h.maxDepth {
		dropped := len(h.undo) - h.maxDepth
		h.baseRevision = h.undoRevisions[dropped-1]
		h.undo = h.undo[dropped:]
		h.undoRevisions = h.undoRevisions[dropped:]
	}
	h.redo = h.redo[:0]
	h.redoRevisions = h.redoRevisions[:0]
	h.currentRevision = revision
}

func (h *HistoryStack[C, S]) Push(command Command[C]) {
	h.push(command)
}

// ClearRedo discards the redo stack while leaving the undo stack intact. Use it
// when the document is mutated outside the normal Execute/Undo/Redo path so that
// the recorded history diverges from the live document: a surviving redo entry
// would replay onto a different base state and corrupt the document.
func (h *HistoryStack[C, S]) ClearRedo() {
	h.redo = h.redo[:0]
	h.redoRevisions = h.redoRevisions[:0]
	h.nextRevision++
	h.currentRevision = h.nextRevision
}

// Undo reverses the most recent command. The command is only moved from the undo
// stack to the redo stack after its Undo succeeds; on failure both stacks are
// left exactly as they were and the error is returned. This keeps the history
// consistent when a restore fails (the entry is not lost and can be retried).
func (h *HistoryStack[C, S]) Undo(ctx C) error {
	if len(h.undo) == 0 {
		return nil
	}
	command := h.undo[len(h.undo)-1]
	revision := h.undoRevisions[len(h.undoRevisions)-1]
	if err := command.Undo(ctx); err != nil {
		return err
	}
	h.undo = h.undo[:len(h.undo)-1]
	h.undoRevisions = h.undoRevisions[:len(h.undoRevisions)-1]
	h.redo = append(h.redo, command)
	h.redoRevisions = append(h.redoRevisions, revision)
	if len(h.undoRevisions) == 0 {
		h.currentRevision = h.baseRevision
	} else {
		h.currentRevision = h.undoRevisions[len(h.undoRevisions)-1]
	}
	return nil
}

// Redo re-applies the most recently undone command. As with Undo, the entry is
// only moved between stacks after Apply succeeds; on failure both stacks are left
// unchanged and the error is returned.
func (h *HistoryStack[C, S]) Redo(ctx C) error {
	if len(h.redo) == 0 {
		return nil
	}
	command := h.redo[len(h.redo)-1]
	revision := h.redoRevisions[len(h.redoRevisions)-1]
	if err := command.Apply(ctx); err != nil {
		return err
	}
	h.redo = h.redo[:len(h.redo)-1]
	h.redoRevisions = h.redoRevisions[:len(h.redoRevisions)-1]
	h.undo = append(h.undo, command)
	h.undoRevisions = append(h.undoRevisions, revision)
	h.currentRevision = revision
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

// Revision identifies the current logical document state. Unlike a render
// cache version, undo followed by redo returns to the same revision.
func (h *HistoryStack[C, S]) Revision() uint64 { return h.currentRevision }

func (h *HistoryStack[C, S]) Clear() {
	h.undo = nil
	h.redo = nil
	h.undoRevisions = nil
	h.redoRevisions = nil
	h.baseRevision = h.currentRevision
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
	// Each Undo/Redo step is atomic: on failure it leaves both stacks unchanged.
	// Stopping at the first error therefore leaves the history in a consistent
	// state (the successfully replayed steps have moved, the failing one has not).
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
