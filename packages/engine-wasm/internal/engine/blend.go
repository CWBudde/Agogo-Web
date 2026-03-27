package engine

import "math"

type rgbaColor struct {
	r float64
	g float64
	b float64
	a float64
}

func compositePixelWithBlend(dest, src []byte, blendMode BlendMode, opacity float64, noiseSeed uint32) {
	if len(dest) < 4 || len(src) < 4 {
		return
	}

	destColor := rgbaFromPixel(dest)
	srcColor := rgbaFromPixel(src)
	srcColor.a *= clampUnit(opacity)
	if srcColor.a <= 0 {
		return
	}

	if blendMode == BlendModeDissolve {
		if dissolveEnabled(srcColor.a, noiseSeed) {
			srcColor.a = 1
		} else {
			srcColor.a = 0
		}
	}
	if srcColor.a <= 0 {
		return
	}

	blended := blendRGB(destColor, srcColor, blendMode)
	outAlpha := srcColor.a + destColor.a*(1-srcColor.a)
	if outAlpha <= 0 {
		for index := 0; index < 4; index++ {
			dest[index] = 0
		}
		return
	}

	outRed := ((1-destColor.a)*srcColor.r*srcColor.a + (1-srcColor.a)*destColor.r*destColor.a + srcColor.a*destColor.a*blended.r) / outAlpha
	outGreen := ((1-destColor.a)*srcColor.g*srcColor.a + (1-srcColor.a)*destColor.g*destColor.a + srcColor.a*destColor.a*blended.g) / outAlpha
	outBlue := ((1-destColor.a)*srcColor.b*srcColor.a + (1-srcColor.a)*destColor.b*destColor.a + srcColor.a*destColor.a*blended.b) / outAlpha

	writeColor(dest, rgbaColor{
		r: clampUnit(outRed),
		g: clampUnit(outGreen),
		b: clampUnit(outBlue),
		a: clampUnit(outAlpha),
	})
}

func rgbaFromPixel(pixel []byte) rgbaColor {
	return rgbaColor{
		r: float64(pixel[0]) / 255,
		g: float64(pixel[1]) / 255,
		b: float64(pixel[2]) / 255,
		a: float64(pixel[3]) / 255,
	}
}

func writeColor(pixel []byte, color rgbaColor) {
	pixel[0] = uint8(math.Round(clampUnit(color.r) * 255))
	pixel[1] = uint8(math.Round(clampUnit(color.g) * 255))
	pixel[2] = uint8(math.Round(clampUnit(color.b) * 255))
	pixel[3] = uint8(math.Round(clampUnit(color.a) * 255))
}

