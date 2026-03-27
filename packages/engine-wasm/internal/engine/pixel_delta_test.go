package engine

import (
	"bytes"
	"testing"
)

func TestNewPixelDeltaCapturesOnlyDirtyRect(t *testing.T) {
	before := makeSurface(4, 4, 0x11)
	after := append([]byte(nil), before...)

	// Mutate a 2x2 block in the middle.
	writePixel(after, 4, 1, 1, [4]byte{0xaa, 0xbb, 0xcc, 0xdd})
	writePixel(after, 4, 2, 1, [4]byte{0xde, 0xad, 0xbe, 0xef})
	writePixel(after, 4, 1, 2, [4]byte{0x01, 0x02, 0x03, 0x04})
	writePixel(after, 4, 2, 2, [4]byte{0x10, 0x20, 0x30, 0x40})

	delta, err := NewPixelDelta(before, after, 4, 4, DirtyRect{X: 1, Y: 1, W: 2, H: 2})
	if err != nil {
		t.Fatalf("NewPixelDelta: %v", err)
	}

	if got, want := len(delta.Before), 2*2*4; got != want {
		t.Fatalf("len(delta.Before) = %d, want %d", got, want)
	}
	if bytes.Equal(delta.Before, delta.After) {
		t.Fatal("expected before/after dirty rect bytes to differ")
	}
}

func TestPixelDeltaApplyAndUndo(t *testing.T) {
	before := makeSurface(3, 3, 0x20)
	after := append([]byte(nil), before...)
	writePixel(after, 3, 1, 1, [4]byte{0xff, 0x00, 0x00, 0xff})

	delta, err := NewPixelDelta(before, after, 3, 3, DirtyRect{X: 1, Y: 1, W: 1, H: 1})
	if err != nil {
		t.Fatalf("NewPixelDelta: %v", err)
	}

	target := append([]byte(nil), before...)
	if err := delta.Apply(target); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !bytes.Equal(target, after) {
		t.Fatal("Apply did not patch the dirty rect to the expected after state")
	}

	if err := delta.Undo(target); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	if !bytes.Equal(target, before) {
		t.Fatal("Undo did not restore the original pixels")
	}
}

func TestPixelDeltaCommandUsesDirtyRectDiff(t *testing.T) {
	before := makeSurface(2, 2, 0x30)
	after := append([]byte(nil), before...)
	writePixel(after, 2, 0, 1, [4]byte{0x99, 0x88, 0x77, 0xff})

	delta, err := NewPixelDelta(before, after, 2, 2, DirtyRect{X: 0, Y: 1, W: 1, H: 1})
	if err != nil {
		t.Fatalf("NewPixelDelta: %v", err)
	}

	inst := &instance{pixels: append([]byte(nil), before...)}
	command := &pixelDeltaCommand{
		description: "Paint dirty rect",
		target: func(inst *instance) []byte {
			return inst.pixels
		},
		delta: delta,
	}

	if err := command.Apply(inst); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !bytes.Equal(inst.pixels, after) {
		t.Fatal("Apply did not update instance pixels")
	}

	if err := command.Undo(inst); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	if !bytes.Equal(inst.pixels, before) {
		t.Fatal("Undo did not restore instance pixels")
	}
}

func makeSurface(width, height int, value byte) []byte {
	pixels := make([]byte, width*height*4)
	for i := range pixels {
		pixels[i] = value
	}
	return pixels
}

func writePixel(pixels []byte, width, x, y int, rgba [4]byte) {
	start := (y*width + x) * 4
	copy(pixels[start:start+4], rgba[:])
}
