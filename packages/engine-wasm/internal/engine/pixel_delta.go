package engine

import "fmt"

// PixelDelta stores before/after bytes for a dirty rectangle so undo/redo can
// patch only the affected region instead of copying a full surface.
type PixelDelta struct {
	Rect   DirtyRect
	Width  int
	Height int
	Before []byte
	After  []byte
}

// PixelTarget resolves the pixel buffer a pixel-delta command mutates.
type PixelTarget func(*instance) []byte

type pixelDeltaCommand struct {
	description string
	target      PixelTarget
	delta       PixelDelta
}

func NewPixelDelta(beforePixels, afterPixels []byte, width, height int, rect DirtyRect) (PixelDelta, error) {
	normalized, err := normalizeDirtyRect(rect, width, height)
	if err != nil {
		return PixelDelta{}, err
	}

	before, err := copyDirtyRect(beforePixels, width, height, normalized)
	if err != nil {
		return PixelDelta{}, err
	}
	after, err := copyDirtyRect(afterPixels, width, height, normalized)
	if err != nil {
		return PixelDelta{}, err
	}

	return PixelDelta{
		Rect:   normalized,
		Width:  width,
		Height: height,
		Before: before,
		After:  after,
	}, nil
}

func (d PixelDelta) Apply(target []byte) error {
	return blitDirtyRect(target, d.Width, d.Height, d.Rect, d.After)
}

func (d PixelDelta) Undo(target []byte) error {
	return blitDirtyRect(target, d.Width, d.Height, d.Rect, d.Before)
}

func (c *pixelDeltaCommand) Apply(inst *instance) error {
	return c.delta.Apply(c.target(inst))
}

func (c *pixelDeltaCommand) Undo(inst *instance) error {
	return c.delta.Undo(c.target(inst))
}

func (c *pixelDeltaCommand) Description() string {
	return c.description
}

func normalizeDirtyRect(rect DirtyRect, width, height int) (DirtyRect, error) {
	if width <= 0 || height <= 0 {
		return DirtyRect{}, fmt.Errorf("invalid surface dimensions %dx%d", width, height)
	}
	if rect.W <= 0 || rect.H <= 0 {
		return DirtyRect{}, fmt.Errorf("dirty rect must be positive, got %v", rect)
	}

	x1 := clampInt(rect.X, 0, width)
	y1 := clampInt(rect.Y, 0, height)
	x2 := clampInt(rect.X+rect.W, 0, width)
	y2 := clampInt(rect.Y+rect.H, 0, height)
	if x2 <= x1 || y2 <= y1 {
		return DirtyRect{}, fmt.Errorf("dirty rect outside surface bounds: %v", rect)
	}

	return DirtyRect{X: x1, Y: y1, W: x2 - x1, H: y2 - y1}, nil
}

func copyDirtyRect(pixels []byte, width, height int, rect DirtyRect) ([]byte, error) {
	if len(pixels) != width*height*4 {
		return nil, fmt.Errorf("pixel buffer len %d does not match %dx%d RGBA surface", len(pixels), width, height)
	}

	rowStride := rect.W * 4
	out := make([]byte, rect.H*rowStride)
	for row := 0; row < rect.H; row++ {
		srcStart := ((rect.Y+row)*width + rect.X) * 4
		srcEnd := srcStart + rowStride
		copy(out[row*rowStride:(row+1)*rowStride], pixels[srcStart:srcEnd])
	}
	return out, nil
}

func blitDirtyRect(target []byte, width, height int, rect DirtyRect, data []byte) error {
	if len(target) != width*height*4 {
		return fmt.Errorf("target buffer len %d does not match %dx%d RGBA surface", len(target), width, height)
	}
	if rect.W <= 0 || rect.H <= 0 {
		return fmt.Errorf("dirty rect must be positive, got %v", rect)
	}
	if len(data) != rect.W*rect.H*4 {
		return fmt.Errorf("dirty rect payload len %d does not match rect %v", len(data), rect)
	}

	rowStride := rect.W * 4
	for row := 0; row < rect.H; row++ {
		dstStart := ((rect.Y+row)*width + rect.X) * 4
		dstEnd := dstStart + rowStride
		copy(target[dstStart:dstEnd], data[row*rowStride:(row+1)*rowStride])
	}
	return nil
}

func clampInt(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}
