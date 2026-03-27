package engine

import (
	"encoding/json"
	"testing"
)

func TestNewPixelLayerDefaults(t *testing.T) {
	layer := NewPixelLayer("", LayerBounds{X: 8, Y: 12, W: 64, H: 48}, []byte{1, 2, 3, 4})

	if layer.ID() == "" {
		t.Fatal("ID() = empty, want UUID")
	}
	if layer.Name() != "Layer" {
		t.Fatalf("Name() = %q, want Layer", layer.Name())
	}
	if !layer.Visible() {
		t.Fatal("Visible() = false, want true")
	}
	if layer.LockMode() != LayerLockNone {
		t.Fatalf("LockMode() = %q, want %q", layer.LockMode(), LayerLockNone)
	}
	if layer.Opacity() != 1 || layer.FillOpacity() != 1 {
		t.Fatalf("opacity defaults = %.2f / %.2f, want 1 / 1", layer.Opacity(), layer.FillOpacity())
	}
	if layer.BlendMode() != BlendModeNormal {
		t.Fatalf("BlendMode() = %q, want %q", layer.BlendMode(), BlendModeNormal)
	}
	if layer.Parent() != nil {
		t.Fatal("Parent() should be nil for a detached layer")
	}
	if got := layer.Children(); got != nil {
		t.Fatalf("Children() = %#v, want nil", got)
	}
	if layer.LayerType() != LayerTypePixel {
		t.Fatalf("LayerType() = %q, want %q", layer.LayerType(), LayerTypePixel)
	}
	if layer.Bounds != (LayerBounds{X: 8, Y: 12, W: 64, H: 48}) {
		t.Fatalf("Bounds = %#v, want expected rectangle", layer.Bounds)
	}
	if len(layer.Pixels) != 4 {
		t.Fatalf("len(Pixels) = %d, want 4", len(layer.Pixels))
	}
}

func TestGroupLayerSetChildrenAssignsParents(t *testing.T) {
	group := NewGroupLayer("Folder")
	childA := NewPixelLayer("A", LayerBounds{W: 10, H: 10}, nil)
	childB := NewPixelLayer("B", LayerBounds{W: 20, H: 20}, nil)

	group.SetChildren([]LayerNode{childA, nil, childB})

	children := group.Children()
	if len(children) != 2 {
		t.Fatalf("len(Children()) = %d, want 2", len(children))
	}
	if childA.Parent() != group {
		t.Fatal("childA parent was not wired to the group")
	}
	if childB.Parent() != group {
		t.Fatal("childB parent was not wired to the group")
	}
	if group.LayerType() != LayerTypeGroup {
		t.Fatalf("LayerType() = %q, want %q", group.LayerType(), LayerTypeGroup)
	}
}

func TestNewAdjustmentLayerDefaultsAndClone(t *testing.T) {
	layer := NewAdjustmentLayer("Curves 1", "curves", json.RawMessage(`{"points":[[0,0],[255,255]]}`))

	if layer.LayerType() != LayerTypeAdjustment {
		t.Fatalf("LayerType() = %q, want %q", layer.LayerType(), LayerTypeAdjustment)
	}
	if layer.AdjustmentKind != "curves" {
		t.Fatalf("AdjustmentKind = %q, want curves", layer.AdjustmentKind)
	}
	if string(layer.Params) != `{"points":[[0,0],[255,255]]}` {
		t.Fatalf("Params = %s, want original payload", string(layer.Params))
	}

	clone, ok := layer.Clone().(*AdjustmentLayer)
	if !ok {
		t.Fatalf("Clone() returned %T, want *AdjustmentLayer", layer.Clone())
	}
	if clone == layer {
		t.Fatal("Clone() returned the original adjustment layer pointer")
	}
	if string(clone.Params) != string(layer.Params) {
		t.Fatalf("clone params = %s, want %s", string(clone.Params), string(layer.Params))
	}

	clone.Params[0] = '{'
	if string(layer.Params) != `{"points":[[0,0],[255,255]]}` {
		t.Fatal("adjustment params share backing storage between original and clone")
	}
	if clone.Parent() != nil {
		t.Fatal("cloned adjustment layer should not have a parent")
	}
	if got := clone.Children(); got != nil {
		t.Fatalf("Children() = %#v, want nil", got)
	}
	if !clone.Visible() || clone.LockMode() != LayerLockNone {
		t.Fatal("adjustment layer lost base defaults during clone")
	}
	if clone.Opacity() != 1 || clone.FillOpacity() != 1 {
		t.Fatal("adjustment layer clone should preserve default opacities")
	}
}

