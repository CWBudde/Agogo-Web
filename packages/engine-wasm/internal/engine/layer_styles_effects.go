package engine

import (
	"math"

	agglib "github.com/cwbudde/agg_go"

	"github.com/cwbudde/agogo-web/packages/engine-wasm/internal/model"
)

// styleRenderContext carries document-scoped lookups into the otherwise
// doc-less layer-style surface builders. The zero value is valid: no pattern
// resolves, so pattern branches fall back to their legacy output.
type styleRenderContext struct {
	resolvePattern func(id string) *model.PatternResource
}

func (ctx styleRenderContext) pattern(id string) *model.PatternResource {
	if id == "" || ctx.resolvePattern == nil {
		return nil
	}
	return ctx.resolvePattern(id)
}

// documentStyleContext builds the style render context for a document. A nil
// document still resolves builtin patterns.
func documentStyleContext(doc *Document) styleRenderContext {
	return styleRenderContext{resolvePattern: func(id string) *model.PatternResource {
		return resolvePattern(doc, id)
	}}
}

var supportedLayerStyleKinds = []LayerStyleKind{
	LayerStyleKindColorOverlay,
	LayerStyleKindGradientOverlay,
	LayerStyleKindPatternOverlay,
	LayerStyleKindStroke,
	LayerStyleKindInnerShadow,
	LayerStyleKindInnerGlow,
	LayerStyleKindBevelEmboss,
	LayerStyleKindSatin,
	LayerStyleKindDropShadow,
	LayerStyleKindOuterGlow,
}

func orderedLayerStyleKinds() []LayerStyleKind {
	return supportedLayerStyleKinds
}

func isSupportedLayerStyleKind(kind LayerStyleKind) bool {
	for _, supported := range supportedLayerStyleKinds {
		if supported == kind {
			return true
		}
	}
	return false
}

func applyLayerStyleEffect(dst, sourceSurface []byte, docW, docH int, style DecodedLayerStyle, ctx styleRenderContext) {
	switch LayerStyleKind(style.Kind) {
	case LayerStyleKindColorOverlay:
		compositeDocumentSurfaceClipped(dst, buildColorOverlaySurface(sourceSurface, docW, docH, style.ColorOverlay), docW, style.ColorOverlay.BlendMode, style.ColorOverlay.Opacity, nil, nil)
	case LayerStyleKindGradientOverlay:
		compositeDocumentSurfaceClipped(dst, buildGradientOverlaySurface(sourceSurface, docW, docH, style.GradientOverlay), docW, style.GradientOverlay.BlendMode, style.GradientOverlay.Opacity, nil, nil)
	case LayerStyleKindPatternOverlay:
		compositeDocumentSurfaceClipped(dst, buildPatternOverlaySurface(sourceSurface, docW, docH, style.PatternOverlay, ctx), docW, style.PatternOverlay.BlendMode, style.PatternOverlay.Opacity, nil, nil)
	case LayerStyleKindStroke:
		compositeDocumentSurfaceClipped(dst, buildStrokeSurface(sourceSurface, docW, docH, style.Stroke, ctx), docW, style.Stroke.BlendMode, style.Stroke.Opacity, nil, nil)
	case LayerStyleKindInnerShadow:
		compositeDocumentSurfaceClipped(dst, buildInnerShadowSurface(sourceSurface, docW, docH, style.InnerShadow), docW, style.InnerShadow.BlendMode, style.InnerShadow.Opacity, nil, nil)
	case LayerStyleKindInnerGlow:
		compositeDocumentSurfaceClipped(dst, buildInnerGlowSurface(sourceSurface, docW, docH, style.InnerGlow), docW, style.InnerGlow.BlendMode, style.InnerGlow.Opacity, nil, nil)
	case LayerStyleKindBevelEmboss:
		highlight, shadow := buildBevelEmbossSurfaces(sourceSurface, docW, docH, style.BevelEmboss)
		compositeDocumentSurfaceClipped(dst, highlight, docW, style.BevelEmboss.Highlight, bevelEmbossOpacity(style.BevelEmboss.HighlightO, style.BevelEmboss.Depth), nil, nil)
		compositeDocumentSurfaceClipped(dst, shadow, docW, style.BevelEmboss.Shadow, bevelEmbossOpacity(style.BevelEmboss.ShadowO, style.BevelEmboss.Depth), nil, nil)
	case LayerStyleKindSatin:
		compositeDocumentSurfaceClipped(dst, buildSatinSurface(sourceSurface, docW, docH, style.Satin), docW, style.Satin.BlendMode, style.Satin.Opacity, nil, nil)
	case LayerStyleKindDropShadow:
		compositeDocumentSurfaceClipped(dst, buildDropShadowSurface(sourceSurface, docW, docH, style.DropShadow), docW, style.DropShadow.BlendMode, style.DropShadow.Opacity, nil, nil)
	case LayerStyleKindOuterGlow:
		compositeDocumentSurfaceClipped(dst, buildOuterGlowSurface(sourceSurface, docW, docH, style.OuterGlow), docW, style.OuterGlow.BlendMode, style.OuterGlow.Opacity, nil, nil)
	}
}

