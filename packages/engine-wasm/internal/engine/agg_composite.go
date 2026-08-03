package engine

import agg "github.com/cwbudde/agg_go"

// aggBlendMode maps the document model's stable string ABI to agg_go's
// compositor. Color Burn intentionally selects the added Photoshop-correct
// mode; agg.BlendColorBurn remains available for callers that depend on the
// historical AGG formula.
func aggBlendMode(mode BlendMode) agg.BlendMode {
	switch mode {
	case "", BlendModeNormal:
		return agg.BlendSrcOver
	case BlendModeDissolve:
		return agg.BlendDissolve
	case BlendModeMultiply:
		return agg.BlendMultiply
	case BlendModeColorBurn:
		return agg.BlendColorBurnPhotoshop
	case BlendModeLinearBurn:
		return agg.BlendLinearBurn
	case BlendModeDarken:
		return agg.BlendDarken
	case BlendModeDarkerColor:
		return agg.BlendDarkerColor
	case BlendModeScreen:
		return agg.BlendScreen
	case BlendModeColorDodge:
		return agg.BlendColorDodge
	case BlendModeLinearDodge:
		return agg.BlendLinearDodge
	case BlendModeLighten:
		return agg.BlendLighten
	case BlendModeLighterColor:
		return agg.BlendLighterColor
	case BlendModeOverlay:
		return agg.BlendOverlay
	case BlendModeSoftLight:
		return agg.BlendSoftLightPhotoshop
	case BlendModeHardLight:
		return agg.BlendHardLight
	case BlendModeVividLight:
		return agg.BlendVividLight
	case BlendModeLinearLight:
		return agg.BlendLinearLight
	case BlendModePinLight:
		return agg.BlendPinLight
	case BlendModeHardMix:
		return agg.BlendHardMix
	case BlendModeDifference:
		return agg.BlendDifference
	case BlendModeExclusion:
		return agg.BlendExclusion
	case BlendModeSubtract:
		return agg.BlendSubtract
	case BlendModeDivide:
		return agg.BlendDivide
	case BlendModeHue:
		return agg.BlendHue
	case BlendModeSaturation:
		return agg.BlendSaturation
	case BlendModeColor:
		return agg.BlendColor
	case BlendModeLuminosity:
		return agg.BlendLuminosity
	default:
		// The legacy engine compositor treated unknown string values as Normal.
		// Preserve that tolerant command-input behavior while model setters keep
		// normalizing stored layer modes at their validation boundary.
		return agg.BlendSrcOver
	}
}

func aggRectFromDirty(rect *DirtyRect) *agg.Rect {
	if rect == nil {
		return nil
	}
	converted := agg.Rect{X1: rect.X, Y1: rect.Y, X2: rect.X + rect.W, Y2: rect.Y + rect.H}
	return &converted
}

func compositeImageStraight(
	dest []byte,
	destW, destH int,
	src []byte,
	srcW, srcH int,
	srcRect agg.Rect,
	destOrigin agg.PointI,
	mode BlendMode,
	opacity float64,
	mask *agg.AlphaMask,
	maskOrigin agg.PointI,
	clip *agg.Rect,
	seed agg.DissolveSeedFunc,
) error {
	return agg.CompositeImage(
		agg.NewImage(dest, destW, destH, destW*4),
		agg.NewImage(src, srcW, srcH, srcW*4),
		srcRect,
		destOrigin,
		agg.CompositeOptions{
			BlendMode:    aggBlendMode(mode),
			Opacity:      clampUnit(opacity),
			AlphaMode:    agg.AlphaStraight,
			Mask:         mask,
			MaskOrigin:   maskOrigin,
			Clip:         clip,
			DissolveSeed: seed,
		},
	)
}

func engineDissolveSeed(x, y int) uint32 {
	return pixelNoiseSeed(x, y)
}

func offsetDissolveSeed(originX, originY int) agg.DissolveSeedFunc {
	return func(x, y int) uint32 {
		return pixelNoiseSeed(originX+x, originY+y)
	}
}

