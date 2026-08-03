package engine

import (
	"math"

	agglib "github.com/cwbudde/agg_go"
)

// InterpolMode selects the resampling quality used when committing a free
// transform. Bilinear is used for all live previews regardless of this setting.
type InterpolMode string

const (
	InterpolNearest  InterpolMode = "nearest"
	InterpolBilinear InterpolMode = "bilinear"
	InterpolBicubic  InterpolMode = "bicubic"
)

const transformDispatchEpsilon = 1e-6

// LastTransformRecord stores the parameters of the most recently committed
// transform so that Transform Again (Ctrl+Shift+T) can replay it on any layer.
//
// For free transforms, all offsets are stored relative to the original layer
// bounds, so the same shape of transform can be applied to a different layer.
// For discrete transforms (flip, rotate), only Kind is needed.
type LastTransformRecord struct {
	// Kind is "free", "flipH", "flipV", "rotate90cw", "rotate90ccw", "rotate180".
	Kind string
	// Affine matrix components (only for Kind == "free").
	A, B, C, D float64
	// TXDelta / TYDelta: translation relative to the original layer bounds origin.
	TXDelta, TYDelta float64
	// PivotXDelta / PivotYDelta: pivot offset relative to the original layer origin.
	PivotXDelta, PivotYDelta float64
	Interpolation            InterpolMode
	// DistortCorners, when non-nil, holds per-corner offsets in doc space
	// from each corner's default (un-transformed) position. Order: TL, TR, BR, BL.
	DistortCorners *[4][2]float64
	// WarpGrid, when non-nil, holds per-point offsets from the default bilinear
	// grid positions. Layout: [row][col][x,y], 4×4.
	WarpGrid *[4][4][2]float64
}

// FreeTransformState holds the live state while free transform is active.
//
// The affine matrix maps layer-local pixel coordinates (lx, ly) in [0,W)×[0,H)
// to document space (dx, dy):
//
//	dx = A*lx + C*ly + TX
//	dy = B*lx + D*ly + TY
//
// When the transform is the identity the matrix is
//
//	A=1, B=0, C=0, D=1, TX=OriginalBounds.X, TY=OriginalBounds.Y
//
// so that doc position = layer-local position + layer origin.
type FreeTransformState struct {
	Active          bool
	LayerID         string
	OriginalPixels  []byte
	ScratchPixels   []byte
	ScratchRenderer *agglib.Agg2D
	ScratchSource   *agglib.Image
	OriginalBounds  LayerBounds
	A, B, C, D      float64
	TX, TY          float64
	PivotX, PivotY  float64 // pivot in doc space; initially layer centre
	Interpolation   InterpolMode
	// DistortCorners is set when the user drags a corner in Ctrl+distort mode.
	// When non-nil, AGG's perspective span pipeline warps the source pixels to
	// these four doc-space corners (TL, TR, BR, BL) instead of the affine matrix.
	DistortCorners *[4][2]float64
	// WarpGrid is set when the user enters mesh-warp mode.
	// It holds a 4×4 grid of doc-space control points (rows top-to-bottom,
	// columns left-to-right). Each 3×3 cell is rendered as a separate
	// perspective-corrected patch via AGG's TransformImageQuad.
	WarpGrid *[4][4][2]float64
	// Floating-selection fields (set when the transform was initiated on a
	// selection rather than a whole layer).
	IsFloating           bool   // true when selected pixels were lifted to a temp layer
	SourceLayerID        string // the layer from which pixels were extracted
	OriginalSourcePixels []byte // source layer pixels before the selection was cut
	OriginalSourceBounds LayerBounds
	PreBeginSnapshot     *snapshot // full-document snapshot taken before begin (for undo)
}

// FreeTransformMeta is serialised into UIMeta so the frontend can render
// handle overlays and numeric option-bar fields.
type FreeTransformMeta struct {
	Active        bool    `json:"active"`
	LayerID       string  `json:"layerId,omitempty"`
	OrigX         float64 `json:"origX"`
	OrigY         float64 `json:"origY"`
	OrigW         float64 `json:"origW"`
	OrigH         float64 `json:"origH"`
	A             float64 `json:"a"`
	B             float64 `json:"b"`
	C             float64 `json:"c"`
	D             float64 `json:"d"`
	TX            float64 `json:"tx"`
	TY            float64 `json:"ty"`
	PivotX        float64 `json:"pivotX"`
	PivotY        float64 `json:"pivotY"`
	Interpolation string  `json:"interpolation"`
	// Corners are the four corners of the source bounding box after the current
	// transform in document space. Order: TL, TR, BR, BL.
	Corners [4][2]float64 `json:"corners"`
	// WarpGrid is populated when the transform is in mesh-warp mode.
	// It mirrors FreeTransformState.WarpGrid so the frontend can hit-test
	// and render the grid control points.
	WarpGrid *[4][4][2]float64 `json:"warpGrid,omitempty"`
	// Decomposed parameters for the options bar.
	ScaleX   float64 `json:"scaleX"` // percentage (100 = original size)
	ScaleY   float64 `json:"scaleY"`
	Rotation float64 `json:"rotation"` // degrees
	SkewX    float64 `json:"skewX"`    // degrees
	SkewY    float64 `json:"skewY"`
}

// ---------------------------------------------------------------------------
// Transform helpers
// ---------------------------------------------------------------------------

// initWarpGridFromBounds builds a 4×4 warp control-point grid by uniform
// bilinear interpolation of the layer bounds in document space.
func initWarpGridFromBounds(b LayerBounds) *[4][4][2]float64 {
	x0 := float64(b.X)
	y0 := float64(b.Y)
	w := float64(b.W)
	h := float64(b.H)
	var g [4][4][2]float64
	for row := range 4 {
		for col := range 4 {
			g[row][col] = [2]float64{
				x0 + w*float64(col)/3,
				y0 + h*float64(row)/3,
			}
		}
	}
	return &g
}

