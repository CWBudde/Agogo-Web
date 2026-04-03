package engine

import "fmt"

// NamedPath is a path entry in the document's Paths panel.
type NamedPath struct {
	Name string `json:"name"`
	Path Path   `json:"path"`
}

// PathMeta is the UIMeta representation of a path entry.
type PathMeta struct {
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

// --- Payloads ---

// CreatePathPayload is the JSON payload for commandCreatePath.
type CreatePathPayload struct {
	Name string `json:"name"`
}

// DeletePathPayload is the JSON payload for commandDeletePath.
type DeletePathPayload struct {
	PathIndex int `json:"pathIndex"`
}

// RenamePathPayload is the JSON payload for commandRenamePath.
type RenamePathPayload struct {
	PathIndex int    `json:"pathIndex"`
	Name      string `json:"name"`
}

// DuplicatePathPayload is the JSON payload for commandDuplicatePath.
type DuplicatePathPayload struct {
	PathIndex int `json:"pathIndex"`
}

// MakeSelectionFromPathPayload is the JSON payload for commandMakeSelectionFromPath
// and commandRasterizePath.
type MakeSelectionFromPathPayload struct {
	PathIndex     *int    `json:"pathIndex,omitempty"` // nil = active path
	FeatherRadius float64 `json:"featherRadius,omitempty"`
	AntiAlias     bool    `json:"antiAlias,omitempty"`
}

// FillPathPayload is the JSON payload for commandFillPath.
type FillPathPayload struct {
	PathIndex *int     `json:"pathIndex,omitempty"` // nil = active path
	Color     [4]uint8 `json:"color,omitempty"`     // if zero, use foreground color
}

// StrokePathPayload is the JSON payload for commandStrokePath.
type StrokePathPayload struct {
	PathIndex *int     `json:"pathIndex,omitempty"` // nil = active path
	ToolWidth float64  `json:"toolWidth,omitempty"`
	Color     [4]uint8 `json:"color,omitempty"` // if zero, use foreground color
}

// --- Document methods ---

// CreatePath adds a new empty named path to the document.
func (doc *Document) CreatePath(name string) {
	if name == "" {
		name = fmt.Sprintf("Path %d", len(doc.Paths)+1)
	}
	doc.Paths = append(doc.Paths, NamedPath{Name: name})
	doc.ActivePathIdx = len(doc.Paths) - 1
}

// DeletePath removes the path at the given index.
func (doc *Document) DeletePath(index int) error {
	if index < 0 || index >= len(doc.Paths) {
		return fmt.Errorf("path index %d out of range", index)
	}
	doc.Paths = append(doc.Paths[:index], doc.Paths[index+1:]...)
	if doc.ActivePathIdx >= len(doc.Paths) {
		doc.ActivePathIdx = len(doc.Paths) - 1
	}
	return nil
}

// RenamePath renames the path at the given index.
func (doc *Document) RenamePath(index int, name string) error {
	if index < 0 || index >= len(doc.Paths) {
		return fmt.Errorf("path index %d out of range", index)
	}
	doc.Paths[index].Name = name
	return nil
}

// DuplicatePath creates a copy of the path at the given index and inserts it after.
func (doc *Document) DuplicatePath(index int) error {
	if index < 0 || index >= len(doc.Paths) {
		return fmt.Errorf("path index %d out of range", index)
	}
	src := doc.Paths[index]
	dup := NamedPath{
		Name: src.Name + " copy",
		Path: *clonePath(&src.Path),
	}
	// Insert after the source.
	doc.Paths = append(doc.Paths, NamedPath{})
	copy(doc.Paths[index+2:], doc.Paths[index+1:])
	doc.Paths[index+1] = dup
	doc.ActivePathIdx = index + 1
	return nil
}

// pathsMeta returns PathMeta slice for UIMeta.
func (doc *Document) pathsMeta() []PathMeta {
	if len(doc.Paths) == 0 {
		return nil
	}
	meta := make([]PathMeta, len(doc.Paths))
	for i, p := range doc.Paths {
		meta[i] = PathMeta{
			Name:   p.Name,
			Active: i == doc.ActivePathIdx,
		}
	}
	return meta
}

// cloneNamedPaths deep-copies a slice of NamedPath.
func cloneNamedPaths(paths []NamedPath) []NamedPath {
	if len(paths) == 0 {
		return nil
	}
	out := make([]NamedPath, len(paths))
	for i, p := range paths {
		out[i] = NamedPath{
			Name: p.Name,
			Path: *clonePath(&p.Path),
		}
	}
	return out
}
