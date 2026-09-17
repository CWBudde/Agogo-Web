package psdexport

import (
	"bytes"
	"testing"

	psdio "github.com/cwbudde/agogo-web/packages/engine-wasm/internal/io/psd"
	"github.com/cwbudde/agogo-web/packages/engine-wasm/internal/model"
)

const (
	testDocW = 32
	testDocH = 24
)

// gradientPixels builds a deterministic, position-dependent RGBA raster so a
// sample taken from the wrong coordinate cannot accidentally match.
func gradientPixels(bounds model.LayerBounds) []byte {
	pixels := make([]byte, bounds.W*bounds.H*4)
	for y := 0; y < bounds.H; y++ {
		for x := 0; x < bounds.W; x++ {
			index := (y*bounds.W + x) * 4
			pixels[index+0] = byte(16 + x*3)
			pixels[index+1] = byte(32 + y*5)
			pixels[index+2] = byte(200 - x - y)
			pixels[index+3] = 255
		}
	}
	return pixels
}

// croppingRenderLayer mimics the engine callback that bakes clip alpha and mask
// alpha into the raster. Any record that still matches its output proves the
// flattening path was taken.
func croppingRenderLayer(opaque model.LayerBounds) func(model.LayerNode) ([]byte, error) {
	return func(model.LayerNode) ([]byte, error) {
		surface := make([]byte, testDocW*testDocH*4)
		for y := opaque.Y; y < opaque.Y+opaque.H; y++ {
			for x := opaque.X; x < opaque.X+opaque.W; x++ {
				index := (y*testDocW + x) * 4
				surface[index+0] = 9
				surface[index+1] = 9
				surface[index+2] = 9
				surface[index+3] = 255
			}
		}
		return surface, nil
	}
}

func encodedPlanes(t *testing.T, bounds model.LayerBounds, pixels []byte) [][]byte {
	t.Helper()
	planes := psdio.RGBAToPlanes(false, pixels)
	encoded := make([][]byte, 0, len(planes))
	for _, plane := range planes {
		payload, err := psdio.EncodeChannelData(plane, bounds.W, bounds.H, false)
		if err != nil {
			t.Fatalf("EncodeChannelData: %v", err)
		}
		encoded = append(encoded, payload)
	}
	return encoded
}

func assertColorChannelsMatch(t *testing.T, record psdio.ExportLayerRecord, bounds model.LayerBounds, pixels []byte) {
	t.Helper()
	want := encodedPlanes(t, bounds, pixels)
	for index, channelID := range []int16{0, 1, 2, -1} {
		if index >= len(record.Channels) {
			t.Fatalf("channel %d missing; channels = %d", index, len(record.Channels))
		}
		got := record.Channels[index]
		if got.ID != channelID {
			t.Fatalf("channel %d id = %d, want %d", index, got.ID, channelID)
		}
		if !bytes.Equal(got.Payload, want[index]) {
			t.Fatalf("channel %d payload differs from the layer's stored pixels", channelID)
		}
	}
}

func findChannel(record psdio.ExportLayerRecord, id int16) (psdio.ExportChannel, bool) {
	for _, channel := range record.Channels {
		if channel.ID == id {
			return channel, true
		}
	}
	return psdio.ExportChannel{}, false
}

