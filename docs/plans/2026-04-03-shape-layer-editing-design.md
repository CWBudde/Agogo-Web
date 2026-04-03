# Shape Layer Editing Design

**Date:** 2026-04-03  
**Scope:** Phase 6.2 — shape layer editing (path edit mode + fill/stroke properties)  
**Out of scope:** Layer-level boolean path operations (deferred; Paths panel already covers this)

## Overview

Two independent features:

1. **Path editing mode** — double-click a VectorLayer to edit its shape via the existing direct-select infrastructure
2. **Fill/stroke properties** — change fill color, stroke color, and stroke width on a VectorLayer without rasterizing from scratch

---

## Feature 1: Path Editing Mode

### New engine commands

**`EnterVectorEditMode` (0x0631)**  
Payload: `{ layerID: string }`

1. Copies `VectorLayer.Shape` into `doc.Paths[0]` (Work Path slot, replacing any existing entry)
2. Sets `doc.ActivePathIdx = 0` and `doc.EditingVectorLayerID = layerID`
3. Sets `pathTool.activeTool = "direct-select"`

**`CommitVectorEdit` (0x0632)**  
Payload: `{}` (empty)

1. Reads `doc.Paths[doc.ActivePathIdx]`
2. Finds the VectorLayer via `doc.EditingVectorLayerID`
3. Updates `VectorLayer.Shape = editedPath`
4. Re-rasterizes via `rasterizeVectorShape` → updates `VectorLayer.CachedRaster`
5. Clears `doc.EditingVectorLayerID = ""`

### UIMeta change

`UIMeta.editingVectorLayerID string` — empty string when not in edit mode.

### Frontend behaviour

- Double-clicking a VectorLayer thumbnail (layers panel) dispatches `EnterVectorEditMode` and switches active tool to `"directSelect"`
- When the active tool changes away from `"pen"` / `"directSelect"`, the frontend checks `editingVectorLayerID` and dispatches `CommitVectorEdit` if set
- A dismissible banner ("Editing shape path — switch tool to commit") is shown while `editingVectorLayerID` is non-empty

---

## Feature 2: Fill/Stroke Properties Panel

### New engine command

**`SetVectorLayerStyle` (0x0633)**  
Payload: `{ layerID: string, fillColor: [r,g,b,a], strokeColor: [r,g,b,a], strokeWidth: number }`

1. Finds VectorLayer, updates `FillColor`, `StrokeColor`, `StrokeWidth`
2. Calls `rasterizeVectorShape` → updates `CachedRaster`
3. Records undo entry "Set shape style"

### LayerNodeMeta change

Three new optional fields (only populated for `layerType === "vector"`):
```
fillColor?:   [number, number, number, number]
strokeColor?: [number, number, number, number]
strokeWidth?: number
```

### Frontend — VectorPropertiesPanel

A new component rendered in the Properties dock when `activeLayer.layerType === "vector"`:

- Fill color swatch (click → set to current foreground color; separate "None" toggle sets alpha to 0)
- Stroke color swatch + width number input (0 = no stroke)
- Changes dispatch `SetVectorLayerStyle` on blur/enter — live update, no separate "Apply"
- "Edit Path" button dispatches `EnterVectorEditMode` (identical to double-click)

`AdjPropertiesPanel` gains a branch: if active layer type is `"vector"`, render `VectorPropertiesPanel`; otherwise existing logic.

---

## File changes summary

| File | Change |
|------|--------|
| `packages/proto/src/commands.ts` | Add `EnterVectorEditMode`, `CommitVectorEdit`, `SetVectorLayerStyle` CmdIDs + payload types |
| `packages/engine-wasm/internal/engine/engine.go` | Add 3 command constants, route to dispatcher |
| `packages/engine-wasm/internal/engine/dispatch_shape.go` | Add handlers for the 3 new commands |
| `packages/engine-wasm/internal/engine/layer_ops.go` | Add `fillColor`, `strokeColor`, `strokeWidth` to `LayerNodeMeta`; populate in `buildLayerNodeMeta` |
| `packages/engine-wasm/internal/engine/render.go` / `engine.go` | Add `editingVectorLayerID` to `UIMeta` |
| `packages/engine-wasm/internal/engine/dispatch_shape_test.go` | Tests for all 3 commands |
| `apps/editor-web/src/App.tsx` | CommitVectorEdit on tool switch away; banner while editing |
| `apps/editor-web/src/components/adjustments-panel.tsx` | Add VectorLayer branch → `VectorPropertiesPanel` |
| `apps/editor-web/src/components/layers-panel.tsx` | Double-click VectorLayer → `EnterVectorEditMode` |
| New: `apps/editor-web/src/components/vector-properties-panel.tsx` | Fill/stroke UI component |
