package abr

import (
	"strings"
	"testing"
)

func TestExtractBrushMetadataMapsSupportedTipAndDynamicsFields(t *testing.T) {
	descriptor := Descriptor{Items: []Item{
		{Key: "Nm  ", Value: Value{Type: "TEXT", String: "Mapped Brush"}},
		{Key: "Dmtr", Value: Value{Type: "UntF", Unit: "#Pxl", Float: 48}},
		{Key: "Spcn", Value: Value{Type: "UntF", Unit: "#Prc", Float: 18}},
		{Key: "shape", Value: Value{Type: "Objc", Object: &Descriptor{Items: []Item{
			{Key: "Hrdn", Value: Value{Type: "long", Integer: 65}},
			{Key: "Angl", Value: Value{Type: "UntF", Unit: "#Ang", Float: 32}},
			{Key: "Rndn", Value: Value{Type: "long", Integer: 72}},
		}}}},
		{Key: "sizeDynamics", Value: Value{Type: "Objc", Object: dynamicsDescriptor(25, "PntP", 240)}},
		{Key: "opacityDynamics", Value: Value{Type: "Objc", Object: dynamicsDescriptor(15, "PntP", 240)}},
		{Key: "flowDynamics", Value: Value{Type: "Objc", Object: dynamicsDescriptor(5, "PntP", 240)}},
	}}

	got := ExtractBrushMetadata(descriptor)
	if got.Name != "Mapped Brush" || value(got.Diameter) != 48 || value(got.Hardness) != 0.65 || value(got.Spacing) != 0.18 {
		t.Fatalf("mapped core metadata = %+v", got)
	}
	if value(got.Angle) != 32 || value(got.Roundness) != 0.72 {
		t.Fatalf("mapped shape metadata = %+v", got)
	}
	if value(got.SizeDynamics.Jitter) != 0.25 || value(got.OpacityDynamics.Jitter) != 0.15 || value(got.FlowDynamics.Jitter) != 0.05 {
		t.Fatalf("mapped dynamics jitter = %+v", got)
	}
	if got.SizeDynamics.Control != "pressure" || got.OpacityDynamics.Control != "pressure" || got.FlowDynamics.Control != "pressure" {
		t.Fatalf("mapped dynamics controls = %+v", got)
	}
	if intValue(got.SizeDynamics.FadeDabs) != 240 || intValue(got.OpacityDynamics.FadeDabs) != 240 || intValue(got.FlowDynamics.FadeDabs) != 240 {
		t.Fatalf("mapped dynamics fade = %+v", got)
	}
	if len(got.Warnings) != 0 {
		t.Fatalf("warnings = %v, want none", got.Warnings)
	}
}

func TestExtractBrushMetadataWarnsForUnsupportedFieldsAndValues(t *testing.T) {
	descriptor := Descriptor{Items: []Item{
		{Key: "texture", Value: Value{Type: "bool", Bool: true}},
		{Key: "Dmtr", Value: Value{Type: "UntF", Unit: "#Prc", Float: 50}},
		{Key: "sizeDynamics", Value: Value{Type: "Objc", Object: &Descriptor{Items: []Item{
			{Key: "Cntrl", Value: Value{Type: "enum", Enum: EnumValue{Value: "stylusWheel"}}},
		}}}},
	}}

	got := ExtractBrushMetadata(descriptor)
	if got.Diameter != nil || got.SizeDynamics.Control != "" {
		t.Fatalf("unsupported values were mapped: %+v", got)
	}
	warnings := strings.Join(got.Warnings, "\n")
	for _, want := range []string{"Dmtr", "stylusWheel", "texture"} {
		if !strings.Contains(warnings, want) {
			t.Fatalf("warnings %q do not mention %q", warnings, want)
		}
	}
}

func dynamicsDescriptor(jitter int32, control string, fade int32) *Descriptor {
	return &Descriptor{Items: []Item{
		{Key: "Jitr", Value: Value{Type: "long", Integer: jitter}},
		{Key: "Cntrl", Value: Value{Type: "enum", Enum: EnumValue{Type: "DynC", Value: control}}},
		{Key: "FadD", Value: Value{Type: "long", Integer: fade}},
	}}
}

func value(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}

func intValue(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}
