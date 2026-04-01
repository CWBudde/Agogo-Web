package engine

import (
	"math"
	"testing"

	aggrender "github.com/MeKo-Tech/agogo-web/packages/engine-wasm/internal/agg"
)

const benchmarkCanvasSize = 512

type benchmarkStrokePoint struct {
	X        float64
	Y        float64
	Pressure float64
}

type benchmarkStroke struct {
	Brush  BrushParams
	Points []benchmarkStrokePoint
}

type renderBenchmarkFixture struct {
	inst          *instance
	doc           *Document
	layer         *PixelLayer
	pristineLayer []byte
	strokes       []benchmarkStroke
}

func BenchmarkRenderPipeline512(b *testing.B) {
	b.Run("PaintStrokes", func(b *testing.B) {
		fixture := newRenderBenchmarkFixture()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			b.StopTimer()
			fixture.resetToEmpty()
			b.StartTimer()

			fixture.paintAllStrokes()
		}

		b.StopTimer()
		if fixture.doc.ContentVersion == 0 {
			b.Fatal("expected painted document content version to advance")
		}
	})

	b.Run("CompositeSurface", func(b *testing.B) {
		fixture := newRenderBenchmarkFixture()
		fixture.preparePaintedDocument()
		b.ReportAllocs()
		b.SetBytes(benchmarkCanvasBytes())
		b.ResetTimer()

		var surface []byte
		for i := 0; i < b.N; i++ {
			surface = fixture.doc.renderCompositeSurface()
		}

		b.StopTimer()
		assertBenchmarkSurfaceLen(b, len(surface))
	})

	b.Run("AffineTransformCommit", func(b *testing.B) {
		fixture := newRenderBenchmarkFixture()
		fixture.preparePaintedDocument()
		for _, interp := range []InterpolMode{InterpolNearest, InterpolBilinear, InterpolBicubic} {
			b.Run(string(interp), func(b *testing.B) {
				state := fixture.affineTransformState(interp)
				warmPixels, warmBounds := applyPixelTransform(state, interp)
				if len(warmPixels) == 0 || warmBounds.W <= 0 || warmBounds.H <= 0 {
					b.Fatal("expected affine transform benchmark setup to produce pixels")
				}

				b.ReportAllocs()
				b.SetBytes(int64(warmBounds.W * warmBounds.H * 4))
				b.ResetTimer()

				var outPixels []byte
				var outBounds LayerBounds
				for i := 0; i < b.N; i++ {
					outPixels, outBounds = applyPixelTransform(state, interp)
				}

				b.StopTimer()
				if got, want := len(outPixels), outBounds.W*outBounds.H*4; got != want {
					b.Fatalf("buffer length = %d, want %d", got, want)
				}
			})
		}

		b.Run("AffineTransformCommitIntegerTranslate", func(b *testing.B) {
			fixture := newRenderBenchmarkFixture()
			fixture.preparePaintedDocument()
			state := fixture.integerTranslateTransformState(13, -9, InterpolNearest)
			warmPixels, warmBounds := applyPixelTransform(state, InterpolNearest)
			if len(warmPixels) == 0 || warmBounds.W != fixture.layer.Bounds.W || warmBounds.H != fixture.layer.Bounds.H {
				b.Fatal("expected integer-translate transform benchmark setup to produce source-sized pixels")
			}

			b.ReportAllocs()
			b.SetBytes(int64(warmBounds.W * warmBounds.H * 4))
			b.ResetTimer()

			var outPixels []byte
			var outBounds LayerBounds
			for i := 0; i < b.N; i++ {
				outPixels, outBounds = applyPixelTransform(state, InterpolNearest)
			}

			b.StopTimer()
			if got, want := len(outPixels), outBounds.W*outBounds.H*4; got != want {
				b.Fatalf("buffer length = %d, want %d", got, want)
			}
		})

		b.Run("AffineTransformCommitAxisAlignedScale", func(b *testing.B) {
			fixture := newRenderBenchmarkFixture()
			fixture.preparePaintedDocument()
			state := fixture.axisAlignedScaleTransformState(0.84, 1.12, 18, -26, InterpolBilinear)
			warmPixels, warmBounds := applyPixelTransform(state, InterpolBilinear)
			if len(warmPixels) == 0 || warmBounds.W <= 0 || warmBounds.H <= 0 {
				b.Fatal("expected axis-aligned transform benchmark setup to produce pixels")
			}

			b.ReportAllocs()
			b.SetBytes(int64(warmBounds.W * warmBounds.H * 4))
			b.ResetTimer()

			var outPixels []byte
			var outBounds LayerBounds
			for i := 0; i < b.N; i++ {
				outPixels, outBounds = applyPixelTransform(state, InterpolBilinear)
			}

			b.StopTimer()
			if got, want := len(outPixels), outBounds.W*outBounds.H*4; got != want {
				b.Fatalf("buffer length = %d, want %d", got, want)
			}
		})
	})

	b.Run("PerspectiveTransformCommit", func(b *testing.B) {
		fixture := newRenderBenchmarkFixture()
		fixture.preparePaintedDocument()
		for _, interp := range []InterpolMode{InterpolNearest, InterpolBilinear, InterpolBicubic} {
			b.Run(string(interp), func(b *testing.B) {
				state := fixture.perspectiveTransformState(interp)
				warmPixels, warmBounds := applyPixelTransform(state, interp)
				if len(warmPixels) == 0 || warmBounds.W <= 0 || warmBounds.H <= 0 {
					b.Fatal("expected perspective transform benchmark setup to produce pixels")
				}

				b.ReportAllocs()
				b.SetBytes(int64(warmBounds.W * warmBounds.H * 4))
				b.ResetTimer()

				var outPixels []byte
				var outBounds LayerBounds
				for i := 0; i < b.N; i++ {
					outPixels, outBounds = applyPixelTransform(state, interp)
				}

				b.StopTimer()
				if got, want := len(outPixels), outBounds.W*outBounds.H*4; got != want {
					b.Fatalf("buffer length = %d, want %d", got, want)
				}
			})
		}
	})

	b.Run("WarpTransformCommit", func(b *testing.B) {
		fixture := newRenderBenchmarkFixture()
		fixture.preparePaintedDocument()
		for _, interp := range []InterpolMode{InterpolNearest, InterpolBilinear, InterpolBicubic} {
			b.Run(string(interp), func(b *testing.B) {
				state := fixture.warpTransformState(interp)
				warmPixels, warmBounds := applyPixelTransform(state, interp)
				if len(warmPixels) == 0 || warmBounds.W <= 0 || warmBounds.H <= 0 {
					b.Fatal("expected warp transform benchmark setup to produce pixels")
				}

				b.ReportAllocs()
				b.SetBytes(int64(warmBounds.W * warmBounds.H * 4))
				b.ResetTimer()

				var outPixels []byte
				var outBounds LayerBounds
				for i := 0; i < b.N; i++ {
					outPixels, outBounds = applyPixelTransform(state, interp)
				}

				b.StopTimer()
				if got, want := len(outPixels), outBounds.W*outBounds.H*4; got != want {
					b.Fatalf("buffer length = %d, want %d", got, want)
				}
			})
		}
	})

	b.Run("DiscreteTransforms", func(b *testing.B) {
		makePixels := func() []byte {
			return make([]byte, benchmarkCanvasSize*benchmarkCanvasSize*4)
		}
		for _, kind := range []string{"flipH", "flipV", "rotate90cw", "rotate90ccw", "rotate180"} {
			b.Run(kind, func(b *testing.B) {
				pixels := makePixels()
				w, h := benchmarkCanvasSize, benchmarkCanvasSize
				b.ReportAllocs()
				b.SetBytes(int64(len(pixels)))
				b.ResetTimer()

				for i := 0; i < b.N; i++ {
					switch kind {
					case "flipH":
						_ = flipPixelsH(pixels, w, h)
					case "flipV":
						_ = flipPixelsV(pixels, w, h)
					case "rotate90cw":
						_, _, _ = rotatePixels90CW(pixels, w, h)
					case "rotate90ccw":
						_, _, _ = rotatePixels90CCW(pixels, w, h)
					case "rotate180":
						_ = rotatePixels180(pixels, w, h)
					}
				}
			})
		}
	})

	b.Run("RenderViewportAggBase", func(b *testing.B) {
		fixture := newRenderBenchmarkFixture()
		fixture.preparePaintedDocument()
		pixels := make([]byte, benchmarkCanvasSize*benchmarkCanvasSize*4)
		b.ReportAllocs()
		b.SetBytes(benchmarkCanvasBytes())
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			pixels = fixture.renderViewportBase(pixels)
		}

		b.StopTimer()
		assertBenchmarkSurfaceLen(b, len(pixels))
	})

	b.Run("RenderViewportAggOverlays", func(b *testing.B) {
		fixture := newRenderBenchmarkFixture()
		fixture.preparePaintedDocument()
		pixels := fixture.renderViewportBase(make([]byte, benchmarkCanvasSize*benchmarkCanvasSize*4))
		b.ReportAllocs()
		b.SetBytes(benchmarkCanvasBytes())
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			pixels = fixture.renderViewportOverlays(pixels)
		}

		b.StopTimer()
		assertBenchmarkSurfaceLen(b, len(pixels))
	})

	b.Run("RenderViewportAggOnly", func(b *testing.B) {
		fixture := newRenderBenchmarkFixture()
		fixture.preparePaintedDocument()
		pixels := make([]byte, benchmarkCanvasSize*benchmarkCanvasSize*4)
		b.ReportAllocs()
		b.SetBytes(benchmarkCanvasBytes())
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			pixels = RenderViewport(fixture.doc, &fixture.inst.viewport, pixels, nil)
		}

		b.StopTimer()
		assertBenchmarkSurfaceLen(b, len(pixels))
	})

	b.Run("RenderViewport", func(b *testing.B) {
		fixture := newRenderBenchmarkFixture()
		fixture.preparePaintedDocument()
		documentSurface := fixture.doc.renderCompositeSurface()
		pixels := make([]byte, benchmarkCanvasSize*benchmarkCanvasSize*4)
		b.ReportAllocs()
		b.SetBytes(benchmarkCanvasBytes())
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			pixels = RenderViewport(fixture.doc, &fixture.inst.viewport, pixels, documentSurface)
		}

		b.StopTimer()
		assertBenchmarkSurfaceLen(b, len(pixels))
	})

	b.Run("RenderFrameCachedComposite", func(b *testing.B) {
		fixture := newRenderBenchmarkFixture()
		fixture.preparePaintedDocument()
		result := fixture.inst.render()
		if result.BufferLen == 0 {
			b.Fatal("expected initial render to populate the viewport buffer")
		}

		b.ReportAllocs()
		b.SetBytes(benchmarkCanvasBytes())
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			result = fixture.inst.render()
		}

		b.StopTimer()
		assertBenchmarkSurfaceLen(b, int(result.BufferLen))
	})

	b.Run("RenderFrameAfterPaint", func(b *testing.B) {
		fixture := newRenderBenchmarkFixture()
		b.ReportAllocs()
		b.SetBytes(benchmarkCanvasBytes())

		var result RenderResult
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			fixture.resetToEmpty()
			b.StartTimer()

			fixture.paintAllStrokes()
			result = fixture.inst.render()
		}

		b.StopTimer()
		assertBenchmarkSurfaceLen(b, int(result.BufferLen))
	})
}

