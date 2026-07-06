package engine

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cwbudde/agogo-web/packages/engine-wasm/internal/model"
)

// --- Builtin resolution -----------------------------------------------------

func TestBuiltinPatternsResolveDeterministic(t *testing.T) {
	ids := []string{"builtin/checker", "builtin/stripes", "builtin/dots", "builtin/noise"}
	for _, id := range ids {
		first := resolvePattern(nil, id)
		if first == nil {
			t.Fatalf("builtin %q did not resolve", id)
		}
		if first.Width <= 0 || first.Height <= 0 {
			t.Fatalf("builtin %q has degenerate dims %dx%d", id, first.Width, first.Height)
		}
		if len(first.Data) != first.Width*first.Height*4 {
			t.Fatalf("builtin %q data length %d, want %d", id, len(first.Data), first.Width*first.Height*4)
		}
		firstData := append([]byte(nil), first.Data...)
		second := resolvePattern(nil, id)
		if second == nil || !bytes.Equal(firstData, second.Data) {
			t.Fatalf("builtin %q not deterministic across two resolutions", id)
		}
	}
	if resolvePattern(nil, "builtin/does-not-exist") != nil {
		t.Fatal("unknown pattern id must resolve to nil")
	}
	if resolvePattern(nil, "") != nil {
		t.Fatal("empty pattern id must resolve to nil")
	}
}

func TestResolvePatternDocOverridesBuiltin(t *testing.T) {
	doc := &Document{
		Patterns: []model.PatternResource{{
			ID: "builtin/checker", Name: "Shadowing", Width: 1, Height: 1,
			Data: []byte{1, 2, 3, 4},
		}},
	}
	got := resolvePattern(doc, "builtin/checker")
	if got == nil || got.Name != "Shadowing" {
		t.Fatalf("doc pattern must take precedence over builtin, got %+v", got)
	}
	// Builtins are still reachable when the doc has no shadowing entry.
	if resolvePattern(doc, "builtin/stripes") == nil {
		t.Fatal("builtin lookup broken when doc patterns present")
	}
}

// --- Sampling ----------------------------------------------------------------

func testSamplingPattern() *model.PatternResource {
	// 4x1 tile: columns A B C D, distinguishable by red channel.
	return &model.PatternResource{
		ID: "test/sample", Name: "Sample", Width: 4, Height: 1,
		Data: []byte{
			10, 0, 0, 255,
			20, 0, 0, 255,
			30, 0, 0, 255,
			40, 0, 0, 255,
		},
	}
}

func TestSamplePatternColorTilingScaleOne(t *testing.T) {
	p := testSamplingPattern()
	cases := []struct {
		x    int
		want uint8
	}{
		{0, 10},
		{1, 20},
		{2, 30},
		{3, 40},
		{4, 10},
		{5, 20}, // wraps
		{-1, 40},
		{-4, 10},
		{-5, 40}, // floored modulo for negatives
	}
	for _, tc := range cases {
		got := samplePatternColor(p, tc.x, 0, 1)
		if got[0] != tc.want {
			t.Fatalf("samplePatternColor(x=%d, scale=1) red = %d, want %d", tc.x, got[0], tc.want)
		}
	}
}

func TestSamplePatternColorScaled(t *testing.T) {
	p := testSamplingPattern()
	// scale 2: each tile column covers 2 doc pixels.
	for x, want := range map[int]uint8{0: 10, 1: 10, 2: 20, 3: 20, 4: 30, 7: 40, 8: 10, -1: 40, -2: 40} {
		got := samplePatternColor(p, x, 0, 2)
		if got[0] != want {
			t.Fatalf("scale 2 x=%d red = %d, want %d", x, got[0], want)
		}
	}
	// scale 0.5: doc pixel x maps to tile column (2x) mod 4.
	for x, want := range map[int]uint8{0: 10, 1: 30, 2: 10, 3: 30} {
		got := samplePatternColor(p, x, 0, 0.5)
		if got[0] != want {
			t.Fatalf("scale 0.5 x=%d red = %d, want %d", x, got[0], want)
		}
	}
	// scale <= 0 falls back to 1.
	if got := samplePatternColor(p, 1, 0, 0); got[0] != 20 {
		t.Fatalf("scale 0 must behave like scale 1, got red %d", got[0])
	}
	if got := samplePatternColor(p, 1, 0, -3); got[0] != 20 {
		t.Fatalf("negative scale must behave like scale 1, got red %d", got[0])
	}
}

