package engine

import (
	"fmt"
	"sync/atomic"
	"time"

	psdio "github.com/cwbudde/agogo-web/packages/engine-wasm/internal/io/psd"
)

func newImportedPSDDocument(header psdio.Header, resources psdio.ImageResources) *Document {
	timestamp := time.Now().UTC().Format(time.RFC3339)
	resolution := resources.Resolution
	if resolution <= 0 {
		resolution = defaultResolutionDPI
	}
	return newDocumentWithCore(newDocumentCore(DocumentCreateParams{
		Width:      header.Width,
		Height:     header.Height,
		Resolution: resolution,
		ColorMode:  psdio.DocumentColorMode(header.ColorMode),
		BitDepth:   header.Depth,
		Background: "transparent",
		ID:         fmt.Sprintf("doc-%04d", atomic.AddInt64(&nextDocID, 1)),
		Name:       "Imported PSD",
		CreatedAt:  timestamp,
		CreatedBy:  "agogo-web",
		ModifiedAt: timestamp,
	}))
}
