package engine

import (
	"math"

	agglib "github.com/cwbudde/agg_go"
)

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

func applyLayerStyleEffect(dst, sourceSurface []byte, docW, docH int, style DecodedLayerStyle) {
	switch LayerStyleKind(style.Kind) {
	case LayerStyleKindColorOverlay:
		compositeDocumentSurface(dst, buildColorOverlaySurface(sourceSurface, docW, docH, style.ColorOverlay), style.ColorOverlay.BlendMode, style.ColorOverlay.Opacity)
	case LayerStyleKindGradientOverlay:
		compositeDocumentSurface(dst, buildGradientOverlaySurface(sourceSurface, docW, docH, style.GradientOverlay), style.GradientOverlay.BlendMode, style.GradientOverlay.Opacity)
	case LayerStyleKindPatternOverlay:
		compositeDocumentSurface(dst, buildPatternOverlaySurface(sourceSurface, docW, docH, style.PatternOverlay), style.PatternOverlay.BlendMode, style.PatternOverlay.Opacity)
	case LayerStyleKindStroke:
		compositeDocumentSurface(dst, buildStrokeSurface(sourceSurface, docW, docH, style.Stroke), style.Stroke.BlendMode, style.Stroke.Opacity)
	case LayerStyleKindInnerShadow:
		compositeDocumentSurface(dst, buildInnerShadowSurface(sourceSurface, docW, docH, style.InnerShadow), style.InnerShadow.BlendMode, style.InnerShadow.Opacity)
	case LayerStyleKindInnerGlow:
		compositeDocumentSurface(dst, buildInnerGlowSurface(sourceSurface, docW, docH, style.InnerGlow), style.InnerGlow.BlendMode, style.InnerGlow.Opacity)
	case LayerStyleKindBevelEmboss:
		highlight, shadow := buildBevelEmbossSurfaces(sourceSurface, docW, docH, style.BevelEmboss)
		compositeDocumentSurface(dst, highlight, style.BevelEmboss.Highlight, bevelEmbossOpacity(style.BevelEmboss.HighlightO, style.BevelEmboss.Depth))
		compositeDocumentSurface(dst, shadow, style.BevelEmboss.Shadow, bevelEmbossOpacity(style.BevelEmboss.ShadowO, style.BevelEmboss.Depth))
	case LayerStyleKindSatin:
		compositeDocumentSurface(dst, buildSatinSurface(sourceSurface, docW, docH, style.Satin), style.Satin.BlendMode, style.Satin.Opacity)
	case LayerStyleKindDropShadow:
		compositeDocumentSurface(dst, buildDropShadowSurface(sourceSurface, docW, docH, style.DropShadow), style.DropShadow.BlendMode, style.DropShadow.Opacity)
	case LayerStyleKindOuterGlow:
		compositeDocumentSurface(dst, buildOuterGlowSurface(sourceSurface, docW, docH, style.OuterGlow), style.OuterGlow.BlendMode, style.OuterGlow.Opacity)
	}
}

func buildColorOverlaySurface(sourceSurface []byte, docW, docH int, params ColorOverlayParams) []byte {
	mask := agglib.AlphaMaskFromRGBA(sourceSurface, docW, docH)
	return agglib.RenderMaskedSolidRGBA(mask, aggColor(params.Color))
}

func buildGradientOverlaySurface(sourceSurface []byte, docW, docH int, params GradientOverlayParams) []byte {
	mask := agglib.AlphaMaskFromRGBA(sourceSurface, docW, docH)
	return agglib.RenderMaskedLinearGradientRGBA(mask, gradientFill(params))
}

func buildPatternOverlaySurface(sourceSurface []byte, docW, docH int, params PatternOverlayParams) []byte {
	mask := agglib.AlphaMaskFromRGBA(sourceSurface, docW, docH)
	return agglib.RenderMaskedCheckerPatternRGBA(mask, checkerPatternFill(params.Scale))
}

func buildStrokeSurface(sourceSurface []byte, docW, docH int, params StrokeParams) []byte {
	mask := agglib.AlphaMaskFromRGBA(sourceSurface, docW, docH)
	strokeMask := strokeMaskFromAlpha(mask, params.Size, params.Position)
	switch params.FillType {
	case "gradient":
		return agglib.RenderMaskedLinearGradientRGBA(strokeMask, gradientFill(GradientOverlayParams{Scale: 1}))
	case "pattern":
		return agglib.RenderMaskedCheckerPatternRGBA(strokeMask, checkerPatternFill(1))
	default:
		return agglib.RenderMaskedSolidRGBA(strokeMask, aggColor(params.Color))
	}
}

func buildDropShadowSurface(sourceSurface []byte, docW, docH int, params DropShadowParams) []byte {
	mask := agglib.AlphaMaskFromRGBA(sourceSurface, docW, docH)
	working := shiftAndBlurShadowMask(mask, params.Angle, params.Distance, params.Spread, params.Size)
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
	return agglib.RenderMaskedSolidRGBA(innerMask, aggColor(params.Color))
}

func buildBevelEmbossSurfaces(sourceSurface []byte, docW, docH int, params BevelEmbossParams) ([]byte, []byte) {
	mask := agglib.AlphaMaskFromRGBA(sourceSurface, docW, docH)
	reliefMask := bevelEmbossMask(mask, params)
	shapeMask := bevelEmbossShapeMask(mask, params)
	offsetDistance := math.Max(1, params.Size)
	dx, dy := dropShadowOffset(params.Angle, offsetDistance)
	if dx == 0 && dy == 0 {
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
	if blurRadius := alphaRadius(params.Soften); blurRadius > 0 {
		highlightMask = highlightMask.Blurred(blurRadius)
		shadowMask = shadowMask.Blurred(blurRadius)
	}
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
	if params.Invert {
		satinMask = mask.Subtract(satinMask)
	}
	return agglib.RenderMaskedSolidRGBA(satinMask, aggColor(params.Color))
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

func dropShadowOffset(angle, distance float64) (int, int) {
	radians := angle * math.Pi / 180
	dx := int(math.Round(math.Cos(radians) * distance))
	dy := int(math.Round(-math.Sin(radians) * distance))
	return dx, dy
}