func blendRGB(backdrop, source rgbaColor, mode BlendMode) rgbaColor {
	backdropRGB := [3]float64{backdrop.r, backdrop.g, backdrop.b}
	sourceRGB := [3]float64{source.r, source.g, source.b}
	result := sourceRGB

	switch mode {
	case BlendModeNormal, BlendModeDissolve:
		result = sourceRGB
	case BlendModeMultiply:
		result = [3]float64{backdrop.r * source.r, backdrop.g * source.g, backdrop.b * source.b}
	case BlendModeColorBurn:
		result = [3]float64{blendColorBurn(backdrop.r, source.r), blendColorBurn(backdrop.g, source.g), blendColorBurn(backdrop.b, source.b)}
	case BlendModeLinearBurn:
		result = [3]float64{clampUnit(backdrop.r + source.r - 1), clampUnit(backdrop.g + source.g - 1), clampUnit(backdrop.b + source.b - 1)}
	case BlendModeDarken:
		result = [3]float64{math.Min(backdrop.r, source.r), math.Min(backdrop.g, source.g), math.Min(backdrop.b, source.b)}
	case BlendModeDarkerColor:
		if colorLuminance(sourceRGB) < colorLuminance(backdropRGB) {
			result = sourceRGB
		} else {
			result = backdropRGB
		}
	case BlendModeScreen:
		result = [3]float64{blendScreen(backdrop.r, source.r), blendScreen(backdrop.g, source.g), blendScreen(backdrop.b, source.b)}
	case BlendModeColorDodge:
		result = [3]float64{blendColorDodge(backdrop.r, source.r), blendColorDodge(backdrop.g, source.g), blendColorDodge(backdrop.b, source.b)}
	case BlendModeLinearDodge:
		result = [3]float64{clampUnit(backdrop.r + source.r), clampUnit(backdrop.g + source.g), clampUnit(backdrop.b + source.b)}
	case BlendModeLighten:
		result = [3]float64{math.Max(backdrop.r, source.r), math.Max(backdrop.g, source.g), math.Max(backdrop.b, source.b)}
	case BlendModeLighterColor:
		if colorLuminance(sourceRGB) > colorLuminance(backdropRGB) {
			result = sourceRGB
		} else {
			result = backdropRGB
		}
	case BlendModeOverlay:
		result = [3]float64{blendOverlay(backdrop.r, source.r), blendOverlay(backdrop.g, source.g), blendOverlay(backdrop.b, source.b)}
	case BlendModeSoftLight:
		result = [3]float64{blendSoftLight(backdrop.r, source.r), blendSoftLight(backdrop.g, source.g), blendSoftLight(backdrop.b, source.b)}
	case BlendModeHardLight:
		result = [3]float64{blendOverlay(source.r, backdrop.r), blendOverlay(source.g, backdrop.g), blendOverlay(source.b, backdrop.b)}
	case BlendModeVividLight:
		result = [3]float64{blendVividLight(backdrop.r, source.r), blendVividLight(backdrop.g, source.g), blendVividLight(backdrop.b, source.b)}
	case BlendModeLinearLight:
		result = [3]float64{clampUnit(backdrop.r + 2*source.r - 1), clampUnit(backdrop.g + 2*source.g - 1), clampUnit(backdrop.b + 2*source.b - 1)}
	case BlendModePinLight:
		result = [3]float64{blendPinLight(backdrop.r, source.r), blendPinLight(backdrop.g, source.g), blendPinLight(backdrop.b, source.b)}
	case BlendModeHardMix:
		result = [3]float64{blendHardMix(backdrop.r, source.r), blendHardMix(backdrop.g, source.g), blendHardMix(backdrop.b, source.b)}
	case BlendModeDifference:
		result = [3]float64{math.Abs(backdrop.r - source.r), math.Abs(backdrop.g - source.g), math.Abs(backdrop.b - source.b)}
	case BlendModeExclusion:
		result = [3]float64{blendExclusion(backdrop.r, source.r), blendExclusion(backdrop.g, source.g), blendExclusion(backdrop.b, source.b)}
	case BlendModeSubtract:
		result = [3]float64{clampUnit(backdrop.r - source.r), clampUnit(backdrop.g - source.g), clampUnit(backdrop.b - source.b)}
	case BlendModeDivide:
		result = [3]float64{blendDivide(backdrop.r, source.r), blendDivide(backdrop.g, source.g), blendDivide(backdrop.b, source.b)}
	case BlendModeHue:
		result = setLuminosity(setSaturation(sourceRGB, saturation(backdropRGB)), luminosity(backdropRGB))
	case BlendModeSaturation:
		result = setLuminosity(setSaturation(backdropRGB, saturation(sourceRGB)), luminosity(backdropRGB))
	case BlendModeColor:
		result = setLuminosity(sourceRGB, luminosity(backdropRGB))
	case BlendModeLuminosity:
		result = setLuminosity(backdropRGB, luminosity(sourceRGB))
	}

	return rgbaColor{r: result[0], g: result[1], b: result[2], a: source.a}
}

func blendScreen(backdrop, source float64) float64 {
	return backdrop + source - backdrop*source
}

func blendOverlay(backdrop, source float64) float64 {
	if backdrop <= 0.5 {
		return 2 * backdrop * source
	}
	return 1 - 2*(1-backdrop)*(1-source)
}

