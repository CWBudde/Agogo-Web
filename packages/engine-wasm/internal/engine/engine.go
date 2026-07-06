// Package engine is the core of the Agogo image editor backend.
// Phase 1 adds document, viewport, history, and a JSON command bridge.
package engine

import (
	"encoding/json"
	"fmt"
	"sync"
	"unsafe"

	agglib "github.com/cwbudde/agg_go"
	cmdpkg "github.com/cwbudde/agogo-web/packages/engine-wasm/internal/command"
	"github.com/cwbudde/agogo-web/packages/engine-wasm/internal/model"
	runtimepkg "github.com/cwbudde/agogo-web/packages/engine-wasm/internal/runtime"
)

// thumbnailSize is the width and height of layer preview thumbnails in pixels.
const thumbnailSize = 32

//nolint:unused // command IDs are part of the engine ABI and are also referenced from tests.
const (
	commandCreateDocument            = 0x0001
	commandCloseDocument             = 0x0002
	commandZoomSet                   = 0x0010
	commandPanSet                    = 0x0011
	commandRotateViewSet             = 0x0012
	commandResize                    = 0x0013
	commandFitToView                 = 0x0014
	commandPointerEvent              = 0x0015
	commandJumpHistory               = 0x0016
	commandSetShowGuides             = 0x0017
	commandAddLayer                  = 0x0100
	commandDeleteLayer               = 0x0101
	commandMoveLayer                 = 0x0102
	commandSetLayerVis               = 0x0103
	commandSetLayerOp                = 0x0104
	commandSetLayerBlend             = 0x0105
	commandDuplicateLayer            = 0x0106
	commandSetLayerLock              = 0x0107
	commandFlattenLayer              = 0x0108
	commandMergeDown                 = 0x0109
	commandMergeVisible              = 0x010a
	commandAddLayerMask              = 0x010b
	commandDeleteLayerMask           = 0x010c
	commandApplyLayerMask            = 0x010d
	commandInvertLayerMask           = 0x010e
	commandSetMaskEnabled            = 0x010f
	commandSetLayerClip              = 0x0110
	commandSetActiveLayer            = 0x0111
	commandSetLayerName              = 0x0112
	commandAddVectorMask             = 0x0113
	commandDeleteVectorMask          = 0x0114
	commandSetMaskEditMode           = 0x0115
	commandGetLayerThumbnails        = 0x0116
	commandFlattenImage              = 0x0117
	commandOpenImageFile             = 0x0118
	commandTranslateLayer            = 0x0119
	commandPickLayerAtPoint          = 0x011a
	commandSetAdjustmentParams       = 0x011b
	commandNewSelection              = 0x0200
	commandSelectAll                 = 0x0201
	commandDeselect                  = 0x0202
	commandReselect                  = 0x0203
	commandInvertSelection           = 0x0204
	commandFeatherSelection          = 0x0205
	commandExpandSelection           = 0x0206
	commandContractSelection         = 0x0207
	commandSmoothSelection           = 0x0208
	commandBorderSelection           = 0x0209
	commandTransformSelection        = 0x020a
	commandSelectColorRange          = 0x020b
	commandQuickSelect               = 0x020c
	commandMagicWand                 = 0x020d
	commandMagneticLassoSuggestPath  = 0x020e
	commandSaveSelectionToChannel    = 0x020f
	commandLoadSelectionFromChannel  = 0x0210
	commandRefineSelection           = 0x0211
	commandOutputSelection           = 0x0212
	commandSetSelectionViewMode      = 0x0213
	commandBeginFreeTransform        = 0x0300
	commandUpdateFreeTransform       = 0x0301
	commandCommitFreeTransform       = 0x0302
	commandCancelFreeTransform       = 0x0303
	commandFlipLayerH                = 0x0304
	commandFlipLayerV                = 0x0305
	commandRotateLayer90CW           = 0x0306
	commandRotateLayer90CCW          = 0x0307
	commandRotateLayer180            = 0x0308
	commandTransformAgain            = 0x0309
	commandBeginCrop                 = 0x0320
	commandUpdateCrop                = 0x0321
	commandCommitCrop                = 0x0322
	commandCancelCrop                = 0x0323
	commandResizeCanvas              = 0x0324
	commandBeginPaintStroke          = 0x0400
	commandContinuePaintStroke       = 0x0401
	commandEndPaintStroke            = 0x0402
	commandSetForegroundColor        = 0x0410
	commandSetBackgroundColor        = 0x0411
	commandSampleMergedColor         = 0x0412
	commandMagicErase                = 0x0413
	commandFill                      = 0x0414
	commandApplyGradient             = 0x0415
	commandResetMixerBrushState      = 0x0416
	commandComputeHistogram          = 0x011c
	commandSetPointFromSample        = 0x011d
	commandIdentifyHueRange          = 0x011e
	commandSetLayerStyleStack        = 0x011f
	commandSetLayerStyleEnabled      = 0x0120
	commandSetLayerStyleParams       = 0x0121
	commandCopyLayerStyle            = 0x0122
	commandPasteLayerStyle           = 0x0123
	commandClearLayerStyle           = 0x0124
	commandCreateDocumentStylePreset = 0x0125
	commandUpdateDocumentStylePreset = 0x0126
	commandDeleteDocumentStylePreset = 0x0127
	commandApplyDocumentStylePreset  = 0x0128
	commandSetArtboard               = 0x0129
	commandApplyFilter               = 0x0500
	commandReapplyFilter             = 0x0501
	commandPreviewFilter             = 0x0502
	commandCancelFilterPreview       = 0x0503
	commandCommitFilterPreview       = 0x0504
	commandFadeFilter                = 0x0505

	// Phase 6.1: Vector Path
	commandSetActiveTool         = 0x0600
	commandPenToolClick          = 0x0601
	commandPenToolClose          = 0x0602
	commandDirectSelectMove      = 0x0603
	commandDirectSelectMarquee   = 0x0604
	commandBreakHandle           = 0x0605
	commandDeleteAnchor          = 0x0606
	commandAddAnchorOnSegment    = 0x0607
	commandPathCombine           = 0x0610
	commandPathSubtract          = 0x0611
	commandPathIntersect         = 0x0612
	commandPathExclude           = 0x0613
	commandFlattenPath           = 0x0614
	commandRasterizePath         = 0x0615
	commandRasterizeLayer        = 0x0616
	commandCreatePath            = 0x0620
	commandDeletePath            = 0x0621
	commandRenamePath            = 0x0622
	commandDuplicatePath         = 0x0623
	commandMakeSelectionFromPath = 0x0624
	commandStrokePath            = 0x0625
	commandFillPath              = 0x0626

	// Phase 6.2: Shape Tools
	commandDrawShape           = 0x0630
	commandEnterVectorEditMode = 0x0631
	commandCommitVectorEdit    = 0x0632
	commandSetVectorLayerStyle = 0x0633

	// Phase 6.3: Text Engine
	commandAddTextLayer      = 0x0640 // create text layer at point, enter edit mode
	commandSetTextContent    = 0x0641 // replace text string + re-rasterize
	commandSetTextStyle      = 0x0642 // update font/size/color/alignment + re-rasterize
	commandEnterTextEditMode = 0x0643 // enter text editing (double-click)
	commandTextEditInput     = 0x0644 // update working text from frontend keyboard input
	commandCommitTextEdit    = 0x0645 // finalize edit (Escape / click-outside)
	commandConvertTextToPath = 0x0646 // Type > Create Outlines → new VectorLayer

	commandBeginTxn     = 0xffe0 //nolint:unused // kept for command ABI coverage in tests
	commandEndTxn       = 0xffe1 //nolint:unused // kept for command ABI coverage in tests
	commandClearHistory = 0xffe2 //nolint:unused // kept for command ABI coverage in tests
	commandUndo         = 0xfff0 //nolint:unused // kept for command ABI coverage in tests
	commandRedo         = 0xfff1 //nolint:unused // kept for command ABI coverage in tests
)

