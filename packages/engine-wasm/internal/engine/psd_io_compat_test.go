package engine

import (
	"bytes"

	psdio "github.com/cwbudde/agogo-web/packages/engine-wasm/internal/io/psd"
)

const (
	psdPSDMaxDimension    = psdio.PSDMaxDimension
	psdColorModeGrayscale = psdio.ColorModeGrayscale
	psdColorModeRGB       = psdio.ColorModeRGB

	psdCompressionRaw           = psdio.CompressionRaw
	psdCompressionRLE           = psdio.CompressionRLE
	psdCompressionZip           = psdio.CompressionZip
	psdCompressionZipPrediction = psdio.CompressionZipPrediction
)

const (
	psdImageResourceDPI          = psdio.ImageResourceDPI
	psdImageResourceICCProfile   = psdio.ImageResourceICCProfile
	psdImageResourceGuides       = psdio.ImageResourceGuides
	psdImageResourceSlices       = psdio.ImageResourceSlices
	psdImageResourceLayerComps   = psdio.ImageResourceLayerComps
	psdImageResourceAgogoProject = psdio.ImageResourceAgogoProject
)

const (
	psdLayerSectionNormal      = psdio.LayerSectionNormal
	psdLayerSectionOpenFolder  = psdio.LayerSectionOpenFolder
	psdLayerSectionCloseFolder = psdio.LayerSectionCloseFolder
	psdLayerSectionNested      = psdio.LayerSectionNested
)

type (
	psdHeader      = psdio.Header
	psdLayerRecord = psdio.LayerRecord
)

type psdParser struct {
	r     *bytes.Reader
	inner *psdio.Parser
}

func (p *psdParser) parser() *psdio.Parser {
	if p.inner == nil {
		if p.r != nil {
			p.inner = psdio.NewParserFromReader(p.r)
		} else {
			p.inner = psdio.NewParser(nil)
		}
	}
	return p.inner
}

func (p *psdParser) parseHeader() (psdHeader, error) {
	return p.parser().ParseHeader()
}

func (p *psdParser) skipColorModeData() error {
	return p.parser().SkipColorModeData()
}

func (p *psdParser) parseImageResources() (psdio.ImageResources, error) {
	return p.parser().ParseImageResources()
}

func (p *psdParser) parseLayerAndMaskInfo(header psdHeader) ([]psdLayerRecord, error) {
	return p.parser().ParseLayerAndMaskInfo(header)
}

func (p *psdParser) parseCompositeImageData(header psdHeader) ([]byte, error) {
	return p.parser().ParseCompositeImageData(header)
}

func parsePSDLayerExtraData(data []byte, record *psdLayerRecord) error {
	return psdio.ParseLayerExtraData(data, record)
}
