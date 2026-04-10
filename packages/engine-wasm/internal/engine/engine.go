// Package engine is the core of the Agogo image editor backend.
// Phase 1 adds document, viewport, history, and a JSON command bridge.
package engine

import (
	"encoding/json"
	"fmt"
	"sync"
	"unsafe"

	agglib "github.com/cwbudde/agg_go"
)

// thumbnailSize is the width and height of layer preview thumbnails in pixels.
const thumbnailSize = 32

const (
	commandCreateDocument           = 0x0001
	commandCloseDocument            = 0x0002
	commandZoomSet                  = 0x0010
	commandPanSet                   = 0x0011
	commandRotateViewSet            = 0x0012
	commandResize                   = 0x0013
	commandFitToView                = 0x0014
	commandPointerEvent             = 0x0015
	commandJumpHistory              = 0x0016
	commandSetShowGuides            = 0x0017
	commandAddLayer                 = 0x0100
	commandDeleteLayer              = 0x0101
	commandMoveLayer                = 0x0102
	commandSetLayerVis              = 0x0103
	commandSetLayerOp               = 0x0104
	commandSetLayerBlend            = 0x0105
	commandDuplicateLayer           = 0x0106
	commandSetLayerLock             = 0x0107
	commandFlattenLayer             = 0x0108
	commandMergeDown                = 0x0109
	commandMergeVisible             = 0x010a
	commandAddLayerMask             = 0x010b
	commandDeleteLayerMask          = 0x010c
	commandApplyLayerMask           = 0x010d
	commandInvertLayerMask          = 0x010e
	commandSetMaskEnabled           = 0x010f
	commandSetLayerClip             = 0x0110
	commandSetActiveLayer           = 0x0111
	commandSetLayerName             = 0x0112
	commandAddVectorMask            = 0x0113
	commandDeleteVectorMask         = 0x0114
	commandSetMaskEditMode          = 0x0115
	commandGetLayerThumbnails       = 0x0116
	commandFlattenImage             = 0x0117
	commandOpenImageFile            = 0x0118
	commandTranslateLayer           = 0x0119
	commandPickLayerAtPoint         = 0x011a
	commandSetAdjustmentParams      = 0x011b
	commandNewSelection             = 0x0200
	commandSelectAll                = 0x0201
	commandDeselect                 = 0x0202
	commandReselect                 = 0x0203
	commandInvertSelection          = 0x0204
	commandFeatherSelection         = 0x0205
	commandExpandSelection          = 0x0206
	commandContractSelection        = 0x0207
	commandSmoothSelection          = 0x0208
	commandBorderSelection          = 0x0209
	commandTransformSelection       = 0x020a
	commandSelectColorRange         = 0x020b
	commandQuickSelect              = 0x020c
	commandMagicWand                = 0x020d
	commandMagneticLassoSuggestPath = 0x020e
	commandBeginFreeTransform       = 0x0300
	commandUpdateFreeTransform      = 0x0301
	commandCommitFreeTransform      = 0x0302
	commandCancelFreeTransform      = 0x0303
	commandFlipLayerH               = 0x0304
	commandFlipLayerV               = 0x0305
	commandRotateLayer90CW          = 0x0306
	commandRotateLayer90CCW         = 0x0307
	commandRotateLayer180           = 0x0308
	commandTransformAgain           = 0x0309
	commandBeginCrop                = 0x0320
	commandUpdateCrop               = 0x0321
	commandCommitCrop               = 0x0322
	commandCancelCrop               = 0x0323
	commandResizeCanvas             = 0x0324
	commandBeginPaintStroke         = 0x0400
	commandContinuePaintStroke      = 0x0401
	commandEndPaintStroke           = 0x0402
	commandSetForegroundColor       = 0x0410
	commandSetBackgroundColor       = 0x0411
	commandSampleMergedColor        = 0x0412
	commandMagicErase               = 0x0413
	commandFill                     = 0x0414
	commandApplyGradient            = 0x0415
	commandComputeHistogram         = 0x011c
	commandSetPointFromSample       = 0x011d
	commandIdentifyHueRange         = 0x011e
	commandSetLayerStyleStack       = 0x011f
	commandSetLayerStyleEnabled     = 0x0120
	commandSetLayerStyleParams      = 0x0121
	commandCopyLayerStyle           = 0x0122
	commandPasteLayerStyle          = 0x0123
	commandClearLayerStyle          = 0x0124
	commandApplyFilter              = 0x0500
	commandReapplyFilter            = 0x0501
	commandPreviewFilter            = 0x0502
	commandCancelFilterPreview      = 0x0503
	commandCommitFilterPreview      = 0x0504
	commandFadeFilter               = 0x0505

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

	commandBeginTxn     = 0xffe0
	commandEndTxn       = 0xffe1
	commandClearHistory = 0xffe2
	commandUndo         = 0xfff0
	commandRedo         = 0xfff1
)

