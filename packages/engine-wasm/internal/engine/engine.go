// Package engine is the core of the Agogo image editor backend.
// Phase 1 adds document, viewport, history, and a JSON command bridge.
package engine

import (
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	agglib "github.com/cwbudde/agg_go"
	aggrender "github.com/cwbudde/agogo-web/packages/engine-wasm/internal/agg"
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
	commandBeginTxn                 = 0xffe0
	commandEndTxn                   = 0xffe1
	commandClearHistory             = 0xffe2
	commandUndo                     = 0xfff0
	commandRedo                     = 0xfff1
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

type instance struct {
	pixels                  []byte
	manager                 *DocumentManager
	viewport                ViewportState
	cachedViewportBase      []byte
	cachedViewportBaseKey   viewportBaseKey
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
}

// compositeSurface returns the precomputed document composite for doc, reusing
// the cached surface when the document content has not changed since the last
// render. This avoids re-running the full compositing pipeline on every frame
// when only the viewport transform changes (pan, zoom, rotate).
func (inst *instance) compositeSurface(doc *Document) []byte {
	if doc == nil {
		inst.cachedDocSurface = nil
		inst.cachedDocID = ""
		inst.cachedDocContentVersion = 0
		return nil
	}
	if inst.cachedDocID == doc.ID && inst.cachedDocContentVersion == doc.ContentVersion && len(inst.cachedDocSurface) > 0 {
		return inst.cachedDocSurface
	}
	inst.cachedDocSurface = doc.renderCompositeSurface()
	inst.cachedDocID = doc.ID
	inst.cachedDocContentVersion = doc.ContentVersion
	return inst.cachedDocSurface
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
	case commandCreateDocument:
		var payload CreateDocumentPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return RenderResult{}, err
		}
		command := &snapshotCommand{
			description: fmt.Sprintf("New document: %s", defaultDocumentName(payload.Name)),
			applyFn: func(inst *instance) (snapshot, error) {
				doc := inst.newDocument(payload)
				inst.manager.Create(doc)
				inst.viewport.CenterX = float64(doc.Width) * 0.5
				inst.viewport.CenterY = float64(doc.Height) * 0.5
				inst.fitViewportToActiveDocument()
				return inst.captureSnapshot(), nil
			},
		}
		if err := inst.history.Execute(inst, command); err != nil {
			return RenderResult{}, err
		}
	case commandCloseDocument:
		command := &snapshotCommand{
			description: "Close document",
			applyFn: func(inst *instance) (snapshot, error) {
				if err := inst.manager.CloseActive(); err != nil {
					return snapshot{}, err
				}
				if doc := inst.manager.Active(); doc != nil {
					inst.viewport.CenterX = float64(doc.Width) * 0.5
					inst.viewport.CenterY = float64(doc.Height) * 0.5
					inst.fitViewportToActiveDocument()
				}
				return inst.captureSnapshot(), nil
			},
		}
		if err := inst.history.Execute(inst, command); err != nil {
			return RenderResult{}, err
		}
	case commandZoomSet:
		var payload ZoomPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return RenderResult{}, err
		}
		command := &snapshotCommand{
			description: fmt.Sprintf("Zoom to %.0f%%", payload.Zoom*100),
			applyFn: func(inst *instance) (snapshot, error) {
				nextZoom := clampZoom(payload.Zoom)
				if payload.HasAnchor {
					inst.viewport.CenterX = payload.AnchorX - (payload.AnchorX-inst.viewport.CenterX)*(inst.viewport.Zoom/nextZoom)
					inst.viewport.CenterY = payload.AnchorY - (payload.AnchorY-inst.viewport.CenterY)*(inst.viewport.Zoom/nextZoom)
				}
				inst.viewport.Zoom = nextZoom
				return inst.captureSnapshot(), nil
			},
		}
		if err := inst.history.Execute(inst, command); err != nil {
			return RenderResult{}, err
		}
	case commandPanSet:
		var payload PanPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return RenderResult{}, err
		}
		command := &snapshotCommand{
			description: "Pan viewport",
			applyFn: func(inst *instance) (snapshot, error) {
				inst.viewport.CenterX = payload.CenterX
				inst.viewport.CenterY = payload.CenterY
				return inst.captureSnapshot(), nil
			},
		}
		if err := inst.history.Execute(inst, command); err != nil {
			return RenderResult{}, err
		}
	case commandRotateViewSet:
		var payload RotatePayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return RenderResult{}, err
		}
		command := &snapshotCommand{
			description: fmt.Sprintf("Rotate view to %.0f°", payload.Rotation),
			applyFn: func(inst *instance) (snapshot, error) {
				inst.viewport.Rotation = normalizeRotation(payload.Rotation)
				return inst.captureSnapshot(), nil
			},
		}
		if err := inst.history.Execute(inst, command); err != nil {
			return RenderResult{}, err
		}
	case commandAddLayer:
		var payload AddLayerPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return RenderResult{}, err
		}
		command := &snapshotCommand{
			description: fmt.Sprintf("Add %s layer", payload.LayerType),
			applyFn: func(inst *instance) (snapshot, error) {
				doc := inst.manager.Active()
				if doc == nil {
					return snapshot{}, fmt.Errorf("no active document")
				}
				layer, err := doc.newLayerFromPayload(payload)
				if err != nil {
					return snapshot{}, err
				}
				index := -1
				if payload.Index != nil {
					index = *payload.Index
				}
				if err := doc.AddLayer(layer, payload.ParentLayerID, index); err != nil {
					return snapshot{}, err
				}
				if err := inst.manager.ReplaceActive(doc); err != nil {
					return snapshot{}, err
				}
				return inst.captureSnapshot(), nil
			},
		}
		if err := inst.history.Execute(inst, command); err != nil {
			return RenderResult{}, err
		}
	case commandDeleteLayer:
		var payload DeleteLayerPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return RenderResult{}, err
		}
		command := &snapshotCommand{
			description: "Delete layer",
			applyFn: func(inst *instance) (snapshot, error) {
				doc := inst.manager.Active()
				if doc == nil {
					return snapshot{}, fmt.Errorf("no active document")
				}
				if err := doc.DeleteLayer(payload.LayerID); err != nil {
					return snapshot{}, err
				}
				if err := inst.manager.ReplaceActive(doc); err != nil {
					return snapshot{}, err
				}
				return inst.captureSnapshot(), nil
			},
		}
		if err := inst.history.Execute(inst, command); err != nil {
			return RenderResult{}, err
		}
	case commandMoveLayer:
		var payload MoveLayerPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return RenderResult{}, err
		}
		command := &snapshotCommand{
			description: "Move layer",
			applyFn: func(inst *instance) (snapshot, error) {
				doc := inst.manager.Active()
				if doc == nil {
					return snapshot{}, fmt.Errorf("no active document")
				}
				index := -1
				if payload.Index != nil {
					index = *payload.Index
				}
				if err := doc.MoveLayer(payload.LayerID, payload.ParentLayerID, index); err != nil {
					return snapshot{}, err
				}
				if err := inst.manager.ReplaceActive(doc); err != nil {
					return snapshot{}, err
				}
				return inst.captureSnapshot(), nil
			},
		}
		if err := inst.history.Execute(inst, command); err != nil {
			return RenderResult{}, err
		}
	case commandSetLayerVis:
		var payload SetLayerVisibilityPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return RenderResult{}, err
		}
		command := &snapshotCommand{
			description: "Toggle layer visibility",
			applyFn: func(inst *instance) (snapshot, error) {
				doc := inst.manager.Active()
				if doc == nil {
					return snapshot{}, fmt.Errorf("no active document")
				}
				if err := doc.SetLayerVisibility(payload.LayerID, payload.Visible); err != nil {
					return snapshot{}, err
				}
				if err := inst.manager.ReplaceActive(doc); err != nil {
					return snapshot{}, err
				}
				return inst.captureSnapshot(), nil
			},
		}
		if err := inst.history.Execute(inst, command); err != nil {
			return RenderResult{}, err
		}
	case commandSetLayerOp:
		var payload SetLayerOpacityPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return RenderResult{}, err
		}
		command := &snapshotCommand{
			description: "Set layer opacity",
			applyFn: func(inst *instance) (snapshot, error) {
				doc := inst.manager.Active()
				if doc == nil {
					return snapshot{}, fmt.Errorf("no active document")
				}
				if err := doc.SetLayerOpacity(payload.LayerID, payload.Opacity, payload.FillOpacity); err != nil {
					return snapshot{}, err
				}
				if err := inst.manager.ReplaceActive(doc); err != nil {
					return snapshot{}, err
				}
				return inst.captureSnapshot(), nil
			},
		}
		if err := inst.history.Execute(inst, command); err != nil {
			return RenderResult{}, err
		}
	case commandSetLayerBlend:
		var payload SetLayerBlendModePayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return RenderResult{}, err
		}
		command := &snapshotCommand{
			description: "Set layer blend mode",
			applyFn: func(inst *instance) (snapshot, error) {
				doc := inst.manager.Active()
				if doc == nil {
					return snapshot{}, fmt.Errorf("no active document")
				}
				if err := doc.SetLayerBlendMode(payload.LayerID, payload.BlendMode); err != nil {
					return snapshot{}, err
				}
				if err := inst.manager.ReplaceActive(doc); err != nil {
					return snapshot{}, err
				}
				return inst.captureSnapshot(), nil
			},
		}
		if err := inst.history.Execute(inst, command); err != nil {
			return RenderResult{}, err
		}
	case commandDuplicateLayer:
		var payload DuplicateLayerPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return RenderResult{}, err
		}
		command := &snapshotCommand{
			description: "Duplicate layer",
			applyFn: func(inst *instance) (snapshot, error) {
				doc := inst.manager.Active()
				if doc == nil {
					return snapshot{}, fmt.Errorf("no active document")
				}
				index := -1
				if payload.Index != nil {
					index = *payload.Index
				}
				if _, err := doc.DuplicateLayer(payload.LayerID, payload.ParentLayerID, index); err != nil {
					return snapshot{}, err
				}
				if err := inst.manager.ReplaceActive(doc); err != nil {
					return snapshot{}, err
				}
				return inst.captureSnapshot(), nil
			},
		}
		if err := inst.history.Execute(inst, command); err != nil {
			return RenderResult{}, err
		}
	case commandSetLayerLock:
		var payload SetLayerLockPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return RenderResult{}, err
		}
		command := &snapshotCommand{
			description: "Set layer lock",
			applyFn: func(inst *instance) (snapshot, error) {
				doc := inst.manager.Active()
				if doc == nil {
					return snapshot{}, fmt.Errorf("no active document")
				}
				if err := doc.SetLayerLock(payload.LayerID, payload.LockMode); err != nil {
					return snapshot{}, err
				}
				if err := inst.manager.ReplaceActive(doc); err != nil {
					return snapshot{}, err
				}
				return inst.captureSnapshot(), nil
			},
		}
		if err := inst.history.Execute(inst, command); err != nil {
			return RenderResult{}, err
		}
	case commandFlattenLayer:
		var payload FlattenLayerPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return RenderResult{}, err
		}
		command := &snapshotCommand{
			description: "Flatten layer",
			applyFn: func(inst *instance) (snapshot, error) {
				doc := inst.manager.Active()
				if doc == nil {
					return snapshot{}, fmt.Errorf("no active document")
				}
				if err := doc.FlattenLayer(payload.LayerID); err != nil {
					return snapshot{}, err
				}
				if err := inst.manager.ReplaceActive(doc); err != nil {
					return snapshot{}, err
				}
				return inst.captureSnapshot(), nil
			},
		}
		if err := inst.history.Execute(inst, command); err != nil {
			return RenderResult{}, err
		}
	case commandMergeDown:
		var payload MergeDownPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return RenderResult{}, err
		}
		command := &snapshotCommand{
			description: "Merge down",
			applyFn: func(inst *instance) (snapshot, error) {
				doc := inst.manager.Active()
				if doc == nil {
					return snapshot{}, fmt.Errorf("no active document")
				}
				if err := doc.MergeDown(payload.LayerID); err != nil {
					return snapshot{}, err
				}
				if err := inst.manager.ReplaceActive(doc); err != nil {
					return snapshot{}, err
				}
				return inst.captureSnapshot(), nil
			},
		}
		if err := inst.history.Execute(inst, command); err != nil {
			return RenderResult{}, err
		}
	case commandMergeVisible:
		command := &snapshotCommand{
			description: "Merge visible layers",
			applyFn: func(inst *instance) (snapshot, error) {
				doc := inst.manager.Active()
				if doc == nil {
					return snapshot{}, fmt.Errorf("no active document")
				}
				if err := doc.MergeVisible(); err != nil {
					return snapshot{}, err
				}
				if err := inst.manager.ReplaceActive(doc); err != nil {
					return snapshot{}, err
				}
				return inst.captureSnapshot(), nil
			},
		}
		if err := inst.history.Execute(inst, command); err != nil {
			return RenderResult{}, err
		}
	case commandAddLayerMask:
		var payload AddLayerMaskPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return RenderResult{}, err
		}
		command := &snapshotCommand{
			description: "Add layer mask",
			applyFn: func(inst *instance) (snapshot, error) {
				doc := inst.manager.Active()
				if doc == nil {
					return snapshot{}, fmt.Errorf("no active document")
				}
				if err := doc.AddLayerMask(payload.LayerID, payload.Mode); err != nil {
					return snapshot{}, err
				}
				if err := inst.manager.ReplaceActive(doc); err != nil {
					return snapshot{}, err
				}
				return inst.captureSnapshot(), nil
			},
		}
		if err := inst.history.Execute(inst, command); err != nil {
			return RenderResult{}, err
		}
	case commandDeleteLayerMask:
		var payload DeleteLayerMaskPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return RenderResult{}, err
		}
		command := &snapshotCommand{
			description: "Delete layer mask",
			applyFn: func(inst *instance) (snapshot, error) {
				doc := inst.manager.Active()
				if doc == nil {
					return snapshot{}, fmt.Errorf("no active document")
				}
				if err := doc.DeleteLayerMask(payload.LayerID); err != nil {
					return snapshot{}, err
				}
				if err := inst.manager.ReplaceActive(doc); err != nil {
					return snapshot{}, err
				}
				return inst.captureSnapshot(), nil
			},
		}
		if err := inst.history.Execute(inst, command); err != nil {
			return RenderResult{}, err
		}
	case commandApplyLayerMask:
		var payload ApplyLayerMaskPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return RenderResult{}, err
		}
		command := &snapshotCommand{
			description: "Apply layer mask",
			applyFn: func(inst *instance) (snapshot, error) {
				doc := inst.manager.Active()
				if doc == nil {
					return snapshot{}, fmt.Errorf("no active document")
				}
				if err := doc.ApplyLayerMask(payload.LayerID); err != nil {
					return snapshot{}, err
				}
				if err := inst.manager.ReplaceActive(doc); err != nil {
					return snapshot{}, err
				}
				return inst.captureSnapshot(), nil
			},
		}
		if err := inst.history.Execute(inst, command); err != nil {
			return RenderResult{}, err
		}
	case commandInvertLayerMask:
		var payload InvertLayerMaskPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return RenderResult{}, err
		}
		command := &snapshotCommand{
			description: "Invert layer mask",
			applyFn: func(inst *instance) (snapshot, error) {
				doc := inst.manager.Active()
				if doc == nil {
					return snapshot{}, fmt.Errorf("no active document")
				}
				if err := doc.InvertLayerMask(payload.LayerID); err != nil {
					return snapshot{}, err
				}
				if err := inst.manager.ReplaceActive(doc); err != nil {
					return snapshot{}, err
				}
				return inst.captureSnapshot(), nil
			},
		}
		if err := inst.history.Execute(inst, command); err != nil {
			return RenderResult{}, err
		}
	case commandSetMaskEnabled:
		var payload SetLayerMaskEnabledPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return RenderResult{}, err
		}
		command := &snapshotCommand{
			description: "Toggle layer mask",
			applyFn: func(inst *instance) (snapshot, error) {
				doc := inst.manager.Active()
				if doc == nil {
					return snapshot{}, fmt.Errorf("no active document")
				}
				if err := doc.SetLayerMaskEnabled(payload.LayerID, payload.Enabled); err != nil {
					return snapshot{}, err
				}
				if err := inst.manager.ReplaceActive(doc); err != nil {
					return snapshot{}, err
				}
				return inst.captureSnapshot(), nil
			},
		}
		if err := inst.history.Execute(inst, command); err != nil {
			return RenderResult{}, err
		}
	case commandSetLayerClip:
		var payload SetLayerClipToBelowPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return RenderResult{}, err
		}
		command := &snapshotCommand{
			description: "Set clipping mask",
			applyFn: func(inst *instance) (snapshot, error) {
				doc := inst.manager.Active()
				if doc == nil {
					return snapshot{}, fmt.Errorf("no active document")
				}
				if err := doc.SetLayerClipToBelow(payload.LayerID, payload.ClipToBelow); err != nil {
					return snapshot{}, err
				}
				if err := inst.manager.ReplaceActive(doc); err != nil {
					return snapshot{}, err
				}
				return inst.captureSnapshot(), nil
			},
		}
		if err := inst.history.Execute(inst, command); err != nil {
			return RenderResult{}, err
		}
	case commandSetActiveLayer:
		var payload SetActiveLayerPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return RenderResult{}, err
		}
		doc := inst.manager.Active()
		if doc == nil {
			return RenderResult{}, fmt.Errorf("no active document")
		}
		if err := doc.SetActiveLayer(payload.LayerID); err != nil {
			return RenderResult{}, err
		}
		if err := inst.manager.ReplaceActive(doc); err != nil {
			return RenderResult{}, err
		}
	case commandSetLayerName:
		var payload SetLayerNamePayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return RenderResult{}, err
		}
		command := &snapshotCommand{
			description: "Rename layer",
			applyFn: func(inst *instance) (snapshot, error) {
				doc := inst.manager.Active()
				if doc == nil {
					return snapshot{}, fmt.Errorf("no active document")
				}
				if err := doc.SetLayerName(payload.LayerID, payload.Name); err != nil {
					return snapshot{}, err
				}
				if err := inst.manager.ReplaceActive(doc); err != nil {
					return snapshot{}, err
				}
				return inst.captureSnapshot(), nil
			},
		}
		if err := inst.history.Execute(inst, command); err != nil {
			return RenderResult{}, err
		}
	case commandAddVectorMask:
		var payload AddVectorMaskPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return RenderResult{}, err
		}
		command := &snapshotCommand{
			description: "Add vector mask",
			applyFn: func(inst *instance) (snapshot, error) {
				doc := inst.manager.Active()
				if doc == nil {
					return snapshot{}, fmt.Errorf("no active document")
				}
				if err := doc.AddVectorMask(payload.LayerID); err != nil {
					return snapshot{}, err
				}
				if err := inst.manager.ReplaceActive(doc); err != nil {
					return snapshot{}, err
				}
				return inst.captureSnapshot(), nil
			},
		}
		if err := inst.history.Execute(inst, command); err != nil {
			return RenderResult{}, err
		}
	case commandDeleteVectorMask:
		var payload DeleteVectorMaskPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return RenderResult{}, err
		}
		command := &snapshotCommand{
			description: "Delete vector mask",
			applyFn: func(inst *instance) (snapshot, error) {
				doc := inst.manager.Active()
				if doc == nil {
					return snapshot{}, fmt.Errorf("no active document")
				}
				if err := doc.DeleteVectorMask(payload.LayerID); err != nil {
					return snapshot{}, err
				}
				if err := inst.manager.ReplaceActive(doc); err != nil {
					return snapshot{}, err
				}
				return inst.captureSnapshot(), nil
			},
		}
		if err := inst.history.Execute(inst, command); err != nil {
			return RenderResult{}, err
		}
	case commandSetMaskEditMode:
		var payload SetMaskEditModePayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return RenderResult{}, err
		}
		// Mask edit mode is UI state only — not tracked in history.
		if payload.Editing {
			inst.maskEditLayerID = payload.LayerID
		} else {
			inst.maskEditLayerID = ""
		}
	case commandGetLayerThumbnails:
		// Read-only command: return a render result with thumbnails embedded.
		result := inst.render()
		doc := inst.manager.Active()
		if doc != nil {
			thumbs, err := doc.generateAllThumbnails(thumbnailSize, thumbnailSize)
			if err == nil {
				result.Thumbnails = thumbs
			}
		}
		return result, nil
	case commandFlattenImage:
		command := &snapshotCommand{
			description: "Flatten image",
			applyFn: func(inst *instance) (snapshot, error) {
				doc := inst.manager.Active()
				if doc == nil {
					return snapshot{}, fmt.Errorf("no active document")
				}
				if err := doc.FlattenImage(); err != nil {
					return snapshot{}, err
				}
				if err := inst.manager.ReplaceActive(doc); err != nil {
					return snapshot{}, err
				}
				return inst.captureSnapshot(), nil
			},
		}
		if err := inst.history.Execute(inst, command); err != nil {
			return RenderResult{}, err
		}
	case commandOpenImageFile:
		var payload OpenImageFilePayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return RenderResult{}, err
		}
		command := &snapshotCommand{
			description: fmt.Sprintf("Open image: %s", payload.Name),
			applyFn: func(inst *instance) (snapshot, error) {
				doc := inst.newDocument(CreateDocumentPayload{
					Name:   payload.Name,
					Width:  payload.Width,
					Height: payload.Height,
				})
				bounds := LayerBounds{X: 0, Y: 0, W: payload.Width, H: payload.Height}
				layer := NewPixelLayer("Background", bounds, payload.Pixels)
				if err := doc.AddLayer(layer, doc.LayerRoot.ID(), -1); err != nil {
					return snapshot{}, err
				}
				inst.manager.Create(doc)
				inst.viewport.CenterX = float64(doc.Width) * 0.5
				inst.viewport.CenterY = float64(doc.Height) * 0.5
				inst.fitViewportToActiveDocument()
				return inst.captureSnapshot(), nil
			},
		}
		if err := inst.history.Execute(inst, command); err != nil {
			return RenderResult{}, err
		}
	case commandTranslateLayer:
		var payload TranslateLayerPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return RenderResult{}, err
		}
		command := &snapshotCommand{
			description: "Move layer",
			applyFn: func(inst *instance) (snapshot, error) {
				doc := inst.manager.Active()
				if doc == nil {
					return snapshot{}, fmt.Errorf("no active document")
				}
				if err := doc.TranslateLayer(payload.LayerID, payload.DX, payload.DY); err != nil {
					return snapshot{}, err
				}
				if err := inst.manager.ReplaceActive(doc); err != nil {
					return snapshot{}, err
				}
				return inst.captureSnapshot(), nil
			},
		}
		if err := inst.history.Execute(inst, command); err != nil {
			return RenderResult{}, err
		}
	case commandPickLayerAtPoint:
		var payload PickLayerAtPointPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return RenderResult{}, err
		}
		doc := inst.manager.Active()
		if doc == nil {
			return RenderResult{}, fmt.Errorf("no active document")
		}
		if _, err := doc.PickLayerAtPoint(payload.X, payload.Y); err != nil {
			return RenderResult{}, err
		}
		if err := inst.manager.ReplaceActive(doc); err != nil {
			return RenderResult{}, err
		}
	case commandNewSelection:
		var payload CreateSelectionPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return RenderResult{}, err
		}
		command := &snapshotCommand{
			description: "Set selection",
			applyFn: func(inst *instance) (snapshot, error) {
				doc := inst.manager.Active()
				if doc == nil {
					return snapshot{}, fmt.Errorf("no active document")
				}
				if err := doc.CreateSelection(payload.Shape, payload.Rect, payload.Polygon, payload.Mode, payload.AntiAlias); err != nil {
					return snapshot{}, err
				}
				if err := inst.manager.ReplaceActive(doc); err != nil {
					return snapshot{}, err
				}
				return inst.captureSnapshot(), nil
			},
		}
		if err := inst.history.Execute(inst, command); err != nil {
			return RenderResult{}, err
		}
	case commandSelectAll:
		command := &snapshotCommand{
			description: "Select all",
			applyFn: func(inst *instance) (snapshot, error) {
				doc := inst.manager.Active()
				if doc == nil {
					return snapshot{}, fmt.Errorf("no active document")
				}
				if err := doc.SelectAll(); err != nil {
					return snapshot{}, err
				}
				if err := inst.manager.ReplaceActive(doc); err != nil {
					return snapshot{}, err
				}
				return inst.captureSnapshot(), nil
			},
		}
		if err := inst.history.Execute(inst, command); err != nil {
			return RenderResult{}, err
		}
	case commandDeselect:
		command := &snapshotCommand{
			description: "Deselect",
			applyFn: func(inst *instance) (snapshot, error) {
				doc := inst.manager.Active()
				if doc == nil {
					return snapshot{}, fmt.Errorf("no active document")
				}
				if err := doc.Deselect(); err != nil {
					return snapshot{}, err
				}
				if err := inst.manager.ReplaceActive(doc); err != nil {
					return snapshot{}, err
				}
				return inst.captureSnapshot(), nil
			},
		}
		if err := inst.history.Execute(inst, command); err != nil {
			return RenderResult{}, err
		}
	case commandReselect:
		command := &snapshotCommand{
			description: "Reselect",
			applyFn: func(inst *instance) (snapshot, error) {
				doc := inst.manager.Active()
				if doc == nil {
					return snapshot{}, fmt.Errorf("no active document")
				}
				if err := doc.Reselect(); err != nil {
					return snapshot{}, err
				}
				if err := inst.manager.ReplaceActive(doc); err != nil {
					return snapshot{}, err
				}
				return inst.captureSnapshot(), nil
			},
		}
		if err := inst.history.Execute(inst, command); err != nil {
			return RenderResult{}, err
		}
	case commandInvertSelection:
		command := &snapshotCommand{
			description: "Invert selection",
			applyFn: func(inst *instance) (snapshot, error) {
				doc := inst.manager.Active()
				if doc == nil {
					return snapshot{}, fmt.Errorf("no active document")
				}
				if err := doc.InvertSelection(); err != nil {
					return snapshot{}, err
				}
				if err := inst.manager.ReplaceActive(doc); err != nil {
					return snapshot{}, err
				}
				return inst.captureSnapshot(), nil
			},
		}
		if err := inst.history.Execute(inst, command); err != nil {
			return RenderResult{}, err
		}
	case commandFeatherSelection:
		var payload FeatherSelectionPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return RenderResult{}, err
		}
		command := &snapshotCommand{
			description: "Feather selection",
			applyFn: func(inst *instance) (snapshot, error) {
				doc := inst.manager.Active()
				if doc == nil {
					return snapshot{}, fmt.Errorf("no active document")
				}
				if err := doc.FeatherSelection(payload.Radius); err != nil {
					return snapshot{}, err
				}
				if err := inst.manager.ReplaceActive(doc); err != nil {
					return snapshot{}, err
				}
				return inst.captureSnapshot(), nil
			},
		}
		if err := inst.history.Execute(inst, command); err != nil {
			return RenderResult{}, err
		}
	case commandExpandSelection:
		var payload ExpandSelectionPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return RenderResult{}, err
		}
		command := &snapshotCommand{
			description: "Expand selection",
			applyFn: func(inst *instance) (snapshot, error) {
				doc := inst.manager.Active()
				if doc == nil {
					return snapshot{}, fmt.Errorf("no active document")
				}
				if err := doc.ExpandSelection(payload.Pixels); err != nil {
					return snapshot{}, err
				}
				if err := inst.manager.ReplaceActive(doc); err != nil {
					return snapshot{}, err
				}
				return inst.captureSnapshot(), nil
			},
		}
		if err := inst.history.Execute(inst, command); err != nil {
			return RenderResult{}, err
		}
	case commandContractSelection:
		var payload ContractSelectionPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return RenderResult{}, err
		}
		command := &snapshotCommand{
			description: "Contract selection",
			applyFn: func(inst *instance) (snapshot, error) {
				doc := inst.manager.Active()
				if doc == nil {
					return snapshot{}, fmt.Errorf("no active document")
				}
				if err := doc.ContractSelection(payload.Pixels); err != nil {
					return snapshot{}, err
				}
				if err := inst.manager.ReplaceActive(doc); err != nil {
					return snapshot{}, err
				}
				return inst.captureSnapshot(), nil
			},
		}
		if err := inst.history.Execute(inst, command); err != nil {
			return RenderResult{}, err
		}
	case commandSmoothSelection:
		var payload SmoothSelectionPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return RenderResult{}, err
		}
		command := &snapshotCommand{
			description: "Smooth selection",
			applyFn: func(inst *instance) (snapshot, error) {
				doc := inst.manager.Active()
				if doc == nil {
					return snapshot{}, fmt.Errorf("no active document")
				}
				if err := doc.SmoothSelection(payload.Radius); err != nil {
					return snapshot{}, err
				}
				if err := inst.manager.ReplaceActive(doc); err != nil {
					return snapshot{}, err
				}
				return inst.captureSnapshot(), nil
			},
		}
		if err := inst.history.Execute(inst, command); err != nil {
			return RenderResult{}, err
		}
	case commandBorderSelection:
		var payload BorderSelectionPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return RenderResult{}, err
		}
		command := &snapshotCommand{
			description: "Border selection",
			applyFn: func(inst *instance) (snapshot, error) {
				doc := inst.manager.Active()
				if doc == nil {
					return snapshot{}, fmt.Errorf("no active document")
				}
				if err := doc.BorderSelection(payload.Width); err != nil {
					return snapshot{}, err
				}
				if err := inst.manager.ReplaceActive(doc); err != nil {
					return snapshot{}, err
				}
				return inst.captureSnapshot(), nil
			},
		}
		if err := inst.history.Execute(inst, command); err != nil {
			return RenderResult{}, err
		}
	case commandTransformSelection:
		var payload TransformSelectionPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return RenderResult{}, err
		}
		command := &snapshotCommand{
			description: "Transform selection",
			applyFn: func(inst *instance) (snapshot, error) {
				doc := inst.manager.Active()
				if doc == nil {
					return snapshot{}, fmt.Errorf("no active document")
				}
				if err := doc.TransformSelection(payload.A, payload.B, payload.C, payload.D, payload.TX, payload.TY); err != nil {
					return snapshot{}, err
				}
				if err := inst.manager.ReplaceActive(doc); err != nil {
					return snapshot{}, err
				}
				return inst.captureSnapshot(), nil
			},
		}
		if err := inst.history.Execute(inst, command); err != nil {
			return RenderResult{}, err
		}
	case commandSelectColorRange:
		var payload SelectColorRangePayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return RenderResult{}, err
		}
		command := &snapshotCommand{
			description: "Color range selection",
			applyFn: func(inst *instance) (snapshot, error) {
				doc := inst.manager.Active()
				if doc == nil {
					return snapshot{}, fmt.Errorf("no active document")
				}
				if err := doc.SelectColorRange(payload.LayerID, payload.TargetColor, payload.Fuzziness, payload.SampleMerged, payload.Mode); err != nil {
					return snapshot{}, err
				}
				if err := inst.manager.ReplaceActive(doc); err != nil {
					return snapshot{}, err
				}
				return inst.captureSnapshot(), nil
			},
		}
		if err := inst.history.Execute(inst, command); err != nil {
			return RenderResult{}, err
		}
	case commandQuickSelect:
		var payload QuickSelectPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return RenderResult{}, err
		}
		command := &snapshotCommand{
			description: "Quick selection",
			applyFn: func(inst *instance) (snapshot, error) {
				doc := inst.manager.Active()
				if doc == nil {
					return snapshot{}, fmt.Errorf("no active document")
				}
				if err := doc.QuickSelect(payload.X, payload.Y, payload.Tolerance, payload.EdgeSensitivity, payload.LayerID, payload.SampleMerged, payload.Mode); err != nil {
					return snapshot{}, err
				}
				if err := inst.manager.ReplaceActive(doc); err != nil {
					return snapshot{}, err
				}
				return inst.captureSnapshot(), nil
			},
		}
		if err := inst.history.Execute(inst, command); err != nil {
			return RenderResult{}, err
		}
	case commandMagicWand:
		var payload MagicWandPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return RenderResult{}, err
		}
		command := &snapshotCommand{
			description: "Magic wand selection",
			applyFn: func(inst *instance) (snapshot, error) {
				doc := inst.manager.Active()
				if doc == nil {
					return snapshot{}, fmt.Errorf("no active document")
				}
				if err := doc.MagicWand(payload.X, payload.Y, payload.Tolerance, payload.LayerID, payload.SampleMerged, payload.Contiguous, payload.AntiAlias, payload.Mode); err != nil {
					return snapshot{}, err
				}
				if err := inst.manager.ReplaceActive(doc); err != nil {
					return snapshot{}, err
				}
				return inst.captureSnapshot(), nil
			},
		}
		if err := inst.history.Execute(inst, command); err != nil {
			return RenderResult{}, err
		}
	case commandResize:
		var payload ResizePayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return RenderResult{}, err
		}
		inst.viewport.CanvasW = maxInt(payload.CanvasW, 1)
		inst.viewport.CanvasH = maxInt(payload.CanvasH, 1)
		inst.viewport.DevicePixelRatio = floatValueOrDefault(payload.DevicePixelRatio, defaultDevicePixelRat)
	case commandPointerEvent:
		var payload PointerEventPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return RenderResult{}, err
		}
		inst.handlePointerEvent(payload)
	case commandBeginTxn:
		var payload BeginTransactionPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return RenderResult{}, err
		}
		inst.history.BeginTransaction(inst, stringValueOrDefault(payload.Description, "Transaction"))
	case commandEndTxn:
		var payload EndTransactionPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return RenderResult{}, err
		}
		commit := payload.Commit
		if payloadJSON == "" {
			commit = true
		}
		inst.history.EndTransaction(commit)
	case commandJumpHistory:
		var payload JumpHistoryPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return RenderResult{}, err
		}
		if err := inst.history.JumpTo(inst, payload.HistoryIndex); err != nil {
			return RenderResult{}, err
		}
	case commandSetShowGuides:
		var payload SetShowGuidesPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return RenderResult{}, err
		}
		inst.viewport.ShowGuides = payload.Show
	case commandClearHistory:
		inst.history.Clear()
	case commandFitToView:
		command := &snapshotCommand{
			description: "Fit document on screen",
			applyFn: func(inst *instance) (snapshot, error) {
				inst.fitViewportToActiveDocument()
				return inst.captureSnapshot(), nil
			},
		}
		if err := inst.history.Execute(inst, command); err != nil {
			return RenderResult{}, err
		}
	case commandUndo:
		if err := inst.history.Undo(inst); err != nil {
			return RenderResult{}, err
		}
	case commandRedo:
		if err := inst.history.Redo(inst); err != nil {
			return RenderResult{}, err
		}
	case commandMagneticLassoSuggestPath:
		var payload MagneticLassoSuggestPathPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return RenderResult{}, err
		}
		doc := inst.manager.Active()
		if doc == nil {
			return RenderResult{}, fmt.Errorf("no active document")
		}
		surface, err := doc.selectionSourceSurface(payload.LayerID, payload.SampleMerged)
		if err != nil {
			return RenderResult{}, err
		}
		suggestedPath = suggestMagneticPath(surface, doc.Width, doc.Height, payload.X1, payload.Y1, payload.X2, payload.Y2)

	// Phase 3.3 – Free Transform
	case commandBeginFreeTransform:
		var payload BeginFreeTransformPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return RenderResult{}, err
		}
		doc := inst.manager.Active()
		if doc == nil {
			return RenderResult{}, fmt.Errorf("no active document")
		}
		layerID := payload.LayerID
		if layerID == "" {
			layerID = doc.ActiveLayerID
		}
		layer := doc.findLayer(layerID)
		if layer == nil {
			return RenderResult{}, fmt.Errorf("layer %q not found", layerID)
		}
		pl, ok := layer.(*PixelLayer)
		if !ok {
			return RenderResult{}, fmt.Errorf("free transform only supported on pixel layers")
		}
		// Phase 3.3 – Floating selection: lift selected pixels into a temp layer.
		if sel := doc.Selection; sel != nil {
			floatPixels, floatBounds, hasContent := extractSelectionContent(pl, sel)
			if hasContent {
				preBegin := inst.captureSnapshot()
				origSrcPixels := append([]byte(nil), pl.Pixels...)
				origSrcBounds := pl.Bounds
				clearSelectionContent(pl, sel)
				floatingLayer := NewPixelLayer("Floating Selection", floatBounds, floatPixels)
				if _, srcParent, srcIndex, ok2 := findLayerByID(doc.ensureLayerRoot(), layerID); ok2 {
					insertChild(srcParent, floatingLayer, srcIndex+1)
				}
				doc.ActiveLayerID = floatingLayer.ID()
				doc.ContentVersion++
				if err := inst.manager.ReplaceActive(doc); err != nil {
					return RenderResult{}, err
				}
				inst.cachedDocContentVersion = -1
				inst.freeTransform = &FreeTransformState{
					Active:               true,
					LayerID:              floatingLayer.ID(),
					OriginalPixels:       append([]byte(nil), floatPixels...),
					OriginalBounds:       floatBounds,
					IsFloating:           true,
					SourceLayerID:        layerID,
					OriginalSourcePixels: origSrcPixels,
					OriginalSourceBounds: origSrcBounds,
					PreBeginSnapshot:     &preBegin,
					A:                    1, B: 0, C: 0, D: 1,
					TX:            float64(floatBounds.X),
					TY:            float64(floatBounds.Y),
					PivotX:        float64(floatBounds.X) + float64(floatBounds.W)*0.5,
					PivotY:        float64(floatBounds.Y) + float64(floatBounds.H)*0.5,
					Interpolation: InterpolBilinear,
				}
				if payload.Mode == "warp" {
					inst.freeTransform.WarpGrid = initWarpGridFromBounds(floatBounds)
				}
				break
			}
		}
		// Normal (full-layer) free transform.
		inst.freeTransform = &FreeTransformState{
			Active:         true,
			LayerID:        layerID,
			OriginalPixels: append([]byte(nil), pl.Pixels...),
			OriginalBounds: pl.Bounds,
			A:              1, B: 0, C: 0, D: 1,
			TX:            float64(pl.Bounds.X),
			TY:            float64(pl.Bounds.Y),
			PivotX:        float64(pl.Bounds.X) + float64(pl.Bounds.W)*0.5,
			PivotY:        float64(pl.Bounds.Y) + float64(pl.Bounds.H)*0.5,
			Interpolation: InterpolBilinear,
		}
		if payload.Mode == "warp" {
			inst.freeTransform.WarpGrid = initWarpGridFromBounds(pl.Bounds)
		}

	case commandUpdateFreeTransform:
		var payload UpdateFreeTransformPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return RenderResult{}, err
		}
		if inst.freeTransform == nil || !inst.freeTransform.Active {
			return RenderResult{}, fmt.Errorf("no active free transform")
		}
		doc := inst.manager.Active()
		if doc == nil {
			return RenderResult{}, fmt.Errorf("no active document")
		}
		layer := doc.findLayer(inst.freeTransform.LayerID)
		pl, ok := layer.(*PixelLayer)
		if !ok || pl == nil {
			return RenderResult{}, fmt.Errorf("transform layer not found or wrong type")
		}
		inst.freeTransform.A = payload.A
		inst.freeTransform.B = payload.B
		inst.freeTransform.C = payload.C
		inst.freeTransform.D = payload.D
		inst.freeTransform.TX = payload.TX
		inst.freeTransform.TY = payload.TY
		inst.freeTransform.PivotX = payload.PivotX
		inst.freeTransform.PivotY = payload.PivotY
		if payload.Interpolation != "" {
			inst.freeTransform.Interpolation = InterpolMode(payload.Interpolation)
		}
		// Handle distort/warp mode (mutually exclusive with affine).
		if payload.WarpGrid != nil {
			inst.freeTransform.WarpGrid = payload.WarpGrid
			inst.freeTransform.DistortCorners = nil
		} else if payload.Corners != nil {
			inst.freeTransform.DistortCorners = payload.Corners
			inst.freeTransform.WarpGrid = nil
		} else {
			inst.freeTransform.DistortCorners = nil
			inst.freeTransform.WarpGrid = nil
		}
		// Apply preview (bilinear always for responsiveness).
		previewPixels, previewBounds := applyPixelTransform(inst.freeTransform, InterpolBilinear)
		pl.Pixels = previewPixels
		pl.Bounds = previewBounds
		doc.ContentVersion++
		if err := inst.manager.ReplaceActive(doc); err != nil {
			return RenderResult{}, err
		}
		inst.cachedDocContentVersion = -1 // force composite rebuild

	case commandCommitFreeTransform:
		if inst.freeTransform == nil || !inst.freeTransform.Active {
			return RenderResult{}, fmt.Errorf("no active free transform")
		}
		doc := inst.manager.Active()
		if doc == nil {
			return RenderResult{}, fmt.Errorf("no active document")
		}
		layer := doc.findLayer(inst.freeTransform.LayerID)
		pl, ok := layer.(*PixelLayer)
		if !ok || pl == nil {
			return RenderResult{}, fmt.Errorf("transform layer not found or wrong type")
		}
		// Compute final pixels from the original (always uses OriginalPixels as source).
		finalPixels, finalBounds := applyPixelTransform(inst.freeTransform, inst.freeTransform.Interpolation)
		ft := inst.freeTransform

		if ft.IsFloating {
			// Floating-selection commit: restore the pre-begin document state so the
			// history "before" snapshot reflects the state before the transform started,
			// then merge the transformed pixels back into the source layer.
			if err := inst.restoreSnapshot(*ft.PreBeginSnapshot); err != nil {
				return RenderResult{}, err
			}
			command := &snapshotCommand{
				description: "Transform Selection",
				applyFn: func(inst *instance) (snapshot, error) {
					d := inst.manager.Active()
					if d == nil {
						return snapshot{}, fmt.Errorf("no active document")
					}
					srcLayer := d.findLayer(ft.SourceLayerID)
					sl, ok := srcLayer.(*PixelLayer)
					if !ok || sl == nil {
						return snapshot{}, fmt.Errorf("source layer not found")
					}
					mergePixelLayerOnto(sl, finalPixels, finalBounds)
					d.Selection = nil
					d.ActiveLayerID = ft.SourceLayerID
					d.ContentVersion++
					if err := inst.manager.ReplaceActive(d); err != nil {
						return snapshot{}, err
					}
					return inst.captureSnapshot(), nil
				},
			}
			if err := inst.history.Execute(inst, command); err != nil {
				return RenderResult{}, err
			}
			inst.lastTransform = recordLastFreeTransform(ft)
			inst.freeTransform = nil
			inst.cachedDocContentVersion = -1
			break
		}

		// Normal (full-layer) commit.
		command := &snapshotCommand{
			description: "Free Transform",
			applyFn: func(inst *instance) (snapshot, error) {
				d := inst.manager.Active()
				if d == nil {
					return snapshot{}, fmt.Errorf("no active document")
				}
				l := d.findLayer(ft.LayerID)
				p, ok := l.(*PixelLayer)
				if !ok || p == nil {
					return snapshot{}, fmt.Errorf("layer not found")
				}
				p.Pixels = finalPixels
				p.Bounds = finalBounds
				d.ContentVersion++
				if err := inst.manager.ReplaceActive(d); err != nil {
					return snapshot{}, err
				}
				return inst.captureSnapshot(), nil
			},
		}
		if err := inst.history.Execute(inst, command); err != nil {
			return RenderResult{}, err
		}
		inst.lastTransform = recordLastFreeTransform(ft)
		inst.freeTransform = nil
		inst.cachedDocContentVersion = -1

	case commandCancelFreeTransform:
		if inst.freeTransform == nil || !inst.freeTransform.Active {
			inst.freeTransform = nil
			break
		}
		doc := inst.manager.Active()
		if doc == nil {
			return RenderResult{}, fmt.Errorf("no active document")
		}
		ft := inst.freeTransform
		if ft.IsFloating {
			// Restore source layer to its pre-begin state and remove floating layer.
			if srcLayer := doc.findLayer(ft.SourceLayerID); srcLayer != nil {
				if sl, ok := srcLayer.(*PixelLayer); ok {
					sl.Pixels = ft.OriginalSourcePixels
					sl.Bounds = ft.OriginalSourceBounds
				}
			}
			_ = doc.DeleteLayer(ft.LayerID)
			doc.ActiveLayerID = ft.SourceLayerID
		} else {
			layer := doc.findLayer(ft.LayerID)
			if pl, ok := layer.(*PixelLayer); ok && pl != nil {
				pl.Pixels = ft.OriginalPixels
				pl.Bounds = ft.OriginalBounds
			}
		}
		doc.ContentVersion++
		if err := inst.manager.ReplaceActive(doc); err != nil {
			return RenderResult{}, err
		}
		inst.freeTransform = nil
		inst.cachedDocContentVersion = -1

	// Phase 3.3 – Discrete transforms (flip / rotate)
	case commandFlipLayerH, commandFlipLayerV,
		commandRotateLayer90CW, commandRotateLayer90CCW, commandRotateLayer180:
		var payload DiscreteTransformPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return RenderResult{}, err
		}
		kind := map[int32]string{
			commandFlipLayerH:       "flipH",
			commandFlipLayerV:       "flipV",
			commandRotateLayer90CW:  "rotate90cw",
			commandRotateLayer90CCW: "rotate90ccw",
			commandRotateLayer180:   "rotate180",
		}[commandID]
		command := &snapshotCommand{
			description: kindDescription(kind),
			applyFn: func(inst *instance) (snapshot, error) {
				doc := inst.manager.Active()
				if doc == nil {
					return snapshot{}, fmt.Errorf("no active document")
				}
				layerID := payload.LayerID
				if layerID == "" {
					layerID = doc.ActiveLayerID
				}
				l := doc.findLayer(layerID)
				pl, ok := l.(*PixelLayer)
				if !ok || pl == nil {
					return snapshot{}, fmt.Errorf("layer %q is not a pixel layer", layerID)
				}
				applyDiscreteTransformToLayer(pl, kind)
				doc.ContentVersion++
				if err := inst.manager.ReplaceActive(doc); err != nil {
					return snapshot{}, err
				}
				return inst.captureSnapshot(), nil
			},
		}
		if err := inst.history.Execute(inst, command); err != nil {
			return RenderResult{}, err
		}
		inst.lastTransform = &LastTransformRecord{Kind: kind}
		inst.cachedDocContentVersion = -1

	case commandTransformAgain:
		if inst.lastTransform == nil {
			return RenderResult{}, fmt.Errorf("no previous transform to repeat")
		}
		doc := inst.manager.Active()
		if doc == nil {
			return RenderResult{}, fmt.Errorf("no active document")
		}
		lt := inst.lastTransform
		if lt.Kind == "free" {
			l := doc.findLayer(doc.ActiveLayerID)
			pl, ok := l.(*PixelLayer)
			if !ok || pl == nil {
				return RenderResult{}, fmt.Errorf("active layer is not a pixel layer")
			}
			finalPixels, finalBounds := applyLastFreeTransform(lt, pl)
			command := &snapshotCommand{
				description: "Transform Again",
				applyFn: func(inst *instance) (snapshot, error) {
					d := inst.manager.Active()
					if d == nil {
						return snapshot{}, fmt.Errorf("no active document")
					}
					layer := d.findLayer(d.ActiveLayerID)
					p, ok := layer.(*PixelLayer)
					if !ok || p == nil {
						return snapshot{}, fmt.Errorf("layer not found")
					}
					p.Pixels = finalPixels
					p.Bounds = finalBounds
					d.ContentVersion++
					if err := inst.manager.ReplaceActive(d); err != nil {
						return snapshot{}, err
					}
					return inst.captureSnapshot(), nil
				},
			}
			if err := inst.history.Execute(inst, command); err != nil {
				return RenderResult{}, err
			}
		} else {
			// Discrete transform again.
			kind := lt.Kind
			command := &snapshotCommand{
				description: kindDescription(kind) + " Again",
				applyFn: func(inst *instance) (snapshot, error) {
					d := inst.manager.Active()
					if d == nil {
						return snapshot{}, fmt.Errorf("no active document")
					}
					layer := d.findLayer(d.ActiveLayerID)
					p, ok := layer.(*PixelLayer)
					if !ok || p == nil {
						return snapshot{}, fmt.Errorf("active layer is not a pixel layer")
					}
					applyDiscreteTransformToLayer(p, kind)
					d.ContentVersion++
					if err := inst.manager.ReplaceActive(d); err != nil {
						return snapshot{}, err
					}
					return inst.captureSnapshot(), nil
				},
			}
			if err := inst.history.Execute(inst, command); err != nil {
				return RenderResult{}, err
			}
		}
		inst.cachedDocContentVersion = -1

	case commandBeginCrop:
		doc := inst.manager.Active()
		if doc == nil {
			return RenderResult{}, fmt.Errorf("no active document")
		}
		inst.crop = &CropState{
			Active:       true,
			X:            0,
			Y:            0,
			W:            float64(doc.Width),
			H:            float64(doc.Height),
			Rotation:     0,
			DeletePixels: false,
		}

	case commandUpdateCrop:
		var payload UpdateCropPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return RenderResult{}, err
		}
		if inst.crop == nil || !inst.crop.Active {
			return RenderResult{}, fmt.Errorf("no active crop tool")
		}
		inst.crop.X = payload.X
		inst.crop.Y = payload.Y
		inst.crop.W = payload.W
		inst.crop.H = payload.H
		inst.crop.Rotation = payload.Rotation
		inst.crop.DeletePixels = payload.DeletePixels

	case commandCommitCrop:
		if inst.crop == nil || !inst.crop.Active {
			return RenderResult{}, fmt.Errorf("no active crop tool")
		}
		// Capture state before clearing
		cropX := inst.crop.X
		cropY := inst.crop.Y
		cropW := inst.crop.W
		cropH := inst.crop.H
		cropRot := inst.crop.Rotation
		deletePixels := inst.crop.DeletePixels
		command := &snapshotCommand{
			description: "Crop Document",
			applyFn: func(inst *instance) (snapshot, error) {
				doc := inst.manager.Active()
				if doc == nil {
					return snapshot{}, fmt.Errorf("no active document")
				}
				w := int(math.Round(cropW))
				h := int(math.Round(cropH))
				if w <= 0 || h <= 0 {
					return snapshot{}, fmt.Errorf("invalid crop dimensions: %dx%d", w, h)
				}

				rotRad := cropRot * math.Pi / 180
				cx := cropX + cropW/2
				cy := cropY + cropH/2

				if cropRot != 0 {
					// Rotated crop: resample each pixel layer
					walkLayerTree(doc.LayerRoot, func(n LayerNode) {
						if pl, ok := n.(*PixelLayer); ok {
							newPixels, newBounds := applyRotatedCropToPixelLayer(pl, cx, cy, cropW, cropH, rotRad)
							pl.Pixels = newPixels
							pl.Bounds = newBounds
						}
					})
				} else {
					// Axis-aligned crop: shift layer origins, optionally trim pixels
					x := int(math.Round(cropX))
					y := int(math.Round(cropY))
					walkLayerTree(doc.LayerRoot, func(n LayerNode) {
						if pl, ok := n.(*PixelLayer); ok {
							pl.Bounds.X -= x
							pl.Bounds.Y -= y
							if deletePixels {
								trimPixelLayerToBounds(pl, w, h)
							}
						}
					})
				}

				doc.Width = w
				doc.Height = h
				doc.ContentVersion++

				if err := inst.manager.ReplaceActive(doc); err != nil {
					return snapshot{}, err
				}
				return inst.captureSnapshot(), nil
			},
		}
		if err := inst.history.Execute(inst, command); err != nil {
			return RenderResult{}, err
		}
		inst.crop = nil
		inst.cachedDocContentVersion = -1

	case commandCancelCrop:
		inst.crop = nil

	case commandResizeCanvas:
		var payload ResizeCanvasPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return RenderResult{}, err
		}
		command := &snapshotCommand{
			description: "Canvas Size",
			applyFn: func(inst *instance) (snapshot, error) {
				doc := inst.manager.Active()
				if doc == nil {
					return snapshot{}, fmt.Errorf("no active document")
				}
				if err := applyResizeCanvas(doc, payload.Width, payload.Height, payload.Anchor); err != nil {
					return snapshot{}, err
				}
				doc.ContentVersion++
				if err := inst.manager.ReplaceActive(doc); err != nil {
					return snapshot{}, err
				}
				return inst.captureSnapshot(), nil
			},
		}
		if err := inst.history.Execute(inst, command); err != nil {
			return RenderResult{}, err
		}
		inst.cachedDocContentVersion = -1

	case commandBeginPaintStroke:
		var payload BeginPaintStrokePayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return RenderResult{}, err
		}
		inst.handleBeginPaintStroke(payload)

	case commandContinuePaintStroke:
		var payload ContinuePaintStrokePayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return RenderResult{}, err
		}
		inst.handleContinuePaintStroke(payload)

	case commandEndPaintStroke:
		inst.handleEndPaintStroke()

	case commandSetForegroundColor:
		var payload SetColorPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return RenderResult{}, err
		}
		inst.foregroundColor = payload.Color

	case commandSetBackgroundColor:
		var payload SetColorPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return RenderResult{}, err
		}
		inst.backgroundColor = payload.Color

	case commandSampleMergedColor:
		var payload SampleMergedColorPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return RenderResult{}, err
		}
		doc := inst.manager.Active()
		if doc != nil {
			var surface []byte
			var width, height int
			var offsetX, offsetY int
			if payload.SampleMerged {
				surface = inst.compositeSurface(doc)
				width, height = doc.Width, doc.Height
			} else if layer := findPixelLayer(doc, doc.ActiveLayerID); layer != nil {
				surface = layer.Pixels
				width, height = layer.Bounds.W, layer.Bounds.H
				offsetX = layer.Bounds.X
				offsetY = layer.Bounds.Y
			}
			px := int(math.Round(payload.X))
			py := int(math.Round(payload.Y))
			px -= offsetX
			py -= offsetY
			if surface != nil && px >= 0 && py >= 0 && px < width && py < height {
				sampleSize := payload.SampleSize
				if sampleSize <= 0 {
					sampleSize = 1
				}
				if color, ok := sampleSurfaceColorAverage(surface, width, height, px, py, sampleSize); ok {
					result := inst.render()
					result.SuggestedPath = suggestedPath
					result.SampledColor = &color
					return result, nil
				}
			}
		}

	case commandMagicErase:
		var payload MagicErasePayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return RenderResult{}, err
		}
		doc := inst.manager.Active()
		if doc != nil {
			layer := findPixelLayer(doc, doc.ActiveLayerID)
			if layer != nil {
				if err := inst.handleMagicErase(payload, doc, layer); err != nil {
					return RenderResult{}, err
				}
			}
		}

	case commandFill:
		var payload FillPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return RenderResult{}, err
		}
		doc := inst.manager.Active()
		if doc != nil {
			if err := inst.handleFill(payload); err != nil {
				return RenderResult{}, err
			}
		}

	case commandApplyGradient:
		var payload ApplyGradientPayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return RenderResult{}, err
		}
		doc := inst.manager.Active()
		if doc != nil {
			if err := inst.handleApplyGradient(payload); err != nil {
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

func (inst *instance) render() RenderResult {
	frameID := inst.nextFrameID()
	doc := inst.manager.Active()
	if doc == nil {
		inst.pixels = inst.pixels[:0]
		return RenderResult{
			FrameID:     frameID,
			Viewport:    inst.viewport,
			DirtyRects:  []DirtyRect{{X: 0, Y: 0, W: inst.viewport.CanvasW, H: inst.viewport.CanvasH}},
			PixelFormat: "rgba8-premultiplied",
			UIMeta: UIMeta{
				CursorType:          "default",
				StatusText:          "No active document",
				History:             inst.history.Entries(),
				CurrentHistoryIndex: inst.history.CurrentIndex(),
				CanUndo:             inst.history.CanUndo(),
				CanRedo:             inst.history.CanRedo(),
				MaskEditLayerID:     inst.maskEditLayerID,
			},
		}
	}

	activeLayerName := ""
	if activeLayer := doc.ActiveLayer(); activeLayer != nil {
		activeLayerName = activeLayer.Name()
	}

	inst.pixels = inst.renderViewportWithCache(doc, inst.compositeSurface(doc))
	inst.pixels = RenderSelectionOverlay(doc, &inst.viewport, inst.pixels, doc.Selection, frameID)
	inst.pixels = RenderTransformHandlesOverlay(inst.freeTransform, &inst.viewport, inst.pixels)
	inst.pixels = RenderCropOverlay(inst.crop, &inst.viewport, inst.pixels)
	return RenderResult{
		FrameID:     frameID,
		Viewport:    inst.viewport,
		DirtyRects:  []DirtyRect{{X: 0, Y: 0, W: inst.viewport.CanvasW, H: inst.viewport.CanvasH}},
		PixelFormat: "rgba8-premultiplied",
		BufferPtr:   int32(uintptr(unsafe.Pointer(&inst.pixels[0]))), //nolint:unsafeptr
		BufferLen:   int32(len(inst.pixels)),
		UIMeta: UIMeta{
			ActiveLayerID:       doc.ActiveLayerID,
			ActiveLayerName:     activeLayerName,
			CursorType:          inst.cursorType(),
			StatusText:          inst.statusText(doc),
			RulerOriginX:        0,
			RulerOriginY:        0,
			History:             inst.history.Entries(),
			CurrentHistoryIndex: inst.history.CurrentIndex(),
			CanUndo:             inst.history.CanUndo(),
			CanRedo:             inst.history.CanRedo(),
			ActiveDocumentID:    doc.ID,
			ActiveDocumentName:  doc.Name,
			DocumentWidth:       doc.Width,
			DocumentHeight:      doc.Height,
			DocumentBackground:  doc.Background.Kind,
			Layers:              doc.LayerMeta(),
			ContentVersion:      doc.ContentVersion,
			MaskEditLayerID:     inst.maskEditLayerID,
			Selection:           doc.selectionMeta(),
			FreeTransform:       inst.freeTransform.meta(),
			Crop:                inst.crop.meta(),
		},
	}
}

func (inst *instance) handlePointerEvent(event PointerEventPayload) {
	switch event.Phase {
	case "down":
		if !event.PanMode {
			inst.pointer = pointerDragState{}
			return
		}
		inst.history.BeginTransaction(inst, "Pan viewport")
		inst.pointer = pointerDragState{
			PointerID: event.PointerID,
			StartX:    event.X,
			StartY:    event.Y,
			CenterX:   inst.viewport.CenterX,
			CenterY:   inst.viewport.CenterY,
			Zoom:      clampZoom(inst.viewport.Zoom),
			Rotation:  inst.viewport.Rotation,
			Active:    true,
		}
	case "move":
		if !inst.pointer.Active || inst.pointer.PointerID != event.PointerID {
			return
		}
		deltaX := event.X - inst.pointer.StartX
		deltaY := event.Y - inst.pointer.StartY
		docDX, docDY := screenDeltaToDocument(deltaX, deltaY, inst.pointer.Zoom, inst.pointer.Rotation)
		inst.viewport.CenterX = inst.pointer.CenterX - docDX
		inst.viewport.CenterY = inst.pointer.CenterY - docDY
	case "up":
		if inst.pointer.PointerID == event.PointerID {
			inst.pointer = pointerDragState{}
			inst.history.EndTransaction(true)
		}
	}
}

func (inst *instance) cursorType() string {
	if inst.pointer.Active {
		return "grabbing"
	}
	return "default"
}

func (inst *instance) statusText(doc *Document) string {
	return fmt.Sprintf("%s  %d x %d px  %.0f%%  %.0f°",
		doc.Name,
		doc.Width,
		doc.Height,
		inst.viewport.Zoom*100,
		inst.viewport.Rotation,
	)
}

func (inst *instance) nextFrameID() int64 {
	inst.frameID++
	return inst.frameID
}

func (inst *instance) captureSnapshot() snapshot {
	return snapshot{
		DocumentID: inst.manager.ActiveID(),
		Document:   inst.manager.Active(),
		Viewport:   inst.viewport,
	}
}

func (inst *instance) restoreSnapshot(state snapshot) error {
	inst.viewport = state.Viewport
	inst.manager = newDocumentManager()
	if state.Document == nil {
		return nil
	}
	inst.manager.Create(state.Document)
	if state.DocumentID != "" && inst.manager.activeID != state.DocumentID {
		inst.manager.activeID = state.DocumentID
	}
	return nil
}

func (inst *instance) fitViewportToActiveDocument() {
	doc := inst.manager.Active()
	if doc == nil {
		return
	}
	inst.viewport.CenterX = float64(doc.Width) * 0.5
	inst.viewport.CenterY = float64(doc.Height) * 0.5

	canvasW := maxInt(inst.viewport.CanvasW, 1)
	canvasH := maxInt(inst.viewport.CanvasH, 1)
	scaleX := float64(canvasW) * 0.84 / float64(maxInt(doc.Width, 1))
	scaleY := float64(canvasH) * 0.84 / float64(maxInt(doc.Height, 1))
	inst.viewport.Zoom = clampZoom(math.Min(scaleX, scaleY))
}

func (inst *instance) handleBeginPaintStroke(p BeginPaintStrokePayload) {
	doc := inst.manager.activeMut()
	if doc == nil {
		return
	}
	layer := findPixelLayer(doc, doc.ActiveLayerID)
	if layer == nil {
		return
	}
	brushParams := p.Brush
	if brushParams.AutoErase {
		// Sample the active layer pixel at the stroke start.
		// If it matches the brush (foreground) color, switch to background color.
		px := int(math.Round(p.X)) - layer.Bounds.X
		py := int(math.Round(p.Y)) - layer.Bounds.Y
		if px >= 0 && py >= 0 && px < layer.Bounds.W && py < layer.Bounds.H {
			idx := (py*layer.Bounds.W + px) * 4
			fg := brushParams.Color
			if layer.Pixels[idx] == fg[0] && layer.Pixels[idx+1] == fg[1] && layer.Pixels[idx+2] == fg[2] {
				brushParams.Color = inst.backgroundColor
			}
		}
	}

	stroke := &activePaintStroke{
		layerID:    layer.ID(),
		params:     brushParams,
		stabilizer: newStabilizer(brushParams.Stabilizer),
	}

	// Background eraser: sample the pixel under the pointer once at stroke begin.
	if brushParams.EraseBackground {
		px := int(math.Round(p.X)) - layer.Bounds.X
		py := int(math.Round(p.Y)) - layer.Bounds.Y
		if px >= 0 && py >= 0 && px < layer.Bounds.W && py < layer.Bounds.H {
			idx := (py*layer.Bounds.W + px) * 4
			stroke.bgEraseBaseColor = [4]uint8{layer.Pixels[idx], layer.Pixels[idx+1], layer.Pixels[idx+2], layer.Pixels[idx+3]}
		}
	}

	// Pre-create the AGG renderer for the stroke's layer so dab rendering
	// reuses the rasterizer's allocated cell blocks instead of re-allocating.
	stroke.renderer = agglib.NewAgg2D()
	if brushParams.MixerBrush {
		stroke.mixerSource, stroke.mixerSourceW, stroke.mixerSourceH, stroke.mixerSourceX, stroke.mixerSourceY = captureStrokeSourceSurface(doc, layer, brushParams.SampleMerged)
	}
	if brushParams.CloneStamp {
		stroke.cloneSource, stroke.cloneSourceW, stroke.cloneSourceH, stroke.cloneSourceX, stroke.cloneSourceY = captureStrokeSourceSurface(doc, layer, brushParams.SampleMerged)
		stroke.cloneOffsetX = brushParams.CloneSourceX - p.X
		stroke.cloneOffsetY = brushParams.CloneSourceY - p.Y
	}
	if brushParams.HistoryBrush {
		if state, ok := inst.history.PreviousSnapshot(inst); ok {
			stroke.historySource, stroke.historySourceW, stroke.historySourceH, stroke.historySourceX, stroke.historySourceY = captureHistorySourceSurface(state, brushParams.SampleMerged)
		}
	}

	inst.paintStroke = stroke

	pressure := p.Pressure
	if pressure == 0 {
		pressure = 0.5
	}
	effective := applyPressure(brushParams, pressure)
	azimuth, squish := applyTilt(p.TiltX, p.TiltY)
	sx, sy := inst.paintStroke.stabilizer.Push(p.X, p.Y)
	dabs := inst.paintStroke.strokeState.AddPoint(sx, sy, 0.25, effective.Size)
	for _, dab := range dabs {
		dx, dy := applyScatter(dab[0], dab[1], effective)
		dabParams := effective
		stroke.saveRowsBeforeDab(layer, dx, dy, effective.Size, &inst.undoRowBuf)
		if brushParams.EraseBackground {
			EraseBackgroundDab(layer, dx, dy, dabParams, inst.paintStroke.bgEraseBaseColor)
		} else if dabParams.CloneStamp {
			CloneStampDab(layer, inst.paintStroke.cloneSource, inst.paintStroke.cloneSourceW, inst.paintStroke.cloneSourceH, inst.paintStroke.cloneSourceX, inst.paintStroke.cloneSourceY, dx, dy, dabParams, inst.paintStroke.cloneOffsetX, inst.paintStroke.cloneOffsetY)
		} else if dabParams.HistoryBrush {
			CloneStampDab(layer, inst.paintStroke.historySource, inst.paintStroke.historySourceW, inst.paintStroke.historySourceH, inst.paintStroke.historySourceX, inst.paintStroke.historySourceY, dx, dy, dabParams, 0, 0)
		} else {
			if dabParams.MixerBrush {
				dabParams.Color = resolveMixerBrushColor(stroke.mixerSource, stroke.mixerSourceW, stroke.mixerSourceH, stroke.mixerSourceX, stroke.mixerSourceY, dx, dy, dabParams.Color, dabParams.MixerMix)
			}
			paintDabReuse(stroke.renderer, layer, dx, dy, dabParams, azimuth, squish)
		}
		inst.paintStroke.expandDirty(layer, dx, dy, effective.Size)
	}
	doc.ContentVersion++
}

func (inst *instance) handleContinuePaintStroke(p ContinuePaintStrokePayload) {
	if inst.paintStroke == nil {
		return
	}
	doc := inst.manager.activeMut()
	if doc == nil {
		return
	}
	layer := findPixelLayer(doc, inst.paintStroke.layerID)
	if layer == nil {
		return
	}
	pressure := p.Pressure
	if pressure == 0 {
		pressure = 0.5
	}
	effective := applyPressure(inst.paintStroke.params, pressure)
	azimuth, squish := applyTilt(p.TiltX, p.TiltY)
	sx, sy := inst.paintStroke.stabilizer.Push(p.X, p.Y)
	dabs := inst.paintStroke.strokeState.AddPoint(sx, sy, 0.25, effective.Size)
	for _, dab := range dabs {
		dx, dy := applyScatter(dab[0], dab[1], effective)
		dabParams := effective
		inst.paintStroke.saveRowsBeforeDab(layer, dx, dy, effective.Size, &inst.undoRowBuf)
		if inst.paintStroke.params.EraseBackground {
			EraseBackgroundDab(layer, dx, dy, dabParams, inst.paintStroke.bgEraseBaseColor)
		} else if dabParams.CloneStamp {
			CloneStampDab(layer, inst.paintStroke.cloneSource, inst.paintStroke.cloneSourceW, inst.paintStroke.cloneSourceH, inst.paintStroke.cloneSourceX, inst.paintStroke.cloneSourceY, dx, dy, dabParams, inst.paintStroke.cloneOffsetX, inst.paintStroke.cloneOffsetY)
		} else if dabParams.HistoryBrush {
			CloneStampDab(layer, inst.paintStroke.historySource, inst.paintStroke.historySourceW, inst.paintStroke.historySourceH, inst.paintStroke.historySourceX, inst.paintStroke.historySourceY, dx, dy, dabParams, 0, 0)
		} else {
			if dabParams.MixerBrush {
				dabParams.Color = resolveMixerBrushColor(inst.paintStroke.mixerSource, inst.paintStroke.mixerSourceW, inst.paintStroke.mixerSourceH, inst.paintStroke.mixerSourceX, inst.paintStroke.mixerSourceY, dx, dy, dabParams.Color, dabParams.MixerMix)
			}
			paintDabReuse(inst.paintStroke.renderer, layer, dx, dy, dabParams, azimuth, squish)
		}
		inst.paintStroke.expandDirty(layer, dx, dy, effective.Size)
	}
	if len(dabs) > 0 {
		doc.ContentVersion++
	}
}

func (inst *instance) handleEndPaintStroke() {
	if inst.paintStroke == nil {
		return
	}
	doc := inst.manager.activeMut()
	stroke := inst.paintStroke
	inst.paintStroke = nil

	if doc == nil || !stroke.hasDirty {
		return
	}
	layer := findPixelLayer(doc, stroke.layerID)
	if layer == nil {
		return
	}

	rect := DirtyRect{
		X: stroke.dirtyMin[0], Y: stroke.dirtyMin[1],
		W: stroke.dirtyMax[0] - stroke.dirtyMin[0],
		H: stroke.dirtyMax[1] - stroke.dirtyMin[1],
	}
	delta, err := newPixelDeltaFromRows(
		stroke.beforeRowBuf, stroke.beforeRowStart, stroke.layerW,
		layer.Pixels, layer.Bounds.W, layer.Bounds.H, rect,
	)
	if err != nil {
		return
	}
	layerID := stroke.layerID
	cmd := &pixelDeltaCommand{
		description: "Brush stroke",
		target: func(inst *instance) []byte {
			l := findPixelLayer(inst.manager.activeMut(), layerID)
			if l == nil {
				return nil
			}
			return l.Pixels
		},
		delta: delta,
	}
	inst.history.push(cmd)
}

// handleMagicErase implements the Magic Eraser: flood-fills (or global-selects)
// pixels within tolerance of the clicked color and clears their alpha to 0.
// The operation is undoable.
func (inst *instance) handleMagicErase(p MagicErasePayload, doc *Document, layer *PixelLayer) error {
	// Determine the source surface for color sampling.
	var surface []byte
	if p.SampleMerged {
		surface = inst.compositeSurface(doc)
	} else {
		surface = layer.Pixels
	}

	// Convert document-space click to pixel coordinates on the source surface.
	var srcW, srcH int
	var offX, offY int
	if p.SampleMerged {
		srcW, srcH = doc.Width, doc.Height
	} else {
		srcW, srcH = layer.Bounds.W, layer.Bounds.H
		offX, offY = layer.Bounds.X, layer.Bounds.Y
	}
	px := int(math.Round(p.X)) - offX
	py := int(math.Round(p.Y)) - offY
	if px < 0 || py < 0 || px >= srcW || py >= srcH {
		return nil
	}

	// Sample the target color.
	targetColor, ok := sampleSurfaceColor(surface, srcW, srcH, px, py)
	if !ok {
		return nil
	}

	// Build a mask of pixels to erase (reuse selection logic, then apply to layer).
	var mask *Selection
	if p.Contiguous {
		mask = magicWandFloodFill(surface, srcW, srcH, px, py, p.Tolerance)
	} else {
		mask = selectColorRange(surface, srcW, srcH, targetColor, p.Tolerance)
	}
	if mask == nil {
		return nil
	}

	// Snapshot layer pixels for undo.
	before := make([]byte, len(layer.Pixels))
	copy(before, layer.Pixels)

	// Apply mask to layer alpha: multiply dest alpha by (1 - mask/255).
	lw := layer.Bounds.W
	lh := layer.Bounds.H
	for ly := range lh {
		for lx := range lw {
			// Map layer-local coordinates to mask coordinates.
			maskX := lx + layer.Bounds.X - offX
			maskY := ly + layer.Bounds.Y - offY
			if maskX < 0 || maskY < 0 || maskX >= mask.Width || maskY >= mask.Height {
				continue
			}
			coverage := float64(mask.Mask[maskY*mask.Width+maskX]) / 255.0
			if coverage <= 0 {
				continue
			}
			idx := (ly*lw + lx) * 4
			newAlpha := float64(layer.Pixels[idx+3]) * (1.0 - coverage)
			if newAlpha < 0 {
				newAlpha = 0
			}
			layer.Pixels[idx+3] = uint8(newAlpha)
		}
	}
	doc.ContentVersion++

	// Record undo.
	layerID := layer.ID()
	delta, err := NewPixelDelta(before, layer.Pixels, lw, lh, DirtyRect{0, 0, lw, lh})
	if err != nil {
		return nil
	}
	inst.history.push(&pixelDeltaCommand{
		description: "Magic Eraser",
		target: func(inst *instance) []byte {
			l := findPixelLayer(inst.manager.activeMut(), layerID)
			if l == nil {
				return nil
			}
			return l.Pixels
		},
		delta: delta,
	})
	return nil
}

func (inst *instance) newDocument(payload CreateDocumentPayload) *Document {
	width := valueOrDefault(payload.Width, defaultDocWidth)
	height := valueOrDefault(payload.Height, defaultDocHeight)
	timestamp := time.Now().UTC().Format(time.RFC3339)
	return &Document{
		Width:      width,
		Height:     height,
		Resolution: floatValueOrDefault(payload.Resolution, defaultResolutionDPI),
		ColorMode:  stringValueOrDefault(payload.ColorMode, "rgb"),
		BitDepth:   valueOrDefault(payload.BitDepth, 8),
		Background: parseBackground(payload.Background),
		ID:         fmt.Sprintf("doc-%04d", atomic.AddInt64(&nextDocID, 1)),
		Name:       defaultDocumentName(payload.Name),
		CreatedAt:  timestamp,
		CreatedBy:  "agogo-web",
		ModifiedAt: timestamp,
		LayerRoot:  NewGroupLayer("Root"),
	}
}

// renderViewportWithCache renders the viewport using a cached background when
// the viewport/document inputs are unchanged. The background (checkerboard or
// solid fill) is the most expensive AGG pass; caching it and copying the buffer
// avoids re-rasterizing hundreds of rectangles every frame.
func (inst *instance) renderViewportWithCache(doc *Document, documentSurface []byte) []byte {
	vp := &inst.viewport
	key := viewportBaseKey{
		DocWidth:   doc.Width,
		DocHeight:  doc.Height,
		Background: doc.Background.Kind,
		CenterX:    vp.CenterX,
		CenterY:    vp.CenterY,
		Zoom:       clampZoom(vp.Zoom),
		Rotation:   vp.Rotation,
		CanvasW:    vp.CanvasW,
		CanvasH:    vp.CanvasH,
	}

	canvasSize := maxInt(vp.CanvasW, 1) * maxInt(vp.CanvasH, 1) * 4

	if key == inst.cachedViewportBaseKey && len(inst.cachedViewportBase) == canvasSize {
		// Cache hit: copy the pre-rendered background into the output buffer.
		if len(inst.pixels) != canvasSize {
			inst.pixels = make([]byte, canvasSize)
		}
		copy(inst.pixels, inst.cachedViewportBase)
	} else {
		// Cache miss: render through AGG and store a copy.
		inst.pixels = aggrender.RenderViewportBase(
			&aggrender.Document{
				Width:      doc.Width,
				Height:     doc.Height,
				Background: doc.Background.Kind,
			},
			&aggrender.Viewport{
				CenterX:  key.CenterX,
				CenterY:  key.CenterY,
				Zoom:     key.Zoom,
				Rotation: key.Rotation,
				CanvasW:  key.CanvasW,
				CanvasH:  key.CanvasH,
			},
			inst.pixels,
		)
		if len(inst.cachedViewportBase) != canvasSize {
			inst.cachedViewportBase = make([]byte, canvasSize)
		}
		copy(inst.cachedViewportBase, inst.pixels)
		inst.cachedViewportBaseKey = key
	}

	if len(documentSurface) > 0 {
		compositeDocumentToViewport(inst.pixels, maxInt(vp.CanvasW, 1), maxInt(vp.CanvasH, 1), doc, vp, documentSurface)
	}

	return aggrender.RenderViewportOverlays(
		&aggrender.Document{
			Width:      doc.Width,
			Height:     doc.Height,
			Background: doc.Background.Kind,
		},
		&aggrender.Viewport{
			CenterX:    vp.CenterX,
			CenterY:    vp.CenterY,
			Zoom:       clampZoom(vp.Zoom),
			Rotation:   vp.Rotation,
			CanvasW:    vp.CanvasW,
			CanvasH:    vp.CanvasH,
			ShowGuides: vp.ShowGuides,
		},
		inst.pixels,
	)
}

// RenderViewport renders the document shell and the current composited layer tree.
// documentSurface is the precomputed RGBA composite for the full document; pass nil
// to skip layer compositing (e.g. when there are no layers).
func RenderViewport(doc *Document, vp *ViewportState, reuse []byte, documentSurface []byte) []byte {
	reuse = aggrender.RenderViewportBase(
		&aggrender.Document{
			Width:      doc.Width,
			Height:     doc.Height,
			Background: doc.Background.Kind,
		},
		&aggrender.Viewport{
			CenterX:  vp.CenterX,
			CenterY:  vp.CenterY,
			Zoom:     clampZoom(vp.Zoom),
			Rotation: vp.Rotation,
			CanvasW:  vp.CanvasW,
			CanvasH:  vp.CanvasH,
		},
		reuse,
	)

	if len(documentSurface) > 0 {
		compositeDocumentToViewport(reuse, maxInt(vp.CanvasW, 1), maxInt(vp.CanvasH, 1), doc, vp, documentSurface)
	}

	return aggrender.RenderViewportOverlays(
		&aggrender.Document{
			Width:      doc.Width,
			Height:     doc.Height,
			Background: doc.Background.Kind,
		},
		&aggrender.Viewport{
			CenterX:    vp.CenterX,
			CenterY:    vp.CenterY,
			Zoom:       clampZoom(vp.Zoom),
			Rotation:   vp.Rotation,
			CanvasW:    vp.CanvasW,
			CanvasH:    vp.CanvasH,
			ShowGuides: vp.ShowGuides,
		},
		reuse,
	)
}

func cloneDocument(doc *Document) *Document {
	if doc == nil {
		return nil
	}
	copyDoc := *doc
	copyDoc.LayerRoot = cloneGroupLayer(doc.LayerRoot)
	copyDoc.Selection = cloneSelection(doc.Selection)
	copyDoc.LastSelection = cloneSelection(doc.LastSelection)
	return &copyDoc
}

func snapshotsEqual(a, b snapshot) bool {
	if a.DocumentID != b.DocumentID {
		return false
	}
	if a.Viewport != b.Viewport {
		return false
	}
	if (a.Document == nil) != (b.Document == nil) {
		return false
	}
	if a.Document == nil {
		return true
	}
	return documentsEqual(a.Document, b.Document)
}

func documentsEqual(a, b *Document) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	if a == nil {
		return true
	}
	if a.Width != b.Width || a.Height != b.Height || a.Resolution != b.Resolution || a.ColorMode != b.ColorMode {
		return false
	}
	if a.BitDepth != b.BitDepth || a.Background != b.Background || a.ID != b.ID || a.Name != b.Name {
		return false
	}
	if a.CreatedAt != b.CreatedAt || a.CreatedBy != b.CreatedBy || a.ModifiedAt != b.ModifiedAt {
		return false
	}
	if a.ActiveLayerID != b.ActiveLayerID {
		return false
	}
	if !selectionEqual(a.Selection, b.Selection) || !selectionEqual(a.LastSelection, b.LastSelection) {
		return false
	}
	return layerTreeEqual(a.LayerRoot, b.LayerRoot)
}

func screenDeltaToDocument(deltaX, deltaY, zoom, rotation float64) (float64, float64) {
	const degToRad = math.Pi / 180
	radians := rotation * degToRad
	cosTheta := math.Cos(radians)
	sinTheta := math.Sin(radians)
	return (deltaX*cosTheta + deltaY*sinTheta) / zoom,
		(-deltaX*sinTheta + deltaY*cosTheta) / zoom
}

func parseBackground(kind string) Background {
	switch kind {
	case "white":
		return Background{Kind: "white", Color: [4]uint8{244, 246, 250, 255}}
	case "color":
		return Background{Kind: "color", Color: [4]uint8{236, 147, 92, 255}}
	default:
		return Background{Kind: "transparent"}
	}
}

func defaultDocumentName(name string) string {
	if name == "" {
		return "Untitled"
	}
	return name
}

func decodePayload[T any](payloadJSON string, target *T) error {
	if payloadJSON == "" {
		return nil
	}
	if err := json.Unmarshal([]byte(payloadJSON), target); err != nil {
		return fmt.Errorf("decode payload: %w", err)
	}
	return nil
}

func clampZoom(zoom float64) float64 {
	if zoom <= 0 {
		return 1
	}
	if zoom < 0.05 {
		return 0.05
	}
	if zoom > 32 {
		return 32
	}
	return zoom
}

func normalizeRotation(rotation float64) float64 {
	normalized := math.Mod(rotation, 360)
	if normalized < 0 {
		normalized += 360
	}
	return normalized
}

func valueOrDefault(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}

func floatValueOrDefault(value, fallback float64) float64 {
	if value <= 0 {
		return fallback
	}
	return value
}

func stringValueOrDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
