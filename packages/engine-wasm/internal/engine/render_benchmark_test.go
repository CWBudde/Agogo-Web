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
