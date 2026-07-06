# Phase 6.1: Vector Path System — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a complete vector path editing system — data model, pen tool, direct selection tool, boolean path operations, rasterization, and a Paths panel in the UI.

**Architecture:** The Go/Wasm engine owns all path state, editing logic, and overlay generation. The frontend renders path overlays (anchors, handles, rubber-band) from `UIMeta` JSON and forwards raw pointer events. Path ↔ AGG bridge converts our path model to AGG's `PathStorage` for rasterization and boolean ops.

**Tech Stack:** Go (engine), AGG (`agg_go`) for rasterization/booleans, React + TypeScript (frontend), `@agogo/proto` (shared command IDs + types)

---

## Task 1: Refactor Path Data Model (Path → Subpath, Compound Path)

Rename the current `Path` to `Subpath`, create a new compound `Path` type holding `[]Subpath`, and add `HandleType` to `PathPoint`. Update all references across the engine.

**Files:**
- Modify: `packages/engine-wasm/internal/engine/layers.go:100-113` (Path, PathPoint types)
- Modify: `packages/engine-wasm/internal/engine/layers.go:495-502` (clonePath)
- Modify: `packages/engine-wasm/internal/engine/layers.go:639-655` (pathEqual)
- Modify: `packages/engine-wasm/internal/engine/layers.go:82-83` (LayerNode interface — VectorMask)
- Modify: `packages/engine-wasm/internal/engine/layers.go:262-267` (layerBase VectorMask get/set)
- Modify: `packages/engine-wasm/internal/engine/layers.go:391-427` (VectorLayer struct + Clone)
- Modify: `packages/engine-wasm/internal/engine/layer_ops.go:11-27` (AddLayerPayload.Path)
- Modify: `packages/engine-wasm/internal/engine/layer_ops.go:620` (AddVectorMask — creates empty Path)
- Modify: `packages/engine-wasm/internal/engine/project_archive.go:44,55` (archive VectorMask/Shape fields)
- Modify: `packages/engine-wasm/internal/engine/project_archive.go:132,231,256` (archive clone/restore)
- Test: `packages/engine-wasm/internal/engine/layers_test.go`
- Test: `packages/engine-wasm/internal/engine/layer_ops_test.go`
- Test: `packages/engine-wasm/internal/engine/engine_state_and_utility_test.go`
- Test: `packages/engine-wasm/internal/engine/project_io_test.go`

**Step 1: Write tests for the new data model**

Add a test in `layers_test.go` that validates the new types:

```go
func TestSubpathAndCompoundPath(t *testing.T) {
	// A compound path with two subpaths (like the letter "O").
	outer := Subpath{
		Closed: true,
		Points: []PathPoint{
			{X: 0, Y: 0, HandleType: HandleCorner},
			{X: 100, Y: 0, HandleType: HandleCorner},
			{X: 100, Y: 100, HandleType: HandleCorner},
			{X: 0, Y: 100, HandleType: HandleCorner},
		},
	}
	inner := Subpath{
		Closed: true,
		Points: []PathPoint{
			{X: 20, Y: 20, HandleType: HandleCorner},
			{X: 80, Y: 20, HandleType: HandleCorner},
			{X: 80, Y: 80, HandleType: HandleCorner},
			{X: 20, Y: 80, HandleType: HandleCorner},
		},
	}
	path := &Path{Subpaths: []Subpath{outer, inner}}

	// Clone must deep-copy.
	cloned := clonePath(path)
	cloned.Subpaths[0].Points[0].X = 999
	if path.Subpaths[0].Points[0].X == 999 {
		t.Fatal("clonePath did not deep-copy subpath points")
	}

	// Equality.
	a := &Path{Subpaths: []Subpath{outer}}
	b := &Path{Subpaths: []Subpath{outer}}
	if !pathEqual(a, b) {
		t.Fatal("identical paths should be equal")
	}
	b.Subpaths[0].Points[0].HandleType = HandleSmooth
	if pathEqual(a, b) {
		t.Fatal("differing HandleType should make paths unequal")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd packages/engine-wasm && go test ./internal/engine/ -run TestSubpathAndCompoundPath -v`
Expected: FAIL — `Subpath` type does not exist yet.

**Step 3: Implement the data model refactor**

In `layers.go`, replace the existing types:

```go
// HandleType controls how control handles behave during editing.
type HandleType int

const (
	HandleCorner    HandleType = iota // Handles move independently
	HandleSmooth                      // Handles stay collinear, independent lengths
	HandleSymmetric                   // Handles collinear + equal length
)

type PathPoint struct {
	X          float64    `json:"x"`
	Y          float64    `json:"y"`
	InX        float64    `json:"inX,omitempty"`
	InY        float64    `json:"inY,omitempty"`
	OutX       float64    `json:"outX,omitempty"`
	OutY       float64    `json:"outY,omitempty"`
	HandleType HandleType `json:"handleType,omitempty"`
}

// Subpath is a sequence of anchor points, optionally closed.
type Subpath struct {
	Closed bool        `json:"closed"`
	Points []PathPoint `json:"points,omitempty"`
}

// Path is a compound path (multiple subpaths, e.g. donut = outer + inner circle).
type Path struct {
	Subpaths []Subpath `json:"subpaths"`
}
```

Update `clonePath`:

```go
func clonePath(path *Path) *Path {
	if path == nil {
		return nil
	}
	cp := &Path{Subpaths: make([]Subpath, len(path.Subpaths))}
	for i, sp := range path.Subpaths {
		cp.Subpaths[i] = Subpath{
			Closed: sp.Closed,
			Points: append([]PathPoint(nil), sp.Points...),
		}
	}
	return cp
}
```

Update `pathEqual`:

```go
func pathEqual(a, b *Path) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	if a == nil {
		return true
	}
	if len(a.Subpaths) != len(b.Subpaths) {
		return false
	}
	for i := range a.Subpaths {
		sa, sb := a.Subpaths[i], b.Subpaths[i]
		if sa.Closed != sb.Closed || len(sa.Points) != len(sb.Points) {
			return false
		}
		for j := range sa.Points {
			if sa.Points[j] != sb.Points[j] {
				return false
			}
		}
	}
	return true
}
```

Update `AddVectorMask` (in `layer_ops.go`) to create an empty compound path:

```go
layer.SetVectorMask(&Path{Subpaths: []Subpath{{Closed: true}}})
```

**Step 4: Fix all existing test call sites**

Every place that constructs `Path{Closed: true, Points: []PathPoint{...}}` must become `Path{Subpaths: []Subpath{{Closed: true, Points: []PathPoint{...}}}}`. Key locations:

- `layers_test.go`: `TestTextAndVectorLayerCloneDeepCopiesRasterState` — the VectorLayer and group vectorMask constructors
- `layer_ops_test.go`: `TestApplyLayerMaskSupportsTextAndVectorLayers`, vector layer merge tests
- `engine_state_and_utility_test.go`: `TestVectorMaskRendersWithoutError`
- `project_io_test.go`: All test VectorLayer/vectorMask construction

Also update `VectorLayer` comparison in `layersEqual` (`layers.go:596-606`) and the `projectLayerArchive` struct fields.

