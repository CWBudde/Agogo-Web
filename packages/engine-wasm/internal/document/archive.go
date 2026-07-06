package document

import (
	"encoding/json"

	"github.com/cwbudde/agogo-web/packages/engine-wasm/internal/model"
)

type ViewportState struct {
	CenterX          float64 `json:"centerX"`
	CenterY          float64 `json:"centerY"`
	Zoom             float64 `json:"zoom"`
	Rotation         float64 `json:"rotation"`
	CanvasW          int     `json:"canvasW"`
	CanvasH          int     `json:"canvasH"`
	DevicePixelRatio float64 `json:"devicePixelRatio"`
	ShowGuides       bool    `json:"showGuides"`
}

type HistoryEntry struct {
	ID          int64  `json:"id"`
	Description string `json:"description"`
	State       string `json:"state"`
}

type RawRenderResult struct {
	FrameID   int64         `json:"frameId"`
	Viewport  ViewportState `json:"viewport"`
	BufferPtr int32         `json:"bufferPtr"`
	BufferLen int32         `json:"bufferLen"`
	Reused    bool          `json:"reused"`
	// Error reports a non-fatal render pipeline failure (e.g. layer
	// compositing failed). The frame buffer is still valid but may not
	// include document content. Empty when rendering succeeded.
	Error string `json:"error,omitempty"`
}

type ProjectArchive struct {
	Version  int                    `json:"version"`
	Document ProjectDocumentArchive `json:"document"`
	History  []HistoryEntry         `json:"history,omitempty"`
}

type ProjectDocumentArchive struct {
	Width           int                     `json:"width"`
	Height          int                     `json:"height"`
	Resolution      float64                 `json:"resolution"`
	ColorMode       string                  `json:"colorMode"`
	BitDepth        int                     `json:"bitDepth"`
	Background      Background              `json:"background"`
	ID              string                  `json:"id"`
	Name            string                  `json:"name"`
	CreatedAt       string                  `json:"createdAt"`
	CreatedBy       string                  `json:"createdBy"`
	ModifiedAt      string                  `json:"modifiedAt"`
	ActiveLayer     string                  `json:"activeLayerId,omitempty"`
	Layers          []ProjectLayerArchive   `json:"layers"`
	Paths           []NamedPath             `json:"paths,omitempty"`
	ActivePathIdx   int                     `json:"activePathIdx,omitempty"`
	SavedSelections []SavedSelectionChannel `json:"savedSelections,omitempty"`
	StylePresets    []DocumentStylePreset   `json:"stylePresets,omitempty"`
}

type ProjectLayerArchive struct {
	ID                string                `json:"id"`
	LayerType         model.LayerType       `json:"layerType"`
	Name              string                `json:"name"`
	Visible           bool                  `json:"visible"`
	LockMode          model.LayerLockMode   `json:"lockMode"`
	Opacity           float64               `json:"opacity"`
	FillOpacity       float64               `json:"fillOpacity"`
	BlendMode         model.BlendMode       `json:"blendMode"`
	ClipToBelow       bool                  `json:"clipToBelow"`
	ClippingBase      bool                  `json:"clippingBase"`
	Mask              *model.LayerMask      `json:"mask,omitempty"`
	VectorMask        *model.Path           `json:"vectorMask,omitempty"`
	StyleStack        []model.LayerStyle    `json:"styleStack,omitempty"`
	BlendIf           *model.BlendIfConfig  `json:"blendIf,omitempty"`
	Isolated          bool                  `json:"isolated,omitempty"`
	IsArtboard        bool                  `json:"isArtboard,omitempty"`
	ArtboardBounds    *model.LayerBounds    `json:"artboardBounds,omitempty"`
	ArtboardBG        *[4]uint8             `json:"artboardBackground,omitempty"`
	Bounds            *model.LayerBounds    `json:"bounds,omitempty"`
	Pixels            []byte                `json:"pixels,omitempty"`
	AdjustmentKind    string                `json:"adjustmentKind,omitempty"`
	Params            json.RawMessage       `json:"params,omitempty"`
	Text              string                `json:"text,omitempty"`
	FontFamily        string                `json:"fontFamily,omitempty"`
	FontStyle         string                `json:"fontStyle,omitempty"`
	FontSize          float64               `json:"fontSize,omitempty"`
	Bold              bool                  `json:"bold,omitempty"`
	Italic            bool                  `json:"italic,omitempty"`
	AntiAlias         string                `json:"antiAlias,omitempty"`
	Color             [4]uint8              `json:"color,omitempty"`
	TextType          string                `json:"textType,omitempty"`
	TextAlignment     string                `json:"textAlignment,omitempty"`
	BaselineShift     float64               `json:"baselineShift,omitempty"`
	TextLeading       float64               `json:"textLeading,omitempty"`
	TextTracking      float64               `json:"textTracking,omitempty"`
	TextKerning       float64               `json:"textKerning,omitempty"`
	TextLanguage      string                `json:"textLanguage,omitempty"`
	TextOrientation   string                `json:"textOrientation,omitempty"`
	TextSuperscript   bool                  `json:"textSuperscript,omitempty"`
	TextSubscript     bool                  `json:"textSubscript,omitempty"`
	TextUnderline     bool                  `json:"textUnderline,omitempty"`
	TextStrikethrough bool                  `json:"textStrikethrough,omitempty"`
	TextAllCaps       bool                  `json:"textAllCaps,omitempty"`
	TextSmallCaps     bool                  `json:"textSmallCaps,omitempty"`
	TextIndentLeft    float64               `json:"textIndentLeft,omitempty"`
	TextIndentRight   float64               `json:"textIndentRight,omitempty"`
	TextIndentFirst   float64               `json:"textIndentFirst,omitempty"`
	TextSpaceBefore   float64               `json:"textSpaceBefore,omitempty"`
	TextSpaceAfter    float64               `json:"textSpaceAfter,omitempty"`
	Shape             *model.Path           `json:"shape,omitempty"`
	FillColor         [4]uint8              `json:"fillColor,omitempty"`
	StrokeColor       [4]uint8              `json:"strokeColor,omitempty"`
	StrokeWidth       float64               `json:"strokeWidth,omitempty"`
	CachedRaster      []byte                `json:"cachedRaster,omitempty"`
	Children          []ProjectLayerArchive `json:"children,omitempty"`
}