func buildColorOverlaySurface(sourceSurface []byte, docW, docH int, params ColorOverlayParams) []byte {
	mask := agglib.AlphaMaskFromRGBA(sourceSurface, docW, docH)
	return agglib.RenderMaskedSolidRGBA(mask, aggColor(params.Color))
}

func buildGradientOverlaySurface(sourceSurface []byte, docW, docH int, params GradientOverlayParams) []byte {
	mask := agglib.AlphaMaskFromRGBA(sourceSurface, docW, docH)
	var surface []byte
	if len(params.Stops) == 0 {
		// Legacy two-color ramp. This path is byte-identical to the pre-stops
		// output REGARDLESS of Align (regression pin on the default param
		// set); Align only takes effect with explicit stops.
		surface = agglib.RenderMaskedLinearGradientRGBA(mask, gradientFill(params))
	} else {
		lut := buildGradientLUT(params.Stops, gradientRampColor(0), gradientRampColor(1))
		surface = renderMaskedLinearGradientLUT(mask, lut, params.Angle, params.Scale, params.Reverse, params.Align)
	}
	if params.Dither {
		applyGradientDitherMasked(surface, docW, docH)
	}
	return surface
}

func buildPatternOverlaySurface(sourceSurface []byte, docW, docH int, params PatternOverlayParams, ctx styleRenderContext) []byte {
	mask := agglib.AlphaMaskFromRGBA(sourceSurface, docW, docH)
	if pattern := ctx.pattern(params.PatternID); pattern != nil {
		return renderMaskedPatternSurface(mask, pattern, params.Scale)
	}
	// Empty or unknown pattern ID keeps the legacy procedural checker
	// byte-identical (regression pin).
	return agglib.RenderMaskedCheckerPatternRGBA(mask, checkerPatternFill(params.Scale))
}

func buildStrokeSurface(sourceSurface []byte, docW, docH int, params StrokeParams, ctx styleRenderContext) []byte {
	mask := agglib.AlphaMaskFromRGBA(sourceSurface, docW, docH)
	strokeMask := strokeMaskFromAlpha(mask, params.Size, params.Position)
	switch params.FillType {
	case "gradient":
		if len(params.Stops) == 0 {
			// Legacy blue-to-orange ramp (regression pin).
			return agglib.RenderMaskedLinearGradientRGBA(strokeMask, gradientFill(GradientOverlayParams{Scale: 1}))
		}
		lut := buildGradientLUT(params.Stops, gradientRampColor(0), gradientRampColor(1))
		return renderMaskedLinearGradientLUT(strokeMask, lut, params.GradientAngle, 1, false, true)
	case "pattern":
		if pattern := ctx.pattern(params.PatternID); pattern != nil {
			return renderMaskedPatternSurface(strokeMask, pattern, 1)
		}
		// Empty or unknown pattern ID keeps the legacy checker (pin).
		return agglib.RenderMaskedCheckerPatternRGBA(strokeMask, checkerPatternFill(1))
	default:
		return agglib.RenderMaskedSolidRGBA(strokeMask, aggColor(params.Color))
	}
}