// --- DefinePattern ------------------------------------------------------------

// newPatternTestInstance builds an 8x4 doc whose single pixel layer holds a
// unique color per pixel (red = x, green = y) so tile capture is verifiable.
func newPatternTestInstance(t *testing.T) (*instance, *Document, string) {
	t.Helper()
	const w, h = 8, 4
	pixels := make([]byte, w*h*4)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := (y*w + x) * 4
			pixels[i] = uint8(x * 10)
			pixels[i+1] = uint8(y * 10)
			pixels[i+2] = 7
			pixels[i+3] = 255
		}
	}
	layer := NewPixelLayer("Layer", LayerBounds{X: 0, Y: 0, W: w, H: h}, pixels)
	doc := &Document{
		ID:            "doc-patterns",
		Width:         w,
		Height:        h,
		LayerRoot:     NewGroupLayer("Root"),
		ActiveLayerID: layer.ID(),
	}
	doc.LayerRoot.SetChildren([]LayerNode{layer})
	inst := &instance{
		manager:         newDocumentManager(),
		history:         newHistoryStack(16),
		viewport:        ViewportState{CanvasW: w, CanvasH: h, Zoom: 1, DevicePixelRatio: 1},
		foregroundColor: [4]uint8{20, 30, 40, 255},
		backgroundColor: [4]uint8{220, 230, 240, 255},
	}
	inst.manager.Create(doc)
	return inst, inst.manager.activeMut(), layer.ID()
}

func TestDefinePatternFromSelection(t *testing.T) {
	inst, doc, _ := newPatternTestInstance(t)
	// Rect selection covering (2,1)-(4,2) inclusive → 3x2 tile.
	sel := newSelection(doc.Width, doc.Height)
	for y := 1; y <= 2; y++ {
		for x := 2; x <= 4; x++ {
			sel.Mask[y*doc.Width+x] = 255
		}
	}
	doc.Selection = sel

	if err := inst.handleDefinePattern("Sel Tile"); err != nil {
		t.Fatalf("handleDefinePattern: %v", err)
	}
	updated := inst.manager.Active()
	if len(updated.Patterns) != 1 {
		t.Fatalf("patterns len = %d, want 1", len(updated.Patterns))
	}
	p := updated.Patterns[0]
	if p.Name != "Sel Tile" {
		t.Fatalf("pattern name = %q, want %q", p.Name, "Sel Tile")
	}
	if !strings.HasPrefix(p.ID, "pattern-") {
		t.Fatalf("pattern id = %q, want pattern-<suffix>", p.ID)
	}
	if p.Width != 3 || p.Height != 2 {
		t.Fatalf("tile dims = %dx%d, want 3x2", p.Width, p.Height)
	}
	for y := 0; y < 2; y++ {
		for x := 0; x < 3; x++ {
			i := (y*3 + x) * 4
			wantR := uint8((x + 2) * 10)
			wantG := uint8((y + 1) * 10)
			if p.Data[i] != wantR || p.Data[i+1] != wantG {
				t.Fatalf("tile pixel (%d,%d) = (%d,%d), want (%d,%d)", x, y, p.Data[i], p.Data[i+1], wantR, wantG)
			}
		}
	}
}

