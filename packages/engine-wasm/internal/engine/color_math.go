package engine

import "math"

// colorLuminance returns the Rec. 601 luma used by the editor's adjustment
// commands and Photoshop-compatible colour operations.
func colorLuminance(color [3]float64) float64 {
	return 0.3*color[0] + 0.59*color[1] + 0.11*color[2]
}

func luminosity(color [3]float64) float64 {
	return colorLuminance(color)
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
		if denom := lum - minComponent; denom > 0 {
			for index := range color {
				color[index] = lum + ((color[index]-lum)*lum)/denom
			}
		} else {
			for index := range color {
				color[index] = lum
			}
		}
	}
	if maxComponent > 1 {
		if denom := maxComponent - lum; denom > 0 {
			for index := range color {
				color[index] = lum + ((color[index]-lum)*(1-lum))/denom
			}
		} else {
			for index := range color {
				color[index] = lum
			}
		}
	}
	for index := range color {
		color[index] = clampUnit(color[index])
	}
	return color
}
