package psdexport

import (
	"bytes"
	"testing"

	psdio "github.com/cwbudde/agogo-web/packages/engine-wasm/internal/io/psd"
	"github.com/cwbudde/agogo-web/packages/engine-wasm/internal/model"
)

func TestBuildLayerRecordsWritesPSDGroupMarkersBottomToTop(t *testing.T) {
	group := model.NewGroupLayer("Group")
	group.Isolated = false
	group.SetMask(&model.LayerMask{Enabled: true, Width: 1, Height: 1, Data: []byte{123}})
	group.SetChildren([]model.LayerNode{
		model.NewPixelLayer("Child", model.LayerBounds{W: 1, H: 1}, []byte{1, 2, 3, 255}),
	})

	records, err := buildLayerRecords(Params{Layers: []model.LayerNode{group}}, false)
	if err != nil {
		t.Fatalf("buildLayerRecords: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("record count = %d, want 3", len(records))
	}
	if records[0].SectionType != psdio.LayerSectionBoundingDivider || records[0].Name != "</Layer group>" {
		t.Fatalf("first record = %+v, want bounding divider", records[0])
	}
	if records[1].Name != "Child" || records[1].SectionType != psdio.LayerSectionNormal {
		t.Fatalf("middle record = %+v, want child", records[1])
	}
	if records[2].SectionType != psdio.LayerSectionOpenFolder || records[2].Name != "Group" {
		t.Fatalf("last record = %+v, want folder", records[2])
	}
	if records[2].BlendKey != "pass" {
		t.Fatalf("group blend key = %q, want pass", records[2].BlendKey)
	}
	if len(records[2].Channels) != 1 || records[2].Channels[0].ID != -2 {
		t.Fatalf("group channels = %+v, want user-mask channel", records[2].Channels)
	}
}

func TestNewGroupRecordWritesIsolatedBlendMode(t *testing.T) {
	group := model.NewGroupLayer("Isolated")
	group.Isolated = true
	group.SetBlendMode(model.BlendModeMultiply)

	record := newGroupRecord(group, psdio.LayerSectionClosedFolder, nil)
	if record.BlendKey != psdio.BlendKey(model.BlendModeMultiply) {
		t.Fatalf("blend key = %q, want multiply", record.BlendKey)
	}
}

// TestBuildLayerRecordsWritesTheGroupOpenClosedFlag guards against the writer
// re-opening every folder: appendLayerRecords used to hardcode
// LayerSectionOpenFolder, so a closed folder silently became lsct 1 on export.
// The bounding divider stays lsct 3 either way.
func TestBuildLayerRecordsWritesTheGroupOpenClosedFlag(t *testing.T) {
	newFolder := func(name string, expanded bool) *model.GroupLayer {
		group := model.NewGroupLayer(name)
		group.Isolated = true
		group.Expanded = expanded
		group.SetChildren([]model.LayerNode{
			model.NewPixelLayer(name+" child", model.LayerBounds{W: 1, H: 1}, []byte{1, 2, 3, 255}),
		})
		return group
	}

	records, err := buildLayerRecords(Params{Layers: []model.LayerNode{
		newFolder("Open", true),
		newFolder("Closed", false),
	}}, false)
	if err != nil {
		t.Fatalf("buildLayerRecords: %v", err)
	}

	sectionTypes := map[string]uint32{}
	dividers := 0
	for _, record := range records {
		if record.SectionType == psdio.LayerSectionBoundingDivider {
			dividers++
			continue
		}
		if record.SectionType != psdio.LayerSectionNormal {
			sectionTypes[record.Name] = record.SectionType
		}
	}
	if dividers != 2 {
		t.Fatalf("bounding divider count = %d, want 2", dividers)
	}
	if got := sectionTypes["Open"]; got != psdio.LayerSectionOpenFolder {
		t.Errorf("Open folder section type = %d, want %d", got, psdio.LayerSectionOpenFolder)
	}
	if got := sectionTypes["Closed"]; got != psdio.LayerSectionClosedFolder {
		t.Errorf("Closed folder section type = %d, want %d", got, psdio.LayerSectionClosedFolder)
	}
}

func findTaggedBlock(record psdio.ExportLayerRecord, key string) *psdio.ExportTaggedBlock {
	for i := range record.ExtraBlocks {
		if record.ExtraBlocks[i].Key == key {
			return &record.ExtraBlocks[i]
		}
	}
	return nil
}

// Fill opacity travels as the iOpa tagged block, written only when the layer is
// not fully opaque - absence means 255 in the format. PLAN.md S.10.7.
func TestBuildLayerExtraBlocksWritesFillOpacity(t *testing.T) {
	testCases := []struct {
		name string
		fill float64
		want []byte
	}{
		{name: "half", fill: 128.0 / 255.0, want: []byte{128, 0, 0, 0}},
		// Zero is the value a "!= 255" condition most easily degrades into
		// dropping; a fully transparent fill is not the same as no block.
		{name: "transparent fill", fill: 0, want: []byte{0, 0, 0, 0}},
		{name: "opaque", fill: 1, want: nil},
		// The condition is on the rounded byte, not the float.
		{name: "rounds to opaque", fill: 0.9999, want: nil},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			layer := model.NewPixelLayer("Layer", model.LayerBounds{W: 1, H: 1}, []byte{1, 2, 3, 255})
			layer.SetFillOpacity(testCase.fill)

			block := findTaggedBlock(psdio.ExportLayerRecord{ExtraBlocks: buildLayerExtraBlocks(layer)}, "iOpa")
			if testCase.want == nil {
				if block != nil {
					t.Fatalf("iOpa block = %v, want none", block.Payload)
				}
				return
			}
			if block == nil {
				t.Fatal("iOpa block missing")
			}
			if !bytes.Equal(block.Payload, testCase.want) {
				t.Fatalf("iOpa payload = %v, want %v", block.Payload, testCase.want)
			}
			if block.Signature != "8BIM" {
				t.Fatalf("iOpa signature = %q, want %q", block.Signature, "8BIM")
			}
		})
	}
}

// A group carries its own fill opacity on the opening record. The bounding
// divider carries no layer state at all and must not gain any: readers identify
// it by lsct 3 and never read attributes off it.
func TestBuildLayerRecordsWritesGroupFillOpacityButNotOnTheDivider(t *testing.T) {
	group := model.NewGroupLayer("Group")
	group.Isolated = true
	group.SetFillOpacity(64.0 / 255.0)
	group.SetChildren([]model.LayerNode{
		model.NewPixelLayer("Child", model.LayerBounds{W: 1, H: 1}, []byte{1, 2, 3, 255}),
	})

	records, err := buildLayerRecords(Params{Layers: []model.LayerNode{group}}, false)
	if err != nil {
		t.Fatalf("buildLayerRecords: %v", err)
	}
	for _, record := range records {
		block := findTaggedBlock(record, "iOpa")
		switch record.SectionType {
		case psdio.LayerSectionBoundingDivider:
			if block != nil {
				t.Errorf("bounding divider carries an iOpa block %v", block.Payload)
			}
		case psdio.LayerSectionOpenFolder, psdio.LayerSectionClosedFolder:
			if block == nil || !bytes.Equal(block.Payload, []byte{64, 0, 0, 0}) {
				t.Errorf("folder record iOpa block = %v, want [64 0 0 0]", block)
			}
		}
	}
}
