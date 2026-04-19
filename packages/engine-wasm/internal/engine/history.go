package engine

import runtimepkg "github.com/cwbudde/agogo-web/packages/engine-wasm/internal/runtime"

type snapshot struct {
	DocumentID string
	Document   *Document
	Viewport   ViewportState
}

type Command = runtimepkg.Command[*instance]

type snapshotCommand struct {
	*runtimepkg.SnapshotCommand[*instance, snapshot]
}

func newSnapshotCommand(description string, applyFn func(*instance) (snapshot, error)) *snapshotCommand {
	return &snapshotCommand{
		SnapshotCommand: runtimepkg.NewSnapshotCommand(
			description,
			applyFn,
			(*instance).captureSnapshot,
			(*instance).restoreSnapshot,
		),
	}
}

type HistoryStack struct {
	*runtimepkg.HistoryStack[*instance, snapshot]
}

func newHistoryStack(maxDepth int) *HistoryStack {
	return &HistoryStack{
		HistoryStack: runtimepkg.NewHistoryStack(
			maxDepth,
			(*instance).captureSnapshot,
			(*instance).restoreSnapshot,
			snapshotsEqual,
		),
	}
}

func (h *HistoryStack) Clone() *HistoryStack {
	if h == nil {
		return nil
	}
	return &HistoryStack{HistoryStack: h.HistoryStack.Clone()}
}

func (h *HistoryStack) Entries() []HistoryEntry {
	runtimeEntries := h.HistoryStack.Entries()
	entries := make([]HistoryEntry, 0, len(runtimeEntries))
	for _, entry := range runtimeEntries {
		entries = append(entries, HistoryEntry{
			ID:          entry.ID,
			Description: entry.Description,
			State:       entry.State,
		})
	}
	return entries
}

func (h *HistoryStack) SnapshotAt(historyIndex int) (snapshot, bool) {
	return h.HistoryStack.SnapshotAt(historyIndex)
}

func (h *HistoryStack) push(command Command) {
	h.Push(command)
}

func (h *HistoryStack) SnapshotAtIndex(inst *instance, historyIndex int) (snapshot, bool) {
	total := len(h.HistoryStack.Entries())
	if historyIndex <= 0 || historyIndex > total || inst == nil {
		return snapshot{}, false
	}
	active := inst.manager.Active()
	if active == nil {
		return snapshot{}, false
	}
	cloneHistory := h.Clone()
	cloneInst := &instance{
		manager:  newDocumentManager(),
		viewport: inst.viewport,
		history:  cloneHistory,
	}
	cloneInst.manager.Create(active)
	if inst.manager.ActiveID() != "" {
		if err := cloneInst.manager.SetActiveID(inst.manager.ActiveID()); err != nil {
			return snapshot{}, false
		}
	}
	if err := cloneHistory.JumpTo(cloneInst, historyIndex); err != nil {
		return snapshot{}, false
	}
	return cloneInst.captureSnapshot(), true
}

func (h *HistoryStack) PreviousSnapshot(inst *instance) (snapshot, bool) {
	if !h.CanUndo() || inst == nil {
		return snapshot{}, false
	}
	active := inst.manager.Active()
	if active == nil {
		return snapshot{}, false
	}

	cloneInst := &instance{
		manager:  newDocumentManager(),
		viewport: inst.viewport,
		history:  newHistoryStack(defaultHistoryMax),
	}
	cloneInst.manager.Create(active)
	if inst.manager.ActiveID() != "" {
		if err := cloneInst.manager.SetActiveID(inst.manager.ActiveID()); err != nil {
			return snapshot{}, false
		}
	}
	if err := h.Undo(cloneInst); err != nil {
		return snapshot{}, false
	}
	return cloneInst.captureSnapshot(), true
}