const (
	defaultDocWidth       = 1920
	defaultDocHeight      = 1080
	defaultResolutionDPI  = 72
	defaultHistoryMax     = 50
	defaultDevicePixelRat = 1.0
)

type Background struct {
	Kind  string   `json:"kind"`
	Color [4]uint8 `json:"color,omitempty"`
}

type Document struct {
	Width          int         `json:"width"`
	Height         int         `json:"height"`
	Resolution     float64     `json:"resolution"`
	ColorMode      string      `json:"colorMode"`
	BitDepth       int         `json:"bitDepth"`
	Background     Background  `json:"background"`
	ID             string      `json:"id"`
	Name           string      `json:"name"`
	CreatedAt      string      `json:"createdAt"`
	CreatedBy      string      `json:"createdBy"`
	ModifiedAt     string      `json:"modifiedAt"`
	ActiveLayerID  string      `json:"activeLayerId,omitempty"`
	LayerRoot      *GroupLayer `json:"-"`
	Selection      *Selection  `json:"-"`
	LastSelection  *Selection  `json:"-"`
	ContentVersion int64       `json:"-"` // monotonic counter; not persisted, used only for composite cache invalidation
	Paths          []NamedPath `json:"-"`
	ActivePathIdx  int         `json:"-"`
}

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

type DirtyRect struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

type HistoryEntry struct {
	ID          int64  `json:"id"`
	Description string `json:"description"`
	State       string `json:"state"`
}

// ThumbnailEntry holds base64-encoded RGBA pixel buffers for a layer preview.
// LayerRGBA is always present (when the layer has rasterizable content).
// MaskRGBA is only present when the layer has a pixel mask.
type ThumbnailEntry struct {
	LayerRGBA string `json:"layerRGBA"`
	MaskRGBA  string `json:"maskRGBA,omitempty"`
}

type UIMeta struct {
	ActiveLayerID       string          `json:"activeLayerId"`
	ActiveLayerName     string          `json:"activeLayerName"`
	CursorType          string          `json:"cursorType"`
	StatusText          string          `json:"statusText"`
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
	MaskEditLayerID string             `json:"maskEditLayerId,omitempty"`
	Selection       SelectionMeta      `json:"selection"`
	FreeTransform   *FreeTransformMeta `json:"freeTransform,omitempty"`
	Crop            *CropMeta          `json:"crop,omitempty"`
	Paths           []PathMeta         `json:"paths,omitempty"`
	PathOverlay     *PathOverlay       `json:"pathOverlay,omitempty"`
	// EditingVectorLayerID is non-empty while a VectorLayer's path is being
	// edited. The UI uses this to show the "editing path" indicator.
	EditingVectorLayerID string `json:"editingVectorLayerId,omitempty"`
	// EditingTextLayerID is non-empty while a TextLayer is in text edit mode.
	EditingTextLayerID string `json:"editingTextLayerId,omitempty"`
	// TextCursorX/Y are doc-space coordinates of the text insertion cursor.
	// Only meaningful when EditingTextLayerID is set.
	TextCursorX float64 `json:"textCursorX,omitempty"`
	TextCursorY float64 `json:"textCursorY,omitempty"`
}

type RenderResult struct {
	FrameID     int64         `json:"frameId"`
	Viewport    ViewportState `json:"viewport"`
	DirtyRects  []DirtyRect   `json:"dirtyRects"`
	PixelFormat string        `json:"pixelFormat"`
	BufferPtr   int32         `json:"bufferPtr"`
	BufferLen   int32         `json:"bufferLen"`
	UIMeta      UIMeta        `json:"uiMeta"`
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
}

