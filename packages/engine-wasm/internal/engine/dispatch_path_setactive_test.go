package engine

import "testing"

// setActivePathPayload is the JSON payload for commandSetActivePath.
type setActivePathPayload struct {
	PathIndex int `json:"pathIndex"`
}

func TestSetActivePathDispatch(t *testing.T) {
	tests := []struct {
		name      string
		pathIndex int
		wantErr   bool
	}{
		{"activate first path", 0, false},
		{"activate second path", 1, false},
		{"negative index", -1, true},
		{"index beyond range", 2, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := initWithDefaultDoc(t)
			defer Free(h)

			// Create two paths; the newest (index 1) becomes active.
			var result RenderResult
			var err error
			for _, name := range []string{"P0", "P1"} {
				result, err = DispatchCommand(h, commandCreatePath, mustJSON(t, CreatePathPayload{Name: name}))
				if err != nil {
					t.Fatalf("create %s: %v", name, err)
				}
			}
			historyBefore := len(result.UIMeta.History)

			result, err = DispatchCommand(h, commandSetActivePath, mustJSON(t, setActivePathPayload{PathIndex: tt.pathIndex}))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("SetActivePath(%d): expected error, got nil", tt.pathIndex)
				}
				return
			}
			if err != nil {
				t.Fatalf("SetActivePath(%d): %v", tt.pathIndex, err)
			}

			// The stored document's ActivePathIdx must change.
			inst, ok := instances[h]
			if !ok {
				t.Fatalf("no instance for handle %d", h)
			}
			if got := inst.manager.activeMut().ActivePathIdx; got != tt.pathIndex {
				t.Errorf("ActivePathIdx = %d, want %d", got, tt.pathIndex)
			}

			// UIMeta must mark exactly the requested path as active.
			for i, p := range result.UIMeta.Paths {
				if p.Active != (i == tt.pathIndex) {
					t.Errorf("path %d (%s): active = %v, want active only at index %d", i, p.Name, p.Active, tt.pathIndex)
				}
			}

			// Activation is not undoable — no new history entry (Photoshop semantics).
			if got := len(result.UIMeta.History); got != historyBefore {
				t.Errorf("history length = %d, want %d (activation must not create a history entry)", got, historyBefore)
			}
		})
	}
}