func TestDefinePatternWholeLayerAndUndo(t *testing.T) {
	inst, doc, layerID := newPatternTestInstance(t)
	if err := inst.handleDefinePattern(""); err != nil {
		t.Fatalf("handleDefinePattern: %v", err)
	}
	updated := inst.manager.Active()
	if len(updated.Patterns) != 1 {
		t.Fatalf("patterns len = %d, want 1", len(updated.Patterns))
	}
	p := updated.Patterns[0]
	if p.Width != doc.Width || p.Height != doc.Height {
		t.Fatalf("tile dims = %dx%d, want whole layer %dx%d", p.Width, p.Height, doc.Width, doc.Height)
	}
	if p.Name != "Pattern 1" {
		t.Fatalf("default name = %q, want %q", p.Name, "Pattern 1")
	}
	layer := findPixelLayer(updated, layerID)
	if !bytes.Equal(p.Data, layer.Pixels) {
		t.Fatal("whole-layer tile must match layer pixels")
	}

	if err := inst.history.Undo(inst); err != nil {
		t.Fatalf("undo: %v", err)
	}
	if got := len(inst.manager.Active().Patterns); got != 0 {
		t.Fatalf("patterns after undo = %d, want 0", got)
	}
}

func TestDefinePatternTileCap(t *testing.T) {
	inst, doc, _ := newPatternTestInstance(t)
	big := NewPixelLayer("Big", LayerBounds{X: 0, Y: 0, W: 1030, H: 2}, make([]byte, 1030*2*4))
	doc.LayerRoot.SetChildren(append(doc.LayerRoot.Children(), big))
	doc.ActiveLayerID = big.ID()
	if err := inst.manager.ReplaceActive(doc); err != nil {
		t.Fatalf("ReplaceActive: %v", err)
	}
	err := inst.handleDefinePattern("Too Big")
	if err == nil || !strings.Contains(err.Error(), "1024") {
		t.Fatalf("expected tile-cap error mentioning 1024, got %v", err)
	}
}

func TestDefinePatternRequiresRasterizableLayer(t *testing.T) {
	inst, doc, _ := newPatternTestInstance(t)
	adj := NewAdjustmentLayer("Adjust", "invert", nil)
	doc.LayerRoot.SetChildren(append(doc.LayerRoot.Children(), adj))
	doc.ActiveLayerID = adj.ID()
	if err := inst.manager.ReplaceActive(doc); err != nil {
		t.Fatalf("ReplaceActive: %v", err)
	}
	err := inst.handleDefinePattern("Nope")
	if err == nil || !strings.Contains(err.Error(), "no rasterizable active layer") {
		t.Fatalf("expected no-rasterizable-active-layer error, got %v", err)
	}
}

func TestDeletePattern(t *testing.T) {
	inst, _, _ := newPatternTestInstance(t)
	if err := inst.handleDefinePattern("Doomed"); err != nil {
		t.Fatalf("handleDefinePattern: %v", err)
	}
	id := inst.manager.Active().Patterns[0].ID
	if err := inst.handleDeletePattern(id); err != nil {
		t.Fatalf("handleDeletePattern: %v", err)
	}
	if got := len(inst.manager.Active().Patterns); got != 0 {
		t.Fatalf("patterns after delete = %d, want 0", got)
	}
	if err := inst.handleDeletePattern("pattern-missing"); err == nil {
		t.Fatal("deleting unknown pattern must error")
	}
	// Undo restores the deleted pattern.
	if err := inst.history.Undo(inst); err != nil {
		t.Fatalf("undo: %v", err)
	}
	if got := len(inst.manager.Active().Patterns); got != 1 {
		t.Fatalf("patterns after undo of delete = %d, want 1", got)
	}
}