// recordLastFreeTransform builds a LastTransformRecord from a committed
// FreeTransformState, expressing all positional values as offsets relative to
// the layer's original bounds so the record can be applied to any layer.
func recordLastFreeTransform(ft *FreeTransformState) *LastTransformRecord {
	origX := float64(ft.OriginalBounds.X)
	origY := float64(ft.OriginalBounds.Y)
	rec := &LastTransformRecord{
		Kind:          "free",
		A:             ft.A,
		B:             ft.B,
		C:             ft.C,
		D:             ft.D,
		TXDelta:       ft.TX - origX,
		TYDelta:       ft.TY - origY,
		PivotXDelta:   ft.PivotX - origX,
		PivotYDelta:   ft.PivotY - origY,
		Interpolation: ft.Interpolation,
	}
	if ft.DistortCorners != nil {
		// Store per-corner offsets from the default (un-transformed) corner positions.
		defaultCorners := defaultBoundsCorners(ft.OriginalBounds)
		offsets := new([4][2]float64)
		for i := range 4 {
			offsets[i][0] = ft.DistortCorners[i][0] - defaultCorners[i][0]
			offsets[i][1] = ft.DistortCorners[i][1] - defaultCorners[i][1]
		}
		rec.DistortCorners = offsets
	} else if ft.WarpGrid != nil {
		// Store per-point offsets from the default bilinear grid positions.
		defaultGrid := initWarpGridFromBounds(ft.OriginalBounds)
		offsets := new([4][4][2]float64)
		for r := range 4 {
			for c := range 4 {
				offsets[r][c][0] = ft.WarpGrid[r][c][0] - defaultGrid[r][c][0]
				offsets[r][c][1] = ft.WarpGrid[r][c][1] - defaultGrid[r][c][1]
			}
		}
		rec.WarpGrid = offsets
	}
	return rec
}

// defaultBoundsCorners returns the four corners of b in TL, TR, BR, BL order.
func defaultBoundsCorners(b LayerBounds) [4][2]float64 {
	x0, y0 := float64(b.X), float64(b.Y)
	x1, y1 := float64(b.X+b.W), float64(b.Y+b.H)
	return [4][2]float64{{x0, y0}, {x1, y0}, {x1, y1}, {x0, y1}}
}

// freeTransformStateFromRecord rebuilds a FreeTransformState from a
// LastTransformRecord, resolving the record's relative offsets against the
// layer's current bounds so the same shape of transform applies to any layer.
func freeTransformStateFromRecord(lt *LastTransformRecord, pl *PixelLayer) *FreeTransformState {
	orig := pl.Bounds
	origX := float64(orig.X)
	origY := float64(orig.Y)
	ft := &FreeTransformState{
		Active:         true,
		LayerID:        pl.ID(),
		OriginalPixels: pl.Pixels,
		OriginalBounds: orig,
		A:              lt.A,
		B:              lt.B,
		C:              lt.C,
		D:              lt.D,
		TX:             origX + lt.TXDelta,
		TY:             origY + lt.TYDelta,
		PivotX:         origX + lt.PivotXDelta,
		PivotY:         origY + lt.PivotYDelta,
		Interpolation:  lt.Interpolation,
	}
	if lt.DistortCorners != nil {
		defaultCorners := defaultBoundsCorners(orig)
		corners := new([4][2]float64)
		for i := range 4 {
			corners[i][0] = defaultCorners[i][0] + lt.DistortCorners[i][0]
			corners[i][1] = defaultCorners[i][1] + lt.DistortCorners[i][1]
		}
		ft.DistortCorners = corners
	} else if lt.WarpGrid != nil {
		defaultGrid := initWarpGridFromBounds(orig)
		grid := new([4][4][2]float64)
		for r := range 4 {
			for c := range 4 {
				grid[r][c][0] = defaultGrid[r][c][0] + lt.WarpGrid[r][c][0]
				grid[r][c][1] = defaultGrid[r][c][1] + lt.WarpGrid[r][c][1]
			}
		}
		ft.WarpGrid = grid
	}
	return ft
}

// transformPoint maps a layer-local point through the affine matrix.
func (s *FreeTransformState) transformPoint(lx, ly float64) (dx, dy float64) {
	return s.A*lx + s.C*ly + s.TX, s.B*lx + s.D*ly + s.TY
}

// transformedCorners returns the four outer corners of the current transform
// in document space (TL, TR, BR, BL).
// In warp mode these are the four corner control points of the mesh.
func (s *FreeTransformState) transformedCorners() [4][2]float64 {
	if s.WarpGrid != nil {
		g := *s.WarpGrid
		return [4][2]float64{g[0][0], g[0][3], g[3][3], g[3][0]}
	}
	if s.DistortCorners != nil {
		return *s.DistortCorners
	}
	w := float64(s.OriginalBounds.W)
	h := float64(s.OriginalBounds.H)
	tl := [2]float64{s.TX, s.TY}
	tr := [2]float64{}
	tr[0], tr[1] = s.transformPoint(w, 0)
	br := [2]float64{}
	br[0], br[1] = s.transformPoint(w, h)
	bl := [2]float64{}
	bl[0], bl[1] = s.transformPoint(0, h)
	return [4][2]float64{tl, tr, br, bl}
}

// transformedAABB returns the axis-aligned bounding box that covers the full
// current transform in document space.  In warp mode it covers all 16 control
// points; otherwise it covers the four outer corners.
func (s *FreeTransformState) transformedAABB() (minX, minY, maxX, maxY float64) {
	minX = math.Inf(1)
	minY = math.Inf(1)
	maxX = math.Inf(-1)
	maxY = math.Inf(-1)
	update := func(x, y float64) {
		if x < minX {
			minX = x
		}
		if x > maxX {
			maxX = x
		}
		if y < minY {
			minY = y
		}
		if y > maxY {
			maxY = y
		}
	}
	if s.WarpGrid != nil {
		for _, row := range *s.WarpGrid {
			for _, pt := range row {
				update(pt[0], pt[1])
			}
		}
		return
	}
	for _, c := range s.transformedCorners() {
		update(c[0], c[1])
	}
	return
}

// det returns the determinant of the 2×2 part of the affine matrix.
func (s *FreeTransformState) det() float64 {
	return s.A*s.D - s.C*s.B
}

// meta builds the UIMeta representation of the current state.
func (s *FreeTransformState) meta() *FreeTransformMeta {
	if s == nil || !s.Active {
		return nil
	}
	corners := s.transformedCorners()

	// Decompose matrix into scale, rotation and skew.
	scaleX := math.Hypot(s.A, s.B)
	scaleY := math.Hypot(s.C, s.D)
	rotation, skewX, skewY := decomposeRotationSkew(s.A, s.B, s.C, s.D)

	origW := float64(s.OriginalBounds.W)
	origH := float64(s.OriginalBounds.H)
	var scaleXPct, scaleYPct float64
	if origW > 0 {
		scaleXPct = scaleX / origW * origW * 100
	} else {
		scaleXPct = 100
	}
	if origH > 0 {
		scaleYPct = scaleY / origH * origH * 100
	} else {
		scaleYPct = 100
	}

	return &FreeTransformMeta{
		Active:        true,
		LayerID:       s.LayerID,
		OrigX:         float64(s.OriginalBounds.X),
		OrigY:         float64(s.OriginalBounds.Y),
		OrigW:         origW,
		OrigH:         origH,
		A:             s.A,
		B:             s.B,
		C:             s.C,
		D:             s.D,
		TX:            s.TX,
		TY:            s.TY,
		PivotX:        s.PivotX,
		PivotY:        s.PivotY,
		Interpolation: string(s.Interpolation),
		Corners:       corners,
		WarpGrid:      s.WarpGrid,
		ScaleX:        scaleXPct,
		ScaleY:        scaleYPct,
		Rotation:      rotation,
		SkewX:         skewX,
		SkewY:         skewY,
	}
}

