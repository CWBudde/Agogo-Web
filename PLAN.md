# Implementation Plan: Photoshop/Photopea Clone (Agogo Web Editor)

> **Architecture Summary:**
> - **Backend:** Go (private AGG port) compiled to WebAssembly — owns all rendering, document state, pixel data, overlays
> - **Frontend:** Vite + React + TypeScript, shadcn + Tailwind CSS v4 + Base UI (headless primitives)
> - **Rule:** No pixel processing in JS/Canvas. Canvas is display-only (`putImageData`). All compositing, overlays, zoom/rotate happen in the Go/Wasm engine.
> - **ABI:** Frontend sends intents/events (pointer, keyboard, commands). Engine returns `RenderResult` (RGBA pixel buffer + UI metadata JSON).

---

## Phase X: Rendering Performance Baseline & Hotspot Reduction

**Goal:** Establish a reproducible performance baseline for the rendering engine, identify the dominant costs in the native Go pipeline before browser/Wasm overhead, and prioritize the first optimization passes.

**Acceptance criterion:** A deterministic benchmark exists for a realistic 512×512 paint-and-render scenario, benchmark and `pprof` commands are documented, and the current top CPU/allocation hotspots are captured in the plan with clear next optimization targets.

### Phase X.1: Benchmark Harness & Profiling Baseline

- [x] Add a dedicated engine benchmark in `packages/engine-wasm/internal/engine/render_benchmark_test.go` that:
  - [x] Creates an empty 512×512 document with a single pixel layer
  - [x] Paints deterministic brush strokes with realistic pressure variation
  - [x] Renders to the offline viewport canvas
  - [x] Splits the pipeline into separate sub-benchmarks for painting, document compositing, viewport rendering, cached-frame rendering, and end-to-end paint+render
- [x] Benchmark command captured:
  - [x] `go test ./internal/engine -run '^$' -bench '^BenchmarkRenderPipeline512$' -benchmem`
- [x] CPU profiling command captured:
  - [x] `go test ./internal/engine -run '^$' -bench '^BenchmarkRenderPipeline512/RenderFrameAfterPaint$' -benchtime=2s -cpuprofile /tmp/agogo-render-frame-after-paint.cpu`
  - [x] `go tool pprof -top /tmp/agogo-render-frame-after-paint.cpu`
- [x] Allocation profiling command captured:
  - [x] `go test ./internal/engine -run '^$' -bench '^BenchmarkRenderPipeline512/RenderFrameAfterPaint$' -benchtime=2s -memprofile /tmp/agogo-render-frame-after-paint.mem`
  - [x] `go tool pprof -top -alloc_space /tmp/agogo-render-frame-after-paint.mem`

### Phase X.2: Current Baseline Findings

- [x] Native Go baseline measured on Linux / Intel i7-1255U for the 512×512 benchmark scenario:
  - [x] `PaintStrokes`: ~18.8 ms/op, ~46.3 MB/op, ~37.2k allocs/op
  - [x] `CompositeSurface`: ~3.36 ms/op, ~1.05 MB/op, 2 allocs/op
  - [x] `RenderViewport`: ~12.6 ms/op, ~1.42 MB/op, ~41.7k allocs/op
  - [x] `RenderFrameCachedComposite`: ~13.2 ms/op, ~2.47 MB/op, ~41.7k allocs/op
  - [x] `RenderFrameAfterPaint`: ~33.8 ms/op, ~50.9 MB/op, ~78.9k allocs/op
- [x] Primary conclusion recorded: the engine is already expensive in native Go, so the current slowdown is not explained primarily by WebAssembly overhead.

### Phase X.3: Hotspots Confirmed By `pprof`

- [x] CPU hotspots identified:
  - [x] Viewport sampling in `internal/engine/viewport_composite.go`, especially `sampleBilinear`
  - [x] AGG-driven viewport background work in `internal/agg/agg.go`, especially checkerboard/document background drawing
  - [x] Brush dab rasterization in `internal/engine/brush.go` (`PaintDab`)
  - [x] Full document compositing in `internal/engine/layer_ops.go`
- [x] Allocation hotspots identified:
  - [x] AGG rasterizer cell allocation (`RasterizerCellsAASimple.allocateBlock`) dominates allocation volume during brush dabs
  - [x] Repeated `NewAgg2D()` construction contributes measurable allocation cost
  - [x] Per-stroke undo snapshotting in `handleBeginPaintStroke` allocates a full copy of the active layer before each stroke
  - [x] Cached-frame rendering still allocates heavily, which indicates the viewport path is allocation-heavy even when document compositing is reused

### Phase X.4: Transform Replacement Audit & Scope Definition

**Goal:** Define exactly which parts of `internal/engine/transform.go` should move from manual pixel math to AGG-backed image transforms, and which parts should intentionally remain manual.

**Acceptance criterion:** Every major transform path is classified as `replace with AGG`, `keep manual`, or `defer`, with rationale tied to correctness, interpolation support, and measured performance.

- [x] Record the architectural starting point and resulting decision:
  - [x] Perspective and mesh-warp paths already use AGG (`TransformImageQuad`)
  - [x] Affine free-transform commit was the last major manual inverse-mapping/manual resampling path and has now been migrated to AGG-backed dispatch
  - [x] Discrete pixel transforms (`flip`, `rotate 90°`, `rotate 180°`) remain exact index-remap loops
  - [x] Transform handles overlay remains a manual canvas-space overlay renderer
- [x] Classify each transform path explicitly:
  - [x] `applyPixelTransform` affine branch: replace with AGG
    - [x] Rationale: this is the one major transform path still doing manual inverse-mapping and manual resampling even though AGG image transforms already exist in the same file for harder cases
    - [x] Rationale: moving affine to AGG unifies affine, perspective, and warp under one image-transform pipeline and removes duplicated sampling logic
  - [x] `DistortCorners` branch: keep AGG-backed and align behavior with affine branch
    - [x] Rationale: this path already uses `TransformImageQuad` and is conceptually the correct backend for perspective-style distortion
    - [x] Rationale: the work here is consistency and shared setup, not replacement
  - [x] `WarpGrid` branch: keep AGG-backed and align behavior with affine branch
    - [x] Rationale: the mesh-warp path is already implemented as AGG perspective patches per cell and should remain the reference implementation for warp rendering
  - [x] Discrete transforms: keep manual exact pixel remaps
    - [x] Rationale: `flip` and `rotate 90°/180°` are exact index-rearrangement operations, not resampling problems
    - [x] Rationale: these paths are simpler, deterministic, and likely cheaper than paying AGG setup cost for no interpolation benefit
  - [x] Transform overlay rendering: keep manual overlay drawing
    - [x] Rationale: `RenderTransformHandlesOverlay` is UI overlay work rather than image-resampling work and does not benefit from routing through AGG by default
    - [x] Rationale: migrate only if measurement later shows the overlay itself is a real bottleneck
- [x] Document the required parity guarantees before migration:
  - [x] Bounds computation and output layer positioning must remain stable
    - [x] The migrated affine path must return the same `LayerBounds` contract and preserve the current layer placement semantics
  - [x] Alpha handling and edge clamping must match current behavior or change intentionally with tests
    - [x] Any differences at transparent edges or clamp boundaries must be treated as an explicit behavior change, not incidental fallout from the migration
  - [x] Interpolation modes must remain available: nearest, bilinear, bicubic
    - [x] The AGG-backed affine path must preserve the editor-facing interpolation choices rather than collapsing them to a single filter mode
  - [x] Undo/redo and history semantics must remain unchanged
    - [x] The migration is a rendering-backend change only; command behavior, snapshots, and history entries must remain identical

### Phase X.5: AGG Affine Free-Transform Replacement

**Goal:** Replace the manual affine resampling path in `internal/engine/transform.go` with AGG-backed image transformation so affine, distort, and warp all use the same image pipeline.

**Acceptance criterion:** The affine branch of `applyPixelTransform` no longer uses the per-pixel inverse-mapping loop and instead renders through AGG-backed dispatch with benchmarked parity or improvement.

- [x] Migration shape agreed before implementation:
  - [x] The affine path should render through AGG, not through the current per-pixel inverse-mapping loop
  - [x] The correct AGG abstraction for the affine case is destination-parallelogram image mapping, not perspective quad mapping
  - [x] Distort and warp remain on their existing AGG paths; X.5 only replaces the affine branch
- [x] Step 1: switch affine rendering over from manual inverse mapping to AGG-backed dispatch
  - [x] The affine-only inverse-mapping loop was removed from `applyPixelTransform`
  - [x] General affine rendering now goes through `TransformImageParallelogram`
  - [x] `DistortCorners` and `WarpGrid` remained on their existing AGG paths during the migration
  - [x] The existing function signature and `LayerBounds` contract were preserved
- [x] Step 2: bind editor interpolation modes to AGG filter configuration
  - [x] `InterpolNearest` → `ImageFilter(NoFilter)`
  - [x] `InterpolBilinear` → `ImageFilter(Bilinear)`
  - [x] `InterpolBicubic` → `ImageFilter(Bicubic)`
  - [x] AGG resample mode is now configured explicitly with `ImageResample(NoResample)` for deterministic affine output
- [x] Step 3: add focused affine benchmark coverage and profile it directly
  - [x] `render_benchmark_test.go` now includes `AffineTransformCommit` sub-benchmarks for nearest, bilinear, and bicubic
  - [x] The benchmark suite also includes dedicated simple-case measurements:
    - [x] `AffineTransformCommitIntegerTranslate`
    - [x] `AffineTransformCommitAxisAlignedScale`
  - [x] CPU and allocation profiles were captured for the affine benchmark path rather than inferred from full-frame render timing
- [x] Step 4: add cheap non-general-affine fallbacks so parallelogram rendering is used only when needed
  - [x] Pure integer translation now falls back to direct pixel copy
  - [x] Axis-aligned positive scale/translate now falls back to `TransformImageSimple`
  - [x] General affine with rotation, shear, or mirrored axes continues to use `TransformImageParallelogram`
- [x] Step 5: reduce wrapper-side allocation churn around AGG affine rendering
  - [x] Transform output now reuses a scratch pixel buffer instead of allocating a fresh destination slice on every apply
  - [x] Free-transform state now reuses the `Agg2D` renderer and source `Image` across preview/commit applications
  - [x] This preserved correctness while removing most of the wrapper-side allocation volume from the affine path
- [x] Step 6: record measured before/after results for the migration work completed so far
  - [x] Initial AGG affine benchmark after replacing the manual path:
    - [x] General affine: ~7.2–7.7 ms/op, ~1.56 MB/op, ~1449 allocs/op
  - [x] After scratch-buffer reuse:
    - [x] General affine: ~7.1–7.4 ms/op, ~233 KB/op, ~1448 allocs/op
  - [x] After engine-side simple-case dispatch plus `Agg2D`/image reuse:
    - [x] General affine: ~7.1–7.3 ms/op, ~34.5 KB/op, ~1381 allocs/op
    - [x] Integer translate fast path: ~51–54 µs/op, 0 B/op, 0 allocs/op
    - [x] Axis-aligned scale fast path: ~7.1 ms/op, ~29.6 KB/op, ~1177 allocs/op
  - [x] Current conclusion:
    - [x] The migration did not materially change general-affine CPU time
    - [x] The migration and reuse work massively reduced affine allocation volume
    - [x] Remaining general-affine cost is now concentrated inside AGG rasterizer/image-sampling internals rather than engine-side setup
- [x] Follow-up cleanup still open before X.5 can be considered fully closed
  - [x] Extract the shared AGG transform setup out of the inline closure/branch structure into dedicated helpers
  - [x] Add explicit edge/border parity tests for partially out-of-bounds transforms
  - [x] Remove now-dead affine-only helpers if no remaining caller needs them (`inverseTransformPoint`, `sampleOriginal`, related manual-affine helpers)

### Phase X.6: Transform Pipeline Unification & Cleanup

**Goal:** Reduce divergence between affine, perspective, and warp transform code so all AGG-backed transform paths share setup, interpolation configuration, image creation, and bounds handling.

**Acceptance criterion:** Affine, distort, and warp go through a shared AGG-oriented transform pipeline with minimal duplicated setup logic.

- [x] Refactor shared AGG transform setup in `internal/engine/transform.go`:
  - [x] Shared renderer/image reuse now exists in free-transform state (`ScratchRenderer`, `ScratchSource`)
  - [x] Shared interpolation/filter selection now lives in one AGG setup path inside `applyPixelTransform`
  - [x] Shared destination quad/parallelogram conversion helpers now exist via `tileQuad` / `affineParallelogram` on the render target helper
  - [x] Shared output-bounds and tile-relative coordinate helpers now exist via `computeTransformRenderTarget`
- [x] Make interpolation behavior consistent across transform modes
  - [x] Added a direct `agg_go` reproducer in `internal/engine/agg_affine_repro_test.go` to separate engine-wrapper behavior from AGG behavior
  - [x] The reproducer currently shows `TransformImageSimple` and `TransformImageParallelogram` producing byte-identical nearest, bilinear, and bicubic outputs for the tested affine destinations even when called outside the engine wrapper
  - [x] After switching `packages/engine-wasm` to a local `agg_go` replace for debugging, the reproducer still shows nearest and bilinear collapsing in the AGG affine `NoResample` path for both smooth-gradient and high-frequency fixtures
  - [x] The same reproducer shows `ResampleAlways` is sufficient to separate bilinear and bicubic, which narrows the remaining issue to nearest/bilinear semantics and the non-resampling affine path
  - [x] Local `agg_go` trace logging now shows nearest and bilinear receive the same raw interpolated coordinates in the affine `NoResample` path; bilinear then subtracts the half-pixel filter offset, frequently shifting the base cell to negative or edge-adjacent coordinates where clamped edge samples dominate the weights
  - [x] Temporary engine-side mitigation: affine commits now keep nearest on `NoResample` but route bilinear and bicubic through `ResampleAlways`, leaving warp/distort unchanged while the underlying AGG nearest-vs-bilinear `NoResample` semantics are still under investigation
  - [x] Resolved: `agg_go` v0.2.10 added `AffineImageResamplePolicy` with `PreferFiltered` mode; the engine uses this via `affineTransformResamplePolicy()` so bilinear/bicubic route through the filtered resampler while nearest uses the original `Agg2D` path — reproducer confirms all three modes now produce distinct output in both `NoResample` and `ResampleAlways`
- [x] Eliminate duplicated AGG wiring between affine, distort, and warp branches where practical
  - [x] Introduced `transformStrategy` struct pairing per-mode resample config with a render function
  - [x] `selectTransformStrategy` returns the right config (warp/distort use `AffineImageResampleAgg2D`; affine uses `PreferFiltered` for non-nearest)
  - [x] Render functions (`renderWarpQuads`, `renderDistortQuad`, `renderAffineImage`) now receive a pre-configured `Agg2D` renderer — no setup logic inside
  - [x] `applyPixelTransform` uses a single setup-then-render path; affine fast-paths (degenerate det, pure integer translate) are checked before AGG setup
- [x] Keep clear separation between transform data-model math and pixel-rendering implementation
  - [x] Data-model math (matrix operations, corners, AABB, meta, record/replay) lives on `FreeTransformState` methods and standalone helpers
  - [x] Pixel rendering is in the three strategy render functions, dispatched via `selectTransformStrategy`
  - [x] Manual samplers (`sampleBilinear`, etc.) remain for viewport compositing and crop — these are orthogonal to the transform commit pipeline

### Phase X.7: Discrete Transform & Overlay Policy

**Goal:** Explicitly decide which transform-related code should remain manual because it is exact, simpler, or cheaper than routing through AGG.

**Acceptance criterion:** The plan records a deliberate policy for discrete transforms and overlays instead of treating all manual transform code as technical debt.

- [x] Evaluate and decide for discrete transforms in `internal/engine/transform.go`:
  - [x] `flipPixelsH` — **keep manual**: exact 1:1 pixel swap, ~10 lines, O(n) single alloc; AGG would need renderer setup + affine matrix for a byte-identical result with rasterizer overhead
  - [x] `flipPixelsV` — **keep manual**: same rationale as flipH
  - [x] `rotatePixels90CW` — **keep manual**: exact index remapping with dimension swap; AGG cannot guarantee bit-exactness for 90° rotation and would require filter/resample configuration for a lossless operation
  - [x] `rotatePixels90CCW` — **keep manual**: same rationale as 90CW
  - [x] `rotatePixels180` — **keep manual**: trivial reverse-index loop; simpler and faster than any AGG path
- [x] Keep manual implementations: all five are exact, allocation-efficient, and faster than AGG setup — no interpolation or anti-aliasing is involved, so AGG adds cost without benefit
- [x] Evaluate transform overlay rendering separately from image transforms:
  - [x] `RenderTransformHandlesOverlay` — **keep manual**: draws pixel-crisp UI elements (bounding box, corner/edge handles, rotation handle, pivot crosshair) directly into a pre-existing overlay buffer; AGG would produce anti-aliased lines/circles that make handles look blurry at screen resolution; the overlay compositing model (separate buffer, blended on top) doesn't benefit from AGG's rasterizer pipeline

