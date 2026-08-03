package runtime

import (
	"errors"
	"testing"
)

// testCtx is the mutable context threaded through history commands in tests.
type testCtx struct {
	value int
}

// stubCommand is a Command[*testCtx] whose Apply/Undo behaviour (and failure) is
// fully controllable, so failure semantics can be asserted deterministically.
type stubCommand struct {
	desc     string
	applyErr error
	undoErr  error
	applied  *int
	undone   *int
}

func (c *stubCommand) Apply(ctx *testCtx) error {
	if c.applyErr != nil {
		return c.applyErr
	}
	if c.applied != nil {
		*c.applied++
	}
	ctx.value++
	return nil
}

func (c *stubCommand) Undo(ctx *testCtx) error {
	if c.undoErr != nil {
		return c.undoErr
	}
	if c.undone != nil {
		*c.undone++
	}
	ctx.value--
	return nil
}

func (c *stubCommand) Description() string { return c.desc }

func newTestStack() *HistoryStack[*testCtx, int] {
	return NewHistoryStack[*testCtx, int](
		100,
		func(ctx *testCtx) int { return ctx.value },
		func(ctx *testCtx, s int) error { ctx.value = s; return nil },
		func(a, b int) bool { return a == b },
	)
}

func TestUndoFailureLeavesStacksConsistent(t *testing.T) {
	ctx := &testCtx{}
	h := newTestStack()

	first := &stubCommand{desc: "first"}
	second := &stubCommand{desc: "second"}
	h.Push(first)
	h.Push(second)

	// Make the top command's Undo fail.
	second.undoErr = errors.New("undo boom")

	if err := h.Undo(ctx); err == nil {
		t.Fatal("Undo should return the command's error")
	}
	// Both stacks must be exactly as before: entry not lost, nothing moved.
	if got := h.CurrentIndex(); got != 2 {
		t.Fatalf("undo stack depth = %d after failed Undo, want 2", got)
	}
	if h.CanRedo() {
		t.Fatal("redo stack must stay empty after failed Undo")
	}

	// A subsequent successful Undo must still work (entry was not lost).
	second.undoErr = nil
	if err := h.Undo(ctx); err != nil {
		t.Fatalf("Undo after clearing error: %v", err)
	}
	if got := h.CurrentIndex(); got != 1 {
		t.Fatalf("undo stack depth = %d after successful Undo, want 1", got)
	}
	if !h.CanRedo() {
		t.Fatal("redo stack should contain the undone entry")
	}
}

func TestRedoFailureLeavesStacksConsistent(t *testing.T) {
	ctx := &testCtx{}
	h := newTestStack()

	only := &stubCommand{desc: "only"}
	h.Push(only)
	if err := h.Undo(ctx); err != nil {
		t.Fatalf("initial Undo: %v", err)
	}
	// Now: undo empty, redo has one entry.
	if h.CanUndo() || !h.CanRedo() {
		t.Fatalf("precondition wrong: canUndo=%v canRedo=%v", h.CanUndo(), h.CanRedo())
	}

	only.applyErr = errors.New("apply boom")
	if err := h.Redo(ctx); err == nil {
		t.Fatal("Redo should return the command's error")
	}
	// Redo entry must not be lost, undo must stay empty.
	if h.CanUndo() {
		t.Fatal("undo stack must stay empty after failed Redo")
	}
	if !h.CanRedo() {
		t.Fatal("redo entry must not be lost after failed Redo")
	}

	only.applyErr = nil
	if err := h.Redo(ctx); err != nil {
		t.Fatalf("Redo after clearing error: %v", err)
	}
	if !h.CanUndo() || h.CanRedo() {
		t.Fatalf("after successful Redo: canUndo=%v canRedo=%v", h.CanUndo(), h.CanRedo())
	}
}