**Step 5: Run all tests to verify everything passes**

Run: `cd packages/engine-wasm && go test ./internal/engine/ -v -count=1`
Expected: ALL PASS

**Step 6: Commit**

```bash
git add packages/engine-wasm/internal/engine/layers.go \
       packages/engine-wasm/internal/engine/layers_test.go \
       packages/engine-wasm/internal/engine/layer_ops.go \
       packages/engine-wasm/internal/engine/layer_ops_test.go \
       packages/engine-wasm/internal/engine/project_archive.go \
       packages/engine-wasm/internal/engine/project_io_test.go \
       packages/engine-wasm/internal/engine/engine_state_and_utility_test.go
git commit -m "refactor: introduce Subpath and compound Path model for Phase 6.1"
```

---

## Task 2: Path ↔ AGG Bridge

Convert our `Path`/`Subpath` model to AGG's `PathStorage` for rasterization, and convert back from `PathStorage` to our model. This is the foundation for all rendering and boolean operations.

**Files:**
- Create: `packages/engine-wasm/internal/engine/path_agg.go`
- Create: `packages/engine-wasm/internal/engine/path_agg_test.go`

**Step 1: Write failing tests**

```go
func TestPathToAGGRoundTrip(t *testing.T) {
	// A simple triangle.
	triangle := &Path{Subpaths: []Subpath{{
		Closed: true,
		Points: []PathPoint{
			{X: 0, Y: 0, HandleType: HandleCorner},
			{X: 100, Y: 0, HandleType: HandleCorner},
			{X: 50, Y: 100, HandleType: HandleCorner},
		},
	}}}

	aggPath := pathToAGG(triangle)
	if aggPath.TotalVertices() == 0 {
		t.Fatal("expected AGG path to have vertices")
	}

	// Round-trip back.
	result := aggPathToPath(aggPath)
	if len(result.Subpaths) != 1 {
		t.Fatalf("expected 1 subpath, got %d", len(result.Subpaths))
	}
	if !result.Subpaths[0].Closed {
		t.Fatal("expected subpath to be closed")
	}
	if len(result.Subpaths[0].Points) != 3 {
		t.Fatalf("expected 3 points, got %d", len(result.Subpaths[0].Points))
	}
}

func TestPathToAGGWithCurves(t *testing.T) {
	// A single curved segment (smooth anchor with control handles).
	curved := &Path{Subpaths: []Subpath{{
		Closed: false,
		Points: []PathPoint{
			{X: 0, Y: 0, OutX: 30, OutY: 0, HandleType: HandleSmooth},
			{X: 100, Y: 100, InX: 70, InY: 100, HandleType: HandleSmooth},
		},
	}}}

	aggPath := pathToAGG(curved)
	// Should contain MoveTo + Curve4 commands.
	if aggPath.TotalVertices() < 2 {
		t.Fatal("expected AGG path to have curve vertices")
	}
}

func TestPathToAGGCompound(t *testing.T) {
	// Two subpaths — each starts with its own MoveTo.
	compound := &Path{Subpaths: []Subpath{
		{Closed: true, Points: []PathPoint{
			{X: 0, Y: 0}, {X: 10, Y: 0}, {X: 10, Y: 10},
		}},
		{Closed: true, Points: []PathPoint{
			{X: 20, Y: 20}, {X: 30, Y: 20}, {X: 30, Y: 30},
		}},
	}}

	aggPath := pathToAGG(compound)
	result := aggPathToPath(aggPath)
	if len(result.Subpaths) != 2 {
		t.Fatalf("expected 2 subpaths, got %d", len(result.Subpaths))
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd packages/engine-wasm && go test ./internal/engine/ -run TestPathToAGG -v`
Expected: FAIL — `pathToAGG` not defined.

**Step 3: Implement the bridge**

In `path_agg.go`:

```go
package engine

import (
	agglib "github.com/cwbudde/agg_go"
	"github.com/cwbudde/agg_go/internal/basics"
	"github.com/cwbudde/agg_go/internal/path"
)

// pathToAGG converts our Path model to an AGG PathStorage.
// Each Subpath becomes a MoveTo-started sub-sequence.
// Curved segments use Curve4 (cubic Bezier) when either anchor has control handles.
func pathToAGG(p *Path) *path.PathStorage {
	ps := path.NewPathStorage()
	if p == nil {
		return ps
	}
	for _, sp := range p.Subpaths {
		if len(sp.Points) == 0 {
			continue
		}
		ps.MoveTo(sp.Points[0].X, sp.Points[0].Y)
		for i := 1; i < len(sp.Points); i++ {
			prev := sp.Points[i-1]
			curr := sp.Points[i]
			if hasCurveHandles(prev, curr, true) {
				// Cubic Bezier: prev.Out → curr.In
				ps.Curve4(prev.OutX, prev.OutY, curr.InX, curr.InY, curr.X, curr.Y)
			} else {
				ps.LineTo(curr.X, curr.Y)
			}
		}
		// Close: segment from last point back to first.
		if sp.Closed && len(sp.Points) > 1 {
			last := sp.Points[len(sp.Points)-1]
			first := sp.Points[0]
			if hasCurveHandles(last, first, true) {
				ps.Curve4(last.OutX, last.OutY, first.InX, first.InY, first.X, first.Y)
			}
			ps.ClosePolygon()
		}
	}
	return ps
}

// hasCurveHandles returns true if the segment from prev to curr has non-trivial control handles.
// outgoing=true checks prev.Out and curr.In (forward direction).
func hasCurveHandles(prev, curr PathPoint, outgoing bool) bool {
	if outgoing {
		return (prev.OutX != prev.X || prev.OutY != prev.Y) ||
			(curr.InX != curr.X || curr.InY != curr.Y)
	}
	return false
}

// aggPathToPath converts an AGG PathStorage back to our compound Path model.
func aggPathToPath(ps *path.PathStorage) *Path {
	p := &Path{}
	var current *Subpath
	n := ps.TotalVertices()

	for i := 0; i < n; i++ {
		x, y := ps.Vertex(i)
		cmd := ps.Command(i)
		switch {
		case basics.IsMoveTo(cmd):
			p.Subpaths = append(p.Subpaths, Subpath{})
			current = &p.Subpaths[len(p.Subpaths)-1]
			current.Points = append(current.Points, PathPoint{
				X: x, Y: y, InX: x, InY: y, OutX: x, OutY: y,
				HandleType: HandleCorner,
			})
		case basics.IsVertex(cmd) && current != nil:
			// LineTo
			current.Points = append(current.Points, PathPoint{
				X: x, Y: y, InX: x, InY: y, OutX: x, OutY: y,
				HandleType: HandleCorner,
			})
		case basics.IsCurve(cmd) && current != nil:
			// Curve4: 2 control points + endpoint (3 vertices in AGG).
			// This is the control-point vertex; skip, handled by endpoint.
		case basics.IsEndPoly(cmd) && current != nil:
			if cmd&basics.PathFlagsClose != 0 {
				current.Closed = true
			}
		}
	}
	return p
}
```