func TestPatternCommandsDispatch(t *testing.T) {
	inst, _, _ := newPatternTestInstance(t)
	handled, _, _, err := inst.dispatchSelectionPaintCommand(0x0417, `{"name":"Dispatched"}`, nil)
	if !handled {
		t.Fatal("DefinePattern (0x0417) not handled by selection/paint dispatcher")
	}
	if err != nil {
		t.Fatalf("DefinePattern dispatch: %v", err)
	}
	patterns := inst.manager.Active().Patterns
	if len(patterns) != 1 || patterns[0].Name != "Dispatched" {
		t.Fatalf("dispatched pattern missing, got %+v", patterns)
	}
	handled, _, _, err = inst.dispatchSelectionPaintCommand(0x0418, `{"patternId":"`+patterns[0].ID+`"}`, nil)
	if !handled {
		t.Fatal("DeletePattern (0x0418) not handled by selection/paint dispatcher")
	}
	if err != nil {
		t.Fatalf("DeletePattern dispatch: %v", err)
	}
	if got := len(inst.manager.Active().Patterns); got != 0 {
		t.Fatalf("patterns after dispatched delete = %d, want 0", got)
	}
}

// --- Fill plumbing --------------------------------------------------------

// REGRESSION PIN: pattern fill without a patternId must stay byte-identical to
// the historical foreground/background 8px checker.
func TestFillPatternEmptyIDLegacyChecker(t *testing.T) {
	layer := NewPixelLayer("Layer", LayerBounds{X: 0, Y: 0, W: 16, H: 1}, make([]byte, 16*1*4))
	doc := &Document{
		ID:            "doc-legacy-checker",
		Width:         16,
		Height:        1,
		LayerRoot:     NewGroupLayer("Root"),
		ActiveLayerID: layer.ID(),
	}
	doc.LayerRoot.SetChildren([]LayerNode{layer})
	inst := &instance{
		manager:         newDocumentManager(),
		history:         newHistoryStack(16),
		viewport:        ViewportState{CanvasW: 16, CanvasH: 1, Zoom: 1, DevicePixelRatio: 1},
		foregroundColor: [4]uint8{20, 30, 40, 255},
		backgroundColor: [4]uint8{220, 230, 240, 255},
	}
	inst.manager.Create(doc)

	if err := inst.handleFill(FillPayload{Source: "pattern"}); err != nil {
		t.Fatalf("handleFill: %v", err)
	}
	got := findPixelLayer(inst.manager.Active(), layer.ID()).Pixels
	for x := 0; x < 16; x++ {
		want := inst.foregroundColor
		if (x/8)%2 != 0 {
			want = inst.backgroundColor
		}
		i := x * 4
		if got[i] != want[0] || got[i+1] != want[1] || got[i+2] != want[2] || got[i+3] != want[3] {
			t.Fatalf("legacy checker pixel %d = %v, want %v", x, got[i:i+4], want)
		}
	}
}

func TestFillWithPatternID(t *testing.T) {
	inst, doc, layerID := newPatternTestInstance(t)
	doc.Patterns = []model.PatternResource{{
		ID: "pattern-fill-test", Name: "Fill Tile", Width: 2, Height: 1,
		Data: []byte{
			11, 0, 0, 255,
			22, 0, 0, 255,
		},
	}}
	if err := inst.manager.ReplaceActive(doc); err != nil {
		t.Fatalf("ReplaceActive: %v", err)
	}

	if err := inst.handleFill(FillPayload{Source: "pattern", PatternID: "pattern-fill-test"}); err != nil {
		t.Fatalf("handleFill: %v", err)
	}
	got := findPixelLayer(inst.manager.Active(), layerID).Pixels
	for x := 0; x < 8; x++ {
		want := uint8(11)
		if x%2 == 1 {
			want = 22
		}
		if got[x*4] != want {
			t.Fatalf("pattern fill pixel %d red = %d, want %d", x, got[x*4], want)
		}
	}

	// Unknown pattern id falls back to the legacy fg/bg checker.
	inst2, _, layerID2 := newPatternTestInstance(t)
	if err := inst2.handleFill(FillPayload{Source: "pattern", PatternID: "pattern-unknown"}); err != nil {
		t.Fatalf("handleFill unknown id: %v", err)
	}
	first := findPixelLayer(inst2.manager.Active(), layerID2).Pixels[0:4]
	if first[0] != inst2.foregroundColor[0] {
		t.Fatalf("unknown pattern id must fall back to legacy checker, first pixel %v", first)
	}
}

