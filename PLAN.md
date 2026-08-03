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
- [x] **Text/vector raster geometry contract is self-contradictory**: canonical contract is now **bounds-local** `CachedRaster` (`Bounds.W×Bounds.H`, composited at `Bounds.X/Y`), documented on the model fields; `rasterizeTextLayer` renders bounds-local (was doc-sized with position baked in → text drew at 2×(X,Y)); `convertTextToPath` outline layer gets doc bounds. Known caveat: center/right-aligned point text clipped at the layer's left edge — resolved in S.6 (tight point-text bounds + persistent anchor)
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

### Phase S.5: Engine Feature-Bug Fixes — ✅ DONE (2026-07-06)

> Executed via 11 implementer subagents in 3 batches (disjoint file ownership) + per-batch adversarial review with fix loops; every fix landed TDD-first (red confirmed, then green).

- [x] **Radial gradient implemented**: `renderCustomGradient` radial case, `t = hypot(px−start)/|end−start|`, symmetry-pinned test discriminates it from the old linear fallthrough
- [x] **Brush/eraser/clone strokes clip to the active selection**: `paintDabClippedToSelection` wraps every dab (paint/erase/background-erase/clone/history/mixer) — dab footprint snapshot, then per-pixel `lerp(before, after, coverage/255)` matching `fillRasterWithMask`'s 0–255 coverage convention; magic eraser multiplies erase coverage by selection coverage; undo pixel-delta rows stay byte-exact
- [x] **Pixel/all lock enforced for all pixel edits**: centralized `ensureLayerEditable(layer, editKind)` (replaces the position-only helper); enforced at begin-paint-stroke (rejected-stroke pattern, error at stroke end), magic erase, fill/gradient (non-CreateLayer), apply/reapply/preview/commit/fade filter, fill/stroke path, ApplyLayerMask; rejection is a guaranteed no-op (no history entry, no ContentVersion bump). Transparency lock has NO model flag yet (out of scope); transform-commit doesn't check position lock (flagged follow-up)
- [x] **Transforms remap layer masks**: free-transform commit + Transform Again transform raster masks through the same AGG pixel pipeline (fresh scratch state) and vector masks via `freeTransformPointMapper`; discrete flip/rotate remap raster+vector masks with the exact pixel index mappings incl. 90° re-centring; all inside the snapshot command so undo covers the mask. Known gaps (deliberate): live preview shows the mask un-transformed until commit; vacated mask regions retain stale values (invisible, documented)
- [x] **FlattenLayer opacity fixed**: `rasterizeAsPixelLayer` no longer re-applies opacity/fill-opacity after the render already baked them (50%→25% bug); flatten now matches the MergeDown/MergeVisible bake-in convention (also fixes the same double-apply in the shape-rasterize path)
- [x] **Drop shadow falls away from the light**: `dropShadowOffset = (−cos a, +sin a)·d` (y-down; 120° → down-right); shared helper also fixes inner shadow/bevel/satin consistency. DropShadow/OuterGlow now composite **behind** layer content (two-phase `applyLayerStyleEffectsForPlacement`, still backdrop-independent per S.4); layers without behind-content effects keep the cheap copy path
- [x] **Non-Gaussian filters process alpha premultiplied**: box/motion/radial/surface/median/min/max/reduce-noise premultiply → filter all 4 channels → unpremultiply (bit-compatible for opaque input); distort filters (ripple/twirl/offset/polar/lens) displace alpha with RGB; Gaussian upgraded to the same convention; feathered-selection blending is premultiplied too (`lerpRGBAPremul`, integer-exact)
- [x] **Levels/Exposure gamma un-inverted** (`v^(1/gamma)` — gamma 2.0 brightens 128→181) and partial payloads get Photoshop defaults via shared `applyLevelsDefaults` (`{outputBlack:10}` → range [10→255]); caveat: explicit `outputWhite:0` inversion no longer expressible (float64 payload ambiguity, documented)
- [x] **Mouse input paints at full size/flow**: frontend sends pressure 1 unless `pointerType === "pen"` reports real pressure (`strokePressure()` replaced all 8 `event.pressure || 0.5` sites); engine treats pressure ≤ 0 as neutral 1.0
- [x] **drawShape pixels-mode offset fixed**: path cloned and translated by `(−Bounds.X, −Bounds.Y)` before rasterizing into the layer-local buffer; origin layers keep the zero-copy path
- [x] **Path booleans are real geometry**: Combine/Subtract/Intersect/Exclude via pure-Go Clipper2 port `bolom009/go-clipper2` v1.3.0 (Boost 1.0, wasm-safe — resolves the Clipper2 side of the GPC licensing blocker); even-odd inputs, ×1000 int64 coords, beziers flattened at the engine's 16-step convention; Intersect no longer errors; coverage-based tests incl. donut-hole preservation. (In-house `cwbudde/go-clipper2` was evaluated — its fixes are unpushed and the module path is wrong; comment in `path_boolean.go`)
- [x] Blend-mode edge cases: ColorDodge/ColorBurn corners now match the W3C/PS spec order (backdrop check first: dodge(0,1)=0, burn(1,0)=1 — also corrects Vivid Light/Hard Mix corners); `clipColor` guards zero denominators (flattens to lum) so Hue/Sat/Color/Luminosity can never emit NaN
- [x] Minor batch — all done: magic eraser uses `bumpContentVersionRect`; mixer rim dab footprint (`dabFootprintSize`) covers save-rows/dirty/selection regions before painting; Add Noise takes an optional `seed` param (seed 0 = fresh stream per apply — reapply no longer correlates); Fade-with-dissolve seeds with `pixelNoiseSeed(doc coords)` (was flat index ≈ 0 → everything dissolved); `meta()` decomposes rotation vs skew (pure rotation → skew 0/0, pure skews exact; 4-DOF attribution heuristic documented); MergeVisible collapses into the bottommost visible layer's slot (Photoshop/Merge-Down direction), hidden layers keep their positions; `DeletePath` decrements `activePathIdx` when deleting below it; painting on text/vector/adjustment layers returns `layer must be rasterized before painting` over the ABI; `marshalFilterParams` returns an error instead of panicking and `NewLayerID` degrades to a time+counter fallback; `PixelFormat` reports `"rgba8-straight"` (Go + proto); dissolve-in-compositor seeds by doc coords and `pixelNoiseSeed` uses the splitmix64 finalizer (old hash never dissolved near the origin)

> Deferred within S.5 (flagged by review, low): live free-transform preview doesn't move masks until commit; stale mask values at vacated positions (mask-editing phase); transform commits don't check position lock; transparency lock needs a model flag first; `pixelNoiseSeed`-style seeding won't survive a future command-log replay architecture.

### Phase S.6: Text & Vector Completion — ✅ DONE (2026-07-06)

> Executed via 7 TDD implementer subagents in 4 waves (disjoint file ownership) + adversarial review; GSV usage deleted from the engine entirely.