// normalizeDegrees wraps an angle to the (-180, 180] range.
func normalizeDegrees(deg float64) float64 {
	deg = math.Mod(deg, 360)
	if deg > 180 {
		deg -= 360
	} else if deg <= -180 {
		deg += 360
	}
	return deg
}

// decomposeRotationSkew splits the 2×2 matrix [A C; B D] into rotation, skewX
// and skewY (all degrees), relative to the rotated frame so a pure rotation
// reports zero skew.
//
// A 2×2 matrix has four degrees of freedom (two scales, rotation, one shear),
// so rotation, skewX and skewY cannot all be independent: the shear between
// the two basis columns is attributed to the axis whose basis column deviates
// more from its home direction, and the rotation is read from the other
// column. This makes the canonical gestures decompose cleanly — a pure
// rotation reports skewX = skewY = 0, a pure horizontal (top/bottom edge)
// skew reports only skewX, and a pure vertical (left/right edge) skew reports
// only skewY.
func decomposeRotationSkew(a, b, c, d float64) (rotation, skewX, skewY float64) {
	// thetaX: angle of the x-basis column (A, B) from the +x axis.
	// thetaY: angle of the y-basis column (C, D) from the +y axis.
	thetaX := normalizeDegrees(math.Atan2(b, a) * 180 / math.Pi)
	thetaY := normalizeDegrees(math.Atan2(d, c)*180/math.Pi - 90)
	shear := normalizeDegrees(thetaX - thetaY)
	if math.Abs(thetaX) <= math.Abs(thetaY) {
		return thetaX, shear, 0
	}
	return thetaY, 0, shear
}

func acquireTransformPixels(s *FreeTransformState, size int) []byte {
	if size <= 0 {
		return nil
	}
	if cap(s.ScratchPixels) < size {
		s.ScratchPixels = make([]byte, size)
	} else {
		s.ScratchPixels = s.ScratchPixels[:size]
		clear(s.ScratchPixels)
	}
	return s.ScratchPixels
}

func isNearlyZero(value float64) bool {
	return math.Abs(value) <= transformDispatchEpsilon
}

func isNearlyInteger(value float64) bool {
	return math.Abs(value-math.Round(value)) <= transformDispatchEpsilon
}

func isPureIntegerTranslate(s *FreeTransformState) bool {
	return math.Abs(s.A-1) <= transformDispatchEpsilon &&
		math.Abs(s.D-1) <= transformDispatchEpsilon &&
		isNearlyZero(s.B) &&
		isNearlyZero(s.C) &&
		isNearlyInteger(s.TX) &&
		isNearlyInteger(s.TY)
}

func isAxisAlignedPositiveScale(s *FreeTransformState) bool {
	return s.A > transformDispatchEpsilon &&
		s.D > transformDispatchEpsilon &&
		isNearlyZero(s.B) &&
		isNearlyZero(s.C)
}

type transformRenderTarget struct {
	outX   int
	outY   int
	outW   int
	outH   int
	pixels []byte
}

func computeTransformRenderTarget(s *FreeTransformState) transformRenderTarget {
	origW := s.OriginalBounds.W
	origH := s.OriginalBounds.H
	minX, minY, maxX, maxY := s.transformedAABB()
	outX := int(math.Floor(minX))
	outY := int(math.Floor(minY))
	outW := int(math.Ceil(maxX)) - outX
	outH := int(math.Ceil(maxY)) - outY
	if outW <= 0 || outH <= 0 {
		return transformRenderTarget{
			outX:   s.OriginalBounds.X,
			outY:   s.OriginalBounds.Y,
			outW:   origW,
			outH:   origH,
			pixels: acquireTransformPixels(s, origW*origH*4),
		}
	}
	const maxTransformDim = 32768
	if outW > maxTransformDim || outH > maxTransformDim {
		outW = minInt(outW, maxTransformDim)
		outH = minInt(outH, maxTransformDim)
	}
	return transformRenderTarget{
		outX:   outX,
		outY:   outY,
		outW:   outW,
		outH:   outH,
		pixels: acquireTransformPixels(s, outW*outH*4),
	}
}

func (target transformRenderTarget) bounds() LayerBounds {
	return LayerBounds{X: target.outX, Y: target.outY, W: target.outW, H: target.outH}
}

func (target transformRenderTarget) tileQuad(pts [4][2]float64) [8]float64 {
	return [8]float64{
		pts[0][0] - float64(target.outX), pts[0][1] - float64(target.outY),
		pts[1][0] - float64(target.outX), pts[1][1] - float64(target.outY),
		pts[2][0] - float64(target.outX), pts[2][1] - float64(target.outY),
		pts[3][0] - float64(target.outX), pts[3][1] - float64(target.outY),
	}
}

func (target transformRenderTarget) affineParallelogram(corners [4][2]float64) []float64 {
	return []float64{
		corners[0][0] - float64(target.outX), corners[0][1] - float64(target.outY),
		corners[1][0] - float64(target.outX), corners[1][1] - float64(target.outY),
		corners[2][0] - float64(target.outX), corners[2][1] - float64(target.outY),
	}
}

func affineTransformResamplePolicy(interp InterpolMode) agglib.AffineImageResamplePolicy {
	if interp == InterpolNearest {
		return agglib.AffineImageResampleAgg2D
	}
	return agglib.AffineImageResamplePreferFiltered
}

func acquireTransformAGGResources(s *FreeTransformState, target transformRenderTarget, origW, origH int, interp InterpolMode, resample agglib.ImageResample, affinePolicy agglib.AffineImageResamplePolicy) (*agglib.Agg2D, *agglib.Image) {
	renderer := s.ScratchRenderer
	if renderer == nil {
		renderer = agglib.NewAgg2D()
		s.ScratchRenderer = renderer
	}
	renderer.Attach(target.pixels, target.outW, target.outH, target.outW*4)
	renderer.ImageResample(resample)
	renderer.AffineImageResamplePolicy(affinePolicy)
	switch interp {
	case InterpolNearest:
		renderer.ImageFilter(agglib.NoFilter)
	case InterpolBicubic:
		renderer.ImageFilter(agglib.Bicubic)
	default:
		renderer.ImageFilter(agglib.Bilinear)
	}
	srcImg := s.ScratchSource
	if srcImg == nil {
		srcImg = agglib.NewImage(s.OriginalPixels, origW, origH, origW*4)
		s.ScratchSource = srcImg
	} else {
		srcImg.Attach(s.OriginalPixels, origW, origH, origW*4)
	}
	return renderer, srcImg
}

