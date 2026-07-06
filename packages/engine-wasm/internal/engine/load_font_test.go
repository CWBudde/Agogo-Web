package engine

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"codeberg.org/go-fonts/dejavu/dejavusansbold"
)

// validFontPayload builds a LoadFontData payload carrying the embedded
// DejaVu Sans Bold TTF registered under the given family as a regular face.
func validFontPayload(family string) LoadFontDataPayload {
	return LoadFontDataPayload{
		Name: family,
		Data: base64.StdEncoding.EncodeToString(dejavusansbold.TTF),
	}
}

// renderTextRaster rasterizes a small point-text layer with the given
// family/bold request and returns the bounds-local RGBA raster.
func renderTextRaster(t *testing.T, family string, bold bool) []byte {
	t.Helper()
	layer := NewTextLayer("t", LayerBounds{X: 0, Y: 0, W: 200, H: 100}, "Agogo", nil)
	layer.AnchorX = 10
	layer.AnchorY = 50
	layer.AnchorSet = true
	layer.FontSize = 24
	layer.Color = [4]uint8{0, 0, 0, 255}
	layer.FontFamily = family
	layer.Bold = bold
	if err := rasterizeTextLayer(layer); err != nil {
		t.Fatalf("rasterize with family %q: %v", family, err)
	}
	if len(layer.CachedRaster) == 0 {
		t.Fatalf("rasterize with family %q produced an empty raster", family)
	}
	return layer.CachedRaster
}

// TestLoadFontData_RegistersFamilyUsedForRendering registers the DejaVu Sans
// Bold bytes under a custom family name (as its Regular face) and asserts the
// custom family renders byte-identically to "DejaVu Sans" bold (same
// underlying face) while differing from "DejaVu Sans" regular.
func TestLoadFontData_RegistersFamilyUsedForRendering(t *testing.T) {
	h := initTextTestDoc(t)
	defer Free(h)

	const family = "Test Family F4 Render"
	if _, err := DispatchCommand(h, commandLoadFontData, mustJSON(t, validFontPayload(family))); err != nil {
		t.Fatalf("LoadFontData: %v", err)
	}

	custom := renderTextRaster(t, family, false)
	dejavuBold := renderTextRaster(t, "DejaVu Sans", true)
	dejavuRegular := renderTextRaster(t, "DejaVu Sans", false)

	if !bytes.Equal(custom, dejavuBold) {
		t.Error("custom family (DejaVu Sans Bold bytes) does not render identically to DejaVu Sans bold")
	}
	if bytes.Equal(custom, dejavuRegular) {
		t.Error("custom family renders identically to DejaVu Sans regular; registered font was not used")
	}
}

// TestLoadFontData_InvalidFontDataErrors covers unparsable font bytes and
// invalid base64: both must surface an error over the ABI without panicking.
func TestLoadFontData_InvalidFontDataErrors(t *testing.T) {
	h := initTextTestDoc(t)
	defer Free(h)

	if _, err := DispatchCommand(h, commandLoadFontData, mustJSON(t, LoadFontDataPayload{
		Name: "Garbage Font",
		Data: base64.StdEncoding.EncodeToString([]byte("this is definitely not an sfnt font")),
	})); err == nil {
		t.Error("expected error for unparsable font bytes")
	}

	if _, err := DispatchCommand(h, commandLoadFontData, mustJSON(t, LoadFontDataPayload{
		Name: "Bad Base64",
		Data: "%%% not base64 %%%",
	})); err == nil {
		t.Error("expected error for invalid base64 data")
	}
}

// TestLoadFontData_EmptyNameErrors verifies the handler-level name guard.
func TestLoadFontData_EmptyNameErrors(t *testing.T) {
	h := initTextTestDoc(t)
	defer Free(h)

	for _, name := range []string{"", "   "} {
		payload := validFontPayload(name)
		_, err := DispatchCommand(h, commandLoadFontData, mustJSON(t, payload))
		if err == nil {
			t.Errorf("name %q: expected error, got nil", name)
			continue
		}
		if !strings.Contains(err.Error(), "font name required") {
			t.Errorf("name %q: error = %q, want it to contain %q", name, err.Error(), "font name required")
		}
	}
}

