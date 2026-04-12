package engine

import "fmt"

// DrawShapePayload is the JSON payload for commandDrawShape.
type DrawShapePayload struct {
	ShapeType      string      `json:"shapeType"`
	X              float64     `json:"x"`
	Y              float64     `json:"y"`
	W              float64     `json:"w"`
	H              float64     `json:"h"`
	CornerRadius   float64     `json:"cornerRadius,omitempty"`
	Sides          int         `json:"sides,omitempty"`
	StarMode       bool        `json:"starMode,omitempty"`
	InnerRadiusPct float64     `json:"innerRadiusPct,omitempty"`
	FillColor      [4]uint8    `json:"fillColor,omitempty"`
	StrokeColor    [4]uint8    `json:"strokeColor,omitempty"`
	StrokeWidth    float64     `json:"strokeWidth,omitempty"`
	Mode           string      `json:"mode,omitempty"`
	Closed         bool        `json:"closed,omitempty"`
	Points         []PathPoint `json:"points,omitempty"`
}

// EnterVectorEditModePayload is the JSON payload for commandEnterVectorEditMode.
type EnterVectorEditModePayload struct {
	LayerID string `json:"layerId"`
}

// CommitVectorEditPayload is the (empty) JSON payload for commandCommitVectorEdit.
type CommitVectorEditPayload struct{}

// SetVectorLayerStylePayload is the JSON payload for commandSetVectorLayerStyle.
type SetVectorLayerStylePayload struct {
	LayerID     string   `json:"layerId"`
	FillColor   [4]uint8 `json:"fillColor"`
	StrokeColor [4]uint8 `json:"strokeColor"`
	StrokeWidth float64  `json:"strokeWidth"`
}

type RasterizeLayerPayload struct {
	LayerID string `json:"layerId,omitempty"`
}