// ---------------------------------------------------------------------------
// Transform strategy dispatch
// ---------------------------------------------------------------------------

// transformStrategy pairs per-mode AGG configuration (resample policy) with a
// render function so that applyPixelTransform uses a single setup path for all
// transform modes. The render function receives a fully-configured AGG renderer
// and source image — it only contains AGG draw calls, not setup logic.
type transformStrategy struct {
	resample     agglib.ImageResample
	affinePolicy agglib.AffineImageResamplePolicy
	render       func(renderer *agglib.Agg2D, src *agglib.Image, s *FreeTransformState, target transformRenderTarget, origW, origH int)
}

func selectTransformStrategy(s *FreeTransformState, interp InterpolMode) transformStrategy {
	switch {
	case s.WarpGrid != nil:
		return transformStrategy{agglib.NoResample, agglib.AffineImageResampleAgg2D, renderWarpQuads}
	case s.DistortCorners != nil:
		return transformStrategy{agglib.NoResample, agglib.AffineImageResampleAgg2D, renderDistortQuad}
	default:
		return transformStrategy{agglib.NoResample, affineTransformResamplePolicy(interp), renderAffineImage}
	}
}

func renderWarpQuads(renderer *agglib.Agg2D, src *agglib.Image, s *FreeTransformState, target transformRenderTarget, origW, origH int) {
	g := *s.WarpGrid
	const cells = 3
	for row := range cells {
		for col := range cells {
			srcX1 := col * origW / cells
			srcY1 := row * origH / cells
			srcX2 := (col + 1) * origW / cells
			srcY2 := (row + 1) * origH / cells
			quad := target.tileQuad([4][2]float64{
				g[row][col],
				g[row][col+1],
				g[row+1][col+1],
				g[row+1][col],
			})
			_ = renderer.TransformImageQuad(src, srcX1, srcY1, srcX2, srcY2, quad)
		}
	}
}

func renderDistortQuad(renderer *agglib.Agg2D, src *agglib.Image, s *FreeTransformState, target transformRenderTarget, origW, origH int) {
	quad := target.tileQuad(*s.DistortCorners)
	_ = renderer.TransformImageQuad(src, 0, 0, origW, origH, quad)
}

func renderAffineImage(renderer *agglib.Agg2D, src *agglib.Image, s *FreeTransformState, target transformRenderTarget, origW, origH int) {
	corners := s.transformedCorners()
	if isAxisAlignedPositiveScale(s) {
		_ = renderer.TransformImageSimple(
			src,
			corners[0][0]-float64(target.outX),
			corners[0][1]-float64(target.outY),
			corners[2][0]-float64(target.outX),
			corners[2][1]-float64(target.outY),
		)
		return
	}
	_ = renderer.TransformImageParallelogram(src, 0, 0, origW, origH, target.affineParallelogram(corners))
}

// applyPixelTransform creates new pixel data by applying the current transform
// (affine, perspective distort, or mesh warp) stored in s. All modes render
// through AGG image transforms via a shared setup path — the per-mode
// transformStrategy selects only the resample policy and the AGG draw calls.
func applyPixelTransform(s *FreeTransformState, interp InterpolMode) (newPixels []byte, newBounds LayerBounds) {
	origW := s.OriginalBounds.W
	origH := s.OriginalBounds.H
	if origW <= 0 || origH <= 0 || len(s.OriginalPixels) < origW*origH*4 {
		return s.OriginalPixels, s.OriginalBounds
	}
	target := computeTransformRenderTarget(s)

	// Affine fast-paths that bypass AGG entirely.
	if s.WarpGrid == nil && s.DistortCorners == nil {
		if math.Abs(s.det()) < 1e-10 {
			return target.pixels, target.bounds()
		}
		if isPureIntegerTranslate(s) && target.outW == origW && target.outH == origH {
			copy(target.pixels, s.OriginalPixels)
			return target.pixels, target.bounds()
		}
	}

	mode := selectTransformStrategy(s, interp)
	renderer, srcImg := acquireTransformAGGResources(s, target, origW, origH, interp, mode.resample, mode.affinePolicy)
	mode.render(renderer, srcImg, s, target, origW, origH)

	return target.pixels, target.bounds()
}

// ---------------------------------------------------------------------------
// Discrete (non-interactive) pixel transforms
// ---------------------------------------------------------------------------

// flipPixelsH flips pixels horizontally within its own buffer.
func flipPixelsH(pixels []byte, w, h int) []byte {
	out := make([]byte, len(pixels))
	for y := range h {
		for x := range w {
			src := (y*w + x) * 4
			dst := (y*w + (w - 1 - x)) * 4
			out[dst] = pixels[src]
			out[dst+1] = pixels[src+1]
			out[dst+2] = pixels[src+2]
			out[dst+3] = pixels[src+3]
		}
	}
	return out
}

// flipPixelsV flips pixels vertically.
func flipPixelsV(pixels []byte, w, h int) []byte {
	out := make([]byte, len(pixels))
	for y := range h {
		for x := range w {
			src := (y*w + x) * 4
			dst := ((h-1-y)*w + x) * 4
			out[dst] = pixels[src]
			out[dst+1] = pixels[src+1]
			out[dst+2] = pixels[src+2]
			out[dst+3] = pixels[src+3]
		}
	}
	return out
}

// rotatePixels90CW rotates pixels 90° clockwise. Returns new pixels and swapped
// width/height (the caller must update bounds accordingly).
func rotatePixels90CW(pixels []byte, w, h int) ([]byte, int, int) {
	out := make([]byte, len(pixels))
	for y := range h {
		for x := range w {
			src := (y*w + x) * 4
			// After 90° CW: new(x', y') = (H-1-y, x) in the new w×h grid.
			dst := (x*h + (h - 1 - y)) * 4
			out[dst] = pixels[src]
			out[dst+1] = pixels[src+1]
			out[dst+2] = pixels[src+2]
			out[dst+3] = pixels[src+3]
		}
	}
	return out, h, w // new dims are swapped
}

// rotatePixels90CCW rotates pixels 90° counter-clockwise.
func rotatePixels90CCW(pixels []byte, w, h int) ([]byte, int, int) {
	out := make([]byte, len(pixels))
	for y := range h {
		for x := range w {
			src := (y*w + x) * 4
			// After 90° CCW: new(x', y') = (y, W-1-x) in the new h×w grid.
			dst := ((w - 1 - x) * h) + y
			dst *= 4
			out[dst] = pixels[src]
			out[dst+1] = pixels[src+1]
			out[dst+2] = pixels[src+2]
			out[dst+3] = pixels[src+3]
		}
	}
	return out, h, w
}

