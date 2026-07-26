package psd

import (
	"bytes"
	"encoding/json"

	"github.com/cwbudde/agogo-web/packages/engine-wasm/internal/model"
)

const (
	ColorModeGrayscale = 1
	ColorModeRGB       = 3
)

const (
	CompressionRaw           = 0
	CompressionRLE           = 1
	CompressionZip           = 2
	CompressionZipPrediction = 3
)

const (
	ImageResourceDPI          = 0x03ed
	ImageResourceICCProfile   = 0x040f
	ImageResourceGuides       = 0x0408
	ImageResourceSlices       = 0x041a
	ImageResourceLayerComps   = 0x0435
	ImageResourceAgogoProject = 0x0fa0
)

const (
	LayerSectionNormal      = 0
	LayerSectionOpenFolder  = 1
	LayerSectionCloseFolder = 2
	LayerSectionNested      = 3
)

const PSDMaxDimension = 30000

type Header struct {
	Version   uint16
	PSB       bool
	Channels  int
	Height    int
	Width     int
	Depth     int
	ColorMode int
}

type Parser struct {
	r        *bytes.Reader
	warnings []string
}

type ImageResources struct {
	Resolution    float64
	HasICCProfile bool
	HasGuides     bool
	HasSlices     bool
	HasLayerComps bool
	AgogoProject  []byte
}

type LayerEffectsMeta struct {
	Legacy *LegacyLayerEffectsMeta
	Object *ObjectLayerEffectsMeta
}

type LegacyLayerEffectsMeta struct {
	Version     uint16
	EffectCount uint16
	EffectKeys  []string
	Malformed   bool
}

type ObjectLayerEffectsMeta struct {
	ObjectVersion     uint32
	DescriptorVersion uint32
	HasDescriptor     bool
	Malformed         bool
	EffectKeys        []string
}

type AdjustmentMeta struct {
	Key        string
	Kind       string
	Version    uint16
	HasVersion bool
	PayloadLen int
	Malformed  bool
}

type SmartObjectMeta struct {
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

type VectorMaskMeta struct {
	Key          string
	PayloadLen   int
	HasBounds    bool
	Bounds       model.LayerBounds
	DefaultColor uint16
	Flags        uint16
	Malformed    bool
}

type TextLayerMeta struct {
	Key               string
	PayloadLen        int
	ParsedText        string
	DescriptorVersion uint32
	HasDescriptor     bool
	Malformed         bool
}

type LayerRecord struct {
	Name              string
	Bounds            model.LayerBounds
	Channels          []ChannelInfo
	Opacity           float64
	Visible           bool
	ClipToBelow       bool
	BlendMode         model.BlendMode
	LayerID           uint32
	LayerColorTag     string
	SectionType       uint32
	HasLayerMask      bool
	LayerMaskBounds   model.LayerBounds
	LayerMaskEnabled  bool
	HasVectorMask     bool
	VectorMask        *VectorMaskMeta
	Effects           *LayerEffectsMeta
	Adjustments       []AdjustmentMeta
	SmartObject       *SmartObjectMeta
	Text              *TextLayerMeta
	ChannelPixels     map[int16][]byte
	UnsupportedBlocks []string
	MetadataWarnings  []string
}

type ChannelInfo struct {
	ID     int16
	Length uint64
}

type ParseResult struct {
	Header        Header
	Resources     ImageResources
	Layers        []LayerRecord
	CompositeRGBA []byte
	Warnings      []string
}

type ImageResourceBlock struct {
	ID      uint16
	Name    string
	Payload []byte
}

type ExportLayerRecord struct {
	Name        string
	Bounds      model.LayerBounds
	Opacity     uint8
	Visible     bool
	ClipToBelow bool
	BlendKey    string
	SectionType uint32
	Mask        *model.LayerMask
	Channels    []ExportChannel
	ExtraBlocks []ExportTaggedBlock
}

type ExportChannel struct {
	ID      int16
	Length  uint64
	Payload []byte
}

type ExportTaggedBlock struct {
	Signature string
	Key       string
	Payload   []byte
}

type WriteParams struct {
	PSB            bool
	Width          int
	Height         int
	ChannelCount   int
	ColorMode      int
	Depth          int
	ImageResources []ImageResourceBlock
	Layers         []ExportLayerRecord
	CompositeData  []byte
}

type DescriptorItem struct {
	Key     string
	Type    string
	Text    string
	Bool    bool
	Float64 float64
	Int32   int32
}

type TextLayerPayload struct {
	Bounds        model.LayerBounds
	Text          string
	FontFamily    string
	FontStyle     string
	FontSize      float64
	Bold          bool
	Italic        bool
	AntiAlias     string
	Color         [4]uint8
	TextType      string
	Alignment     string
	BaselineShift float64
	Leading       float64
	Tracking      float64
	Kerning       float64
	Language      string
	Orientation   string
	Superscript   bool
	Subscript     bool
	Underline     bool
	Strikethrough bool
	AllCaps       bool
	SmallCaps     bool
	IndentLeft    float64
	IndentRight   float64
	IndentFirst   float64
	SpaceBefore   float64
	SpaceAfter    float64
}

type AdjustmentLayerPayload struct {
	Kind   string
	Params json.RawMessage
}