// TestLoadFontData_WorksWithoutDocument covers app-startup font registration:
// LoadFontData must succeed on an engine with no open document, while other
// text commands keep rejecting the no-document state.
func TestLoadFontData_WorksWithoutDocument(t *testing.T) {
	h := Init(`{}`)
	if h <= 0 {
		t.Fatalf("Init returned invalid handle %d", h)
	}
	defer Free(h)

	const family = "Startup Family F4"
	result, err := DispatchCommand(h, commandLoadFontData, mustJSON(t, validFontPayload(family)))
	if err != nil {
		t.Fatalf("LoadFontData without document: %v", err)
	}
	if result.UIMeta == nil {
		t.Fatal("LoadFontData returned no UIMeta")
	}
	if !uiMetaHasFontFamily(result.UIMeta, family) {
		t.Errorf("availableFonts is missing %q after no-document registration", family)
	}

	// The domain guard must still reject document-dependent text commands.
	if _, err := DispatchCommand(h, commandSetTextContent, mustJSON(t, SetTextContentPayload{
		LayerID: "missing", Text: "x",
	})); err == nil {
		t.Error("SetTextContent without document: expected \"no active document\" error, got nil")
	}
}

// TestLoadFontData_CreatesNoHistoryEntry pins that font registration is
// app-level state and never becomes undoable.
func TestLoadFontData_CreatesNoHistoryEntry(t *testing.T) {
	h := initTextTestDoc(t)
	defer Free(h)

	before, err := RenderFrame(h)
	if err != nil {
		t.Fatalf("RenderFrame: %v", err)
	}

	result, err := DispatchCommand(h, commandLoadFontData, mustJSON(t, validFontPayload("History Family F4")))
	if err != nil {
		t.Fatalf("LoadFontData: %v", err)
	}
	if result.UIMeta == nil {
		t.Fatal("LoadFontData returned no UIMeta")
	}
	if got, want := len(result.UIMeta.History), len(before.UIMeta.History); got != want {
		t.Errorf("history length after LoadFontData = %d, want %d", got, want)
	}
	if result.UIMeta.CanUndo != before.UIMeta.CanUndo {
		t.Errorf("CanUndo changed from %v to %v after LoadFontData", before.UIMeta.CanUndo, result.UIMeta.CanUndo)
	}
}

// TestUIMeta_AvailableFontsListsRegisteredFamilies asserts UIMeta enumerates
// the embedded DejaVu Sans family (all four styles) plus a freshly loaded
// family, and that the field serializes under the "availableFonts" key.
func TestUIMeta_AvailableFontsListsRegisteredFamilies(t *testing.T) {
	h := initTextTestDoc(t)
	defer Free(h)

	const family = "UIMeta Family F4"
	result, err := DispatchCommand(h, commandLoadFontData, mustJSON(t, validFontPayload(family)))
	if err != nil {
		t.Fatalf("LoadFontData: %v", err)
	}
	if result.UIMeta == nil {
		t.Fatal("no UIMeta in dispatch result")
	}

	var dejavu *FontFamilyMeta
	for i := range result.UIMeta.AvailableFonts {
		if result.UIMeta.AvailableFonts[i].Family == "DejaVu Sans" {
			dejavu = &result.UIMeta.AvailableFonts[i]
			break
		}
	}
	if dejavu == nil {
		t.Fatal("availableFonts is missing the embedded DejaVu Sans family")
	}
	wantStyles := []string{"Regular", "Bold", "Italic", "Bold Italic"}
	if len(dejavu.Styles) != len(wantStyles) {
		t.Fatalf("DejaVu Sans styles = %v, want %v", dejavu.Styles, wantStyles)
	}
	for i, s := range wantStyles {
		if dejavu.Styles[i] != s {
			t.Fatalf("DejaVu Sans styles = %v, want %v", dejavu.Styles, wantStyles)
		}
	}

	if !uiMetaHasFontFamily(result.UIMeta, family) {
		t.Errorf("availableFonts is missing the loaded family %q", family)
	}

	data, err := json.Marshal(result.UIMeta)
	if err != nil {
		t.Fatalf("marshal UIMeta: %v", err)
	}
	if !strings.Contains(string(data), `"availableFonts"`) {
		t.Error("UIMeta JSON is missing the availableFonts key")
	}
}

// uiMetaHasFontFamily reports whether meta lists the given family with a
// Regular style entry.
func uiMetaHasFontFamily(meta *UIMeta, family string) bool {
	for _, f := range meta.AvailableFonts {
		if f.Family != family {
			continue
		}
		for _, s := range f.Styles {
			if s == "Regular" {
				return true
			}
		}
	}
	return false
}
