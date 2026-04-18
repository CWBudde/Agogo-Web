package engine

import (
	"bytes"
	"fmt"
)

// LoadPSD parses a PSD/PSB byte stream and maps supported content into a Document.
func LoadPSD(data []byte) (*Document, []string, error) {
	parser := &psdParser{r: bytes.NewReader(data)}

	header, err := parser.parseHeader()
	if err != nil {
		return nil, nil, err
	}
	if header.Depth != 8 {
		return nil, nil, fmt.Errorf("unsupported PSD bit depth %d", header.Depth)
	}
	if header.ColorMode != psdColorModeRGB && header.ColorMode != psdColorModeGrayscale {
		return nil, nil, fmt.Errorf("unsupported PSD color mode %d", header.ColorMode)
	}

	if err := parser.skipColorModeData(); err != nil {
		return nil, nil, err
	}
	resources, err := parser.parseImageResources()
	if err != nil {
		return nil, nil, err
	}
	if len(resources.AgogoProject) > 0 {
		doc, _, loadErr := LoadProject(resources.AgogoProject)
		if loadErr == nil {
			return doc, append([]string(nil), parser.warnings...), nil
		}
		parser.warnings = append(parser.warnings, fmt.Sprintf("embedded Agogo project metadata ignored: %v", loadErr))
	}
	layers, err := parser.parseLayerAndMaskInfo(header)
	if err != nil {
		if len(layers) == 0 {
			return nil, nil, err
		}
		parser.warnings = append(parser.warnings, fmt.Sprintf("partial layer info: %v", err))
	}
	compositeRGBA, err := parser.parseCompositeImageData(header)
	if err != nil {
		if len(layers) == 0 {
			return nil, nil, err
		}
		parser.warnings = append(parser.warnings, fmt.Sprintf("partial composite image: %v", err))
	}

	doc := newImportedPSDDocument(header, resources)
	importedLayers, warnings, err := buildPSDLayerNodes(header, layers)
	if err != nil {
		return nil, nil, err
	}
	parser.warnings = append(parser.warnings, warnings...)

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
	return doc, append([]string(nil), parser.warnings...), nil
}

func (p *psdParser) warnf(format string, args ...any) {
	p.warnings = append(p.warnings, fmt.Sprintf(format, args...))
}
