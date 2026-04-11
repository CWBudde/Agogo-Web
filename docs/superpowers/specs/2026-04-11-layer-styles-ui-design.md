# Layer Styles UI And Presets Design

## Purpose

Define the next implementation slice for Phase 6.5 in `PLAN.md`: the user-facing layer style editing workflow that sits on top of the completed backend render core.

This spec covers the Photoshop-style editing dialog, document-scoped style presets, and the frontend/engine contract changes required to make the UI read and edit authoritative layer-style state. It intentionally does not expand scope into app-wide preset storage or advanced blending.

## Relationship To Prior Work

The backend render core was designed first in `docs/superpowers/specs/2026-04-10-layer-styles-design.md`.

That earlier design established:

- the effect catalog
- the render ordering contract
- merge/flatten semantics
- the engine-side style model and commands for layer style mutation

This document builds on that contract and defines the next safe layer-styles subproject: editing and reusing style stacks from the React frontend without inventing a second rendering or persistence model.

## Current State

The repository now has the key backend prerequisites for a real style editor:

- the engine understands the supported layer-style effect catalog
- layer-style commands already exist in `packages/proto/src/commands.ts`
- the Layers panel already owns the main layer context menu and layer-level property entry points
- the frontend already uses engine-driven `LayerNodeMeta` and `UIMeta` for active document state

The main missing pieces are on the UI boundary:

- `LayerNodeMeta` does not currently expose the current layer's `styleStack`
- document UI metadata does not expose reusable style presets
- there is no layer-style dialog or properties surface in `apps/editor-web`
- copy/paste/clear style workflows are not exposed in the UI

This means the next blocker is no longer rendering. The blocker is the absence of a frontend editing workflow and the response metadata needed to drive it.

## Design Goals

- Add a real layer-style editing workflow without duplicating backend style logic in the frontend
- Keep the engine authoritative for current style state, validation, and preview rendering
- Reuse existing UI seams in the Layers panel, Properties area, and existing dialog patterns
- Support live preview during editing with safe `OK` and `Cancel` semantics
- Add reusable style presets in a document-scoped form that fits the current architecture
- Keep the scope tight enough that one implementation plan can execute it end to end

## Non-Goals

These remain deferred to later work:

- app-wide style presets shared across documents
- `.asl` import/export
- Blend If controls
- per-channel blending checkboxes
- a dedicated global Styles studio panel detached from the layer editing workflow
- visual parity polish beyond what is needed for a coherent working editor

## Recommended Approach

Use an engine-authoritative editing workflow with immediate preview and document-scoped presets.

Why this approach:

- the current architecture already routes authoritative document state through engine UI metadata
- preview rendering already belongs in the engine, not in React
- document-scoped presets provide real reuse without introducing a new cross-document persistence layer
- the Layers panel already provides the right entry points, so the new workflow can attach to existing surfaces instead of creating unrelated UI

The rejected alternatives are:

- dialog-only editing with no presets yet: safe but leaves a meaningful part of Phase 6.5 missing
- app-wide presets now: broader user value, but requires new persistence architecture that is not currently justified by the existing code seams

## Scope

This subproject includes two tightly related deliverables.

### 1. Layer Style Editing Workflow

Build a dedicated `Layer Style...` dialog that edits the active layer's full style stack.

Scope:

- open from the Layers panel context menu
- open from Layer Properties
- open from a compact effects surface in the Properties area
- show the fixed effect catalog in Photoshop-style order
- allow per-effect enable/disable
- allow per-effect parameter editing for the backend-supported controls
- preview changes live by dispatching commands during editing
- support `OK`, `Cancel`, `Reset`, and `New Style`

### 2. Document-Scoped Style Workflows

Add reusable style workflows bound to the active document.

Scope:

- `Copy Layer Style`
- `Paste Layer Style`
- `Clear Layer Style`
- save the current style stack as a named document preset
- apply a document preset to the current layer
- delete or rename a document preset
- show compact preset affordances in the editing flow and active-layer properties surface