// rotatePixels180 rotates pixels 180°.
func rotatePixels180(pixels []byte, w, h int) []byte {
	out := make([]byte, len(pixels))
	total := w * h
	for i := range total {
		src := i * 4
		dst := (total - 1 - i) * 4
		out[dst] = pixels[src]
		out[dst+1] = pixels[src+1]
		out[dst+2] = pixels[src+2]
		out[dst+3] = pixels[src+3]
	}
	return out
}

// applyDiscreteTransformToLayer applies a non-interactive (immediate) pixel
// transform to a PixelLayer and re-centres the bounds. The centre of the layer
// in document space is preserved. The layer's raster and vector masks are
// remapped through the same doc-space mapping as the pixels.
func applyDiscreteTransformToLayer(layer *PixelLayer, kind string) {
	w, h := layer.Bounds.W, layer.Bounds.H
	if w <= 0 || h <= 0 || len(layer.Pixels) < w*h*4 {
		return
	}
	oldBounds := layer.Bounds
	newW, newH := w, h
	switch kind {
	case "flipH":
		layer.Pixels = flipPixelsH(layer.Pixels, w, h)
	case "flipV":
		layer.Pixels = flipPixelsV(layer.Pixels, w, h)
	case "rotate90cw":
		layer.Pixels, newW, newH = rotatePixels90CW(layer.Pixels, w, h)
	case "rotate90ccw":
		layer.Pixels, newW, newH = rotatePixels90CCW(layer.Pixels, w, h)
	case "rotate180":
		layer.Pixels = rotatePixels180(layer.Pixels, w, h)
	}
	// Keep layer centre in the same document position.
	if newW != w || newH != h {
		cx := layer.Bounds.X + w/2
		cy := layer.Bounds.Y + h/2
		layer.Bounds.X = cx - newW/2
		layer.Bounds.Y = cy - newH/2
		layer.Bounds.W = newW
		layer.Bounds.H = newH
	}
	remapLayerMaskDiscrete(layer.Mask(), kind, oldBounds, layer.Bounds)
	remapVectorMaskDiscrete(layer.VectorMask(), kind, oldBounds, layer.Bounds)
}

// ---------------------------------------------------------------------------
// Mask transformation
//
// Raster layer masks are stored in document space (Width×Height single-channel
// buffers, see layerMaskAlphaAt); vector masks are Paths with absolute
// document-space coordinates. When a layer's pixels are transformed, the mask
// coverage under the layer must follow the exact same doc-space mapping or the
// mask is left behind (S.5). Mask coverage outside the transformed region is
// retained untouched — it has no effect on the transformed layer content.
// ---------------------------------------------------------------------------

// maskHasData reports whether the mask carries a usable doc-space buffer.
func maskHasData(mask *LayerMask) bool {
	return mask != nil && mask.Width > 0 && mask.Height > 0 &&
		len(mask.Data) >= mask.Width*mask.Height
}

// maskRegionToRGBA extracts the doc-space mask coverage under bounds into a
// bounds-local RGBA buffer (the mask value replicated into all four channels)
// so it can be resampled through the same AGG pipeline as the layer pixels.
// Doc pixels outside the mask extents read as 0.
func maskRegionToRGBA(mask *LayerMask, bounds LayerBounds) []byte {
	buf := make([]byte, bounds.W*bounds.H*4)
	for y := range bounds.H {
		docY := bounds.Y + y
		if docY < 0 || docY >= mask.Height {
			continue
		}
		for x := range bounds.W {
			docX := bounds.X + x
			if docX < 0 || docX >= mask.Width {
				continue
			}
			v := mask.Data[docY*mask.Width+docX]
			i := (y*bounds.W + x) * 4
			buf[i], buf[i+1], buf[i+2], buf[i+3] = v, v, v, v
		}
	}
	return buf
}

// writeMaskRegionFromRGBA writes the alpha channel of a bounds-local RGBA
// buffer back into the doc-space mask, clipped to the mask extents.
func writeMaskRegionFromRGBA(mask *LayerMask, bounds LayerBounds, rgba []byte) {
	if len(rgba) < bounds.W*bounds.H*4 {
		return
	}
	for y := range bounds.H {
		docY := bounds.Y + y
		if docY < 0 || docY >= mask.Height {
			continue
		}
		for x := range bounds.W {
			docX := bounds.X + x
			if docX < 0 || docX >= mask.Width {
				continue
			}
			mask.Data[docY*mask.Width+docX] = rgba[(y*bounds.W+x)*4+3]
		}
	}
}

// transformLayerMaskForFree remaps the layer's doc-space raster mask through
// the same mapping applyPixelTransform used for the layer pixels: the mask
// region under the original bounds is resampled through an identical
// FreeTransformState (affine, distort, or warp) and written back under the
// transformed bounds. A fresh state is used so the mask pass cannot clobber
// the pixel pass's scratch buffers (which alias the committed pixels).
func transformLayerMaskForFree(mask *LayerMask, src *FreeTransformState, interp InterpolMode) {
	if !maskHasData(mask) || src == nil || src.OriginalBounds.W <= 0 || src.OriginalBounds.H <= 0 {
		return
	}
	maskState := &FreeTransformState{
		Active:         true,
		OriginalPixels: maskRegionToRGBA(mask, src.OriginalBounds),
		OriginalBounds: src.OriginalBounds,
		A:              src.A,
		B:              src.B,
		C:              src.C,
		D:              src.D,
		TX:             src.TX,
		TY:             src.TY,
		PivotX:         src.PivotX,
		PivotY:         src.PivotY,
		Interpolation:  interp,
		DistortCorners: src.DistortCorners,
		WarpGrid:       src.WarpGrid,
	}
	outPixels, outBounds := applyPixelTransform(maskState, interp)
	writeMaskRegionFromRGBA(mask, outBounds, outPixels)
}

// transformVectorMaskForFree maps every vector-mask point (anchor and bezier
// handles) through the same geometry as the pixel transform.
func transformVectorMaskForFree(path *Path, src *FreeTransformState) {
	if path == nil || src == nil {
		return
	}
	mapPathPointsInPlace(path, freeTransformPointMapper(src))
}

// mapPathPointsInPlace applies mapPt to every anchor and handle of the path.
func mapPathPointsInPlace(path *Path, mapPt func(x, y float64) (float64, float64)) {
	if path == nil || mapPt == nil {
		return
	}
	for si := range path.Subpaths {
		pts := path.Subpaths[si].Points
		for pi := range pts {
			pts[pi].X, pts[pi].Y = mapPt(pts[pi].X, pts[pi].Y)
			pts[pi].InX, pts[pi].InY = mapPt(pts[pi].InX, pts[pi].InY)
			pts[pi].OutX, pts[pi].OutY = mapPt(pts[pi].OutX, pts[pi].OutY)
		}
	}
}

