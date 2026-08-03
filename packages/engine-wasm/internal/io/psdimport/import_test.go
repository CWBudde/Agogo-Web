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