type Document struct {
	Width             int                     `json:"width"`
	Height            int                     `json:"height"`
	Resolution        float64                 `json:"resolution"`
	ColorMode         string                  `json:"colorMode"`
	BitDepth          int                     `json:"bitDepth"`
	Background        Background              `json:"background"`
	ID                string                  `json:"id"`
	Name              string                  `json:"name"`
	CreatedAt         string                  `json:"createdAt"`
	CreatedBy         string                  `json:"createdBy"`
	ModifiedAt        string                  `json:"modifiedAt"`
	ActiveLayerID     string                  `json:"activeLayerId,omitempty"`
	LayerRoot         *GroupLayer             `json:"-"`
	Selection         *Selection              `json:"-"`
	LastSelection     *Selection              `json:"-"`
	SavedSelections   []SavedSelectionChannel `json:"-"`
	ContentVersion    int64                   `json:"-"` // monotonic counter; not persisted, used only for composite cache invalidation
	dirtyComposite    DirtyRect               `json:"-"`
	hasDirtyComposite bool                    `json:"-"`
	// dirtyCompositeBase is the ContentVersion the accumulated dirtyComposite
	// rect is relative to (set whenever the rect is cleared after a successful
	// composite). Content versions are globally unique, so an incremental
	// recomposite of a cached surface is valid only when the cache's version
	// equals this base — snapshot restores clone documents together with their
	// dirty state, and without this check a restored stale rect could be
	// mistaken for the delta against an unrelated cached surface.
	dirtyCompositeBase int64                   `json:"-"`
	Paths              []NamedPath             `json:"-"`
	ActivePathIdx      int                     `json:"-"`
	StylePresets       []DocumentStylePreset   `json:"-"`
	Patterns           []model.PatternResource `json:"-"`
}

