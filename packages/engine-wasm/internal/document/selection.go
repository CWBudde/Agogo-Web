package document

import (
	"bytes"
	"fmt"
	"strings"

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

func SelectAll(width, height int) *Selection {
	selection := NewSelection(width, height)
	for index := range selection.Mask {
		selection.Mask[index] = 255
	}
	return selection
}

func Deselect(selection, lastSelection *Selection) (*Selection, *Selection) {
	if nextLast := NormalizeSelection(CloneSelection(selection)); nextLast != nil {
		lastSelection = nextLast
	}
	return nil, lastSelection
}

func Reselect(lastSelection *Selection) (*Selection, error) {
	selection := NormalizeSelection(CloneSelection(lastSelection))
	if selection == nil {
		return nil, fmt.Errorf("no stored selection")
	}
	return selection, nil
}

func InvertSelection(selection *Selection, width, height int) *Selection {
	selection = NormalizeSelection(CloneSelection(selection))
	if selection == nil {
		return SelectAll(width, height)
	}
	for index := range selection.Mask {
		selection.Mask[index] = 255 - selection.Mask[index]
	}
	return NormalizeSelection(selection)
}

func BuildSelectionMeta(selection, lastSelection *Selection) SelectionMeta {
	meta := SelectionMeta{
		LastSelectionAvailable: NormalizeSelection(CloneSelection(lastSelection)) != nil,
	}
	selection = NormalizeSelection(CloneSelection(selection))
	if selection == nil {
		return meta
	}
	meta.Active = true
	meta.PixelCount = selection.PixelCount()
	if bounds, ok := selection.Bounds(); ok {
		meta.Bounds = &bounds
	}
	return meta
}

func BuildSavedSelectionChannelMeta(channels []SavedSelectionChannel) []SavedSelectionChannelMeta {
	if len(channels) == 0 {
		return nil
	}
	meta := make([]SavedSelectionChannelMeta, 0, len(channels))
	for _, channel := range channels {
		selection := NormalizeSelection(CloneSelection(channel.Selection))
		if selection == nil {
			continue
		}
		meta = append(meta, SavedSelectionChannelMeta{
			Name:       channel.Name,
			PixelCount: selection.PixelCount(),
		})
	}
	return meta
}

func DefaultSavedSelectionName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "Alpha 1"
	}
	return name
}

func SaveSelectionToChannel(selection *Selection, channels []SavedSelectionChannel, name string) ([]SavedSelectionChannel, error) {
	selection = NormalizeSelection(CloneSelection(selection))
	if selection == nil {
		return nil, fmt.Errorf("no active selection")
	}
	name = DefaultSavedSelectionName(name)
	saved := SavedSelectionChannel{Name: name, Selection: selection}
	next := append([]SavedSelectionChannel(nil), channels...)
	for i := range next {
		if next[i].Name == name {
			next[i] = saved
			return next, nil
		}
	}
	return append(next, saved), nil
}

func LoadSelectionFromChannel(current *Selection, channels []SavedSelectionChannel, name string, combine func(current, next *Selection) *Selection) (*Selection, error) {
	name = DefaultSavedSelectionName(name)
	for _, channel := range channels {
		if channel.Name != name {
			continue
		}
		selection := NormalizeSelection(CloneSelection(channel.Selection))
		if selection == nil {
			return nil, fmt.Errorf("saved selection %q is empty", name)
		}
		return combine(current, selection), nil
	}
	return nil, fmt.Errorf("saved selection %q not found", name)
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
