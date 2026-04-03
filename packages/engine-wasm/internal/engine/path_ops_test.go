package engine

import "testing"

func TestDocumentPathCRUD(t *testing.T) {
	h := initWithDefaultDoc(t)
	defer Free(h)

	// Create a path named "Shape 1".
	result, err := DispatchCommand(h, commandCreatePath, mustJSON(t, CreatePathPayload{Name: "Shape 1"}))
	if err != nil {
		t.Fatalf("create path: %v", err)
	}
	if len(result.UIMeta.Paths) != 1 {
		t.Fatalf("expected 1 path, got %d", len(result.UIMeta.Paths))
	}
	if result.UIMeta.Paths[0].Name != "Shape 1" {
		t.Fatalf("expected path name %q, got %q", "Shape 1", result.UIMeta.Paths[0].Name)
	}
	if !result.UIMeta.Paths[0].Active {
		t.Fatal("expected newly created path to be active")
	}

	// Rename it to "Outline".
	result, err = DispatchCommand(h, commandRenamePath, mustJSON(t, RenamePathPayload{PathIndex: 0, Name: "Outline"}))
	if err != nil {
		t.Fatalf("rename path: %v", err)
	}
	if result.UIMeta.Paths[0].Name != "Outline" {
		t.Fatalf("expected path name %q, got %q", "Outline", result.UIMeta.Paths[0].Name)
	}

	// Duplicate it.
	result, err = DispatchCommand(h, commandDuplicatePath, mustJSON(t, DuplicatePathPayload{PathIndex: 0}))
	if err != nil {
		t.Fatalf("duplicate path: %v", err)
	}
	if len(result.UIMeta.Paths) != 2 {
		t.Fatalf("expected 2 paths, got %d", len(result.UIMeta.Paths))
	}
	if result.UIMeta.Paths[1].Name != "Outline copy" {
		t.Fatalf("expected duplicate name %q, got %q", "Outline copy", result.UIMeta.Paths[1].Name)
	}
	if !result.UIMeta.Paths[1].Active {
		t.Fatal("expected duplicated path to be active")
	}
	if result.UIMeta.Paths[0].Active {
		t.Fatal("expected original path to be inactive after duplicate")
	}

	// Delete the copy (index 1).
	result, err = DispatchCommand(h, commandDeletePath, mustJSON(t, DeletePathPayload{PathIndex: 1}))
	if err != nil {
		t.Fatalf("delete path: %v", err)
	}
	if len(result.UIMeta.Paths) != 1 {
		t.Fatalf("expected 1 path after delete, got %d", len(result.UIMeta.Paths))
	}
	if result.UIMeta.Paths[0].Name != "Outline" {
		t.Fatalf("expected remaining path %q, got %q", "Outline", result.UIMeta.Paths[0].Name)
	}

	// Undo should restore the deleted path.
	result, err = DispatchCommand(h, commandUndo, "")
	if err != nil {
		t.Fatalf("undo: %v", err)
	}
	if len(result.UIMeta.Paths) != 2 {
		t.Fatalf("expected 2 paths after undo, got %d", len(result.UIMeta.Paths))
	}
}

func TestCreatePathDefaultName(t *testing.T) {
	h := initWithDefaultDoc(t)
	defer Free(h)

	// Create with empty name — should get auto-name.
	result, err := DispatchCommand(h, commandCreatePath, mustJSON(t, CreatePathPayload{}))
	if err != nil {
		t.Fatalf("create path: %v", err)
	}
	if result.UIMeta.Paths[0].Name != "Path 1" {
		t.Fatalf("expected auto-name %q, got %q", "Path 1", result.UIMeta.Paths[0].Name)
	}
}

func TestDeletePathOutOfRange(t *testing.T) {
	h := initWithDefaultDoc(t)
	defer Free(h)

	_, err := DispatchCommand(h, commandDeletePath, mustJSON(t, DeletePathPayload{PathIndex: 0}))
	if err == nil {
		t.Fatal("expected error for out-of-range delete, got nil")
	}
}

func TestPathArchiveRoundTrip(t *testing.T) {
	doc := testDocumentFixture("path-archive", "PathArchive", 64, 32)
	layer := NewPixelLayer("Base", LayerBounds{X: 0, Y: 0, W: 2, H: 2}, filledPixels(2, 2, [4]byte{1, 2, 3, 255}))
	doc.LayerRoot.SetChildren([]LayerNode{layer})
	doc.ActiveLayerID = layer.ID()

	doc.CreatePath("Shape A")
	doc.CreatePath("Shape B")

	data, err := SaveProject(doc, nil)
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, _, err := LoadProject(data)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded.Paths) != 2 {
		t.Fatalf("expected 2 paths after load, got %d", len(loaded.Paths))
	}
	if loaded.Paths[0].Name != "Shape A" {
		t.Fatalf("expected path name %q, got %q", "Shape A", loaded.Paths[0].Name)
	}
	if loaded.Paths[1].Name != "Shape B" {
		t.Fatalf("expected path name %q, got %q", "Shape B", loaded.Paths[1].Name)
	}
	if loaded.ActivePathIdx != 1 {
		t.Fatalf("expected active path index 1, got %d", loaded.ActivePathIdx)
	}
}
