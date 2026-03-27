package engine

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// SaveProjectZip serializes a document to a ZIP archive containing:
//   - manifest.json — all project data except pixel blobs
//   - layers/<id>.bin — raw RGBA bytes per PixelLayer
//   - layers/<id>.raster.bin — cached raster for TextLayer/VectorLayer
func SaveProjectZip(doc *Document, history []HistoryEntry) ([]byte, error) {
	if doc == nil {
		return nil, fmt.Errorf("document is required")
	}
	raw, err := SaveProject(doc, history)
	if err != nil {
		return nil, err
	}
	var archive projectArchive
	if err := json.Unmarshal(raw, &archive); err != nil {
		return nil, fmt.Errorf("re-parse archive for zip: %w", err)
	}

	blobs := map[string][]byte{}
	stripLayerBlobs(archive.Document.Layers, blobs)

	manifestJSON, err := json.Marshal(archive)
	if err != nil {
		return nil, fmt.Errorf("marshal zip manifest: %w", err)
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	mw, err := zw.Create("manifest.json")
	if err != nil {
		return nil, fmt.Errorf("create manifest.json entry: %w", err)
	}
	if _, err := mw.Write(manifestJSON); err != nil {
		return nil, fmt.Errorf("write manifest.json: %w", err)
	}

	for name, data := range blobs {
		bw, err := zw.Create("layers/" + name)
		if err != nil {
			return nil, fmt.Errorf("create layer entry %s: %w", name, err)
		}
		if _, err := bw.Write(data); err != nil {
			return nil, fmt.Errorf("write layer entry %s: %w", name, err)
		}
	}

	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("close zip: %w", err)
	}
	return buf.Bytes(), nil
}

// LoadProjectZip deserializes a ZIP project archive.
// Returns an error if data is not a valid ZIP — use LoadProject for legacy JSON.
func LoadProjectZip(data []byte) (*Document, []HistoryEntry, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, nil, fmt.Errorf("not a valid zip archive: %w", err)
	}

	blobs := map[string][]byte{}
	var manifestData []byte

	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			return nil, nil, fmt.Errorf("open zip entry %s: %w", f.Name, err)
		}
		var entry bytes.Buffer
		_, readErr := entry.ReadFrom(rc)
		_ = rc.Close()
		if readErr != nil {
			return nil, nil, fmt.Errorf("read zip entry %s: %w", f.Name, readErr)
		}
		switch {
		case f.Name == "manifest.json":
			manifestData = entry.Bytes()
		case strings.HasPrefix(f.Name, "layers/"):
			name := strings.TrimPrefix(f.Name, "layers/")
			blobs[name] = entry.Bytes()
		}
	}

	if manifestData == nil {
		return nil, nil, fmt.Errorf("zip archive is missing manifest.json")
	}

	var archive projectArchive
	if err := json.Unmarshal(manifestData, &archive); err != nil {
		return nil, nil, fmt.Errorf("decode zip manifest: %w", err)
	}

	restoreLayerBlobs(archive.Document.Layers, blobs)

	restored, err := json.Marshal(archive)
	if err != nil {
		return nil, nil, fmt.Errorf("re-encode archive: %w", err)
	}
	return LoadProject(restored)
}

func stripLayerBlobs(layers []projectLayerArchive, blobs map[string][]byte) {
	for i := range layers {
		if len(layers[i].Pixels) > 0 {
			blobs[layers[i].ID+".bin"] = layers[i].Pixels
			layers[i].Pixels = nil
		}
		if len(layers[i].CachedRaster) > 0 {
			blobs[layers[i].ID+".raster.bin"] = layers[i].CachedRaster
			layers[i].CachedRaster = nil
		}
		stripLayerBlobs(layers[i].Children, blobs)
	}
}

func restoreLayerBlobs(layers []projectLayerArchive, blobs map[string][]byte) {
	for i := range layers {
		if data, ok := blobs[layers[i].ID+".bin"]; ok {
			layers[i].Pixels = data
		}
		if data, ok := blobs[layers[i].ID+".raster.bin"]; ok {
			layers[i].CachedRaster = data
		}
		restoreLayerBlobs(layers[i].Children, blobs)
	}
}