func blendSoftLight(backdrop, source float64) float64 {
	if source <= 0.5 {
		return backdrop - (1-2*source)*backdrop*(1-backdrop)
	}
	var d float64
	if backdrop <= 0.25 {
		d = ((16*backdrop-12)*backdrop + 4) * backdrop
	} else {
		d = math.Sqrt(backdrop)
	}
	return backdrop + (2*source-1)*(d-backdrop)
}

func blendColorDodge(backdrop, source float64) float64 {
	if source >= 1 {
		return 1
	}
	return clampUnit(backdrop / (1 - source))
}

func blendColorBurn(backdrop, source float64) float64 {
	if source <= 0 {
		return 0
	}
	return 1 - clampUnit((1-backdrop)/source)
}

func blendVividLight(backdrop, source float64) float64 {
	if source < 0.5 {
		return blendColorBurn(backdrop, 2*source)
	}
	return blendColorDodge(backdrop, 2*source-1)
}

func blendPinLight(backdrop, source float64) float64 {
	if source < 0.5 {
		return math.Min(backdrop, 2*source)
	}
	return math.Max(backdrop, 2*source-1)
}

func blendHardMix(backdrop, source float64) float64 {
	if blendVividLight(backdrop, source) < 0.5 {
		return 0
	}
	return 1
}

func blendExclusion(backdrop, source float64) float64 {
	return backdrop + source - 2*backdrop*source
}

func blendDivide(backdrop, source float64) float64 {
	if source <= 0 {
		return 1
	}
	return clampUnit(backdrop / source)
}

func dissolveEnabled(alpha float64, seed uint32) bool {
	value := float64(seed%10000) / 10000
	return value < alpha
}

func colorLuminance(color [3]float64) float64 {
	return 0.3*color[0] + 0.59*color[1] + 0.11*color[2]
}

func luminosity(color [3]float64) float64 {
	return colorLuminance(color)
}

func saturation(color [3]float64) float64 {
	maxComponent := math.Max(color[0], math.Max(color[1], color[2]))
	minComponent := math.Min(color[0], math.Min(color[1], color[2]))
	return maxComponent - minComponent
}

func setLuminosity(color [3]float64, target float64) [3]float64 {
	delta := target - luminosity(color)
	adjusted := [3]float64{color[0] + delta, color[1] + delta, color[2] + delta}
	return clipColor(adjusted)
}

func clipColor(color [3]float64) [3]float64 {
	minComponent := math.Min(color[0], math.Min(color[1], color[2]))
	maxComponent := math.Max(color[0], math.Max(color[1], color[2]))
	lum := luminosity(color)
	if minComponent < 0 {
		for index := range color {
			color[index] = lum + ((color[index]-lum)*lum)/(lum-minComponent)
		}
	}
	if maxComponent > 1 {
		for index := range color {
			color[index] = lum + ((color[index]-lum)*(1-lum))/(maxComponent-lum)
		}
	}
	for index := range color {
		color[index] = clampUnit(color[index])
	}
	return color
}

func setSaturation(color [3]float64, target float64) [3]float64 {
	values := []float64{color[0], color[1], color[2]}
	indices := []int{0, 1, 2}
	for i := 0; i < len(indices); i++ {
		for j := i + 1; j < len(indices); j++ {
			if values[indices[i]] > values[indices[j]] {
				indices[i], indices[j] = indices[j], indices[i]
			}
		}
	}

	minIndex := indices[0]
	midIndex := indices[1]
	maxIndex := indices[2]
	if values[maxIndex] > values[minIndex] {
		values[midIndex] = ((values[midIndex] - values[minIndex]) * target) / (values[maxIndex] - values[minIndex])
		values[maxIndex] = target
	} else {
		values[midIndex] = 0
		values[maxIndex] = 0
	}
	values[minIndex] = 0
	return [3]float64{clampUnit(values[0]), clampUnit(values[1]), clampUnit(values[2])}
}
