package engine

import (
	"encoding/json"
	"fmt"
	"math"
)

func cloneDocument(doc *Document) *Document {
	if doc == nil {
		return nil
	}
	copyDoc := *doc
	copyDoc.LayerRoot = cloneGroupLayer(doc.LayerRoot)
	copyDoc.Selection = cloneSelection(doc.Selection)
	copyDoc.LastSelection = cloneSelection(doc.LastSelection)
	return &copyDoc
}

func snapshotsEqual(a, b snapshot) bool {
	if a.DocumentID != b.DocumentID {
		return false
	}
	if a.Viewport != b.Viewport {
		return false
	}
	if (a.Document == nil) != (b.Document == nil) {
		return false
	}
	if a.Document == nil {
		return true
	}
	return documentsEqual(a.Document, b.Document)
}

func documentsEqual(a, b *Document) bool {
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
	if a.CreatedAt != b.CreatedAt || a.CreatedBy != b.CreatedBy || a.ModifiedAt != b.ModifiedAt {
		return false
	}
	if a.ActiveLayerID != b.ActiveLayerID {
		return false
	}
	if !selectionEqual(a.Selection, b.Selection) || !selectionEqual(a.LastSelection, b.LastSelection) {
		return false
	}
	return layerTreeEqual(a.LayerRoot, b.LayerRoot)
}

func screenDeltaToDocument(deltaX, deltaY, zoom, rotation float64) (float64, float64) {
	const degToRad = math.Pi / 180
	radians := rotation * degToRad
	cosTheta := math.Cos(radians)
	sinTheta := math.Sin(radians)
	return (deltaX*cosTheta + deltaY*sinTheta) / zoom,
		(-deltaX*sinTheta + deltaY*cosTheta) / zoom
}

func parseBackground(kind string) Background {
	switch kind {
	case "white":
		return Background{Kind: "white", Color: [4]uint8{244, 246, 250, 255}}
	case "color":
		return Background{Kind: "color", Color: [4]uint8{236, 147, 92, 255}}
	default:
		return Background{Kind: "transparent"}
	}
}

func defaultDocumentName(name string) string {
	if name == "" {
		return "Untitled"
	}
	return name
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