// renderMaskedLinearGradientLUT paints a linear gradient sampled from a
// 256-entry LUT through the mask. It replicates the center-based projection
// math of agg_go's RenderMaskedLinearGradientRGBA (span = maxAxis*scale along
// angle theta, clamped projection). align=true anchors the gradient to the
// bounding box of nonzero mask alpha (Photoshop "align with layer");
// align=false spans the full document surface.
func renderMaskedLinearGradientLUT(mask agglib.AlphaMask, lut [256]agglib.Color, angle, scale float64, reverse, align bool) []byte {
	surface := make([]byte, maxInt(0, mask.Width*mask.Height*4))
	minX, minY := 0, 0
	maxX, maxY := mask.Width-1, mask.Height-1
	if align {
		if bMinX, bMinY, bMaxX, bMaxY, ok := maskAlphaBounds(mask); ok {
			minX, minY, maxX, maxY = bMinX, bMinY, bMaxX, bMaxY
		}
	}
	centerX := (float64(minX) + float64(maxX)) / 2
	centerY := (float64(minY) + float64(maxY)) / 2
	maxAxis := math.Max(float64(maxX-minX+1), float64(maxY-minY+1))
	theta := angle * math.Pi / 180
	span := math.Max(1, maxAxis*math.Max(0.1, scale))
	cosTheta := math.Cos(theta)
	sinTheta := math.Sin(theta)
	x1 := centerX - cosTheta*span/2
	y1 := centerY + sinTheta*span/2
	x2 := centerX + cosTheta*span/2
	y2 := centerY - sinTheta*span/2
	denom := math.Max(1, (x2-x1)*(x2-x1)+(y2-y1)*(y2-y1))
	for y := 0; y < mask.Height; y++ {
		for x := 0; x < mask.Width; x++ {
			maskAlpha := mask.At(x, y)
			if maskAlpha == 0 {
				continue
			}
			projected := ((float64(x)-x1)*(x2-x1) + (float64(y)-y1)*(y2-y1)) / denom
			t := clampUnit(projected)
			if reverse {
				t = 1 - t
			}
			color := lut[int(math.Round(t*255))]
			offset := (y*mask.Width + x) * 4
			surface[offset] = color.R
			surface[offset+1] = color.G
			surface[offset+2] = color.B
			surface[offset+3] = scaleMaskedAlpha(color.A, maskAlpha)
		}
	}
	return surface
}

// maskAlphaBounds returns the inclusive bounding box of nonzero mask alpha.
func maskAlphaBounds(mask agglib.AlphaMask) (minX, minY, maxX, maxY int, ok bool) {
	minX, minY = mask.Width, mask.Height
	maxX, maxY = -1, -1
	for y := 0; y < mask.Height; y++ {
		rowStart := y * mask.Width
		for x := 0; x < mask.Width; x++ {
			if mask.Pix[rowStart+x] == 0 {
				continue
			}
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
	}
	if maxX < 0 {
		return 0, 0, 0, 0, false
	}
	return minX, minY, maxX, maxY, true
}

// renderMaskedPatternSurface samples the resolved pattern tile in document
// space (nearest neighbor via samplePatternColor) wherever the mask has
// coverage, scaling the tile alpha by the mask alpha.
func renderMaskedPatternSurface(mask agglib.AlphaMask, pattern *model.PatternResource, scale float64) []byte {
	surface := make([]byte, maxInt(0, mask.Width*mask.Height*4))
	for y := 0; y < mask.Height; y++ {
		for x := 0; x < mask.Width; x++ {
			maskAlpha := mask.At(x, y)
			if maskAlpha == 0 {
				continue
			}
			color := samplePatternColor(pattern, x, y, scale)
			offset := (y*mask.Width + x) * 4
			surface[offset] = color[0]
			surface[offset+1] = color[1]
			surface[offset+2] = color[2]
			surface[offset+3] = scaleMaskedAlpha(color[3], maskAlpha)
		}
	}
	return surface
}

// applyGradientDitherMasked applies the same ordered jitter as
// applyGradientDither (fill_gradient.go) but only where the surface has
// coverage, so transparent pixels keep their zero bytes.
func applyGradientDitherMasked(buffer []byte, width, height int) {
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			idx := (y*width + x) * 4
			if buffer[idx+3] == 0 {
				continue
			}
			noise := uint32(x*1103515245 ^ y*12345)
			jitter := float64((noise>>24)&0x7) / 255.0
			for channel := 0; channel < 3; channel++ {
				value := float64(buffer[idx+channel]) / 255.0
				value += (jitter - 0.014) * 0.25
				if value < 0 {
					value = 0
				} else if value > 1 {
					value = 1
				}
				buffer[idx+channel] = uint8(math.Round(value * 255))
			}
		}
	}
}