> **Note:** The actual AGG API calls (`ps.MoveTo`, `ps.Curve4`, etc.) and command inspection (`basics.IsMoveTo`, etc.) depend on what's exported by `agg_go`. The implementer must check the actual exported API and adapt. The key files to reference are:
> - `../agg_go/internal/path/path_storage.go` — PathStorage methods
> - `../agg_go/internal/basics/path.go` — command constants and helpers
> - `../agg_go/public.go` or similar — public re-exports
>
> If `agg_go` doesn't export `internal/path` and `internal/basics` directly, use whatever public API wraps them (likely `agglib.NewPathStorage()` etc.).

**Step 4: Run tests to verify they pass**

Run: `cd packages/engine-wasm && go test ./internal/engine/ -run TestPathToAGG -v`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add packages/engine-wasm/internal/engine/path_agg.go \
       packages/engine-wasm/internal/engine/path_agg_test.go
git commit -m "feat: add Path↔AGG PathStorage bridge for vector path system"
```

---

## Task 3: Path Command IDs and Dispatch Skeleton

Register all new path-related command IDs in both Go and TypeScript, and add dispatch routing stubs that return "not implemented" errors.

**Files:**
- Modify: `packages/engine-wasm/internal/engine/engine.go` (add command constants)
- Create: `packages/engine-wasm/internal/engine/dispatch_path.go` (dispatch handlers)
- Create: `packages/engine-wasm/internal/engine/path_tool.go` (pen/direct-select state machine types)
- Modify: `packages/proto/src/commands.ts` (add TS command IDs + payload types)

**Step 1: Add Go command constants**

In `engine.go`, after the filter commands:

```go
// Phase 6.1: Vector Path
commandSetActiveTool          = 0x0600
commandPenToolClick           = 0x0601
commandPenToolClose           = 0x0602
commandDirectSelectMove       = 0x0603
commandDirectSelectMarquee    = 0x0604
commandBreakHandle            = 0x0605
commandDeleteAnchor           = 0x0606
commandAddAnchorOnSegment     = 0x0607
commandPathCombine            = 0x0610
commandPathSubtract           = 0x0611
commandPathIntersect          = 0x0612
commandPathExclude            = 0x0613
commandFlattenPath            = 0x0614
commandRasterizePath          = 0x0615
commandCreatePath             = 0x0620
commandDeletePath             = 0x0621
commandRenamePath             = 0x0622
commandDuplicatePath          = 0x0623
commandMakeSelectionFromPath  = 0x0624
commandStrokePath             = 0x0625
commandFillPath               = 0x0626
```

**Step 2: Add TypeScript command IDs and payload types**

In `packages/proto/src/commands.ts`, add inside the `CommandID` enum:

```typescript
// Phase 6.1: Vector Path
SetActiveTool = 0x0600,
PenToolClick = 0x0601,
PenToolClose = 0x0602,
DirectSelectMove = 0x0603,
DirectSelectMarquee = 0x0604,
BreakHandle = 0x0605,
DeleteAnchor = 0x0606,
AddAnchorOnSegment = 0x0607,
PathCombine = 0x0610,
PathSubtract = 0x0611,
PathIntersect = 0x0612,
PathExclude = 0x0613,
FlattenPath = 0x0614,
RasterizePath = 0x0615,
CreatePath = 0x0620,
DeletePath = 0x0621,
RenamePath = 0x0622,
DuplicatePath = 0x0623,
MakeSelectionFromPath = 0x0624,
StrokePath = 0x0625,
FillPath = 0x0626,
```

Add TypeScript payload interfaces:

```typescript
export type HandleType = "corner" | "smooth" | "symmetric";

export interface PathPointData {
  x: number;
  y: number;
  inX?: number;
  inY?: number;
  outX?: number;
  outY?: number;
  handleType?: HandleType;
}

export interface SubpathData {
  closed: boolean;
  points: PathPointData[];
}

export interface PathData {
  subpaths: SubpathData[];
}

export interface PenToolClickCommand {
  x: number;
  y: number;
  dragX?: number;  // If present, click+drag (pull handles)
  dragY?: number;
  shift?: boolean;
}

export interface DirectSelectMoveCommand {
  anchorIndex: number;
  subpathIndex: number;
  handleKind: "anchor" | "in" | "out";
  x: number;
  y: number;
}

export interface DirectSelectMarqueeCommand {
  x1: number;
  y1: number;
  x2: number;
  y2: number;
  shift?: boolean;
}

export interface BreakHandleCommand {
  subpathIndex: number;
  anchorIndex: number;
}

export interface DeleteAnchorCommand {
  subpathIndex: number;
  anchorIndices: number[];
}

export interface AddAnchorOnSegmentCommand {
  subpathIndex: number;
  segmentIndex: number;
  t: number;  // Parameter along the segment [0,1]
}

export interface PathBooleanCommand {
  pathIndex?: number;  // Which document path; omit for active path
}

export interface RasterizePathCommand {
  targetLayerId?: string;
  asSelection?: boolean;
}

export interface CreatePathCommand {
  name: string;
  path?: PathData;
}

export interface RenamePathCommand {
  pathIndex: number;
  name: string;
}

export interface MakeSelectionFromPathCommand {
  pathIndex?: number;
  featherRadius?: number;
  antiAlias?: boolean;
}

export interface StrokePathCommand {
  pathIndex?: number;
  toolWidth?: number;
  color?: [number, number, number, number];
}

export interface FillPathCommand {
  pathIndex?: number;
  color?: [number, number, number, number];
}

// Path overlay returned in UIMeta
export interface PathOverlayAnchor {
  x: number;
  y: number;
  selected: boolean;
  first: boolean;
}

export interface PathOverlayLine {
  x1: number;
  y1: number;
  x2: number;
  y2: number;
}

export interface PathOverlayPolyline {
  points: Array<{ x: number; y: number }>;
}

export interface PathOverlay {
  segments: PathOverlayPolyline[];
  anchors: PathOverlayAnchor[];
  handleLines: PathOverlayLine[];
  rubberBand?: PathOverlayPolyline;
}
```

Add `pathOverlay` to UIMeta in `packages/proto/src/responses.ts`:

```typescript
pathOverlay?: import("./commands.js").PathOverlay;
```

**Step 3: Create Go dispatch stub**

In `dispatch_path.go`:

```go
package engine

import "fmt"

