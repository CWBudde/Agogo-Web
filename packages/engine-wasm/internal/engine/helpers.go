package engine

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/cwbudde/agogo-web/packages/engine-wasm/internal/model"
)

func cloneDocument(doc *Document) *Document {
	if doc == nil {
		return nil
	}
	copyDoc := *doc
	copyDoc.LayerRoot = cloneGroupLayer(doc.LayerRoot)
	copyDoc.Selection = cloneSelection(doc.Selection)
	copyDoc.LastSelection = cloneSelection(doc.LastSelection)
	copyDoc.SavedSelections = cloneSavedSelectionChannels(doc.SavedSelections)
	copyDoc.Paths = cloneNamedPaths(doc.Paths)
	copyDoc.StylePresets = cloneDocumentStylePresets(doc.StylePresets)
	return &copyDoc
}

func snapshotsEqual(a, b snapshot) bool {
	if a.DocumentID != b.DocumentID {
		return false
	}
	if a.Viewport != b.Viewport {
		return false
	}
	// Fast path for pointer snapshots (see captureSnapshot): identical Document
	// pointers mean no snapshot-based command replaced the stored document
	// between the two captures, so they describe the same state. In-place pixel
	// mutations that may have happened in between (brush strokes) are recorded
	// by their own pixelDeltaCommands and are deliberately NOT a difference
	// here — this mirrors how transactions whose commands never replaced the
	// document must collapse to a no-op instead of deep-comparing megabytes.
	if a.Document == b.Document {
		return true
	}
	if (a.Document == nil) != (b.Document == nil) {
		return false
	}
	if a.Document == nil {
		return true
	}
	return documentsEqual(a.Document, b.Document)
}

// documentsEqual reports whether two documents are equal for the purpose of
// history no-op detection (it is the only consumer, via snapshotsEqual).
//
// ModifiedAt and ContentVersion are intentionally NOT treated as content:
// several no-op mutations bump them unconditionally (for example SetLayerName
// calls touchModifiedAt even when the name is unchanged, which bumps both). If
// they were compared as ordinary fields, genuine no-ops such as "rename a layer
// to its current name" would never be suppressed.
//
// ContentVersion is instead used only as a cheap fast-path signal for the
// expensive layer-tree comparison. Every pixel mutation bumps ContentVersion
// (verified: paint, fill, gradient, transforms, filters, magic eraser all route
// through touchModified*/bumpContentVersionRect/ContentVersion++). Therefore an
// identical ContentVersion guarantees identical pixel bytes, so we can compare
// only structure/metadata and skip the multi-hundred-millisecond bytes.Equal
// over large pixel buffers (see layerTreeEqualSkipPixels). The converse does not
// hold — a metadata-only touch also bumps ContentVersion — so when the versions
// differ we fall back to the full pixel-inclusive comparison. That fallback runs
// a full byte scan only when the trees are otherwise identical (a true no-op that
// still bumped the version); for real edits bytes.Equal short-circuits at the
// first differing pixel.
func documentsEqual(a, b *Document) bool {
	if a == b {
		return true
	}
	if (a == nil) != (b == nil) {
		return false
	}
	if a == nil {
		return true
	}
	if a.Width != b.Width || a.Height != b.Height || a.Resolution != b.Resolution || a.ColorMode != b.ColorMode {
		return false
	}
	if a.BitDepth != b.BitDepth || a.Background != b.Background || a.ID != b.ID || a.Name != b.Name {
		return false
	}
	if a.CreatedAt != b.CreatedAt || a.CreatedBy != b.CreatedBy {
		return false
	}
	if a.ActiveLayerID != b.ActiveLayerID {
		return false
	}
	if !selectionEqual(a.Selection, b.Selection) || !selectionEqual(a.LastSelection, b.LastSelection) {
		return false
	}
	if !savedSelectionChannelsEqual(a.SavedSelections, b.SavedSelections) {
		return false
	}
	if a.ActivePathIdx != b.ActivePathIdx || len(a.Paths) != len(b.Paths) {
		return false
	}
	for i := range a.Paths {
		if a.Paths[i].Name != b.Paths[i].Name {
			return false
		}
	}
	if !documentStylePresetsEqual(a.StylePresets, b.StylePresets) {
		return false
	}
	if a.ContentVersion == b.ContentVersion {
		// Identical version ⇒ identical pixels; compare structure only.
		return layerTreeEqualSkipPixels(a.LayerRoot, b.LayerRoot)
	}
	return layerTreeEqual(a.LayerRoot, b.LayerRoot)
}

