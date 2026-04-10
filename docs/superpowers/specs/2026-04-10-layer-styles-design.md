# Layer Styles Design

## Purpose

Define how Agogo-Web should complete Phase 6.5 from `PLAN.md`: a full Photoshop-style layer styles system implemented in sequenced subprojects rather than one large combined rollout.

The user explicitly wants the full Photoshop-style stack, even if that takes longer. This design therefore keeps the full scope, but decomposes it so the engine render contract is built first and the later UI work sits on stable behavior.

## Current State

The repository already has partial layer-style foundations:

- `LayerNode` includes `StyleStack() []LayerStyle` and `SetStyleStack([]LayerStyle)`.
- Project save/load already persists `styleStack`.
- Layer cloning already deep-copies style entries.
- Layer compositing currently ignores styles entirely.
- Merge and flatten intentionally reject styled layers because styles are not rasterized yet.

This means the primary blocker is not persistence or basic layer metadata. The blocker is the absence of a real backend style compositor.

## Design Goals

- Support the full Phase 6.5 Photoshop-style effect catalog over multiple subprojects.
- Keep all rendering in the Go/Wasm engine.
- Preserve the current layer tree, blend mode, clipping, masking, and viewport architecture.
- Make the backend authoritative for style validation and saved project compatibility.
- Unblock merge/flatten behavior for styled layers once the render core exists.
- Avoid frontend rework by stabilizing engine-side semantics before building the full dialog and preset workflows.

## Non-Goals For The First Subproject

These remain part of Phase 6.5 overall, but are intentionally deferred until later subprojects:

- Full Photoshop-style editing dialog in React
- Styles preset panel and preset thumbnails
- `.asl` import/export
- Blend If sliders and advanced per-channel blending controls
- Performance tuning beyond what is necessary for correctness and acceptable baseline usability

## Subproject Breakdown

### Subproject 1: Layer Styles Render Core

Build the engine-side layer style compositor and typed effect model.

Scope:

- Typed effect parameter structs in `packages/engine-wasm`
- Style decoding/validation from persisted `[]LayerStyle`
- Style-aware layer surface rendering
- Full effect render ordering
- Merge/flatten support for styled layers
- Project archive compatibility
- Engine commands for setting, enabling, clearing, copying, and pasting styles
- Regression tests for individual effects, composition order, masking, clipping, and merge semantics

This is the recommended first implementation slice.

### Subproject 2: Layer Style Editing UI

Build the Photoshop-style layer style dialog in `apps/editor-web`.

Scope:

- Entry point from the Layers panel / layer properties
- Left-side effect list with enable toggles
- Right-side parameter editor for the selected effect
- Live preview while editing
- OK / Cancel semantics
- New Style and Reset affordances backed by the engine commands from Subproject 1

### Subproject 3: Styles Panel And Style Workflows

Build reusable style presets and common workflows.

Scope:

- Styles panel with preset thumbnails
- Save current style as preset
- Apply preset to selected layer
- Copy Layer Style / Paste Layer Style UI affordances
- Preset persistence format for Agogo-Web

`.asl` import/export remains deferred unless separately approved later.

### Subproject 4: Advanced Blending

Implement the remaining Photoshop-style blending controls that sit at the compositing boundary.

Scope:

- Fill opacity UI parity
- Channel inclusion toggles (R/G/B)
- Blend If sliders for This Layer / Underlying Layer
- Split-slider support for smooth transitions

This is sequenced after the render core because it modifies the same compositing boundary and should not be designed against a placeholder style renderer.

## Recommended Approach

Use a backend-first, effect-by-effect pipeline.

Why this approach:

- The current compositor is the true blocker.
- The existing persisted `styleStack` field means the data model already exists at the archive boundary.
- UI-first work would create rework risk because effect ordering, mask semantics, and merge/flatten behavior are not yet defined in code.
- A stable render core lets the dialog, styles panel, and future PSD mapping target one authoritative engine contract.

## Backend Render Core Architecture

### Integration Point

The current layer compositing seam is:

- `renderLayerToSurface(layer)`
- `renderLayersToSurface(layers)`
- `compositeLayerOntoWithClip(dest, layer, clipAlpha)`