func buildDropShadowSurface(sourceSurface []byte, docW, docH int, params DropShadowParams) []byte {
	mask := agglib.AlphaMaskFromRGBA(sourceSurface, docW, docH)
	working := shiftAndBlurShadowMask(mask, params.Angle, params.Distance, params.Spread, params.Size)
	if params.Knockout {
		// Knockout removes the shadow wherever the layer's own pixels cover
		// it; default false keeps the legacy output (regression pin).
		working = working.Subtract(mask)
	}
	working = applyNoiseToMask(working, params.Noise)
	return agglib.RenderMaskedSolidRGBA(working, aggColor(params.Color))
}

func buildInnerShadowSurface(sourceSurface []byte, docW, docH int, params InnerShadowParams) []byte {
	mask := agglib.AlphaMaskFromRGBA(sourceSurface, docW, docH)
	dx, dy := dropShadowOffset(params.Angle, params.Distance)
	shadowMask := mask.Subtract(mask.Shifted(dx, dy))
	if chokeRadius := alphaRadius(params.Choke * params.Size); chokeRadius > 0 {
		shadowMask = shadowMask.Dilated(chokeRadius)
	}
	if blurRadius := alphaRadius(params.Size); blurRadius > 0 {
		shadowMask = shadowMask.Blurred(blurRadius)
	}
	shadowMask = shadowMask.Intersect(mask)
	shadowMask = applyNoiseToMask(shadowMask, params.Noise)
	return agglib.RenderMaskedSolidRGBA(shadowMask, aggColor(params.Color))
}

func buildOuterGlowSurface(sourceSurface []byte, docW, docH int, params GlowParams) []byte {
	mask := agglib.AlphaMaskFromRGBA(sourceSurface, docW, docH)
	working := mask.Clone()
	if spreadRadius := alphaRadius(params.Spread * params.Size); spreadRadius > 0 {
		working = working.Dilated(spreadRadius)
	}
	if blurRadius := alphaRadius(params.Size); blurRadius > 0 {
		working = working.Blurred(blurRadius)
	}
	working = working.Subtract(mask)
	working = applyNoiseToMask(working, params.Noise)
	return agglib.RenderMaskedSolidRGBA(working, aggColor(params.Color))
}

func buildInnerGlowSurface(sourceSurface []byte, docW, docH int, params GlowParams) []byte {
	mask := agglib.AlphaMaskFromRGBA(sourceSurface, docW, docH)
	innerMask := mask.Clone()
	if spreadRadius := alphaRadius(params.Spread * params.Size); spreadRadius > 0 {
		innerMask = innerMask.Eroded(spreadRadius)
	}
	innerMask = mask.Subtract(innerMask)
	if blurRadius := alphaRadius(params.Size); blurRadius > 0 {
		innerMask = innerMask.Blurred(blurRadius)
	}
	innerMask = innerMask.Intersect(mask)
	innerMask = applyNoiseToMask(innerMask, params.Noise)
	return agglib.RenderMaskedSolidRGBA(innerMask, aggColor(params.Color))
}