type UIMeta struct {
	// Version is the engine's uiMetaVersion counter at the time this meta was
	// built. The frontend compares it against RenderResult.UIMetaVersion from
	// hot-path acks to detect when its cached UIMeta went stale.
	Version             int64           `json:"version"`
	ActiveLayerID       string          `json:"activeLayerId"`
	ActiveLayerName     string          `json:"activeLayerName"`
	CursorType          string          `json:"cursorType"`
	StatusText          string          `json:"statusText"`
	ImportWarnings      []string        `json:"importWarnings,omitempty"`
	RulerOriginX        float64         `json:"rulerOriginX"`
	RulerOriginY        float64         `json:"rulerOriginY"`
	History             []HistoryEntry  `json:"history"`
	CanUndo             bool            `json:"canUndo"`
	CanRedo             bool            `json:"canRedo"`
	CurrentHistoryIndex int             `json:"currentHistoryIndex"`
	ActiveDocumentID    string          `json:"activeDocumentId"`
	ActiveDocumentName  string          `json:"activeDocumentName"`
	DocumentWidth       int             `json:"documentWidth"`
	DocumentHeight      int             `json:"documentHeight"`
	DocumentBackground  string          `json:"documentBackground"`
	Layers              []LayerNodeMeta `json:"layers"`
	// ContentVersion is a monotonic counter incremented on every document mutation.
	// The UI uses this to know when to refresh layer thumbnails.
	ContentVersion int64 `json:"contentVersion"`
	// MaskEditLayerID is set when the user is actively editing a layer mask.
	// The UI uses this to show the mask-edit border indicator.
	MaskEditLayerID        string                      `json:"maskEditLayerId,omitempty"`
	Selection              SelectionMeta               `json:"selection"`
	SavedSelectionChannels []SavedSelectionChannelMeta `json:"savedSelectionChannels,omitempty"`
	FreeTransform          *FreeTransformMeta          `json:"freeTransform,omitempty"`
	Crop                   *CropMeta                   `json:"crop,omitempty"`
	Paths                  []PathMeta                  `json:"paths,omitempty"`
	PathOverlay            *PathOverlay                `json:"pathOverlay,omitempty"`
	// EditingVectorLayerID is non-empty while a VectorLayer's path is being
	// edited. The UI uses this to show the "editing path" indicator.
	EditingVectorLayerID string `json:"editingVectorLayerId,omitempty"`
	// EditingTextLayerID is non-empty while a TextLayer is in text edit mode.
	EditingTextLayerID string `json:"editingTextLayerId,omitempty"`
	// TextCursorX/Y are doc-space coordinates of the text insertion cursor.
	// Only meaningful when EditingTextLayerID is set.
	TextCursorX  float64               `json:"textCursorX,omitempty"`
	TextCursorY  float64               `json:"textCursorY,omitempty"`
	StylePresets []DocumentStylePreset `json:"stylePresets,omitempty"`
	// Patterns lists the fill patterns available in the active document
	// (builtins followed by document-defined patterns).
	Patterns []PatternMeta `json:"patterns,omitempty"`
}

type RenderResult struct {
	FrameID     int64         `json:"frameId"`
	Viewport    ViewportState `json:"viewport"`
	DirtyRects  []DirtyRect   `json:"dirtyRects,omitempty"`
	PixelFormat string        `json:"pixelFormat,omitempty"`
	BufferPtr   int32         `json:"bufferPtr"`
	BufferLen   int32         `json:"bufferLen"`
	// UIMeta is nil for hot-path command acks (ContinuePaintStroke and
	// move-phase PointerEvent) — see DispatchCommand. Everything else,
	// including RenderFrame, carries the full meta.
	UIMeta *UIMeta `json:"uiMeta,omitempty"`
	// CursorType/StatusText mirror the equally-named UIMeta fields on hot-path
	// acks (hover/pan moves must still update the cursor and status line even
	// when the full UIMeta is skipped). Empty on full results.
	CursorType string `json:"cursorType,omitempty"`
	StatusText string `json:"statusText,omitempty"`
	// UIMetaVersion is the engine's monotonic UI-meta counter. On acks the
	// frontend compares it with the version inside its last full UIMeta and
	// schedules a RenderFrame refresh when they differ.
	UIMetaVersion int64 `json:"uiMetaVersion"`
	// ContentVersion mirrors the active document's content counter on acks.
	ContentVersion int64 `json:"contentVersion,omitempty"`
	// Thumbnails is non-nil only in the response to commandGetLayerThumbnails.
	Thumbnails map[string]ThumbnailEntry `json:"thumbnails,omitempty"`
	// SuggestedPath is set only in response to commandMagneticLassoSuggestPath.
	SuggestedPath []SelectionPoint `json:"suggestedPath,omitempty"`
	// SampledColor is set only in response to commandSampleMergedColor.
	SampledColor *[4]uint8 `json:"sampledColor,omitempty"`
	// Histogram is set only in response to commandComputeHistogram.
	Histogram *HistogramData `json:"histogram,omitempty"`
	// IdentifiedHueRange is set only in response to commandIdentifyHueRange.
	IdentifiedHueRange string `json:"identifiedHueRange,omitempty"`
	// Error reports a non-fatal render pipeline failure (e.g. layer
	// compositing failed). The frame buffer is still valid but may not
	// include document content. Empty when rendering succeeded.
	Error string `json:"error,omitempty"`
}