The layer-styles render core should integrate at that seam rather than replacing the whole document compositor.

The key addition is a style-aware layer renderer that produces a document-sized intermediate surface for a single layer before it is composited into the destination document buffer.

Proposed structure:

- Add a helper such as `renderStyledLayerSurface(doc, layer, clipAlpha)`.
- For raster layers (`PixelLayer`, `TextLayer`, `VectorLayer`), render the layer's base content surface first.
- Build effect surfaces from the base content's alpha and color information.
- Composite those effect surfaces together in Photoshop-style order.
- Composite the final styled result into the destination using the layer's blend mode and whole-layer opacity.

Group and adjustment layer composition should continue using the current document compositor with minimal disruption.

### Render Model

Each renderable non-adjustment layer should conceptually produce:

1. `baseContentSurface`
2. `styledSurface`
3. final document composite result

Rules:

- `fillOpacity` applies only to the base content, not to outer effects such as shadows or glows.
- whole-layer `opacity` applies after the layer and its effects have been combined.
- layer `blendMode` applies when the final styled result is composited into the destination document.
- masks and clipping should affect the silhouette that effects derive from.

This preserves the existing distinction already described in `effectiveContentOpacity()` vs `effectiveLayerOpacity()`.

### Effect Categories And Render Order

Effects should be rendered in ordered groups, not as an unordered list of blobs.

Proposed ordering:

1. Base layer content, using `fillOpacity`
2. Fill effects
   - Color Overlay
   - Gradient Overlay
   - Pattern Overlay
3. Interior / edge effects
   - Stroke
   - Inner Shadow
   - Inner Glow
   - Bevel & Emboss
   - Satin
4. Outer effects
   - Outer Glow
   - Drop Shadow
5. Final layer composite using layer `opacity` and layer `blendMode`

This order is the engine contract that later UI work must target.

### Raster Generation Strategy

The first implementation should stay entirely inside Go/Wasm and prefer correctness over aggressive optimization.

Approach:

- Derive the relevant alpha mask from the rendered layer surface.
- Render effect surfaces as full document-sized RGBA buffers in the first pass.
- Use AGG-backed drawing where shape/path rasterization is needed.
- Use engine-side pixel operations for effect post-processing such as blur, spread/choke, contour lookup, and compositing.
- Use the existing blend-mode implementation in `blend.go` for effect blend modes wherever possible.

The initial implementation may accept full-document temporary buffers for simpler correctness. Later optimization can tighten these to dirty bounds if profiling shows they are a bottleneck.

### Masks And Clipping Semantics

The style compositor must preserve the existing layer mask and clipping behavior.

Rules:

- Effects derive from the masked/clipped silhouette, not from the raw unmasked layer raster.
- Inner effects operate inside that silhouette.
- Outer effects may extend beyond the layer bounds, but they still originate from the masked/clipped silhouette.
- Vector masks should participate once the underlying layer render path includes them; style rendering must not introduce a second, conflicting mask contract.
- Clip-to-below behavior must continue to happen at the layer composite boundary, not as a special case inside individual effects.

### Merge / Flatten Semantics

Subproject 1 must remove the current “styled layers cannot be merged or flattened” blocker.

Rules:

- `renderLayerToSurface(layer)` should return the fully styled surface for merge/flatten callers.
- `MergeDown`, `MergeVisible`, and `FlattenImage` should therefore rasterize styles naturally through the existing surface-rendering flow.
- Saved results from merge/flatten are pixel layers; style metadata is intentionally baked in at that point.

## Data Model

### Persisted Boundary

Keep the persisted outer shape:

```go
type LayerStyle struct {
    Kind    string
    Enabled bool
    Params  json.RawMessage
}
```

This avoids archive format churn and preserves forward compatibility.

### Engine-Internal Typed Model

The engine should decode persisted entries into typed param structs before rendering. Unknown or malformed styles should fail safe by being treated as disabled entries rather than crashing render or rejecting project load.

Representative typed params:

- `DropShadowParams`
- `InnerShadowParams`
- `OuterGlowParams`
- `InnerGlowParams`
- `BevelEmbossParams`
- `SatinParams`
- `ColorOverlayParams`
- `GradientOverlayParams`
- `PatternOverlayParams`
- `StrokeParams`