// bilerpQuad bilinearly interpolates a point inside the quad (tl, tr, br, bl)
// at normalized coordinates (u, v).
func bilerpQuad(tl, tr, br, bl [2]float64, u, v float64) (float64, float64) {
	topX := tl[0] + (tr[0]-tl[0])*u
	topY := tl[1] + (tr[1]-tl[1])*u
	botX := bl[0] + (br[0]-bl[0])*u
	botY := bl[1] + (br[1]-bl[1])*u
	return topX + (botX-topX)*v, topY + (botY-topY)*v
}

// freeTransformPointMapper returns a doc-space → doc-space point mapping that
// matches the pixel mapping of the state: the affine matrix in the default
// mode, or — for distort/warp — the bilinear interpolation of the transformed
// corner/grid control points (an approximation of the perspective raster
// mapping that is exact at the control points).
func freeTransformPointMapper(s *FreeTransformState) func(x, y float64) (float64, float64) {
	ox := float64(s.OriginalBounds.X)
	oy := float64(s.OriginalBounds.Y)
	w := float64(s.OriginalBounds.W)
	h := float64(s.OriginalBounds.H)
	if w <= 0 || h <= 0 {
		return func(x, y float64) (float64, float64) { return x, y }
	}
	switch {
	case s.WarpGrid != nil:
		g := *s.WarpGrid
		return func(x, y float64) (float64, float64) {
			u := clampFloat((x-ox)/w*3, 0, 3)
			v := clampFloat((y-oy)/h*3, 0, 3)
			col := clampInt(int(u), 0, 2)
			row := clampInt(int(v), 0, 2)
			return bilerpQuad(g[row][col], g[row][col+1], g[row+1][col+1], g[row+1][col],
				u-float64(col), v-float64(row))
		}
	case s.DistortCorners != nil:
		c := *s.DistortCorners
		return func(x, y float64) (float64, float64) {
			return bilerpQuad(c[0], c[1], c[2], c[3], (x-ox)/w, (y-oy)/h)
		}
	default:
		return func(x, y float64) (float64, float64) {
			lx := x - ox
			ly := y - oy
			return s.A*lx + s.C*ly + s.TX, s.B*lx + s.D*ly + s.TY
		}
	}
}

// transformMaskRegionDiscrete applies a discrete transform to a single-channel
// w×h buffer, returning the transformed buffer and its new dimensions. The
// index mappings mirror the RGBA pixel variants exactly.
func transformMaskRegionDiscrete(data []byte, w, h int, kind string) ([]byte, int, int) {
	out := make([]byte, len(data))
	switch kind {
	case "flipH":
		for y := range h {
			for x := range w {
				out[y*w+(w-1-x)] = data[y*w+x]
			}
		}
		return out, w, h
	case "flipV":
		for y := range h {
			for x := range w {
				out[(h-1-y)*w+x] = data[y*w+x]
			}
		}
		return out, w, h
	case "rotate90cw":
		for y := range h {
			for x := range w {
				out[x*h+(h-1-y)] = data[y*w+x]
			}
		}
		return out, h, w
	case "rotate90ccw":
		for y := range h {
			for x := range w {
				out[(w-1-x)*h+y] = data[y*w+x]
			}
		}
		return out, h, w
	case "rotate180":
		total := w * h
		for i := range total {
			out[total-1-i] = data[i]
		}
		return out, w, h
	}
	return data, w, h
}

// remapLayerMaskDiscrete rewrites the doc-space raster mask so that its
// coverage under oldBounds undergoes the same flip/rotation as the layer
// pixels and lands under newBounds. Mask values outside the new layer region
// are retained.
func remapLayerMaskDiscrete(mask *LayerMask, kind string, oldBounds, newBounds LayerBounds) {
	if !maskHasData(mask) || oldBounds.W <= 0 || oldBounds.H <= 0 {
		return
	}
	region := make([]byte, oldBounds.W*oldBounds.H)
	for y := range oldBounds.H {
		docY := oldBounds.Y + y
		if docY < 0 || docY >= mask.Height {
			continue
		}
		for x := range oldBounds.W {
			docX := oldBounds.X + x
			if docX < 0 || docX >= mask.Width {
				continue
			}
			region[y*oldBounds.W+x] = mask.Data[docY*mask.Width+docX]
		}
	}
	region, newW, newH := transformMaskRegionDiscrete(region, oldBounds.W, oldBounds.H, kind)
	if newW != newBounds.W || newH != newBounds.H {
		return
	}
	for y := range newH {
		docY := newBounds.Y + y
		if docY < 0 || docY >= mask.Height {
			continue
		}
		for x := range newW {
			docX := newBounds.X + x
			if docX < 0 || docX >= mask.Width {
				continue
			}
			mask.Data[docY*mask.Width+docX] = region[y*newW+x]
		}
	}
}

// remapVectorMaskDiscrete maps the vector-mask geometry through the doc-space
// mapping of a discrete transform: flips mirror about the layer region's
// centre lines, rotations pivot about its centre (with the same integer
// re-centring the pixel path applies via newBounds).
func remapVectorMaskDiscrete(path *Path, kind string, oldBounds, newBounds LayerBounds) {
	if path == nil {
		return
	}
	ox := float64(oldBounds.X)
	oy := float64(oldBounds.Y)
	w := float64(oldBounds.W)
	h := float64(oldBounds.H)
	nx := float64(newBounds.X)
	ny := float64(newBounds.Y)
	var mapPt func(x, y float64) (float64, float64)
	switch kind {
	case "flipH":
		mapPt = func(x, y float64) (float64, float64) { return 2*ox + w - x, y }
	case "flipV":
		mapPt = func(x, y float64) (float64, float64) { return x, 2*oy + h - y }
	case "rotate180":
		mapPt = func(x, y float64) (float64, float64) { return 2*ox + w - x, 2*oy + h - y }
	case "rotate90cw":
		mapPt = func(x, y float64) (float64, float64) { return nx + (h - (y - oy)), ny + (x - ox) }
	case "rotate90ccw":
		mapPt = func(x, y float64) (float64, float64) { return nx + (y - oy), ny + (w - (x - ox)) }
	default:
		return
	}
	mapPathPointsInPlace(path, mapPt)
}

// ---------------------------------------------------------------------------
// Transform handles overlay (rendered on top of the canvas)
// ---------------------------------------------------------------------------

// transformHandleSize is the half-extent (in canvas pixels) of each handle square.
const transformHandleSize = 5

// overlayColor is a simple RGBA colour used for the transform-handles overlay.
type overlayColor struct{ R, G, B, A uint8 }

var (
	transformBoxColor     = overlayColor{255, 255, 255, 220}
	transformHandleColor  = overlayColor{255, 255, 255, 255}
	transformHandleBorder = overlayColor{0, 0, 0, 200}
	transformPivotColor   = overlayColor{255, 255, 255, 220}
)