## Architecture

### Existing UI Seams To Reuse

The new workflow should attach to existing editor structure instead of inventing new navigation:

- `apps/editor-web/src/components/layers-panel.tsx`
  - owns the layer context menu
  - already opens `LayerPropertiesDialog`
  - already dispatches layer-level commands
- `apps/editor-web/src/components/adjustments-panel.tsx`
  - already establishes the “active-layer properties panel” pattern
- `apps/editor-web/src/components/ui/dialog.tsx`
  - provides the baseline modal shell to extend for the larger editor dialog

This means the layer-style UI should be introduced as a focused extension of the current panel/dialog system, not as an unrelated windowing subsystem.

### Engine-Authoritative State Model

The frontend should not maintain a long-lived shadow copy of layer styles as a second source of truth.

Rules:

- the engine owns the canonical `styleStack`
- the engine owns document preset storage for the active document
- the frontend reads both through normal render-response UI metadata
- the frontend may hold only short-lived draft state needed to manage dialog interaction and cancellation

This keeps save/load behavior, direct command dispatch, and preview semantics aligned with the backend render contract.

## Response Metadata Changes

### Layer Metadata

Extend `packages/proto/src/responses.ts` so `LayerNodeMeta` includes the current layer style stack.

Proposed shape:

```ts
styleStack?: LayerStyleEntryCommand[];
```

This should be present for all renderable layer types that can own styles, and absent or empty where styles do not apply.

### Document UI Metadata

Extend `UIMeta` with document-scoped style presets.

Proposed shape:

```ts
stylePresets?: Array<{
  id: string;
  name: string;
  styles: LayerStyleEntryCommand[];
}>;
```

This keeps preset reads on the same UI metadata path already used for active document state.

Layer styles in this subproject are explicitly supported for:

- pixel layers
- text layers
- vector layers

They are explicitly not part of this UI slice for:

- adjustment layers
- group layers

## Commands

### Existing Layer Style Commands

The dialog and context menu should continue using the existing layer-style commands:

- `SetLayerStyleStack`
- `SetLayerStyleEnabled`
- `SetLayerStyleParams`
- `CopyLayerStyle`
- `PasteLayerStyle`
- `ClearLayerStyle`

### New Preset Commands

Document-scoped preset workflows need explicit commands rather than frontend-only state:

- `CreateDocumentStylePreset`
- `UpdateDocumentStylePreset`
- `DeleteDocumentStylePreset`
- `ApplyDocumentStylePreset`

These commands should be added to `packages/proto/src/commands.ts` and handled by the engine beside the existing layer-style command set.

`UpdateDocumentStylePreset` should support at least name updates and style-stack replacement so the UI does not need separate rename/save-over flows.

## Dialog Interaction Model

### Open Behavior

Opening the dialog should capture the current layer's style stack as the restoration point for `Cancel`.

The dialog should read the live layer state from engine metadata and initialize:

- selected effect kind
- original style stack snapshot
- current preset selection, if any is visually indicated

### Live Preview

All edits should preview immediately in the engine.

Rules:

- toggling an effect dispatches `SetLayerStyleEnabled`
- editing effect params dispatches `SetLayerStyleParams`
- resetting the whole stack dispatches `SetLayerStyleStack`
- applying a preset dispatches `ApplyDocumentStylePreset`

The frontend does not render preview pixels itself.

### OK / Cancel / Reset

Dialog semantics:

- `OK`: close the dialog and keep the already-previewed engine state
- `Cancel`: restore the original captured stack with `SetLayerStyleStack`, then close
- `Reset`: replace the current layer stack with an empty stack immediately

This model is simple, deterministic, and aligned with the current engine-owned document state.

### New Style

`New Style` should capture the current live stack and create a new document-scoped preset.

The first implementation should use a lightweight naming flow:

- default the name from the current layer name or `Style N`
- allow inline rename after creation or a simple name prompt in the dialog flow

The exact text-entry affordance is a UI detail; the requirement is that creating a preset is a first-class action inside the dialog.

## UI Structure

### Entry Points

Expose the workflow in three places:

1. Layers panel context menu
   - `Layer Style...`
   - `Copy Layer Style`
   - `Paste Layer Style`
   - `Clear Layer Style`

2. Layer Properties dialog
   - add a `Layer Style...` action button

3. Active-layer properties surface
   - for layers that are not handled by the adjustment, text, or vector-specific editors, show a compact `Effects` card
   - the card should summarize enabled effects and show preset affordances
   - include an `Edit Styles...` action

### Dialog Layout

Use a three-part dialog layout:

- left column: fixed effect catalog in backend order, each row with enable toggle
- main editor pane: controls for the currently selected effect
- footer: `Reset`, `New Style`, `Cancel`, `OK`

A compact preset strip should be visible either above the editor pane or directly above the footer. Presets should be part of the editing workflow, not buried elsewhere.

### Effect Editors

Build explicit effect editors instead of a schema-driven generic form system in this pass.

Rules:

- use one editor section per effect kind
- reuse existing numeric input and slider styling patterns from current inspector UIs
- reuse existing gradient editing pieces where practical
- expose only controls the backend already supports
- do not add UI for deferred features with no backend behavior

This keeps the implementation grounded in the actual engine contract and avoids a large form-abstraction project.

## Preset Semantics

### Storage Scope

Presets are document-scoped in this subproject.

Implications:

- presets persist with the current document
- duplicating or saving the document keeps its style presets
- opening a different document yields that document's own preset set
- no global preset library or browser storage is introduced

### Apply Behavior

Applying a preset replaces the current layer's style stack. It does not merge selectively with the existing stack.

This behavior is simpler, predictable, and aligned with Photoshop-style named style application.

### Rename And Delete

Users should be able to rename and delete document presets from the same workflow. The first pass can keep these affordances lightweight, but they must exist because preset lists become cluttered quickly without maintenance actions.

## Validation And Error Handling

### Unsupported Layers

Entry points should be hidden or disabled for layers that cannot meaningfully own styles.

At minimum:

- adjustment layers should not show a style editor entry
- groups should remain excluded unless the engine explicitly supports group styles in current metadata and render behavior

### Stale State Protection

If the active layer changes while the dialog is open, the dialog should close rather than attempting to silently retarget edits to a different layer.

If the layer disappears, the dialog should also close cleanly.

### Failed Preset Operations

If preset creation or update fails at the engine boundary, the UI should keep the dialog open and avoid pretending the operation succeeded. The first implementation can surface this conservatively through existing status text or a minimal inline error surface.

## Testing Strategy

### Engine Tests

Add tests for:

- document preset create/update/delete/apply behavior
- preset persistence through project save/load
- applying a preset to a layer replaces the full stack
- invalid preset payloads fail safely

### Proto / Metadata Tests

Add coverage for:

- `LayerNodeMeta.styleStack`
- `UIMeta.stylePresets`
- command typing for document preset operations

### Frontend Tests

Add targeted tests around the layers panel and dialog workflow:

- opening `Layer Style...` from the context menu
- dispatching effect enable/disable and param-edit commands
- `Cancel` restoring the original stack
- `Copy Layer Style`, `Paste Layer Style`, `Clear Layer Style`
- creating, applying, renaming, and deleting document presets
- closing the dialog when the active layer changes

At least one test should assert that editing dispatches live-preview commands before `OK`.

## Delivery Notes

This subproject is intentionally broader than “just a dialog” but narrower than the full remaining Phase 6.5 scope.

It completes the core user-facing editing loop on top of the render core and leaves the following work for later:

- richer Styles panel presentation
- app-wide preset storage
- advanced blending controls
- import/export compatibility work beyond Agogo-Web's own document format