// layerTreeEqualSkipPixels compares two layer trees for structural and metadata
// equality WITHOUT byte-comparing PixelLayer pixel buffers. It is only sound
// when the enclosing documents share an identical ContentVersion (see
// documentsEqual): under that precondition identical pixel content is guaranteed,
// so comparing PixelLayer buffer lengths is sufficient. Non-pixel leaf layers
// (adjustment/text/vector) carry only small buffers and are delegated to the
// authoritative model comparison to keep a single source of truth for their
// many fields; those leaf types never have children, so no descendant pixel
// buffers are byte-compared.
func layerTreeEqualSkipPixels(a, b LayerNode) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	if a == nil {
		return true
	}
	switch left := a.(type) {
	case *PixelLayer:
		right, ok := b.(*PixelLayer)
		if !ok || !layerCommonFieldsEqual(a, b) {
			return false
		}
		// Skip bytes.Equal(left.Pixels, right.Pixels): the shared ContentVersion
		// precondition guarantees the bytes are identical. A length check still
		// catches resizes (which also change Bounds). PixelLayers have no children.
		return left.Bounds == right.Bounds && len(left.Pixels) == len(right.Pixels)
	case *GroupLayer:
		right, ok := b.(*GroupLayer)
		if !ok || !layerCommonFieldsEqual(a, b) || left.Isolated != right.Isolated {
			return false
		}
		switch {
		case left.Artboard == nil && right.Artboard == nil:
		case left.Artboard == nil || right.Artboard == nil:
			return false
		case left.Artboard.Bounds != right.Artboard.Bounds || left.Artboard.Background != right.Artboard.Background:
			return false
		}
		leftChildren := a.Children()
		rightChildren := b.Children()
		if len(leftChildren) != len(rightChildren) {
			return false
		}
		for index := range leftChildren {
			if !layerTreeEqualSkipPixels(leftChildren[index], rightChildren[index]) {
				return false
			}
		}
		return true
	default:
		// Adjustment/Text/Vector leaves: small buffers, authoritative model compare.
		return layerTreeEqual(a, b)
	}
}

// layerCommonFieldsEqual compares the type-independent LayerNode fields shared by
// every node kind. It mirrors the common-field checks in model.LayerTreeEqual.
func layerCommonFieldsEqual(a, b LayerNode) bool {
	if a.ID() != b.ID() || a.LayerType() != b.LayerType() || a.Name() != b.Name() || a.Visible() != b.Visible() {
		return false
	}
	if a.LockMode() != b.LockMode() || a.Opacity() != b.Opacity() || a.FillOpacity() != b.FillOpacity() {
		return false
	}
	if a.BlendMode() != b.BlendMode() || a.ClipToBelow() != b.ClipToBelow() || a.ClippingBase() != b.ClippingBase() {
		return false
	}
	if !model.LayerMaskEqual(a.Mask(), b.Mask()) || !model.PathEqual(a.VectorMask(), b.VectorMask()) || !model.BlendIfEqual(a.BlendIf(), b.BlendIf()) {
		return false
	}
	return model.LayerStylesEqual(a.StyleStack(), b.StyleStack())
}

func screenDeltaToDocument(deltaX, deltaY, zoom, rotation float64) (float64, float64) {
	const degToRad = math.Pi / 180
	radians := rotation * degToRad
	cosTheta := math.Cos(radians)
	sinTheta := math.Sin(radians)
	return (deltaX*cosTheta + deltaY*sinTheta) / zoom,
		(-deltaX*sinTheta + deltaY*cosTheta) / zoom
}

func decodePayload[T any](payloadJSON string, target *T) error {
	if payloadJSON == "" {
		return nil
	}
	if err := json.Unmarshal([]byte(payloadJSON), target); err != nil {
		return fmt.Errorf("decode payload: %w", err)
	}
	return nil
}

func decodePayloadAny(payloadJSON string, target any) error {
	if payloadJSON == "" {
		return nil
	}
	if err := json.Unmarshal([]byte(payloadJSON), target); err != nil {
		return fmt.Errorf("decode payload: %w", err)
	}
	return nil
}

func clampZoom(zoom float64) float64 {
	if zoom <= 0 {
		return 1
	}
	if zoom < 0.05 {
		return 0.05
	}
	if zoom > 32 {
		return 32
	}
	return zoom
}

func normalizeRotation(rotation float64) float64 {
	normalized := math.Mod(rotation, 360)
	if normalized < 0 {
		normalized += 360
	}
	return normalized
}

//nolint:unused // kept for package-local tests
func valueOrDefault(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}

func floatValueOrDefault(value, fallback float64) float64 {
	if value <= 0 {
		return fallback
	}
	return value
}

func stringValueOrDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