The effect catalog should be fixed and normalized in one place.

### Shared Substructures

To reduce schema duplication:

- Reuse existing blend mode enums for effect blend modes.
- Reuse the existing gradient stop schema where practical.
- Define a shared color type consistent with current `[4]uint8`.
- Define reusable gradient and pattern payload structs so `Gradient Overlay`, `Outer Glow`, `Inner Glow`, and `Stroke` do not invent separate incompatible formats.

## Commands

The backend-first slice should add explicit commands to `packages/proto/src/commands.ts` and corresponding engine dispatch:

- `SetLayerStyleStack`
- `SetLayerStyleEnabled`
- `SetLayerStyleParams`
- `CopyLayerStyle`
- `PasteLayerStyle`
- `ClearLayerStyle`

Why include these early:

- The render core needs a stable mutation API before the full dialog exists.
- Copy/paste style is just style-stack mutation plus engine-side clipboard state and does not require waiting for the Styles panel.
- The later UI subprojects can build on the same commands instead of inventing temporary shape-specific or dialog-specific mutations.

## Validation Rules

Validation should happen in the engine, not only in React.

Rules:

- Numeric values are clamped to safe ranges.
- Enum-like fields normalize unknown values to defaults.
- Missing fields receive conservative defaults.
- Unknown effect kinds remain present in persisted data when possible but render as disabled/no-op until understood.
- Invalid payloads should not panic or corrupt document state.

This keeps the engine authoritative for project load, direct command dispatch, and future PSD import mapping.

## Full Effect Catalog For Phase 6.5

The full planned effect stack remains:

- Drop Shadow
- Inner Shadow
- Outer Glow
- Inner Glow
- Bevel & Emboss
- Satin
- Color Overlay
- Gradient Overlay
- Pattern Overlay
- Stroke

The backend-first slice should define typed params and render support for this whole catalog, even if some editors in the later UI rollout begin with denser or simpler controls than Photoshop.

## Testing Strategy

Subproject 1 should be test-heavy before any large UI rollout.

Required coverage:

- Style-stack archive round-trip
- Layer clone defensive copies remain correct with richer style payloads
- Single-effect render tests for every effect kind
- Ordered multi-effect composition tests
- `fillOpacity` vs `opacity` semantics
- Mask interaction
- Clip-to-below interaction
- Merge-down / merge-visible / flatten-image on styled layers
- Copy/paste/clear style commands
- Malformed params and unknown effect kinds fail safely

### Render Assertion Style

Prefer deterministic engine tests over frontend snapshots.

Use:

- fixture-driven pixel assertions at representative coordinates
- compact golden-surface tests only where point checks are too weak, such as bevel/emboss or satin interactions

The aim is to keep tests resilient while still proving compositing correctness.

## Risks And Mitigations

### Risk: Full-document temporary surfaces are expensive

Mitigation:

- Start with full-document surfaces for correctness.
- Benchmark after correctness lands.
- Optimize to dirty bounds only if profiling shows the style renderer is a real bottleneck.

### Risk: Effect schemas fragment across the UI and engine

Mitigation:

- Define effect catalog and typed params once in the engine boundary and mirror them in proto.
- Reuse shared gradient/pattern/color substructures.

### Risk: Merge and flatten behavior diverges from on-canvas rendering

Mitigation:

- Route both through the same style-aware `renderLayerToSurface` path.

### Risk: Advanced blending expands scope too early

Mitigation:

- Explicitly keep `Blend If` and per-channel blending in Subproject 4, after the render core and style dialog are stable.

## Acceptance Criteria

Phase 6.5 should be considered fully designed when:

- The work is decomposed into sequenced subprojects with clear boundaries.
- Subproject 1 is defined as the engine render core and merge/flatten unblocker.
- The full Photoshop-style effect catalog remains in scope.
- The later UI, preset, and advanced-blending work is defined but intentionally deferred in sequence rather than removed.

Subproject 1 should be considered ready for planning when:

- The render order is fixed.
- The compositing integration point is fixed.
- The command surface is fixed.
- Validation and test expectations are fixed.