- [x] **Real font engine**: new pure-Go `internal/text` package — `golang.org/x/image/font/sfnt` + 4 embedded DejaVu Sans styles (`codeberg.org/go-fonts/dejavu`, ~2.7 MB raw wasm growth); registry with fallback chain (unknown/"system-ui" → DejaVu Sans), real pair kerning (kern table) + manual `Kerning` (1/1000 em), Bold/Italic via real styled files, SmallCaps as 0.7× uppercase glyphs, BaselineShift/Super/Subscript, post-table underline metrics, AntiAlias mode mapping, glyph/advance/kern caches. Text layers carry a persistent doc-space anchor (`AnchorX/Y/AnchorSet`) and point text gets **tight computed bounds** — fixes the S.2 center/right-alignment clipping caveat. `LoadFontData` (0x0647) registers runtime TTF/OTF (works with no document open, not undoable); `UIMeta.availableFonts` feeds the character panel's new font/style selects. Ligatures/GSUB deferred (documented).
- [x] Real Create Outlines — rewritten on the shared layout pass: closed filled TTF glyph contours (exact quad→cubic, fill-only VectorLayer, ink IoU vs raster > 0.85; the S.1 GSV centerline tracer is gone)
- [x] **Vector masks render**: raster×vector coverage multiply through plain/styled/group composite paths (`effectiveLayerMask`), content-validated raster cache (`PathEqual` + dims — survives in-place transform/crop mutation), empty mask = reveal-all (byte-pinned); `AddVectorMask{fromActivePath}` + new `SetVectorMaskPath` (0x012a)
- [x] Layer styles: gradient overlay/stroke honor user `stops` via 256-LUT renderer (+ real Align, Reverse, Scale, masked Dither; empty stops = legacy ramp, byte-pinned); pattern overlay/stroke sample registry patterns; all five dead params implemented — Contour (bevel+satin LUTs; satin's gaussian default deliberately rebaselined), Noise (deterministic splitmix64), drop-shadow Knockout, bevel Altitude, bevel Technique (smooth/chisel distance ramp)
- [x] Real pattern fill: document-scoped `PatternResource` registry (archived in JSON+ZIP, undoable) + 4 builtin tiles, `DefinePattern` (0x0417, selection bbox or whole layer, 1024² cap) / `DeletePattern` (0x0418), `patternId`/`patternScale` through paint-bucket fill (empty id = legacy fg/bg checker, byte-pinned), `UIMeta.patterns` + minimal picker UI

> Deferred within S.6 (documented in code): GSUB ligatures + GPOS-only kerning; bilinear pattern sampling; underline/strikethrough bars not converted by Create Outlines (glyph ink only); frontend does not yet dispatch LoadFontData for browser/system fonts (→ S.8 candidate).

### Phase S.7: Frontend Architecture & UX Repair — ✅ DONE (2026-07-07)

> Executed via TDD implementer subagents + two-stage review (spec then quality), as in S.6. Part A = 9 targeted fixes; Part B = the App.tsx decomposition (**6,413 → 547 lines**) in 8 steps (B1 helpers, B2 engine subscription layer, B3 8 domain providers [127 state hooks], B4 render sections + `memo(EditorCanvas)`, B5 dialog extraction, B6 context flip [ends per-frame fan-out], B7 `memo(LayerTreeRow)`, B8 verification). Also: untracked `go.work` so the Pages deploy resolves `agg_go` from the published module; fixed a vacuous `typecheck` gate (`tsc --noEmit` on a solution tsconfig checked zero files → now `tsc -b`).

- [x] **Decompose `App.tsx`** — split state into 8 domain providers under `src/state/`; render sections into components/hooks; `EditorCanvas` and `LayerTreeRow` memoized; `render` removed from the engine context so committed frames no longer re-render `useEngine()` consumers (subscription layer + selector hooks `useUiMeta`/`useViewport`/`useEngineRender`, tripwire + render-count tests prove isolation)
- [x] **Engine loading/error UI**: module-level toast bus + `ToastViewport`; `context.run()` and export/import wrappers catch and toast; `EngineLoadErrorScreen` replaces the dead WelcomeScreen on load failure
- [x] **`onPointerCancel`/`lostpointercapture` handling**: `cancelActiveGesture` reverts open drag transactions (engine `CancelTransaction` restores the pre-txn snapshot) and commits partial brush strokes; double-fire guard
- [x] **Layers context menu is dead with a real pointer**: close ownership moved into `LayerContextMenu` with a `contains()` bail; regression test uses the real pointerdown-then-click sequence
- [x] **Paths panel "activate" is a no-op** — new `SetActivePath` (0x0627) command; panel dispatches it (non-undoable, clone+ReplaceActive)
- [x] **Gradient editor regenerates stop IDs** → seed stop state only on closed→open transition (stable ids); **Curves drag** → clamp dragged x between neighbors (no re-sort)
- [x] **Zoom-out shortcut zooms in** — keymap values became distinct action strings (kills the duplicate-case-label class)
- [x] Keyboard hygiene — modal guard, `HTMLSelectElement` in the editable-target check, blur/visibilitychange unstick Space-pan, `actions` in an effect-updated ref (single listener registration)
- [x] Throttle slider dispatches — `useTransactionalSlider` (lazy begin, rAF throttle, one history entry per drag); `parseNumericInput` fixes `Number("")===0` at ~20 sites
- [x] Autosave off the critical path — `use-autosave` idle-schedules + burst-coalesces the export; quota errors toast once (export stays main-thread WASM — documented)
- [x] StrictMode double-init — module-level promise-cached Go runtime; one `go.run`, per-call `EngineInit` handle, failure/ crash resets the cache
- [x] Accessibility floor — Dialog focus trap + Escape + focus restore + aria; menubar roles + arrow-key nav; icon-button aria-labels; `TextEditOverlay` Escape cancels via new `CancelTextEdit` (0x0648)
- [x] Design-token migration — raw slate/blue/cyan/zinc/amber/emerald classes → `@theme` tokens across the panels + App stragglers (color-identity usages kept raw)
- [x] ABI payload hazards — `Button` threaded into the Go pointer payload; `AddLayer.pixels`/`cachedRaster` retyped to base64 `string` in TS

### Phase S.8: Wire Implemented Backend Features into the UI

> Recurring audit finding: engine command exists, tested, and works — frontend never dispatches it. 24 handled-but-never-dispatched commands total.

**Goal:** Finish the remaining engine-to-UI feature slices without hiding multi-day backend, ABI, state-management, and test work behind single checklist items.

**Acceptance criterion:** Every remaining S.8 feature is either usable end-to-end from the UI or deliberately removed from the visible product; all new commands have shared TypeScript/Go contracts, undo/error behavior is explicit, and focused engine plus frontend tests cover the user-visible flow.

- [x] **Filter menu** (2026-07-26) — the entire filter domain (0x0500–0x0505, 26 registered filters, live preview, Ctrl+F reapply, Fade) is now wired. Added TS payload types (`ApplyFilterCommand`/`PreviewFilterCommand`/`FadeFilterCommand`), a static frontend filter catalog mirroring the engine registry (id/name/category/hasDialog + per-filter param schema), a generic catalog-driven `FilterDialog` managing the engine's preview lifecycle (open→PreviewFilter, tweak→re-preview, Preview toggle→Cancel, Apply→CommitFilterPreview / ApplyFilter, Cancel/Escape/unmount→CancelFilterPreview), a `FadeDialog` (opacity + blend mode → FadeFilter), a `FilterStateProvider` (active dialog, last-applied filter, one-shot fade availability), Filter-menu regeneration from the catalog with category section headers, and Ctrl+F (reapply) / Ctrl+Shift+F (fade) shortcuts. TDD: filter-catalog, filter-state reducer, FilterDialog, FadeDialog, MenuPreviewPanel, keyboard-shortcut tests. As a side benefit, `MenuPreviewPanel` now renders the (already-authored) section titles for every multi-section menu.
- [x] **Menu bar de-mock** (2026-08-01) — every visible entry now has a real action: wired Undo/Redo, core adjustment-layer creation, all Layer actions, viewport Zoom/Fit, and Window panel focus with accurate disabled/checked state; wired the shortcuts the menu advertises (Fill, Canvas Size, Reselect, layer commands, modified zoom/fit). Removed deferred Clipboard entries, dead Generate Assets/Image Size/Trim/Pixel Grid/Rulers/workspace/Help placeholders, and the Scale/Rotate/Skew/Distort/Perspective aliases that all mislabeled plain free transform. A menu-model invariant test prevents actionless entries from returning.
- [x] **Clipboard** (2026-08-02) — added instance-scoped pixel clipboard commands (`Copy`/`Cut`/`Paste`, 0x0214–0x0216), selection-aware copy/cut with feather coverage, lock-safe undoable cuts, same-document positioning and cross-document centering on paste, `canPaste` UI metadata, Edit-menu actions with truthful enablement, and Ctrl+X/C/V shortcuts.

#### Phase S.8.1: Adjustment Histograms & Canvas Eyedroppers

**Goal:** Connect the existing histogram and adjustment-sampling commands to Levels, Curves, and Hue/Saturation controls with one coherent canvas interaction mode.

**Acceptance criterion:** Levels displays the requested live histogram, Curves black/white/gray/add-point samplers mutate the selected Curves layer from a canvas click, and the Hue/Saturation sampler selects the identified color range; Escape and tool/layer changes cancel sampling cleanly.

- [x] Complete the shared ABI types that the implemented backend responses currently lack:
  - [x] Add payload types for `ComputeHistogram`, `SetPointFromSample`, and `IdentifyHueRange`
  - [x] Add `HistogramData` and `histogram`/`identifiedHueRange` fields to the TypeScript `RenderResult` contract
  - [x] Verify command errors and command-specific response fields survive the Wasm context merge path
- [x] Build reusable adjustment canvas-sampling state instead of three independent pointer handlers:
  - [x] Track the target adjustment layer, sampler kind, sample size, and cursor
  - [x] Route the next eligible canvas click in document coordinates to the owning adjustment editor
  - [x] Cancel on Escape, panel/layer change, tool change, dialog close, or deleted target layer
  - [x] Avoid collisions with the normal Eyedropper, Color Sampler, pan, and selection modifier gestures
- [x] Wire the Levels histogram display:
  - [x] Request active-layer or merged data and draw the selected RGB/luminance channel behind the Levels controls
  - [x] Refresh on active layer, content version, source, or channel changes without recomputing on every render frame
  - [x] Handle empty/transparent documents and scale bins without losing narrow peaks
- [x] Wire Curves sampling controls for black, white, gray, and add-point modes to `SetPointFromSample`
- [x] Refresh Curves parameters/UIMeta after a sample and preserve channel-specific curve editing and undo semantics
- [x] Wire the Hue/Saturation eyedropper to `IdentifyHueRange`, update the active range selector from the returned value, and keep existing range parameters intact
- [x] Add Go command-contract regression tests where missing and frontend tests for response decoding, histogram refresh keys, sampler lifecycle, cursor state, dispatch payloads, and cancellation

#### Phase S.8.2: Transform Again UI & Command State

**Goal:** Expose the already-implemented `TransformAgain` operation with honest availability and Photoshop-style keyboard access.

**Acceptance criterion:** Edit → Transform Again and its shortcut replay the last committed free or discrete transform on the current eligible layer, remain disabled when replay is impossible, and produce a normal undoable history entry.

- [x] Expose `canTransformAgain` (and any eligibility reason needed by the UI) in `UIMeta` rather than enabling the action optimistically
- [x] Add the Transform Again menu action with accurate disabled state and `Ctrl/Cmd+Shift+T`
- [x] Flush pending pointer input and reject invocation during incompatible in-flight crop, transform, text-edit, or paint states
- [x] Preserve backend errors in the frontend notification path instead of silently closing the menu
- [x] Add menu-model, shortcut, and engine-context tests covering unavailable, free-transform, discrete-transform, and undo cases

#### Phase S.8.3: Brush Tip & Jitter Dynamics Wiring

**Goal:** Make the visible Brush Settings controls affect rendered strokes, beginning with size/opacity/flow jitter and including the adjacent preset fields on which real brush import depends.

**Acceptance criterion:** Size, opacity, and flow jitter visibly and measurably affect dabs; zero jitter remains byte-identical to the current output; tip shape, angle, roundness, and spacing are either engine-backed or removed from the panel; all paint-like tools receive the same applicable settings.

- [x] Audit the complete Brush Settings payload path from `brush-state.tsx` → `CanvasHost` → `EditorCanvas` → `packages/proto` → Go and list every currently decorative field
- [x] Extend the shared `BrushParams` contract with normalized, documented fields for:
  - [x] Size, opacity, and flow jitter
  - [x] Tip shape/preset ID, angle, roundness, and spacing
  - [x] Per-dynamic control-source semantics (`off`, pressure, tilt, fade) without conflicting with the existing pressure toggles
- [x] Define deterministic stroke dynamics:
  - [x] Create a per-stroke random source/seed rather than relying on the package-global `math/rand` stream
  - [x] Sample dynamics once per emitted dab, clamp effective size/opacity/flow, and compute the undo/dirty footprint from the effective size
  - [x] Preserve repeatable tests and stable coalesced-event behavior
- [x] Render supported procedural tip shapes through `agg_go` paths and make angle/roundness affect the dab transform
- [x] Replace the fixed `0.25` dab spacing in the stroke pipeline with validated brush/preset spacing while keeping a safe minimum
- [x] Thread the settings through brush, pencil, eraser, mixer, clone stamp, and history brush paths, explicitly documenting settings that do not apply to a tool
- [x] Ensure preset selection and JSON import/export round-trip every engine-backed field; make unsupported imported fields visible as warnings rather than silently inventing behavior
- [x] Add statistical engine tests for jitter ranges/distribution, byte-parity tests at zero jitter, dirty-rect/selection/undo tests for variable-size dabs, and frontend payload tests for every paint tool
- [x] Re-run paint benchmarks to ensure per-dab dynamics do not regress the S.4 hot path disproportionately

#### Phase S.8.4: Real Navigator Preview & Viewport Control

**Goal:** Replace the static Navigator placeholder with an engine-rendered document preview and an interactive viewport indicator.

**Acceptance criterion:** Navigator shows the active composited document, refreshes after content/document changes without work on idle frames, displays the visible canvas region, and supports click/drag pan plus zoom control.

- [x] Add a dedicated navigator-thumbnail command/response rather than placing base64 pixel data in every `UIMeta`
- [x] Render a bounded, aspect-correct composite thumbnail in Go/Wasm using `agg_go` image scaling and return its dimensions plus RGBA bytes
- [x] Cache the thumbnail by active document ID, content version, requested size, and background mode
- [x] Build a display-only thumbnail canvas component that only blits the engine-owned RGBA result
- [x] Project the current canvas corners into document/thumbnail space and draw an accurate viewport polygon for pan, zoom, and rotated views
- [x] Add click-to-center and drag-to-pan interactions; keep the existing zoom slider synchronized with engine viewport state
- [x] Refresh on panel open, active-document change, content-version change, and panel resize, with request deduplication for React Strict Mode
- [x] Cover transparent documents, extreme aspect ratios, zoom beyond document bounds, rotation, no-active-document state, and stale async responses in Go and frontend tests

#### Phase S.8.5: Multi-Document Sessions & Tab Bar

**Goal:** Turn the existing `DocumentManager` storage into safe user-facing multi-document sessions with switching, closing, and per-document workspace state.

**Acceptance criterion:** Users can keep multiple documents open, switch from a tab bar without losing edits or sibling documents, return to each document's viewport/history/UI state, and close documents with correct active-tab and unsaved-change behavior.

- [x] Define the document-session contract before exposing tabs:
  - [x] Decide which state is per document: viewport, history, selection/edit modes, last transform/filter, clone/history sources, and transient previews
  - [x] Cancel or commit in-flight paint, transform, crop, text, and filter-preview state deterministically before switching
  - [x] Define tab ordering, next-active behavior on close, and whether document creation/opening participates in edit history
- [x] Add a `SwitchDocument` command and shared payload type; expose an ordered document summary list in `UIMeta` with ID, display name, active state, dimensions, and modified/clean state
- [x] Replace the single global viewport/history assumptions with document-keyed session state (or an equivalent ownership model) so switching does not mix undo stacks or camera positions
- [x] Audit all manager-replacing flows:
  - [x] Opening/importing a project or image must add/replace only the intended document rather than discard unrelated open documents
  - [x] Undo/redo and history jump must operate on the active document without unexpectedly activating or mutating a sibling
  - [x] Autosave, Save/Save As, export, thumbnails, filter state, and cached render keys must target the active document ID
- [x] Add an accessible tab bar with active/modified indicators, close buttons, overflow behavior, middle-click close, and `Ctrl/Cmd+Tab` / `Ctrl/Cmd+Shift+Tab` navigation
- [x] Add a close-confirmation flow for modified documents and define the last-document empty-workspace state
- [x] Invalidate/rebuild render, UIMeta, layer-thumbnail, Navigator, and document-sized canvas caches on switch without leaking Wasm buffers
- [x] Add engine tests with at least three documents covering switch/edit/undo/redo/close/reopen/import and the S.2 sibling-preservation regression; add frontend tab, shortcut, confirmation, and active-state tests

#### Phase S.8.6: Atomic Layer-Visibility Solo

**Goal:** Implement the advertised Alt/Option-click eye gesture as one coherent layer-tree operation rather than a burst of frontend visibility commands.

**Acceptance criterion:** Alt/Option-clicking a layer eye isolates the intended layer, preserves the ancestor visibility needed to render it, restores the prior visibility set on the matching gesture, and is undoable as one history step.

- [x] Define solo semantics for nested groups, clipped layers, multi-selection, hidden ancestors, and a second Alt/Option-click
- [x] Add one engine command that captures/applies the full visibility change atomically and records one history entry
- [x] Store the pre-solo visibility set with a document/layer-tree version guard so stale state is never restored after structural edits
- [x] Pass `event.altKey`/`event.metaKey` from the actual eye pointer event without changing normal click behavior or row selection
- [x] Add nested-group engine tests plus Layers-panel tests for normal toggle, solo, restore, undo/redo, and modifier handling

#### Phase S.8.7: Real Contextual Color & Mask Controls

**Goal:** Replace demo toggles and decorative Properties controls with reusable, cancel-safe editors backed by real engine state.

**Acceptance criterion:** Text color and vector fill/stroke open a real color editor with preview/apply/cancel semantics; every visible mask control changes compositing or is removed; no Properties-panel control pretends to edit state it cannot persist.

- [x] Generalize the existing `ColorPickerDialog` from foreground/background global state to a contextual draft/commit API
- [x] Wire Character text color:
  - [x] Open from the current text color, preview through the existing text-layer update command, and restore the original value on Cancel/Escape
  - [x] Preserve alpha and include the change in the correct text-edit/history transaction
- [x] Wire vector fill and stroke independently:
  - [x] Support full RGBA selection plus an explicit None/transparent action
  - [x] Preserve the other paint and stroke width while one color is previewed
  - [x] Re-rasterize through the engine and make Apply/Cancel/undo behavior deterministic
- [x] Decide mask Density/Feather scope from actual engine support:
  - [x] If implemented, add non-destructive mask density/feather fields, commands, `UIMeta`, project serialization, clone/equality/history support, and cache invalidation
  - [x] Apply density and feather in the engine compositor using `agg_go` primitives; if the required filter API is still internal, complete the narrow S.9 export first
  - [x] If deferred, remove the controls and explanatory placeholder rather than leaving disabled decoration
- [x] Audit the rest of Character, Vector Properties, Adjustments/Mask, Channels, and related panels for demo toggles, hard-coded previews, or callbacks that only mutate local display state
- [x] For every audit finding, wire it end-to-end or remove it; add a frontend invariant/test that visible action controls have a state-changing handler
- [x] Add dialog preview/cancel tests, text/vector payload tests, mask render/history/serialization tests when applicable, and accessibility coverage for keyboard and focus return

#### Phase S.8.8: Real ABR Brush Preset Import

**Goal:** Replace filename/name guessing with fixture-driven ABR parsing and render imported brush tips through the engine.

**Acceptance criterion:** Supported `.abr` files import their real preset names, tip images, spacing/shape/dynamics metadata, and usable thumbnails; malformed or unsupported files fail safely with actionable warnings; the regex heuristic is gone.

- [ ] Define the first supported ABR compatibility matrix from licensed or redistributable fixtures (format versions, sampled brushes, computed brushes, compression variants, and descriptor fields)
  - [x] Document and test the synthetic v6/v7, sampled RAW/RLE, computed-descriptor, malformed-input, and resource-limit matrix without claiming external provenance
  - [ ] Validate Photoshop-produced descriptor layouts, sampled-mask polarity, version variants, and compression behavior against a licensed redistributable external fixture corpus
- [x] Move ABR binary parsing into Go/Wasm so decoded tip pixels never become a JavaScript rendering pipeline:
  - [x] Add an import command accepting raw file bytes and a response containing sanitized preset metadata, stable IDs, thumbnails, and per-preset warnings
  - [x] Implement bounds-checked big-endian readers, versioned sections, descriptor/string parsing, sampled-tip dimensions/depth/compression decoding, and strict allocation limits
  - [x] Reject truncated, oversized, or unsupported records without partially registering corrupt presets
- [x] Add an engine brush-resource registry for imported grayscale/alpha tips and computed preset metadata; define app-session/document ownership and cleanup on engine disposal
- [x] Extend `BrushParams`/preset selection to reference an imported tip ID and render the scaled/rotated tip via `agg_go` image/span/mask primitives
- [x] Map supported ABR spacing, diameter, hardness, angle, roundness, and dynamics fields onto the S.8.3 brush model; report unsupported Adobe-only behaviors instead of approximating them silently
- [x] Return engine-generated preview thumbnails for the preset picker; keep React limited to displaying the returned bytes and metadata
- [x] Decide persistence explicitly: retain imported libraries in IndexedDB/app storage and embed only referenced resources in `.agp`, or document a simpler scoped alternative
- [x] Keep JSON preset import as a separate validated format and remove `extractAbrPresetNames` plus all regex-based inference
- [ ] Add golden metadata/tip-decode tests from external fixtures, malformed/truncation/size-limit tests, parser fuzz tests, and frontend import/duplicate/error/persistence tests
  - [x] Cover malformed, truncated, oversized, fuzz-seed, frontend import/error/persistence, and project portability paths with synthetic fixtures
  - [ ] Add golden metadata and tip-decode assertions derived from licensed external ABR files
- [x] Benchmark large brush libraries and sampled-tip strokes; avoid decoding every tip eagerly if fixture measurements justify indexed lazy loading

### Phase S.9: agg_go Upgrade & Alignment

> Recurring audit finding: AGENTS.md mandates that all pixel work go through `agg_go`, but the primitives the engine actually needs for compositing live under `agg_go/internal/` and are unimportable. The engine therefore carries its own per-pixel float64 blend engine, its own layer compositor, and its own viewport resampler — the rule is currently impossible to follow, not merely unfollowed.

> **Completed 2026-08-03:** public APIs released in `agg_go v0.5.0`; engine dependency, compositor, viewport, gradients, and crop resampling migrated; intentional manual loops documented and guarded.

**Goal:** Make the "use agg_go primitives" rule actually satisfiable by exporting the missing public API in `agg_go`, then retire the engine's hand-written pixel loops on the paths where AGG is genuinely equivalent or better — without regressing the S.4 dirty-rect performance work or changing rendered output outside of documented, intentional fixes.

**Acceptance criterion:** Every remaining hand-written pixel loop in the engine is either replaced by a public `agg_go` call or listed on the keep-manual list with a stated reason; blend/composite/resample output is covered by golden-pixel tests that pin behavior across the migration; the S.4 benchmarks show no disproportionate regression; and `agg_go` ships the exports and comp-ops as a tagged release the engine depends on by version, not by `go.work` replace.

**Cross-repo note:** `agg_go` work happens in `../agg_go` (the root `go.work` already `use`s it, so engine builds pick up local changes immediately). `packages/engine-wasm/go.mod` still pins `github.com/cwbudde/agg_go v0.3.2`; each sub-phase that changes `agg_go` must end in a tagged release and a `go.mod` bump so CI — which has no `go.work` — builds the same code. `agg_go v0.4.0` is already tagged and unconsumed; fold that bump into S.9.1.

- [x] **Upgrade `agg_go` v0.2.21 → v0.3.2** — done 2026-07-06; full Go test suite passes, host and js/wasm builds clean. Brings AVX2/SSE2 SIMD blend kernels, the **DstOut comp-op fix (eraser)**, `FillLinearGradientStops`, sRGB/premultiplied-alpha correctness fixes, raster-text overhaul, dashed strokes

#### Phase S.9.1: Public agg_go Compositing, Span & Image-Filter API

**Goal:** Export the narrow, stable surface of `agg_go/internal/{pixfmt,span,image,rasterizer,renderer}` that the engine needs, so compositing and resampling can be written against a public API instead of being reimplemented.

**Acceptance criterion:** The engine can composite an RGBA source rect onto an RGBA destination with a comp-op, opacity, and an alpha mask, and can draw a transformed/filtered image, using only exported `agg_go` identifiers; the exported API is documented, tested in `agg_go`, and released as a tag the engine consumes by version.

- [x] Derive the required surface from real call sites rather than exporting `internal/` wholesale:
  - [x] Inventory what `blend.go`, `layer_ops.go`, `viewport_composite.go`, `fill_gradient.go`, and `crop.go` actually need (comp-op blending on a byte slice, alpha-mask modulated composite, transformed-image span rendering, image filter kernels, multi-stop gradient LUTs)
  - [x] Record which needs are already met by existing public API (`Context.DrawImageTransformed`/`DrawImageQuad`/`DrawImageRegion`, `SetImageFilter`/`SetImageResample`, `AlphaMask`, `FillLinearGradientStops`) so the export list only covers genuine gaps
  - [x] Decide per gap: promote to a public wrapper type, or re-export like `blending.go` re-exports `BlendMode`
  - [x] Decision record: keep existing `Context`/filter APIs intact; add narrow caller-owned-buffer wrappers (`CompositeImage`, `DrawImageAffine`, `RenderGradient`, `GradientLUT`) and additive canonical filter aliases rather than exposing `internal/` packages
- [x] Add the public compositing entry points in `agg_go`:
  - [x] Attach a pixel format to a caller-owned RGBA byte slice with explicit stride, premultiplication state, and bounds
  - [x] Blend a source rect onto a destination rect with comp-op + cover/opacity + optional `AlphaMask`, with clipping handled inside AGG
  - [x] Keep the SIMD blend kernels reachable through the public path so the export does not silently land on the scalar fallback
- [x] Add the public image-filter/span surface: named filter kernels (nearest, bilinear, bicubic, Lanczos where present), resample-vs-non-resample selection, and transformed-image drawing into a caller-owned buffer without a `Context` round-trip when one is not wanted
- [x] Add the public gradient-span surface needed by S.9.6: arbitrary-stop LUT construction plus linear/radial/conical/diamond span generators addressable independently of `Context` fill state
- [x] Define and document the API contract explicitly: premultiplied vs straight alpha, RGBA byte order, stride/offset conventions, clipping behavior, and which calls allocate
- [x] Add `agg_go` tests for every exported symbol (including a parity test that the public path and the pre-existing `internal/` path produce identical bytes) and doc examples for the compositing and transformed-image calls
- [x] Release `agg_go` v0.5.0 with the exports, bump `packages/engine-wasm/go.mod` off v0.3.2, and verify host + `js/wasm` builds and the full Go suite pass without `go.work`
- [x] Re-check the AGENTS.md "Known gaps" table and remove/adjust the "Public color-conversion API" and image-filter rows if this phase closes them

#### Phase S.9.2: Photoshop Comp-Op Coverage in agg_go

**Goal:** Give `agg_go` the 14 Photoshop blend modes the engine's `blend.go` implements privately, so a migrated compositor does not have to fall back to hand-written math for half the mode list.

**Acceptance criterion:** All 27 engine `BlendMode` values in `internal/model/layers.go:35–61` map onto a public `agg_go` comp-op (or are documented as deliberately engine-side), and per-mode pixel parity against the current `blendRGB` implementation is proven by test.

- [x] Establish the reference before writing kernels: extract the current `blendRGB`/`compositePixelWithBlend` behavior into a golden per-mode expected-pixel table (this table is the migration contract for S.9.3)
- [x] Add the separable modes missing from `blending.go`'s re-export list (which today stops at the Porter-Duff + PDF set `BlendAlpha`…`BlendExclusion`): Linear Burn, Linear Dodge, Vivid Light, Linear Light, Pin Light, Hard Mix, Divide, Subtract
- [x] Add the non-separable modes: Hue, Saturation, Color, Luminosity — port `setLuminosity`/`clipColor`/`setSaturation` semantics from the C++ AGG/PDF definitions rather than from the engine copy, then diff against the engine copy
- [x] Add the two special cases explicitly, since neither is a pure per-channel function:
  - [x] Darker Color / Lighter Color operate on whole-pixel luminance, not per channel
  - [x] Dissolve is stochastic — define the randomness contract (caller-supplied per-pixel seed, matching the engine's `pixelNoiseSeed(x, y)` determinism) so results stay reproducible and test-stable
- [x] Decide and document alpha handling for the new modes (premultiplied path, zero-alpha backdrop, out-of-range intermediates) so `NaN`/clamp behavior is specified, not incidental
- [x] Provide SIMD kernels where the mode is cheap enough to vectorize, and verify the scalar and SIMD paths agree bit-for-bit
  - [x] The public uniform-row SrcOver path reaches the existing AVX2/SSE2 kernel and has a forced-scalar byte-for-byte differential test; varying-color and complex Photoshop modes remain exact scalar paths
- [x] Add `agg_go` per-mode tests over the golden table, plus edge-case tests (alpha 0/1, black/white backdrops, saturated channels) and benchmarks for the new comp-ops
- [x] Tag the consolidated `agg_go` v0.5.0 release and bump the engine dependency (completed once in S.9.1 after all cross-repo work)

#### Phase S.9.3: Retire the Engine's Private Blend Engine

**Goal:** Replace `internal/engine/blend.go` (312 lines of per-pixel `float64` math, invoked per pixel from `dispatch_filter.go:448`, `viewport_composite.go:115/152/209`, `brush.go:739`, `fill_gradient.go:253/282`, and `layer_ops.go:1184/1192/1488`) with the public `agg_go` comp-ops from S.9.2.

**Acceptance criterion:** `blendRGB` and its per-mode helpers are deleted; all six call sites go through `agg_go`; the S.9.2 golden table passes unchanged (or every intentional difference is listed with a reason); and the blend-heavy benchmarks do not regress.

- [x] Convert the per-pixel call sites to span/rect-level calls first — the win is as much about leaving the per-pixel function-call granularity as about the math
- [x] Reconcile the two color models: the engine blends in straight-alpha `float64` 0–1, `agg_go` blends premultiplied 8-bit; quantify the rounding delta on the golden table and decide per mode whether to accept it, widen precision (`BlenderRGBA8PlainFixed`, `pixfmt_rgba128`), or keep the mode engine-side
  - [x] Opaque output is exact for all 27 engine modes; translucent output is exact for corrected Soft Light and within one byte for the remaining modes, the expected 8-bit quantization boundary
- [x] Preserve the semantics the engine layered on top of raw blending: `opacity` scaling, `pixelNoiseSeed` determinism for Dissolve, and the BlendIf/mask modulation applied by the callers
- [x] Migrate call sites in isolation-first order — `fill_gradient.go` and `brush.go` (simple, `BlendModeNormal`) → `dispatch_filter.go` → `viewport_composite.go` → `layer_ops.go` — keeping each step independently testable and revertable
- [x] Delete `blend.go` and retarget `blend_test.go` at the new path so the existing mode coverage (including the finiteness fuzz at `blend_test.go:308`) keeps running
- [x] Re-run `BenchmarkRenderPipeline512`, `BenchmarkRenderCompositeSurface`, and `BenchmarkRenderFrameAfterPaintDirtyRect`; record before/after numbers in this plan the way Phase X did
  - [x] 512² paint strokes: 23.6–24.7 ms → 9.2–11.7 ms; pipeline composite: 10.0–11.2 ms → 1.24–1.35 ms; full surface render: 67.4–71.5 ms → 16.3–25.6 ms
  - [x] Dirty viewport at aligned 100%: 185–207 µs → 217–232 µs after warm-up; fractional 137%: 0.50–0.96 ms → 5.1–6.0 ms, still below a 16.7 ms frame and bounded to the dirty region via the premultiplied surface cache

#### Phase S.9.4: Layer Compositor Migration

**Goal:** Move the layer-stack compositor onto `agg_go` surface/mask primitives while preserving the S.4 incremental dirty-rect contract.

**Acceptance criterion:** `compositeRasterIntoDocument` and `compositeDocumentSurfaceClipped` composite through public `agg_go` calls; clipped and full recomposites remain byte-identical to each other; and masks, clipping masks, BlendIf, group isolation, and adjustment-layer caching behave exactly as before.

- [x] Correct the stale plan reference: the per-pixel writers are `layer_ops.go:1184/1192` (raster into document) and `layer_ops.go:1488` (surface composite), not `layer_ops.go:987–1045` — that range is now the `compositeLayerOntoWithClipOptions` dispatcher
- [x] Map each engine concept onto an AGG primitive before touching code: layer mask + vector mask + clip alpha → `AlphaMask` composition; `effectiveLayerOpacity × effectiveContentOpacity` → cover; BlendIf → a derived per-pixel mask or a retained engine-side pre-pass
- [x] Fold the mask chain into a single `AlphaMask` per composite instead of re-multiplying per pixel, and confirm `effectiveLayerMask`'s existing vector-mask folding still short-circuits for empty masks
- [x] Keep the clip-rect path a first-class argument, not an afterthought: AGG clip box must reproduce the current `clip *DirtyRect` semantics, including the documented rule that style surfaces render unclipped and only their final composite is clipped
- [x] Leave the adjustment-layer branch alone in this phase (it is a backdrop transform, not a blend) and re-verify the `allowAdjustmentCache` / `copySurfaceOutsideRect` invariants after migration
- [x] Verify `surfacePool` reuse still holds — AGG attaching to pooled buffers must not retain references past the composite
- [x] Add golden-pixel tests for the compositor before migrating (nested groups, isolated groups, clipping masks, BlendIf, partial opacity, masked + styled layers) — this is the S.11 golden-image work pulled forward where it is a prerequisite
- [x] Add an equivalence test asserting clipped incremental recomposite == full recomposite for randomized dirty rects

#### Phase S.9.5: Viewport Resampler Migration

**Goal:** Replace the three hand-written viewport samplers in `viewport_composite.go` with a single AGG transformed-image draw.

**Acceptance criterion:** `compositeViewportIdentity` (`:130`), `compositeViewportBilinearUnrotated` (`:218`), and `compositeViewportBilinearRotated` (`:329`) collapse into one transform-driven path; pan/zoom/rotate output is at least as good as today; and the zoom benchmarks do not regress.

- [x] Express the viewport as a single affine transform (pan + zoom + rotation about the canvas center) and drive `DrawImageTransformed`/`DrawImageQuad` with it, replacing the three special-cased loops and `docBoundsOnCanvas`'s manual corner projection
- [x] Preserve the identity-zoom fast path deliberately: measure whether AGG's setup cost beats the current straight copy at zoom 1 and keep the manual path if it does not (the Phase X.7 policy already establishes that AGG setup cost can dominate for cheap ops)
- [x] Choose filters per zoom regime (nearest for ≥100% pixel-accurate inspection, resampling filter for minification) and expose that choice where the UI's existing zoom semantics expect it
  - [x] Policy selected from the existing editor behavior: bilinear below 4× or whenever rotated; nearest at unrotated zoom ≥4× for pixel inspection
- [x] Preserve `storeBilinearPixel`'s premultiplied handling and the checkerboard/transparency backdrop composition order
- [x] Confirm the clip-rect argument still restricts work to the dirty region — the S.4 win is that a paint frame resamples O(dirty), and an unclipped AGG draw would silently undo it
- [x] Re-run `BenchmarkCompositeViewportZoom1` and `BenchmarkViewportZoomScenarios512` across zoom levels and rotation on/off; keep whichever path wins per regime and document the split
  - [x] Aligned identity stays a measured manual row-copy fast path (0.28–0.73 ms, zero allocations); transformed views use the unified AGG affine path, with interactive dirty frames fed by a cached premultiplied document surface
- [x] Add golden tests for zoom in/out, fractional zoom, rotated views, and document edges partially off-canvas

#### Phase S.9.6: Gradient & Crop Resampling Migration

**Goal:** Route the remaining two manual sampling paths — custom gradient rendering and crop/content-aware resampling — through `agg_go` span generators and image filters.

**Acceptance criterion:** `renderCustomGradient` produces its output from AGG gradient spans, rotated crop and content-aware fill sample through AGG image filters, and gradient dithering/banding behavior is preserved or improved.

- [x] Correct the stale plan references: `renderCustomGradient` is `fill_gradient.go:450–496` (with `buildGradientLUT` at `:355`), and crop resampling is `applyRotatedCropToPixelLayer` (`crop.go:470`) plus `buildContentAwareCropFillLayer` (`crop.go:500`), both via `sampleBilinear`
- [x] Map `buildGradientLUT`'s 256-entry `agglib.Color` table onto the S.9.1 public gradient-LUT API instead of hand-indexing it in `gradientColorAt`
- [x] Cover every gradient type the UI offers (linear, radial, angle, reflected, diamond) with an AGG span generator; add any missing generator to `agg_go/internal/span/` following the AGENTS.md porting rules rather than keeping an engine-side special case
- [x] Decide the fate of `applyGradientDither` (`fill_gradient.go:523`): keep it as a post-pass, or fold it into the span path if AGG's higher-precision LUT already removes the banding it exists to hide — test the banding case either way
  - [x] Dither is folded into `RenderGradient`; tests pin deterministic RGB jitter while preserving alpha exactly
- [x] Replace `sampleBilinear` in the rotated-crop path with an AGG transformed-image draw, keeping the pixel-center convention (`+0.5`) that the current call sites depend on
- [x] Route content-aware crop-fill sampling through the same filter path, leaving `diffuseCropExpansion` (`crop.go:562`) manual — it is a diffusion solver, not a resample
- [x] Add golden tests for each gradient type (including multi-stop and alpha stops), rotated crops at several angles, and crop expansion into unknown regions

#### Phase S.9.7: Keep-Manual Boundary & Enforcement

**Goal:** Turn the keep-manual list from a plan footnote into a stated, reviewable policy so the next audit does not re-flag intentional decisions as violations.

**Acceptance criterion:** Every remaining hand-written pixel loop in the engine is either on the documented keep-manual list with a measured or structural justification, or has a ticketed migration; AGENTS.md reflects the real rule; and new manual loops are visible in review.

- [x] Re-validate the keep-manual list against the post-migration code: flips/rotate90/180, discrete remaps, transform overlay (Phase X.7 policy), `EraseBackgroundDab` tolerance erase, `diffuseCropExpansion`
- [x] Give each entry a reason of record — measured (AGG setup cost exceeds the op, per X.7 benchmarks) or structural (not a rasterization problem) — rather than "kept manual"
- [x] Sweep the engine for pixel loops not on the list after S.9.3–S.9.6 and classify each as migrate/keep/ticket
  - [x] Reason-of-record and ticket ledger lives in `internal/engine/PIXEL_LOOP_POLICY.md`; open groups are `S9-MASK-COMPOSITE`, `S9-BRUSH-SAMPLING`, `S9-IMAGE-SCALE`, `S9-FILTER-DISTORT`, `S9-ALPHA-CONVERSION`, and `S9-STYLE-SPANS`
- [x] Update the AGENTS.md agg_go section so the mandate names the public API that now exists and cites the keep-manual exceptions
- [x] Add a lightweight guard (lint rule, test, or documented review checklist item) that surfaces new direct per-pixel writes in `internal/engine` so the drift is caught at review time
  - [x] `TestManualPixelLoopAllowlist` parses production engine files, rejects unreviewed adjacent RGBA writes in loops, verifies allowlisted functions still exist, and documents the syntax-only review fallback

### Phase S.10: PSD Interop Repair

> Current state: PSD I/O is effectively an Agogo↔Agogo container. Round-trip tests pass only because reader and writer share the same bugs and fidelity rides on an embedded `AgogoProject` JSON block — there are zero external fixtures.

**Goal:** Turn PSD from a private container into real interop: files Photoshop wrote open correctly, files Agogo writes open correctly in Photoshop, and the embedded-project shortcut becomes an optional fidelity bonus rather than the thing that makes round-trips pass.

**Acceptance criterion:** A corpus of genuine Photoshop-exported fixtures imports with correct layer structure, masks, blend modes, and pixels — **with the `AgogoProject` resource stripped**; Agogo-written PSDs open in Photoshop (or a third-party reader) with the same structure; the parser survives fuzzing and hostile length fields without OOM; and every remaining fidelity gap is a recorded warning, not a silent loss.

**Ordering note:** S.10.1 (fixtures) gates everything else. Every other sub-phase is currently unfalsifiable — reader and writer share their bugs, so the existing round-trip tests pass on wrong data. Do not "fix" the semantics sub-phases before there is an external file to be wrong against.

#### Phase S.10.1: External Fixtures & Honest Round-Trip Testing

**Goal:** Establish ground truth. Today there is not a single `.psd`/`.psb` file or `testdata/` directory anywhere in `packages/engine-wasm`, so no test can distinguish "correct" from "self-consistent".

**Acceptance criterion:** A licensed fixture corpus lives in the repo with provenance; import assertions compare against externally-derived expectations, not against Agogo's own writer; and the `AgogoProject` bypass is disabled in fixture tests.

- [ ] Close the bypass that makes current round-trip tests vacuous:
  - [ ] `psd_reader.go:32–38` returns `LoadProject(resources.AgogoProject)` and never reaches `loadPSDFallback` when the resource is present — the real parser is untested on any file Agogo wrote
  - [ ] Add a test-only (or explicit-option) path that forces the spec parser regardless of the embedded resource, and run the entire existing round-trip suite through it
  - [ ] Decide the product rule: is `AgogoProject` a fidelity bonus layered *on top of* a correct parse, or a replacement for it? Document the answer — it determines whether unsupported PSD features may silently ride the JSON block
- [ ] Build the fixture corpus with licensing recorded per file (self-authored in Photoshop, CC0, or explicitly redistributable — no scraped sample files)
- [ ] Cover the matrix that actually exercises the parser: 8-bit RGB and grayscale; RAW/RLE/ZIP/ZIP-with-prediction compression; nested groups; layer masks with non-zero offsets; clipping masks; all blend modes; adjustment and text layers; layer effects; PSB (>30000px) and a PSD near the dimension limit
- [ ] Derive expectations independently — record per-fixture expected layer tree, bounds, blend modes, and sampled pixel values from Photoshop or a known-good third-party reader, not from Agogo's output
- [ ] Add the writer half: export each fixture and verify the bytes reopen correctly in a reader that is not Agogo's own; document the verification method (manual Photoshop check is acceptable if recorded, but automate what can be automated)
- [ ] Add a golden-warning assertion per fixture so a regression that starts silently dropping data shows up as a changed warning set
- [ ] Keep fixture size bounded (small canvases, few layers) so the corpus does not bloat the repo or CI

#### Phase S.10.2: Compression & Channel Decode Correctness

**Goal:** Make the byte-level channel decoders spec-conformant for every compression scheme, and verify against fixtures rather than against Agogo's encoder.

**Acceptance criterion:** RAW, RLE, ZIP, and ZIP-with-prediction channels decode correctly from real Photoshop files for both layer channels and the composite image, and Agogo's encoder produces bytes those same decoders and Photoshop accept.

- [ ] **Update stale status:** the compression constants are already fixed on `codex/fix-psd-compression-ids` (`types.go:14–18` now reads Raw=0/RLE=1/Zip=2/ZipPred=3, commit `8abfe00`), and the ZIP channel payload read was fixed in `0727dc9` — verify both against S.10.1 fixtures, since neither fix has ever been tested against a genuine Photoshop file
- [ ] Validate `decodeZipChannel` / `decodeZipImageData` / `applyZipPredictionInPlace` (`pixels.go:140–207`) on real ZIP and ZIP-prediction fixtures, including the prediction reset at each row boundary
- [ ] Validate RLE/PackBits both directions: `DecodePackBits` (`helpers.go:80`) against Photoshop-written scanlines, and `EncodePackBitsRow` (`helpers.go:351`) for the pathological runs (alternating bytes, 128-byte runs, single-byte rows) where PackBits encoders classically go wrong
- [ ] Verify the per-row byte-count table is read and written with the right width for PSD vs PSB (2 bytes vs 4) — a PSB regression here is invisible until a large file is opened
- [ ] Handle bit depths beyond 8 explicitly: either implement 16/32-bit channel decode or reject with a clear, actionable error instead of misreading the data
- [ ] Add per-compression decode tests over fixtures and an encode→decode→compare test for each scheme

#### Phase S.10.3: Layer Group & Section-Divider Semantics

**Goal:** Reconstruct the layer tree the way Photoshop actually encodes it, instead of the current inverted reading.

**Acceptance criterion:** Nested groups from Photoshop fixtures import with correct nesting, order, and group attributes; Agogo-written groups reopen in Photoshop with the same structure; and unbalanced markers degrade to a warning plus a flat-but-complete tree, never to dropped layers.

- [ ] Fix the inverted divider handling in `psdimport/import.go:61–67`: PSD stores layers bottom-to-top, so in file order the type-3 bounding divider marks the group's *bottom* and the type-1/2 record marks its *top*. The current code treats type 1 (open folder) **and** type 3 (bounding divider) as "begin group" and type 2 (closed folder) as "close" — so a collapsed folder is misread as an end marker and a divider as a start
- [ ] Preserve the open/closed distinction (type 1 vs 2) as group expanded state rather than discarding it
- [ ] Fix the writer side to match: `buildSectionDivider` (`writer.go:128`) emits only the 4-byte section type, while Photoshop's `lsct` block is 12+ bytes including the blend-mode signature and sub-type — verify against what Photoshop accepts
- [ ] Ensure the writer emits the bounding-divider record for each group, not just the folder record, and in the correct bottom-to-top order
- [ ] Carry group attributes through both directions: visibility, opacity, blend mode, clipping (`ClipToBelow`), and pass-through vs isolated blending (pass-through is `passthru`, not a normal blend key — confirm it survives)
- [ ] Make unbalanced markers non-destructive: the current `popStack` failure path (`import.go:30–40`) and the trailing unclosed-group loop should always yield every layer, flattened if necessary, with a warning
- [ ] Add fixture tests for deep nesting, adjacent sibling groups, a group as the first/last layer, empty groups, and clipping across a group boundary

#### Phase S.10.4: Layer Masks (Read & Write)

**Goal:** Make layer masks actually survive a round trip — today they are recognized, sized, and then thrown away.

**Acceptance criterion:** A Photoshop layer mask imports with correct pixels, offset, enabled state, and default fill; Agogo-written masks reopen with the same; and a masked layer renders identically to Photoshop's flattened composite within a stated tolerance.

- [ ] Fix mask channel decode dimensions: `parser.go:212` decodes **every** channel at the layer's `Bounds.W/H`, but the user-mask channel (ID −2) has its own `LayerMaskBounds` — mask pixels are currently decoded at the wrong size
- [ ] Assign the decoded pixels: `import.go:93–98` builds a `model.LayerMask` with `Enabled`/`Width`/`Height` but never sets `Data`, so every imported mask is empty
- [ ] Handle the real-mask flag set: disabled (`flags&0x0001`, already read at `parser.go:403`), inverted, "mask from vector data", and the default color/fill byte for pixels outside the mask rect
- [ ] Handle the mask rectangle honestly — masks are stored in document space with their own offset, independent of the layer bounds; decide whether the model stores document-space or layer-relative masks and convert once, in one place
- [ ] Fix the writer: `writeLayerMaskData` (`writer.go:101–119`) writes the rect as `(0, 0, Height, Width)` — discarding the mask offset — and emits no channel −2 data at all, so exported masks are a rectangle with no pixels
- [ ] Support the real-mask/user-mask channel pair (−2 and −3) at least well enough to not misattribute one as the other
- [ ] Add fixture tests for offset masks, disabled masks, inverted masks, masks larger and smaller than their layer, and a mask on a group

#### Phase S.10.5: Complete Blend-Mode Mapping

**Goal:** Map all 27 engine blend modes to their PSD keys in both directions.

**Acceptance criterion:** Every `model.BlendMode` round-trips through `BlendKey`→`MapBlendMode` unchanged; unknown incoming keys produce a warning rather than a silent Normal; and a fixture using every Photoshop blend mode imports with all 27 correct.

- [ ] Extend `MapBlendMode` (`helpers.go:121–140`) beyond its current 7 keys (`mul`, `scrn`, `over`, `diff`, `smud`, `dark`, `lite`) to the full Photoshop set — Dissolve `"diss"`, Linear Burn `"lbrn"`, Darker Color `"dkCl"`, Color Dodge `"div "`, Linear Dodge `"lddg"`, Lighter Color `"lgCl"`, Soft Light `"sLit"`, Hard Light `"hLit"`, Vivid Light `"vLit"`, Linear Light `"lLit"`, Pin Light `"pLit"`, Hard Mix `"hMix"`, Subtract `"fsub"`, Divide `"fdiv"`, Hue `"hue "`, Saturation `"sat "`, Color `"colr"`, Luminosity `"lum "`, Color Burn `"idiv"`, plus pass-through `"pass"` (note the space-padded four-byte keys)
- [ ] Mirror the same table in `BlendKey` (`helpers.go:584–603`) — the two functions must be one table, not two switch statements that can drift
- [ ] Respect the 4-byte fixed-width key convention (trailing spaces are significant on write, trimmed on read) and verify Agogo's written keys are byte-exact
- [ ] Replace the silent `default: BlendModeNormal` fallback with a recorded warning naming the unmapped key
- [ ] Add a table-driven exhaustive round-trip test over all 27 `model.BlendMode` values plus a fixture whose layers use every Photoshop mode

#### Phase S.10.6: Parser Hardening Against Hostile Input

**Goal:** Treat PSD parsing as what it is — untrusted binary input parsed inside the user's browser tab — and make it fail safely.

**Acceptance criterion:** No crafted or truncated file can OOM, panic, or hang the editor; every allocation derived from a file field is bounded by remaining input; and `Fuzz*` targets run in CI with a seed corpus.

- [ ] Bound every length-driven allocation against actual remaining bytes: `readBytesFrom` (`helpers.go:166`) currently does `make([]byte, n)` after only a negative check, so a 4-byte field can request gigabytes
- [ ] Audit the same pattern across all readers — section lengths (`readSectionLengthFrom`), channel lengths, descriptor strings, Unicode strings (`ParseUnicodeString`, `parseUnicodeStringFromReader`), Pascal strings, and the additional-layer-info block loop
- [ ] Bound decompression output, not just input: zlib and PackBits are both expansion vectors, so cap decoded size by the expected pixel count before decoding rather than after
- [ ] Enforce total document limits consistently — `PSDMaxDimension` exists at `types.go:37`; verify width × height × channels × depth is checked before any allocation, including for PSB
- [ ] Guard the structural loops against non-termination: layer counts, channel counts, and the additional-info key loop must all make forward progress or abort
- [ ] Add `Fuzz*` targets for `Parse`, `ParseLayerAndMaskInfo`, `ParseCompositeImageData`, `DecodePackBits`, and the descriptor reader; seed the corpus from S.10.1 fixtures and check in any crashers found
- [ ] Convert panics to errors at the package boundary so a malformed file surfaces as a user-facing message, never as a Wasm trap that kills the engine instance
- [ ] Add explicit truncation tests: every fixture cut at many offsets must produce an error or partial-with-warnings result, never a panic

#### Phase S.10.7: Real Adjustment, Text & Effect Reconstruction

**Goal:** Replace the JSON-in-descriptor pseudo-format and the flatten-to-pixels fallback with spec-conformant descriptors, so adjustment layers, text layers, and effects survive interop with Photoshop rather than only with Agogo.

**Acceptance criterion:** Photoshop adjustment and text layers import as live, editable layers of the right type; Agogo-written ones open in Photoshop as native adjustment/text layers; and the `AgAJ` JSON block is gone or demoted to a redundant fidelity extra.

- [ ] Remove the private pseudo-format: `metadata.go:235–252` stores Agogo state as JSON inside an `AgAJ` block — no other application can read it, and its presence hides how little of the real format is implemented
- [ ] Parse the genuine adjustment blocks into live `AdjustmentLayer`s: `levl` (Levels), `curv` (Curves), `hue2` (Hue/Saturation), plus `brit`, `blnc`, `blwh`, `mixr`, `phfl`, `selc`, `thrs`, `post`, `nvrt`, and the solid/gradient/pattern fill types
- [ ] Implement a real descriptor reader/writer for the ones that need it (Photoshop moved most adjustments to descriptor form) rather than special-casing each binary layout
- [ ] Reconstruct text layers from `TySh` — transform matrix, the engine data (`Txt2`) or classic style-run structures, font/size/color/tracking/leading — instead of the current flatten-to-pixels path that emits `"unsupported metadata block TySh imported as flattened pixel layer"` (`import.go:86–88`)
- [ ] Reconstruct layer effects from `lfx2`/`lrFX` into the engine's style stack, mapping drop shadow, inner shadow, outer/inner glow, stroke, and overlays to the S.5 style model — and warn on effects the engine cannot represent
- [ ] Define the fallback contract explicitly: when reconstruction is impossible, import the flattened pixels **and** record a warning naming the block, so data is never silently dropped or silently misrepresented as editable
- [ ] Keep smart objects (`SoLd`/`PlLd`) explicitly out of scope for this phase, but make their warning accurate about what was lost
- [ ] Add fixture tests per adjustment type, per text-layer style variation, and per effect, asserting the reconstructed layer's parameters — not just that some layer appeared

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

- [ ] Blend engine is a manual per-pixel float64 implementation bypassing agg_go (→ S.9); ~~dodge/burn extremes deviate from spec; `clipColor` NaN on lum==min~~ — fixed in S.5 (2026-07-06)
- [x] `FlattenLayer` double-applies opacity/fill-opacity; MergeVisible loses hidden-layer z-order — fixed in S.5 (2026-07-06)
- [x] Pixel/all layer locks are not enforced for painting, fills, or filters — fixed in S.5 (2026-07-06)
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

- [x] Transforms (free transform and discrete flip/rotate) do not transform layer masks — fixed in S.5 (2026-07-06; live-preview mask still static until commit, see S.5 deferred notes)
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
  - [x] Types: Linear, Radial, Angle, Reflected, Diamond — Radial implemented in S.5 (2026-07-06)
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

- [x] Brush/eraser/clone strokes ignore the active selection — fixed in S.5 (2026-07-06)
- [x] Mouse input paints at 75% size / 50% flow — fixed in S.5 (2026-07-06; pointerType-gated pressure)
- [ ] Brush dynamics jitter sliders + control-source dropdown are display-only — values never reach the engine, no proto fields exist (→ S.8)
- [ ] `.abr` import is a filename-regex heuristic, not a format parser (→ S.8)
- [ ] Gradient "fill layer" is rasterized, not a non-destructive parametric layer; opacity stops are folded into stop colors, not independent
- [x] Paint-bucket pattern fill hardcoded checkerboard — fixed in S.6 (pattern registry + DefinePattern, 2026-07-06); still open: LAB sliders missing; "gamut warning" is a web-safe-color label
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
- [x] Levels/Exposure gamma convention is inverted vs Photoshop; output-white default trap — fixed in S.5 (2026-07-06)
- [ ] Curves black/white/gray eyedropper (`SetPointFromSample`) and Hue/Sat range eyedropper (`IdentifyHueRange`) exist in the engine but are never dispatched (→ S.8)
- [x] Non-Gaussian filters average RGB only and copy original alpha — fixed in S.5 (2026-07-06; premultiplied 4-channel filtering)
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
  - [x] Combine (union), Subtract, Intersect, Exclude — real boolean geometry via Clipper2 since S.5 (2026-07-06); ~~Divide~~ (no engine op exists)
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
  - [x] Engine-side runtime font registration: `LoadFontData` ABI (S.6, 2026-07-06) — frontend `FontFace`/system-font wiring still open (→ S.8)
  - [x] Font catalog: `UIMeta.availableFonts` + character-panel selects (S.6) — visual preview still open
  - [ ] Web font loading from URL (later)
- [x] **Text rendering via AGG:**
  - [x] ~~Rasterize text via AGG `FontGSV`~~ superseded in S.6: sfnt glyph outlines filled via AGG scanline (GSV removed)
  - [x] Load TrueType/OpenType outlines via `golang.org/x/image/font/sfnt` + embedded DejaVu Sans ×4 (S.6, 2026-07-06)
  - [x] Kerning (kern table + manual) and float-accurate glyph placement (S.6) — ligatures deferred (no GSUB in sfnt)
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

- [x] Drop shadow falls *toward* the light at the PS-default 120° angle, and DropShadow/OuterGlow composite on top of layer content instead of behind it — fixed in S.5 (2026-07-06)
- [x] Gradient Overlay hardcoded ramp / Pattern Overlay + pattern stroke hardcoded checkerboards — fixed in S.6 (user stops via LUT renderer; registry patterns; 2026-07-06)
- [x] Contour, Noise, Knockout, Altitude, and bevel Technique params — implemented in S.6 (2026-07-06)

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