func (inst *instance) dispatchPathCommand(cmdID int, raw json.RawMessage) (*RenderResult, error) {
	doc := inst.activeDocument()
	if doc == nil {
		return nil, fmt.Errorf("no active document")
	}
	switch cmdID {
	case commandPenToolClick:
		return nil, fmt.Errorf("pen tool click: not yet implemented")
	case commandPenToolClose:
		return nil, fmt.Errorf("pen tool close: not yet implemented")
	case commandDirectSelectMove:
		return nil, fmt.Errorf("direct select move: not yet implemented")
	case commandDirectSelectMarquee:
		return nil, fmt.Errorf("direct select marquee: not yet implemented")
	case commandBreakHandle:
		return nil, fmt.Errorf("break handle: not yet implemented")
	case commandDeleteAnchor:
		return nil, fmt.Errorf("delete anchor: not yet implemented")
	case commandAddAnchorOnSegment:
		return nil, fmt.Errorf("add anchor on segment: not yet implemented")
	case commandPathCombine, commandPathSubtract, commandPathIntersect, commandPathExclude:
		return nil, fmt.Errorf("path boolean: not yet implemented")
	case commandFlattenPath:
		return nil, fmt.Errorf("flatten path: not yet implemented")
	case commandRasterizePath:
		return nil, fmt.Errorf("rasterize path: not yet implemented")
	case commandCreatePath:
		return nil, fmt.Errorf("create path: not yet implemented")
	case commandDeletePath:
		return nil, fmt.Errorf("delete path: not yet implemented")
	case commandRenamePath:
		return nil, fmt.Errorf("rename path: not yet implemented")
	case commandDuplicatePath:
		return nil, fmt.Errorf("duplicate path: not yet implemented")
	case commandMakeSelectionFromPath:
		return nil, fmt.Errorf("make selection from path: not yet implemented")
	case commandStrokePath:
		return nil, fmt.Errorf("stroke path: not yet implemented")
	case commandFillPath:
		return nil, fmt.Errorf("fill path: not yet implemented")
	default:
		return nil, fmt.Errorf("unknown path command: 0x%04x", cmdID)
	}
}
```

Wire into the main dispatch in `dispatch_core.go` — add a range check that routes `0x0600–0x06FF` to `dispatchPathCommand`.

**Step 4: Create path tool state types**

In `path_tool.go`:

```go
package engine

// pathToolState holds the engine-side state for pen and direct selection tools.
type pathToolState struct {
	// activeTool is "pen", "direct-select", or "" (no path tool active).
	activeTool string
	// workPath is the path currently being drawn/edited.
	workPath *Path
	// activeSubpathIdx is the subpath being extended (pen tool).
	activeSubpathIdx int
	// selectedAnchors tracks which anchors are selected (direct select).
	// Key: subpathIdx*10000 + anchorIdx
	selectedAnchors map[int]bool
	// cursorDocX, cursorDocY are the last known cursor position in doc space.
	cursorDocX, cursorDocY float64
}

func newPathToolState() *pathToolState {
	return &pathToolState{
		selectedAnchors: make(map[int]bool),
	}
}
```

**Step 5: Run Go vet and tests**

Run: `cd packages/engine-wasm && go vet ./internal/engine/ && go test ./internal/engine/ -v -count=1`
Expected: ALL PASS (stubs don't break anything)

**Step 6: Commit**

```bash
git add packages/engine-wasm/internal/engine/engine.go \
       packages/engine-wasm/internal/engine/dispatch_path.go \
       packages/engine-wasm/internal/engine/dispatch_core.go \
       packages/engine-wasm/internal/engine/path_tool.go \
       packages/proto/src/commands.ts \
       packages/proto/src/responses.ts
git commit -m "feat: add path command IDs, dispatch skeleton, and tool state types"
```

---

## Task 4: Document Path Storage (Named Paths Collection)

The document needs a collection of named paths (like Photoshop's Paths panel). Add a `Paths` field to `Document`, with commands to create/delete/rename/duplicate paths.

**Files:**
- Modify: `packages/engine-wasm/internal/engine/engine.go:124-139` (Document struct)
- Create: `packages/engine-wasm/internal/engine/path_ops.go` (path CRUD operations)
- Create: `packages/engine-wasm/internal/engine/path_ops_test.go`
- Modify: `packages/engine-wasm/internal/engine/project_archive.go` (serialize paths)

**Step 1: Write failing tests**

```go
func TestDocumentPathCRUD(t *testing.T) {
	h := setupTestEngine(t)
	createTestDocument(t, h)

	// Create a path.
	result, err := DispatchCommand(h, commandCreatePath, mustJSON(t, CreatePathPayload{
		Name: "Shape 1",
	}))
	if err != nil {
		t.Fatalf("create path: %v", err)
	}
	if len(result.UIMeta.Paths) != 1 || result.UIMeta.Paths[0].Name != "Shape 1" {
		t.Fatalf("expected 1 path named 'Shape 1', got %+v", result.UIMeta.Paths)
	}

	// Rename.
	_, err = DispatchCommand(h, commandRenamePath, mustJSON(t, RenamePathPayload{
		PathIndex: 0,
		Name:      "Outline",
	}))
	if err != nil {
		t.Fatalf("rename path: %v", err)
	}

	// Duplicate.
	result, err = DispatchCommand(h, commandDuplicatePath, mustJSON(t, DuplicatePathPayload{
		PathIndex: 0,
	}))
	if err != nil {
		t.Fatalf("duplicate path: %v", err)
	}
	if len(result.UIMeta.Paths) != 2 {
		t.Fatalf("expected 2 paths after duplicate, got %d", len(result.UIMeta.Paths))
	}

	// Delete.
	result, err = DispatchCommand(h, commandDeletePath, mustJSON(t, DeletePathPayload{
		PathIndex: 1,
	}))
	if err != nil {
		t.Fatalf("delete path: %v", err)
	}
	if len(result.UIMeta.Paths) != 1 {
		t.Fatalf("expected 1 path after delete, got %d", len(result.UIMeta.Paths))
	}
}
```

**Step 2: Run tests — expect fail**

Run: `cd packages/engine-wasm && go test ./internal/engine/ -run TestDocumentPathCRUD -v`

**Step 3: Implement path storage on Document**

Add to `Document` struct:

```go
Paths         []NamedPath `json:"-"`
ActivePathIdx int         `json:"-"`
```

Types in `path_ops.go`:

```go
type NamedPath struct {
	Name string
	Path Path
}

type PathMeta struct {
	Name   string `json:"name"`
	Active bool   `json:"active"`
}
```

Add `Paths []PathMeta` to `UIMeta`. Implement `CreatePath`, `DeletePath`, `RenamePath`, `DuplicatePath` methods on `Document`. Wire up dispatch in `dispatch_path.go`. Add path serialization to `project_archive.go`.

**Step 4: Run tests — expect pass**

Run: `cd packages/engine-wasm && go test ./internal/engine/ -run TestDocumentPathCRUD -v`

**Step 5: Commit**

```bash
git add packages/engine-wasm/internal/engine/engine.go \
       packages/engine-wasm/internal/engine/path_ops.go \
       packages/engine-wasm/internal/engine/path_ops_test.go \
       packages/engine-wasm/internal/engine/dispatch_path.go \
       packages/engine-wasm/internal/engine/project_archive.go