func newRenderBenchmarkFixture() *renderBenchmarkFixture {
	doc := testDocumentFixture("bench-doc", "Benchmark", benchmarkCanvasSize, benchmarkCanvasSize)
	layer := NewPixelLayer(
		"Paint Layer",
		LayerBounds{X: 0, Y: 0, W: benchmarkCanvasSize, H: benchmarkCanvasSize},
		make([]byte, benchmarkCanvasSize*benchmarkCanvasSize*4),
	)
	doc.LayerRoot.SetChildren([]LayerNode{layer})
	doc.ActiveLayerID = layer.ID()

	inst := &instance{
		manager:          newDocumentManager(),
		viewport:         ViewportState{CenterX: benchmarkCanvasSize * 0.5, CenterY: benchmarkCanvasSize * 0.5, Zoom: 1, CanvasW: benchmarkCanvasSize, CanvasH: benchmarkCanvasSize, DevicePixelRatio: 1},
		history:          newHistoryStack(defaultHistoryMax),
		foregroundColor:  [4]uint8{0, 0, 0, 255},
		backgroundColor:  [4]uint8{255, 255, 255, 255},
		cachedDocSurface: nil,
	}
	inst.manager.Create(doc)

	return &renderBenchmarkFixture{
		inst:          inst,
		doc:           doc,
		layer:         layer,
		pristineLayer: make([]byte, len(layer.Pixels)),
		strokes:       benchmarkStrokeSet(),
	}
}

