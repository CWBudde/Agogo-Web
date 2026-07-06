package engine

import (
	"fmt"
	"math"
	"sync"

	"github.com/cwbudde/agogo-web/packages/engine-wasm/internal/model"
)

// PatternMeta is the UIMeta projection of a pattern resource (no tile bytes).
type PatternMeta struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// maxPatternTileSize caps DefinePattern capture per dimension.
const maxPatternTileSize = 1024

var (
	builtinPatternsOnce sync.Once
	builtinPatterns     []model.PatternResource
)

// builtinPatternList returns the procedural builtin tiles, generated exactly
// once. All builtins use fixed colors and deterministic generation so their
// bytes are stable across sessions and platforms.
func builtinPatternList() []model.PatternResource {
	builtinPatternsOnce.Do(func() {
		builtinPatterns = []model.PatternResource{
			makeBuiltinChecker(),
			makeBuiltinStripes(),
			makeBuiltinDots(),
			makeBuiltinNoise(),
		}
	})
	return builtinPatterns
}

func writePatternPixel(data []byte, width, x, y int, c [4]uint8) {
	i := (y*width + x) * 4
	data[i] = c[0]
	data[i+1] = c[1]
	data[i+2] = c[2]
	data[i+3] = c[3]
}

// makeBuiltinChecker builds an 8px two-tone gray checker (4px cells).
func makeBuiltinChecker() model.PatternResource {
	const size, cell = 8, 4
	light := [4]uint8{200, 200, 200, 255}
	dark := [4]uint8{100, 100, 100, 255}
	data := make([]byte, size*size*4)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			c := light
			if ((x/cell)+(y/cell))%2 == 1 {
				c = dark
			}
			writePatternPixel(data, size, x, y, c)
		}
	}
	return model.PatternResource{ID: "builtin/checker", Name: "Checker", Width: size, Height: size, Data: data}
}

// makeBuiltinStripes builds an 8px diagonal stripe tile.
func makeBuiltinStripes() model.PatternResource {
	const size = 8
	light := [4]uint8{210, 210, 210, 255}
	dark := [4]uint8{90, 90, 90, 255}
	data := make([]byte, size*size*4)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			c := light
			if (x+y)%size < size/2 {
				c = dark
			}
			writePatternPixel(data, size, x, y, c)
		}
	}
	return model.PatternResource{ID: "builtin/stripes", Name: "Diagonal Stripes", Width: size, Height: size, Data: data}
}

// makeBuiltinDots builds an 8px tile with a dark dot on a light background.
func makeBuiltinDots() model.PatternResource {
	const size = 8
	background := [4]uint8{230, 230, 230, 255}
	dot := [4]uint8{60, 60, 60, 255}
	const centerX, centerY, radius = 3.5, 3.5, 2.5
	data := make([]byte, size*size*4)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx := float64(x) - centerX
			dy := float64(y) - centerY
			c := background
			if dx*dx+dy*dy <= radius*radius {
				c = dot
			}
			writePatternPixel(data, size, x, y, c)
		}
	}
	return model.PatternResource{ID: "builtin/dots", Name: "Dots", Width: size, Height: size, Data: data}
}

// makeBuiltinNoise builds a 16px grayscale noise tile seeded by pixelNoiseSeed
// (the splitmix64 finalizer used by dissolve blending) — deterministic.
func makeBuiltinNoise() model.PatternResource {
	const size = 16
	data := make([]byte, size*size*4)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			v := uint8(pixelNoiseSeed(x, y) >> 24)
			writePatternPixel(data, size, x, y, [4]uint8{v, v, v, 255})
		}
	}
	return model.PatternResource{ID: "builtin/noise", Name: "Noise", Width: size, Height: size, Data: data}
}

// resolvePattern looks a pattern up by ID: document-defined patterns take
// precedence over builtins; unknown or empty IDs resolve to nil.
func resolvePattern(doc *Document, id string) *model.PatternResource {
	if id == "" {
		return nil
	}
	if doc != nil {
		for i := range doc.Patterns {
			if doc.Patterns[i].ID == id {
				return &doc.Patterns[i]
			}
		}
	}
	builtins := builtinPatternList()
	for i := range builtins {
		if builtins[i].ID == id {
			return &builtins[i]
		}
	}
	return nil
}

// floorModInt returns the floored (always non-negative for positive modulus)
// remainder of v/m, so negative document coordinates wrap correctly.
func floorModInt(v, m int) int {
	r := v % m
	if r < 0 {
		r += m
	}
	return r
}

// samplePatternColor samples the tile at a document coordinate using nearest
// neighbor. scale > 1 magnifies the tile, scale < 1 shrinks it; scale <= 0 is
// treated as 1. Bilinear sampling via agg span accessors is a documented
// follow-up.
func samplePatternColor(p *model.PatternResource, docX, docY int, scale float64) [4]uint8 {
	if p == nil || p.Width <= 0 || p.Height <= 0 || len(p.Data) < p.Width*p.Height*4 {
		return [4]uint8{}
	}
	if scale <= 0 {
		scale = 1
	}
	tx := floorModInt(int(math.Floor(float64(docX)/scale)), p.Width)
	ty := floorModInt(int(math.Floor(float64(docY)/scale)), p.Height)
	i := (ty*p.Width + tx) * 4
	return [4]uint8{p.Data[i], p.Data[i+1], p.Data[i+2], p.Data[i+3]}
}

