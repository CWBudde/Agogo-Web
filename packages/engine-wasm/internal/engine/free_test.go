package engine

import "testing"

// TestFreeThenOperationsReturnErrorNotPanic verifies that once an engine
// instance has been released via Free, every exported operation on that
// handle fails safely (returns an error / zero value) instead of panicking,
// and that freeing an already-freed or never-allocated handle is a safe
// no-op. This guards the WASM ABI boundary: a panic here would previously
// have been reachable from JS and would have crashed the whole Wasm runtime.
func TestFreeThenOperationsReturnErrorNotPanic(t *testing.T) {
	h := Init("")
	if h <= 0 {
		t.Fatalf("Init() returned invalid handle %d", h)
	}

	Free(h)

	if _, err := DispatchCommand(h, commandZoomSet, "{}"); err == nil {
		t.Error("DispatchCommand on freed handle: expected error, got nil")
	}
	if _, err := RenderFrame(h); err == nil {
		t.Error("RenderFrame on freed handle: expected error, got nil")
	}
	if _, err := RenderFrameRaw(h); err == nil {
		t.Error("RenderFrameRaw on freed handle: expected error, got nil")
	}
	if _, err := ExportProject(h); err == nil {
		t.Error("ExportProject on freed handle: expected error, got nil")
	}
	if _, err := ExportDocument(h, "png"); err == nil {
		t.Error("ExportDocument on freed handle: expected error, got nil")
	}
	if _, err := ImportProject(h, "{}"); err == nil {
		t.Error("ImportProject on freed handle: expected error, got nil")
	}
	if got := GetBufferPtr(h); got != 0 {
		t.Errorf("GetBufferPtr on freed handle = %d, want 0", got)
	}
	if got := GetBufferLen(h); got != 0 {
		t.Errorf("GetBufferLen on freed handle = %d, want 0", got)
	}

	// Double-free must be a safe no-op, never a panic.
	Free(h)
	Free(h)

	// Freeing a handle that was never allocated must also be safe.
	Free(999999)

	// A fresh instance should still work normally, i.e. Free of one handle
	// must not have corrupted the registry for others.
	h2 := Init("")
	defer Free(h2)
	if _, err := RenderFrame(h2); err != nil {
		t.Errorf("RenderFrame on fresh handle after unrelated Free: %v", err)
	}
}