// applyNoiseToMask applies a style's Noise parameter as the final mask step:
// alpha' = round(alpha * (1 - noise*u)) with u = pixelNoiseSeed(x, y)
// normalized to [0, 1) — the deterministic splitmix64 dissolve convention in
// document coordinates (effect masks are document-sized). noise <= 0 returns
// the mask untouched (regression pin).
func applyNoiseToMask(mask agglib.AlphaMask, noise float64) agglib.AlphaMask {
	if noise <= 0 {
		return mask
	}
	out := mask.Clone()
	for y := 0; y < out.Height; y++ {
		rowStart := y * out.Width
		for x := 0; x < out.Width; x++ {
			alpha := out.Pix[rowStart+x]
			if alpha == 0 {
				continue
			}
			u := float64(pixelNoiseSeed(x, y)) / (1 << 32)
			out.Pix[rowStart+x] = uint8(math.Round(float64(alpha) * (1 - noise*u)))
		}
	}
	return out
}

func buildBevelEmbossSurfaces(sourceSurface []byte, docW, docH int, params BevelEmbossParams) ([]byte, []byte) {
	mask := agglib.AlphaMaskFromRGBA(sourceSurface, docW, docH)
	shapeMask := bevelEmbossShapeMask(mask, params)
	reliefMask := bevelEmbossMask(mask, params)
	chisel := params.Technique == "chisel-hard" || params.Technique == "chisel-soft"
	if chisel {
		reliefMask = reliefMask.Intersect(chiselRampMask(shapeMask, math.Max(1, params.Size)))
	}
	// Altitude scales the directional offset; the historical default of 30
	// degrees keeps a factor of exactly 1 (regression pin). At >= 89 degrees
	// the light is overhead: the offset collapses to zero and the min-1px
	// fallback is deliberately dropped so the bevel flattens.
	offsetDistance := math.Max(1, params.Size) * bevelAltitudeFactor(params.Altitude)
	dx, dy := dropShadowOffset(params.Angle, offsetDistance)
	if dx == 0 && dy == 0 && params.Altitude < 89 {
		dx = 1
	}
	highlightMask := reliefMask.Intersect(shapeMask.Subtract(shapeMask.Shifted(dx, dy)))
	shadowMask := reliefMask.Intersect(shapeMask.Subtract(shapeMask.Shifted(-dx, -dy)))
	if params.Direction == "down" {
		highlightMask, shadowMask = shadowMask, highlightMask
	}
	if params.Style == "pillow-emboss" {
		highlightMask, shadowMask = shadowMask.Intersect(mask), highlightMask.Intersect(mask)
	}
	switch params.Technique {
	case "chisel-hard":
		// Hard chisel keeps the raw distance-ramp relief: no Soften blur.
	case "chisel-soft":
		blurRadius := maxInt(1, alphaRadius(params.Size/4))
		highlightMask = highlightMask.Blurred(blurRadius)
		shadowMask = shadowMask.Blurred(blurRadius)
	default: // "smooth" keeps the legacy Soften blur (regression pin).
		if blurRadius := alphaRadius(params.Soften); blurRadius > 0 {
			highlightMask = highlightMask.Blurred(blurRadius)
			shadowMask = shadowMask.Blurred(blurRadius)
		}
	}
	highlightMask = applyContourToMask(highlightMask, params.Contour)
	shadowMask = applyContourToMask(shadowMask, params.Contour)
	return agglib.RenderMaskedSolidRGBA(highlightMask, aggColor(params.HighlightC)),
		agglib.RenderMaskedSolidRGBA(shadowMask, aggColor(params.ShadowC))
}

func buildSatinSurface(sourceSurface []byte, docW, docH int, params SatinParams) []byte {
	mask := agglib.AlphaMaskFromRGBA(sourceSurface, docW, docH)
	dx, dy := dropShadowOffset(params.Angle, params.Distance)
	satinMask := mask.Shifted(dx, dy).AbsDiff(mask.Shifted(-dx, -dy)).Intersect(mask)
	if blurRadius := alphaRadius(params.Size); blurRadius > 0 {
		satinMask = satinMask.Blurred(blurRadius).Intersect(mask)
	}
	// Contour remaps the blurred satin coverage before Invert. Satin's
	// DEFAULT contour is "gaussian", so satin output with blurred (non-binary)
	// masks deliberately changed when contours were wired up in Phase S.6 V3;
	// the rebaselined expectations live in the layer-style params tests.
	satinMask = applyContourToMask(satinMask, params.Contour)
	if params.Invert {
		satinMask = mask.Subtract(satinMask)
	}
	return agglib.RenderMaskedSolidRGBA(satinMask, aggColor(params.Color))
}