func TestTextAndVectorLayerCloneDeepCopiesRasterState(t *testing.T) {
	text := NewTextLayer("Title", LayerBounds{X: 3, Y: 4, W: 2, H: 1}, "Agogo", []byte{1, 2, 3, 255, 4, 5, 6, 255})
	text.FontFamily = "Recursive"
	text.FontSize = 42
	text.Color = [4]uint8{10, 20, 30, 255}
	textClone, ok := text.Clone().(*TextLayer)
	if !ok {
		t.Fatalf("text clone type = %T, want *TextLayer", text.Clone())
	}
	textClone.CachedRaster[0] = 99
	if text.CachedRaster[0] == 99 {
		t.Fatal("text cached raster shares backing storage")
	}
	if textClone.Text != "Agogo" || textClone.FontFamily != "Recursive" || textClone.FontSize != 42 {
		t.Fatal("text clone lost textual properties")
	}

	vector := NewVectorLayer("Shape", LayerBounds{X: 0, Y: 0, W: 2, H: 2}, &Path{Closed: true, Points: []PathPoint{{X: 0, Y: 0}, {X: 2, Y: 2}}}, []byte{7, 8, 9, 255})
	vector.FillColor = [4]uint8{200, 100, 50, 255}
	vector.StrokeColor = [4]uint8{5, 6, 7, 255}
	vector.StrokeWidth = 3
	vectorClone, ok := vector.Clone().(*VectorLayer)
	if !ok {
		t.Fatalf("vector clone type = %T, want *VectorLayer", vector.Clone())
	}
	vectorClone.Shape.Points[0].X = 20
	if vector.Shape.Points[0].X == 20 {
		t.Fatal("vector shape shares backing storage")
	}
	vectorClone.CachedRaster[0] = 88
	if vector.CachedRaster[0] == 88 {
		t.Fatal("vector cached raster shares backing storage")
	}
}

func TestGroupLayerCloneDeepCopiesNestedState(t *testing.T) {
	group := NewGroupLayer("Effects")
	group.SetOpacity(0.5)
	group.SetFillOpacity(0.25)
	group.SetLockMode(LayerLockPosition)
	group.SetMask(&LayerMask{Enabled: true, Width: 2, Height: 2, Data: []byte{255, 127, 64, 0}})
	group.SetVectorMask(&Path{Closed: true, Points: []PathPoint{{X: 1, Y: 2}, {X: 3, Y: 4}}})
	group.SetClippingBase(true)
	group.SetStyleStack([]LayerStyle{{
		Kind:    "drop-shadow",
		Enabled: true,
		Params:  json.RawMessage(`{"distance":8}`),
	}})

	child := NewPixelLayer("Paint", LayerBounds{X: 1, Y: 2, W: 1, H: 1}, []byte{10, 20, 30, 255})
	group.SetChildren([]LayerNode{child})

	clone, ok := group.Clone().(*GroupLayer)
	if !ok {
		t.Fatalf("Clone() returned %T, want *GroupLayer", group.Clone())
	}

	if clone == group {
		t.Fatal("Clone() returned the original group pointer")
	}
	if clone.Parent() != nil {
		t.Fatal("cloned root group should not have a parent")
	}
	if clone.Opacity() != 0.5 || clone.FillOpacity() != 0.25 {
		t.Fatalf("clone opacity = %.2f / %.2f, want 0.5 / 0.25", clone.Opacity(), clone.FillOpacity())
	}
	if clone.LockMode() != LayerLockPosition {
		t.Fatalf("clone lock mode = %q, want %q", clone.LockMode(), LayerLockPosition)
	}
	if !clone.ClippingBase() {
		t.Fatal("clone lost clipping base flag")
	}

	cloneChildren := clone.Children()
	if len(cloneChildren) != 1 {
		t.Fatalf("len(clone.Children()) = %d, want 1", len(cloneChildren))
	}
	cloneChild, ok := cloneChildren[0].(*PixelLayer)
	if !ok {
		t.Fatalf("clone child type = %T, want *PixelLayer", cloneChildren[0])
	}
	if cloneChild.Parent() != clone {
		t.Fatal("cloned child parent was not rewired to the cloned group")
	}

	cloneChild.Pixels[0] = 99
	if child.Pixels[0] == 99 {
		t.Fatal("child pixels share backing storage between original and clone")
	}

	clone.Mask().Data[0] = 1
	if group.Mask().Data[0] == 1 {
		t.Fatal("mask data share backing storage between original and clone")
	}

	clone.VectorMask().Points[0].X = 42
	if group.VectorMask().Points[0].X == 42 {
		t.Fatal("vector mask points share backing storage between original and clone")
	}

	cloneStyles := clone.StyleStack()
	cloneStyles[0].Params[0] = '{'
	if string(group.StyleStack()[0].Params) != `{"distance":8}` {
		t.Fatal("style params share backing storage between original and clone")
	}
	if string(clone.StyleStack()[0].Params) != `{"distance":8}` {
		t.Fatal("StyleStack() should return defensive copies")
	}
}