type EngineConfig struct {
	DocumentWidth  int     `json:"documentWidth"`
	DocumentHeight int     `json:"documentHeight"`
	Background     string  `json:"background"`
	Resolution     float64 `json:"resolution"`
}

type CreateDocumentPayload struct {
	Name       string  `json:"name"`
	Width      int     `json:"width"`
	Height     int     `json:"height"`
	Resolution float64 `json:"resolution"`
	ColorMode  string  `json:"colorMode"`
	BitDepth   int     `json:"bitDepth"`
	Background string  `json:"background"`
}

type ZoomPayload struct {
	Zoom      float64 `json:"zoom"`
	HasAnchor bool    `json:"hasAnchor"`
	AnchorX   float64 `json:"anchorX"`
	AnchorY   float64 `json:"anchorY"`
}

type PanPayload struct {
	CenterX float64 `json:"centerX"`
	CenterY float64 `json:"centerY"`
}

type RotatePayload struct {
	Rotation float64 `json:"rotation"`
}

type ResizePayload struct {
	CanvasW          int     `json:"canvasW"`
	CanvasH          int     `json:"canvasH"`
	DevicePixelRatio float64 `json:"devicePixelRatio"`
}

type PointerEventPayload struct {
	Phase     string  `json:"phase"`
	PointerID int     `json:"pointerId"`
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	Button    int     `json:"button"`
	Buttons   int     `json:"buttons"`
	PanMode   bool    `json:"panMode"`
	Pressure  float64 `json:"pressure"` // 0.0–1.0; 0.5 if device has no pressure
}

type SetColorPayload struct {
	Color [4]uint8 `json:"color"` // [R, G, B, A]
}

// SampleMergedColorPayload requests the RGBA color of the composite at a
// document-space position. The result is returned in RenderResult.SampledColor.
type SampleMergedColorPayload struct {
	X            float64 `json:"x"`
	Y            float64 `json:"y"`
	SampleSize   int     `json:"sampleSize,omitempty"`
	SampleMerged bool    `json:"sampleMerged,omitempty"`
}

// MagicErasePayload describes a one-click flood-clear by color similarity.
type MagicErasePayload struct {
	X            float64 `json:"x"` // document-space click position
	Y            float64 `json:"y"`
	Tolerance    float64 `json:"tolerance"`    // 0–255 Euclidean RGB distance
	Contiguous   bool    `json:"contiguous"`   // true = flood-fill, false = all matching pixels
	SampleMerged bool    `json:"sampleMerged"` // sample composite instead of active layer
}

type FillPayload struct {
	HasPoint     bool     `json:"hasPoint,omitempty"`
	X            float64  `json:"x,omitempty"`
	Y            float64  `json:"y,omitempty"`
	Tolerance    float64  `json:"tolerance,omitempty"`
	Contiguous   bool     `json:"contiguous,omitempty"`
	SampleMerged bool     `json:"sampleMerged,omitempty"`
	Source       string   `json:"source,omitempty"`
	Color        [4]uint8 `json:"color,omitempty"`
	CreateLayer  bool     `json:"createLayer,omitempty"`
	// PatternID selects a pattern when Source is "pattern"; empty or unknown
	// IDs fall back to the legacy foreground/background 8px checker.
	PatternID string `json:"patternId,omitempty"`
	// PatternScale scales the pattern tile; 0 (or negative) means 1.
	PatternScale float64 `json:"patternScale,omitempty"`
}

type GradientType string

const (
	GradientTypeLinear    GradientType = "linear"
	GradientTypeRadial    GradientType = "radial"
	GradientTypeAngle     GradientType = "angle"
	GradientTypeReflected GradientType = "reflected"
	GradientTypeDiamond   GradientType = "diamond"
)

type ApplyGradientPayload struct {
	StartX      float64               `json:"startX"`
	StartY      float64               `json:"startY"`
	EndX        float64               `json:"endX"`
	EndY        float64               `json:"endY"`
	Type        GradientType          `json:"type"`
	Reverse     bool                  `json:"reverse,omitempty"`
	Dither      bool                  `json:"dither,omitempty"`
	CreateLayer bool                  `json:"createLayer,omitempty"`
	Stops       []GradientStopPayload `json:"stops,omitempty"`
}

type GradientStopPayload struct {
	Position float64  `json:"position"`
	Color    [4]uint8 `json:"color"`
}

type BeginPaintStrokePayload struct {
	X        float64     `json:"x"`
	Y        float64     `json:"y"`
	Pressure float64     `json:"pressure"`
	TiltX    float64     `json:"tiltX"` // PointerEvent.tiltX degrees (−90…+90); 0 = upright
	TiltY    float64     `json:"tiltY"` // PointerEvent.tiltY degrees (−90…+90); 0 = upright
	Brush    BrushParams `json:"brush"`
}

