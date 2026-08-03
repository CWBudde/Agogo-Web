package abr

import (
	"fmt"
	"math"
	"strings"
)

// BrushDynamics is the supported common subset of an ABR dynamics channel.
type BrushDynamics struct {
	Jitter   *float64
	Control  string
	FadeDabs *int
}

// BrushMetadata is the supported brush-model subset extracted from an Action
// Descriptor. Optional numeric fields distinguish an absent value from zero.
type BrushMetadata struct {
	Name            string
	Diameter        *float64
	Hardness        *float64
	Angle           *float64
	Roundness       *float64
	Spacing         *float64
	SizeDynamics    BrushDynamics
	OpacityDynamics BrushDynamics
	FlowDynamics    BrushDynamics
	Warnings        []string
}

type descriptorLeaf struct {
	path     string
	key      string
	value    Value
	consumed bool
}

// ExtractBrushMetadata maps the documented Agogo descriptor subset onto brush
// values. Unknown leaves are reported instead of silently approximated.
func ExtractBrushMetadata(descriptor Descriptor) BrushMetadata {
	leaves := descriptorLeaves(descriptor, "")
	metadata := BrushMetadata{}
	if leaf := findDescriptorLeaf(leaves, "nm", "name"); leaf != nil && leaf.value.Type == "TEXT" {
		metadata.Name = strings.TrimSpace(leaf.value.String)
		leaf.consumed = true
	}
	metadata.Diameter = mapDescriptorNumber(leaves, &metadata.Warnings, 1, 2500, numberPixels, "dmtr", "diameter")
	metadata.Hardness = mapDescriptorPercent(leaves, &metadata.Warnings, 0, 1, "hrdn", "hardness")
	metadata.Angle = mapDescriptorNumber(leaves, &metadata.Warnings, -180, 180, numberAngle, "angl", "angle")
	metadata.Roundness = mapDescriptorPercent(leaves, &metadata.Warnings, 0.01, 1, "rndn", "roundness")
	metadata.Spacing = mapDescriptorPercent(leaves, &metadata.Warnings, 0.01, 2, "spcn", "spacing")
	metadata.SizeDynamics = mapDescriptorDynamics(leaves, &metadata.Warnings, []string{"sizedynamics", "szdn"}, "size")
	metadata.OpacityDynamics = mapDescriptorDynamics(leaves, &metadata.Warnings, []string{"opacitydynamics", "opdn"}, "opacity")
	metadata.FlowDynamics = mapDescriptorDynamics(leaves, &metadata.Warnings, []string{"flowdynamics", "fldn"}, "flow")
	for index := range leaves {
		if !leaves[index].consumed {
			metadata.Warnings = append(metadata.Warnings, fmt.Sprintf("Unsupported ABR descriptor field %q was ignored.", leaves[index].path))
		}
	}
	return metadata
}

func descriptorLeaves(descriptor Descriptor, prefix string) []*descriptorLeaf {
	leaves := make([]*descriptorLeaf, 0, len(descriptor.Items))
	for _, item := range descriptor.Items {
		path := item.Key
		if prefix != "" {
			path = prefix + "." + item.Key
		}
		if item.Value.Object != nil {
			leaves = append(leaves, descriptorLeaves(*item.Value.Object, path)...)
			continue
		}
		leaves = append(leaves, &descriptorLeaf{path: path, key: normalizeDescriptorKey(item.Key), value: item.Value})
	}
	return leaves
}

func findDescriptorLeaf(leaves []*descriptorLeaf, keys ...string) *descriptorLeaf {
	for _, key := range keys {
		normalized := normalizeDescriptorKey(key)
		for _, leaf := range leaves {
			if !leaf.consumed && leaf.key == normalized {
				return leaf
			}
		}
	}
	return nil
}

type descriptorNumberKind int

const (
	numberPlain descriptorNumberKind = iota
	numberPixels
	numberAngle
)

func mapDescriptorNumber(leaves []*descriptorLeaf, warnings *[]string, minValue, maxValue float64, kind descriptorNumberKind, keys ...string) *float64 {
	leaf := findDescriptorLeaf(leaves, keys...)
	if leaf == nil {
		return nil
	}
	leaf.consumed = true
	value, ok := descriptorNumber(leaf.value, kind)
	if !ok || math.IsNaN(value) || math.IsInf(value, 0) {
		*warnings = append(*warnings, fmt.Sprintf("ABR descriptor field %q has an unsupported numeric value and was ignored.", leaf.path))
		return nil
	}
	clamped := math.Max(minValue, math.Min(maxValue, value))
	if clamped != value {
		*warnings = append(*warnings, fmt.Sprintf("ABR descriptor field %q was clamped from %g to %g.", leaf.path, value, clamped))
	}
	return &clamped
}