### Phase X.8: Transform Replacement Validation & Benchmarking

**Goal:** Prove the AGG replacement is correct and faster, rather than assuming architectural cleanup alone improves the engine.

**Acceptance criterion:** Dedicated transform benchmarks/tests exist or are expanded, and before/after measurements are recorded for the migrated affine path.

- [x] Add or extend focused benchmarks for transform commit paths:
  - [x] Affine free transform
  - [x] Perspective transform — ~8.7 ms/op (nearest), ~8.9 ms/op (bilinear), ~9.5 ms/op (bicubic); 576 allocs/op, ~18 KB/op
  - [x] Warp transform — ~10.3 ms/op (nearest), ~12.0 ms/op (bilinear), ~10.8 ms/op (bicubic); 1803 allocs/op, ~60 KB/op (9 perspective patches)
  - [x] Discrete rotate/flip — all ~0.65–1.1 ms/op at 512×512; 1 alloc/op, ~1 MB/op (output buffer only); rotate180 fastest at ~0.65 ms; confirms X.7 policy that discrete ops are far cheaper than AGG setup
- [x] Add correctness tests for interpolation parity and output bounds stability
  - [x] Output bounds stability now has direct regression coverage for integer-translate and negative-offset axis-aligned affine cases
  - [x] Add explicit interpolation-parity fixtures across nearest, bilinear, and bicubic at the sampler level using a representative fixture and multiple subpixel sample coordinates
  - [x] Transform-level tests now verify that nearest, bilinear, and bicubic preserve bounds/output shape stability for both axis-aligned and general affine AGG-backed paths
  - [x] Added a direct AGG reproducer test for affine image transforms so interpolation collapse can be verified without going through the engine wrapper
  - [x] Added an engine-side regression that requires affine bilinear and bicubic outputs to diverge for at least one representative candidate under the `AffineImageResamplePreferFiltered` policy
- [x] Capture CPU and allocation profiles before and after AGG affine migration
- [x] Record whether AGG affine replacement is actually faster than the previous manual path
  - [x] Result so far: the main win is allocation reduction and path simplification; general-affine CPU time remains dominated by AGG internals

### Phase X.9: Viewport Rendering Optimization

- [x] Refine the viewport profile with isolated stage benchmarks:
  - [x] `RenderViewportAggBase`: ~8.65 ms/op, ~1.15 MB/op, ~40.5k allocs/op
  - [x] `RenderViewportAggOverlays`: ~0.52 ms/op, ~272 KB/op, ~1.1k allocs/op
  - [x] `RenderViewportAggOnly`: ~8.44 ms/op, ~1.42 MB/op, ~41.7k allocs/op
  - [x] `RenderViewport` (full): ~20.3–22.0 ms/op, ~1.42 MB/op, ~41.7k allocs/op
- [x] Record the current conclusion for X.9:
  - [x] Full viewport CPU time is dominated more by this repository's `compositeDocumentToViewport` / `sampleBilinear` path than by AGG background drawing
  - [x] Viewport allocations are dominated by the AGG path (`RenderViewportBase` / checkerboard / border), not by `sampleBilinear`
  - [x] In the focused full-viewport CPU profile, `sampleBilinear` alone is the single biggest hotspot
  - [x] In the focused AGG-only profile, checkerboard rectangles dominate through AGG scanline/rasterizer work (`qsortCellsByX`, `SortCells`, `RenderScanlineAASolid`)
- [x] Reduce viewport cost in `internal/engine/viewport_composite.go`
  - [x] Profile and optimize `sampleBilinear`: inlined bilinear sampling with fixed-point (8-bit fractional) integer weights, eliminating `math.Floor`, `float64` channel multiplications, and `[4]byte` return copies
  - [x] Reduce CPU overhead from `txPixelAt`, `clampFloat`, and repeated bilinear math: eliminated all three from the viewport hot path; direct buffer index arithmetic with fast interior check that skips clamping for non-edge pixels
  - [x] Preserve near-zero allocation behavior: confirmed 0 new allocations (41,667 allocs/op unchanged, all from AGG background)
  - [x] Specialized fast path for unrotated viewports (`compositeViewportBilinearUnrotated`): hoists Y weights and row offsets out of the inner loop since docY is constant per scanline; rotated path (`compositeViewportBilinearRotated`) also uses inlined fixed-point sampling
  - [x] Re-run benchmark and pprof after optimization:
    - Before: `RenderViewport` ~10.0 ms/op; `compositeDocumentToViewport` 48.2% cum, `sampleBilinear` 22.4% flat
    - After: `RenderViewport` ~7.3 ms/op (**27% faster**); `compositeDocumentToViewport` 26.0% cum, bilinear sampling 14.9% flat
    - Bottleneck has shifted to AGG checkerboard rendering (~66% cum) — that is Phase X.10

### Phase X.10: Background Rendering Optimization

- [x] Reduce background rendering cost in `internal/agg/agg.go`
  - [x] Avoid redrawing the checkerboard and document shell every frame: added `viewportBaseKey` cache on the `instance` struct keyed on document dimensions, background type, and all viewport transform inputs (center, zoom, rotation, canvas size); on cache hit the pre-rendered background is `copy()`'d instead of re-rasterizing ~460 AGG rectangles
  - [x] Caching approach chosen over pre-rendering: a `cachedViewportBase` buffer stores the last `RenderViewportBase` output and is reused when the key matches; cache auto-invalidates when any input changes — no manual invalidation needed
  - [x] Re-run benchmark after background-rendering optimization:
    - `RenderFrameCachedComposite` (pan/zoom/idle frames): **~20 ms → 2.6 ms/op (87% faster)**, allocs **41.7k → 1.2k/op (97% fewer)**
    - `RenderViewport` (standalone, no cache): ~7.5 ms/op unchanged — the cache only benefits the `instance.render()` path
    - `RenderFrameAfterPaint` (cache-miss frame after content change): ~23 ms/op unchanged — correctly falls through to full AGG render

### Phase X.11: Brush Stroke Optimization

- [x] Reduce brush stroke allocation pressure in `internal/engine/brush.go`
  - [x] Reuse AGG renderer across dabs within a stroke: cached `*agglib.Agg2D` on `activePaintStroke`, created once at stroke begin; `paintDabReuse` calls `Attach` per dab (resets transforms/state) but the rasterizer keeps its pre-allocated cell blocks
  - [x] Batching evaluation: not needed — renderer reuse alone eliminated the dominant cost; the rasterizer cell allocations dropped from 82% → 7.7% of total alloc; remaining per-dab allocations are `Attach` pixel-format setup (~2 KB/dab) and internal agg_go span/scanline objects — further gains require agg_go library changes
  - [x] Re-run benchmark after brush optimization:
    - `PaintStrokes`: **16.4 ms → 8.3 ms/op (49% faster)**, allocs **37.2k → 19.6k/op (47% fewer)**, memory **46.3 MB → 5.3 MB/op (89% less)**
    - Remaining top allocators: `handleBeginPaintStroke` undo copy (59%) and `copyDirtyRect` (23%) — both Phase X.12

### Phase X.12: Stroke-Start Memory Optimization

- [x] Reduce stroke-start memory overhead in `internal/engine/engine.go`
  - [x] Replaced full-layer `beforePixels` copy with lazy row-level copy-on-write (`saveRowsBeforeDab`)
  - [x] Dirty-rect-bounded stroke history: only the rows the dirty rect touches are snapshotted before each dab paints, instead of copying the entire layer upfront
  - [x] Reusable buffer (`instance.undoRowBuf`) avoids per-stroke allocation — buffer grows once and is reused across strokes
  - [x] Added `newPixelDeltaFromRows` in `pixel_delta.go` to build undo deltas from the row-bounded snapshot
  - [x] Benchmark results (PaintStrokes, 512×512):
    - Before: ~8.3 ms/op, 5.31 MB/op, 19,593 allocs/op
    - After: ~6.7 ms/op, 2.04 MB/op, 13,935 allocs/op
    - **19% faster, 62% less memory, 29% fewer allocations**
  - [x] `pprof` confirms `handleBeginPaintStroke` eliminated from allocation top-10 (was 59.7% / 1041 MB → gone)
  - [x] `saveRowsBeforeDab` appears at 3.4% (32.5 MB) — 97% reduction from the original full-layer copy
  - [x] Remaining top allocator: `copyDirtyRect` at 58.9% — inherent to undo storage (before+after dirty rect data persisted in history)

### Phase X.13: Performance Regression Tracking

- [ ] After each optimization pass, update this phase with before/after benchmark numbers for `PaintStrokes`, `CompositeSurface`, `RenderViewport`, `RenderFrameCachedComposite`, and `RenderFrameAfterPaint`
- [x] Track transform-specific before/after numbers alongside the global render benchmark once the AGG replacement work starts
  - [x] Affine free-transform commit is now benchmarked directly in `render_benchmark_test.go`
  - [x] Before/after data recorded in X.5 shows large allocation wins from scratch-buffer reuse, simple-case dispatch, and `Agg2D` reuse
- [x] Add a dedicated zoom-stress benchmark for heavily zoomed viewports
  - [x] `render_benchmark_test.go` now includes `BenchmarkViewportZoomScenarios512`
  - [x] Benchmark command:
    - [x] `go test ./internal/engine -run '^$' -bench '^BenchmarkViewportZoomScenarios512$' -benchmem`
  - [x] Native Go zoom-stress baseline measured on Linux / Intel i7-1255U:
    - [x] `Zoom100/CompositeDocumentToViewport`: ~1.92 ms/op, 0 B/op, 0 allocs/op
    - [x] `Zoom100/RenderViewportAggBase`: ~4.19 ms/op, ~398 KB/op, ~17.0k allocs/op
    - [x] `Zoom100/RenderViewportAggOverlays`: ~0.32 ms/op, ~240 KB/op, ~120 allocs/op
    - [x] `Zoom100/RenderViewport`: ~6.73 ms/op, ~638 KB/op, ~17.1k allocs/op
    - [x] `Zoom1000/CompositeDocumentToViewport`: ~0.64 ms/op, 0 B/op, 0 allocs/op
    - [x] `Zoom1000/RenderViewportAggBase`: ~0.90 ms/op, ~197 KB/op, ~472 allocs/op
    - [x] `Zoom1000/RenderViewportAggOverlays`: ~7.90 ms/op, ~1.60 MB/op, ~133 allocs/op
    - [x] `Zoom1000/RenderViewport`: ~9.68 ms/op, ~1.80 MB/op, ~605 allocs/op
    - [x] `Zoom1000/RenderFrameCachedComposite`: ~9.23 ms/op, ~2.65 MB/op, ~149 allocs/op
  - [x] Current conclusion:
    - [x] At `1000%` zoom the document compositing path is faster, not slower, because nearest-neighbour sampling and reduced background coverage lower its cost
    - [x] The dominant regression is `RenderViewportAggOverlays`, which currently spends most of the frame budget drawing the transformed document border through AGG at extreme zoom
    - [x] Next optimization target for zoomed-in navigation should be the overlay pass, not `compositeDocumentToViewport`
  - [x] Focused `pprof` for the zoomed overlay path:
    - [x] CPU profiling command:
      - [x] `go test ./internal/engine -run '^$' -bench '^BenchmarkViewportZoomScenarios512/Zoom1000/RenderViewportAggOverlays$' -benchtime=2s -cpuprofile /tmp/agogo-zoom1000-overlays.cpu`
      - [x] `go tool pprof -top /tmp/agogo-zoom1000-overlays.cpu`
    - [x] Allocation profiling command:
      - [x] `go test ./internal/engine -run '^$' -bench '^BenchmarkViewportZoomScenarios512/Zoom1000/RenderViewportAggOverlays$' -benchtime=2s -memprofile /tmp/agogo-zoom1000-overlays.mem`
      - [x] `go tool pprof -top -alloc_space /tmp/agogo-zoom1000-overlays.mem`
    - [x] CPU hotspot summary:
      - [x] `RenderViewportOverlays` spends ~98% cumulative CPU time under `renderDocumentBorder`
      - [x] `github.com/cwbudde/agg_go/internal/scanline.(*ScanlineU8).AddSpan` is ~83.7% flat CPU
      - [x] The rest is AGG rasterizer sorting/cell generation (`qsortCellsByX`, `SortCells`, `SweepScanline`)
    - [x] Allocation hotspot summary:
      - [x] ~96% of allocation space is inside `RenderViewportOverlays` → `renderDocumentBorder`
      - [x] The dominant allocators are AGG rasterizer cell/block growth (`RasterizerCellsAASimple.allocateBlock`) and scanline backing-store resizing
      - [x] This confirms the zoomed-in regression is the anti-aliased stroked border path, not document-pixel compositing
  - [x] Zoomed overlay optimization implemented in `internal/agg/agg.go`
    - [x] High-zoom document border rendering now switches from a transformed world-space AGG rectangle to clipped screen-space border segments
    - [x] The border remains AGG-rendered, but only for the visible on-canvas segments instead of rasterizing a huge off-screen stroke
    - [x] Post-fix benchmark results (`BenchmarkViewportZoomScenarios512`):
      - [x] `Zoom1000/RenderViewportAggOverlays`: ~7.90 ms/op, ~1.60 MB/op, ~133 allocs/op → **~10 µs/op, ~10 KB/op, 63 allocs/op**
      - [x] `Zoom1000/RenderViewport`: ~9.68 ms/op, ~1.80 MB/op, ~605 allocs/op → **~1.78 ms/op, ~207 KB/op, ~535 allocs/op**
      - [x] `Zoom1000/RenderFrameCachedComposite`: ~9.23 ms/op, ~2.65 MB/op, ~149 allocs/op → **~0.93 ms/op, ~1.06 MB/op, 78 allocs/op**
    - [x] Current conclusion:
      - [x] The pathological cost at extreme zoom was the border rasterization strategy, not an unavoidable consequence of high zoom itself
      - [x] With clipped screen-space border rendering, heavily zoomed viewport navigation is back in the same rough cost class as the underlying image/base passes
  - [x] Browser/Wasm zoom profile captured after the native overlay fix
    - [x] Added reproducible browser profiling helpers:
      - [x] [browser-zoom-profile.html](/mnt/projekte/Code/Agogo-Web/apps/editor-web/public/browser-zoom-profile.html)
      - [x] [profile_browser_zoom1000.mjs](/mnt/projekte/Code/Agogo-Web/scripts/profile_browser_zoom1000.mjs)
    - [x] Browser profiling workflow:
      - [x] `just wasm-build`
      - [x] `bun run --cwd apps/editor-web dev --host 127.0.0.1 --port 4173`
      - [x] `node scripts/profile_browser_zoom1000.mjs`
    - [x] Headless Chrome / Linux baseline for the 512×512 painted document at `1000%` zoom:
      - [x] `renderFrameOnly`: **~3.60 ms/op**
      - [x] `pixelCopyOnly`: **~0.33 ms/op**
      - [x] `putImageDataOnly`: **~0.04 ms/op**
      - [x] `endToEnd`: **~4.62 ms/op**
    - [x] Current conclusion:
      - [x] After the overlay fix, browser/Wasm overhead is now dominated by the Wasm `RenderFrame()` call itself rather than the JS-side pixel copy or canvas blit
      - [x] The JS pixel copy is measurable but secondary; `putImageData` is comparatively cheap in this 512×512 scenario
      - [x] Browser/Wasm execution is now roughly ~2.6× the native `Zoom1000/RenderViewport` cost, which is a credible remaining Wasm/runtime tax rather than a pathological engine hotspot
  - [x] Removed the extra JS-side pixel copy from the real canvas render loop
    - [x] `apps/editor-web/src/components/editor-canvas.tsx` now passes the Wasm-backed `Uint8ClampedArray` directly into `ImageData` and frees the engine buffer only after `putImageData` returns
    - [x] Browser repro page was updated to match the production no-copy path before remeasurement
    - [x] `bun run --cwd apps/editor-web typecheck` passes after the change
    - [x] Updated browser/Wasm baseline after the no-copy change:
      - [x] `renderFrameOnly`: ~3.66 ms/op
      - [x] `pixelCopyOnly`: ~0.36 ms/op (still measurable in isolation, but no longer on the hot end-to-end path)
      - [x] `putImageDataOnly`: ~0.05 ms/op
      - [x] `endToEnd`: **~3.77 ms/op** (down from **~4.62 ms/op**)
    - [x] Current conclusion:
      - [x] Removing the JS copy cut the browser end-to-end cost by roughly **18%**
      - [x] Browser overhead is now much closer to pure Wasm `RenderFrame()` cost, which confirms the copied RGBA staging buffer was a real but tractable frontend tax
  - [x] Browser `RenderFrame()` bridge overhead profiled directly
    - [x] Browser profiling helpers now split:
      - [x] raw `window.RenderFrame(handle)` string-return path
      - [x] `JSON.parse` on a representative `RenderResult`
      - [x] wrapped `handle.renderFrame()` path
    - [x] Headless Chrome / Linux bridge-stage baseline at `1000%` zoom:
      - [x] `renderFrameRawOnly`: **~3.83 ms/op**
      - [x] `jsonParseOnly`: **~0.003 ms/op**
      - [x] `renderFrameOnly`: **~4.03 ms/op**
      - [x] `endToEnd`: **~4.53 ms/op**
    - [x] Current conclusion:
      - [x] `JSON.parse` is negligible here; it is not the remaining bottleneck
      - [x] The dominant browser/Wasm tax is the raw `RenderFrame` bridge itself: Go `render()` + Go `json.Marshal` + `syscall/js` string transfer back into JS
      - [x] The next serious browser-side optimization target is reducing or eliminating the per-frame JSON/string bridge, for example by exposing a compact raw metadata ABI for steady-state `RenderFrame()` calls
  - [x] Added a compact steady-state `RenderFrameRaw` ABI for the hot canvas loop
    - [x] Go engine now exposes `RenderFrameRaw(handle)` returning only `frameId`, `viewport`, `bufferPtr`, `bufferLen`, and later an explicit `reused` bit for frontend presentation skipping
    - [x] The raw path skips `UIMeta` construction in the engine and avoids sending the full `RenderResult` JSON on steady-state frames
    - [x] Frontend continuous render loop now uses `handle.renderFrameRaw()` while command responses and initial load still use the full `RenderResult`
    - [x] Browser/Wasm measurements after the ABI split:
      - [x] legacy full `renderFrameOnly`: **~4.08 ms/op**
      - [x] hot-path `renderFrameHotOnly`: **~3.97 ms/op**
      - [x] hot-path `endToEnd`: **~4.03 ms/op**
    - [x] Current conclusion:
      - [x] The compact raw ABI provides a real but modest improvement for this scenario, bringing browser end-to-end cost down from the previous ~4.53 ms/op to ~4.03 ms/op
      - [x] The remaining cost is still mostly inside the Go/Wasm render call itself rather than frontend parsing or blitting
      - [x] Larger wins from here will require reducing engine work on steady-state frames or moving more frame metadata off the JSON/string bridge entirely
  - [x] Added idle-frame reuse in `renderRaw()` for non-animated steady-state frames
    - [x] When the document content and viewport are unchanged and there is no active selection animation, free-transform overlay, or crop overlay, the engine now reuses the previously rendered final frame instead of rerunning viewport composition and overlays
    - [x] Post-fix browser/Wasm measurements for the 512×512 painted document at `1000%` zoom:
      - [x] `renderFrameHotOnly`: **~3.97 ms/op → ~0.026 ms/op**
      - [x] `endToEnd`: **~4.03 ms/op → ~0.078 ms/op**
      - [x] legacy full `renderFrameOnly`: **~0.039 ms/op** in the same idle scenario because `render()` now reuses `renderRaw()` and only rebuilds `UIMeta`
    - [x] Current conclusion:
      - [x] The remaining steady-state browser cost in the profiled idle scenario is now dominated by the canvas upload (`putImageData`) and any optional JS-side pixel staging, not engine rendering
      - [x] The large previous Wasm cost was not intrinsic to idle `1000%` frames; it was repeated redundant engine work on unchanged frames
  - [x] Added frontend presentation skipping for reused raw frames
    - [x] `RawRenderResult` now carries a `reused` bit so the frontend can distinguish a genuinely new pixel buffer from an idle engine-cache hit without inferring it from `frameId`
    - [x] `apps/editor-web/src/components/editor-canvas.tsx` now skips `readPixels()` and `putImageData()` when the engine reports a reused frame with the same buffer pointer, byte length, and canvas size as the frame already on screen
    - [x] Browser profiling helpers now include `endToEndSkipReused`, which mirrors the production skip-on-reuse presentation path
    - [x] Updated browser/Wasm measurements for the 512×512 painted document at `1000%` zoom:
      - [x] `renderFrameHotOnly`: **~0.031 ms/op**
      - [x] `putImageDataOnly`: **~0.049 ms/op**
      - [x] baseline idle `endToEnd`: **~0.077 ms/op**
      - [x] skip-on-reuse `endToEndSkipReused`: **~0.021 ms/op**
      - [x] `presentedFrames`: **1 / 2000** in the idle benchmark loop
    - [x] Current conclusion:
      - [x] Once the engine can prove a frame is unchanged, the right browser optimization is to skip the canvas upload entirely rather than trying to micro-optimize `putImageData`
      - [x] Idle high-zoom browser cost is now mostly the tiny raw-frame call overhead; the canvas upload only occurs on the first presented frame or when content actually changes
      - [x] Animated selection marching-ants and active transform/crop overlays still opt out of this reuse path by design