type ContinuePaintStrokePayload struct {
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Pressure float64 `json:"pressure"`
	TiltX    float64 `json:"tiltX"` // PointerEvent.tiltX degrees (−90…+90); 0 = upright
	TiltY    float64 `json:"tiltY"` // PointerEvent.tiltY degrees (−90…+90); 0 = upright
}

type BeginTransactionPayload struct {
	Description string `json:"description"`
}

type EndTransactionPayload struct {
	Commit bool `json:"commit"`
}

type JumpHistoryPayload struct {
	HistoryIndex int `json:"historyIndex"`
}

type SetShowGuidesPayload struct {
	Show bool `json:"show"`
}

// activePaintStroke holds per-stroke state while painting is in progress.
type activePaintStroke struct {
	layerID string
	// rejected is set when the stroke was refused at begin time (e.g. the
	// target layer's pixels are locked). Continue points are ignored and
	// handleEndPaintStroke surfaces this error over the ABI — the
	// begin/continue dispatch path discards return values.
	rejected             error
	params               BrushParams
	strokeState          brushStrokeState
	stabilizer           stabilizerState
	dirtyMin             [2]int // min corner of painted dirty rect (layer-local)
	dirtyMax             [2]int // max corner of painted dirty rect (layer-local)
	hasDirty             bool
	bgEraseBaseColor     [4]uint8 // sampled once at stroke begin for background eraser
	mixerSource          []byte   // sampled once at stroke begin for mixer brush
	mixerSourceW         int
	mixerSourceH         int
	mixerSourceX         int
	mixerSourceY         int
	cloneSource          []byte // sampled once at stroke begin for clone stamp
	cloneSourceW         int
	cloneSourceH         int
	cloneSourceX         int
	cloneSourceY         int
	cloneOffsetX         float64
	cloneOffsetY         float64
	cloneRemainingLoad   float64
	historySource        []byte // sampled once at stroke begin for history brush
	historySourceW       int
	historySourceH       int
	historySourceX       int
	historySourceY       int
	historyRemainingLoad float64
	mixer                mixerBrushState
	lastDabX             float64
	lastDabY             float64
	lastDirX             float64
	lastDirY             float64
	hasLastDab           bool
	// renderer is a reusable AGG context for the stroke's layer. Created once at
	// stroke begin and reused across all dabs so the rasterizer's internal cell
	// blocks stay allocated instead of being re-allocated per dab.
	renderer *agglib.Agg2D
	// Lazy row-saving for undo: instead of snapshotting the entire layer at
	// stroke begin, we save only the rows that the dirty rect touches, captured
	// before each dab paints over them.  The buffer is provided by instance and
	// reused across strokes to avoid per-stroke allocations.
	beforeRowBuf   []byte // contiguous pixel data for saved rows
	beforeRowStart int    // first saved row (layer-local Y)
	beforeRowEnd   int    // exclusive end row
	layerW         int    // layer width in pixels (for row stride)
}

type mixerBrushState struct {
	docID          string
	reservoirColor [4]uint8
	remainingLoad  float64
	contamination  float64
	bristleColors  [7][4]uint8
	bristleLoads   [7]float64
	clean          bool
}

type cloneStampState struct {
	docID            string
	sourceX          float64
	sourceY          float64
	offsetX          float64
	offsetY          float64
	hasAlignedOffset bool
}

type pointerDragState struct {
	PointerID int
	StartX    float64
	StartY    float64
	CenterX   float64
	CenterY   float64
	Zoom      float64
	Rotation  float64
	Active    bool
}

type DocumentManager struct {
	*runtimepkg.Manager[*Document]
}

func newDocumentManager() *DocumentManager {
	return &DocumentManager{
		Manager: runtimepkg.NewManager(cloneDocument, func(doc *Document) string {
			if doc == nil {
				return ""
			}
			return doc.ID
		}),
	}
}

// activeMut returns the stored document directly without cloning.
// Callers may modify the returned document in place; it is the caller's
// responsibility to ensure the mutation is intentional (e.g. direct pixel
// painting during a brush stroke). Most code should use Active() instead.
func (m *DocumentManager) activeMut() *Document {
	if m == nil || m.Manager == nil {
		return nil
	}
	return m.ActiveMut()
}

// viewportBaseKey captures everything that affects the rendered background
// (checkerboard/solid fill, document shell). When unchanged between frames,
// the cached background buffer is memcpy'd instead of re-rendered through AGG.
type viewportBaseKey struct {
	DocWidth   int
	DocHeight  int
	Background string
	CenterX    float64
	CenterY    float64
	Zoom       float64
	Rotation   float64
	CanvasW    int
	CanvasH    int
}

type rawFrameKey struct {
	DocID            string
	ContentVersion   int64
	CenterX          float64
	CenterY          float64
	Zoom             float64
	Rotation         float64
	CanvasW          int
	CanvasH          int
	DevicePixelRatio float64
	ShowGuides       bool
}