// bevelAltitudeFactor scales the bevel's directional offset by the global
// light altitude: cos(alt)/cos(30 degrees), so altitude 30 (the historical
// default) yields exactly 1. The reference cosine goes through the same
// runtime conversion as the input so the division is bit-exact at 30 —
// a constant-folded 30*math.Pi/180 would differ in the last ulp.
func bevelAltitudeFactor(altitude float64) float64 {
	toRadians := func(degrees float64) float64 { return degrees * math.Pi / 180 }
	return math.Cos(toRadians(altitude)) / math.Cos(toRadians(30))
}

// contourLUT builds the 256-entry remap table for a normalized contour name.
// "linear" (and any unknown name) is the identity table.
func contourLUT(name string) [256]uint8 {
	var lut [256]uint8
	for i := range lut {
		t := float64(i) / 255
		var v float64
		switch name {
		case "gaussian":
			v = t * t * (3 - 2*t)
		case "cone":
			if t < 0.5 {
				v = 2 * t
			} else {
				v = 2 - 2*t
			}
		case "rolling-slope":
			v = math.Sqrt(1 - (1-t)*(1-t))
		case "rounded-steps":
			s := t * 4
			step := math.Floor(s)
			if step > 3 {
				step = 3
			}
			frac := s - step
			v = (step + frac*frac*(3-2*frac)) / 4
		default: // "linear" and unknown names: identity
			v = t
		}
		lut[i] = uint8(math.Round(clampUnit(v) * 255))
	}
	return lut
}

// applyContourToMask remaps mask coverage through the contour LUT. Linear is
// the identity and returns the mask untouched (regression pin).
func applyContourToMask(mask agglib.AlphaMask, contour string) agglib.AlphaMask {
	if contour == "" || contour == "linear" {
		return mask
	}
	lut := contourLUT(contour)
	out := mask.Clone()
	for i, alpha := range out.Pix {
		out.Pix[i] = lut[alpha]
	}
	return out
}

// chiselRampMask approximates a distance-transform relief for the chisel
// techniques by accumulating iterated radius-1 erosions (box min-filters
// compose, so applying the radius-1 erosion i times equals Eroded(i)). The
// average over the mask itself plus its first size-1 erosions ramps from
// 255/size at the shape edge to full coverage `size` pixels inside —
// O(size) erosion passes. (Starting at the un-eroded mask instead of
// Eroded(1) keeps the outermost bevel pixel visible and lets size 1
// degenerate to the flat band rather than an empty relief.)
func chiselRampMask(mask agglib.AlphaMask, size float64) agglib.AlphaMask {
	passes := maxInt(1, alphaRadius(math.Max(1, size)))
	acc := make([]uint32, len(mask.Pix))
	working := mask
	for i := 0; i < passes; i++ {
		if i > 0 {
			working = working.Eroded(1)
		}
		for index, alpha := range working.Pix {
			acc[index] += uint32(alpha)
		}
	}
	out := agglib.NewAlphaMask(mask.Width, mask.Height)
	for index := range out.Pix {
		out.Pix[index] = uint8(acc[index] / uint32(passes))
	}
	return out
}

func bevelEmbossOpacity(baseOpacity, depth float64) float64 {
	return clampUnit(baseOpacity * math.Max(0.25, depth))
}

func bevelEmbossMask(mask agglib.AlphaMask, params BevelEmbossParams) agglib.AlphaMask {
	size := math.Max(1, params.Size)
	switch params.Style {
	case "outer-bevel":
		return strokeMaskFromAlpha(mask, size, "outside")
	case "emboss":
		return strokeMaskFromAlpha(mask, size, "outside").Max(strokeMaskFromAlpha(mask, size, "inside"))
	case "pillow-emboss":
		return strokeMaskFromAlpha(mask, size, "inside")
	case "stroke-emboss":
		return strokeMaskFromAlpha(mask, size, "center")
	default:
		return strokeMaskFromAlpha(mask, size, "inside")
	}
}