func (fixture *renderBenchmarkFixture) resetToEmpty() {
	copy(fixture.layer.Pixels, fixture.pristineLayer)
	fixture.doc.ContentVersion++
	fixture.doc.ActiveLayerID = fixture.layer.ID()
	fixture.inst.history = newHistoryStack(defaultHistoryMax)
	fixture.inst.paintStroke = nil
	fixture.inst.cachedDocSurface = nil
	fixture.inst.cachedDocID = ""
	fixture.inst.cachedDocContentVersion = 0
	fixture.inst.pixels = fixture.inst.pixels[:0]
	fixture.inst.frameID = 0
	fixture.inst.viewport = ViewportState{
		CenterX:          benchmarkCanvasSize * 0.5,
		CenterY:          benchmarkCanvasSize * 0.5,
		Zoom:             1,
		CanvasW:          benchmarkCanvasSize,
		CanvasH:          benchmarkCanvasSize,
		DevicePixelRatio: 1,
	}
	fixture.inst.manager.activeID = fixture.doc.ID
	fixture.inst.manager.docs = map[string]*Document{fixture.doc.ID: fixture.doc}
}

func (fixture *renderBenchmarkFixture) preparePaintedDocument() {
	fixture.resetToEmpty()
	fixture.paintAllStrokes()
	fixture.inst.cachedDocSurface = nil
	fixture.inst.cachedDocID = ""
	fixture.inst.cachedDocContentVersion = 0
}