type instance struct {
	pixels                []byte
	manager               *DocumentManager
	viewport              ViewportState
	cachedViewportBase    []byte
	cachedViewportBaseKey viewportBaseKey
	cachedRawFrameKey     rawFrameKey
	hasCachedRawFrame     bool
	// cachedAnimBase holds the fully composited + viewport-resampled frame
	// WITHOUT the animated selection overlay (marching ants). While a
	// selection is active the rAF loop reuses this base and only stamps the
	// current ants phase on top, avoiding a full recomposite every frame.
	cachedAnimBase    []byte
	cachedAnimBaseKey rawFrameKey
	hasCachedAnimBase bool
	// fullRecompositeCount counts full recomposite+viewport-resample passes in
	// renderRaw. Test-only instrumentation: lets tests assert the cheap
	// ants-only path did not trigger an expensive recomposite.
	fullRecompositeCount int64
	// incrementalCompositeCount counts in-place dirty-rect document
	// recomposites (compositeSurfaceChecked's incremental path). Test-only
	// instrumentation.
	incrementalCompositeCount int64
	// partialViewportUpdateCount counts partial viewport resamples in
	// renderRaw (content changed in a sub-rect, viewport unchanged).
	// Test-only instrumentation.
	partialViewportUpdateCount int64
	history                    *HistoryStack
	frameID                    int64
	pointer                    pointerDragState
	cachedDocSurface           []byte
	cachedDocID                string
	cachedDocContentVersion    int64
	// maskEditLayerID tracks which layer's mask is currently being edited.
	// This is UI state only — not included in history snapshots.
	maskEditLayerID string
	// freeTransform holds the live state while free transform is active.
	// It is UI-only state not included in history snapshots.
	freeTransform *FreeTransformState
	// selectionViewMode controls the Select and Mask preview rendering.
	// UI-only — not included in history snapshots.
	selectionViewMode SelectionViewMode
	// lastTransform records the most recently committed transform (free or
	// discrete) so that Transform Again can replay it on any layer.
	lastTransform *LastTransformRecord
	// crop holds the live state while the crop tool is active.
	crop *CropState
	// foregroundColor is the active foreground (paint) color.
	foregroundColor [4]uint8 // RGBA
	// backgroundColor is the active background color.
	backgroundColor [4]uint8 // RGBA
	// mixerBrush stores runtime-only wet-paint reservoir state for the Mixer Brush.
	// It is intentionally excluded from document snapshots and project files.
	mixerBrush mixerBrushState
	// cloneStamp stores runtime-only aligned-offset state for Clone Stamp.
	// It is intentionally excluded from document snapshots and project files.
	cloneStamp cloneStampState
	// paintStroke is non-nil while a brush stroke is in progress.
	paintStroke *activePaintStroke
	// undoRowBuf is a reusable buffer for stroke undo row snapshots.
	// Avoids allocating a new buffer every stroke.
	undoRowBuf []byte
	// lastFilter records the most recently applied destructive filter so
	// that ReapplyFilter can replay it on the active layer.
	lastFilter *lastFilterState
	// filterPreview holds the live preview state while a filter dialog is open.
	filterPreview *filterPreviewState
	// preFadeSnapshot stores pixel data before the last filter was applied,
	// enabling Filter > Fade to blend the result back with the original.
	preFadeSnapshot *fadeSnapshot
	// styleClipboard stores copied layer styles outside document history.
	styleClipboard styleClipboard
	// pathTool holds the pen / direct-selection tool UI state.
	pathTool *pathToolState
	// editingVectorLayerID is set while the user is editing a VectorLayer's
	// path via the direct-select tool. UI-only — not included in snapshots.
	editingVectorLayerID string
	// textEdit holds the in-flight state while a TextLayer is being edited.
	// UI-only — not included in history snapshots. Cleared on commit.
	textEdit textEditState
	// importWarnings stores non-fatal issues from the most recent external import.
	importWarnings []string
	// uiMetaVersion is a monotonic counter bumped by every DispatchCommand
	// EXCEPT the hot-path ones (ContinuePaintStroke, move-phase PointerEvent)
	// and by ImportProject. Hot commands CAN change state that the full UIMeta
	// reflects (painting bumps ContentVersion, a quick-select drag updates the
	// selection meta), but they deliberately do NOT bump this counter
	// mid-gesture: the gesture-ending non-hot command (pointer up,
	// EndPaintStroke, EndTransaction) bumps it and delivers the full refresh.
	// Panel lag during a drag is acceptable — the canvas shows the live
	// engine-rendered overlay via the rAF RenderFrameRaw loop.
	uiMetaVersion int64
}

// textEditState tracks the in-progress text edit for a single TextLayer.
type textEditState struct {
	layerID     string
	workingText string
}

var (
	mu             sync.Mutex
	nextID         int32 = 1
	nextDocID      int64 = 1
	nextDocVersion int64
	instances      = make(map[int32]*instance)
)

