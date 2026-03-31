# AGENTS.md

This file provides guidance to ai agents (Claude Code, Codex etc.) when working with code in this repository.

## Project Overview

Agogo-Web is a Photoshop/Photopea clone built as a monorepo. All pixel rendering runs in a Go WebAssembly engine (`packages/engine-wasm`); the React/Vite frontend (`apps/editor-web`) is display-only — it calls `putImageData` on an HTML Canvas using pixel buffers returned from Wasm.

## Monorepo Structure

```plain
apps/editor-web/        # React 19 + Vite 6 + Tailwind v4 frontend
packages/engine-wasm/   # Go 1.25 WebAssembly rendering engine (AGG-backed)
packages/proto/         # Shared TypeScript command IDs & response types
justfile                # Primary task runner
```

## Common Commands

All major workflows go through `just`:

```bash
just dev                # Build wasm then start Vite dev server
just build              # Full production build (wasm + frontend)
just test               # Run all tests (Go + TypeScript)
just test-go            # Go unit tests only
just test-go-race       # Go tests with race detector
just test-go-coverage   # Coverage report
just lint               # Lint everything (Go + TS)
just lint-fix           # Auto-fix linting issues
just fmt                # Format all code via treefmt
just ci                 # Full CI: format → test → lint → build
just wasm-build         # Compile Go to Wasm, copy wasm_exec.js to public/
just clean              # Remove all build artifacts
```

Frontend-only commands (from `apps/editor-web/`):
```bash
bun run dev             # Vite dev server
bun run lint            # Biome lint only (no formatter drift checks)
bun run lint:fix        # Biome lint --write
bun run format          # Biome formatter --write
bun run typecheck       # tsc --noEmit
```

Go commands (from `packages/engine-wasm/`):
```bash
go test ./...           # Run tests
go test -run TestName ./internal/engine/  # Single test
go vet ./...            # Vet
```

## Architecture

### Command ABI (frontend → engine)
Commands are JSON-encoded and dispatched via `DispatchCommand(cmdId, jsonPayload)`. Command IDs and TypeScript payload types live in `packages/proto/src/commands.ts`. The Go engine receives them in `packages/engine-wasm/internal/engine/engine.go`.

### Render Loop
1. Frontend calls `RenderFrame()` on the Wasm engine
2. Engine returns a `RenderResult` (JSON) with `bufferPtr`, `bufferLen`, dirty rects, viewport state, and UI metadata
3. Frontend reads the RGBA pixel buffer via `GetBufferPtr()`/`GetBufferLen()` and calls `putImageData` on the canvas

### Key WASM exports (`cmd/engine/main.go`)
- `EngineInit(jsonConfig)` – Initialize engine
- `DispatchCommand(cmdId, jsonPayload)` – Send command to engine
- `RenderFrame()` – Render and get result
- `GetBufferPtr()` / `GetBufferLen()` – Access pixel buffer
- `Free(ptr)` – Free allocated memory

### Frontend Wasm integration
- `apps/editor-web/src/wasm/loader.ts` – Loads Go runtime + engine.wasm
- `apps/editor-web/src/wasm/context.tsx` – React context exposing engine handle
- `apps/editor-web/src/wasm/types.ts` – TypeScript interfaces for the engine

### Canvas component
`apps/editor-web/src/components/editor-canvas.tsx` — receives pixel data from Wasm and blits it to the HTML Canvas. No JS-side pixel manipulation.

### Graphics Rendering: agg_go (MANDATORY)

All graphic processing **must** use `agg_go` — the Go port of the Anti-Grain Geometry (AGG) library. This is a core design constraint, not a preference:

- `agg_go` handles all pixel formats (RGBA, RGB, grayscale, etc.) and rendering styles
- The project exists specifically to showcase the breadth and quality of `agg_go`'s capabilities
- Never implement pixel-level rendering logic outside of `agg_go` — use its scanline rasterizer, span generators, color interpolation, and compositing primitives
- When adding new rendering features (gradients, patterns, filters, blending modes, anti-aliasing), first explore what `agg_go` already provides before writing custom code

### Extending agg_go with Missing Features

The Go port lives at `../agg_go`; the original C++ AGG 2.6 source lives at `../agg-2.6/agg-src`.

**Step 1 — Can the feature be composed from existing agg_go primitives?**

Before adding anything to `agg_go`, check whether the missing behaviour can be assembled from what already exists:

| Technique | Where to look in agg_go |
|-----------|------------------------|
| Combine two vertex sources | `internal/conv/concat.go` |
| Apply a transform inline | `internal/conv/transform.go` |
| Custom fill via scanline | `internal/span/` generators + `internal/renderer/scanline/` |
| Blend two rendered layers | `internal/pixfmt/composite.go` + blending modes |
| Filtered image sampling | `internal/image/image_accessors.go` + `internal/span/span_image_filter*.go` |
| Reuse an existing gradient | `internal/span/span_gradient*.go` + `internal/span/gradient_lut.go` |

Only proceed to Step 2 if no combination of existing primitives can produce the desired result.

**Step 2 — Porting from C++ AGG**

When a feature genuinely does not exist in `agg_go`, port it directly from the corresponding C++ header in `../agg-2.6/agg-src/include/`. Maintain the same structural style as the rest of `agg_go`:

1. **Find the C++ source** — locate `agg_<feature>.h` (and the matching `.cpp` if any) in `../agg-2.6/agg-src/include/` or `../agg-2.6/agg-src/src/`.
2. **Mirror the architecture** — one Go file per C++ header, placed in the matching `internal/` subdirectory (e.g. a new span generator goes in `internal/span/`, a new path converter in `internal/conv/`).
3. **Use idiomatic Go, not literal C++ translation** — generics via interfaces, not templates; value receivers where the C++ uses stack objects; slices instead of raw pointer arithmetic.
4. **Keep the AGG naming convention** — types and files should be recognisable to anyone familiar with the C++ library (e.g. `SpanGradientConical` for a new gradient, `RendererOutlineImage` for image-textured outlines).
5. **Write a test that exercises the new code** — place it in the same package with a `_test.go` suffix.

**Known gaps** (C++ features not yet in the Go port, in rough priority order):

| Feature | C++ header | Target Go location |
|---------|------------|-------------------|
| Image-textured outline rendering | `agg_renderer_outline_image.h` | `internal/renderer/outline/` |
| Complete image-filter kernel suite (Lanczos, sinc, etc.) | `agg_image_filters.h` | `internal/image/filters.go` |
| Styled-cell AA rasteriser (compound sub-paths) | `agg_rasterizer_cells_aa.h` styled variant | `internal/rasterizer/cells_aa_styled.go` |
| Perspective-subdivision span interpolator | `agg_span_interpolator_persp.h` subdivided variant | `internal/span/` |
| Public color-conversion API (RGB8/RGB16) | `agg_color_conv_rgb8/16.h` | `internal/color/conv/` → expose via public API |

If you add a feature not listed here, follow the same two-step process and document the gap you closed.

## Tooling

| Tool | Purpose |
|------|---------|
| Bun | Package manager + workspace runner |
| Just | Task runner (primary entry point) |
| Biome 2.x | TypeScript/JS/JSON linting & formatting (CSS excluded) |
| treefmt | Multi-language formatter (gofumpt, gci, biome, shfmt) |
| golangci-lint v2 | Go linting |
| lefthook | Pre-commit hooks: biome, typecheck, go-vet (parallel) |

Biome config is in `apps/editor-web/biome.json`. It only lints TS/JS/JSON — CSS linting is disabled to avoid conflicts with Tailwind syntax.

## Vite Dev Server

The dev server sets `Cross-Origin-Opener-Policy: same-origin` and `Cross-Origin-Embedder-Policy: require-corp` headers (in `vite.config.ts`) to enable `SharedArrayBuffer` for Wasm.

## CI

GitHub Actions workflows in `.github/workflows/`:
- `ci.yml` — Orchestrator: biome → typecheck → go-test → build (sequential dependencies)
- `test-biome.yml`, `test-typecheck.yml`, `test-go.yml`, `build.yml` — Individual job workflows

## Licensing

The code is proprietary (MeKo-Tech). Two licensing issues exist before commercial release:
- `agg_go` dependency needs a LICENSE file
- GPC (Polygon Clipper) is non-commercial only → must be replaced with Clipper2

## Pre-commit Hook Failures

The project uses **lefthook** to run `biome`, `typecheck`, and `go-vet` in parallel before every commit. If `git commit` fails, address the failing check:

| Hook | Failure symptom | Fix |
| ---- | --------------- | --- |
| `biome` | Lint rule violations | `just lint-fix`, fix remaining issues manually, then re-stage |
| `typecheck` | TypeScript type errors | Fix the TS errors, then re-stage |
| `go-vet` | Go vet warnings | Fix the Go issues, then re-stage |

**General workflow when a commit is blocked:**

```bash
just lint-fix     # auto-fix lint issues
git add -u        # re-stage the fixed files
git commit -m "your message"
```

Run `just fmt` separately when you want a focused formatting pass, for example at the end of a phase.

Never use `--no-verify` to bypass hooks — the same checks run in CI and will fail there.

## Implementation Plan

See `PLAN.md` for the full phased roadmap.