func buildRasterCompositeMask(docW, docH int, bounds LayerBounds, src, dest []byte, layerMask *LayerMask, clipAlpha []byte, blendIf *BlendIfConfig) *agg.AlphaMask {
	useLayerMask := layerMask != nil && layerMask.Enabled
	useClipAlpha := len(clipAlpha) != 0
	useBlendIf := !blendIfIsIdentity(blendIf)
	if !useLayerMask && !useClipAlpha && !useBlendIf {
		return nil
	}

	mask := agg.AlphaMask{Width: bounds.W, Height: bounds.H, Pix: acquireSurface(bounds.W * bounds.H)}
	for y := 0; y < bounds.H; y++ {
		docY := bounds.Y + y
		for x := 0; x < bounds.W; x++ {
			docX := bounds.X + x
			coverage := uint8(255)
			if docX < 0 || docX >= docW || docY < 0 || docY >= docH {
				coverage = 0
			} else {
				if useLayerMask {
					coverage = scaleMaskedAlpha(coverage, layerMaskAlphaAt(layerMask, docX, docY))
				}
				if useClipAlpha {
					coverage = scaleMaskedAlpha(coverage, clipSurfaceAlphaAt(clipAlpha, docW, docX, docY))
				}
				if useBlendIf && coverage != 0 {
					srcOffset := (y*bounds.W + x) * 4
					destOffset := (docY*docW + docX) * 4
					srcRGBA := [4]uint8{src[srcOffset], src[srcOffset+1], src[srcOffset+2], src[srcOffset+3]}
					destRGBA := [4]uint8{dest[destOffset], dest[destOffset+1], dest[destOffset+2], dest[destOffset+3]}
					blendIfCoverage := uint8(clampUnit(blendIfAlpha(srcRGBA, destRGBA, blendIf))*255 + 0.5)
					coverage = scaleMaskedAlpha(coverage, blendIfCoverage)
				}
			}
			mask.Pix[y*bounds.W+x] = coverage
		}
	}
	return &mask
}

func buildDocumentCompositeMask(docW, docH int, src, dest []byte, blendIf *BlendIfConfig) *agg.AlphaMask {
	if blendIfIsIdentity(blendIf) {
		return nil
	}
	mask := agg.AlphaMask{Width: docW, Height: docH, Pix: acquireSurface(docW * docH)}
	for pixelIndex := 0; pixelIndex < docW*docH; pixelIndex++ {
		offset := pixelIndex * 4
		srcRGBA := [4]uint8{src[offset], src[offset+1], src[offset+2], src[offset+3]}
		destRGBA := [4]uint8{dest[offset], dest[offset+1], dest[offset+2], dest[offset+3]}
		mask.Pix[pixelIndex] = uint8(clampUnit(blendIfAlpha(srcRGBA, destRGBA, blendIf))*255 + 0.5)
	}
	return &mask
}

func releaseCompositeMask(mask *agg.AlphaMask) {
	if mask != nil {
		releaseSurface(mask.Pix)
	}
}

func applyBlendIfChannelsClipped(dest, original []byte, docW, docH int, bounds LayerBounds, blendIf *BlendIfConfig, clip *DirtyRect) {
	xStart := maxInt(0, bounds.X)
	yStart := maxInt(0, bounds.Y)
	xEnd := minInt(docW, bounds.X+bounds.W)
	yEnd := minInt(docH, bounds.Y+bounds.H)
	if clip != nil {
		xStart = maxInt(xStart, clip.X)
		yStart = maxInt(yStart, clip.Y)
		xEnd = minInt(xEnd, clip.X+clip.W)
		yEnd = minInt(yEnd, clip.Y+clip.H)
	}
	for y := yStart; y < yEnd; y++ {
		for x := xStart; x < xEnd; x++ {
			offset := (y*docW + x) * 4
			before := [4]uint8{original[offset], original[offset+1], original[offset+2], original[offset+3]}
			after := [4]uint8{dest[offset], dest[offset+1], dest[offset+2], dest[offset+3]}
			applyChannelsMask(&before, &after, blendIf)
			copy(dest[offset:offset+4], after[:])
		}
	}
}
