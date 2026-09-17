package psdimport

import (
	"strings"
	"testing"

	psdio "github.com/cwbudde/agogo-web/packages/engine-wasm/internal/io/psd"
	"github.com/cwbudde/agogo-web/packages/engine-wasm/internal/model"
)

func TestBuildLayerNodesUsesBoundingDividerThenFolderRecord(t *testing.T) {
	header := psdio.Header{Width: 1, Height: 1, Depth: 8, ColorMode: psdio.ColorModeRGB}
	layers := []psdio.LayerRecord{
		{SectionType: psdio.LayerSectionBoundingDivider},
		pixelRecord("Bottom", 10),
		{SectionType: psdio.LayerSectionBoundingDivider},
		pixelRecord("Nested child", 20),
		{
			Name:        "Nested",
			SectionType: psdio.LayerSectionClosedFolder,
			Visible:     true,
			Opacity:     0.75,
			BlendMode:   model.BlendModeMultiply,
			PassThrough: false,
		},
		{
			Name:        "Outer",
			SectionType: psdio.LayerSectionOpenFolder,
			Visible:     true,
			Opacity:     0.5,
			BlendMode:   model.BlendModeNormal,
			PassThrough: true,
		},
	}

	nodes, warnings, err := BuildLayerNodes(header, layers)
	if err != nil {
		t.Fatalf("BuildLayerNodes: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v", warnings)
	}
	if len(nodes) != 1 {
		t.Fatalf("root node count = %d, want 1", len(nodes))
	}
	outer, ok := nodes[0].(*model.GroupLayer)
	if !ok {
		t.Fatalf("root node type = %T, want *model.GroupLayer", nodes[0])
	}
	if outer.Name() != "Outer" || outer.Isolated || outer.Opacity() != 0.5 {
		t.Fatalf("outer attributes = name %q isolated %v opacity %v", outer.Name(), outer.Isolated, outer.Opacity())
	}
	children := outer.Children()
	if len(children) != 2 || children[0].Name() != "Bottom" {
		t.Fatalf("outer children = %#v", children)
	}
	nested, ok := children[1].(*model.GroupLayer)
	if !ok {
		t.Fatalf("nested child type = %T, want *model.GroupLayer", children[1])
	}
	if nested.Name() != "Nested" || !nested.Isolated || nested.BlendMode() != model.BlendModeMultiply {
		t.Fatalf("nested attributes = name %q isolated %v blend %q", nested.Name(), nested.Isolated, nested.BlendMode())
	}
	if got := nested.Children(); len(got) != 1 || got[0].Name() != "Nested child" {
		t.Fatalf("nested children = %#v", got)
	}
}

func TestBuildLayerNodesFlattensUnclosedGroupWithoutDroppingChildren(t *testing.T) {
	header := psdio.Header{Width: 1, Height: 1, Depth: 8, ColorMode: psdio.ColorModeRGB}
	nodes, warnings, err := BuildLayerNodes(header, []psdio.LayerRecord{
		{SectionType: psdio.LayerSectionBoundingDivider},
		pixelRecord("Preserved", 42),
	})
	if err != nil {
		t.Fatalf("BuildLayerNodes: %v", err)
	}
	if len(nodes) != 1 || nodes[0].Name() != "Preserved" {
		t.Fatalf("nodes = %#v, want preserved flattened child", nodes)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "imported its contents without a group") {
		t.Fatalf("warnings = %v", warnings)
	}
}

func TestBuildLayerNodesPlacesOffsetMaskInDocumentSpace(t *testing.T) {
	header := psdio.Header{Width: 4, Height: 3, Depth: 8, ColorMode: psdio.ColorModeRGB}
	record := pixelRecord("Masked", 42)
	record.HasLayerMask = true
	record.LayerMaskEnabled = true
	record.LayerMaskDefault = 255
	record.LayerMaskBounds = model.LayerBounds{X: 1, Y: 1, W: 2, H: 1}
	record.ChannelPixels[-2] = []byte{10, 20}

	nodes, warnings, err := BuildLayerNodes(header, []psdio.LayerRecord{record})
	if err != nil {
		t.Fatalf("BuildLayerNodes: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v", warnings)
	}
	mask := nodes[0].Mask()
	if mask == nil || mask.Width != 4 || mask.Height != 3 || !mask.Enabled {
		t.Fatalf("mask = %+v", mask)
	}
	want := []byte{
		255, 255, 255, 255,
		255, 10, 20, 255,
		255, 255, 255, 255,
	}
	if len(mask.Data) != len(want) {
		t.Fatalf("mask data length = %d, want %d", len(mask.Data), len(want))
	}
	for index := range want {
		if mask.Data[index] != want[index] {
			t.Fatalf("mask data[%d] = %d, want %d", index, mask.Data[index], want[index])
		}
	}
}