func (fixture *renderBenchmarkFixture) paintAllStrokes() {
	for _, stroke := range fixture.strokes {
		if len(stroke.Points) == 0 {
			continue
		}

		start := stroke.Points[0]
		fixture.inst.handleBeginPaintStroke(BeginPaintStrokePayload{
			X:        start.X,
			Y:        start.Y,
			Pressure: start.Pressure,
			Brush:    stroke.Brush,
		})

		for _, point := range stroke.Points[1:] {
			fixture.inst.handleContinuePaintStroke(ContinuePaintStrokePayload{
				X:        point.X,
				Y:        point.Y,
				Pressure: point.Pressure,
			})
		}

		fixture.inst.handleEndPaintStroke()
	}
}

func (fixture *renderBenchmarkFixture) renderViewportBase(reuse []byte) []byte {
	return aggrender.RenderViewportBase(
		&aggrender.Document{
			Width:      fixture.doc.Width,
			Height:     fixture.doc.Height,
			Background: fixture.doc.Background.Kind,
		},
		&aggrender.Viewport{
			CenterX:  fixture.inst.viewport.CenterX,
			CenterY:  fixture.inst.viewport.CenterY,
			Zoom:     clampZoom(fixture.inst.viewport.Zoom),
			Rotation: fixture.inst.viewport.Rotation,
			CanvasW:  fixture.inst.viewport.CanvasW,
			CanvasH:  fixture.inst.viewport.CanvasH,
		},
		reuse,
	)
}