git commit -m "feat: add named path collection with CRUD commands"
```

---

## Task 5: Pen Tool State Machine

The core pen tool: click to add corner anchors, click+drag for smooth anchors with control handles, click first anchor to close path. Returns overlay data in UIMeta.

**Files:**
- Modify: `packages/engine-wasm/internal/engine/path_tool.go` (pen tool logic)
- Modify: `packages/engine-wasm/internal/engine/dispatch_path.go` (wire up handlers)
- Create: `packages/engine-wasm/internal/engine/path_overlay.go` (overlay generation)
- Create: `packages/engine-wasm/internal/engine/path_tool_test.go`

**Step 1: Write failing tests**

```go
func TestPenToolAddCornerAnchors(t *testing.T) {
	h := setupTestEngine(t)
	createTestDocument(t, h)

	// Set active tool to pen.
	_, err := DispatchCommand(h, commandSetActiveTool, mustJSON(t, SetActiveToolPayload{Tool: "pen"}))
	if err != nil {
		t.Fatalf("set tool: %v", err)
	}

	// Click 3 points to create a triangle (corner anchors, no drag).
	points := [][2]float64{{10, 10}, {100, 10}, {50, 90}}
	for _, pt := range points {
		_, err := DispatchCommand(h, commandPenToolClick, mustJSON(t, PenToolClickPayload{
			X: pt[0], Y: pt[1],
		}))
		if err != nil {
			t.Fatalf("pen click at (%.0f, %.0f): %v", pt[0], pt[1], err)
		}
	}

	// Close the path.
	result, err := DispatchCommand(h, commandPenToolClose, nil)
	if err != nil {
		t.Fatalf("pen close: %v", err)
	}

	// Should have 1 path with 1 closed subpath and 3 points.
	if len(result.UIMeta.Paths) != 1 {
		t.Fatalf("expected 1 path, got %d", len(result.UIMeta.Paths))
	}
}

func TestPenToolSmoothAnchor(t *testing.T) {
	h := setupTestEngine(t)
	createTestDocument(t, h)

	_, _ = DispatchCommand(h, commandSetActiveTool, mustJSON(t, SetActiveToolPayload{Tool: "pen"}))

	// Click+drag: creates smooth anchor with control handles.
	_, err := DispatchCommand(h, commandPenToolClick, mustJSON(t, PenToolClickPayload{
		X: 50, Y: 50, DragX: floatPtr(80), DragY: floatPtr(50),
	}))
	if err != nil {
		t.Fatalf("pen click+drag: %v", err)
	}

	// The anchor should have symmetric handles.
	doc := getActiveDocument(h)
	if len(doc.Paths) == 0 || len(doc.Paths[0].Path.Subpaths) == 0 {
		t.Fatal("expected work path with a subpath")
	}
	pt := doc.Paths[0].Path.Subpaths[0].Points[0]
	if pt.HandleType != HandleSmooth {
		t.Fatalf("expected HandleSmooth, got %d", pt.HandleType)
	}
	if pt.OutX == pt.X && pt.OutY == pt.Y {
		t.Fatal("expected non-zero outgoing handle after drag")
	}
}

func TestPenToolOverlay(t *testing.T) {
	h := setupTestEngine(t)
	createTestDocument(t, h)

	_, _ = DispatchCommand(h, commandSetActiveTool, mustJSON(t, SetActiveToolPayload{Tool: "pen"}))
	result, _ := DispatchCommand(h, commandPenToolClick, mustJSON(t, PenToolClickPayload{
		X: 10, Y: 10,
	}))

	// Overlay should show the anchor.
	if result.UIMeta.PathOverlay == nil {
		t.Fatal("expected path overlay in UIMeta")
	}
	if len(result.UIMeta.PathOverlay.Anchors) != 1 {
		t.Fatalf("expected 1 anchor in overlay, got %d", len(result.UIMeta.PathOverlay.Anchors))
	}
}
```

Helper: `func floatPtr(f float64) *float64 { return &f }`

**Step 2: Run tests — expect fail**

Run: `cd packages/engine-wasm && go test ./internal/engine/ -run TestPenTool -v`

**Step 3: Implement pen tool**

In `path_tool.go`, implement:

- `SetActiveToolPayload{Tool string}` — sets `pathToolState.activeTool`
- `PenToolClickPayload{X, Y, DragX, DragY *float64, Shift bool}`
- On click: create work path if needed, append anchor to active subpath
  - No drag: `HandleCorner`, handles = anchor position
  - With drag: `HandleSmooth`, `OutX/OutY` = drag position, `InX/InY` = mirror of drag
- `PenToolClose`: set `Closed = true` on active subpath, clear active subpath index

In `path_overlay.go`, implement:

- `buildPathOverlay(state *pathToolState, viewport ViewportState) *PathOverlay`
  - Transform doc-space anchors to viewport coordinates
  - Generate polyline segments by evaluating Bezier curves
  - Mark first anchor with `First: true` for close indicator

Add `PathOverlay *PathOverlay` to `UIMeta` struct. Populate it in the render result when a path tool is active.

**Step 4: Run tests — expect pass**

Run: `cd packages/engine-wasm && go test ./internal/engine/ -run TestPenTool -v`

**Step 5: Commit**

```bash
git add packages/engine-wasm/internal/engine/path_tool.go \
       packages/engine-wasm/internal/engine/path_tool_test.go \
       packages/engine-wasm/internal/engine/path_overlay.go \
       packages/engine-wasm/internal/engine/dispatch_path.go \
       packages/engine-wasm/internal/engine/engine.go
git commit -m "feat: implement pen tool state machine with overlay generation"
```

---

## Task 6: Direct Selection Tool

Select, move, and edit anchors and control handles. Alt+click to break smooth handles to corner. Marquee selection of multiple anchors.

**Files:**
- Modify: `packages/engine-wasm/internal/engine/path_tool.go`
- Create: `packages/engine-wasm/internal/engine/path_direct_select_test.go`

**Step 1: Write failing tests**

```go
func TestDirectSelectMoveAnchor(t *testing.T) {
	h := setupTestEngine(t)
	createTestDocument(t, h)

	// Create a triangle path via pen tool.
	_, _ = DispatchCommand(h, commandSetActiveTool, mustJSON(t, SetActiveToolPayload{Tool: "pen"}))
	for _, pt := range [][2]float64{{10, 10}, {100, 10}, {50, 90}} {
		_, _ = DispatchCommand(h, commandPenToolClick, mustJSON(t, PenToolClickPayload{X: pt[0], Y: pt[1]}))
	}
	_, _ = DispatchCommand(h, commandPenToolClose, nil)

	// Switch to direct select.
	_, _ = DispatchCommand(h, commandSetActiveTool, mustJSON(t, SetActiveToolPayload{Tool: "direct-select"}))

	// Move anchor 1 from (100,10) to (120,20).
	result, err := DispatchCommand(h, commandDirectSelectMove, mustJSON(t, DirectSelectMovePayload{
		SubpathIndex: 0, AnchorIndex: 1, HandleKind: "anchor",
		X: 120, Y: 20,
	}))
	if err != nil {
		t.Fatalf("direct select move: %v", err)
	}

	doc := getActiveDocument(h)
	pt := doc.Paths[0].Path.Subpaths[0].Points[1]
	if pt.X != 120 || pt.Y != 20 {
		t.Fatalf("expected anchor at (120,20), got (%.1f,%.1f)", pt.X, pt.Y)
	}
	_ = result
}