func TestJumpToStopsAtFirstFailure(t *testing.T) {
	ctx := &testCtx{}
	h := newTestStack()

	a := &stubCommand{desc: "a"}
	b := &stubCommand{desc: "b"}
	c := &stubCommand{desc: "c"}
	h.Push(a)
	h.Push(b)
	h.Push(c)
	// undo depth = 3.

	// Jumping back to index 0 undoes c, then b, then a. Make b fail so the jump
	// stops after undoing c, with stacks left consistent.
	b.undoErr = errors.New("undo boom")
	if err := h.JumpTo(ctx, 0); err == nil {
		t.Fatal("JumpTo should surface the failing Undo error")
	}
	// c was undone successfully (moved to redo); b and a remain on undo.
	if got := h.CurrentIndex(); got != 2 {
		t.Fatalf("undo depth = %d after halted JumpTo, want 2 (a,b remain)", got)
	}
	if !h.CanRedo() {
		t.Fatal("c should have moved to the redo stack before the failure")
	}

	// Clearing the fault lets the jump complete.
	b.undoErr = nil
	if err := h.JumpTo(ctx, 0); err != nil {
		t.Fatalf("JumpTo after clearing error: %v", err)
	}
	if h.CurrentIndex() != 0 {
		t.Fatalf("undo depth = %d after completed JumpTo, want 0", h.CurrentIndex())
	}
}

func TestCancelTransactionRevertsToPreTransactionState(t *testing.T) {
	ctx := &testCtx{}
	h := newTestStack()

	// Build one committed entry, then undo it so a redo entry exists.
	if err := h.Execute(ctx, &stubCommand{desc: "edit"}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if err := h.Undo(ctx); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	if ctx.value != 0 || h.CanUndo() || !h.CanRedo() {
		t.Fatalf("precondition wrong: value=%d canUndo=%v canRedo=%v", ctx.value, h.CanUndo(), h.CanRedo())
	}

	h.BeginTransaction(ctx, "gesture")
	if err := h.Execute(ctx, &stubCommand{desc: "in-txn"}); err != nil {
		t.Fatalf("Execute in transaction: %v", err)
	}
	if ctx.value != 1 {
		t.Fatalf("value during transaction = %d, want 1", ctx.value)
	}

	if err := h.CancelTransaction(ctx); err != nil {
		t.Fatalf("CancelTransaction: %v", err)
	}
	if ctx.value != 0 {
		t.Fatalf("value after cancel = %d, want 0 (restored to pre-transaction state)", ctx.value)
	}
	// Undo/redo stacks must be untouched: cancel restored the exact head state
	// the redo entry was recorded against, so it stays valid.
	if h.CanUndo() {
		t.Fatal("cancel must not add undo entries")
	}
	if !h.CanRedo() {
		t.Fatal("cancel must not clear the redo stack")
	}

	// A later EndTransaction must be a no-op: the transaction is gone.
	h.EndTransaction(true)
	if h.CanUndo() {
		t.Fatal("EndTransaction after cancel must not push an entry")
	}

	// The surviving redo entry still replays correctly.
	if err := h.Redo(ctx); err != nil {
		t.Fatalf("Redo after cancel: %v", err)
	}
	if ctx.value != 1 {
		t.Fatalf("value after redo = %d, want 1", ctx.value)
	}
}

func TestCancelTransactionWithoutActiveIsNoOp(t *testing.T) {
	ctx := &testCtx{value: 5}
	h := newTestStack()
	h.Push(&stubCommand{desc: "existing"})

	if err := h.CancelTransaction(ctx); err != nil {
		t.Fatalf("CancelTransaction without active transaction: %v", err)
	}
	if ctx.value != 5 {
		t.Fatalf("value = %d after no-op cancel, want 5 (untouched)", ctx.value)
	}
	if !h.CanUndo() || h.CanRedo() {
		t.Fatalf("stacks changed by no-op cancel: canUndo=%v canRedo=%v", h.CanUndo(), h.CanRedo())
	}
}

func TestCancelTransactionKeepsTransactionOnRestoreFailure(t *testing.T) {
	restoreErr := errors.New("restore boom")
	fail := true
	ctx := &testCtx{}
	h := NewHistoryStack[*testCtx, int](
		100,
		func(ctx *testCtx) int { return ctx.value },
		func(ctx *testCtx, s int) error {
			if fail {
				return restoreErr
			}
			ctx.value = s
			return nil
		},
		func(a, b int) bool { return a == b },
	)

	h.BeginTransaction(ctx, "gesture")
	ctx.value = 3

	if err := h.CancelTransaction(ctx); !errors.Is(err, restoreErr) {
		t.Fatalf("CancelTransaction error = %v, want %v", err, restoreErr)
	}
	// The transaction must survive a failed restore so the caller can retry.
	fail = false
	if err := h.CancelTransaction(ctx); err != nil {
		t.Fatalf("retried CancelTransaction: %v", err)
	}
	if ctx.value != 0 {
		t.Fatalf("value after retried cancel = %d, want 0", ctx.value)
	}
}

func TestExecuteSuppressesNoOpCommands(t *testing.T) {
	tests := []struct {
		name       string
		applyFn    func(ctx *testCtx) (int, error)
		wantPushed bool
	}{
		{
			name:       "no-op leaves snapshots equal",
			applyFn:    func(ctx *testCtx) (int, error) { return ctx.value, nil },
			wantPushed: false,
		},
		{
			name:       "real change differs",
			applyFn:    func(ctx *testCtx) (int, error) { ctx.value++; return ctx.value, nil },
			wantPushed: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := &testCtx{value: 7}
			h := newTestStack()
			cmd := NewSnapshotCommand[*testCtx, int](
				tc.name,
				tc.applyFn,
				func(ctx *testCtx) int { return ctx.value },
				func(ctx *testCtx, s int) error { ctx.value = s; return nil },
			)
			if err := h.Execute(ctx, cmd); err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if got := h.CanUndo(); got != tc.wantPushed {
				t.Fatalf("CanUndo = %v after Execute, want %v (pushed)", got, tc.wantPushed)
			}
		})
	}
}