// Init allocates a new engine instance and returns its handle.
func Init(configJSON string) int32 {
	config := EngineConfig{}
	if configJSON != "" {
		_ = json.Unmarshal([]byte(configJSON), &config)
	}

	mu.Lock()
	defer mu.Unlock()

	id := nextID
	nextID++

	inst := &instance{
		manager: newDocumentManager(),
		viewport: ViewportState{
			Zoom:             1,
			CanvasW:          defaultDocWidth,
			CanvasH:          defaultDocHeight,
			DevicePixelRatio: defaultDevicePixelRat,
		},
		history:         newHistoryStack(defaultHistoryMax),
		foregroundColor: [4]uint8{0, 0, 0, 255},
		backgroundColor: [4]uint8{255, 255, 255, 255},
		pathTool:        newPathToolState(),
		mixerBrush: mixerBrushState{
			clean: true,
		},
	}

	if config.DocumentWidth > 0 && config.DocumentHeight > 0 {
		doc := inst.newDocument(CreateDocumentPayload{
			Name:       "Untitled-1",
			Width:      config.DocumentWidth,
			Height:     config.DocumentHeight,
			Resolution: floatValueOrDefault(config.Resolution, defaultResolutionDPI),
			ColorMode:  "rgb",
			BitDepth:   8,
			Background: stringValueOrDefault(config.Background, "transparent"),
		})
		inst.manager.Create(doc)
		inst.viewport.CenterX = float64(doc.Width) * 0.5
		inst.viewport.CenterY = float64(doc.Height) * 0.5
	}

	instances[id] = inst
	return id
}

// Free releases the engine instance identified by handle.
func Free(handle int32) {
	mu.Lock()
	defer mu.Unlock()
	delete(instances, handle)
}

// FreePointer is a no-op placeholder while the engine keeps ownership of its
// render buffer inside Wasm linear memory.
func FreePointer(_ int32) {}

func DispatchCommand(handle, commandID int32, payloadJSON string) (RenderResult, error) {
	mu.Lock()
	defer mu.Unlock()

	inst, ok := instances[handle]
	if !ok {
		return RenderResult{}, fmt.Errorf("invalid engine handle %d", handle)
	}

	// Hot-path commands (per-rAF pointer input) skip the full render + UIMeta
	// marshal and return a minimal ack instead; every other command bumps the
	// UI-meta version so the frontend knows its cached UIMeta is stale. The
	// bump happens before dispatch so custom render results (thumbnails,
	// histogram, …) already carry the new version.
	hot := isHotCommand(commandID, payloadJSON)
	if !hot {
		inst.uiMetaVersion++
	}

	var suggestedPath []SelectionPoint

	switch cmdpkg.DomainOf(commandID) {
	case cmdpkg.DomainLayer:
		handled, err := inst.dispatchLayerCommand(commandID, payloadJSON)
		if err != nil {
			return RenderResult{}, err
		}
		if !handled {
			return RenderResult{}, fmt.Errorf("unsupported layer command id 0x%04x", commandID)
		}
	case cmdpkg.DomainCore:
		handled, err := inst.dispatchCoreCommand(commandID, payloadJSON)
		if err != nil {
			return RenderResult{}, err
		}
		if !handled {
			return RenderResult{}, fmt.Errorf("unsupported core command id 0x%04x", commandID)
		}
	case cmdpkg.DomainTransform:
		handled, err := inst.dispatchTransformCommand(commandID, payloadJSON)
		if err != nil {
			return RenderResult{}, err
		}
		if !handled {
			return RenderResult{}, fmt.Errorf("unsupported transform command id 0x%04x", commandID)
		}
	case cmdpkg.DomainUI:
		handled, customResult, err := inst.dispatchUICommand(commandID, payloadJSON)
		if err != nil {
			return RenderResult{}, err
		}
		if !handled {
			return RenderResult{}, fmt.Errorf("unsupported ui command id 0x%04x", commandID)
		}
		if customResult != nil {
			return *customResult, nil
		}
	case cmdpkg.DomainSelectionPaint:
		handled, customResult, nextSuggestedPath, err := inst.dispatchSelectionPaintCommand(commandID, payloadJSON, suggestedPath)
		if err != nil {
			return RenderResult{}, err
		}
		if !handled {
			return RenderResult{}, fmt.Errorf("unsupported selection/paint command id 0x%04x", commandID)
		}
		suggestedPath = nextSuggestedPath
		if customResult != nil {
			return *customResult, nil
		}
		// selection/paint handlers generally fall through to the normal render.

	case cmdpkg.DomainFilter:
		handled, err := inst.dispatchFilterCommand(commandID, payloadJSON)
		if err != nil {
			return RenderResult{}, err
		}
		if !handled {
			return RenderResult{}, fmt.Errorf("unsupported filter command id 0x%04x", commandID)
		}

	case cmdpkg.DomainPath:
		handled, err := inst.dispatchPathCommand(commandID, payloadJSON)
		if err != nil {
			return RenderResult{}, err
		}
		if !handled {
			return RenderResult{}, fmt.Errorf("unsupported path command id 0x%04x", commandID)
		}

	case cmdpkg.DomainShape:
		handled, err := inst.dispatchShapeCommand(commandID, payloadJSON)
		if err != nil {
			return RenderResult{}, err
		}
		if !handled {
			return RenderResult{}, fmt.Errorf("unsupported shape command id 0x%04x", commandID)
		}

	case cmdpkg.DomainText:
		handled, err := inst.dispatchTextCommand(commandID, payloadJSON)
		if err != nil {
			return RenderResult{}, err
		}
		if !handled {
			return RenderResult{}, fmt.Errorf("unsupported text command id 0x%04x", commandID)
		}

	case cmdpkg.DomainUnknown:
		return RenderResult{}, fmt.Errorf("unsupported command id 0x%04x", commandID)
	}

	if hot {
		// No pixels are rendered for acks: the frontend's continuous rAF loop
		// picks up the new frame via RenderFrameRaw on the next tick anyway.
		return inst.ackResult(), nil
	}

	result := inst.render()
	result.SuggestedPath = suggestedPath
	return result, nil
}