func TestDirectSelectMoveHandle(t *testing.T) {
	h := setupTestEngine(t)
	createTestDocument(t, h)

	// Create a path with a smooth anchor.
	_, _ = DispatchCommand(h, commandSetActiveTool, mustJSON(t, SetActiveToolPayload{Tool: "pen"}))
	_, _ = DispatchCommand(h, commandPenToolClick, mustJSON(t, PenToolClickPayload{
		X: 0, Y: 0, DragX: floatPtr(30), DragY: floatPtr(0),
	}))
	_, _ = DispatchCommand(h, commandPenToolClick, mustJSON(t, PenToolClickPayload{X: 100, Y: 100}))
	_, _ = DispatchCommand(h, commandPenToolClose, nil)

	// Switch to direct select, drag outgoing handle of anchor 0.
	_, _ = DispatchCommand(h, commandSetActiveTool, mustJSON(t, SetActiveToolPayload{Tool: "direct-select"}))
	_, err := DispatchCommand(h, commandDirectSelectMove, mustJSON(t, DirectSelectMovePayload{
		SubpathIndex: 0, AnchorIndex: 0, HandleKind: "out",
		X: 50, Y: 0,
	}))
	if err != nil {
		t.Fatalf("move handle: %v", err)
	}

	doc := getActiveDocument(h)
	pt := doc.Paths[0].Path.Subpaths[0].Points[0]
	if pt.OutX != 50 || pt.OutY != 0 {
		t.Fatalf("expected OutX=50, got %.1f", pt.OutX)
	}
	// Smooth: incoming handle should mirror.
	if pt.HandleType != HandleSmooth {
		t.Fatalf("expected smooth, got %d", pt.HandleType)
	}
}

func TestBreakHandleToCorner(t *testing.T) {
	h := setupTestEngine(t)
	createTestDocument(t, h)

	_, _ = DispatchCommand(h, commandSetActiveTool, mustJSON(t, SetActiveToolPayload{Tool: "pen"}))
	_, _ = DispatchCommand(h, commandPenToolClick, mustJSON(t, PenToolClickPayload{
		X: 50, Y: 50, DragX: floatPtr(80), DragY: floatPtr(50),
	}))
	_, _ = DispatchCommand(h, commandPenToolClick, mustJSON(t, PenToolClickPayload{X: 100, Y: 100}))
	_, _ = DispatchCommand(h, commandPenToolClose, nil)

	// Break handle → corner.
	_, err := DispatchCommand(h, commandBreakHandle, mustJSON(t, BreakHandlePayload{
		SubpathIndex: 0, AnchorIndex: 0,
	}))
	if err != nil {
		t.Fatalf("break handle: %v", err)
	}

	doc := getActiveDocument(h)
	pt := doc.Paths[0].Path.Subpaths[0].Points[0]
	if pt.HandleType != HandleCorner {
		t.Fatalf("expected corner after break, got %d", pt.HandleType)
	}
}
```

**Step 2: Run tests — expect fail**

Run: `cd packages/engine-wasm && go test ./internal/engine/ -run 'TestDirectSelect|TestBreakHandle' -v`

**Step 3: Implement direct selection**

In `path_tool.go`:

- `DirectSelectMovePayload{SubpathIndex, AnchorIndex int, HandleKind string, X, Y float64}`
  - `"anchor"`: move anchor + both handles by delta
  - `"in"`: move incoming handle; if smooth, mirror outgoing; if corner, move independently
  - `"out"`: move outgoing handle; if smooth, mirror incoming; if corner, move independently
- `BreakHandlePayload{SubpathIndex, AnchorIndex int}`: set `HandleType = HandleCorner`
- `DirectSelectMarqueePayload{X1, Y1, X2, Y2 float64, Shift bool}`: select all anchors within rect
- `DeleteAnchorPayload{SubpathIndex int, AnchorIndices []int}`: remove anchors, reconnect path

Handle smooth mirroring: when moving "out" handle on a smooth anchor, compute the unit vector from anchor to the new out position, then set the in handle to the opposite direction at the original in-handle length (and vice versa).

**Step 4: Run tests — expect pass**

**Step 5: Commit**

```bash
git add packages/engine-wasm/internal/engine/path_tool.go \
       packages/engine-wasm/internal/engine/path_direct_select_test.go \
       packages/engine-wasm/internal/engine/dispatch_path.go
git commit -m "feat: implement direct selection tool with handle editing"
```

---

## Task 7: Path Boolean Operations

Wire up AGG's `ConvGPC` for union, subtract, intersect, exclude on compound paths.

**Files:**
- Create: `packages/engine-wasm/internal/engine/path_boolean.go`
- Create: `packages/engine-wasm/internal/engine/path_boolean_test.go`
- Modify: `packages/engine-wasm/internal/engine/dispatch_path.go`

**Step 1: Write failing tests**

```go
func TestPathCombineUnion(t *testing.T) {
	// Two overlapping rectangles → union should produce a single compound path.
	h := setupTestEngine(t)
	createTestDocument(t, h)

	// Create two paths.
	rect1 := Path{Subpaths: []Subpath{{Closed: true, Points: []PathPoint{
		{X: 0, Y: 0}, {X: 50, Y: 0}, {X: 50, Y: 50}, {X: 0, Y: 50},
	}}}}
	rect2 := Path{Subpaths: []Subpath{{Closed: true, Points: []PathPoint{
		{X: 25, Y: 25}, {X: 75, Y: 25}, {X: 75, Y: 75}, {X: 25, Y: 75},
	}}}}

	result := pathBoolean(&rect1, &rect2, PathBoolUnion)
	if result == nil || len(result.Subpaths) == 0 {
		t.Fatal("expected non-empty union result")
	}
}

func TestPathSubtract(t *testing.T) {
	rect1 := Path{Subpaths: []Subpath{{Closed: true, Points: []PathPoint{
		{X: 0, Y: 0}, {X: 100, Y: 0}, {X: 100, Y: 100}, {X: 0, Y: 100},
	}}}}
	rect2 := Path{Subpaths: []Subpath{{Closed: true, Points: []PathPoint{
		{X: 25, Y: 25}, {X: 75, Y: 25}, {X: 75, Y: 75}, {X: 25, Y: 75},
	}}}}

	result := pathBoolean(&rect1, &rect2, PathBoolSubtract)
	if result == nil || len(result.Subpaths) == 0 {
		t.Fatal("expected non-empty subtract result (donut shape)")
	}
}
```

**Step 2: Run — expect fail**

**Step 3: Implement**

In `path_boolean.go`:

```go
type PathBoolOp int

const (
	PathBoolUnion     PathBoolOp = iota
	PathBoolSubtract
	PathBoolIntersect
	PathBoolExclude
)

func pathBoolean(a, b *Path, op PathBoolOp) *Path {
	aggA := pathToAGG(a)
	aggB := pathToAGG(b)
	// Use ConvGPC to compute boolean result.
	// Map PathBoolOp to GPC operation enum.
	// Convert result back to our Path model.
	...
}
```

> **Note:** The implementer must check how `ConvGPC` is exposed in `agg_go`. Reference: `../agg_go/internal/conv/gpc.go`. If it operates on VertexSources, flatten our paths through `ConvCurve` first (GPC needs straight-line polygons).

Wire dispatch: `commandPathCombine` → `pathBoolean(..., PathBoolUnion)`, etc.

**Step 4: Run — expect pass**

**Step 5: Commit**

```bash
git add packages/engine-wasm/internal/engine/path_boolean.go \
       packages/engine-wasm/internal/engine/path_boolean_test.go \
       packages/engine-wasm/internal/engine/dispatch_path.go
