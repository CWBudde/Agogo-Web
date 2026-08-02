package engine

import (
	"strings"
	"testing"

	"github.com/cwbudde/agogo-web/packages/engine-wasm/internal/io/abr"
)

func TestImportedBrushPresetMapsDescriptorMetadata(t *testing.T) {
	descriptor := abr.Descriptor{Items: []abr.Item{
		{Key: "Nm  ", Value: abr.Value{Type: "TEXT", String: "Mapped Engine Brush"}},
		{Key: "Dmtr", Value: abr.Value{Type: "UntF", Unit: "#Pxl", Float: 36}},
		{Key: "Hrdn", Value: abr.Value{Type: "long", Integer: 70}},
		{Key: "Spcn", Value: abr.Value{Type: "long", Integer: 22}},
		{Key: "Angl", Value: abr.Value{Type: "UntF", Unit: "#Ang", Float: -15}},
		{Key: "Rndn", Value: abr.Value{Type: "long", Integer: 80}},
		{Key: "sizeDynamics", Value: abr.Value{Type: "Objc", Object: engineDynamicsDescriptor(30, "PntP", 180)}},
		{Key: "opacityDynamics", Value: abr.Value{Type: "Objc", Object: engineDynamicsDescriptor(20, "PntP", 180)}},
		{Key: "flowDynamics", Value: abr.Value{Type: "Objc", Object: engineDynamicsDescriptor(10, "PntP", 180)}},
	}}

	got := importedBrushPreset("preset", "Fallback", "tip", 12, &descriptor)
	if got.Name != "Mapped Engine Brush" || got.Size != 36 || got.Hardness != 0.7 || got.Spacing != 0.22 || got.Angle != -15 || got.Roundness != 0.8 {
		t.Fatalf("mapped preset core fields = %+v", got)
	}
	if got.SizeJitter != 0.3 || got.OpacityJitter != 0.2 || got.FlowJitter != 0.1 || got.ControlSource != "pressure" || got.FadeDabs != 180 {
		t.Fatalf("mapped preset dynamics = %+v", got)
	}
	if len(got.Warnings) != 0 {
		t.Fatalf("warnings = %v, want none", got.Warnings)
	}
}

func TestImportedBrushPresetWarnsWhenIndependentDynamicsCannotBeRepresented(t *testing.T) {
	descriptor := abr.Descriptor{Items: []abr.Item{
		{Key: "sizeDynamics", Value: abr.Value{Type: "Objc", Object: engineDynamicsDescriptor(0, "PntP", 100)}},
		{Key: "opacityDynamics", Value: abr.Value{Type: "Objc", Object: engineDynamicsDescriptor(0, "PnTl", 250)}},
	}}

	got := importedBrushPreset("preset", "Fallback", "tip", 12, &descriptor)
	warnings := strings.Join(got.Warnings, "\n")
	if got.ControlSource != "pressure" || got.FadeDabs != 100 {
		t.Fatalf("shared dynamics fallback = %+v", got)
	}
	if !strings.Contains(warnings, "Independent ABR dynamics controls") || !strings.Contains(warnings, "Independent ABR fade lengths") {
		t.Fatalf("warnings = %q", warnings)
	}
}

func engineDynamicsDescriptor(jitter int32, control string, fade int32) *abr.Descriptor {
	return &abr.Descriptor{Items: []abr.Item{
		{Key: "Jitr", Value: abr.Value{Type: "long", Integer: jitter}},
		{Key: "Cntrl", Value: abr.Value{Type: "enum", Enum: abr.EnumValue{Type: "DynC", Value: control}}},
		{Key: "FadD", Value: abr.Value{Type: "long", Integer: fade}},
	}}
}