// buildPatternsMeta lists builtins followed by document-defined patterns for
// UIMeta consumption.
func buildPatternsMeta(doc *Document) []PatternMeta {
	builtins := builtinPatternList()
	var docPatterns []model.PatternResource
	if doc != nil {
		docPatterns = doc.Patterns
	}
	meta := make([]PatternMeta, 0, len(builtins)+len(docPatterns))
	for _, p := range builtins {
		meta = append(meta, PatternMeta{ID: p.ID, Name: p.Name, Width: p.Width, Height: p.Height})
	}
	for _, p := range docPatterns {
		meta = append(meta, PatternMeta{ID: p.ID, Name: p.Name, Width: p.Width, Height: p.Height})
	}
	return meta
}

// handleDefinePattern captures a tile from the active layer (intersected with
// the selection bounding rect when a selection exists) and appends it to the
// document's pattern list as an undoable snapshot command.
func (inst *instance) handleDefinePattern(name string) error {
	command := newSnapshotCommand("Define Pattern", func(inst *instance) (snapshot, error) {
		doc := inst.manager.Active()
		if doc == nil {
			return snapshot{}, fmt.Errorf("no active document")
		}
		pattern, err := capturePatternFromDocument(doc, name)
		if err != nil {
			return snapshot{}, err
		}
		doc.Patterns = append(doc.Patterns, pattern)
		doc.touchModifiedAt()
		if err := inst.manager.ReplaceActive(doc); err != nil {
			return snapshot{}, err
		}
		return inst.captureSnapshot(), nil
	})
	return inst.history.Execute(inst, command)
}

// handleDeletePattern removes a document-defined pattern by ID as an undoable
// snapshot command. Builtins cannot be deleted.
func (inst *instance) handleDeletePattern(patternID string) error {
	command := newSnapshotCommand("Delete Pattern", func(inst *instance) (snapshot, error) {
		doc := inst.manager.Active()
		if doc == nil {
			return snapshot{}, fmt.Errorf("no active document")
		}
		index := -1
		for i := range doc.Patterns {
			if doc.Patterns[i].ID == patternID {
				index = i
				break
			}
		}
		if index < 0 {
			return snapshot{}, fmt.Errorf("pattern not found: %s", patternID)
		}
		doc.Patterns = append(doc.Patterns[:index:index], doc.Patterns[index+1:]...)
		doc.touchModifiedAt()
		if err := inst.manager.ReplaceActive(doc); err != nil {
			return snapshot{}, err
		}
		return inst.captureSnapshot(), nil
	})
	return inst.history.Execute(inst, command)
}

// capturePatternFromDocument extracts the pattern tile bytes for DefinePattern.
func capturePatternFromDocument(doc *Document, name string) (model.PatternResource, error) {
	layer := doc.ActiveLayer()
	if layer == nil {
		return model.PatternResource{}, fmt.Errorf("no rasterizable active layer")
	}
	bounds, raster, err := rasterizableLayerSource(layer)
	if err != nil || bounds.W <= 0 || bounds.H <= 0 || len(raster) != bounds.W*bounds.H*4 {
		return model.PatternResource{}, fmt.Errorf("no rasterizable active layer")
	}

	rect := bounds
	if sel := normalizeSelection(cloneSelection(doc.Selection)); sel != nil {
		selRect, ok := selectionBounds(sel)
		if !ok {
			return model.PatternResource{}, fmt.Errorf("selection has no bounds")
		}
		rect = intersectPatternBounds(selRect, bounds)
		if rect.W <= 0 || rect.H <= 0 {
			return model.PatternResource{}, fmt.Errorf("selection does not overlap the active layer")
		}
	}
	if rect.W > maxPatternTileSize || rect.H > maxPatternTileSize {
		return model.PatternResource{}, fmt.Errorf("pattern tile %dx%d exceeds the %dx%d limit", rect.W, rect.H, maxPatternTileSize, maxPatternTileSize)
	}

	data := make([]byte, rect.W*rect.H*4)
	for y := 0; y < rect.H; y++ {
		srcY := rect.Y - bounds.Y + y
		srcStart := (srcY*bounds.W + (rect.X - bounds.X)) * 4
		copy(data[y*rect.W*4:(y+1)*rect.W*4], raster[srcStart:srcStart+rect.W*4])
	}

	if name == "" {
		name = fmt.Sprintf("Pattern %d", len(doc.Patterns)+1)
	}
	return model.PatternResource{
		ID:     model.NewPatternID(),
		Name:   name,
		Width:  rect.W,
		Height: rect.H,
		Data:   data,
	}, nil
}

func intersectPatternBounds(a, b LayerBounds) LayerBounds {
	x1 := maxInt(a.X, b.X)
	y1 := maxInt(a.Y, b.Y)
	x2 := a.X + a.W
	if b.X+b.W < x2 {
		x2 = b.X + b.W
	}
	y2 := a.Y + a.H
	if b.Y+b.H < y2 {
		y2 = b.Y + b.H
	}
	return LayerBounds{X: x1, Y: y1, W: x2 - x1, H: y2 - y1}
}
