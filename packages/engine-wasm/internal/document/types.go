package document

import "github.com/cwbudde/agogo-web/packages/engine-wasm/internal/model"

type Background struct {
	Kind  string   `json:"kind"`
	Color [4]uint8 `json:"color,omitempty"`
}

type DirtyRect struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

// NamedPath is a path entry in the document's Paths panel.
type NamedPath struct {
	Name string     `json:"name"`
	Path model.Path `json:"path"`
}

// PathMeta is the UIMeta representation of a path entry.
type PathMeta struct {
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

// ThumbnailEntry holds base64-encoded RGBA pixel buffers for a layer preview.
// LayerRGBA is always present (when the layer has rasterizable content).
// MaskRGBA is only present when the layer has a pixel mask.
type ThumbnailEntry struct {
	LayerRGBA string `json:"layerRGBA"`
	MaskRGBA  string `json:"maskRGBA,omitempty"`
}

func ParseBackground(kind string) Background {
	switch kind {
	case "white":
		return Background{Kind: "white", Color: [4]uint8{244, 246, 250, 255}}
	case "color":
		return Background{Kind: "color", Color: [4]uint8{236, 147, 92, 255}}
	default:
		return Background{Kind: "transparent"}
	}
}

func CloneNamedPaths(paths []NamedPath) []NamedPath {
	if len(paths) == 0 {
		return nil
	}
	out := make([]NamedPath, len(paths))
	for i, p := range paths {
		out[i] = NamedPath{
			Name: p.Name,
			Path: *model.ClonePath(&p.Path),
		}
	}
	return out
}

func BuildPathsMeta(paths []NamedPath, activePathIdx int) []PathMeta {
	if len(paths) == 0 {
		return nil
	}
	meta := make([]PathMeta, len(paths))
	for i, p := range paths {
		meta[i] = PathMeta{
			Name:   p.Name,
			Active: i == activePathIdx,
		}
	}
	return meta
}