func pixelRecord(name string, value byte) psdio.LayerRecord {
	return psdio.LayerRecord{
		Name:      name,
		Bounds:    model.LayerBounds{W: 1, H: 1},
		Visible:   true,
		Opacity:   1,
		BlendMode: model.BlendModeNormal,
		ChannelPixels: map[int16][]byte{
			0:  {value},
			1:  {value},
			2:  {value},
			-1: {255},
		},
	}
}

// TestBuildLayerNodesPreservesFolderOpenClosedState pins the lsct 1 vs lsct 2
// distinction. Before PLAN.md S.10.3 both funnelled down one popStack path and
// the flag was discarded, so an open and a closed folder imported identically.
func TestBuildLayerNodesPreservesFolderOpenClosedState(t *testing.T) {
	header := psdio.Header{Width: 1, Height: 1, Depth: 8, ColorMode: psdio.ColorModeRGB}
	layers := []psdio.LayerRecord{
		{SectionType: psdio.LayerSectionBoundingDivider},
		pixelRecord("Open child", 10),
		{Name: "Open", SectionType: psdio.LayerSectionOpenFolder, Visible: true, Opacity: 1, BlendMode: model.BlendModeNormal},
		{SectionType: psdio.LayerSectionBoundingDivider},
		pixelRecord("Closed child", 20),
		{Name: "Closed", SectionType: psdio.LayerSectionClosedFolder, Visible: true, Opacity: 1, BlendMode: model.BlendModeNormal},
	}

	nodes, warnings, err := BuildLayerNodes(header, layers)
	if err != nil {
		t.Fatalf("BuildLayerNodes: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v", warnings)
	}
	if len(nodes) != 2 {
		t.Fatalf("root node count = %d, want 2", len(nodes))
	}
	openGroup, ok := nodes[0].(*model.GroupLayer)
	if !ok {
		t.Fatalf("node 0 type = %T, want *model.GroupLayer", nodes[0])
	}
	closedGroup, ok := nodes[1].(*model.GroupLayer)
	if !ok {
		t.Fatalf("node 1 type = %T, want *model.GroupLayer", nodes[1])
	}
	if !openGroup.Expanded {
		t.Errorf("group %q imported from lsct 1 must be expanded", openGroup.Name())
	}
	if closedGroup.Expanded {
		t.Errorf("group %q imported from lsct 2 must be collapsed", closedGroup.Name())
	}
}

// Fill opacity is a compositing parameter like Opacity, so it must reach the
// model on both the raster and the group path. PLAN.md S.10.7.
func TestBuildLayerNodesPreservesFillOpacity(t *testing.T) {
	header := psdio.Header{Width: 1, Height: 1, Depth: 8, ColorMode: psdio.ColorModeRGB}
	half := pixelRecord("Half fill", 10)
	half.FillOpacity = 128.0 / 255.0
	plain := pixelRecord("Plain", 20)
	plain.FillOpacity = 1

	layers := []psdio.LayerRecord{
		{SectionType: psdio.LayerSectionBoundingDivider},
		half,
		{
			Name: "Group", SectionType: psdio.LayerSectionOpenFolder, Visible: true,
			Opacity: 1, FillOpacity: 64.0 / 255.0, BlendMode: model.BlendModeNormal,
		},
		plain,
	}

	nodes, warnings, err := BuildLayerNodes(header, layers)
	if err != nil {
		t.Fatalf("BuildLayerNodes: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v", warnings)
	}
	if len(nodes) != 2 {
		t.Fatalf("root node count = %d, want 2", len(nodes))
	}

	group, ok := nodes[0].(*model.GroupLayer)
	if !ok {
		t.Fatalf("node 0 type = %T, want *model.GroupLayer", nodes[0])
	}
	if got, want := group.FillOpacity(), 64.0/255.0; got != want {
		t.Errorf("group fill opacity = %v, want %v", got, want)
	}
	if got, want := group.Children()[0].FillOpacity(), 128.0/255.0; got != want {
		t.Errorf("child fill opacity = %v, want %v", got, want)
	}
	if got := nodes[1].FillOpacity(); got != 1 {
		t.Errorf("layer without an iOpa block has fill opacity %v, want 1", got)
	}
}