// --- Archive round-trip -----------------------------------------------------

func TestPatternArchiveRoundTrip(t *testing.T) {
	_, doc, _ := newPatternTestInstance(t)
	doc.Patterns = []model.PatternResource{{
		ID: "pattern-archived", Name: "Archived", Width: 1, Height: 2,
		Data: []byte{1, 2, 3, 4, 5, 6, 7, 8},
	}}
	saved, err := SaveProject(doc, nil)
	if err != nil {
		t.Fatalf("SaveProject: %v", err)
	}
	restored, _, err := LoadProject(saved)
	if err != nil {
		t.Fatalf("LoadProject: %v", err)
	}
	if !model.PatternsEqual(doc.Patterns, restored.Patterns) {
		t.Fatalf("patterns after round-trip = %+v, want %+v", restored.Patterns, doc.Patterns)
	}

	// Old archives without a patterns key must load with nil patterns.
	_, plain, _ := newPatternTestInstance(t)
	savedPlain, err := SaveProject(plain, nil)
	if err != nil {
		t.Fatalf("SaveProject (no patterns): %v", err)
	}
	if bytes.Contains(savedPlain, []byte(`"patterns"`)) {
		t.Fatal("archive without patterns must omit the patterns key")
	}
	restoredPlain, _, err := LoadProject(savedPlain)
	if err != nil {
		t.Fatalf("LoadProject (no patterns): %v", err)
	}
	if len(restoredPlain.Patterns) != 0 {
		t.Fatalf("patterns for legacy archive = %d, want 0", len(restoredPlain.Patterns))
	}
}

// --- Snapshot equality / clone ------------------------------------------------

func TestDocumentsEqualDetectsPatternChanges(t *testing.T) {
	_, doc, _ := newPatternTestInstance(t)
	doc.Patterns = []model.PatternResource{{
		ID: "pattern-x", Name: "X", Width: 1, Height: 1, Data: []byte{9, 9, 9, 255},
	}}
	clone := cloneDocument(doc)
	if !documentsEqual(doc, clone) {
		t.Fatal("identical clone must compare equal")
	}
	// Deep clone: mutating the clone's tile bytes must not affect the original.
	clone.Patterns[0].Data[0] = 42
	if doc.Patterns[0].Data[0] != 9 {
		t.Fatal("cloneDocument must deep-copy pattern data")
	}
	if documentsEqual(doc, clone) {
		t.Fatal("pattern byte change must be detected")
	}
	clone.Patterns[0].Data[0] = 9
	clone.Patterns = append(clone.Patterns, model.PatternResource{ID: "pattern-y", Width: 1, Height: 1, Data: []byte{0, 0, 0, 0}})
	if documentsEqual(doc, clone) {
		t.Fatal("pattern append must be detected")
	}
}

// --- UIMeta -------------------------------------------------------------------

func TestUIMetaListsPatterns(t *testing.T) {
	inst, _, _ := newPatternTestInstance(t)
	meta := inst.renderUIMeta()
	builtinIDs := map[string]bool{}
	for _, p := range meta.Patterns {
		builtinIDs[p.ID] = true
	}
	for _, id := range []string{"builtin/checker", "builtin/stripes", "builtin/dots", "builtin/noise"} {
		if !builtinIDs[id] {
			t.Fatalf("UIMeta patterns missing builtin %q, got %+v", id, meta.Patterns)
		}
	}

	if err := inst.handleDefinePattern("Mine"); err != nil {
		t.Fatalf("handleDefinePattern: %v", err)
	}
	meta = inst.renderUIMeta()
	found := false
	for _, p := range meta.Patterns {
		if p.Name == "Mine" && p.Width == 8 && p.Height == 4 {
			found = true
		}
	}
	if !found {
		t.Fatalf("UIMeta patterns missing defined pattern, got %+v", meta.Patterns)
	}
}