func TestRevisionIdentifiesUndoRedoAndBranches(t *testing.T) {
	ctx := &testCtx{}
	h := newTestStack()
	initial := h.Revision()

	if err := h.Execute(ctx, &stubCommand{desc: "first"}); err != nil {
		t.Fatalf("Execute first: %v", err)
	}
	first := h.Revision()
	if first == initial {
		t.Fatal("an edit must advance the revision")
	}
	if err := h.Undo(ctx); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	if got := h.Revision(); got != initial {
		t.Fatalf("revision after undo = %d, want initial %d", got, initial)
	}
	if err := h.Redo(ctx); err != nil {
		t.Fatalf("Redo: %v", err)
	}
	if got := h.Revision(); got != first {
		t.Fatalf("revision after redo = %d, want original edited revision %d", got, first)
	}

	if err := h.Undo(ctx); err != nil {
		t.Fatalf("second Undo: %v", err)
	}
	if err := h.Execute(ctx, &stubCommand{desc: "branch"}); err != nil {
		t.Fatalf("Execute branch: %v", err)
	}
	if got := h.Revision(); got == first || got == initial {
		t.Fatalf("branched edit reused revision %d", got)
	}
}

func TestClearKeepsCurrentRevision(t *testing.T) {
	ctx := &testCtx{}
	h := newTestStack()
	if err := h.Execute(ctx, &stubCommand{desc: "edit"}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	want := h.Revision()
	h.Clear()
	if got := h.Revision(); got != want {
		t.Fatalf("revision after Clear = %d, want %d", got, want)
	}
	if h.CanUndo() || h.CanRedo() {
		t.Fatal("Clear must empty undo and redo stacks")
	}
}