func mapDescriptorPercent(leaves []*descriptorLeaf, warnings *[]string, minValue, maxValue float64, keys ...string) *float64 {
	leaf := findDescriptorLeaf(leaves, keys...)
	if leaf == nil {
		return nil
	}
	leaf.consumed = true
	value, ok := descriptorNumber(leaf.value, numberPlain)
	if !ok || (leaf.value.Type == "UntF" && leaf.value.Unit != "#Prc") || math.IsNaN(value) || math.IsInf(value, 0) {
		*warnings = append(*warnings, fmt.Sprintf("ABR descriptor field %q must be a percentage and was ignored.", leaf.path))
		return nil
	}
	value /= 100
	clamped := math.Max(minValue, math.Min(maxValue, value))
	if clamped != value {
		*warnings = append(*warnings, fmt.Sprintf("ABR descriptor field %q was clamped from %g%% to %g%%.", leaf.path, value*100, clamped*100))
	}
	return &clamped
}

func descriptorNumber(value Value, kind descriptorNumberKind) (float64, bool) {
	var number float64
	switch value.Type {
	case "long":
		number = float64(value.Integer)
	case "doub":
		number = value.Float
	case "UntF":
		number = value.Float
		switch kind {
		case numberPixels:
			if value.Unit != "#Pxl" {
				return 0, false
			}
		case numberAngle:
			if value.Unit != "#Ang" {
				return 0, false
			}
		}
	default:
		return 0, false
	}
	return number, true
}

func mapDescriptorDynamics(leaves []*descriptorLeaf, warnings *[]string, prefixes []string, channel string) BrushDynamics {
	var result BrushDynamics
	prefixMatches := func(path string) bool {
		parts := strings.Split(path, ".")
		if len(parts) < 2 {
			return false
		}
		parent := normalizeDescriptorKey(parts[len(parts)-2])
		for _, prefix := range prefixes {
			if parent == normalizeDescriptorKey(prefix) {
				return true
			}
		}
		return false
	}
	find := func(keys ...string) *descriptorLeaf {
		for _, leaf := range leaves {
			if leaf.consumed || !prefixMatches(leaf.path) {
				continue
			}
			for _, key := range keys {
				if leaf.key == normalizeDescriptorKey(key) {
					return leaf
				}
			}
		}
		return nil
	}
	if leaf := find("jitr", "jitter"); leaf != nil {
		leaf.consumed = true
		value, ok := descriptorNumber(leaf.value, numberPlain)
		if !ok || (leaf.value.Type == "UntF" && leaf.value.Unit != "#Prc") {
			*warnings = append(*warnings, fmt.Sprintf("ABR %s dynamics jitter has an unsupported value and was ignored.", channel))
		} else {
			value = math.Max(0, math.Min(100, value)) / 100
			result.Jitter = &value
		}
	}
	if leaf := find("cntrl", "control"); leaf != nil {
		leaf.consumed = true
		if control, ok := descriptorControl(leaf.value); ok {
			result.Control = control
		} else {
			*warnings = append(*warnings, fmt.Sprintf("ABR %s dynamics control %q is unsupported and was ignored.", channel, descriptorValueLabel(leaf.value)))
		}
	}
	if leaf := find("fadd", "fadedabs"); leaf != nil {
		leaf.consumed = true
		value, ok := descriptorNumber(leaf.value, numberPlain)
		if !ok {
			*warnings = append(*warnings, fmt.Sprintf("ABR %s dynamics fade length has an unsupported value and was ignored.", channel))
		} else {
			fade := int(math.Round(math.Max(1, math.Min(10000, value))))
			result.FadeDabs = &fade
		}
	}
	return result
}

func descriptorControl(value Value) (string, bool) {
	label := descriptorValueLabel(value)
	switch normalizeDescriptorKey(label) {
	case "off", "none":
		return "off", true
	case "pressure", "penpressure", "pntp":
		return "pressure", true
	case "tilt", "pentilt", "pntl":
		return "tilt", true
	case "fade", "fad":
		return "fade", true
	default:
		return "", false
	}
}

func descriptorValueLabel(value Value) string {
	switch value.Type {
	case "enum":
		return value.Enum.Value
	case "TEXT":
		return value.String
	default:
		return value.Type
	}
}

func normalizeDescriptorKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
