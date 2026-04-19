package engine

import (
	"fmt"

	psdio "github.com/cwbudde/agogo-web/packages/engine-wasm/internal/io/psd"
	psdimport "github.com/cwbudde/agogo-web/packages/engine-wasm/internal/io/psdimport"
)

// LoadPSD parses a PSD/PSB byte stream and maps supported content into a Document.
func LoadPSD(data []byte) (*Document, []string, error) {
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
	if len(resources.AgogoProject) > 0 {
		doc, _, loadErr := LoadProject(resources.AgogoProject)
		if loadErr == nil {
			return doc, parser.Warnings(), nil
		}
		parserWarn := parser.Warnings()
		parserWarn = append(parserWarn, fmt.Sprintf("embedded Agogo project metadata ignored: %v", loadErr))
		return loadPSDFallback(data, header, resources, parser, parserWarn)
	}
	return loadPSDFallback(data, header, resources, parser, parser.Warnings())
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
