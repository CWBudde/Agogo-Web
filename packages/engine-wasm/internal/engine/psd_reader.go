package engine

import (
	"fmt"

	psdio "github.com/cwbudde/agogo-web/packages/engine-wasm/internal/io/psd"
	psdimport "github.com/cwbudde/agogo-web/packages/engine-wasm/internal/io/psdimport"
)

// PSDLoadOptions controls optional Agogo-specific import behavior.
type PSDLoadOptions struct {
	// IgnoreEmbeddedProject returns only the document reconstructed from the PSD
	// structures. The embedded AgogoProject resource is a fidelity bonus; it
	// never substitutes for successfully parsing the PSD itself.
	IgnoreEmbeddedProject bool
}

// LoadPSD parses a PSD/PSB byte stream and maps supported content into a Document.
func LoadPSD(data []byte) (*Document, []string, error) {
	return LoadPSDWithOptions(data, PSDLoadOptions{})
}

// LoadPSDWithOptions parses a PSD/PSB byte stream and maps supported content
// into a Document. PSD structures are always parsed before the optional
// AgogoProject resource is considered.
func LoadPSDWithOptions(data []byte, options PSDLoadOptions) (doc *Document, warnings []string, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			doc = nil
			warnings = nil
			err = fmt.Errorf("invalid PSD data: %v", recovered)
		}
	}()
	parser := psdio.NewParser(data)

	header, err := parser.ParseHeader()
	if err != nil {
		return nil, nil, err
	}
	if header.Depth != 8 {
		return nil, nil, fmt.Errorf("unsupported PSD bit depth %d", header.Depth)
	}
	if header.ColorMode != psdio.ColorModeRGB && header.ColorMode != psdio.ColorModeGrayscale {
		return nil, nil, fmt.Errorf("unsupported PSD color mode %d", header.ColorMode)
	}

	if err := parser.SkipColorModeData(); err != nil {
		return nil, nil, err
	}
	resources, err := parser.ParseImageResources()
	if err != nil {
		return nil, nil, err
	}
	parsedDoc, warnings, err := loadPSDFallback(data, header, resources, parser, parser.Warnings())
	if err != nil {
		return nil, warnings, err
	}
	if len(resources.AgogoProject) > 0 && !options.IgnoreEmbeddedProject {
		doc, _, loadErr := LoadProject(resources.AgogoProject)
		if loadErr == nil {
			return doc, warnings, nil
		}
		warnings = append(warnings, fmt.Sprintf("embedded Agogo project metadata ignored: %v", loadErr))
	}
	return parsedDoc, warnings, nil
}

func loadPSDFallback(_ []byte, header psdio.Header, resources psdio.ImageResources, parser *psdio.Parser, warnings []string) (*Document, []string, error) {
	layers, err := parser.ParseLayerAndMaskInfo(header)
	if err != nil {
		if len(layers) == 0 {
			return nil, nil, err
		}
		warnings = append(warnings, fmt.Sprintf("partial layer info: %v", err))
	}
	compositeRGBA, err := parser.ParseCompositeImageData(header)
	if err != nil {
		if len(layers) == 0 {
			return nil, nil, err
		}
		warnings = append(warnings, fmt.Sprintf("partial composite image: %v", err))
	}

	doc := newImportedPSDDocument(header, resources)
	importedLayers, importWarnings, err := psdimport.BuildLayerNodes(header, layers)
	if err != nil {
		return nil, nil, err
	}
	warnings = append(warnings, importWarnings...)

	if len(importedLayers) == 0 && len(compositeRGBA) > 0 {
		importedLayers = append(importedLayers, NewPixelLayer("Background", LayerBounds{
			X: 0, Y: 0, W: header.Width, H: header.Height,
		}, compositeRGBA))
	}
	doc.LayerRoot.SetChildren(importedLayers)
	doc.normalizeClippingState()
	if len(importedLayers) > 0 {
		doc.ActiveLayerID = importedLayers[len(importedLayers)-1].ID()
	}
	return doc, append([]string(nil), warnings...), nil
}
