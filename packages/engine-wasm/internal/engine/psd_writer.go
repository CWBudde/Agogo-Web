package engine

import (
	"encoding/base64"
	"fmt"
	"strings"

	psdexport "github.com/cwbudde/agogo-web/packages/engine-wasm/internal/io/psdexport"
)

func SavePSD(doc *Document) ([]byte, error) {
	return savePSDDocument(doc, false)
}

func SavePSB(doc *Document) ([]byte, error) {
	return savePSDDocument(doc, true)
}

func savePSDDocument(doc *Document, forcePSB bool) ([]byte, error) {
	if doc == nil {
		return nil, fmt.Errorf("document is required")
	}
	if doc.Width <= 0 || doc.Height <= 0 {
		return nil, fmt.Errorf("document must have positive dimensions")
	}
	if doc.BitDepth != 0 && doc.BitDepth != 8 {
		return nil, fmt.Errorf("unsupported bit depth %d", doc.BitDepth)
	}
	if doc.ColorMode != "" && doc.ColorMode != "rgb" && doc.ColorMode != "gray" {
		return nil, fmt.Errorf("unsupported color mode %q", doc.ColorMode)
	}
	projectArchive, err := SaveProject(doc, nil)
	if err != nil {
		return nil, fmt.Errorf("build embedded project archive: %w", err)
	}
	return psdexport.Export(psdexport.Params{
		Width:          doc.Width,
		Height:         doc.Height,
		Resolution:     doc.Resolution,
		ColorMode:      doc.ColorMode,
		BitDepth:       doc.BitDepth,
		ForcePSB:       forcePSB,
		ProjectArchive: projectArchive,
		Layers:         doc.ensureLayerRoot().Children(),
		RenderLayer: func(layer LayerNode) ([]byte, error) {
			return doc.renderLayerToSurface(layer)
		},
		RenderComposite: func() []byte {
			return doc.renderCompositeSurface()
		},
	})
}

func exportDocumentPayload(doc *Document, format string) (string, error) {
	var data []byte
	var err error

	switch strings.ToLower(strings.TrimSpace(format)) {
	case "", "archive", "agp":
		data, err = SaveProjectZip(doc, nil)
	case "psd":
		data, err = SavePSD(doc)
	case "psb":
		data, err = SavePSB(doc)
	default:
		return "", fmt.Errorf("unsupported export format %q", format)
	}
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(data), nil
}