func bevelEmbossShapeMask(mask agglib.AlphaMask, params BevelEmbossParams) agglib.AlphaMask {
	size := alphaRadius(math.Max(1, params.Size))
	switch params.Style {
	case "outer-bevel", "emboss":
		return mask.Dilated(size)
	case "stroke-emboss":
		return strokeMaskFromAlpha(mask, math.Max(1, params.Size), "center")
	default:
		return mask
	}
}

func shiftAndBlurShadowMask(mask agglib.AlphaMask, angle, distance, spread, size float64) agglib.AlphaMask {
	dx, dy := dropShadowOffset(angle, distance)
	working := mask.Shifted(dx, dy)
	if spreadRadius := alphaRadius(spread * size); spreadRadius > 0 {
		working = working.Dilated(spreadRadius)
	}
	if blurRadius := alphaRadius(size); blurRadius > 0 {
		working = working.Blurred(blurRadius)
	}
	return working
}

func strokeMaskFromAlpha(mask agglib.AlphaMask, size float64, position string) agglib.AlphaMask {
	radius := alphaRadius(size)
	if radius <= 0 {
		return agglib.NewAlphaMask(mask.Width, mask.Height)
	}

	switch position {
	case "inside":
		return mask.Subtract(mask.Eroded(radius))
	case "center":
		outerRadius := maxInt(1, alphaRadius(size/2))
		innerRadius := maxInt(1, int(math.Floor(size/2)))
		return mask.Dilated(outerRadius).Subtract(mask.Eroded(innerRadius))
	default:
		return mask.Dilated(radius).Subtract(mask)
	}
}

func gradientRampColor(t float64) [4]uint8 {
	start := [3]float64{32, 64, 255}
	end := [3]float64{255, 196, 32}
	return [4]uint8{
		uint8(math.Round(start[0] + (end[0]-start[0])*t)),
		uint8(math.Round(start[1] + (end[1]-start[1])*t)),
		uint8(math.Round(start[2] + (end[2]-start[2])*t)),
		255,
	}
}

func gradientFill(params GradientOverlayParams) agglib.LinearGradientFill {
	return agglib.LinearGradientFill{
		Start:   aggColor(gradientRampColor(0)),
		End:     aggColor(gradientRampColor(1)),
		Angle:   params.Angle,
		Scale:   math.Max(0.1, params.Scale),
		Reverse: params.Reverse,
	}
}

func checkerPatternFill(scale float64) agglib.CheckerPatternFill {
	return agglib.CheckerPatternFill{
		First:  agglib.NewColorRGB(32, 160, 255),
		Second: agglib.NewColorRGB(255, 224, 96),
		Scale:  math.Max(1, scale),
	}
}

func aggColor(color [4]uint8) agglib.Color {
	return agglib.NewColor(color[0], color[1], color[2], color[3])
}

func alphaRadius(value float64) int {
	return int(math.Round(math.Max(0, value)))
}

// dropShadowOffset converts a Photoshop-style light angle into a shadow
// offset in screen coordinates (y grows downward).
//
// Photoshop convention: `angle` is the direction the light comes FROM,
// measured counterclockwise from the positive x-axis. The light vector for
// angle a is (cos a, -sin a) in screen coords, and the shadow falls in the
// opposite direction, so the offset is (-cos(a), +sin(a)) * distance. At the
// default global light of 120° the shadow falls down-right (dx>0, dy>0),
// away from an upper-left light.
func dropShadowOffset(angle, distance float64) (int, int) {
	radians := angle * math.Pi / 180
	dx := int(math.Round(-math.Cos(radians) * distance))
	dy := int(math.Round(math.Sin(radians) * distance))
	return dx, dy
}

// layerStyleKindRendersBehindContent reports whether an effect belongs to
// the group Photoshop composites beneath the layer's own pixels (drop
// shadow and outer glow); all remaining effects render above the content.
func layerStyleKindRendersBehindContent(kind LayerStyleKind) bool {
	return kind == LayerStyleKindDropShadow || kind == LayerStyleKindOuterGlow
}