func (inst *instance) dispatchShapeCommand(commandID int32, payloadJSON string) (bool, error) {
	if inst.manager.Active() == nil {
		return true, fmt.Errorf("no active document")
	}

	switch commandID {
	case commandDrawShape:
		var payload DrawShapePayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, inst.drawShape(payload)

	case commandEnterVectorEditMode:
		var payload EnterVectorEditModePayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, inst.enterVectorEditMode(payload)

	case commandCommitVectorEdit:
		return true, inst.commitVectorEdit()

	case commandSetVectorLayerStyle:
		var payload SetVectorLayerStylePayload
		if err := decodePayload(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, inst.setVectorLayerStyle(payload)
	}
	return false, nil
}

func (inst *instance) enterVectorEditMode(p EnterVectorEditModePayload) error {
	doc := inst.manager.Active()
	if doc == nil {
		return fmt.Errorf("no active document")
	}
	layer, _, _, ok := findLayerByID(doc.ensureLayerRoot(), p.LayerID)
	if !ok {
		return fmt.Errorf("layer %q not found", p.LayerID)
	}
	vl, ok := layer.(*VectorLayer)
	if !ok {
		return fmt.Errorf("layer %q is not a vector layer", p.LayerID)
	}

	workPath := clonePath(vl.Shape)
	if workPath == nil {
		workPath = &Path{}
	}
	layerName := layer.Name()
	if err := inst.executeDocCommand("Enter vector edit mode", func(doc *Document) error {
		if len(doc.Paths) == 0 {
			doc.Paths = []NamedPath{{Name: layerName, Path: *workPath}}
		} else {
			doc.Paths[0] = NamedPath{Name: layerName, Path: *workPath}
		}
		doc.ActivePathIdx = 0
		return nil
	}); err != nil {
		return err
	}

	inst.editingVectorLayerID = p.LayerID
	inst.pathTool.activeTool = "direct-select"
	return nil
}

func (inst *instance) commitVectorEdit() error {
	if inst.editingVectorLayerID == "" {
		return nil
	}
	layerID := inst.editingVectorLayerID
	inst.editingVectorLayerID = ""
	inst.pathTool.activeTool = ""

	return inst.executeDocCommand("Commit vector edit", func(doc *Document) error {
		layer, _, _, ok := findLayerByID(doc.ensureLayerRoot(), layerID)
		if !ok {
			return nil // layer deleted — silently succeed
		}
		vl, ok := layer.(*VectorLayer)
		if !ok {
			return nil
		}
		if doc.ActivePathIdx < 0 || doc.ActivePathIdx >= len(doc.Paths) {
			return nil
		}
		editedPath := doc.Paths[doc.ActivePathIdx].Path
		vl.Shape = clonePath(&editedPath)
		raster, err := rasterizeVectorShape(vl.Shape, doc.Width, doc.Height, vl.FillColor, vl.StrokeColor, vl.StrokeWidth)
		if err != nil {
			return err
		}
		vl.CachedRaster = raster
		return nil
	})
}

func (inst *instance) setVectorLayerStyle(p SetVectorLayerStylePayload) error {
	return inst.executeDocCommand("Set shape style", func(doc *Document) error {
		layer, _, _, ok := findLayerByID(doc.ensureLayerRoot(), p.LayerID)
		if !ok {
			return fmt.Errorf("layer %q not found", p.LayerID)
		}
		vl, ok := layer.(*VectorLayer)
		if !ok {
			return fmt.Errorf("layer %q is not a vector layer", p.LayerID)
		}
		vl.FillColor = p.FillColor
		vl.StrokeColor = p.StrokeColor
		vl.StrokeWidth = p.StrokeWidth
		raster, err := rasterizeVectorShape(vl.Shape, doc.Width, doc.Height, vl.FillColor, vl.StrokeColor, vl.StrokeWidth)
		if err != nil {
			return err
		}
		vl.CachedRaster = raster
		return nil
	})
}

func (inst *instance) rasterizeLayer(p RasterizeLayerPayload) error {
	return inst.executeDocCommand("Rasterize layer", func(doc *Document) error {
		layerID := p.LayerID
		if layerID == "" {
			layerID = doc.ActiveLayerID
		}
		if layerID == "" {
			return fmt.Errorf("no layer id")
		}

		layer, parent, index, ok := findLayerByID(doc.ensureLayerRoot(), layerID)
		if !ok || parent == nil {
			return fmt.Errorf("layer %q not found", layerID)
		}
		vectorLayer, ok := layer.(*VectorLayer)
		if !ok {
			return fmt.Errorf("layer %q is not a vector layer", layerID)
		}

		pixelLayer, err := doc.rasterizeAsPixelLayer(vectorLayer, vectorLayer.Name())
		if err != nil {
			return err
		}
		replaceChild(parent, index, pixelLayer)
		doc.normalizeClippingState()
		doc.ActiveLayerID = pixelLayer.ID()
		return nil
	})
}

func (inst *instance) drawShape(p DrawShapePayload) error {
	path, err := buildShapePath(p)
	if err != nil {
		return err
	}

	mode := p.Mode
	if mode == "" {
		mode = "shape"
	}

	switch mode {
	case "path":
		return inst.executeDocCommand("Draw shape path", func(doc *Document) error {
			doc.CreatePath("Shape")
			idx := len(doc.Paths) - 1
			doc.Paths[idx].Path = *path
			return nil
		})

	case "pixels":
		return inst.executeDocCommand("Draw shape pixels", func(doc *Document) error {
			activeLayer, _, _, ok := findLayerByID(doc.ensureLayerRoot(), doc.ActiveLayerID)
			if !ok {
				return fmt.Errorf("no active layer")
			}
			px, ok := activeLayer.(*PixelLayer)
			if !ok {
				return fmt.Errorf("active layer is not a pixel layer")
			}
			rgba, err := rasterizeVectorShape(path, px.Bounds.W, px.Bounds.H, p.FillColor, p.StrokeColor, p.StrokeWidth)
			if err != nil {
				return err
			}
			compositeOver(px.Pixels, rgba, px.Bounds.W, px.Bounds.H)
			return nil
		})

	default: // "shape"
		return inst.executeDocCommand("Draw shape layer", func(doc *Document) error {
			bounds := LayerBounds{
				X: 0,
				Y: 0,
				W: doc.Width,
				H: doc.Height,
			}
			raster, err := rasterizeVectorShape(path, doc.Width, doc.Height, p.FillColor, p.StrokeColor, p.StrokeWidth)
			if err != nil {
				return err
			}
			layer := NewVectorLayer("Shape", bounds, path, raster)
			layer.FillColor = p.FillColor
			layer.StrokeColor = p.StrokeColor
			layer.StrokeWidth = p.StrokeWidth

			// Insert above active layer.
			parentID := ""
			index := -1
			if _, parent, idx, ok := findLayerByID(doc.ensureLayerRoot(), doc.ActiveLayerID); ok && parent != nil {
				parentID = parent.ID()
				if parentID == doc.ensureLayerRoot().ID() {
					parentID = ""
				}
				index = idx + 1
			}
			if err := doc.AddLayer(layer, parentID, index); err != nil {
				return err
			}
			doc.ActiveLayerID = layer.ID()
			return nil
		})
	}
}

// buildShapePath constructs the appropriate Path for the given payload.
func buildShapePath(p DrawShapePayload) (*Path, error) {
	switch p.ShapeType {
	case "rect":
		return makeRectPath(p.X, p.Y, p.W, p.H), nil
	case "rounded-rect":
		return makeRoundedRectPath(p.X, p.Y, p.W, p.H, p.CornerRadius), nil
	case "ellipse":
		return makeEllipsePath(p.X, p.Y, p.W, p.H), nil
	case "polygon":
		sides := p.Sides
		if sides < 3 {
			sides = 6
		}
		return makePolygonPath(p.X, p.Y, p.W, p.H, sides, p.StarMode, p.InnerRadiusPct)
	case "line":
		return makeLinePath(p.X, p.Y, p.X+p.W, p.Y+p.H), nil
	case "custom-shape":
		if len(p.Points) < 1 {
			return nil, fmt.Errorf("custom-shape requires at least 1 point")
		}
		points := make([]PathPoint, 0, len(p.Points))
		for _, raw := range p.Points {
			point := raw
			if !customShapeHasHandles(point) {
				point.InX = point.X
				point.InY = point.Y
				point.OutX = point.X
				point.OutY = point.Y
				point.HandleType = HandleCorner
			}
			points = append(points, point)
		}
		return &Path{Subpaths: []Subpath{{Closed: p.Closed, Points: points}}}, nil
	default:
		return nil, fmt.Errorf("unknown shape type %q", p.ShapeType)
	}
}

func customShapeHasHandles(point PathPoint) bool {
	return point.HandleType != HandleCorner ||
		point.InX != point.X || point.InY != point.Y ||
		point.OutX != point.X || point.OutY != point.Y
}

// compositeOver alpha-composites src over dst in-place (both are w*h*4 RGBA).
func compositeOver(dst, src []byte, w, h int) {
	n := w * h * 4
	if len(dst) < n || len(src) < n {
		return
	}
	for i := 0; i < n; i += 4 {
		sa := uint32(src[i+3])
		if sa == 0 {
			continue
		}
		if sa == 255 {
			dst[i] = src[i]
			dst[i+1] = src[i+1]
			dst[i+2] = src[i+2]
			dst[i+3] = 255
			continue
		}
		da := uint32(dst[i+3])
		outA := sa + da*(255-sa)/255
		if outA == 0 {
			continue
		}
		dst[i] = uint8((uint32(src[i])*sa + uint32(dst[i])*da*(255-sa)/255) / outA)
		dst[i+1] = uint8((uint32(src[i+1])*sa + uint32(dst[i+1])*da*(255-sa)/255) / outA)
		dst[i+2] = uint8((uint32(src[i+2])*sa + uint32(dst[i+2])*da*(255-sa)/255) / outA)
		dst[i+3] = uint8(outA)
	}
}
