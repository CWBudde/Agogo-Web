package engine

import (
	"math"
	"testing"
)

func TestBlendIfAlpha_NilConfigPassesThrough(t *testing.T) {
	if got := blendIfAlpha([4]uint8{10, 20, 30, 255}, [4]uint8{40, 50, 60, 255}, nil); got != 1 {
		t.Errorf("blendIfAlpha(nil) = %v, want 1", got)
	}
}

func TestBlendIfAlpha_DefaultConfigPassesThrough(t *testing.T) {
	cfg := defaultBlendIfConfig()
	if got := blendIfAlpha([4]uint8{0, 0, 0, 255}, [4]uint8{255, 255, 255, 255}, cfg); got != 1 {
		t.Errorf("default blendIfAlpha = %v, want 1", got)
	}
}

func TestBlendIfAlpha_ThisLayerHardCutoffGray(t *testing.T) {
	cfg := defaultBlendIfConfig()
	// Allow only gray values in [128, 255] — hard cutoff at 128.
	cfg.ThisLayer.Gray = BlendIfChannel{128, 128, 255, 255}

	dst := [4]uint8{0, 0, 0, 255}
	mid := [4]uint8{200, 200, 200, 255}
	dark := [4]uint8{64, 64, 64, 255}

	if got := blendIfAlpha(mid, dst, cfg); got != 1 {
		t.Errorf("mid grey (luma 200) factor = %v, want 1", got)
	}
	if got := blendIfAlpha(dark, dst, cfg); got != 0 {
		t.Errorf("dark grey (luma 64) factor = %v, want 0", got)
	}
}

func TestBlendIfAlpha_SmoothTransition(t *testing.T) {
	cfg := defaultBlendIfConfig()
	// Fade in from 64..128, fully pass 128..192, fade out 192..255.
	cfg.ThisLayer.Gray = BlendIfChannel{64, 128, 192, 255}

	dst := [4]uint8{0, 0, 0, 255}

	// Luma 96 = halfway through fade-in.
	src := [4]uint8{96, 96, 96, 255}
	got := blendIfAlpha(src, dst, cfg)
	if math.Abs(got-0.5) > 0.01 {
		t.Errorf("halfway fade-in factor = %v, want ~0.5", got)
	}

	// Luma 223.5 = halfway through fade-out (midpoint of 192..255).
	src = [4]uint8{224, 224, 224, 255}
	got = blendIfAlpha(src, dst, cfg)
	want := (255.0 - 224.0) / (255.0 - 192.0)
	if math.Abs(got-want) > 0.01 {
		t.Errorf("fade-out factor = %v, want ~%v", got, want)
	}

	// Luma 160 = inside pass-through zone.
	src = [4]uint8{160, 160, 160, 255}
	got = blendIfAlpha(src, dst, cfg)
	if got != 1 {
		t.Errorf("pass-through factor = %v, want 1", got)
	}
}

func TestBlendIfAlpha_UnderlyingLayerFilters(t *testing.T) {
	cfg := defaultBlendIfConfig()
	// Only allow backdrop luma in [0, 64].
	cfg.UnderlyingLayer.Gray = BlendIfChannel{0, 0, 64, 64}

	src := [4]uint8{255, 255, 255, 255}
	darkDst := [4]uint8{30, 30, 30, 255}
	brightDst := [4]uint8{200, 200, 200, 255}

	if got := blendIfAlpha(src, darkDst, cfg); got != 1 {
		t.Errorf("dark backdrop factor = %v, want 1", got)
	}
	if got := blendIfAlpha(src, brightDst, cfg); got != 0 {
		t.Errorf("bright backdrop factor = %v, want 0", got)
	}
}

func TestBlendIfAlpha_ThisAndUnderlyingCombine(t *testing.T) {
	cfg := defaultBlendIfConfig()
	cfg.ThisLayer.Gray = BlendIfChannel{0, 128, 255, 255}     // source fades in 0..128
	cfg.UnderlyingLayer.Gray = BlendIfChannel{0, 0, 128, 255} // backdrop fades out 128..255
	src := [4]uint8{64, 64, 64, 255}                          // luma 64 -> factor 0.5
	dst := [4]uint8{192, 192, 192, 255}                       // luma 192 -> factor 0.5
	got := blendIfAlpha(src, dst, cfg)
	want := 0.5 * ((255.0 - 192.0) / (255.0 - 128.0))
	if math.Abs(got-want) > 0.01 {
		t.Errorf("combined factor = %v, want ~%v", got, want)
	}
}

func TestChannelFactor_DegenerateRanges(t *testing.T) {
	// Both hard cutoffs with no fade zone: values inside the range fully pass;
	// values strictly outside are blocked.
	if got := channelFactor(BlendIfChannel{100, 100, 200, 200}, 150); got != 1 {
		t.Errorf("pass-through = %v, want 1", got)
	}
	// At the hard boundaries in a degenerate (no-fade) range, the pixel still
	// passes — matches Photoshop: sliding the handle to 100 blocks <100, not 100.
	if got := channelFactor(BlendIfChannel{100, 100, 200, 200}, 100); got != 1 {
		t.Errorf("at hard low = %v, want 1", got)
	}
	if got := channelFactor(BlendIfChannel{100, 100, 200, 200}, 200); got != 1 {
		t.Errorf("at hard high = %v, want 1", got)
	}
	// Strictly outside the hard range is always blocked.
	if got := channelFactor(BlendIfChannel{100, 100, 200, 200}, 99); got != 0 {
		t.Errorf("just below hard low = %v, want 0", got)
	}
	if got := channelFactor(BlendIfChannel{100, 100, 200, 200}, 201); got != 0 {
		t.Errorf("just above hard high = %v, want 0", got)
	}
}

func TestApplyChannelsMask(t *testing.T) {
	cfg := defaultBlendIfConfig()
	cfg.Channels.R = false
	original := [4]uint8{10, 20, 30, 255}
	dst := [4]uint8{100, 110, 120, 255}
	applyChannelsMask(&original, &dst, cfg)
	if dst[0] != original[0] {
		t.Errorf("R not restored: dst[0] = %d, want %d", dst[0], original[0])
	}
	if dst[1] != 110 || dst[2] != 120 {
		t.Errorf("G/B changed unexpectedly: %v", dst)
	}
}

func TestBlendIfIsIdentity(t *testing.T) {
	if !blendIfIsIdentity(nil) {
		t.Error("nil cfg should be identity")
	}
	if !blendIfIsIdentity(defaultBlendIfConfig()) {
		t.Error("default cfg should be identity")
	}
	cfg := defaultBlendIfConfig()
	cfg.Channels.G = false
	if blendIfIsIdentity(cfg) {
		t.Error("masked channel should not be identity")
	}
	cfg = defaultBlendIfConfig()
	cfg.ThisLayer.Gray = BlendIfChannel{10, 10, 200, 200}
	if blendIfIsIdentity(cfg) {
		t.Error("constrained range should not be identity")
	}
}
