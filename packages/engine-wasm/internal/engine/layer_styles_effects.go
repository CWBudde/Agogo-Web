package engine

import (
	"math"

	agglib "github.com/cwbudde/agg_go"
)

func orderedLayerStyleKinds() []LayerStyleKind {
	return []LayerStyleKind{
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
}

func isSupportedLayerStyleKind(kind LayerStyleKind) bool {
	switch kind {
	case LayerStyleKindColorOverlay,
		LayerStyleKindGradientOverlay,
		LayerStyleKindPatternOverlay,
		LayerStyleKindStroke,
		LayerStyleKindDropShadow,
		LayerStyleKindInnerShadow,
		LayerStyleKindOuterGlow,
		LayerStyleKindInnerGlow,
		LayerStyleKindBevelEmboss,
		LayerStyleKindSatin:
		return true
	default:
		return false
	}
}

func applyLayerStyleEffect(dst, sourceSurface []byte, docW, docH int, style DecodedLayerStyle) {
	switch LayerStyleKind(style.Kind) {
	case LayerStyleKindColorOverlay:
		compositeDocumentSurface(dst, buildColorOverlaySurface(sourceSurface, style.ColorOverlay), style.ColorOverlay.BlendMode, style.ColorOverlay.Opacity)
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

func buildColorOverlaySurface(sourceSurface []byte, params ColorOverlayParams) []byte {
	mask := alphaMaskFromSurface(sourceSurface)
	return colorSurfaceFromMask(mask, params.Color)
}

func buildGradientOverlaySurface(sourceSurface []byte, docW, docH int, params GradientOverlayParams) []byte {
	mask := alphaMaskFromSurface(sourceSurface)
	return gradientSurfaceFromMask(mask, docW, docH, params.Angle, params.Scale, params.Reverse)
}

func buildPatternOverlaySurface(sourceSurface []byte, docW, docH int, params PatternOverlayParams) []byte {
	mask := alphaMaskFromSurface(sourceSurface)
	return patternSurfaceFromMask(mask, docW, docH, params.Scale)
}

func buildStrokeSurface(sourceSurface []byte, docW, docH int, params StrokeParams) []byte {
	mask := alphaMaskFromSurface(sourceSurface)
	strokeMask := strokeMaskFromAlpha(mask, docW, docH, params.Size, params.Position)
	switch params.FillType {
	case "gradient":
		return gradientSurfaceFromMask(strokeMask, docW, docH, 0, 1, false)
	case "pattern":
		return patternSurfaceFromMask(strokeMask, docW, docH, 1)
	default:
		return colorSurfaceFromMask(strokeMask, params.Color)
	}
}

func buildDropShadowSurface(sourceSurface []byte, docW, docH int, params DropShadowParams) []byte {
	mask := alphaMaskFromSurface(sourceSurface)
	working := shiftAndBlurShadowMask(mask, docW, docH, params.Angle, params.Distance, params.Spread, params.Size)
	return colorSurfaceFromMask(working, params.Color)
}

func buildInnerShadowSurface(sourceSurface []byte, docW, docH int, params InnerShadowParams) []byte {
	mask := alphaMaskFromSurface(sourceSurface)
	dx, dy := dropShadowOffset(params.Angle, params.Distance)
	shifted := shiftAlphaMask(mask, docW, docH, dx, dy)
	shadowMask := subtractAlphaMask(mask, shifted)
	if chokeRadius := alphaRadius(params.Choke * params.Size); chokeRadius > 0 {
		shadowMask = dilateAlphaMask(shadowMask, docW, docH, chokeRadius)
	}
	if blurRadius := alphaRadius(params.Size); blurRadius > 0 {
		shadowMask = blurAlphaMask(shadowMask, docW, docH, blurRadius)
	}
	shadowMask = intersectAlphaMask(shadowMask, mask)
	return colorSurfaceFromMask(shadowMask, params.Color)
}

func buildOuterGlowSurface(sourceSurface []byte, docW, docH int, params GlowParams) []byte {
	mask := alphaMaskFromSurface(sourceSurface)
	working := append([]byte(nil), mask...)
	if spreadRadius := alphaRadius(params.Spread * params.Size); spreadRadius > 0 {
		working = dilateAlphaMask(working, docW, docH, spreadRadius)
	}
	if blurRadius := alphaRadius(params.Size); blurRadius > 0 {
		working = blurAlphaMask(working, docW, docH, blurRadius)
	}
	working = subtractAlphaMask(working, mask)
	return colorSurfaceFromMask(working, params.Color)
}

func buildInnerGlowSurface(sourceSurface []byte, docW, docH int, params GlowParams) []byte {
	mask := alphaMaskFromSurface(sourceSurface)
	innerMask := append([]byte(nil), mask...)
	if spreadRadius := alphaRadius(params.Spread * params.Size); spreadRadius > 0 {
		innerMask = erodeAlphaMask(innerMask, docW, docH, spreadRadius)
	}
	innerMask = subtractAlphaMask(mask, innerMask)
	if blurRadius := alphaRadius(params.Size); blurRadius > 0 {
		innerMask = blurAlphaMask(innerMask, docW, docH, blurRadius)
	}
	innerMask = intersectAlphaMask(innerMask, mask)
	return colorSurfaceFromMask(innerMask, params.Color)
}

func buildBevelEmbossSurfaces(sourceSurface []byte, docW, docH int, params BevelEmbossParams) ([]byte, []byte) {
	mask := alphaMaskFromSurface(sourceSurface)
	reliefMask := bevelEmbossMask(mask, docW, docH, params)
	shapeMask := bevelEmbossShapeMask(mask, docW, docH, params)
	offsetDistance := math.Max(1, params.Size)
	dx, dy := dropShadowOffset(params.Angle, offsetDistance)
	if dx == 0 && dy == 0 {
		dx = 1
	}
	highlightMask := intersectAlphaMask(reliefMask, subtractAlphaMask(shapeMask, shiftAlphaMask(shapeMask, docW, docH, dx, dy)))
	shadowMask := intersectAlphaMask(reliefMask, subtractAlphaMask(shapeMask, shiftAlphaMask(shapeMask, docW, docH, -dx, -dy)))
	if params.Direction == "down" {
		highlightMask, shadowMask = shadowMask, highlightMask
	}
	if params.Style == "pillow-emboss" {
		highlightMask, shadowMask = intersectAlphaMask(shadowMask, mask), intersectAlphaMask(highlightMask, mask)
	}
	if blurRadius := alphaRadius(params.Soften); blurRadius > 0 {
		highlightMask = blurAlphaMask(highlightMask, docW, docH, blurRadius)
		shadowMask = blurAlphaMask(shadowMask, docW, docH, blurRadius)
	}
	return colorSurfaceFromMask(highlightMask, params.HighlightC), colorSurfaceFromMask(shadowMask, params.ShadowC)
}

func buildSatinSurface(sourceSurface []byte, docW, docH int, params SatinParams) []byte {
	mask := alphaMaskFromSurface(sourceSurface)
	dx, dy := dropShadowOffset(params.Angle, params.Distance)
	forward := shiftAlphaMask(mask, docW, docH, dx, dy)
	backward := shiftAlphaMask(mask, docW, docH, -dx, -dy)
	satinMask := absDiffAlphaMask(forward, backward)
	satinMask = intersectAlphaMask(satinMask, mask)
	if blurRadius := alphaRadius(params.Size); blurRadius > 0 {
		satinMask = blurAlphaMask(satinMask, docW, docH, blurRadius)
		satinMask = intersectAlphaMask(satinMask, mask)
	}
	if params.Invert {
		satinMask = subtractAlphaMask(mask, satinMask)
	}
	return colorSurfaceFromMask(satinMask, params.Color)
}

func bevelEmbossOpacity(baseOpacity, depth float64) float64 {
	return clampUnit(baseOpacity * math.Max(0.25, depth))
}

func bevelEmbossMask(mask []byte, docW, docH int, params BevelEmbossParams) []byte {
	size := math.Max(1, params.Size)
	switch params.Style {
	case "outer-bevel":
		return strokeMaskFromAlpha(mask, docW, docH, size, "outside")
	case "emboss":
		outer := strokeMaskFromAlpha(mask, docW, docH, size, "outside")
		inner := strokeMaskFromAlpha(mask, docW, docH, size, "inside")
		return maxAlphaMask(outer, inner)
	case "pillow-emboss":
		return strokeMaskFromAlpha(mask, docW, docH, size, "inside")
	case "stroke-emboss":
		return strokeMaskFromAlpha(mask, docW, docH, size, "center")
	default:
		return strokeMaskFromAlpha(mask, docW, docH, size, "inside")
	}
}

func bevelEmbossShapeMask(mask []byte, docW, docH int, params BevelEmbossParams) []byte {
	size := alphaRadius(math.Max(1, params.Size))
	switch params.Style {
	case "outer-bevel":
		return dilateAlphaMask(mask, docW, docH, size)
	case "emboss":
		return dilateAlphaMask(mask, docW, docH, size)
	case "stroke-emboss":
		return strokeMaskFromAlpha(mask, docW, docH, math.Max(1, params.Size), "center")
	default:
		return mask
	}
}

func shiftAndBlurShadowMask(mask []byte, docW, docH int, angle, distance, spread, size float64) []byte {
	dx, dy := dropShadowOffset(angle, distance)
	working := shiftAlphaMask(mask, docW, docH, dx, dy)
	if spreadRadius := alphaRadius(spread * size); spreadRadius > 0 {
		working = dilateAlphaMask(working, docW, docH, spreadRadius)
	}
	if blurRadius := alphaRadius(size); blurRadius > 0 {
		working = blurAlphaMask(working, docW, docH, blurRadius)
	}
	return working
}

func strokeMaskFromAlpha(mask []byte, docW, docH int, size float64, position string) []byte {
	if len(mask) == 0 {
		return nil
	}
	radius := alphaRadius(size)
	if radius <= 0 {
		return make([]byte, len(mask))
	}

	switch position {
	case "inside":
		inner := erodeAlphaMask(mask, docW, docH, radius)
		return subtractAlphaMask(mask, inner)
	case "center":
		outerRadius := maxInt(1, alphaRadius(size/2))
		innerRadius := maxInt(1, int(math.Floor(size/2)))
		outer := dilateAlphaMask(mask, docW, docH, outerRadius)
		inner := erodeAlphaMask(mask, docW, docH, innerRadius)
		return subtractAlphaMask(outer, inner)
	default:
		outer := dilateAlphaMask(mask, docW, docH, radius)
		return subtractAlphaMask(outer, mask)
	}
}

func colorSurfaceFromMask(mask []byte, color [4]uint8) []byte {
	surface := make([]byte, len(mask)*4)
	for index, alpha := range mask {
		if alpha == 0 {
			continue
		}
		offset := index * 4
		surface[offset] = color[0]
		surface[offset+1] = color[1]
		surface[offset+2] = color[2]
		surface[offset+3] = scaleMaskedAlpha(alpha, color[3])
	}
	return surface
}

func gradientSurfaceFromMask(mask []byte, docW, docH int, angle, scale float64, reverse bool) []byte {
	surface := make([]byte, len(mask)*4)
	if len(mask) == 0 || docW <= 0 || docH <= 0 {
		return surface
	}
	centerX := float64(docW-1) / 2
	centerY := float64(docH-1) / 2
	maxAxis := math.Max(float64(docW), float64(docH))
	theta := angle * math.Pi / 180
	cosTheta := math.Cos(theta)
	sinTheta := math.Sin(theta)
	scale = math.Max(0.1, scale)
	span := math.Max(1, maxAxis*scale)

	for index, alpha := range mask {
		if alpha == 0 {
			continue
		}
		x := float64(index % docW)
		y := float64(index / docW)
		projected := ((x-centerX)*cosTheta + (centerY-y)*sinTheta) / span
		t := clampUnit(0.5 + projected)
		if reverse {
			t = 1 - t
		}
		color := gradientRampColor(t)
		offset := index * 4
		surface[offset] = color[0]
		surface[offset+1] = color[1]
		surface[offset+2] = color[2]
		surface[offset+3] = alpha
	}
	return surface
}

func patternSurfaceFromMask(mask []byte, docW, docH int, scale float64) []byte {
	surface := make([]byte, len(mask)*4)
	if len(mask) == 0 || docW <= 0 || docH <= 0 {
		return surface
	}
	tile := maxInt(1, int(math.Round(math.Max(1, scale)*4)))
	for index, alpha := range mask {
		if alpha == 0 {
			continue
		}
		x := index % docW
		y := index / docW
		var color [4]uint8
		if ((x/tile)+(y/tile))%2 == 0 {
			color = [4]uint8{32, 160, 255, 255}
		} else {
			color = [4]uint8{255, 224, 96, 255}
		}
		offset := index * 4
		surface[offset] = color[0]
		surface[offset+1] = color[1]
		surface[offset+2] = color[2]
		surface[offset+3] = alpha
	}
	return surface
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

func alphaMaskFromSurface(surface []byte) []byte {
	mask := make([]byte, len(surface)/4)
	for offset := 0; offset+3 < len(surface); offset += 4 {
		mask[offset/4] = surface[offset+3]
	}
	return mask
}

func shiftAlphaMask(mask []byte, docW, docH, dx, dy int) []byte {
	shifted := make([]byte, len(mask))
	if len(mask) == 0 || docW <= 0 || docH <= 0 {
		return shifted
	}
	for index, alpha := range mask {
		if alpha == 0 {
			continue
		}
		x := index % docW
		y := index / docW
		dstX := x + dx
		dstY := y + dy
		if dstX < 0 || dstX >= docW || dstY < 0 || dstY >= docH {
			continue
		}
		destIndex := dstY*docW + dstX
		if alpha > shifted[destIndex] {
			shifted[destIndex] = alpha
		}
	}
	return shifted
}

func blurAlphaMask(mask []byte, docW, docH, radius int) []byte {
	if len(mask) == 0 || docW <= 0 || docH <= 0 || radius <= 0 {
		return append([]byte(nil), mask...)
	}
	buffer := make([]byte, len(mask)*4)
	for index, alpha := range mask {
		offset := index * 4
		buffer[offset] = alpha
		buffer[offset+1] = alpha
		buffer[offset+2] = alpha
		buffer[offset+3] = alpha
	}
	stackBlur := agglib.NewStackBlur[agglib.ColorSpaceSRGB]()
	stackBlur.BlurRGBA8(buffer, docW, docH, docW*4, radius)
	blurred := make([]byte, len(mask))
	for offset := 0; offset+3 < len(buffer); offset += 4 {
		blurred[offset/4] = buffer[offset+3]
	}
	return blurred
}

func dilateAlphaMask(mask []byte, docW, docH, radius int) []byte {
	if len(mask) == 0 || docW <= 0 || docH <= 0 || radius <= 0 {
		return append([]byte(nil), mask...)
	}
	out := make([]byte, len(mask))
	for y := 0; y < docH; y++ {
		for x := 0; x < docW; x++ {
			var maxAlpha uint8
			for sampleY := maxInt(0, y-radius); sampleY <= minInt(docH-1, y+radius); sampleY++ {
				for sampleX := maxInt(0, x-radius); sampleX <= minInt(docW-1, x+radius); sampleX++ {
					alpha := mask[sampleY*docW+sampleX]
					if alpha > maxAlpha {
						maxAlpha = alpha
					}
				}
			}
			out[y*docW+x] = maxAlpha
		}
	}
	return out
}

func erodeAlphaMask(mask []byte, docW, docH, radius int) []byte {
	if len(mask) == 0 || docW <= 0 || docH <= 0 || radius <= 0 {
		return append([]byte(nil), mask...)
	}
	out := make([]byte, len(mask))
	for y := 0; y < docH; y++ {
		for x := 0; x < docW; x++ {
			minAlpha := uint8(255)
			for sampleY := y - radius; sampleY <= y+radius; sampleY++ {
				for sampleX := x - radius; sampleX <= x+radius; sampleX++ {
					if sampleX < 0 || sampleX >= docW || sampleY < 0 || sampleY >= docH {
						minAlpha = 0
						continue
					}
					alpha := mask[sampleY*docW+sampleX]
					if alpha < minAlpha {
						minAlpha = alpha
					}
				}
			}
			out[y*docW+x] = minAlpha
		}
	}
	return out
}

func subtractAlphaMask(mask, subtract []byte) []byte {
	out := make([]byte, len(mask))
	for index := range mask {
		value := int(mask[index]) - int(alphaAt(subtract, index))
		if value < 0 {
			value = 0
		}
		out[index] = uint8(value)
	}
	return out
}

func intersectAlphaMask(a, b []byte) []byte {
	out := make([]byte, len(a))
	for index := range a {
		out[index] = scaleMaskedAlpha(a[index], alphaAt(b, index))
	}
	return out
}

func maxAlphaMask(a, b []byte) []byte {
	size := len(a)
	if len(b) > size {
		size = len(b)
	}
	out := make([]byte, size)
	for index := range out {
		left := alphaAt(a, index)
		right := alphaAt(b, index)
		if left > right {
			out[index] = left
		} else {
			out[index] = right
		}
	}
	return out
}

func absDiffAlphaMask(a, b []byte) []byte {
	size := len(a)
	if len(b) > size {
		size = len(b)
	}
	out := make([]byte, size)
	for index := range out {
		left := int(alphaAt(a, index))
		right := int(alphaAt(b, index))
		diff := left - right
		if diff < 0 {
			diff = -diff
		}
		out[index] = uint8(diff)
	}
	return out
}

func alphaAt(mask []byte, index int) uint8 {
	if index < 0 || index >= len(mask) {
		return 0
	}
	return mask[index]
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