// RenderTransformHandlesOverlay draws the free-transform bounding box and
// handles onto the canvas buffer.
func RenderTransformHandlesOverlay(state *FreeTransformState, vp *ViewportState, reuse []byte) []byte {
	if state == nil || !state.Active || len(reuse) == 0 {
		return reuse
	}

	canvasW := maxInt(vp.CanvasW, 1)
	canvasH := maxInt(vp.CanvasH, 1)
	zoom := clampZoom(vp.Zoom)
	radians := vp.Rotation * (math.Pi / 180)
	cosTheta := math.Cos(radians)
	sinTheta := math.Sin(radians)
	halfCanvasW := float64(canvasW) * 0.5
	halfCanvasH := float64(canvasH) * 0.5

	docToCanvas := func(docX, docY float64) (cx, cy int) {
		dx := docX - vp.CenterX
		dy := docY - vp.CenterY
		sx := dx*cosTheta*zoom - dy*sinTheta*zoom + halfCanvasW
		sy := dx*sinTheta*zoom + dy*cosTheta*zoom + halfCanvasH
		return int(math.Round(sx)), int(math.Round(sy))
	}

	setPixelBlend := func(cx, cy int, col overlayColor) {
		if cx < 0 || cx >= canvasW || cy < 0 || cy >= canvasH {
			return
		}
		i := (cy*canvasW + cx) * 4
		a := float64(col.A) / 255
		reuse[i] = byte(float64(reuse[i])*(1-a) + float64(col.R)*a)
		reuse[i+1] = byte(float64(reuse[i+1])*(1-a) + float64(col.G)*a)
		reuse[i+2] = byte(float64(reuse[i+2])*(1-a) + float64(col.B)*a)
		reuse[i+3] = 255
	}

	// Draw a line between two canvas points.
	drawLine := func(ax, ay, bx, by int, col overlayColor) {
		dx := bx - ax
		dy := by - ay
		steps := maxInt(absInt(dx), absInt(dy))
		if steps == 0 {
			setPixelBlend(ax, ay, col)
			return
		}
		for s := range steps + 1 {
			t := float64(s) / float64(steps)
			cx := ax + int(math.Round(float64(dx)*t))
			cy := ay + int(math.Round(float64(dy)*t))
			setPixelBlend(cx, cy, col)
		}
	}

	// Draw a filled square handle.
	drawHandle := func(cx, cy int) {
		for dy := -transformHandleSize; dy <= transformHandleSize; dy++ {
			for dx := -transformHandleSize; dx <= transformHandleSize; dx++ {
				if dx == -transformHandleSize || dx == transformHandleSize ||
					dy == -transformHandleSize || dy == transformHandleSize {
					setPixelBlend(cx+dx, cy+dy, transformHandleBorder)
				} else {
					setPixelBlend(cx+dx, cy+dy, transformHandleColor)
				}
			}
		}
	}

	// ---------------------------------------------------------------------------
	// Warp mesh overlay
	// ---------------------------------------------------------------------------
	if state.WarpGrid != nil {
		g := *state.WarpGrid
		// Draw all horizontal and vertical grid segments.
		for r := range 4 {
			for c := range 4 {
				cx, cy := docToCanvas(g[r][c][0], g[r][c][1])
				// Horizontal: connect to right neighbour.
				if c < 3 {
					rx, ry := docToCanvas(g[r][c+1][0], g[r][c+1][1])
					drawLine(cx, cy, rx, ry, transformBoxColor)
				}
				// Vertical: connect to bottom neighbour.
				if r < 3 {
					bx, by := docToCanvas(g[r+1][c][0], g[r+1][c][1])
					drawLine(cx, cy, bx, by, transformBoxColor)
				}
			}
		}
		// Draw handles at all 16 control points.
		for r := range 4 {
			for c := range 4 {
				hcx, hcy := docToCanvas(g[r][c][0], g[r][c][1])
				drawHandle(hcx, hcy)
			}
		}
		return reuse
	}

	// ---------------------------------------------------------------------------
	// Standard free-transform overlay
	// ---------------------------------------------------------------------------

	// Bounding box corners in canvas space.
	corners := state.transformedCorners()
	var sx, sy [4]int
	for i, c := range corners {
		sx[i], sy[i] = docToCanvas(c[0], c[1])
	}

	// Draw bounding box lines.
	drawLine(sx[0], sy[0], sx[1], sy[1], transformBoxColor)
	drawLine(sx[1], sy[1], sx[2], sy[2], transformBoxColor)
	drawLine(sx[2], sy[2], sx[3], sy[3], transformBoxColor)
	drawLine(sx[3], sy[3], sx[0], sy[0], transformBoxColor)

	// 8 handle positions: corners + edge midpoints.
	handleDocs := [8][2]float64{
		corners[0],
		{(corners[0][0] + corners[1][0]) * 0.5, (corners[0][1] + corners[1][1]) * 0.5},
		corners[1],
		{(corners[1][0] + corners[2][0]) * 0.5, (corners[1][1] + corners[2][1]) * 0.5},
		corners[2],
		{(corners[2][0] + corners[3][0]) * 0.5, (corners[2][1] + corners[3][1]) * 0.5},
		corners[3],
		{(corners[3][0] + corners[0][0]) * 0.5, (corners[3][1] + corners[0][1]) * 0.5},
	}
	for _, hd := range handleDocs {
		hcx, hcy := docToCanvas(hd[0], hd[1])
		drawHandle(hcx, hcy)
	}

	// Rotation handle: above the top-centre edge midpoint.
	topMidDoc := handleDocs[1]
	topEdgeDX := corners[1][0] - corners[0][0]
	topEdgeDY := corners[1][1] - corners[0][1]
	topEdgeLen := math.Hypot(topEdgeDX, topEdgeDY)
	const rotHandleOffset = 24.0 / 1.0
	var rotDocOffX, rotDocOffY float64
	if topEdgeLen > 1e-6 && zoom > 1e-6 {
		perpX := -topEdgeDY / topEdgeLen
		perpY := topEdgeDX / topEdgeLen
		docOff := rotHandleOffset / zoom
		rotDocOffX = perpX * docOff
		rotDocOffY = perpY * docOff
	}
	rotHandleDoc := [2]float64{topMidDoc[0] + rotDocOffX, topMidDoc[1] + rotDocOffY}
	rcx, rcy := docToCanvas(rotHandleDoc[0], rotHandleDoc[1])
	const rotR = 5
	for dy := -rotR; dy <= rotR; dy++ {
		for dx := -rotR; dx <= rotR; dx++ {
			dist := math.Hypot(float64(dx), float64(dy))
			if dist <= float64(rotR) && dist >= float64(rotR)-1.5 {
				setPixelBlend(rcx+dx, rcy+dy, transformHandleBorder)
			} else if dist < float64(rotR)-1.5 {
				setPixelBlend(rcx+dx, rcy+dy, transformHandleColor)
			}
		}
	}
	tmcx, tmcy := docToCanvas(topMidDoc[0], topMidDoc[1])
	drawLine(tmcx, tmcy, rcx, rcy, transformBoxColor)

	// Pivot point crosshair.
	pcx, pcy := docToCanvas(state.PivotX, state.PivotY)
	const pivR = 6
	drawLine(pcx-pivR, pcy, pcx+pivR, pcy, transformPivotColor)
	drawLine(pcx, pcy-pivR, pcx, pcy+pivR, transformPivotColor)
	for dy := -3; dy <= 3; dy++ {
		for dx := -3; dx <= 3; dx++ {
			dist := math.Hypot(float64(dx), float64(dy))
			if dist <= 3 {
				setPixelBlend(pcx+dx, pcy+dy, transformPivotColor)
			}
		}
	}

	return reuse
}

