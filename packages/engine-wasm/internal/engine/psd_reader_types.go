package engine

import "bytes"

const (
	psdColorModeGrayscale = 1
	psdColorModeRGB       = 3

	psdCompressionRaw = iota
	psdCompressionRLE
	psdCompressionZip
	psdCompressionZipPrediction
)

const (
	psdImageResourceDPI          = 0x03ed
	psdImageResourceICCProfile   = 0x040f
	psdImageResourceGuides       = 0x0408
	psdImageResourceSlices       = 0x041a
	psdImageResourceLayerComps   = 0x0435
	psdImageResourceAgogoProject = 0x0fa0
)

const (
	psdLayerSectionNormal      = 0
	psdLayerSectionOpenFolder  = 1
	psdLayerSectionCloseFolder = 2
	psdLayerSectionNested      = 3
)

type psdHeader struct {
	Version   uint16
	PSB       bool
	Channels  int
	Height    int
	Width     int
	Depth     int
	ColorMode int
}

type psdParser struct {
	r        *bytes.Reader
	warnings []string
}

type psdImageResources struct {
	Resolution    float64
	HasICCProfile bool
	HasGuides     bool
	HasSlices     bool
	HasLayerComps bool
	AgogoProject  []byte
}

type psdLayerEffectsMeta struct {
	Legacy *psdLegacyLayerEffectsMeta
	Object *psdObjectLayerEffectsMeta
}

type psdLegacyLayerEffectsMeta struct {
	Version     uint16
	EffectCount uint16
	EffectKeys  []string
	Malformed   bool
}

type psdObjectLayerEffectsMeta struct {
	ObjectVersion     uint32
	DescriptorVersion uint32
	HasDescriptor     bool
	Malformed         bool
	EffectKeys        []string
}

type psdAdjustmentMeta struct {
	Key        string
	Kind       string
	Version    uint16
	HasVersion bool
	PayloadLen int
	Malformed  bool
}

type psdSmartObjectMeta struct {
	Key           string
	Version       uint32
	Identifier    string
	UniqueID      string
	PayloadLen    int
	HasDescriptor bool
	HasVersion    bool
	Malformed     bool
	PageNumber    *uint32
	TotalPages    *uint32
	PlacedType    *uint32
}

type psdVectorMaskMeta struct {
	Key          string
	PayloadLen   int
	HasBounds    bool
	Bounds       LayerBounds
	DefaultColor uint16
	Flags        uint16
	Malformed    bool
}

type psdTextLayerMeta struct {
	Key               string
	PayloadLen        int
	ParsedText        string
	DescriptorVersion uint32
	HasDescriptor     bool
	Malformed         bool
}

type psdLayerRecord struct {
	Name              string
	Bounds            LayerBounds
	Channels          []psdChannelInfo
	Opacity           float64
	Visible           bool
	ClipToBelow       bool
	BlendMode         BlendMode
	LayerID           uint32
	LayerColorTag     string
	SectionType       uint32
	HasLayerMask      bool
	LayerMaskBounds   LayerBounds
	LayerMaskEnabled  bool
	HasVectorMask     bool
	VectorMask        *psdVectorMaskMeta
	Effects           *psdLayerEffectsMeta
	Adjustments       []psdAdjustmentMeta
	SmartObject       *psdSmartObjectMeta
	Text              *psdTextLayerMeta
	ChannelPixels     map[int16][]byte
	UnsupportedBlocks []string
	MetadataWarnings  []string
}

type psdChannelInfo struct {
	ID     int16
	Length uint64
}