type RawRenderResult struct {
	FrameID   int64         `json:"frameId"`
	Viewport  ViewportState `json:"viewport"`
	BufferPtr int32         `json:"bufferPtr"`
	BufferLen int32         `json:"bufferLen"`
	Reused    bool          `json:"reused"`
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
	layerID          string
	params           BrushParams
	strokeState      brushStrokeState
	stabilizer       stabilizerState
	dirtyMin         [2]int // min corner of painted dirty rect (layer-local)
	dirtyMax         [2]int // max corner of painted dirty rect (layer-local)
	hasDirty         bool
	bgEraseBaseColor [4]uint8 // sampled once at stroke begin for background eraser
	mixerSource      []byte   // sampled once at stroke begin for mixer brush
	mixerSourceW     int
	mixerSourceH     int
	mixerSourceX     int
	mixerSourceY     int
	cloneSource      []byte // sampled once at stroke begin for clone stamp
	cloneSourceW     int
	cloneSourceH     int
	cloneSourceX     int
	cloneSourceY     int
	cloneOffsetX     float64
	cloneOffsetY     float64
	historySource    []byte // sampled once at stroke begin for history brush
	historySourceW   int
	historySourceH   int
	historySourceX   int
	historySourceY   int
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

type snapshot struct {
	DocumentID string
	Document   *Document
	Viewport   ViewportState
}

type Command interface {
	Apply(*instance) error
	Undo(*instance) error
	Description() string
}

type snapshotCommand struct {
	description string
	before      snapshot
	after       snapshot
	applyFn     func(*instance) (snapshot, error)
}

func (c *snapshotCommand) Apply(inst *instance) error {
	if c.applyFn != nil {
		before := inst.captureSnapshot()
		after, err := c.applyFn(inst)
		if err != nil {
			return err
		}
		c.before = before
		c.after = after
		c.applyFn = nil
		return nil
	}
	return inst.restoreSnapshot(c.after)
}

func (c *snapshotCommand) Undo(inst *instance) error {
	return inst.restoreSnapshot(c.before)
}

func (c *snapshotCommand) Description() string {
	return c.description
}

type HistoryStack struct {
	undo     []Command
	redo     []Command
	maxDepth int
	active   *groupedCommand
}

type groupedCommand struct {
	description string
	before      snapshot
	after       snapshot
}

func (c *groupedCommand) Apply(inst *instance) error {
	return inst.restoreSnapshot(c.after)
}

func (c *groupedCommand) Undo(inst *instance) error {
	return inst.restoreSnapshot(c.before)
}

func (c *groupedCommand) Description() string {
	return c.description
}

func newHistoryStack(maxDepth int) *HistoryStack {
	return &HistoryStack{maxDepth: maxDepth}
}

func (h *HistoryStack) Execute(inst *instance, command Command) error {
	if err := command.Apply(inst); err != nil {
		return err
	}
	if h.active != nil {
		h.active.after = inst.captureSnapshot()
		return nil
	}
	h.push(command)
	return nil
}

func (h *HistoryStack) BeginTransaction(inst *instance, description string) {
	if h.active != nil {
		return
	}
	state := inst.captureSnapshot()
	h.active = &groupedCommand{
		description: description,
		before:      state,
		after:       state,
	}
}

func (h *HistoryStack) EndTransaction(commit bool) {
	if h.active == nil {
		return
	}
	active := h.active
	h.active = nil
	if !commit || snapshotsEqual(active.before, active.after) {
		return
	}
	h.push(active)
}

func (h *HistoryStack) push(command Command) {
	h.undo = append(h.undo, command)
	if len(h.undo) > h.maxDepth {
		h.undo = h.undo[len(h.undo)-h.maxDepth:]
	}
	h.redo = h.redo[:0]
}

func (h *HistoryStack) Undo(inst *instance) error {
	if len(h.undo) == 0 {
		return nil
	}
	command := h.undo[len(h.undo)-1]
	h.undo = h.undo[:len(h.undo)-1]
	if err := command.Undo(inst); err != nil {
		return err
	}
	h.redo = append(h.redo, command)
	return nil
}

func (h *HistoryStack) Redo(inst *instance) error {
	if len(h.redo) == 0 {
		return nil
	}
	command := h.redo[len(h.redo)-1]
	h.redo = h.redo[:len(h.redo)-1]
	if err := command.Apply(inst); err != nil {
		return err
	}
	h.undo = append(h.undo, command)
	return nil
}

func (h *HistoryStack) Entries() []HistoryEntry {
	entries := make([]HistoryEntry, 0, len(h.undo)+len(h.redo))
	for i, command := range h.undo {
		state := "done"
		if i == len(h.undo)-1 {
			state = "current"
		}
		entries = append(entries, HistoryEntry{
			ID:          int64(i + 1),
			Description: command.Description(),
			State:       state,
		})
	}
	for i := len(h.redo) - 1; i >= 0; i-- {
		command := h.redo[i]
		entries = append(entries, HistoryEntry{
			ID:          int64(len(entries) + 1),
			Description: command.Description(),
			State:       "undone",
		})
	}
	return entries
}

func (h *HistoryStack) CurrentIndex() int {
	return len(h.undo)
}

func (h *HistoryStack) SnapshotAt(historyIndex int) (snapshot, bool) {
	if historyIndex < 0 || historyIndex > len(h.undo) {
		return snapshot{}, false
	}
	if historyIndex == 0 {
		return snapshot{}, false
	}
	command := h.undo[historyIndex-1]
	switch typed := command.(type) {
	case *snapshotCommand:
		return typed.after, true
	case *groupedCommand:
		return typed.after, true
	default:
		return snapshot{}, false
	}
}

func (h *HistoryStack) PreviousSnapshot(inst *instance) (snapshot, bool) {
	if len(h.undo) == 0 || inst == nil {
		return snapshot{}, false
	}
	active := inst.manager.Active()
	if active == nil {
		return snapshot{}, false
	}

	cloneInst := &instance{
		manager:  newDocumentManager(),
		viewport: inst.viewport,
		history:  newHistoryStack(defaultHistoryMax),
	}
	cloneInst.manager.Create(active)
	if inst.manager.ActiveID() != "" {
		cloneInst.manager.activeID = inst.manager.ActiveID()
	}
	if err := h.undo[len(h.undo)-1].Undo(cloneInst); err != nil {
		return snapshot{}, false
	}
	return cloneInst.captureSnapshot(), true
}

func (h *HistoryStack) CanUndo() bool { return len(h.undo) > 0 }
func (h *HistoryStack) CanRedo() bool { return len(h.redo) > 0 }

func (h *HistoryStack) Clear() {
	h.undo = nil
	h.redo = nil
	h.active = nil
}

func (h *HistoryStack) JumpTo(inst *instance, historyIndex int) error {
	total := len(h.undo) + len(h.redo)
	if historyIndex < 0 {
		historyIndex = 0
	}
	if historyIndex > total {
		historyIndex = total
	}

	for len(h.undo) > historyIndex {
		if err := h.Undo(inst); err != nil {
			return err
		}
	}
	for len(h.undo) < historyIndex {
		if err := h.Redo(inst); err != nil {
			return err
		}
	}
	return nil
}

type DocumentManager struct {
	docs     map[string]*Document
	order    []string
	activeID string
}

func newDocumentManager() *DocumentManager {
	return &DocumentManager{docs: make(map[string]*Document)}
}

func (m *DocumentManager) Create(doc *Document) {
	m.docs[doc.ID] = cloneDocument(doc)
	m.order = append(m.order, doc.ID)
	m.activeID = doc.ID
}

func (m *DocumentManager) ReplaceActive(doc *Document) error {
	if doc == nil {
		return fmt.Errorf("document is required")
	}
	if m.activeID == "" {
		m.Create(doc)
		return nil
	}
	m.docs[m.activeID] = cloneDocument(doc)
	return nil
}

func (m *DocumentManager) Active() *Document {
	if m.activeID == "" {
		return nil
	}
	doc := m.docs[m.activeID]
	return cloneDocument(doc)
}

// activeMut returns the stored document directly without cloning.
// Callers may modify the returned document in place; it is the caller's
// responsibility to ensure the mutation is intentional (e.g. direct pixel
// painting during a brush stroke).  Most code should use Active() instead.
func (m *DocumentManager) activeMut() *Document {
	if m.activeID == "" {
		return nil
	}
	return m.docs[m.activeID]
}

func (m *DocumentManager) ActiveID() string {
	return m.activeID
}

func (m *DocumentManager) Switch(id string) error {
	if _, ok := m.docs[id]; !ok {
		return fmt.Errorf("document %q not found", id)
	}
	m.activeID = id
	return nil
}

func (m *DocumentManager) CloseActive() error {
	if m.activeID == "" {
		return nil
	}
	delete(m.docs, m.activeID)
	nextOrder := make([]string, 0, len(m.order))
	for _, id := range m.order {
		if id != m.activeID {
			nextOrder = append(nextOrder, id)
		}
	}
	m.order = nextOrder
	if len(m.order) == 0 {
		m.activeID = ""
		return nil
	}
	m.activeID = m.order[len(m.order)-1]
	return nil
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
	pixels                  []byte
	manager                 *DocumentManager
	viewport                ViewportState
	cachedViewportBase      []byte
	cachedViewportBaseKey   viewportBaseKey
	cachedRawFrameKey       rawFrameKey
	hasCachedRawFrame       bool
	history                 *HistoryStack
	frameID                 int64
	pointer                 pointerDragState
	cachedDocSurface        []byte
	cachedDocID             string
	cachedDocContentVersion int64
	// maskEditLayerID tracks which layer's mask is currently being edited.
	// This is UI state only — not included in history snapshots.
	maskEditLayerID string
	// freeTransform holds the live state while free transform is active.
	// It is UI-only state not included in history snapshots.
	freeTransform *FreeTransformState
	// lastTransform records the most recently committed transform (free or
	// discrete) so that Transform Again can replay it on any layer.
	lastTransform *LastTransformRecord
	// crop holds the live state while the crop tool is active.
	crop *CropState
	// foregroundColor is the active foreground (paint) color.
	foregroundColor [4]uint8 // RGBA
	// backgroundColor is the active background color.
	backgroundColor [4]uint8 // RGBA
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

	var suggestedPath []SelectionPoint

	switch commandID {
	case commandAddLayer, commandDeleteLayer, commandMoveLayer, commandSetLayerVis,
		commandSetLayerOp, commandSetLayerBlend, commandDuplicateLayer, commandSetLayerLock,
		commandFlattenLayer, commandMergeDown, commandMergeVisible, commandAddLayerMask,
		commandDeleteLayerMask, commandApplyLayerMask, commandInvertLayerMask,
		commandSetMaskEnabled, commandSetLayerClip, commandSetLayerName, commandSetActiveLayer,
		commandSetAdjustmentParams, commandAddVectorMask, commandDeleteVectorMask,
		commandSetPointFromSample, commandSetLayerStyleStack, commandSetLayerStyleEnabled,
		commandSetLayerStyleParams, commandCopyLayerStyle, commandPasteLayerStyle,
		commandClearLayerStyle:
		handled, err := inst.dispatchLayerCommand(commandID, payloadJSON)
		if err != nil {
			return RenderResult{}, err
		}
		if !handled {
			return RenderResult{}, fmt.Errorf("unsupported layer command id 0x%04x", commandID)
		}
	case commandCreateDocument, commandCloseDocument, commandZoomSet, commandPanSet,
		commandRotateViewSet, commandResize, commandPointerEvent, commandBeginTxn,
		commandEndTxn, commandJumpHistory, commandSetShowGuides, commandClearHistory,
		commandFitToView, commandUndo, commandRedo, commandFlattenImage, commandOpenImageFile,
		commandTranslateLayer:
		if handled, err := inst.dispatchCoreCommand(commandID, payloadJSON); handled || err != nil {
			if err != nil {
				return RenderResult{}, err
			}
		}
	case commandBeginFreeTransform, commandUpdateFreeTransform, commandCommitFreeTransform,
		commandCancelFreeTransform, commandFlipLayerH, commandFlipLayerV, commandRotateLayer90CW,
		commandRotateLayer90CCW, commandRotateLayer180, commandTransformAgain,
		commandBeginCrop, commandUpdateCrop, commandCommitCrop, commandCancelCrop, commandResizeCanvas:
		if handled, err := inst.dispatchTransformCommand(commandID, payloadJSON); handled || err != nil {
			if err != nil {
				return RenderResult{}, err
			}
		}
	case commandSetMaskEditMode, commandGetLayerThumbnails, commandComputeHistogram, commandIdentifyHueRange:
		handled, customResult, err := inst.dispatchUICommand(commandID, payloadJSON)
		if err != nil {
			return RenderResult{}, err
		}
		if handled && customResult != nil {
			return *customResult, nil
		}
	case commandPickLayerAtPoint, commandNewSelection, commandSelectAll, commandDeselect,
		commandReselect, commandInvertSelection, commandFeatherSelection, commandExpandSelection,
		commandContractSelection, commandSmoothSelection, commandBorderSelection,
		commandTransformSelection, commandSelectColorRange, commandQuickSelect,
		commandMagicWand, commandMagneticLassoSuggestPath, commandBeginPaintStroke,
		commandContinuePaintStroke, commandEndPaintStroke, commandSetForegroundColor,
		commandSetBackgroundColor, commandSampleMergedColor, commandMagicErase,
		commandFill, commandApplyGradient:
		handled, customResult, nextSuggestedPath, err := inst.dispatchSelectionPaintCommand(commandID, payloadJSON, suggestedPath)
		if err != nil {
			return RenderResult{}, err
		}
		if handled {
			suggestedPath = nextSuggestedPath
			if customResult != nil {
				return *customResult, nil
			}
			// selection/paint handlers generally fall through to the normal render.
		}

	case commandApplyFilter, commandReapplyFilter, commandPreviewFilter,
		commandCancelFilterPreview, commandCommitFilterPreview, commandFadeFilter:
		if handled, err := inst.dispatchFilterCommand(commandID, payloadJSON); handled || err != nil {
			if err != nil {
				return RenderResult{}, err
			}
		}

	case commandSetActiveTool, commandPenToolClick, commandPenToolClose,
		commandDirectSelectMove, commandDirectSelectMarquee, commandBreakHandle,
		commandDeleteAnchor, commandAddAnchorOnSegment,
		commandPathCombine, commandPathSubtract, commandPathIntersect, commandPathExclude,
		commandFlattenPath, commandRasterizePath,
		commandCreatePath, commandDeletePath, commandRenamePath, commandDuplicatePath,
		commandMakeSelectionFromPath, commandStrokePath, commandFillPath:
		if handled, err := inst.dispatchPathCommand(commandID, payloadJSON); handled || err != nil {
			if err != nil {
				return RenderResult{}, err
			}
		}

	case commandDrawShape, commandEnterVectorEditMode, commandCommitVectorEdit, commandSetVectorLayerStyle:
		if handled, err := inst.dispatchShapeCommand(commandID, payloadJSON); handled || err != nil {
			if err != nil {
				return RenderResult{}, err
			}
		}

	case commandAddTextLayer, commandSetTextContent, commandSetTextStyle,
		commandEnterTextEditMode, commandTextEditInput, commandCommitTextEdit,
		commandConvertTextToPath:
		if handled, err := inst.dispatchTextCommand(commandID, payloadJSON); handled || err != nil {
			if err != nil {
				return RenderResult{}, err
			}
		}

	default:
		return RenderResult{}, fmt.Errorf("unsupported command id 0x%04x", commandID)
	}

	result := inst.render()
	result.SuggestedPath = suggestedPath
	return result, nil
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

// ImportProject loads a JSON project archive into the active engine instance.
func ImportProject(handle int32, payload string) (RenderResult, error) {
	mu.Lock()
	defer mu.Unlock()

	inst, ok := instances[handle]
	if !ok {
		return RenderResult{}, fmt.Errorf("invalid engine handle %d", handle)
	}

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
	return int32(uintptr(unsafe.Pointer(&inst.pixels[0]))) //nolint:unsafeptr
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