// absInt returns the absolute value of n.
func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// ---------------------------------------------------------------------------
// Floating-selection helpers
// ---------------------------------------------------------------------------

// extractSelectionContent lifts selected pixels from pl into a new buffer.
// Only pixels inside both the layer bounds and the selection mask are included.
// Returns false when the selection and layer do not overlap.
func extractSelectionContent(pl *PixelLayer, sel *Selection) (pixels []byte, bounds LayerBounds, ok bool) {
	bnd := pl.Bounds
	selW, selH := sel.Width, sel.Height

	minX, minY := bnd.X+bnd.W, bnd.Y+bnd.H
	maxX, maxY := bnd.X-1, bnd.Y-1
	for ly := range bnd.H {
		for lx := range bnd.W {
			docX := lx + bnd.X
			docY := ly + bnd.Y
			if docX < 0 || docX >= selW || docY < 0 || docY >= selH {
				continue
			}
			if sel.Mask[docY*selW+docX] == 0 {
				continue
			}
			if docX < minX {
				minX = docX
			}
			if docX > maxX {
				maxX = docX
			}
			if docY < minY {
				minY = docY
			}
			if docY > maxY {
				maxY = docY
			}
		}
	}
	if maxX < minX || maxY < minY {
		return nil, LayerBounds{}, false
	}

	floatW := maxX - minX + 1
	floatH := maxY - minY + 1
	pixels = make([]byte, floatW*floatH*4)

	for ly := range bnd.H {
		for lx := range bnd.W {
			docX := lx + bnd.X
			docY := ly + bnd.Y
			if docX < minX || docX > maxX || docY < minY || docY > maxY {
				continue
			}
			if docX < 0 || docX >= selW || docY < 0 || docY >= selH {
				continue
			}
			selA := sel.Mask[docY*selW+docX]
			if selA == 0 {
				continue
			}
			srcIdx := (ly*bnd.W + lx) * 4
			if srcIdx+3 >= len(pl.Pixels) {
				continue
			}
			fx := docX - minX
			fy := docY - minY
			dstIdx := (fy*floatW + fx) * 4
			outA := byte(uint16(pl.Pixels[srcIdx+3]) * uint16(selA) / 255)
			pixels[dstIdx] = pl.Pixels[srcIdx]
			pixels[dstIdx+1] = pl.Pixels[srcIdx+1]
			pixels[dstIdx+2] = pl.Pixels[srcIdx+2]
			pixels[dstIdx+3] = outA
		}
	}

	bounds = LayerBounds{X: minX, Y: minY, W: floatW, H: floatH}
	return pixels, bounds, true
}

// clearSelectionContent removes selected pixels from pl by reducing their
// alpha by the selection mask value (multiply by 1 − selA/255).
func clearSelectionContent(pl *PixelLayer, sel *Selection) {
	bnd := pl.Bounds
	selW, selH := sel.Width, sel.Height
	for ly := range bnd.H {
		for lx := range bnd.W {
			docX := lx + bnd.X
			docY := ly + bnd.Y
			if docX < 0 || docX >= selW || docY < 0 || docY >= selH {
				continue
			}
			selA := sel.Mask[docY*selW+docX]
			if selA == 0 {
				continue
			}
			i := (ly*bnd.W + lx) * 4
			if i+3 >= len(pl.Pixels) {
				continue
			}
			newA := byte(uint16(pl.Pixels[i+3]) * uint16(255-selA) / 255)
			pl.Pixels[i+3] = newA
			if newA == 0 {
				pl.Pixels[i] = 0
				pl.Pixels[i+1] = 0
				pl.Pixels[i+2] = 0
			}
		}
	}
}

// mergePixelLayerOnto composites srcPixels (at srcBounds) over dst using
// source-over blending. dst's pixel buffer and bounds are expanded if necessary
// to cover srcBounds.
func mergePixelLayerOnto(dst *PixelLayer, srcPixels []byte, srcBounds LayerBounds) {
	if len(srcPixels) == 0 || srcBounds.W <= 0 || srcBounds.H <= 0 {
		return
	}
	dstB := dst.Bounds
	if len(dst.Pixels) < dstB.W*dstB.H*4 || dstB.W <= 0 || dstB.H <= 0 {
		dst.Pixels = append([]byte(nil), srcPixels...)
		dst.Bounds = srcBounds
		return
	}

	unionX := minInt(dstB.X, srcBounds.X)
	unionY := minInt(dstB.Y, srcBounds.Y)
	unionW := maxInt(dstB.X+dstB.W, srcBounds.X+srcBounds.W) - unionX
	unionH := maxInt(dstB.Y+dstB.H, srcBounds.Y+srcBounds.H) - unionY

	out := make([]byte, unionW*unionH*4)

	// Copy existing dst pixels into the union canvas.
	for ly := range dstB.H {
		for lx := range dstB.W {
			si := (ly*dstB.W + lx) * 4
			ox := lx + dstB.X - unionX
			oy := ly + dstB.Y - unionY
			di := (oy*unionW + ox) * 4
			out[di] = dst.Pixels[si]
			out[di+1] = dst.Pixels[si+1]
			out[di+2] = dst.Pixels[si+2]
			out[di+3] = dst.Pixels[si+3]
		}
	}

	// Composite the transformed layer through agg_go's straight-alpha
	// source-over path. The union surface is local, while Dissolve (if this
	// helper ever grows mode support) would still use document coordinates.
	_ = compositeImageStraight(
		out, unionW, unionH,
		srcPixels, srcBounds.W, srcBounds.H,
		agglib.Rect{X2: srcBounds.W, Y2: srcBounds.H},
		agglib.PointI{X: srcBounds.X - unionX, Y: srcBounds.Y - unionY},
		BlendModeNormal, 1,
		nil, agglib.PointI{}, nil, offsetDissolveSeed(unionX, unionY),
	)

	dst.Pixels = out
	dst.Bounds = LayerBounds{X: unionX, Y: unionY, W: unionW, H: unionH}
}