// A clipped layer keeps every pixel it owns. PSD stores the clipping flag and
// re-evaluates clipping at composite time, so baking the clip base's alpha into
// the stored raster destroys content that lives outside the base.
func TestClippedLayerKeepsFullBoundsAndPixels(t *testing.T) {
	bounds := model.LayerBounds{X: 8, Y: 6, W: 18, H: 14}
	pixels := gradientPixels(bounds)
	layer := model.NewPixelLayer("Clipped", bounds, pixels)
	layer.SetClipToBelow(true)

	records, err := buildLayerRecords(Params{
		Width:       testDocW,
		Height:      testDocH,
		Layers:      []model.LayerNode{layer},
		RenderLayer: croppingRenderLayer(model.LayerBounds{X: 8, Y: 9, W: 11, H: 11}),
	}, false)
	if err != nil {
		t.Fatalf("buildLayerRecords: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("record count = %d, want 1", len(records))
	}
	if records[0].Bounds != bounds {
		t.Fatalf("bounds = %+v, want %+v (clip base must not shrink the stored layer)", records[0].Bounds, bounds)
	}
	if !records[0].ClipToBelow {
		t.Fatalf("clipping flag was not written; the reader would never re-apply the clip")
	}
	assertColorChannelsMatch(t, records[0], bounds, pixels)
}

// A masked layer keeps every pixel it owns. The mask travels as the -2 channel,
// so attenuating the stored raster as well applies the mask twice on re-read.
func TestMaskedLayerKeepsFullBoundsAndUnattenuatedPixels(t *testing.T) {
	bounds := model.LayerBounds{X: 4, Y: 3, W: 24, H: 18}
	pixels := gradientPixels(bounds)
	layer := model.NewPixelLayer("Masked", bounds, pixels)
	mask := &model.LayerMask{Enabled: true, Width: testDocW, Height: testDocH, Data: make([]byte, testDocW*testDocH)}
	for y := 7; y < 15; y++ {
		for x := 10; x < 22; x++ {
			mask.Data[y*testDocW+x] = 255
		}
	}
	layer.SetMask(mask)

	records, err := buildLayerRecords(Params{
		Width:       testDocW,
		Height:      testDocH,
		Layers:      []model.LayerNode{layer},
		RenderLayer: croppingRenderLayer(model.LayerBounds{X: 10, Y: 7, W: 12, H: 8}),
	}, false)
	if err != nil {
		t.Fatalf("buildLayerRecords: %v", err)
	}
	if records[0].Bounds != bounds {
		t.Fatalf("bounds = %+v, want %+v (the mask must not trim the layer)", records[0].Bounds, bounds)
	}
	assertColorChannelsMatch(t, records[0], bounds, pixels)

	maskChannel, ok := findChannel(records[0], -2)
	if !ok {
		t.Fatalf("no -2 user-mask channel written; the mask would be lost entirely")
	}
	wantMask, err := psdio.EncodeChannelData(mask.Data, mask.Width, mask.Height, false)
	if err != nil {
		t.Fatalf("EncodeChannelData: %v", err)
	}
	if !bytes.Equal(maskChannel.Payload, wantMask) {
		t.Fatalf("-2 channel does not carry the mask raster")
	}
}

// Vector masks, density and feather have no PSD representation of their own.
// They used to reach the file by being baked into the pixels; once the pixels
// are left alone they must reach it through the resolved mask channel instead.
func TestResolveMaskSuppliesTheEffectiveMaskChannel(t *testing.T) {
	bounds := model.LayerBounds{X: 0, Y: 0, W: 8, H: 8}
	pixels := gradientPixels(bounds)
	layer := model.NewPixelLayer("Masked", bounds, pixels)
	raw := &model.LayerMask{Enabled: true, Width: testDocW, Height: testDocH, Data: bytes.Repeat([]byte{255}, testDocW*testDocH)}
	layer.SetMask(raw)

	effective := &model.LayerMask{Enabled: true, Width: testDocW, Height: testDocH, Data: bytes.Repeat([]byte{128}, testDocW*testDocH)}
	records, err := buildLayerRecords(Params{
		Width:       testDocW,
		Height:      testDocH,
		Layers:      []model.LayerNode{layer},
		RenderLayer: croppingRenderLayer(bounds),
		ResolveMask: func(model.LayerNode) *model.LayerMask { return effective },
	}, false)
	if err != nil {
		t.Fatalf("buildLayerRecords: %v", err)
	}
	maskChannel, ok := findChannel(records[0], -2)
	if !ok {
		t.Fatalf("no -2 user-mask channel written")
	}
	want, err := psdio.EncodeChannelData(effective.Data, effective.Width, effective.Height, false)
	if err != nil {
		t.Fatalf("EncodeChannelData: %v", err)
	}
	if !bytes.Equal(maskChannel.Payload, want) {
		t.Fatalf("-2 channel carries the raw mask, not the resolved effective mask")
	}
	if records[0].Mask == nil || records[0].Mask.Data[0] != 128 {
		t.Fatalf("mask block does not describe the resolved mask: %+v", records[0].Mask)
	}
}

// Layer styles and BlendIf still have no faithful PSD encoding, so they keep
// going through the flattening path on purpose. This pins that decision.
func TestStyledAndBlendIfLayersStillFlatten(t *testing.T) {
	opaque := model.LayerBounds{X: 2, Y: 2, W: 4, H: 4}
	bounds := model.LayerBounds{X: 0, Y: 0, W: 16, H: 16}

	styled := model.NewPixelLayer("Styled", bounds, gradientPixels(bounds))
	styled.SetStyleStack([]model.LayerStyle{{Kind: "dropShadow", Enabled: true}})

	blendIf := model.NewPixelLayer("BlendIf", bounds, gradientPixels(bounds))
	blendIf.SetBlendIf(&model.BlendIfConfig{
		Channels:  model.BlendChannelsMask{R: true, G: true, B: true},
		ThisLayer: model.BlendIfRange{Gray: model.BlendIfChannel{0, 0, 200, 200}},
	})

	for _, layer := range []model.LayerNode{styled, blendIf} {
		records, err := buildLayerRecords(Params{
			Width:       testDocW,
			Height:      testDocH,
			Layers:      []model.LayerNode{layer},
			RenderLayer: croppingRenderLayer(opaque),
		}, false)
		if err != nil {
			t.Fatalf("buildLayerRecords(%s): %v", layer.Name(), err)
		}
		if records[0].Bounds != opaque {
			t.Fatalf("%s bounds = %+v, want the flattened %+v", layer.Name(), records[0].Bounds, opaque)
		}
	}
}
