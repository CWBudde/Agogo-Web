package engine

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"

	agglib "github.com/cwbudde/agg_go"
	docpkg "github.com/cwbudde/agogo-web/packages/engine-wasm/internal/document"
	"github.com/cwbudde/agogo-web/packages/engine-wasm/internal/io/abr"
)

const importedBrushThumbnailSize = 64

type importAbrBrushLibraryPayload struct {
	Data     string `json:"data"`
	FileName string `json:"fileName,omitempty"`
}

func (inst *instance) importAbrBrushLibrary(payloadJSON string) (*ImportedBrushLibrary, error) {
	var payload importAbrBrushLibraryPayload
	if err := json.Unmarshal([]byte(payloadJSON), &payload); err != nil {
		return nil, fmt.Errorf("decode ABR import payload: %w", err)
	}
	data, err := base64.StdEncoding.DecodeString(payload.Data)
	if err != nil {
		return nil, fmt.Errorf("decode ABR bytes: %w", err)
	}
	parsed, err := abr.Parse(data)
	if err != nil {
		return nil, err
	}
	if len(parsed.Sampled) == 0 && len(parsed.Descriptors) == 0 {
		return nil, fmt.Errorf("ABR library contains no supported brush presets")
	}

	resources := make(map[string]*brushTipResource, len(parsed.Sampled))
	presets := make([]ImportedBrushPreset, 0, maxInt(len(parsed.Sampled), len(parsed.Descriptors)))
	for index, sampled := range parsed.Sampled {
		id := brushTipResourceID(sampled.Width, sampled.Height, sampled.Pixels)
		resource := &brushTipResource{
			ID: id, Width: sampled.Width, Height: sampled.Height,
			Alpha: append([]byte(nil), sampled.Pixels...),
		}
		resources[id] = resource
		name := fmt.Sprintf("Imported Brush %d", index+1)
		if index < len(parsed.Descriptors) && parsed.Descriptors[index].Name != "" {
			name = parsed.Descriptors[index].Name
		}
		thumbnail, thumbnailErr := renderBrushResourceThumbnail(resource)
		warnings := []string(nil)
		if thumbnailErr != nil {
			warnings = append(warnings, thumbnailErr.Error())
		}
		var descriptor *abr.Descriptor
		if index < len(parsed.Descriptors) {
			descriptor = &parsed.Descriptors[index]
		}
		preset := importedBrushPreset(id, name, id, float64(maxInt(sampled.Width, sampled.Height)), descriptor)
		preset.ThumbnailRGBA = thumbnail
		preset.Warnings = append(preset.Warnings, warnings...)
		presets = append(presets, preset)
	}
	for index := len(parsed.Sampled); index < len(parsed.Descriptors); index++ {
		name := parsed.Descriptors[index].Name
		if name == "" {
			name = fmt.Sprintf("Computed Brush %d", index-len(parsed.Sampled)+1)
		}
		preset := importedBrushPreset(descriptorPresetID(parsed.Descriptors[index]), name, "", 20, &parsed.Descriptors[index])
		preset.Warnings = append(preset.Warnings, "Computed ABR preset has no sampled tip; Agogo uses its procedural round tip.")
		presets = append(presets, preset)
	}
	if inst.brushResources == nil {
		inst.brushResources = make(map[string]*brushTipResource)
	}
	if inst.brushPresets == nil {
		inst.brushPresets = make(map[string]ImportedBrushPreset)
	}
	for id, resource := range resources {
		inst.brushResources[id] = resource
	}
	for _, preset := range presets {
		if preset.TipResourceID != "" {
			inst.brushPresets[preset.TipResourceID] = docpkg.CloneImportedBrushPreset(preset)
		}
	}
	libraryHash := sha256.Sum256(data)
	return &ImportedBrushLibrary{
		LibraryID: "abr-" + hex.EncodeToString(libraryHash[:12]),
		Name:      stringValueOrDefault(payload.FileName, "Imported ABR Library"),
		Presets:   presets,
	}, nil
}

func importedBrushPreset(id, name, tipResourceID string, defaultSize float64, descriptor *abr.Descriptor) ImportedBrushPreset {
	preset := ImportedBrushPreset{
		ID: id, Name: name, TipResourceID: tipResourceID, TipShape: "round",
		Size: defaultSize, Hardness: 1, Spacing: defaultBrushSpacing, Roundness: 1,
		ControlSource: "off", FadeDabs: 100,
	}
	if descriptor == nil {
		return preset
	}
	metadata := abr.ExtractBrushMetadata(*descriptor)
	if metadata.Name != "" {
		preset.Name = metadata.Name
	}
	if metadata.Diameter != nil {
		preset.Size = *metadata.Diameter
	}
	if metadata.Hardness != nil {
		preset.Hardness = *metadata.Hardness
	}
	if metadata.Spacing != nil {
		preset.Spacing = *metadata.Spacing
	}
	if metadata.Angle != nil {
		preset.Angle = *metadata.Angle
	}
	if metadata.Roundness != nil {
		preset.Roundness = *metadata.Roundness
	}
	if metadata.SizeDynamics.Jitter != nil {
		preset.SizeJitter = *metadata.SizeDynamics.Jitter
	}
	if metadata.OpacityDynamics.Jitter != nil {
		preset.OpacityJitter = *metadata.OpacityDynamics.Jitter
	}
	if metadata.FlowDynamics.Jitter != nil {
		preset.FlowJitter = *metadata.FlowDynamics.Jitter
	}
	preset.Warnings = append(preset.Warnings, metadata.Warnings...)
	applySharedImportedDynamics(&preset, metadata)
	return preset
}

