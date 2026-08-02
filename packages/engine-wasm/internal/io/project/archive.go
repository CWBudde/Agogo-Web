package project

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	docpkg "github.com/cwbudde/agogo-web/packages/engine-wasm/internal/document"
)

const ArchiveVersion = 1

func Save(document docpkg.ProjectDocumentArchive, history []docpkg.HistoryEntry) ([]byte, error) {
	archive := docpkg.ProjectArchive{
		Version:  ArchiveVersion,
		Document: document,
		History:  append([]docpkg.HistoryEntry(nil), history...),
	}
	return json.Marshal(archive)
}

func Load(data []byte) (docpkg.ProjectDocumentArchive, []docpkg.HistoryEntry, error) {
	if len(data) == 0 {
		return docpkg.ProjectDocumentArchive{}, nil, fmt.Errorf("project archive is empty")
	}
	var archive docpkg.ProjectArchive
	if err := json.Unmarshal(data, &archive); err != nil {
		return docpkg.ProjectDocumentArchive{}, nil, fmt.Errorf("decode project archive: %w", err)
	}
	if archive.Version != 0 && archive.Version != ArchiveVersion {
		return docpkg.ProjectDocumentArchive{}, nil, fmt.Errorf("unsupported project archive version %d", archive.Version)
	}
	return archive.Document, append([]docpkg.HistoryEntry(nil), archive.History...), nil
}

func SaveZip(document docpkg.ProjectDocumentArchive, history []docpkg.HistoryEntry) ([]byte, error) {
	archive := docpkg.ProjectArchive{
		Version:  ArchiveVersion,
		Document: document,
		History:  append([]docpkg.HistoryEntry(nil), history...),
	}

	blobs := map[string][]byte{}
	stripLayerBlobs(archive.Document.Layers, blobs)
	brushBlobs := map[string][]byte{}
	stripBrushTipBlobs(archive.Document.BrushTips, brushBlobs)

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
	for name, data := range brushBlobs {
		bw, err := zw.Create("brushes/" + name)
		if err != nil {
			return nil, fmt.Errorf("create brush entry %s: %w", name, err)
		}
		if _, err := bw.Write(data); err != nil {
			return nil, fmt.Errorf("write brush entry %s: %w", name, err)
		}
	}

	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("close zip: %w", err)
	}
	return buf.Bytes(), nil
}

func LoadZip(data []byte) (docpkg.ProjectDocumentArchive, []docpkg.HistoryEntry, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return docpkg.ProjectDocumentArchive{}, nil, fmt.Errorf("not a valid zip archive: %w", err)
	}

	blobs := map[string][]byte{}
	brushBlobs := map[string][]byte{}
	var manifestData []byte

	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			return docpkg.ProjectDocumentArchive{}, nil, fmt.Errorf("open zip entry %s: %w", f.Name, err)
		}
		var entry bytes.Buffer
		_, readErr := entry.ReadFrom(rc)
		_ = rc.Close()
		if readErr != nil {
			return docpkg.ProjectDocumentArchive{}, nil, fmt.Errorf("read zip entry %s: %w", f.Name, readErr)
		}
		switch {
		case f.Name == "manifest.json":
			manifestData = entry.Bytes()
		case strings.HasPrefix(f.Name, "layers/"):
			name := strings.TrimPrefix(f.Name, "layers/")
			blobs[name] = entry.Bytes()
		case strings.HasPrefix(f.Name, "brushes/"):
			name := strings.TrimPrefix(f.Name, "brushes/")
			brushBlobs[name] = entry.Bytes()
		}
	}

	if manifestData == nil {
		return docpkg.ProjectDocumentArchive{}, nil, fmt.Errorf("zip archive is missing manifest.json")
	}

	var archive docpkg.ProjectArchive
	if err := json.Unmarshal(manifestData, &archive); err != nil {
		return docpkg.ProjectDocumentArchive{}, nil, fmt.Errorf("decode zip manifest: %w", err)
	}

	restoreLayerBlobs(archive.Document.Layers, blobs)
	restoreBrushTipBlobs(archive.Document.BrushTips, brushBlobs)
	return archive.Document, append([]docpkg.HistoryEntry(nil), archive.History...), nil
}

func stripLayerBlobs(layers []docpkg.ProjectLayerArchive, blobs map[string][]byte) {
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

func restoreLayerBlobs(layers []docpkg.ProjectLayerArchive, blobs map[string][]byte) {
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

func stripBrushTipBlobs(resources []docpkg.BrushTipResource, blobs map[string][]byte) {
	for index := range resources {
		if len(resources[index].Alpha) == 0 {
			continue
		}
		blobs[resources[index].ID+".alpha.bin"] = resources[index].Alpha
		resources[index].Alpha = nil
	}
}

func restoreBrushTipBlobs(resources []docpkg.BrushTipResource, blobs map[string][]byte) {
	for index := range resources {
		if data, ok := blobs[resources[index].ID+".alpha.bin"]; ok {
			resources[index].Alpha = data
		}
	}
}
