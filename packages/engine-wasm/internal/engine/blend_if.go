package engine

// blendIfAlpha computes the 0..1 opacity multiplier produced by a Blend If
// configuration for a given source pixel being composited on top of a
// destination (backdrop) pixel.
//
// The algorithm follows Photoshop:
//   - For each channel (gray uses Rec. 601 luma), the channel value is
//     evaluated against [lowHard, lowSoft, highSoft, highHard]:
//   - below lowHard or above highHard: factor = 0 (pixel suppressed)
//   - between lowHard and lowSoft: factor fades 0 -> 1
//   - between lowSoft and highSoft: factor = 1 (pass through)
//   - between highSoft and highHard: factor fades 1 -> 0
//   - The per-channel factor is computed for the source (This Layer) and the
//     destination (Underlying Layer), then multiplied together.
//   - The channel factors within a single layer are multiplied so all four
//     channels (gray, R, G, B) must pass for the pixel to be visible, which
//     matches Photoshop's behaviour where each enabled slider constrains the
//     output further.
//
// Returns 1.0 when cfg is nil.
func blendIfAlpha(src, dst [4]uint8, cfg *BlendIfConfig) float64 {
	if cfg == nil {
		return 1
	}
	thisFactor := rangeFactor(cfg.ThisLayer, src)
	if thisFactor == 0 {
		return 0
	}
	underFactor := rangeFactor(cfg.UnderlyingLayer, dst)
	return thisFactor * underFactor
}

// applyChannelsMask restores any destination channel that is masked out by
// the Blend If configuration. It mutates dst in place.
func applyChannelsMask(original, dst *[4]uint8, cfg *BlendIfConfig) {
	if cfg == nil {
		return
	}
	if !cfg.Channels.R {
		dst[0] = original[0]
	}
	if !cfg.Channels.G {
		dst[1] = original[1]
	}
	if !cfg.Channels.B {
		dst[2] = original[2]
	}
}

func rangeFactor(r BlendIfRange, pixel [4]uint8) float64 {
	luma := 0.299*float64(pixel[0]) + 0.587*float64(pixel[1]) + 0.114*float64(pixel[2])
	factor := channelFactor(r.Gray, luma)
	if factor == 0 {
		return 0
	}
	factor *= channelFactor(r.Red, float64(pixel[0]))
	if factor == 0 {
		return 0
	}
	factor *= channelFactor(r.Green, float64(pixel[1]))
	if factor == 0 {
		return 0
	}
	factor *= channelFactor(r.Blue, float64(pixel[2]))
	return factor
}

func channelFactor(c BlendIfChannel, value float64) float64 {
	lowHard, lowSoft, highSoft, highHard := c[0], c[1], c[2], c[3]
	if value < lowHard || value > highHard {
		return 0
	}
	if value >= lowSoft && value <= highSoft {
		return 1
	}
	if value < lowSoft {
		width := lowSoft - lowHard
		if width <= 0 {
			// Hard cutoff: value == lowHard == lowSoft treated as pass.
			return 1
		}
		return (value - lowHard) / width
	}
	width := highHard - highSoft
	if width <= 0 {
		return 1
	}
	return (highHard - value) / width
}

// blendIfIsIdentity reports whether the config lets every pixel pass with
// its channels intact. Useful for skipping the per-pixel filter on hot paths.
func blendIfIsIdentity(cfg *BlendIfConfig) bool {
	if cfg == nil {
		return true
	}
	if !cfg.Channels.R || !cfg.Channels.G || !cfg.Channels.B {
		return false
	}
	return rangeIsIdentity(cfg.ThisLayer) && rangeIsIdentity(cfg.UnderlyingLayer)
}

func rangeIsIdentity(r BlendIfRange) bool {
	return channelIsIdentity(r.Gray) &&
		channelIsIdentity(r.Red) &&
		channelIsIdentity(r.Green) &&
		channelIsIdentity(r.Blue)
}

func channelIsIdentity(c BlendIfChannel) bool {
	return c[0] == 0 && c[1] == 0 && c[2] == 255 && c[3] == 255
}