func applySharedImportedDynamics(preset *ImportedBrushPreset, metadata abr.BrushMetadata) {
	controls := []string{metadata.SizeDynamics.Control, metadata.OpacityDynamics.Control, metadata.FlowDynamics.Control}
	fades := []*int{metadata.SizeDynamics.FadeDabs, metadata.OpacityDynamics.FadeDabs, metadata.FlowDynamics.FadeDabs}
	controlSet := false
	for _, control := range controls {
		if control == "" {
			continue
		}
		if !controlSet {
			preset.ControlSource = control
			controlSet = true
		} else if preset.ControlSource != control {
			preset.Warnings = append(preset.Warnings, fmt.Sprintf("Independent ABR dynamics controls are unsupported; using %q for all channels.", preset.ControlSource))
			break
		}
	}
	fadeSet := false
	for _, fade := range fades {
		if fade == nil {
			continue
		}
		if !fadeSet {
			preset.FadeDabs = *fade
			fadeSet = true
		} else if preset.FadeDabs != *fade {
			preset.Warnings = append(preset.Warnings, fmt.Sprintf("Independent ABR fade lengths are unsupported; using %d dabs for all channels.", preset.FadeDabs))
			break
		}
	}
}

func brushTipResourceID(width, height int, alpha []byte) string {
	hash := sha256.New()
	_, _ = fmt.Fprintf(hash, "%dx%d:", width, height)
	_, _ = hash.Write(alpha)
	return "tip-" + hex.EncodeToString(hash.Sum(nil)[:16])
}

func descriptorPresetID(descriptor abr.Descriptor) string {
	hash := sha256.Sum256([]byte(descriptor.Name + "\x00" + descriptor.ClassID))
	return "computed-" + hex.EncodeToString(hash[:12])
}

func renderBrushResourceThumbnail(resource *brushTipResource) (string, error) {
	if !validBrushTipResource(resource) {
		return "", fmt.Errorf("invalid sampled brush tip")
	}
	source := make([]byte, resource.Width*resource.Height*4)
	for index, alpha := range resource.Alpha {
		offset := index * 4
		source[offset] = 255
		source[offset+1] = 255
		source[offset+2] = 255
		source[offset+3] = alpha
	}
	dest := make([]byte, importedBrushThumbnailSize*importedBrushThumbnailSize*4)
	renderer := agglib.NewAgg2D()
	renderer.Attach(dest, importedBrushThumbnailSize, importedBrushThumbnailSize, importedBrushThumbnailSize*4)
	renderer.ImageFilter(agglib.Bilinear)
	image := agglib.NewImage(source, resource.Width, resource.Height, resource.Width*4)
	if err := renderer.TransformImageSimple(image, 0, 0, importedBrushThumbnailSize, importedBrushThumbnailSize); err != nil {
		return "", fmt.Errorf("render brush thumbnail: %w", err)
	}
	return base64.StdEncoding.EncodeToString(dest), nil
}

func validBrushTipResource(resource *brushTipResource) bool {
	if resource == nil || resource.Width <= 0 || resource.Height <= 0 || resource.Width > len(resource.Alpha) {
		return false
	}
	return len(resource.Alpha)%resource.Width == 0 && len(resource.Alpha)/resource.Width == resource.Height
}

func (inst *instance) brushResource(id string) *brushTipResource {
	if inst == nil {
		return nil
	}
	resource := inst.brushResources[id]
	return docpkg.CloneBrushTipResource(resource)
}

// documentBrushResource resolves an imported tip and records a document-owned
// copy on first use. Embedded project tips are resolved from the document even
// when the original app-level ABR library is unavailable.
func (inst *instance) documentBrushResource(doc *Document, id string) *brushTipResource {
	if doc == nil || id == "" {
		return nil
	}
	if resource := doc.BrushResources[id]; resource != nil {
		return docpkg.CloneBrushTipResource(resource)
	}
	resource := inst.brushResource(id)
	if resource == nil {
		return nil
	}
	if doc.BrushResources == nil {
		doc.BrushResources = make(map[string]*brushTipResource)
	}
	doc.BrushResources[id] = docpkg.CloneBrushTipResource(resource)
	if preset, ok := inst.brushPresets[id]; ok {
		if doc.BrushPresets == nil {
			doc.BrushPresets = make(map[string]ImportedBrushPreset)
		}
		doc.BrushPresets[id] = docpkg.CloneImportedBrushPreset(preset)
	}
	return resource
}

// registerDocumentBrushResources restores portable project resources to the
// instance-level registry so imported documents behave like locally imported
// ABR libraries for future strokes.
func (inst *instance) registerDocumentBrushResources(doc *Document) {
	if inst == nil || doc == nil {
		return
	}
	if inst.brushResources == nil {
		inst.brushResources = make(map[string]*brushTipResource)
	}
	if inst.brushPresets == nil {
		inst.brushPresets = make(map[string]ImportedBrushPreset)
	}
	for id, resource := range doc.BrushResources {
		inst.brushResources[id] = docpkg.CloneBrushTipResource(resource)
	}
	for id, preset := range doc.BrushPresets {
		inst.brushPresets[id] = docpkg.CloneImportedBrushPreset(preset)
	}
}