func (fixture *renderBenchmarkFixture) renderViewportOverlays(reuse []byte) []byte {
	return aggrender.RenderViewportOverlays(
		&aggrender.Document{
			Width:      fixture.doc.Width,
			Height:     fixture.doc.Height,
			Background: fixture.doc.Background.Kind,
		},
		&aggrender.Viewport{
			CenterX:    fixture.inst.viewport.CenterX,
			CenterY:    fixture.inst.viewport.CenterY,
			Zoom:       clampZoom(fixture.inst.viewport.Zoom),
			Rotation:   fixture.inst.viewport.Rotation,
			CanvasW:    fixture.inst.viewport.CanvasW,
			CanvasH:    fixture.inst.viewport.CanvasH,
			ShowGuides: fixture.inst.viewport.ShowGuides,
		},
		reuse,
	)
}

func (fixture *renderBenchmarkFixture) affineTransformState(interp InterpolMode) *FreeTransformState {
	rotation := 17 * math.Pi / 180
	scaleX := 0.84
	scaleY := 1.12
	shearX := 0.18
	cosR := math.Cos(rotation)
	sinR := math.Sin(rotation)
	return fixture.affineStateFromMatrix(
		cosR*scaleX,
		sinR*scaleX,
		-sinR*scaleY+shearX,
		cosR*scaleY,
		18,
		-26,
		interp,
	)
}

func (fixture *renderBenchmarkFixture) axisAlignedScaleTransformState(scaleX, scaleY, offsetX, offsetY float64, interp InterpolMode) *FreeTransformState {
	return fixture.affineStateFromMatrix(scaleX, 0, 0, scaleY, offsetX, offsetY, interp)
}

func (fixture *renderBenchmarkFixture) integerTranslateTransformState(offsetX, offsetY float64, interp InterpolMode) *FreeTransformState {
	return fixture.affineStateFromMatrix(1, 0, 0, 1, offsetX, offsetY, interp)
}

func (fixture *renderBenchmarkFixture) perspectiveTransformState(interp InterpolMode) *FreeTransformState {
	bounds := fixture.layer.Bounds
	x0, y0 := float64(bounds.X), float64(bounds.Y)
	w, h := float64(bounds.W), float64(bounds.H)
	// Trapezoid perspective: pinch the top edge inward.
	corners := &[4][2]float64{
		{x0 + w*0.15, y0 - h*0.05}, // TL
		{x0 + w*0.85, y0 + h*0.08}, // TR
		{x0 + w*1.05, y0 + h*1.02}, // BR
		{x0 - w*0.05, y0 + h*0.98}, // BL
	}
	return &FreeTransformState{
		Active:         true,
		LayerID:        fixture.layer.ID(),
		OriginalPixels: fixture.layer.Pixels,
		OriginalBounds: bounds,
		A:              1, B: 0, C: 0, D: 1,
		TX:             x0,
		TY:             y0,
		PivotX:         x0 + w*0.5,
		PivotY:         y0 + h*0.5,
		Interpolation:  interp,
		DistortCorners: corners,
	}
}

func (fixture *renderBenchmarkFixture) warpTransformState(interp InterpolMode) *FreeTransformState {
	bounds := fixture.layer.Bounds
	grid := initWarpGridFromBounds(bounds)
	// Displace a few interior points to create a non-trivial warp.
	grid[1][1][0] += float64(bounds.W) * 0.06
	grid[1][1][1] += float64(bounds.H) * 0.04
	grid[2][2][0] -= float64(bounds.W) * 0.05
	grid[2][2][1] += float64(bounds.H) * 0.07
	x0, y0 := float64(bounds.X), float64(bounds.Y)
	w, h := float64(bounds.W), float64(bounds.H)
	return &FreeTransformState{
		Active:         true,
		LayerID:        fixture.layer.ID(),
		OriginalPixels: fixture.layer.Pixels,
		OriginalBounds: bounds,
		A:              1, B: 0, C: 0, D: 1,
		TX:            x0,
		TY:            y0,
		PivotX:        x0 + w*0.5,
		PivotY:        y0 + h*0.5,
		Interpolation: interp,
		WarpGrid:      grid,
	}
}

