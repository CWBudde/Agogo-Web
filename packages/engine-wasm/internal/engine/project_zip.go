package engine

import (
	"fmt"

	projectio "github.com/cwbudde/agogo-web/packages/engine-wasm/internal/io/project"
)

// SaveProjectZip serializes a document to a ZIP archive containing:
//   - manifest.json — all project data except pixel blobs
//   - layers/<id>.bin — raw RGBA bytes per PixelLayer
//   - layers/<id>.raster.bin — cached raster for TextLayer/VectorLayer
func SaveProjectZip(doc *Document, history []HistoryEntry) ([]byte, error) {
	if doc == nil {
		return nil, fmt.Errorf("document is required")
	}
	archive := projectDocumentArchive{
		Width:           doc.Width,
		Height:          doc.Height,
		Resolution:      doc.Resolution,
		ColorMode:       doc.ColorMode,
		BitDepth:        doc.BitDepth,
		Background:      doc.Background,
		ID:              doc.ID,
		Name:            doc.Name,
		CreatedAt:       doc.CreatedAt,
		CreatedBy:       doc.CreatedBy,
		ModifiedAt:      doc.ModifiedAt,
		ActiveLayer:     doc.ActiveLayerID,
		Layers:          make([]projectLayerArchive, 0),
		Paths:           cloneNamedPaths(doc.Paths),
		ActivePathIdx:   doc.ActivePathIdx,
		SavedSelections: cloneSavedSelectionChannels(doc.SavedSelections),
		StylePresets:    cloneDocumentStylePresets(doc.StylePresets),
	}
	if root := doc.ensureLayerRoot(); root != nil {
		children := root.Children()
		archive.Layers = make([]projectLayerArchive, 0, len(children))
		for _, child := range children {
			archive.Layers = append(archive.Layers, buildProjectLayerArchive(child))
		}
	}
	return projectio.SaveZip(archive, history)
}

// LoadProjectZip deserializes a ZIP project archive.
// Returns an error if data is not a valid ZIP — use LoadProject for legacy JSON.
func LoadProjectZip(data []byte) (*Document, []HistoryEntry, error) {
	archive, history, err := projectio.LoadZip(data)
	if err != nil {
		return nil, nil, err
	}
	doc, err := projectDocumentArchiveToDocument(archive)
	if err != nil {
		return nil, nil, err
	}
	return doc, history, nil
}