git commit -m "feat: add path boolean operations via AGG ConvGPC"
```

---

## Task 8: Rasterize Path (Path → Mask / Selection / Fill / Stroke)

Render a path to an alpha mask using AGG's scanline rasterizer, for "Make Selection from Path", "Fill Path", "Stroke Path", and "Rasterize Layer".

**Files:**
- Create: `packages/engine-wasm/internal/engine/path_rasterize.go`
- Create: `packages/engine-wasm/internal/engine/path_rasterize_test.go`
- Modify: `packages/engine-wasm/internal/engine/dispatch_path.go`

**Step 1: Write failing tests**

```go
func TestRasterizePathToMask(t *testing.T) {
	// Rasterize a 100×100 square path into a 100×100 alpha mask.
	square := &Path{Subpaths: []Subpath{{Closed: true, Points: []PathPoint{
		{X: 10, Y: 10}, {X: 90, Y: 10}, {X: 90, Y: 90}, {X: 10, Y: 90},
	}}}}

	mask, err := rasterizePathToMask(square, 100, 100, 0, false)
	if err != nil {
		t.Fatalf("rasterize: %v", err)
	}
	if len(mask) != 100*100 {
		t.Fatalf("expected 10000 bytes, got %d", len(mask))
	}
	// Center pixel should be opaque (inside the square).
	if mask[50*100+50] == 0 {
		t.Fatal("expected center pixel to be opaque")
	}
	// Corner pixel (0,0) should be transparent (outside the square).
	if mask[0] != 0 {
		t.Fatal("expected corner pixel to be transparent")
	}
}

func TestMakeSelectionFromPath(t *testing.T) {
	h := setupTestEngine(t)
	createTestDocument(t, h)

	// Create a path via pen tool.
	_, _ = DispatchCommand(h, commandSetActiveTool, mustJSON(t, SetActiveToolPayload{Tool: "pen"}))
	for _, pt := range [][2]float64{{10, 10}, {90, 10}, {90, 90}, {10, 90}} {
		_, _ = DispatchCommand(h, commandPenToolClick, mustJSON(t, PenToolClickPayload{X: pt[0], Y: pt[1]}))
	}
	_, _ = DispatchCommand(h, commandPenToolClose, nil)

	// Make selection from path.
	result, err := DispatchCommand(h, commandMakeSelectionFromPath, mustJSON(t, MakeSelectionFromPathPayload{
		AntiAlias: true,
	}))
	if err != nil {
		t.Fatalf("make selection: %v", err)
	}
	if !result.UIMeta.Selection.HasSelection {
		t.Fatal("expected selection to be active")
	}
}
```

**Step 2: Run — expect fail**

**Step 3: Implement**

In `path_rasterize.go`:

```go
// rasterizePathToMask renders a Path into an 8-bit alpha mask using AGG's scanline rasterizer.
func rasterizePathToMask(p *Path, width, height int, featherRadius float64, antiAlias bool) ([]byte, error) {
	aggPath := pathToAGG(p)
	// Flatten curves for the rasterizer.
	// Feed into AGG's ScanlineRasterizer → ScanlinePacked → render to alpha buffer.
	// Use the engine's existing agglib.Agg2D or direct rasterizer access.
	...
}
```

Also implement:
- `MakeSelectionFromPath`: rasterize → set document Selection
- `FillPath`: rasterize mask → composite fill color onto active layer
- `StrokePath`: convert path to stroked outline via `ConvStroke`, rasterize, composite

**Step 4: Run — expect pass**

**Step 5: Commit**

```bash
git add packages/engine-wasm/internal/engine/path_rasterize.go \
       packages/engine-wasm/internal/engine/path_rasterize_test.go \
       packages/engine-wasm/internal/engine/dispatch_path.go
git commit -m "feat: rasterize paths to masks and selections via AGG scanline rasterizer"
```

---

## Task 9: Frontend — Path Overlay Renderer

Render the `PathOverlay` data from UIMeta as SVG elements on top of the canvas. This shows anchor points, control handles, path segments, and the rubber-band preview.

**Files:**
- Create: `apps/editor-web/src/components/path-overlay.tsx`
- Modify: `apps/editor-web/src/components/editor-canvas.tsx` (mount overlay)
- Modify: `apps/editor-web/src/App.tsx` (add pen/direct-select to tool list)

**Step 1: Create the overlay component**

In `path-overlay.tsx`:

```tsx
import type { PathOverlay } from "@agogo/proto";

interface PathOverlayRendererProps {
  overlay: PathOverlay;
  canvasWidth: number;
  canvasHeight: number;
}

export function PathOverlayRenderer({ overlay, canvasWidth, canvasHeight }: PathOverlayRendererProps) {
  return (
    <svg
      width={canvasWidth}
      height={canvasHeight}
      style={{ position: "absolute", top: 0, left: 0, pointerEvents: "none" }}
    >
      {/* Path segments */}
      {overlay.segments.map((seg, i) => (
        <polyline
          key={`seg-${i}`}
          points={seg.points.map((p) => `${p.x},${p.y}`).join(" ")}
          fill="none"
          stroke="#00a8ff"
          strokeWidth={1}
        />
      ))}
      {/* Handle lines */}
      {overlay.handleLines.map((line, i) => (
        <line
          key={`hl-${i}`}
          x1={line.x1} y1={line.y1} x2={line.x2} y2={line.y2}
          stroke="#888"
          strokeWidth={1}
        />
      ))}
      {/* Rubber band */}
      {overlay.rubberBand && (
        <polyline
          points={overlay.rubberBand.points.map((p) => `${p.x},${p.y}`).join(" ")}
          fill="none"
          stroke="#00a8ff"
          strokeWidth={1}
          strokeDasharray="4 4"
        />
      )}
      {/* Anchors */}
      {overlay.anchors.map((a, i) => (
        <rect
          key={`a-${i}`}
          x={a.x - 3} y={a.y - 3} width={6} height={6}
          fill={a.selected ? "#00a8ff" : "white"}
          stroke="#333"
          strokeWidth={1}
        />
      ))}
    </svg>
  );
}
```

**Step 2: Mount in editor-canvas**

In `editor-canvas.tsx`, after the existing canvas element and inside the same positioning wrapper, conditionally render:

```tsx
{render?.uiMeta.pathOverlay && (
  <PathOverlayRenderer
    overlay={render.uiMeta.pathOverlay}
    canvasWidth={canvasWidth}
    canvasHeight={canvasHeight}
  />
)}
```

**Step 3: Add tools to App.tsx toolbar**

Add `"pen"` and `"direct-select"` to the tool definitions. Wire keyboard shortcuts: `P` for pen, `A` for direct select.

**Step 4: Run typecheck**

Run: `cd apps/editor-web && bun run typecheck`
Expected: PASS

**Step 5: Commit**

```bash
git add apps/editor-web/src/components/path-overlay.tsx \
       apps/editor-web/src/components/editor-canvas.tsx \
       apps/editor-web/src/App.tsx