// isHotCommand reports whether a command is on the per-rAF pointer hot path:
// ContinuePaintStroke, and PointerEvent with phase == "move". Non-move pointer
// phases (down/up) end transactions and push history, so they stay full.
func isHotCommand(commandID int32, payloadJSON string) bool {
	switch commandID {
	case commandContinuePaintStroke:
		return true
	case commandPointerEvent:
		var peek struct {
			Phase string `json:"phase"`
		}
		if err := json.Unmarshal([]byte(payloadJSON), &peek); err != nil {
			return false
		}
		return peek.Phase == "move"
	default:
		return false
	}
}

// ackResult builds the minimal DispatchCommand response for hot-path commands:
// no pixels, no UIMeta — just the viewport (move-phase pans change it), the
// cheap cursor/status strings, and the version counters the frontend needs to
// detect staleness. See the uiMetaVersion field comment for why hot commands
// never bump the version mid-gesture.
func (inst *instance) ackResult() RenderResult {
	statusText := "No active document"
	var contentVersion int64
	if doc := inst.manager.activeMut(); doc != nil {
		statusText = inst.statusText(doc)
		contentVersion = doc.ContentVersion
	}
	return RenderResult{
		FrameID:        inst.frameID,
		Viewport:       inst.viewport,
		CursorType:     inst.cursorType(),
		StatusText:     statusText,
		UIMetaVersion:  inst.uiMetaVersion,
		ContentVersion: contentVersion,
	}
}

func RenderFrame(handle int32) (RenderResult, error) {
	mu.Lock()
	defer mu.Unlock()

	inst, ok := instances[handle]
	if !ok {
		return RenderResult{}, fmt.Errorf("invalid engine handle %d", handle)
	}

	return inst.render(), nil
}

func RenderFrameRaw(handle int32) (RawRenderResult, error) {
	mu.Lock()
	defer mu.Unlock()

	inst, ok := instances[handle]
	if !ok {
		return RawRenderResult{}, fmt.Errorf("invalid engine handle %d", handle)
	}

	return inst.renderRaw(), nil
}

// ExportProject returns the current active document as a JSON project archive.
func ExportProject(handle int32) (string, error) {
	mu.Lock()
	defer mu.Unlock()

	inst, ok := instances[handle]
	if !ok {
		return "", fmt.Errorf("invalid engine handle %d", handle)
	}

	return inst.exportProject()
}

// ExportDocument returns the current active document in the requested format.
func ExportDocument(handle int32, format string) (string, error) {
	mu.Lock()
	defer mu.Unlock()

	inst, ok := instances[handle]
	if !ok {
		return "", fmt.Errorf("invalid engine handle %d", handle)
	}

	return inst.exportDocument(format)
}

// ImportProject loads a JSON project archive into the active engine instance.
func ImportProject(handle int32, payload string) (RenderResult, error) {
	mu.Lock()
	defer mu.Unlock()

	inst, ok := instances[handle]
	if !ok {
		return RenderResult{}, fmt.Errorf("invalid engine handle %d", handle)
	}

	// ImportProject mutates engine state outside DispatchCommand, so it must
	// bump the UI-meta version itself (see instance.uiMetaVersion).
	inst.uiMetaVersion++
	return inst.importProject(payload)
}

// GetBufferPtr returns the pointer to the pixel buffer inside Wasm linear memory.
func GetBufferPtr(handle int32) int32 {
	mu.Lock()
	defer mu.Unlock()

	inst, ok := instances[handle]
	if !ok || len(inst.pixels) == 0 {
		return 0
	}
	return int32(uintptr(unsafe.Pointer(&inst.pixels[0]))) //nolint:govet // intentional Wasm ABI pointer handoff to JS
}

// GetBufferLen returns the byte length of the current pixel buffer.
func GetBufferLen(handle int32) int32 {
	mu.Lock()
	defer mu.Unlock()

	inst, ok := instances[handle]
	if !ok {
		return 0
	}
	return int32(len(inst.pixels))
}
