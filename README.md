> Try it online: [Agogo Web live demo](https://cwbudde.github.io/Agogo-Web/)

# Agogo Web

Agogo Web is a browser-native image editor built around a strict split of responsibilities: a Go WebAssembly engine owns document state, rendering, compositing, selections, filters, and export logic, while a React and Vite frontend provides the desktop-style shell and presents engine-produced pixels on an HTML canvas.

The result is a Photoshop and Photopea style editor that keeps pixel work out of JavaScript. The canvas is display-only. The editor UI sends intents and commands to the engine, and the engine returns rendered frames, dirty rectangles, and UI metadata.

## Highlights

- Full layer-based editing with groups, masks, clipping, blend modes, layer styles, and non-destructive adjustment layers
- Desktop-style workspace with document tabs, dense panel layout, customizable shortcuts, rulers, guides, grids, and smart snapping
- Selection, transform, crop, brush, pencil, eraser, fill, gradient, and eyedropper workflows rendered by the Wasm engine
- Text, vector paths, shapes, pen tooling, and editable style stacks for design-heavy workflows
- PSD and PSB import and export, plus Agogo's native project format for fast save and restore
- Artboards, slices, actions, variables, scripting, and batch export for production workflows
- Worker-based rendering pipeline with dirty-rect updates, cache layers, and SharedArrayBuffer-ready transport for large documents

## Architecture

Agogo Web is designed around one non-negotiable rule: no JavaScript-side pixel manipulation.

### Rendering model

1. The frontend collects user intent through menus, tools, pointer input, keyboard shortcuts, and panel controls.
2. Commands are sent across a shared ABI to the Go engine compiled for `js/wasm`.
3. The engine updates document state, re-renders affected regions, and returns a `RenderResult` with viewport metadata, dirty rects, and a pointer to the RGBA buffer in Wasm memory.
4. The frontend reads the buffer and blits it to the canvas with `putImageData`.

This design keeps rendering deterministic, centralizes editing logic, and avoids splitting imaging behavior across two runtimes.

### Monorepo layout

```text
apps/
  editor-web/        React 19 + Vite 6 editor shell
packages/
  engine-wasm/       Go 1.25 image engine compiled to WebAssembly
  proto/             Shared command IDs and response contracts
docs/                Product and UI design documentation
justfile             Primary task runner
treefmt.toml         Cross-language formatting config
LICENSES.md          Third-party and licensing notes
```

### Core packages

- `apps/editor-web`: the application shell, canvas host, panel system, command dispatch, and Wasm lifecycle
- `packages/engine-wasm`: document model, viewport renderer, compositing pipeline, history system, painting and filter logic, import and export, and performance infrastructure
- `packages/proto`: the ABI surface between frontend and engine, including command IDs and render response types

## Feature Set

### Editing model

- Multi-document workspace with tabs, presets, recovery, and autosave
- Layer tree with pixel, group, adjustment, text, and vector layers
- Full blend mode support, independent opacity and fill, clipping masks, vector masks, and raster masks
- Undo and redo history with grouped transactions and jump-to-state navigation

### Navigation and layout

- Cursor-anchored zoom, pan, rotate-view, fit-to-screen, and navigator preview
- Dense desktop UI with menu bar, options bar, compact tool rail, status bar, and right-side dock panels
- Rulers, guides, grids, snapping, smart guides, and configurable workspace presets

### Selection and transform

- Rectangular, elliptical, polygon, freehand, magic wand, and quick selection tools
- Marching ants overlay, feather, expand, contract, smooth, border, inverse, and saved selections
- Free transform, skew, distort, perspective, warp, transform selection, and crop workflows

### Painting and retouching

- Brush, pencil, eraser, paint bucket, gradient, and eyedropper tools
- Pressure-aware stroke engine with spacing, flow, opacity, smoothing, and dynamics
- Destructive and history-aware painting operations on pixel layers with dirty-region updates

### Non-destructive color and filter pipeline

- Adjustment layers for levels, curves, hue and saturation, color balance, exposure, vibrance, black and white, threshold, posterize, gradient map, channel mixer, and more
- Filter framework with live preview and fade controls
- Blur, sharpen, noise, distort, stylize, and morphology filters rendered in the engine

### Design workflows

- Pen, direct selection, shape, and custom path tools
- Editable text layers with character and paragraph controls
- Layer styles including shadows, glows, bevel, satin, overlays, stroke, and blend-if behavior

### File compatibility and automation

- Native `.agp` project save and load
- PSD and PSB parsing and writing for layered documents
- Artboards and slices for asset export
- Actions, variables, datasets, batch processing, and scriptable command execution

### Performance model

- Engine execution in a Web Worker to keep the UI thread responsive
- Dirty rectangle rendering and selective canvas blits
- Layer and tile caching, multi-resolution pyramids, and memory-budgeted cache eviction
- SharedArrayBuffer-ready frame transport when cross-origin isolation is enabled

## Command ABI

The frontend and engine communicate through a small, explicit command protocol.

- Shared command IDs live in `packages/proto/src/commands.ts`
- Shared render response types live in `packages/proto/src/responses.ts`
- The engine exports initialization, dispatch, render, memory, and project import and export entry points

The ABI keeps tool interactions, history, viewport changes, and panel state synchronized without duplicating editor logic in the frontend.

## Development Stack

- Go 1.25 for the rendering and document engine
- WebAssembly via `GOOS=js GOARCH=wasm`
- React 19 and Vite 6 for the editor shell
- Bun workspaces for package management and workspace scripts
- Tailwind CSS v4 and local UI wrappers for shell composition
- Just as the main task runner
- Biome, treefmt, golangci-lint, and lefthook for formatting, linting, and pre-commit checks

## Getting Started

### Prerequisites

- Bun
- Go 1.25+
- `just`
- `treefmt`
- `golangci-lint`

### Install

```bash
just install
```

### Run the editor

```bash
just dev
```

This builds the Wasm engine, copies `wasm_exec.js` into the frontend public directory, and starts the Vite development server.

### Build for production

```bash
just build
```

## Common Commands

| Command | Purpose |
| --- | --- |
| `just install` | Install workspace dependencies and git hooks |
| `just dev` | Build the Wasm engine and start the editor locally |
| `just wasm-build` | Compile `engine.wasm` and refresh frontend runtime assets |
| `just build` | Produce a full production build |
| `just test` | Run Go tests and frontend type checking |
| `just test-go` | Run Go unit tests |
| `just test-go-race` | Run Go tests with the race detector |
| `just test-go-coverage` | Generate Go coverage output and HTML report |
| `just update-golden` | Refresh golden snapshots for render tests |
| `just lint` | Run Go vet, golangci-lint, and frontend linting |
| `just lint-fix` | Auto-fix lint issues where possible |
| `just fmt` | Format the repository with treefmt and Biome |
| `just ci` | Run formatting checks, tests, linting, tidy verification, and production build |
| `just clean` | Remove generated build artifacts |

Frontend note: `bun run lint` is lint-only. Formatting is intentionally separate so you can run it in a focused pass via `just fmt` or `bun run --cwd apps/editor-web format`.

## Workspace Guide

### Frontend

The frontend lives in `apps/editor-web` and is responsible for:

- bootstrapping the Go runtime and loading `engine.wasm`
- hosting the editor shell and workspace chrome
- forwarding commands and pointer input to the engine
- blitting returned pixel buffers to the canvas
- presenting engine-returned UI metadata in panels and status views

### Engine

The engine lives in `packages/engine-wasm` and is responsible for:

- document state and viewport management
- compositing, blending, masks, and layer effects
- selections, transforms, painting, filters, and adjustments
- project persistence and PSD/PSB compatibility
- history, undo and redo, and render invalidation
- performance features such as dirty rects, caches, tiling, and worker-safe execution

### Shared protocol

The shared ABI lives in `packages/proto`. It defines the command and response contract used by both runtimes so the editor shell stays thin and the imaging engine remains authoritative.

## Testing and Quality

Agogo Web is structured to make rendering behavior testable and reproducible.

- Go unit tests validate compositing, masks, history behavior, document operations, and format handling
- Golden render tests catch visual regressions in blend modes, filters, selections, and viewport output
- ABI tests verify frontend to engine command compatibility
- Browser-level end-to-end tests exercise document creation, painting, transform flows, history, and export behavior

For a full repository check, run:

```bash
just ci
```

## Browser and Runtime Notes

- The development server sends `Cross-Origin-Opener-Policy: same-origin` and `Cross-Origin-Embedder-Policy: require-corp` so SharedArrayBuffer-based Wasm paths can run during local development.
- The production deployment should preserve those headers if zero-copy and thread-adjacent Wasm optimizations are enabled.
- The editor is built for modern evergreen browsers with solid WebAssembly support.

## Licensing

This repository is proprietary.

Third-party licensing notes and known review items live in `LICENSES.md`. In particular, any release process should keep the AGG dependency and polygon clipping licensing situation under active review.

## Repository Notes

- `PLAN.md` documents the full implementation plan and product surface area
- `docs/ui-shell-phase-1_5-design.md` captures the dense desktop shell direction
- `AGENTS.md` contains repository-specific guidance for coding agents working in this monorepo

## Philosophy

Agogo Web is built to behave like a serious desktop editor, not a canvas toy. The frontend owns interaction and presentation. The engine owns imaging truth. That separation is what makes the project coherent, testable, and scalable as the feature set expands.