func (fixture *renderBenchmarkFixture) affineStateFromMatrix(a, b, c, d, offsetX, offsetY float64, interp InterpolMode) *FreeTransformState {
	bounds := fixture.layer.Bounds
	localPivotX := float64(bounds.W) * 0.5
	localPivotY := float64(bounds.H) * 0.5
	docPivotX := float64(bounds.X) + localPivotX + offsetX
	docPivotY := float64(bounds.Y) + localPivotY + offsetY
	tx := docPivotX - (a*localPivotX + c*localPivotY)
	ty := docPivotY - (b*localPivotX + d*localPivotY)

	return &FreeTransformState{
		Active:         true,
		LayerID:        fixture.layer.ID(),
		OriginalPixels: fixture.layer.Pixels,
		OriginalBounds: bounds,
		A:              a,
		B:              b,
		C:              c,
		D:              d,
		TX:             tx,
		TY:             ty,
		PivotX:         docPivotX,
		PivotY:         docPivotY,
		Interpolation:  interp,
	}
}

func benchmarkStrokeSet() []benchmarkStroke {
	softBrush := BrushParams{Size: 28, Hardness: 0.72, Flow: 0.9, Color: [4]uint8{232, 64, 48, 255}}
	hardBrush := BrushParams{Size: 18, Hardness: 1, Flow: 0.82, Color: [4]uint8{56, 124, 224, 255}}

	return []benchmarkStroke{
		{Brush: softBrush, Points: benchmarkWaveStroke(72, 128, 440, 56, 18, 0.45)},
		{Brush: softBrush, Points: benchmarkArcStroke(256, 288, 144, 104, 20, 2.9, 6.05)},
		{Brush: hardBrush, Points: benchmarkWaveStroke(80, 376, 432, 28, 16, 1.1)},
	}
}

func benchmarkWaveStroke(startX, centerY, width, amplitude float64, steps int, phase float64) []benchmarkStrokePoint {
	points := make([]benchmarkStrokePoint, 0, steps+1)
	for step := 0; step <= steps; step++ {
		t := float64(step) / float64(steps)
		points = append(points, benchmarkStrokePoint{
			X:        startX + t*width,
			Y:        centerY + math.Sin((t*math.Pi*2)+phase)*amplitude,
			Pressure: 0.55 + 0.35*math.Sin(t*math.Pi),
		})
	}
	return points
}

func benchmarkArcStroke(centerX, centerY, radiusX, radiusY float64, steps int, startAngle, endAngle float64) []benchmarkStrokePoint {
	points := make([]benchmarkStrokePoint, 0, steps+1)
	for step := 0; step <= steps; step++ {
		t := float64(step) / float64(steps)
		angle := startAngle + t*(endAngle-startAngle)
		points = append(points, benchmarkStrokePoint{
			X:        centerX + math.Cos(angle)*radiusX,
			Y:        centerY + math.Sin(angle)*radiusY,
			Pressure: 0.48 + 0.28*math.Cos(t*math.Pi),
		})
	}
	return points
}

func benchmarkCanvasBytes() int64 {
	return int64(benchmarkCanvasSize * benchmarkCanvasSize * 4)
}

func assertBenchmarkSurfaceLen(b *testing.B, got int) {
	b.Helper()
	want := benchmarkCanvasSize * benchmarkCanvasSize * 4
	if got != want {
		b.Fatalf("buffer length = %d, want %d", got, want)
	}
}
