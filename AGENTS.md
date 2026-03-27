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
bun run lint            # Biome check
bun run lint:fix        # Biome check --write
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
| `biome` | "Formatter would have printed…" | `just fmt`, then re-stage |
| `biome` | Lint rule violations | `just lint-fix`, fix remaining issues manually, then re-stage |
| `typecheck` | TypeScript type errors | Fix the TS errors, then re-stage |
| `go-vet` | Go vet warnings | Fix the Go issues, then re-stage |

**General workflow when a commit is blocked:**

```bash
just fmt          # auto-format everything (biome + gofumpt + gci + shfmt)
just lint-fix     # auto-fix lint issues
git add -u        # re-stage the fixed files
git commit -m "your message"
```

Never use `--no-verify` to bypass hooks — the same checks run in CI and will fail there.

## Implementation Plan

See `PLAN.md` for the full phased roadmap.