git commit -m "feat: add path overlay renderer and pen/direct-select tool switching"
```

---

## Task 10: Frontend — Paths Panel

A panel listing named paths with create/delete/duplicate/rename and context menu actions (stroke, fill, make selection).

**Files:**
- Create: `apps/editor-web/src/components/paths-panel.tsx`
- Modify: `apps/editor-web/src/App.tsx` (add panel to sidebar/tab system)

**Step 1: Create the panel**

In `paths-panel.tsx`, build a panel that:

- Reads `render.uiMeta.paths` (array of `PathMeta`)
- Renders a list of named paths with active-path highlighting
- Click to select active path
- Double-click to rename (inline edit)
- Footer buttons: New Path, Duplicate, Delete, Make Selection, Stroke Path, Fill Path
- Each button dispatches the corresponding command via the engine handle

Follow the same patterns as `layers-panel.tsx` for list rendering, context menus, and engine dispatch.

**Step 2: Add to the panel tab system**

In `App.tsx`, add a "Paths" tab alongside the existing panels (Layers, Adjustments, etc.). Show it when the user switches to the Paths tab.

**Step 3: Run typecheck and lint**

Run: `cd apps/editor-web && bun run typecheck && bun run lint`
Expected: PASS

**Step 4: Commit**

```bash
git add apps/editor-web/src/components/paths-panel.tsx \
       apps/editor-web/src/App.tsx
git commit -m "feat: add Paths panel with CRUD and path operations"
```

---

## Task 11: Integration Test & Polish

End-to-end test: create a document, draw a path with the pen tool, edit with direct selection, boolean combine two paths, make a selection from path, verify the full round trip.

**Files:**
- Create: `packages/engine-wasm/internal/engine/path_integration_test.go`

**Step 1: Write integration test**

```go
func TestPathFullWorkflow(t *testing.T) {
	h := setupTestEngine(t)
	createTestDocument(t, h)

	// 1. Pen tool: draw a rectangle path.
	_, _ = DispatchCommand(h, commandSetActiveTool, mustJSON(t, SetActiveToolPayload{Tool: "pen"}))
	for _, pt := range [][2]float64{{10, 10}, {90, 10}, {90, 90}, {10, 90}} {
		_, _ = DispatchCommand(h, commandPenToolClick, mustJSON(t, PenToolClickPayload{X: pt[0], Y: pt[1]}))
	}
	_, _ = DispatchCommand(h, commandPenToolClose, nil)

	// 2. Direct select: move an anchor.
	_, _ = DispatchCommand(h, commandSetActiveTool, mustJSON(t, SetActiveToolPayload{Tool: "direct-select"}))
	_, err := DispatchCommand(h, commandDirectSelectMove, mustJSON(t, DirectSelectMovePayload{
		SubpathIndex: 0, AnchorIndex: 2, HandleKind: "anchor", X: 95, Y: 95,
	}))
	if err != nil {
		t.Fatalf("move anchor: %v", err)
	}

	// 3. Create a second path.
	_, _ = DispatchCommand(h, commandSetActiveTool, mustJSON(t, SetActiveToolPayload{Tool: "pen"}))
	_, _ = DispatchCommand(h, commandCreatePath, mustJSON(t, CreatePathPayload{Name: "Circle"}))
	for _, pt := range [][2]float64{{40, 40}, {60, 40}, {60, 60}, {40, 60}} {
		_, _ = DispatchCommand(h, commandPenToolClick, mustJSON(t, PenToolClickPayload{X: pt[0], Y: pt[1]}))
	}
	_, _ = DispatchCommand(h, commandPenToolClose, nil)

	// 4. Make selection from first path.
	result, err := DispatchCommand(h, commandMakeSelectionFromPath, mustJSON(t, MakeSelectionFromPathPayload{
		PathIndex: intPtr(0), AntiAlias: true,
	}))
	if err != nil {
		t.Fatalf("make selection: %v", err)
	}
	if !result.UIMeta.Selection.HasSelection {
		t.Fatal("expected active selection")
	}

	// 5. Undo should revert selection.
	result, _ = DispatchCommand(h, commandUndo, nil)
	if result.UIMeta.Selection.HasSelection {
		t.Fatal("expected no selection after undo")
	}
}
```

**Step 2: Run full test suite**

Run: `cd packages/engine-wasm && go test ./internal/engine/ -v -count=1`
Expected: ALL PASS

**Step 3: Run lint and format**

Run: `just lint && just fmt`

**Step 4: Commit**

```bash
git add packages/engine-wasm/internal/engine/path_integration_test.go
git commit -m "test: add end-to-end integration test for vector path workflow"
```

---

## Summary of Files

**New files:**
- `packages/engine-wasm/internal/engine/path_agg.go` — Path ↔ AGG bridge
- `packages/engine-wasm/internal/engine/path_agg_test.go`
- `packages/engine-wasm/internal/engine/path_tool.go` — Pen + direct selection state machine
- `packages/engine-wasm/internal/engine/path_tool_test.go`
- `packages/engine-wasm/internal/engine/path_direct_select_test.go`
- `packages/engine-wasm/internal/engine/path_overlay.go` — Overlay generation for UIMeta
- `packages/engine-wasm/internal/engine/path_ops.go` — Named path CRUD
- `packages/engine-wasm/internal/engine/path_ops_test.go`
- `packages/engine-wasm/internal/engine/path_boolean.go` — Boolean ops via ConvGPC
- `packages/engine-wasm/internal/engine/path_boolean_test.go`
- `packages/engine-wasm/internal/engine/path_rasterize.go` — Path → alpha mask
- `packages/engine-wasm/internal/engine/path_rasterize_test.go`
- `packages/engine-wasm/internal/engine/path_integration_test.go`
- `packages/engine-wasm/internal/engine/dispatch_path.go` — Command dispatch
- `apps/editor-web/src/components/path-overlay.tsx`
- `apps/editor-web/src/components/paths-panel.tsx`

**Modified files:**
- `packages/engine-wasm/internal/engine/layers.go` — Path/Subpath refactor
- `packages/engine-wasm/internal/engine/layers_test.go`
- `packages/engine-wasm/internal/engine/layer_ops.go`
- `packages/engine-wasm/internal/engine/layer_ops_test.go`
- `packages/engine-wasm/internal/engine/engine.go` — Command IDs, UIMeta
- `packages/engine-wasm/internal/engine/engine_state_and_utility_test.go`
- `packages/engine-wasm/internal/engine/dispatch_core.go` — Route path commands
- `packages/engine-wasm/internal/engine/project_archive.go` — Serialize paths
- `packages/engine-wasm/internal/engine/project_io_test.go`
- `packages/proto/src/commands.ts` — Command IDs + payload types
- `packages/proto/src/responses.ts` — PathOverlay in UIMeta
- `apps/editor-web/src/components/editor-canvas.tsx` — Mount overlay
- `apps/editor-web/src/App.tsx` — Tools + Paths panel
