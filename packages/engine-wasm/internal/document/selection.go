package document

import (
	"bytes"

	"github.com/cwbudde/agogo-web/packages/engine-wasm/internal/model"
)

type Selection struct {
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Mask   []byte `json:"mask,omitempty"`
}

type SelectionMeta struct {
	Active                 bool       `json:"active"`
	Bounds                 *DirtyRect `json:"bounds,omitempty"`
	PixelCount             int        `json:"pixelCount"`
	LastSelectionAvailable bool       `json:"lastSelectionAvailable"`
}

type SavedSelectionChannel struct {
	Name      string     `json:"name"`
	Selection *Selection `json:"selection"`
}

type SavedSelectionChannelMeta struct {
	Name       string `json:"name"`
	PixelCount int    `json:"pixelCount"`
}

func CloneSelection(selection *Selection) *Selection {
	if selection == nil {
		return nil
	}
	cloned := *selection
	cloned.Mask = append([]byte(nil), selection.Mask...)
	return &cloned
}

func SelectionEqual(a, b *Selection) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	if a == nil {
		return true
	}
	return a.Width == b.Width && a.Height == b.Height && bytes.Equal(a.Mask, b.Mask)
}

func CloneSavedSelectionChannels(channels []SavedSelectionChannel) []SavedSelectionChannel {
	if channels == nil {
		return nil
	}
	cloned := make([]SavedSelectionChannel, len(channels))
	for i := range channels {
		cloned[i] = SavedSelectionChannel{
			Name:      channels[i].Name,
			Selection: CloneSelection(channels[i].Selection),
		}
	}
	return cloned
}

func SavedSelectionChannelsEqual(a, b []SavedSelectionChannel) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Name != b[i].Name || !SelectionEqual(a[i].Selection, b[i].Selection) {
			return false
		}
	}
	return true
}

func (selection *Selection) Bounds() (DirtyRect, bool) {
	if selection == nil || selection.Width <= 0 || selection.Height <= 0 || len(selection.Mask) < selection.Width*selection.Height {
		return DirtyRect{}, false
	}
	minX := selection.Width
	minY := selection.Height
	maxX := -1
	maxY := -1
	for y := range selection.Height {
		rowOffset := y * selection.Width
		for x := range selection.Width {
			if selection.Mask[rowOffset+x] == 0 {
				continue
			}
			if x < minX {
				minX = x
			}
			if y < minY {
				minY = y
			}
			if x > maxX {
				maxX = x
			}
			if y > maxY {
				maxY = y
			}
		}
	}
	if maxX < minX || maxY < minY {
		return DirtyRect{}, false
	}
	return DirtyRect{X: minX, Y: minY, W: maxX - minX + 1, H: maxY - minY + 1}, true
}

func (selection *Selection) PixelCount() int {
	if selection == nil {
		return 0
	}
	count := 0
	for _, alpha := range selection.Mask {
		if alpha != 0 {
			count++
		}
	}
	return count
}

func NormalizeSelection(selection *Selection) *Selection {
	if selection == nil || selection.Width <= 0 || selection.Height <= 0 {
		return nil
	}
	expectedLen := selection.Width * selection.Height
	if len(selection.Mask) < expectedLen {
		return nil
	}
	selection.Mask = selection.Mask[:expectedLen]
	for _, alpha := range selection.Mask {
		if alpha != 0 {
			return selection
		}
	}
	return nil
}

func NewSelection(width, height int) *Selection {
	if width <= 0 || height <= 0 {
		return &Selection{Width: width, Height: height}
	}
	return &Selection{Width: width, Height: height, Mask: make([]byte, width*height)}
}

func NewLayerMaskFromSelection(selection *Selection) *model.LayerMask {
	if selection == nil {
		return nil
	}
	return &model.LayerMask{
		Enabled: true,
		Width:   selection.Width,
		Height:  selection.Height,
		Data:    append([]byte(nil), selection.Mask...),
	}
}
