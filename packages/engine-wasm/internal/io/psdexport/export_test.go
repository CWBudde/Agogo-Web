package psdexport

import (
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

	record := newGroupRecord(group, psdio.LayerSectionClosedFolder)
	if record.BlendKey != psdio.BlendKey(model.BlendModeMultiply) {
		t.Fatalf("blend key = %q, want multiply", record.BlendKey)
	}
}