- [ ] Only move on to browser/Wasm-specific tuning once the native-Go bottlenecks above have been reduced and re-measured

---

## Phase S: Stabilization, Correctness & Completion (Full-Codebase Review, 2026-07-06)

**Context:** A six-track deep review (engine core, performance, frontend, tests/CI, plan-vs-code audit, agg_go alignment) found that most backend checkboxes in Phases 1–5 are genuinely implemented and tested, but the product feels *slow, buggy and incomplete* for three systemic reasons:

1. **Architecture:** every pointer event runs a synchronous full-document recomposite + full-canvas resample + full `UIMeta` JSON round-trip + app-wide React re-render. No dirty rects reach the frontend. Snapshot-per-command history deep-clones the whole document ~4× per command.
2. **Correctness:** several flagship flows are broken end-to-end (undo doesn't repaint, undo deletes sibling documents, live text editing does nothing, filter-dialog commit is a no-op, real Photoshop PSDs cannot be opened).
3. **Wiring:** a recurring pattern of "engine done, frontend never dispatches it" (whole filter system, histogram, eyedroppers, Transform Again, jitter dynamics, half the menu bar).

**Review ratings (0–10):** engine core **4**, performance engineering **6**, frontend **4**, quality infrastructure **6.5**, plan accuracy **7.5** (engine ≈9, UI wiring ≈6), agg_go alignment **5.5**. **Overall product state: ~4.5** — consistent with the hands-on impression of 4.

**Goal:** Ship-quality stabilization. Phases S.1–S.4 are ordered by leverage; do them in order. S.5+ can proceed in parallel afterwards.

**Acceptance criterion:** main is green; painting/undo/text/filters work end-to-end; a changed-content frame costs O(dirty area), not O(document); real Photoshop PSD fixtures open correctly; no dead menu items remain (either wired or removed).

### Phase S.1: Make Main Green & CI Honest — ✅ DONE (2026-07-06)

- [x] Fix failing test `TestConvertTextToPath_ProducesGlyphOutlinePath`: implemented **for real** instead of failing safely — `appendOutlinedText` now traces GSV glyph centerlines via the public `agglib.GSVText` vertex source (available since agg_go v0.2.16; the stub's "not yet published" comment was outdated). The resulting VectorLayer is stroke-only (open centerline subpaths at `fontSize * 0.08`, matching Agg2D's GSV raster stroke width); empty text layers now error instead of being destroyed
- [x] Fix golangci-lint error `layer_styles_types.go` (`reflect.Ptr` → `reflect.Pointer`)
- [x] Fix `//nolint:unsafeptr` directives → `//nolint:govet` (unsafeptr is a govet analyzer, not a linter name)
- [x] Fixed while here: `.golangci.yml` used the invalid v1 `issues.exclusions` block (silently ignored by golangci-lint v2) — moved to `linters.exclusions`; excluded revive's file-length-limit for `_test.go` files; fixed 2 real staticcheck SA4003s in `brush_test.go` (`uint8 >= 255`); removed unused test helper — **lint is now clean including test files (CI lints tests; local `just lint` uses `--tests=false`)**
- [x] Fixed while here: 9 pre-existing `tsc -b` errors that broke the CI **build** job since April (JSX namespace in blend-if-slider, non-exhaustive switch in layer-style-model, null/readonly-tuple state setters in App.tsx, nullable path-point predicate in editor-canvas, missing `exportDocument` + vitest-4 mock typings in two test files)
- [x] Fixed while here: formatter drift that broke the CI go-format job — `treefmt.toml` invoked a bare `biome` (resolved to a global 1.9.4 instead of the workspace-pinned 2.4.9; now points at `apps/editor-web/node_modules/.bin/biome`), biome config restructured for monorepo use (root `biome.json`, app config extends it), CI formatter installs pinned (`gofumpt@v0.10.0`, `gci@v0.14.0`, `shfmt@v3.12.0` instead of `@latest`), plus the resulting one-time reformat of files never formatted with the pinned versions
- [x] Add vitest to CI (`test-vitest.yml`, wired into `ci.yml` and the build job's `needs`)
- [x] Deleted the dead `just update-golden` recipe
- [x] Add Go coverage reporting to CI (coverage profile in `test-go.yml`, total in the job summary, artifact upload)
- [x] `just ci` passes end-to-end (format → tests → lint → tidy → wasm+frontend build)

### Phase S.2: Critical Engine Correctness (data loss & broken flagship flows) — ✅ DONE (2026-07-06)

- [x] **Undo/redo of brush strokes doesn't repaint the canvas**: `pixelDeltaCommand` now carries a post-blit `bump` callback (`bumpLayerContentVersion`) that translates the delta rect into document space and bumps `doc.ContentVersion` on Apply/Undo; wired into brush stroke + Magic Eraser sites (regression: `TestPixelDeltaCommand_UndoRedoBumpsContentVersion`)
- [x] **Undo destroys all other open documents**: `restoreSnapshot` now `Replace`s only the snapshot's document in the existing manager (new generic `Manager.Replace`/`IDs` in `internal/runtime/manager.go`); nil-snapshot restore is non-destructive (regression: `TestUndoPreservesOtherOpenDocuments`)
- [x] **Live text editing mutates a discarded clone**: `enterTextEditMode`/`textEditInput` use `activeMut()`; commit compares against the recorded pre-edit original (side table) and reverts before `executeDocCommand` so history captures the true original→new transition; useless `_ = result` test replaced with real assertions
- [x] **Filter-dialog commit applies the filter with nil params**: `filterPreviewState.Params` added, updated on every preview dispatch; commit re-applies at full resolution with the last-previewed params and stores them in `lastFilter` for Ctrl+F (regression: `TestCommitFilterPreviewAppliesLastPreviewedParams`)
- [x] **Text/vector raster geometry contract is self-contradictory**: canonical contract is now **bounds-local** `CachedRaster` (`Bounds.W×Bounds.H`, composited at `Bounds.X/Y`), documented on the model fields; `rasterizeTextLayer` renders bounds-local (was doc-sized with position baked in → text drew at 2×(X,Y)); `convertTextToPath` outline layer gets doc bounds. Known caveat: center/right-aligned point text clips at the layer's left edge until tight-bounds computation lands (S.6)
- [x] **Crop / Canvas Size only transform PixelLayers**: both paths now remap ALL layer kinds — text position + re-rasterize, vector anchors/bezier handles + doc-sized raster re-rasterize, raster/vector/adjustment masks via shared `remapDocMask`, artboard bounds, adjustment caches invalidated; `Selection`/`LastSelection`/`SavedSelections` translated, clipped, deselected when fully outside (tests in `crop_remap_test.go` incl. undo)
- [x] Stop swallowing composite/render errors: `renderCompositeSurfaceChecked` propagates errors, failed composites are never cached (recovery verified without version bump), errors flow to the frontend via new `RenderResult.Error` JSON field; `importProject` surfaces real zip/PSD parse errors instead of "unsupported import payload"
- [x] Real instance disposal over the WASM ABI: existing JS `Free(ptr)` kept (per-frame buffer no-op the frontend already calls); new `EngineFree(handle)` export wired to `engine.Free` and called from the frontend `EngineHandle.dispose()` on provider teardown; all `js.FuncOf` exports now bounds/type-check `args[]` (malformed JS calls return error results, never panic)
- [x] Unknown command IDs are an error in **all** domains: every `DispatchCommand` branch returns `unsupported <domain> command id 0x…` on unhandled IDs; dispatcher exhaustiveness verified (only real gaps are reserved Path/SelectionPaint ranges, covered by `dispatch_unsupported_test.go`)

### Phase S.3: History Architecture Replacement — ✅ DONE (2026-07-06)

- [x] **Pointer-snapshot history** (replaces the 4×-deep-clone snapshot-per-command): `captureSnapshot` now returns the stored document POINTER under an immutability-by-replacement invariant (snapshot commands only ever displace the stored doc via the new `Manager.ReplaceActiveNoClone`; in-place mutations are delta-tracked or fully reverted — audited table in commit); `restoreSnapshot` installs a clone and first reverts in-flight preview mutations (active brush stroke, filter preview, live text edit) to close the mid-preview undo/redo hole. Benchmark (rename, 1024×1024×3 layers): 16.2 ms → 5.6 ms/op, 50.3 MB → 12.6 MB/op (exactly 4→1 doc clones). Audit also fixed two latent bugs: MagicErase mutated a discarded `Active()` clone (screen no-op + history divergence); non-floating free-transform commit captured the last preview frame as its before-snapshot (undo rewound to the preview). Full delta-based history for all commands remains possible future work (COW pixel buffers), but the OOM/latency cliff is gone. Stress test: `TestHistoryStressPointerSnapshotsWithStrokes`
- [x] Navigation removed from history: ZoomSet/PanSet/RotateViewSet/FitToView mutate the viewport directly (no history entries), pan-drag transaction removed, and `restoreSnapshot` no longer restores the viewport — undo/redo leaves the user's view alone (Photoshop semantics)
- [x] History failure semantics: `Undo`/`Redo`/`JumpTo` are peek-then-commit — on error both stacks are left untouched and the operation is retryable; failed brush-delta builds now return an error over the ABI and clear the redo stack (document diverged) instead of silently creating a non-undoable edit
- [x] No-op commands are not pushed: `Execute` skips commands whose before/after snapshots are equal (`SnapshotCommand.Before()` added); `documentsEqual` excludes `ModifiedAt`/`ContentVersion` (bumped even by no-op mutations) and gates pixel-byte comparison on ContentVersion equality — no per-command memcmp of pixel buffers on the hot path

### Phase S.4: Rendering Performance Architecture (the actual "slow") — ✅ DONE (2026-07-06)

> Phase X optimized constants inside a full-repaint architecture. This phase changed the architecture. Old baseline: content-changing frame ~128–139 ms native on a 2000×1500 doc (composite + full-canvas resample); after S.4 the same paint frame is **~160–310 µs** (~500–800×), 64 B/op, 3 allocs. Executed via 6 subagents (4 parallel + 2 sequential ABI changes); all paths gated on byte-identical equivalence tests; `just ci` green.

- [x] **Batch pointer input** — `ContinuePaintStroke` gained an optional `points[]` payload (`PaintStrokePoint`, backward compatible; engine iterates via shared `continuePaintStrokePoint` helper, one `bumpContentVersionRect` per batch). Frontend accumulates `getCoalescedEvents()` samples and flushes once per rAF as one multi-point command; move-phase `PointerEvent` is latest-wins per rAF; `flushPendingInput()` runs before any other dispatch (pointerdown/up/leave, airbrush tick) to preserve command order. Equivalence test: `TestContinuePaintStroke_BatchEqualsSinglePoints` (byte-identical to N single dispatches).
- [x] **Minimal ack instead of full render + `UIMeta` per `DispatchCommand`** — hot commands (`ContinuePaintStroke`, move-phase `PointerEvent`) skip `inst.render()` entirely and return a ~292 B ack (viewport, cursorType, statusText, uiMetaVersion, contentVersion) vs 32 KB before: **~110× smaller, ~130× faster marshal** (`BenchmarkDispatchResponseMarshal`, 50 layers + 100 history entries). `RenderResult.UIMeta` is now `*UIMeta` + omitempty; `instance.uiMetaVersion` bumps on every non-hot command (and ImportProject); hot commands never bump mid-gesture — the gesture-ending command delivers the refresh. context.tsx merges acks (same uiMeta reference; **zero React re-renders for a no-change brush frame**) and refetches full UIMeta at most once per rAF when the version goes stale. App.tsx needed zero changes.
- [x] **Dirty-rect rendering end-to-end** (subsumes Phase 8.2) — three tiers, each byte-identical to a full render (randomized equivalence tests in `render_incremental_test.go` / `render_partial_viewport_test.go`): (1) incremental doc recomposite — `recompositeSurfaceRect` zeroes + recomposites only the dirty rect in place in the cached surface, clip-rect threaded through the whole compositing stack; styled layers stay incremental (effect surfaces are backdrop-independent), visible adjustment layers bail to full (cache-corruption hazard, documented); full recomposites also reuse the cached buffer in place; (2) partial viewport resample — doc dirty rect projected to canvas rect, base rows copied from `cachedViewportBase`, doc composited clipped; gated on unchanged viewport key, no overlays intersecting, `dirtyCompositeBase` version guard (protects against stale dirty rects on undo-restored clones — hazard found and fixed); (3) real `DirtyRects` on `RawRenderResult` + partial `putImageData` in the blit loop. Sampling loops made bit-exact for mid-row clipped passes (absolute-coordinate computation instead of float accumulation).
- [x] Composite scratch-buffer reuse — package-level `sync.Pool` (`acquireSurface`/`releaseSurface`, zeroed on acquire, ownership rule documented) for transient sites (group-layer temps incl. recursion); the escaping headline site (`renderLayersToSurfaceWithOptions` → `cachedDocSurface`) is solved by the in-place reuse from the dirty-rect work; merge/flatten/PSD callers still allocate (results escape by design).
- [x] Row-copy fast path for zoom=1.0/rotation=0/integer alignment (`compositeViewportIdentity`) — ~7× vs bilinear at 512² (0.50 ms vs 3.52 ms), also fixes the slight blur at 100% zoom; subpixel pans correctly stay bilinear.
- [x] Alpha-weighted bilinear (`bilinearPremultSample`) — interpolates premultiplied then un-premultiplies; kills dark fringes at layer/doc edges over the checkerboard at zoom < 4×; bit-identical for fully-opaque taps; applied to unrotated + rotated variants.
- [x] Marching ants on cached frame — `cachedAnimBase` holds the frame minus the animated overlay; selection frames are now memcpy + ants stamp (`Reused=false` so the frontend still blits): ~2× faster on an empty doc and **0 allocs vs ~8.3 MB garbage/frame** (≈0.5 GB/s at 60 fps eliminated); real-layer docs win far more since the avoided composite scales with content. Zero-copy no-selection reuse untouched.
- [x] `ImageData` reuse in the blit loop (recreated only on canvas resize) + `Agg2D` reuse via `sync.Pool` in internal/agg (agg_go's `Attach()` verified to fully reset all state incl. clip box/blend mode/master alpha; `TestRenderViewportBaseReuseNoStateLeak`): `RenderViewportOverlays` 504 KB/143 allocs → **1.2 KB/69 allocs** per frame.
- [x] `debug.SetGCPercent(300)` in wasm main as interim GC-hitch mitigation (comment references this phase; revisit once real-workload profiling exists).
- [x] Re-evaluated Phase 8.1/8.3 with the new baseline: **8.1 (render worker) is no longer justified by paint latency** — the hot path is µs-scale and acks freed the main thread from per-event JSON; it would only serve heavy one-shot ops (filters on huge docs) and can wait. **8.3 (SharedArrayBuffer) deferred**: the remaining per-frame cost is one full-buffer `data.set` copy + `putImageData` upload; SAB would remove the copy but not the upload, and dirty-rect blits already shrink the effective transfer. Re-profile in the browser after S.7's React-render cleanup before investing.

> Deferred within S.4: adjustment-layer incremental compositing (needs adjustment-cache invalidation redesign); sub-rect dirty tracking for the ants path (reports full canvas); partial content update combined with an active selection (falls back to full render — correct, just not partial).

### Phase S.5: Engine Feature-Bug Fixes

- [ ] **Radial gradient is not implemented** — UI offers it; `fill_gradient.go:445` switch has no radial case and falls through to linear
- [ ] **Brush/eraser/clone strokes ignore the active selection** (`paint.go`/`brush.go` never consult `doc.Selection`; fills/gradients do it correctly via `fillRasterWithMask`)
- [ ] **Pixel/all lock never enforced for pixel edits**: only position lock is checked (`layer_ops_helpers.go:59`, TranslateLayer only) — painting/fills/filters modify fully-locked layers
- [ ] **Transforms don't transform layer masks**: free transform and discrete flips/rotates move pixels only; the mask stays behind in document space
- [ ] **FlattenLayer double-applies opacity/fill-opacity** (`layer_ops.go:805–820`): 50% opacity renders at 25% after flatten; MergeDown/MergeVisible use the other convention
- [ ] **Drop shadow falls toward the light** (`layer_styles_effects.go:274–279` — sign error at PS-default 120°) and DropShadow/OuterGlow composite **on top of** layer content instead of behind it (`layer_styles_render.go:105–116`)
- [ ] **Non-Gaussian filters mishandle alpha**: box/motion/radial/surface/median average RGB only and copy original alpha (black halo bleed, frozen silhouettes under ripple/twirl) while Gaussian blurs all four channels
- [ ] **Levels/Exposure gamma convention inverted** (`adjustments_core.go:170–172`: `v^gamma` instead of `v^(1/gamma)` — sliders work backwards) and `{outputBlack:10}` alone yields output range [10→0]
- [ ] **Mouse input paints at 75% size / 50% flow**: pressure defaults to 0.5 with PressureSize/Flow on (`brush.go:317–337`) — a 100px brush paints 75px for every mouse user; make dynamics neutral when no pressure device reports
- [ ] **drawShape pixels-mode coordinate bug** (`dispatch_shape.go:240`): doc-space path rasterized into a layer-local buffer without `-Bounds.X/Y` translation — shapes offset on any layer not at origin
- [ ] **Path booleans are not real geometry** (`path_boolean.go:38–46`): Combine/Exclude = subpath concatenation over even-odd fill, Subtract = winding reversal (no-op under even-odd), Intersect returns an error — implement via Clipper2 (also resolves the GPC licensing blocker)
- [ ] Blend-mode edge cases: color dodge/burn extremes deviate from spec (`blend.go:167–179`); `clipColor` divides by zero when lum==min → NaN pixels (`blend.go:244–256`)
- [ ] Minor batch: magic eraser bypasses the atomic version counter and dirty-rect marking (`paint.go:305`); mixer rim dabs can paint outside saved undo rows (`brush.go:879–896`); fixed-seed Add Noise doubles on reapply; Fade-with-dissolve seed bug; `meta()` never computes SkewY; MergeVisible loses hidden-layer z-order; `DeletePath` doesn't adjust `activePathIdx`; painting on non-pixel layers should surface "rasterize?" instead of silently no-op'ing; library panics in `filters_builtin_helpers.go:98` / `model/layers.go:838`; `RenderResult.PixelFormat` claims "rgba8-premultiplied" but the pipeline is straight alpha

### Phase S.6: Text & Vector Completion

- [ ] Real font engine: TrueType/OpenType via `golang.org/x/image/font/sfnt` (GSV stroke font is the only renderer; FontFamily/Bold/Italic/Kerning/BaselineShift are stored, serialized, and ignored; SmallCaps = AllCaps)
- [x] Real Create Outlines implementation — GSV centerline tracing landed with S.1 (stroke-only VectorLayer); revisit for filled TTF outlines once the sfnt font engine exists
- [ ] Render vector masks (creatable/stored but "silently ignored in rendering", `layer_ops.go:961`)
- [ ] Layer styles: user-defined gradient overlay (currently hardcoded blue→orange ramp), real pattern overlay/stroke patterns (hardcoded checkerboards), implement or remove decoded-but-unused params (Contour, Noise, Knockout, Altitude, bevel Technique)
- [ ] Real pattern fill for paint bucket (hardcoded 8px checkerboard)

### Phase S.7: Frontend Architecture & UX Repair

- [ ] **Decompose `App.tsx`** (6,229 lines, 149 `useState` hooks, ~70 props to `EditorCanvas` with fresh inline objects per render) — split state by domain, memoize `EditorCanvas` and `LayerTreeRow`, stop app-wide re-renders per engine frame
- [ ] **Engine loading/error UI**: `engine.status`/`engine.error` are rendered nowhere — a failed wasm load shows a WelcomeScreen with silently dead buttons; add toast/notification system and try/catch in `context.run()` (engine errors currently throw uncaught mid-gesture and leave transactions open)
- [ ] **`onPointerCancel`/`lostpointercapture` handling**: pen palm rejection / alt-tab currently leaves paint strokes and move/quick-select/zoom transactions dangling → corrupted history
- [ ] **Layers context menu is dead with a real pointer**: closes on window `pointerdown` without a `contains()` check, unmounting before `click` fires (`layers-panel.tsx:151–169`) — `fireEvent.click` tests mask it
- [ ] **Paths panel "activate" is a no-op** dispatching `RenamePath` with the unchanged name (`paths-panel.tsx:18–25`) — polluting undo and making footer actions hit the wrong path; add a real `SetActivePath` command
- [ ] **Gradient editor regenerates stop IDs per edit** → drags die after one move (`gradient-editor.tsx:341–350`); **Curves drag corrupts on re-sort** (`adjustments-panel.tsx:603–609`)
- [ ] **Zoom-out shortcut zooms in**: `+`/`=`/`-` all map to `CommandID.ZoomSet` and the switch compares by value, so `case get("-")` is unreachable (`keymap.ts:18–20`, `use-keyboard-shortcuts.ts:196–207`)
- [ ] Keyboard hygiene: single-key tool shortcuts fire inside modals; `HTMLSelectElement` missing from the editable-target check; Space-pan sticks on window blur; `useKeyboardShortcuts` re-registers window listeners every render (fresh `actions` object)
- [ ] Throttle slider/number-input dispatches inside a history transaction (opacity, adjustment params, curves emit one synchronous engine command + likely one undo entry per tick); fix `Number("") === 0` dispatching 0 on cleared fields
- [ ] Move autosave off the main thread (synchronous base64-zip export every 10 content versions freezes large docs; quota errors silently swallowed while the restore banner implies autosave works)
- [ ] StrictMode double-init: `loadEngine()` runs twice with no dispose — two Go runtimes, leaked handle, possible double `wasm_exec.js` injection (`context.tsx:69–96`, `loader.ts:40–66`)
- [ ] Accessibility floor: Dialog focus trap + Escape + aria; menu roles/keyboard nav; labeled icon buttons; `TextEditOverlay` Escape currently *commits* instead of canceling
- [ ] Unify styling on design tokens (character/vector panels use raw blue/slate/zinc palettes; hard-coded hexes in welcome-screen, layers-panel)
- [ ] Fix ABI payload hazards: `PointerEventCommand.button` typed in TS but silently dropped by Go; `AddLayer.pixels`/`cachedRaster` typed `number[]` in TS but `[]byte` (base64) in Go

### Phase S.8: Wire Implemented Backend Features into the UI

> Recurring audit finding: engine command exists, tested, and works — frontend never dispatches it. 24 handled-but-never-dispatched commands total.

- [ ] **Filter menu**: the entire filter domain (0x0500–0x0505, ~22 working filters, reduced-res preview, Ctrl+F reapply, Fade) has **zero frontend wiring** — the Filter menu is a static mock; add TS payload types + dialogs
- [ ] **Menu bar de-mock**: items without `actionId` are permanently disabled — Edit▸Undo/Redo/Cut/Copy/Paste, entire Layer menu, Image▸Levels/Curves/…, View▸Zoom/Fit, entire Window and Help menus; wire or remove (Edit▸Scale/Rotate/Skew/… also all mislabel plain free transform)
- [ ] **Clipboard**: no cut/copy/paste exists anywhere (needs new engine commands + UI)
- [ ] Histogram display in Levels UI (`ComputeHistogram` has no consumer); Curves black/white/gray eyedroppers (`SetPointFromSample` never dispatched); Hue/Sat range eyedropper (`IdentifyHueRange` never dispatched)
- [ ] Transform Again menu item (backend complete, zero frontend)
- [ ] Brush jitter dynamics: panel sliders are display-only — values never reach the engine, no proto fields exist
- [ ] Navigator: real document thumbnail (currently a static CSS-gradient placeholder, `App.tsx:4400`)
- [ ] Multi-document: `SwitchDocument` proto command + document tab bar (engine `DocumentManager` supports it; unreachable by users) — blocked on S.2 undo-vs-multi-doc fix
- [ ] Alt+click visibility eye = solo (claimed in 2.4, no altKey handling)
- [ ] Character panel real color picker (currently a black↔red demo toggle); vector properties real fill/stroke pickers (transparent↔black toggle); remove or implement mask Density/Feather sliders and other decorative controls
- [ ] Real `.abr` parser (current "import" is a filename-regex heuristic)

### Phase S.9: agg_go Upgrade & Alignment

- [x] **Upgrade `agg_go` v0.2.21 → v0.3.2** — done 2026-07-06; full Go test suite passes, host and js/wasm builds clean. Brings AVX2/SSE2 SIMD blend kernels, the **DstOut comp-op fix (eraser)**, `FillLinearGradientStops`, sRGB/premultiplied-alpha correctness fixes, raster-text overhaul, dashed strokes
- [ ] In agg_go: export the composite/span/image-filter primitives the engine needs (they live under `internal/` and cannot be imported — the AGENTS.md "use agg_go primitives" rule is currently impossible to follow for compositing)
- [ ] In agg_go: extend `CompOp` with the 14 missing Photoshop modes (Linear Burn/Dodge, Vivid/Pin/Linear Light, Hard Mix, Divide, Subtract, Darker/Lighter Color, Dissolve, Hue/Sat/Color/Luminosity)
- [ ] Then migrate the three highest-traffic manual-pixel paths to agg_go: `blend.go` (287-line per-pixel float64 blend engine, called per pixel from 5 files), the layer compositor (`layer_ops.go:987–1045`), and the viewport resampler (`viewport_composite.go` — replaceable by one AGG transformed-image draw)
- [ ] Migrate `renderCustomGradient` (`fill_gradient.go:429–498`) to agg_go span generators / `FillLinearGradientStops`; route crop resampling (`crop.go:369–406`) through AGG image filters
- [ ] Keep-manual list stays valid: flips/rotate90/180, discrete remaps, transform overlay (Phase X.7 policy), `EraseBackgroundDab` tolerance erase

### Phase S.10: PSD Interop Repair

> Current state: PSD I/O is effectively an Agogo↔Agogo container. Round-trip tests pass only because reader and writer share the same bugs and fidelity rides on an embedded `AgogoProject` JSON block — there are zero external fixtures.

- [ ] **Compression constants off by +2** (`internal/io/psd/types.go:10–18`: mid-block `iota` reuse yields Raw=2/RLE=3/Zip=4/ZipPred=5 vs spec 0/1/2/3) — **any genuine Photoshop file fails with "unsupported compression 1"** and written files are invalid for other apps
- [ ] Add real Photoshop-exported fixture files and round-trip tests against them
- [ ] Fix group section-divider semantics (inverted vs Photoshop → spurious/inverted group structure)
- [ ] Fix layer masks (wrong decode dims `parser.go:212`; decoded pixels never assigned `import.go:94–98`; writer discards mask offset `writer.go:107`)
- [ ] Map all 27 blend modes (only 7 mapped; soft light etc. collapse to Normal, `helpers.go:121–140`)
- [ ] Fix zip channel decode (always `ErrUnexpectedEOF`, `pixels.go:130`)
- [ ] Bound allocations on untrusted length fields (`helpers.go:166` — hostile/corrupt file OOM-kills the editor) and add `Fuzz*` tests for the parser (highest-risk untested surface: binary parsing of untrusted input in-browser)
- [ ] Reconstruct (not just capture as metadata) adjustment layers and text layers on import; replace the JSON-in-descriptor TySh/lfx2 pseudo-format with spec-conformant descriptors

### Phase S.11: Test & QA Hardening

- [ ] Golden-image render tests for the compositor/blend/styles pipeline (a *rendering engine* currently has zero pixel-snapshot tests; "golden" tests are expected-pixel tables)
- [ ] Playwright E2E: create doc → paint → undo → redo → export → hash; PSD fixture open; adjustment layer visual change (the C1–C4 class of bugs is invisible to unit tests and exactly what E2E catches)
- [ ] Direct tests for `internal/model`, `internal/document`, `internal/command`, `internal/runtime` (zero today — the clone logic and history stack that undo depends on), and the new `internal/io/psd*` packages (only covered via the engine compat shim)
- [ ] Frontend tests for `src/wasm/loader.ts`, `context.tsx`, `editor-canvas.tsx` (entire engine-integration path untested)
- [ ] Benchmark regression job in CI (2 benchmarks exist, nothing runs or compares them)

---

## ✅ Phase 0: Scaffolding, Repo Structure, Build Pipeline — COMPLETE

- **Monorepo:** Bun workspaces — `apps/editor-web` (Vite + React + TS + Tailwind v4 + shadcn + Base UI), `packages/engine-wasm` (Go 1.25 → `js/wasm`), `packages/proto` (shared TS types/command IDs)
- **Go Wasm engine:** `syscall/js` bridge; `EngineInit` / `DispatchCommand` / `RenderFrame` / `Free` exported via `js.FuncOf`; checkerboard viewport rendered through `agg_go`; build-time version stamp via `-X buildinfo.BuildTime`
- **Frontend integration:** `loadEngine()` → `EngineContext` → `<EditorCanvas />` (`putImageData` only, zero JS pixel work)
- **Tooling:** `justfile` (`dev`, `build`, `test`, `lint`, `lint-fix`, `fmt`, `check-formatted`); `treefmt.toml` (gofumpt + gci + biome + shfmt); Biome + lefthook pre-commit; golangci-lint v2
- **CI:** GitHub Actions — reusable workflows (biome, typecheck, go-test, build) on `ubuntu-latest`
- **Licenses:** `LICENSES.md` — `agg_go` needs LICENSE before public release; GPC is non-commercial only → Clipper2 replacement is a pre-release blocker

---

## Phase 1: Engine Core (Document, Viewport, Pan/Zoom) + UI Shell

**Goal:** New document, pan/zoom/rotate view, status bar, basic panels.

**Acceptance criterion:** Open empty document, navigate, change zoom levels (engine renders correctly), History shows entries.

### Phase 1.1: Document & Viewport Backend

- [x] `Document`, `ViewportState`, and `DocumentManager` implemented with document metadata, viewport state, and active-document switching.
- [x] `RenderViewport(doc *Document, vp *ViewportState) []byte` implemented with AGG-backed pan/zoom/rotate rendering, backend checkerboard compositing, and `RGBA8` output.

### Phase 1.2: UI Shell & Workspace Layout

- [x] Main workspace layout implemented: menubar, toolbar, options bar, canvas, and right-side panel dock.
- [x] Panel system implemented: resizable/collapsible dock with tab groups and initial Layers, History, Properties, and Navigator panels.
- [x] Status bar (bottom): zoom %, document dimensions, cursor position (doc-space)
- [x] Canvas resize observer: fires `devicePixelRatio`-aware resize event → sends to engine
- [x] Keyboard shortcut foundation implemented with shared keymap and default pan/zoom/fit/undo/redo shortcuts.
- [x] New Document dialog implemented with presets, px/cm/in/mm sizing, DPI, color mode, bit depth, and background settings.
- [x] Navigator panel implemented with mini-viewport UI and zoom slider.

### Phase 1.3: Command System & ABI Protocol

- [x] Define command IDs in `/packages/proto/commands.ts` (enum/const map)
- [x] Define payload schemas (TypeScript interfaces): `CreateDocumentCommand`, `ZoomCommand`, `PanCommand`, `RotateViewCommand`, and related viewport/history payloads.
- [x] Go side: deserialize JSON command payloads, dispatch to engine
- [x] `RenderResult` response schema implemented with frame metadata, dirty rects, buffer references, and UI metadata.
- [x] Frontend input routing implemented: pointer events dispatch through engine commands, wheel zoom is cursor-anchored, and default browser behavior is suppressed on the canvas.
- [x] Pan tool: Space+drag pans viewport, hand tool icon
- [x] Zoom tool: click/drag zoom, `Alt`=zoom-out, scroll wheel zoom at cursor position
  - Done: zoom supports click-to-step, drag-to-scrub, `Alt` zoom-out, and cursor-anchored wheel zoom.

### Phase 1.4: Undo/Redo System

- [x] Command pattern in engine implemented with `Command`, `HistoryStack`, bounded depth, and grouped transactions for multi-step interactions.
- [x] Snapshots vs deltas: pixel-layer commands store diff (before/after dirty rect) not full copy
- [x] Dirty-rect pixel-delta history infrastructure implemented for future pixel-layer commands.
- [x] History panel UI implemented with command list, jump-to-state, and clear-history behavior.
- [x] Keyboard shortcuts: `Ctrl+Z` (undo), `Ctrl+Shift+Z` (redo), `Ctrl+Alt+Z` (step back in history)

### Phase 1 — Review gaps (2026-07-06, see Phase S)

- [ ] Undo of paint strokes doesn't repaint the canvas; undo with multiple documents open destroys the sibling documents (→ S.2)
- [ ] History is snapshot-per-command with ~4× full-document deep clones; navigation (zoom/pan/rotate) pollutes document history (→ S.3)
- [ ] Navigator "mini-viewport" is a static CSS-gradient placeholder, not a document thumbnail (→ S.8)
- [ ] Multi-document switching is engine-only — no `SwitchDocument` proto command, no tab UI (→ S.8)
- [ ] Zoom-out keyboard shortcut (`-`) zooms in due to a keymap value collision (→ S.7)
- [x] Every `DispatchCommand` returns a full render + complete `UIMeta` — resolved by the S.4 split (hot-path acks + versioned UIMeta, 2026-07-06)

---

## Phase 2: Layer System (Pixel Layer, Groups, Blend Modes, Masks) + Layers Panel

**Goal:** Photoshop foundation — multiple layers, blend modes, masks, visibility.

**Acceptance criterion:** Add/duplicate/move layers; blend modes visually correct; masks affect rendering.

### Phase 2.1: Layer Tree Data Model

- [x] `LayerNode` interface (Go):
  - [x] `ID` (UUID), `Name`, `Visible`, `Locked` (pixels/position/all)
  - [x] `Opacity` (0–1), `Fill` (0–1, separate from opacity for layer styles)
  - [x] `BlendMode` enum
  - [x] `Parent` pointer, `Children []LayerNode` (for groups)
  - [x] `Mask *LayerMask`, `VectorMask *Path`, `ClippingBase bool`
  - [x] `StyleStack []LayerStyle`
- [x] Layer types implementing `LayerNode`:
  - [x] `PixelLayer`: raw RGBA pixel buffer, bounds (x/y offset within doc)
  - [x] `GroupLayer`: contains children, pass-through or isolated blend
  - [x] `AdjustmentLayer`: params only, no pixel data (Phase 5)
  - [x] `TextLayer`: text params + cached raster (Phase 6)
  - [x] `VectorLayer`: path + fill/stroke params + cached raster (Phase 6)
- [x] Layer operations:
  - [x] `AddLayer`, `DeleteLayer`, `DuplicateLayer`, `MoveLayer` (reorder in tree)
  - [x] `SetVisibility`, `SetOpacity`, `SetBlendMode`, `SetLock`
  - [x] `FlattenLayer`, `MergeDown`, `MergeVisible`
- [x] All operations go through history (undo-able)

### Phase 2.2: Blend Modes & Compositing

- [x] Implement full blend mode set (Porter-Duff + Photoshop blend formulas):
  - [x] **Normal group:** Normal, Dissolve
  - [x] **Darkening:** Multiply, Color Burn, Linear Burn, Darken, Darker Color
  - [x] **Lightening:** Screen, Color Dodge, Linear Dodge (Add), Lighten, Lighter Color
  - [x] **Contrast:** Overlay, Soft Light, Hard Light, Vivid Light, Linear Light, Pin Light, Hard Mix
  - [x] **Inversion:** Difference, Exclusion, Subtract, Divide
  - [x] **Component:** Hue, Saturation, Color, Luminosity
- [x] Compositing pipeline:
  - [x] Walk layer tree bottom-to-top
  - [x] Apply blend mode formulas and composite each layer onto the accumulator
  - [x] Apply layer masks during compositing
  - [x] Group layers: composite children into isolated buffer, then composite group onto parent
  - [x] Pass-through groups: children blend directly into parent context
  - [x] Clipping mask: clip layer's alpha to base layer's alpha
- [x] Performance: cache layer composites; invalidate only when layer or ancestors change
- [x] Write golden-image unit tests for each blend mode formula

### Phase 2.3: Layer Masks

- [x] Raster layer mask:
  - [x] Grayscale 8-bit buffer (same size as document, white=reveal, black=hide)
  - [x] Operations: `AddMask(revealAll/hideAll/fromSelection)`, `DeleteMask`, `ApplyMask`, `InvertMask`
  - [x] Edit mask: painting on mask activates mask-edit mode (border indicator in UI)
  - [x] Disable/enable mask (Shift+click thumbnail in Layers panel)
- [x] Clipping mask:
  - [x] `ClipToBelow bool` flag on layer
  - [x] Compositing: clipped layer alpha *= base layer alpha
  - [x] Visual indent in Layers panel for clipped layers
- [x] Vector mask placeholder:
  - [x] `VectorMask *Path` field (renders to raster mask at composite time)
  - [x] Full implementation deferred to Phase 6.1

### Phase 2.4: Layers Panel UI

- [x] Tree view:
  - [x] Nested rows for groups (collapsible with arrow toggle)
  - [x] Layer thumbnail (canvas-rendered RGBA preview; engine returns 32×32 via GetLayerThumbnails command, updated on ContentVersion change)
  - [x] Mask thumbnail next to layer thumbnail (grayscale mask rendered to RGBA canvas, clickable to enter mask-edit mode)
  - [x] Layer name (double-click to rename inline)
- [x] Controls per layer row:
  - [x] Visibility eye icon (click to toggle, Alt+click to solo)
  - [x] Lock icon (click → cycle none/pixels/position/all)
  - [x] Blend mode dropdown (all 27 modes in grouped optgroups)
  - [x] Opacity slider/input (0–100%)
  - [x] Fill opacity slider/input (0–100%)
- [x] Panel toolbar: New Layer, New Group, Add Mask, Delete Layer, Merge Down
- [x] Context menu (right-click on layer):
  - [x] Duplicate Layer, Delete Layer, Merge Down, Merge Visible, Flatten Image
  - [x] Group Layers, Ungroup
  - [x] Add Layer Mask (Reveal All / Hide All / From Selection)
  - [x] Add Clipping Mask / Release Clipping Mask
  - [x] Layer Properties (rename + color tag — UI-only color labels, 8 colours)
- [x] Drag-and-drop reordering within the tree
- [x] Multi-select (Shift/Ctrl+click) for bulk operations
- [x] Channels panel stub (RGB + Alpha channels with per-channel visibility toggles, view only)

### Phase 2.5: Internal Project Save/Load

- [x] Define internal project format (`.agp` — Agogo Project):
  - [x] JSON manifest: document metadata, layer tree structure, blend modes, masks config, history metadata
  - [x] Binary blobs: pixel data per layer (raw RGBA, deflate-compressed via ZIP)
  - [x] Packaged as ZIP (JSON + blobs in single file, easy to inspect)
- [x] `SaveProject(doc) -> []byte`: serialize to `.agp`
- [x] `LoadProject([]byte) -> Document`: deserialize from `.agp`
- [x] File > Save / Save As (browser file system API / download)
- [x] File > Open (file picker, drag & drop onto canvas)
- [x] Auto-save to `localStorage` (every N commands, configurable) — *interval is hardcoded to 10, not configurable; runs synchronously on the main thread (→ S.7)*
- [x] Recovery on next open if auto-save present

### Phase 2 — Review gaps (2026-07-06, see Phase S)

- [ ] Blend engine is a manual per-pixel float64 implementation bypassing agg_go; dodge/burn extremes deviate from spec; `clipColor` NaN on lum==min (→ S.5, S.9)
- [ ] `FlattenLayer` double-applies opacity/fill-opacity (50% renders as 25%); MergeVisible loses hidden-layer z-order (→ S.5)
- [ ] Pixel/all layer locks are not enforced for painting, fills, or filters (→ S.5)
- [x] "Cache layer composites" was a single document-level cache with no partial invalidation — S.4 added dirty-rect incremental recomposite into the cached surface (2026-07-06); true per-layer/subtree caching remains future work if profiling demands it
- [ ] Alt+click eye-icon solo is missing; layers context menu doesn't work with a real pointer (closes before click fires) (→ S.7, S.8)
- [ ] "Golden-image unit tests" are expected-pixel tables — no actual image-snapshot tests exist (→ S.11)

---

## Phase 3: Selection & Transform (Move, Marquee/Lasso, Free Transform, Crop)

**Goal:** Photoshop-like interaction — select, move, transform, crop.

**Acceptance criterion:** Select areas, move layers, transform; UI stays responsive.

### Phase 3.1: Selection Engine (Backend)

- [x] `Selection` type: 8-bit alpha mask in document-space (`W × H` bytes, 0=transparent, 255=fully selected)
- [x] Selection operations:
  - [x] `New(rect/ellipse/polygon)` — creates new selection
  - [x] `Add` (Shift modifier), `Subtract` (Alt modifier), `Intersect` (Shift+Alt)
  - [x] `SelectAll`, `Deselect`, `Reselect` (reloads last selection)
  - [x] `Invert` — flips mask
- [x] Selection modification commands:
  - [x] `Feather(radius float)` — Gaussian blur on mask
  - [x] `Expand(px int)`, `Contract(px int)` — morphological dilation/erosion
  - [x] `Smooth(radius int)` — median-like smoothing on mask edges
  - [x] `Border(width int)` — select only the border band
  - [x] `TransformSelection` — free-transform the selection shape itself (not content)
- [x] Marching ants overlay:
  - [x] Backend renders animated dashed line border of selection
  - [x] `RenderSelectionOverlay(selection, viewport) -> []byte` composited into viewport output
  - [x] Animation frame counter incremented per render call
- [x] Color Range selection:
  - [x] `SelectColorRange(doc, layer, targetColor, fuzziness) -> Selection`
- [x] Quick Selection (flood-fill with edge detection) — foundation for Phase 3.2

### Phase 3.2: Selection Tools

- [x] **Marquee tools:**
  - [x] Rectangular Marquee: click-drag bounding box
  - [x] Elliptical Marquee: click-drag with AA edge
  - [x] Single Row / Single Column Marquee (1px-height/width)
  - [x] Modifier keys: Shift=add, Alt=subtract, Shift+Alt=intersect
  - [x] Shift during drag=constrain to square/circle
  - [x] Options bar: feather radius, anti-alias toggle
  - [x] Options bar: style (normal/fixed ratio/fixed size)
- [x] **Lasso tools:**
  - [x] Free Lasso: freehand path while pointer held down, auto-close on release
  - [x] Polygon Lasso: click points, double-click or click start to close
  - [x] Magnetic Lasso: click anchor points, hover snaps path to edges (Dijkstra on Sobel gradient)
- [x] **Magic Wand / Quick Selection:**
  - [x] Magic Wand: flood-fill selection by color similarity from click point
    - [x] Options: tolerance, anti-alias, contiguous, sample all layers
  - [x] Quick Selection: paint-to-expand selection with edge detection
- [x] **Move Tool:**
  - [x] Move active layer (or selection content) with pointer drag
  - [x] Auto-select layer: click picks topmost non-transparent layer under cursor
  - [x] Auto-select group: option to select group vs individual layer
  - [x] Arrow keys: nudge by 1px (Shift = 10px)
  - [x] Drag multiple selected layers simultaneously

### Phase 3.3: Transform System

- [x] Free Transform (`Ctrl+T`):
  - [x] Compute bounding box of layer (or selection content)
  - [x] Render transform handles overlay in backend:
    - [x] 8 scale handles (corners + edge midpoints)
    - [x] Rotation handle (above top-center)
    - [x] Center pivot point (draggable)
    - [x] Reference point grid (Photoshop-style) — 3×3 grid in options bar; clicking a cell moves the pivot to that corner/midpoint/centre of the bounding box
  - [x] Operations:
    - [x] **Scale:** drag corner/edge handles (uniform scale; Shift=constrain proportions not yet implemented)
    - [x] **Rotate:** drag outside bounding box (Shift=snap to 15° not yet implemented)
    - [x] **Move:** drag inside bounding box
    - [x] **Skew:** `Ctrl+drag` edge handle — shifts the dragged edge by delta, recomputes A/B/C/D from updated corner positions
    - [x] **Distort:** `Ctrl+drag` corner handle (free distort, no constraint) — uses AGG perspective span pipeline
    - [x] **Perspective:** `Ctrl+Shift+Alt+drag` corner — symmetric trapezoid (same AGG quad pipeline)
    - [x] **Warp:** grid-based mesh warp — 4×4 control-point grid; toggle via "Warp" button; per-cell AGG `TransformImageQuad` rendering; 16 draggable handles in overlay
  - [x] Numeric display in Options bar: X, Y, W%, H%, rotation angle (read-only; skew H/V and lock-aspect checkbox not yet implemented)
  - [x] Commit: Enter; Cancel: Escape
  - [x] Interpolation mode for pixel layers: Nearest Neighbor, Bilinear, Bicubic — selector in options bar, dispatched via UpdateFreeTransform
- [x] Transform on selection content:
  - [x] Floating selection: selected pixels become a temporary floating layer during transform
  - [x] Merge back on commit
- [x] **Edit > Transform sub-menu:**
  - [x] Scale, Rotate, Skew, Distort, Perspective, Warp — each begins free transform (Warp also initialises the 4×4 mesh via BeginFreeTransformPayload.Mode="warp")
  - [x] Flip Horizontal / Flip Vertical
  - [x] Rotate 90° CW/CCW, 180°
  - [x] Again (repeat last transform)

### Phase 3.4: Crop Tool

- [x] Crop overlay rendered in backend:
  - [x] Darkened area outside crop box
  - [x] Rule-of-thirds grid overlay inside crop box
  - [x] 8 resize handles on crop box
- [x] Operations:
  - [x] Resize crop box (drag handles)
  - [x] Move crop box (drag inside)
  - [x] Rotate crop box (drag outside — rotates the canvas, not just view)
  - [x] Constrain aspect ratio (Shift key during corner-handle drag)
- [x] Options bar: width/height inputs ✓, delete cropped pixels vs hide; [x] resolution, straighten, overlay type
- [x] Commit (Enter) / Cancel (Escape)
- [x] Content-Aware Fill for crop expansion
- [x] **Image > Canvas Size:** resize canvas independently of content, with anchor grid

### Phase 3.5: Selection & Transform UI

- [x] Options bar for each selection tool (feather, anti-alias, mode buttons)
- [x] Selection menu commands:
  - [x] All, Deselect, Reselect, Inverse
  - [x] Feather, Modify (Expand/Contract/Smooth/Border)
  - [x] Transform Selection
  - [x] Color Range dialog
  - [x] Save/Load selection to/from channel
- [x] Select and Mask workspace (Refine Edge):
  - [x] Dedicated full-screen workspace mode
  - [x] View modes: Onion Skin, Marching Ants, Overlay, Black/White, Black, White, Layer
  - [x] Edge refinement controls: Smooth, Feather, Shift Edge (expand/contract)
  - [x] Edge refinement controls: Smart Radius, Contrast (require new engine commands)
  - [x] Output to: Selection, Layer Mask
  - [x] Output to: New Layer, New Layer with Mask, Document
- [x] Transform Options bar: all numeric fields editable (X/Y/W/H/R), interpolation dropdown, warp toggle

### Phase 3 — Review gaps (2026-07-06, see Phase S)

- [ ] Transforms (free transform and discrete flip/rotate) do not transform layer masks — the mask stays behind in doc space (→ S.5)
- [ ] Crop / Canvas Size only handle PixelLayers; text/vector layers, masks, and the active selection stay in the old coordinate space and can blank the canvas (→ S.2)
- [ ] "Again (repeat last transform)" is backend-complete but has zero frontend wiring (→ S.8)
- [ ] Content-Aware Fill is BFS neighbor-average diffusion, not PatchMatch-class inpainting — the checked item overstates
- [ ] `meta()` never computes SkewY; Shift-constrain and 15° rotation snap remain open (as noted inline)
- [ ] No `onPointerCancel` handling: a cancelled pointer mid-gesture leaves move/quick-select transactions dangling in history (→ S.7)

---

## Phase 4: Painting Basics (Brush/Pencil/Eraser/Fill/Gradient) + Brush UI

**Goal:** Painting and basic retouch foundation.

**Acceptance criterion:** Draw on pixel layers; Undo works; engine renders strokes.

### Phase 4.1: Brush Engine (Backend)

- [x] Dab rasterization via AGG:
  - [x] Circular dab with configurable `size`, `hardness` (soft/hard edge via AGG radial gradient)
  - [x] Subpixel placement (AGG affine transform for fractional-pixel positioning)
  - [x] Alpha compositing of dab onto layer buffer with `flow` (per-dab alpha)
- [x] Stroke generation:
  - [x] Dab spacing as percentage of brush size (25% default, evenly spaced with carry-over)
  - [x] Interpolate dab positions along pointer path (catmull-rom for smoothness)
  - [x] Wet edges mode (accumulate at edges)
- [x] Brush dynamics:
  - [x] Pressure sensitivity: size and flow mapped from `PointerEvent.pressure` (0–1)
  - [x] Tilt sensitivity: direction mapping from `tiltX/tiltY` (Phase 4.1b)
  - [x] Jitter/scatter: random offset per dab (Phase 4.1b)
- [x] Stabilizer: weighted average of last N input points before finalizing position (configurable lag)
- [x] Blend modes for brush: all standard modes (paint directly with blend mode, not just Normal)
- [x] Sample merged option: eyedropper mode during painting (SampleMergedColor command returns composite RGBA at document-space point)

### Phase 4.2: Paint Tools

- [x] **Brush Tool (B):**
  - [x] Uses full brush engine (size, hardness, flow, spacing, pressure dynamics)
  - [x] Paints with foreground color
  - [x] Shortcut: `[`/`]` resize, `Shift+[`/`]` hardness
- [x] **Pencil Tool:**
  - [x] Hard-edge dabs only (no anti-aliasing), `hardness` locked to 100%
  - [x] Auto-erase mode (paints background color if stroke begins on foreground color)
- [x] **Eraser Tool (E):**
  - [x] Normal mode: paints transparency (clears alpha) on pixel layers
  - [x] Background Eraser: erases to background color (or transparency based on sampling)
  - [x] Magic Eraser: one-click flood-clear by color similarity (like Paint Bucket but erases)
- [x] **Mixer Brush (later, Phase 4.2b):**
  - [x] Tool exists in the frontend and can be selected from the toolrail
  - [x] Stroke payload now carries mixer-specific parameters through the existing paint ABI
  - [x] Engine samples the underlying canvas at stroke start and blends the sampled color into each dab before AGG rasterization
  - [x] Supports `Wetness`, `Load`, `Clean Brush`, and `Sample Merged` controls
  - [x] True wet-paint simulation: persistent runtime paint reservoir carries load/contamination across strokes until cleaned or depleted
  - [x] Brush-load / paint-load state, clean-brush reset, and dab-driven wetness decay are implemented
  - [x] Sampling now uses the full dab footprint instead of a center-point sample
  - [x] Directional smear / bristle streaking and edge accumulation specific to Photoshop-style Mixer Brush
  - [x] Basic interaction tuning with pressure, flow, scatter, and tilt now affects pickup/deposit behaviour
  - [x] Mixer Brush-specific presets and options-bar polish
- [x] **Clone Stamp (S) (later, Phase 4.2b):**
  - [x] Tool exists in the frontend and can be selected from the toolrail / `S` shortcut
  - [x] Alt+click on the canvas defines a clone source point
  - [x] Paint strokes clone from the captured source location with a fixed aligned offset
  - [x] Supports `Sample Merged` for source sampling
  - [x] Supports aligned and non-aligned source-offset modes
  - [x] Source overlays/crosshair preview and source-offset UI
  - [x] Source cloning from arbitrary history states
  - [x] Fade/opacity and paint-load style controls
  - [x] More exact Photoshop-style edge behavior and transform-aware source handling
- [x] **History Brush (later, Phase 4.2b):**
  - [x] Tool exists in the frontend and can be selected from the toolrail / `Y` shortcut
  - [x] Paint strokes restore pixels from the previous history state
  - [x] Supports `Sample Merged` for history-state source sampling
  - [x] User-selectable history source state
  - [x] Source overlay / crosshair preview and source-state UI
  - [x] Arbitrary non-destructive history-state painting from older checkpoints
  - [x] Fade / opacity-style controls and paint-load behavior
  - [x] Persistence rules when the document history is truncated or branched

### Phase 4.3: Fill & Gradient Tools

- [x] **Paint Bucket / Fill Tool (G):**
  - [x] Flood-fill from click point by color similarity
  - [x] Options: tolerance (0–255), contiguous, sample all layers, fill with foreground/pattern
  - [x] Respects selection mask
  - [x] `Edit > Fill` dialog: fill with color / background color / pattern
- [x] **Gradient Tool (G):**
  - [ ] Types: Linear, ~~Radial~~, Angle, Reflected, Diamond — **Radial is not implemented: the UI offers it but `fill_gradient.go:445` falls through to linear (→ S.5)**
  - [x] Gradient editor:
    - [x] Color stops (add/remove/move)
    - [x] Opacity stops
    - [x] Reverse checkbox, dither checkbox
    - [x] Gradient presets (save/load)
  - [x] Apply: drag to set direction and length; respects selection
  - [x] Modes: paint over layer, create fill layer (non-destructive gradient fill layer type)
- [x] **Eyedropper Tool (I):**
  - [x] Click to sample foreground color
  - [x] Alt+click to sample background color
  - [x] Sample size: point / 3×3 avg / 5×5 avg / 11×11 avg / 31×31 avg / 51×51 avg / 101×101 avg
  - [x] Sample: current layer / all layers / all layers no adj
  - [x] Color sampler points: place up to 4 persistent sample points (shown in Info panel)

### Phase 4.4: Brush & Color UI Panels

- [x] **Brush Settings Panel (Window > Brush Settings):**
  - [x] Tip shape selector: round / custom shapes
  - [x] Hardness slider, size slider, angle, roundness, spacing
  - [x] Brush Tip Shape preview
  - [x] Dynamics sections (Phase 4.1b): Size/Opacity/Flow jitter controls, control source dropdown (pressure/tilt/fade)
- [x] **Brush Preset Picker** (inline dropdown from Options bar):
  - [x] Grid of brush tip previews
  - [x] Search/filter by name
  - [x] Import `.abr` brush preset files (later)
- [x] **Color Picker (foreground/background):**
  - [x] Foreground/background color state in engine (SetForegroundColor / SetBackgroundColor commands)
  - [x] Foreground/background swatches in toolrail (minimal — click to reset to black/white)
  - [x] Click foreground or background swatch opens picker
  - [x] HSB wheel + SB field (or rectangular HSB box)
  - [x] Hex input, RGB sliders, HSB sliders, LAB sliders (later)
  - [x] "Only Web Colors" toggle
  - [x] Recent colors strip
  - [x] Swap foreground/background (`X` key), reset to black/white (`D` key)
- [x] **Color Panel (Window > Color):**
  - [x] Compact always-visible color sliders (RGB/HSB switchable)
  - [x] Gamut warning indicator
- [x] **Swatches Panel (Window > Swatches):**
  - [x] Grid of color swatches
  - [x] Click to set foreground, Alt+click to set background
  - [x] Add current foreground color, delete swatch
  - [x] Load/save swatch sets (`.aco` import later)
- [x] Options bar per paint tool: blend mode, opacity slider, flow slider, airbrush toggle, smoothing slider, pressure buttons

### Phase 4 — Review gaps (2026-07-06, see Phase S)

- [ ] Brush/eraser/clone strokes ignore the active selection — painting with a marquee active paints outside it (→ S.5)
- [ ] Mouse input paints at 75% size / 50% flow: pressure defaults to 0.5 with size/flow dynamics on, so every mouse user gets a weakened brush (→ S.5)
- [ ] Brush dynamics jitter sliders + control-source dropdown are display-only — values never reach the engine, no proto fields exist (→ S.8)
- [ ] `.abr` import is a filename-regex heuristic, not a format parser (→ S.8)
- [ ] Gradient "fill layer" is rasterized, not a non-destructive parametric layer; opacity stops are folded into stop colors, not independent
- [ ] Paint-bucket pattern fill is a hardcoded 8px checkerboard (→ S.6); LAB sliders missing; "gamut warning" is a web-safe-color label
- [x] Paint input is dispatched per raw pointermove with no coalescing — fixed in S.4 (rAF-batched multi-point ContinuePaintStroke, 2026-07-06)

---

## Phase 5: Adjustments & Filter System (Non-Destructive) + Properties/Adjustments Panel

**Goal:** Photo editing core — tonal corrections, curves, hue/sat as non-destructive adjustment layers; filter pipeline.

**Acceptance criterion:** Adjustment layers work non-destructively; core filters run; live preview updates.

### Phase 5.1: Adjustment Layer Framework

- [x] `AdjustmentLayer` base type:
  - [x] `Type` enum (Levels, Curves, HueSat, ColorBalance, etc.)
  - [x] `Params` (JSON-serializable, type-specific struct)
  - [x] `Mask *LayerMask` (optional — restrict adjustment to masked area)
  - [x] `Clip bool` (clip to layer below, like any layer)
- [x] Render pipeline integration:
  - [x] Before compositing a layer, walk up the stack to apply all adjustment layers above it (that are clipped or affect the composite group)
  - [x] Apply adjustment as a pixel-space color transform function: `adjustPixel(r, g, b, a, params) -> (r, g, b, a)`
  - [x] Cache adjustment result per dirty region; invalidate only when params or input change
- [x] Invalidation propagation:
  - [x] Change to adjustment layer params → re-render all layers below in the composite
  - [x] Upstream invalidation: only dirty the region that needs re-compositing
- [x] Non-destructive guarantee: deleting or hiding adjustment layer returns composite to original state
- [x] Serialize/deserialize adjustment params in `.agp` format

### Phase 5.2: Core Adjustment Layers

- [x] **Levels:**
  - [x] Input black point, white point, midtone gamma (per channel: R/G/B/RGB)
  - [x] Output black point, white point
  - [x] Auto-calculate (stretch to full range), Auto Options (clipping %)
  - [ ] Histogram display inside properties panel — **backend `ComputeHistogram` exists but has zero frontend consumers; no histogram is visible anywhere in the UI (→ S.8)**
- [x] **Curves:**
  - [x] Curve editor: click+drag to add/move control points on the curve
  - [x] Per channel: RGB composite + R/G/B individual
  - [x] Input/Output numeric readout at cursor (frontend UI concern — data available in curve points)
  - [x] Presets (save/load named curves) (frontend concern — params are JSON, storable as presets)
  - [x] Eyedropper: click image to set black/white/gray point from sample (backend: SetPointFromSample command)
- [x] **Hue/Saturation:**
  - [x] Master + per-color-range (Reds, Yellows, Greens, Cyans, Blues, Magentas)
  - [x] Hue shift (−180 to +180), Saturation (−100 to +100), Lightness (−100 to +100)
  - [x] Colorize mode (monochromatic)
  - [x] Color range selector eyedropper (backend: IdentifyHueRange command classifies sampled pixel)
- [x] **Color Balance:**
  - [x] Shadows, Midtones, Highlights sliders (Cyan-Red, Magenta-Green, Yellow-Blue)
  - [x] Preserve Luminosity checkbox
- [x] **Brightness/Contrast:**
  - [x] Simple Brightness (−150 to +150), Contrast (−50 to +100)
  - [x] Legacy mode checkbox
- [x] **Exposure:**
  - [x] Exposure (f-stops), Offset, Gamma Correction
- [x] **Vibrance:**
  - [x] Vibrance (smart saturation boost), Saturation
- [x] **Black & White:**
  - [x] Six color-range sliders (Reds/Yellows/Greens/Cyans/Blues/Magentas)
  - [x] Tint option (adds color overlay on grayscale)
  - [x] Auto

### Phase 5.3: Extended Adjustment Layers

- [x] Shared extended-adjustment plumbing:
  - [x] Add round-trip tests for the new adjustment payloads so `.agp` save/load preserves their parameters exactly
  - [x] Add panel-state coverage for switching between adjustment dialogs so in-progress edits survive normal shell updates
  - [x] Add a reusable parameter schema for extended adjustment layers so each type serializes cleanly in `.agp`
  - [x] Add undo/redo regression coverage for extended adjustment parameter edits
  - [x] Add live-preview re-render regression coverage for extended adjustment parameter edits
  - [x] Keep all extended adjustments on the same non-destructive render path used by Phase 5.1 and verify layer visibility / masking / clipping semantics stay identical
- [x] **Gradient Map:**
  - [x] Map source luminance to gradient stops across the full 0–255 range
  - [x] Reuse the gradient editor from Phase 4.3 for stop editing, color selection, and stop ordering
  - [x] Support reverse-gradient behavior and preserve alpha handling from the source layer
  - [x] Add a preview rendering path that updates as gradient stops are edited
- [x] **Invert:**
  - [x] Flip all RGB channels with `255 - v`
  - [x] Leave alpha unchanged unless the existing adjustment model explicitly treats alpha as part of the transform
  - [x] Keep the implementation minimal and deterministic so it is effectively a no-parameter adjustment
- [x] **Threshold:**
  - [x] Convert the image to a hard black/white split using a single threshold slider
  - [x] Define whether threshold is based on luminance or a specific channel and document that choice in the UI copy
  - [x] Clamp and validate slider values so preview and committed output always match
- [x] **Posterize:**
  - [x] Reduce tonal levels per channel with a slider in the 2–255 range
  - [x] Preserve alpha and avoid banding artifacts beyond the intended level reduction
  - [x] Make the control behave consistently for RGB composite and individual channels if the editor exposes both
- [x] **Channel Mixer:**
  - [x] Implement per-output-channel mixing coefficients for source R/G/B inputs
  - [x] Add monochrome output mode and define how the coefficients contribute to the grayscale result
  - [x] Expose a UI that makes the source-to-output relationship clear enough to tune without guesswork
  - [x] Validate coefficient normalization / clipping behavior so committed output does not overflow channel bounds
- [x] **Selective Color:**
  - [x] Adjust CMY+K components per named color range: Reds, Yellows, Greens, Cyans, Blues, Magentas, Whites, Neutrals, Blacks
  - [x] Support both Relative and Absolute adjustment modes and document the behavioral difference in the panel
  - [x] Define how the selected color range is sampled from the source pixel and how overlapping ranges are resolved
  - [x] Add per-range controls in the properties panel with immediate live preview
- [x] **Photo Filter:**
  - [x] Simulate gel-color filtering with a color picker, density slider, and preserve-luminosity toggle
  - [x] Blend the filter in a way that feels like a physical lens filter rather than a flat tint
  - [x] Ensure the filter preserves transparency and does not introduce unintended channel shifts outside the configured density
  - [x] Add a small set of representative filter presets if that matches the eventual UI design, otherwise keep the control fully custom

### Phase 5.4: Filter Framework

- [x] Filter registry:
  - [x] Each filter: `ID`, `Name`, `Category`, `HasDialog bool`, `Apply(layer, params, selection) -> modified_layer`
  - [x] Category menu structure: Blur, Sharpen, Noise, Distort, Stylize, Render, Other
- [x] Filter dialog system:
  - [x] Immediate filters: apply directly (e.g. Invert)
  - [x] Dialog filters: open parameter dialog with live preview before committing
  - [x] Preview: backend renders filter preview at reduced resolution for speed
  - [x] "Last Filter" shortcut (`Ctrl+F`) to re-apply last used filter with same params
  - [x] `Filter > Fade` after applying: blend filtered result with original (opacity + blend mode)
- [x] Filter applied destructively to pixel layer (vs Smart Filter on Smart Objects — Phase 7+)
- [x] Smart Filter placeholder: if layer is Smart Object, filter is stored non-destructively in style stack

### Phase 5.5: Core Filters

- [x] **Blur category:**
  - [x] Gaussian Blur: `radius` (float), uses AGG StackBlur
  - [x] Box Blur: fast approximate, `radius`
  - [x] Motion Blur: `angle`, `distance`
  - [x] Radial Blur: spin or zoom type, `amount`, `quality`
  - [x] Surface Blur: preserves edges, `radius`, `threshold`
- [x] **Sharpen category:**
  - [x] Sharpen / Sharpen More (fixed-kernel)
  - [x] Unsharp Mask: `amount`, `radius`, `threshold`
  - [x] Smart Sharpen: `amount`, `radius`, remove (Gaussian/Lens/Motion), shadow/highlight fade
- [x] **Noise category:**
  - [x] Add Noise: `amount`, Uniform/Gaussian distribution, monochromatic checkbox
  - [x] Reduce Noise: `strength`, preserve details, reduce color noise, sharpen details
  - [x] Median: `radius`
  - [x] Despeckle (one-shot)
- [x] **Distort category:**
  - [x] Ripple: `amount`, size (small/medium/large)
  - [x] Twirl: `angle`
  - [x] Offset: `horizontal`, `vertical`, wrap/repeat/fold edges
  - [x] Polar Coordinates: rectangular-to-polar / polar-to-rectangular
  - [x] Lens Correction: remove distortion, chromatic aberration, vignette, perspective
- [x] **Stylize category:**
  - [x] Emboss: `angle`, `height`, `amount`
  - [x] Find Edges (one-shot)
  - [x] Solarize (one-shot — partial inversion)
- [x] **Other category:**
  - [x] High Pass: `radius` (extracts edges — useful with overlay blend mode)
  - [x] Minimum / Maximum: morphological erosion/dilation, `radius`

### Phase 5.6: Adjustments & Properties Panel UI

- [x] **Adjustments Panel:**
  - [x] Grid of adjustment type icons (15 types with abbreviation + label)
  - [x] Click to create that adjustment layer above current layer
- [x] **Properties Panel** (context-sensitive):
  - [x] When adjustment layer selected: show params UI for that adjustment type
  - [x] All params are live — changes re-render immediately via SetAdjustmentParams command
  - [x] Clip to Layer below button, visibility toggle, delete button
  - [x] Mask section: show mask enabled/disabled toggle, invert, delete (Density/Feather pending engine support)
- [x] Live preview toggle: temporarily disable adjustment to compare before/after (via visibility toggle)

### Phase 5 — Review gaps (2026-07-06, see Phase S)

- [ ] **The entire filter system is unreachable from the UI**: all ~22 engine filters, the preview pipeline, Ctrl+F, and Fade work and are tested in Go, but the Filter menu is a static mock with no dispatch and no TS payload types (→ S.8)
- [ ] Filter-dialog commit applies the filter with nil params — OK is a visual no-op even once wired (→ S.2)
- [ ] Levels/Exposure gamma convention is inverted vs Photoshop (`v^gamma` instead of `v^(1/gamma)`); output-white default trap yields near-black output (→ S.5)
- [ ] Curves black/white/gray eyedropper (`SetPointFromSample`) and Hue/Sat range eyedropper (`IdentifyHueRange`) exist in the engine but are never dispatched (→ S.8)
- [ ] Non-Gaussian filters average RGB only and copy original alpha (halo bleed, frozen silhouettes in distort filters) (→ S.5)
- [ ] Adjustment sliders dispatch one synchronous engine command per tick with no debounce/transaction batching (→ S.7)

---

## Phase 6: Text & Vector (Pen/Shapes/Type) + Layer Styles

**Goal:** Design/UI workflows — text, shapes, vector masks, layer styles.

**Acceptance criterion:** Text/shapes editable; layer styles visible; PNG export works.

### Phase 6.1: Vector Path System

- [x] **Path data model:**
  - [x] `Path`: list of `Subpath`s
  - [x] `Subpath`: list of `AnchorPoint`s + `closed bool`
  - [x] `AnchorPoint`: `anchor (x,y)`, `controlIn (x,y)`, `controlOut (x,y)`, handle type (corner/smooth/symmetric)
  - [x] Path stored in doc-space coordinates (resolution-independent)
- [x] **Pen Tool (P):**
  - [x] Click: add corner anchor point
  - [x] Click+drag: add smooth anchor point (pull out control handles)
  - [x] Close path: click first anchor point
  - [x] Continue open path: click endpoint, continue adding anchors
  - [x] Rubber-band preview: line/curve from last anchor to cursor
- [x] **Direct Selection Tool (A):**
  - [x] Click anchor: select single anchor (white fill = selected, hollow = deselected)
  - [x] Drag anchor: move anchor point
  - [x] Drag control handle: adjust curve independently
  - [x] Alt+click handle: break smooth to corner (independent handles)
  - [x] Shift+click: add/remove from selection
  - [x] Drag selection rect: marquee-select multiple anchors
- [x] **Path Operations:**
  - [ ] Combine (union), Subtract, ~~Intersect~~, Exclude, ~~Divide~~ — **not real boolean geometry: Combine/Exclude are subpath concatenation over even-odd fill, Subtract is a winding reversal (no-op under even-odd), Intersect returns an error; needs Clipper2 (→ S.5)**
  - [x] Flatten path to single subpath
- [x] **Rasterize path to mask / layer:**
  - [x] Render path via AGG rasterizer with AA → alpha mask or pixel layer
  - [x] `Rasterize Layer` command for Vector layers
- [x] **Paths Panel:**
  - [x] List of named paths in document
  - [x] Work Path (temporary), Shape paths, Saved paths
  - [x] New, Duplicate, Delete, Make Selection from Path, Stroke Path, Fill Path

### Phase 6.2: Shape Tools

- [x] **Rectangle Tool (U):**
  - [x] Drag to draw rectangle
  - [x] Shift = constrain to square
  - [x] Creates Vector Layer with fill color and optional stroke
  - [x] Options bar: fill color, stroke color/width, corner radius
- [x] **Rounded Rectangle Tool:** as above, with corner radius
- [x] **Ellipse Tool:** drag for ellipse, Shift = circle
- [x] **Polygon Tool:** N sides, star mode (inner radius %)
- [x] **Line Tool:** draws 1D path with stroke (width from options bar)
- [x] **Custom Shape Tool:**
  - [x] Shape library panel (preset shapes: arrows, logos, nature, ornaments)
  - [x] Import custom shapes from `.csh` files
  - [x] Persist imported shapes in the shape library and expose them in the Custom Shape picker
  - [x] Preserve compound shapes via multi-subpath custom shape presets
- [x] Shape layer editing:
  - [x] Double-click shape layer → enters path editing mode
  - [x] Can change fill/stroke without rasterizing
  - [x] Path operations (combine shapes on same layer)  ← covered by Paths panel boolean ops
- [x] **Mode toggle** in options bar: Shape layer vs Path (no fill) vs Pixels (rasterize immediately)

### Phase 6.3: Text Engine

- [ ] **Font loading:**
  - [ ] Load fonts via `FontFace` API (browser system fonts + uploaded fonts)
  - [ ] Font catalog: list available fonts with preview
  - [ ] Web font loading from URL (later)
- [x] **Text rendering via AGG:**
  - [x] Rasterize text via AGG `FontGSV` (WASM-safe built-in stroke-vector font)
  - [ ] Load TrueType/OpenType outlines (using Go font library, e.g. `golang.org/x/image/font/sfnt`)
  - [ ] Subpixel-accurate glyph placement, kerning, ligatures (basic)
- [x] **Text layer types:**
  - [x] **Point Text:** click to start, type horizontally, no auto-wrap
  - [x] **Area Text:** word-wrapping within bounds
  - [ ] **Type on Path:** (Phase 6.3b) text flows along a path
- [ ] **Text properties stored per-layer** (per-run rich text deferred):
  - [x] Font size
  - [x] Color (RGBA)
  - [x] Tracking (letter-spacing)
  - [x] Leading (line-spacing)
  - [x] Underline
  - [x] Strikethrough
  - [x] All caps
  - [x] Small caps (approximated as all caps at reduced size)
  - [x] Font family, style (Regular/Bold/Italic/Bold-Italic) metadata stored per layer; actual font loading/rendering still pending
  - [x] Baseline shift, superscript, subscript metadata stored per layer
  - [x] Anti-alias mode metadata stored per layer
- [x] **Paragraph properties (per layer):**
  - [x] Alignment: Left/Center/Right/Justify
  - [x] Indents: left indent, right indent, first-line indent
  - [x] Space before/after paragraph (split on `\n\n`)
  - [ ] Hyphenation (optional)
- [x] **Edit mode:**
  - [x] Type tool click → creates text layer + enters edit mode
  - [x] Frontend textarea overlay for text input (pre-filled with existing text)
  - [x] Double-click existing text layer in Layers panel → enters text editing mode
  - [ ] Cursor, selection highlight rendered in backend overlay
  - [ ] Click+drag to select text range, Shift+click to extend
  - [ ] Keyboard: standard text navigation (Home/End, Ctrl+A, Ctrl+C/X/V)
- [x] **Commit text:** Escape or Done button; undo reverts to pre-edit state (single history entry)
- [x] **Type > Create Outlines:** converts text to GSV-based outline VectorLayer — implemented 2026-07-06 via public `GSVText` vertex tracing (stroke-only vector layer; open centerline subpaths, stroked at the GSV raster width)

### Phase 6.4: Text UI Panels

- [x] **Character Panel (Window > Character):**
  - [x] Size input
  - [x] Leading input
  - [x] Tracking input
  - [x] Color swatch (basic toggle; full color picker TODO)
  - [x] Style toggles: All Caps, Small Caps, Underline, Strikethrough
  - [ ] Font family dropdown (searchable), font style dropdown
  - [ ] Bold, Italic, Superscript, Subscript, Kerning, Baseline shift
  - [ ] Anti-alias mode dropdown
  - [ ] Language selector (for hyphenation/spell check)
- [x] **Paragraph Panel (integrated into Character panel):**
  - [x] Alignment buttons (Left/Center/Right/Justify)
  - [x] Indent left/right/first-line inputs
  - [x] Space before/after inputs
  - [ ] Hyphenation checkbox
- [ ] **Options bar for Type Tool:**
  - [ ] Orientation (horizontal/vertical toggle)
  - [ ] Quick access: font, style, size, anti-alias, alignment, color, warp text, panels

### Phase 6.5: Layer Styles

- [x] **Layer Style data model:**
  - [x] `StyleStack []Effect` per layer
  - [x] Each effect: enabled bool, params struct
  - [x] Effects ordered: Fill effects applied first, then stroke, then shadow effects
- [x] **Layer Style dialog:**
  - [x] Left column: effect list (checkboxes to enable/disable each effect)
  - [x] Right panel: params for selected effect
  - [x] Live preview on canvas while dialog is open
  - [x] OK / Cancel / New Style (save as preset) / Reset
- [x] **Implement effects (rendered in backend during composite):**
  - [x] **Drop Shadow:** color, opacity, angle, distance, spread, size, noise, layer knocks out shadow
  - [x] **Inner Shadow:** color, opacity, angle, distance, choke, size, noise
  - [x] **Outer Glow:** color or gradient, opacity, noise, technique (softer/precise), spread, size
  - [x] **Inner Glow:** color or gradient, source (edge/center), choke, size
  - [x] **Bevel & Emboss:** style (outer/inner/emboss/pillow/stroke), technique, depth, direction, size, soften; shading: angle, altitude, gloss contour, highlight/shadow modes
  - [x] **Satin:** color, blend mode, opacity, angle, distance, size, contour
  - [x] **Color Overlay:** color, blend mode, opacity
  - [x] **Gradient Overlay:** gradient, blend mode, opacity, style, angle, scale, align with layer
  - [x] **Pattern Overlay:** pattern, blend mode, opacity, scale, link with layer
  - [x] **Stroke:** size, position (outside/inside/center), blend mode, opacity, fill type (color/gradient/pattern)
- [x] **Blend If / Advanced Blending:**
  - [x] Fill opacity (separate from layer opacity for effects)
  - [x] Channels (R/G/B checkboxes to include in blend)
  - [x] Blend If sliders: "This Layer" and "Underlying Layer" — split sliders for smooth transitions
- [x] **Styles Panel (Window > Styles):**
  - [x] Preset style thumbnails
  - [x] Click to apply style to current layer
  - [x] Save current layer style as preset
  - [ ] Import/export `.asl` style files (later)
- [x] **Copy/Paste Layer Style** (right-click context menu)
- [x] **Flatten/Merge with effects:** merge effects into pixel data

### Phase 6.5 — Review gaps (2026-07-06, see Phase S)

- [ ] Drop shadow falls *toward* the light at the PS-default 120° angle (sign error), and DropShadow/OuterGlow composite on top of layer content instead of behind it (→ S.5)
- [ ] Gradient Overlay renders a hardcoded blue→orange ramp — user gradient stops are ignored; Pattern Overlay / pattern stroke are hardcoded checkerboards (→ S.6)
- [ ] Contour, Noise, Knockout, Altitude, and bevel Technique params are decoded but never used in rendering (→ S.6)

---

## Phase 7: PSD/PSB Compatibility, Artboards, Slices, Automation

**Goal:** Photopea-level feature set — PSD as native format, artboards/slices/actions.

**Acceptance criterion:** Open/save PSD (subset) works; slices/artboards export; actions/variables run rudimentarily.

### Phase 7.1: PSD Parser (Read)

- [x] Implement PSD/PSB file format reader per Adobe specification:
  - [x] **File header:** magic (`8BPS`), version (1=PSD, 2=PSB), channels, height, width, depth, color mode
  - [x] **Color mode data section**
  - [x] **Image resources section:** DPI (0x03ED), ICC profile (0x040F), guides (0x0408), slices (0x041A), layer comps (0x0435)
    - [x] DPI (0x03ED)
    - [x] ICC profile (0x040F)
    - [x] Guides (0x0408)
    - [x] Slices (0x041A)
    - [x] Layer comps (0x0435)
  - [x] **Layer and mask information section:**
    - [x] Layer count and layer records (bounding rect, channels, blend mode, opacity, flags, name, extra data)
    - [x] Extra layer data: layer name (Unicode)
    - [x] Extra layer data: layer ID, layer color tag, sections (groups/begin-end markers)
    - [x] Layer masks: mask data per layer
    - [x] Layer effects (legacy effects list + object-based effects / descriptor)
    - [x] Text layer data (descriptor-based: TySh)
    - [x] Vector mask data (vmsk / vsms)
    - [x] Adjustment layer params per type (leve, curv, hue2, etc.)
    - [x] Smart object data (PlLd, SoLd, lsct descriptors)
  - [x] **Image data section:** channel pixel data (raw, RLE, zip with/without prediction)
    - [x] raw
    - [x] RLE
    - [x] zip without prediction (zip stream)
    - [x] zip with prediction
  - [x] PSB differences: 8-byte length fields, layer/channel lengths, height/width width limits in header parsing
- [x] Map parsed data to internal `Document` / `LayerNode` tree
- [x] Fallback: unknown layer types import as flattened pixel layer with warning
- [x] Error handling: corrupt/partial PSDs load what they can, report issues

### Phase 7.2: PSD Writer (Save)

- [x] Serialize internal document to PSD/PSB byte stream:
  - [x] Write file header
  - [x] Serialize all image resources
  - [x] Serialize layer tree (order, bounding rects, pixel data, blend mode, opacity)
  - [x] Serialize masks per layer
  - [x] Serialize layer effects as descriptors
  - [x] Serialize text layers as TySh descriptors
  - [x] Serialize adjustment layer params
  - [x] Serialize merged image data (composite of all visible layers)
  - [x] RLE compression for pixel data (PackBits)
- [ ] Round-trip test: open PSD → modify → save → re-open, verify no loss for supported features — **current round-trip passes only because reader and writer share the same bugs and fidelity rides on an embedded `AgogoProject` JSON block; compression constants are off by +2, so real Photoshop files cannot be opened and written files are invalid for other apps; zero external fixtures (→ S.10)**
- [x] PSB write for documents exceeding PSD limits (30,000px)
- [x] Save as PSD / Save as PSB in File menu

### Phase 7.1/7.2 — Review gaps (2026-07-06, see Phase S.10)

- [ ] Compression codes off by +2 vs spec — genuine Photoshop files fail to open ("unsupported compression 1"); written files invalid for other apps
- [ ] Group section-divider semantics inverted vs Photoshop; layer masks triple-broken (decode dims, never assigned, offset discarded); only 7 of 27 blend modes mapped; zip channel decode always errors
- [ ] Adjustment layers / text layers / smart objects are parsed as metadata only and flattened on import — checked sub-items above describe capture, not reconstruction; TySh/lfx2 are written as JSON-in-descriptor pseudo-formats no other app can read
- [ ] No allocation bounds on untrusted length fields (hostile file OOM-kills the editor); no fuzz tests, no real-world fixtures

### Phase 7.3: Artboards

- [ ] **Artboard data model:**
  - [x] An artboard is a special `GroupLayer` with a fixed rectangular bounds in doc-space
  - [x] Document can contain multiple artboards (or none — traditional document)
  - [x] Artboard has its own background color
  - [ ] Artboards are visible as labeled frames on the canvas; content outside bounds is clipped during export
- [ ] **Artboard tool:**
  - [x] Create artboard: drag on canvas (similar to shape tool)
  - [x] Resize artboard: drag handles (like free transform)
  - [x] Move artboard with content
  - [x] Preset sizes: iPhone, iPad, Desktop, Custom
- [ ] **Artboards Panel / Layers Panel integration:**
  - [x] Artboards appear as top-level groups with special icon
  - [ ] Layers inside artboards are children
- [ ] **Export artboards:**
  - [ ] File > Export > Artboards to Files → choose format (PNG/JPG/PDF), naming, destination
  - [ ] File > Export > Artboards to ZIP
  - [ ] Per-artboard scale variants (1x, 2x, 3x)

### Phase 7.4: Slices

- [ ] **Slice data model:**
  - [ ] `Slice`: rect (x, y, w, h) in doc-space, name, URL, alt text, export settings
  - [ ] Slice types: user-created, layer-based (auto from layer bounds), auto (fill space between user slices)
- [ ] **Slice Tool (C):**
  - [ ] Drag to create user slice
  - [ ] Slices shown as numbered rectangles with labels
  - [ ] Slice Select Tool: click to select, resize, move
  - [ ] Divide Slice: split into N rows/columns
  - [ ] Delete Slice
- [ ] **Slice Options (double-click):**
  - [ ] Name, URL, Target, Alt, Message, dimensions (numeric)
  - [ ] Export format override per slice (PNG/JPG/GIF, quality)
- [ ] **Export slices:**
  - [ ] File > Export > Save for Web (slices tab): preview all slices, adjust formats
  - [ ] File > Export > Slices to Files → folder + naming pattern
  - [ ] Layer-based slices: `Layer > New Layer Based Slice` auto-creates slice from layer bounds
- [ ] View > Show Slices toggle (on/off overlay)

### Phase 7.5: Actions / Automation

- [ ] **Action data model:**
  - [ ] `Action`: name, list of `ActionStep`s
  - [ ] `ActionStep`: `commandID`, serialized params (same command protocol as regular commands)
  - [ ] `ActionSet`: named group of actions
- [ ] **Record mode:**
  - [ ] Start recording: all subsequent commands are appended to current action
  - [ ] Stop recording
  - [ ] Recorded steps shown in Actions panel in real-time
  - [ ] Edit individual step params (double-click step)
  - [ ] Toggle modal/non-modal per step (dialogs stop playback or auto-confirm)
- [ ] **Play action:**
  - [ ] Play from beginning or from selected step
  - [ ] Step-through mode (pause after each step for review)
  - [ ] Playback speed controls
- [ ] **Batch processing:**
  - [ ] File > Automate > Batch: select action set + action, source (folder), destination (folder/same/save&close), naming pattern
  - [ ] Process list of files through browser File System Access API
- [ ] **Actions Panel UI:**
  - [ ] Accordion list of action sets > actions > steps
  - [ ] Record button (red), Stop, Play, New Action, New Set, Delete, duplicate
  - [ ] Step enable/disable checkboxes
  - [ ] Import/Export actions as `.atn` (Adobe format — best-effort parsing)

### Phase 7.6: Variables & Data Sets

- [ ] **Variables data model:**
  - [ ] Variables defined per document, bound to specific layers
  - [ ] Variable types: Text Replacement (replaces text layer content), Pixel Replacement (replaces pixel layer image), Visibility (show/hide layer)
  - [ ] Data sets: each data set is a row — maps variable names to values
- [ ] **Variables dialog (File > Variables):**
  - [ ] Tab 1 — Define: create/delete variables, bind to layers, set type
  - [ ] Tab 2 — Data Sets: import CSV, edit rows, preview
- [ ] **Export data sets:**
  - [ ] Iterate over data sets: substitute variables, export each variant as image
  - [ ] Output: individual files with naming from data set column

### Phase 7.7: Scripting

- [ ] **Script console panel:**
  - [ ] Text input area for script commands
  - [ ] Output/log area
- [ ] **Script model:**
  - [ ] Expose global `app` object to scripts (similar to Photoshop/Photopea scripting API)
  - [ ] `app.activeDocument`: Document properties
  - [ ] `app.activeDocument.activeLayer`: Layer operations
  - [ ] Available methods: `addLayer`, `deleteLayer`, `setLayerBlendMode`, `applyFilter`, etc.
  - [ ] Scripts are sequences of calls to the same command protocol (macro over the command bus)
- [ ] **Import/run scripts:** File > Scripts > Browse (file picker for `.js` script files)
- [ ] **Bundled scripts:** File > Scripts > built-in utilities (e.g. "Export Layers to Files")

---

## Phase 8: Performance Hardening (Worker, Dirty Rects, Caches) + Pro UX

**Goal:** Professional-feeling editor — no jank, large files, fast tools.

**Acceptance criterion:** Large documents remain navigable; brush strokes feel fluid; UI thread never blocks.

### Phase 8.1: Web Worker Migration

- [ ] Move Wasm engine instantiation and execution to a `Worker`:
  - [ ] Worker file: `engine.worker.ts` — loads Wasm, runs event loop
  - [ ] Main thread ↔ Worker communication via `postMessage` / `MessageChannel`
- [ ] Stabilize message protocol:
  - [ ] Commands: serialize to `Uint8Array` (binary command packets) — avoid JSON for hot path
  - [ ] Responses: `RenderResult` with `Transferable` pixel buffer (`ArrayBuffer.transfer`)
  - [ ] Control messages: ping/pong, worker-ready, error
- [ ] UI thread never blocks on engine:
  - [ ] All engine calls are async (fire-and-forget commands, receive rendered frames when ready)
  - [ ] Decouple input rate from render rate (engine can render at lower FPS than pointer events arrive)
- [ ] Frame pipeline:
  - [ ] Input collector: accumulates pointer events between frames
  - [ ] Frame request: send accumulated commands, request new render
  - [ ] Frame receive: apply `putImageData` on `requestAnimationFrame`
  - [ ] Back-pressure: don't queue more than N outstanding render requests

### Phase 8.2: Dirty Rect Rendering

- [ ] Engine tracks dirty rectangles:
  - [ ] Brush stroke: bounding box of new dabs in this command
  - [ ] Transform: pre+post transform bounding box union
  - [ ] Adjustment change: affected layer bounds
  - [ ] Union of all dirtied rects for the frame
- [ ] Backend returns `dirtyRects[]` in `RenderResult` (already in protocol)
- [ ] Frontend: only re-blit dirty regions via `ctx.putImageData(imageData, dx, dy, dirtyX, dirtyY, dirtyW, dirtyH)`
- [ ] Compositor: only re-render dirty region (skip unchanged tiles/layers)
- [ ] Benchmark: measure fps improvement on large canvases with small stroke areas

### Phase 8.3: SharedArrayBuffer & Zero-Copy (Optional Optimization)

- [ ] Set up cross-origin isolation (required for SharedArrayBuffer):
  - [ ] Configure server/hosting with headers: `Cross-Origin-Opener-Policy: same-origin`, `Cross-Origin-Embedder-Policy: require-corp`
  - [ ] Service Worker approach as fallback for hosts that don't support custom headers
  - [ ] Verify isolation: `crossOriginIsolated === true` in browser
- [ ] SharedArrayBuffer ring buffer:
  - [ ] Allocate SAB for pixel output (sized to max canvas dimensions)
  - [ ] Ring buffer with write-head and read-head pointers (also in SAB)
  - [ ] Worker writes completed frame to SAB, increments write head
  - [ ] UI thread reads from SAB on RAF, no copy needed
- [ ] Frame fences:
  - [ ] Use `Atomics.wait` / `Atomics.notify` for synchronization
  - [ ] Frame ID in SAB header for stale-frame detection
- [ ] Fallback: if `crossOriginIsolated` is false, fall back to Transferable ArrayBuffer mode

### Phase 8.4: Multi-Resolution & Tile Cache

- [ ] Downscale pyramid (mipmaps) in backend:
  - [ ] For each pixel layer, maintain pre-computed lower-resolution versions
  - [ ] Update pyramid tiles only in regions touched by edits
  - [ ] Zoom-out rendering uses appropriate pyramid level (avoids reading all pixels)
- [ ] Tile-based rendering:
  - [ ] Divide canvas output into tiles (e.g. 256×256 device pixels)
  - [ ] Track per-tile dirty state; only re-composite dirty tiles
  - [ ] Viewport render: union dirty tiles in view frustum
- [ ] Layer cache:
  - [ ] Cache composited result of sub-trees that haven't changed
  - [ ] Smart Object cache: cache rendered smart object at multiple resolutions
- [ ] Memory budget:
  - [ ] LRU cache eviction for layer and pyramid caches
  - [ ] Configurable max cache size (user preference)

### Phase 8.5: Pro UX Features

- [ ] **Guides and Rulers:**
  - [ ] Ruler display (horizontal + vertical, draggable origin corner)
  - [ ] Units: px/pt/cm/mm/in/percent (configurable per doc)
  - [ ] Drag guides from ruler edge; drag to reposition; double-click to set exact position
  - [ ] Lock guides, clear all guides
  - [ ] Smart guides: snap to edges/centers of other layers (live feedback lines rendered in backend overlay)
- [ ] **Grid:**
  - [ ] Show/hide grid (View > Show > Grid)
  - [ ] Grid color, style, spacing — all configurable in Preferences
  - [ ] Snap to Grid
- [ ] **Snap system:**
  - [ ] Snap targets: guides, grid, layer edges, layer centers, document edges, artboard edges, slices
  - [ ] Toggle each snap type independently (View > Snap To)
  - [ ] Snap threshold in pixels
- [ ] **Histogram Panel:** live histogram of current document composite or active layer (R/G/B/A/Luminosity channels, switchable)
- [ ] **Info Panel:** cursor position (doc-space), color readout at cursor (mode-dependent), document size, selection dimensions, transform feedback
- [ ] **Keyboard Shortcut Customizer:**
  - [ ] File > Keyboard Shortcuts dialog
  - [ ] Browse all commands by menu/panel
  - [ ] Click a command row → press new key combination
  - [ ] Conflict detection with warning
  - [ ] Save named shortcut set, load preset (Photoshop-like defaults)
  - [ ] Export shortcuts as PDF/reference sheet
- [ ] **Workspace Presets:**
  - [ ] Window > Workspace: Essentials, Photography, Typography, Painting, Custom
  - [ ] Save current panel layout + keyboard shortcuts as named workspace
  - [ ] Reset to saved workspace
- [ ] **Preferences dialog** (Edit > Preferences):
  - [ ] UI: theme (dark/medium dark/light/medium light), language, font size
  - [ ] Performance: history states count, cache levels, tile size
  - [ ] Guides & Grid: colors, style, subdivision
  - [ ] File Handling: auto-save interval, recovery location
  - [ ] Rulers & Units: ruler units, column size
- [ ] **Fullscreen mode:** hide all UI chrome, canvas fills browser window (Ctrl+Shift+F)
- [ ] **Tab bar for multiple open documents:** each document in a tab, drag to reorder

---

## Quality, Testing, Build & Deployment

### Testing Strategy

- [ ] **Go Engine Unit Tests:**
  - [ ] Blend mode formulas: input/output pairs for each mode (compare to known-correct values)
  - [ ] Selection ops: add/subtract/intersect/feather — golden image masks
  - [ ] Filter kernels: Gaussian, Unsharp Mask — compare output buffers (within epsilon)
  - [ ] Adjustment layers: Levels/Curves/HueSat — compare pixel transforms
  - [ ] PSD parser: known PSD files → parse → re-serialize → compare bytes
  - [ ] AGG path rasterization: known paths → compare rendered alpha masks

- [ ] **Deterministic Render Tests:**
  - [ ] Snapshot test: `RenderViewport(doc, vp)` → SHA256 hash stored as golden
  - [ ] CI fails on hash mismatch (flag intentional changes by updating goldens explicitly)
  - [ ] Test fixtures: minimal documents with 1–5 layers covering each layer type

- [ ] **ABI Stability / Interop Tests:**
  - [ ] Run Wasm via Node.js (`node --experimental-wasm-...` or standard Node 20+)
  - [ ] Test: JS calls `EngineInit` → `CreateDocument` → `RenderViewport` → `Free`; verify no memory leaks
  - [ ] Test: command round-trip (serialize TS payload → Go deserialize → verify fields)

- [ ] **E2E Tests (Playwright):**
  - [ ] Open editor → create new document → paint stroke → undo → redo → export PNG → compare hash
  - [ ] Open PSD fixture → verify layer count + names → render → compare screenshot
  - [ ] Apply adjustment layer → verify visual change
  - [ ] Text tool → type text → commit → verify text rendered in export
  - [ ] Run via CI on Chromium headless

### Build & Release

- [ ] **Production builds:**
  - [ ] Frontend: `vite build` with code splitting, tree-shaking
  - [ ] Wasm: `go build` with optimization flags (`-ldflags="-s -w"`, `tinygo` consideration for size)
  - [ ] Wasm compression: Brotli pre-compressed + gzip fallback; server configured to serve with `Content-Encoding`
  - [ ] Bundle size budget: track JS bundle and Wasm size in CI (fail if exceeds threshold)
- [ ] **Version stamping:**
  - [ ] Embed build-time version in Wasm binary (`go:embed` or linker flag)
  - [ ] Frontend version from `package.json` + git short SHA
  - [ ] Version displayed in Help > About and diagnostics panel
- [ ] **Feature flags:**
  - [ ] Runtime flag system for beta features (Liquify, Smart Objects, CMYK mode, RAW import)
  - [ ] Flags configurable via URL param (`?flags=liquify,smart-objects`) or settings

### Deployment & Security Headers

- [ ] Configure hosting with required headers:
  - [ ] `Cross-Origin-Opener-Policy: same-origin`
  - [ ] `Cross-Origin-Embedder-Policy: require-corp`
  - [ ] (Required for SharedArrayBuffer and Wasm Threads)
- [ ] Verify `crossOriginIsolated === true` in deployed app
- [ ] Service Worker COOP/COEP header injection fallback (for hosts without custom headers)
- [ ] CSP (Content Security Policy) — allow Wasm eval, no inline scripts

### License & Third-Party Audit

- [ ] **AGG:** determine exact version/fork; replace any GPL components if commercial use intended
- [ ] **GPC (General Polygon Clipper):** non-commercial only — replace with alternative (e.g. Clipper2 / Polyclipping library with permissive license) before any commercial deployment
- [ ] **Fonts:** verify EULA for any bundled fonts; use OFL or custom-licensed fonts only
- [ ] **PSD/PSB Specification:** Adobe spec is publicly accessible; parser is clean-room
- [ ] **RAW decoding:** patent/licensing review before implementing RAW support
- [ ] Document all third-party dependencies and licenses in `THIRD_PARTY_LICENSES.md`
- [ ] Run `go licenses` and `license-checker` (npm) in CI

---

## Deferred / Later Features (Post-Phase 8)

These are explicitly out of scope for Phases 0–8 but should be planned for:

- **Liquify filter** (mesh-warp, forward warp, reconstruct, smear — very complex, needs special UI)
- **Vanishing Point** (3D plane definition + perspective-correct clone/paste)
- **Smart Objects** (embedded documents, non-destructive transforms, linked Smart Objects from disk)
- **Smart Filters** (non-destructive filter stack on Smart Objects)
- **CMYK / Lab color modes** (needs ICC color profile management)
- **16-bit and 32-bit per channel** editing
- **RAW file support** (dcraw-like decoder or LibRaw port)
- **Healing Brush / Spot Healing / Content-Aware Fill** (complex inpainting algorithms)
- **Content-Aware Scale** (seam carving)
- **Mixer Brush** (wet paint simulation)
- **3D features** (very late / optional)
- **Oil Paint filter** (GPU-required in Photoshop; in Wasm requires heavy CPU implementation)
- **Wasm Threads** (requires full COOP/COEP deployment, complex parallelism)
- **Offline PWA** (Service Worker cache, offline-first)
- **Cloud storage / autosave to server**
- **Collaboration / multiplayer**
- **AI-assisted selection** (Subject Select, Remove Background via ML model in Wasm)
