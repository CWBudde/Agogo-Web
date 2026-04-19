package psd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cwbudde/agogo-web/packages/engine-wasm/internal/model"
)

func parseLayerObjectEffectsPayload(payload []byte, record *LayerRecord) error {
	if record.Effects == nil {
		record.Effects = &LayerEffectsMeta{}
	}
	meta := &ObjectLayerEffectsMeta{}
	record.Effects.Object = meta
	if len(payload) < 8 {
		meta.Malformed = true
		return fmt.Errorf("lfx2 payload too short")
	}
	meta.ObjectVersion = uint32(payload[0])<<24 | uint32(payload[1])<<16 | uint32(payload[2])<<8 | uint32(payload[3])
	meta.DescriptorVersion = uint32(payload[4])<<24 | uint32(payload[5])<<16 | uint32(payload[6])<<8 | uint32(payload[7])
	meta.HasDescriptor = len(payload) > 8
	if len(payload) > 8 {
		meta.EffectKeys = scanObjectEffectStyleKeys(payload[8:])
	}
	return nil
}

func parseLayerLegacyEffectsPayload(payload []byte, record *LayerRecord) error {
	reader := bytes.NewReader(payload)
	if record.Effects == nil {
		record.Effects = &LayerEffectsMeta{}
	}
	if record.Effects.Legacy == nil {
		record.Effects.Legacy = &LegacyLayerEffectsMeta{}
	}
	meta := record.Effects.Legacy
	version, err := readUint16From(reader)
	if err != nil {
		meta.Malformed = true
		meta.Version = 0
		meta.EffectCount = 0
		return nil
	}
	meta.Version = version
	count, err := readUint16From(reader)
	if err != nil {
		meta.Malformed = true
		return nil
	}
	meta.EffectCount = count
	for i := uint16(0); i < count; i++ {
		signature, err := readStringFrom(reader, 4)
		if err != nil {
			meta.Malformed = true
			return err
		}
		if signature != "8BIM" && signature != "8B64" {
			meta.Malformed = true
			return fmt.Errorf("invalid legacy effect signature %q", signature)
		}
		key, err := readStringFrom(reader, 4)
		if err != nil {
			meta.Malformed = true
			return err
		}
		meta.EffectKeys = append(meta.EffectKeys, key)
		size, err := readUint32From(reader)
		if err != nil {
			meta.Malformed = true
			return err
		}
		if int(size) > reader.Len() {
			meta.Malformed = true
			return fmt.Errorf("legacy effect %q data truncated", key)
		}
		if _, err := readBytesFrom(reader, int(size)); err != nil {
			meta.Malformed = true
			return err
		}
	}
	return nil
}

func (meta *LayerEffectsMeta) LegacyStyleStack() []model.LayerStyle {
	if meta == nil || meta.Legacy == nil {
		return nil
	}
	return mapLegacyEffectKeysToLayerStyles(meta.Legacy.EffectKeys)
}

func (meta *LayerEffectsMeta) StyleStack() []model.LayerStyle {
	if meta == nil {
		return nil
	}
	styles := make([]model.LayerStyle, 0)
	seen := make(map[string]struct{})
	appendUniqueStyles := func(layerStyles []model.LayerStyle) {
		for _, style := range layerStyles {
			if style.Kind == "" {
				continue
			}
			if _, ok := seen[style.Kind]; ok {
				continue
			}
			styles = append(styles, style)
			seen[style.Kind] = struct{}{}
		}
	}
	appendUniqueStyles(meta.LegacyStyleStack())
	if meta.Object != nil {
		appendUniqueStyles(mapObjectEffectKeysToLayerStyles(meta.Object.EffectKeys))
	}
	return styles
}

func mapLegacyEffectKeysToLayerStyles(keys []string) []model.LayerStyle {
	if len(keys) == 0 {
		return nil
	}
	styles := make([]model.LayerStyle, 0, len(keys))
	for _, key := range keys {
		kind, ok := legacyEffectStyleKind(key)
		if !ok {
			continue
		}
		styles = append(styles, model.LayerStyle{Kind: kind, Enabled: false})
	}
	return styles
}

func mapObjectEffectKeysToLayerStyles(keys []string) []model.LayerStyle {
	if len(keys) == 0 {
		return nil
	}
	styles := make([]model.LayerStyle, 0, len(keys))
	for _, key := range keys {
		kind, ok := objectEffectStyleKind(key)
		if !ok {
			continue
		}
		styles = append(styles, model.LayerStyle{Kind: kind, Enabled: false})
	}
	return styles
}

func legacyEffectStyleKind(key string) (string, bool) {
	switch key {
	case "drSh":
		return "drop-shadow", true
	case "dsSh":
		return "inner-shadow", true
	case "eglw":
		return "outer-glow", true
	case "iglw":
		return "inner-glow", true
	case "ebbl":
		return "bevel-emboss", true
	default:
		return "", false
	}
}

func objectEffectStyleKind(key string) (string, bool) {
	switch key {
	case "drsh", "dropshadow":
		return "drop-shadow", true
	case "dssh", "innershadow":
		return "inner-shadow", true
	case "eglw", "outerglow":
		return "outer-glow", true
	case "iglw", "innerglow":
		return "inner-glow", true
	case "ebbl", "bevelemboss":
		return "bevel-emboss", true
	case "stroke", "strokestyle":
		return "stroke", true
	case "coloroverlay":
		return "color-overlay", true
	case "gradientoverlay":
		return "gradient-overlay", true
	case "patternoverlay":
		return "pattern-overlay", true
	case "satin":
		return "satin", true
	default:
		return "", false
	}
}

func scanObjectEffectStyleKeys(payload []byte) []string {
	if len(payload) == 0 {
		return nil
	}
	normalized := strings.ToLower(string(payload))
	var keys []string
	for _, pattern := range []string{
		"drsh",
		"dssh",
		"eglw",
		"iglw",
		"ebbl",
		"dropshadow",
		"innershadow",
		"outerglow",
		"innerglow",
		"bevelemboss",
		"strokestyle",
		"coloroverlay",
		"gradientoverlay",
		"patternoverlay",
		"satin",
	} {
		if strings.Contains(normalized, pattern) {
			keys = append(keys, pattern)
		}
	}
	return keys
}

func parseLayerAdjustmentMetadata(key string, payload []byte, record *LayerRecord) error {
	adjustment := AdjustmentMeta{
		Key:        key,
		PayloadLen: len(payload),
	}
	switch key {
	case "levl":
		adjustment.Kind = "levels"
	case "curv":
		adjustment.Kind = "curves"
	case "hue2":
		adjustment.Kind = "hue-saturation"
	case "AgAJ":
		adjustment.Kind = "custom"
	default:
		adjustment.Kind = strings.ToLower(key)
	}
	if len(payload) >= 2 {
		version, err := readUint16From(bytes.NewReader(payload))
		if err == nil {
			adjustment.Version = version
			adjustment.HasVersion = true
		}
	}
	if key == "AgAJ" && len(payload) > 2 {
		var decoded struct {
			Kind string `json:"kind"`
		}
		if err := json.Unmarshal(payload[2:], &decoded); err == nil && decoded.Kind != "" {
			adjustment.Kind = decoded.Kind
		}
	}
	record.Adjustments = append(record.Adjustments, adjustment)
	return nil
}

func parseLayerSmartObjectMetadata(key string, payload []byte, record *LayerRecord) error {
	meta := &SmartObjectMeta{Key: key, PayloadLen: len(payload)}
	record.SmartObject = meta
	reader := bytes.NewReader(payload)
	version, err := readUint32From(reader)
	if err != nil {
		meta.Malformed = true
		return nil
	}
	meta.HasVersion = true
	meta.Version = version
	if reader.Len() == 0 {
		return nil
	}
	identifierLen, err := readUint8From(reader)
	if err != nil {
		meta.Malformed = true
		return fmt.Errorf("smart object identifier length missing")
	}
	if int(identifierLen) > reader.Len() {
		meta.Malformed = true
		return fmt.Errorf("smart object identifier length exceeds payload")
	}
	identifierBytes, err := readBytesFrom(reader, int(identifierLen))
	if err != nil {
		meta.Malformed = true
		return fmt.Errorf("smart object identifier missing")
	}
	meta.Identifier = string(identifierBytes)
	if key == "PlLd" || key == "plLd" {
		meta.HasDescriptor = true
	}
	if key == "SoLd" {
		if reader.Len() >= 4 {
			pageNumber, err := readUint32From(reader)
			if err == nil {
				meta.PageNumber = &pageNumber
			}
		}
		if reader.Len() >= 4 {
			totalPages, err := readUint32From(reader)
			if err == nil {
				meta.TotalPages = &totalPages
			}
		}
		if reader.Len() >= 4 {
			placedType, err := readUint32From(reader)
			if err == nil {
				meta.PlacedType = &placedType
			}
		}
	}
	if reader.Len() > 0 {
		meta.UniqueID = meta.Identifier
	}
	return nil
}

func parseLayerVectorMaskMetadata(key string, payload []byte, record *LayerRecord) error {
	meta := &VectorMaskMeta{Key: key, PayloadLen: len(payload)}
	record.HasVectorMask = true
	record.VectorMask = meta
	reader := bytes.NewReader(payload)
	if len(payload) < 20 {
		meta.Malformed = true
		return nil
	}
	top, err := readInt32From(reader)
	if err != nil {
		meta.Malformed = true
		return nil
	}
	left, err := readInt32From(reader)
	if err != nil {
		meta.Malformed = true
		return nil
	}
	bottom, err := readInt32From(reader)
	if err != nil {
		meta.Malformed = true
		return nil
	}
	right, err := readInt32From(reader)
	if err != nil {
		meta.Malformed = true
		return nil
	}
	width := int(right - left)
	height := int(bottom - top)
	if width < 0 {
		width = 0
	}
	if height < 0 {
		height = 0
	}
	meta.Bounds = model.LayerBounds{
		X: int(left),
		Y: int(top),
		W: width,
		H: height,
	}
	meta.HasBounds = true
	meta.DefaultColor = uint16(payload[16])<<8 | uint16(payload[17])
	meta.Flags = uint16(payload[18])<<8 | uint16(payload[19])
	return nil
}

func parseTextLayerMetadata(key string, payload []byte, record *LayerRecord) error {
	meta := &TextLayerMeta{Key: key, PayloadLen: len(payload)}
	record.Text = meta
	if len(payload) == 0 {
		meta.Malformed = true
		return nil
	}
	if len(payload) >= 2+48+2+4 {
		reader := bytes.NewReader(payload)
		version, err := readUint16From(reader)
		if err == nil && version == 1 {
			if _, err := readBytesFrom(reader, 48); err == nil {
				if _, err := readUint16From(reader); err == nil {
					if descriptorVersion, err := readUint32From(reader); err == nil {
						meta.DescriptorVersion = descriptorVersion
						meta.HasDescriptor = true
						if text, _, err := ParseDescriptorTextValue(payload[len(payload)-reader.Len():], map[string]struct{}{
							"Txt ": {},
							"text": {},
						}); err == nil && text != "" {
							meta.ParsedText = text
							return nil
						}
					}
				}
			}
		}
	}
	textPayload := payload
	if len(payload) >= 4 {
		version, err := readUint32From(bytes.NewReader(payload))
		if err == nil {
			meta.DescriptorVersion = version
			meta.HasDescriptor = true
			textPayload = payload[4:]
		}
	}
	if text, err := ParseUnicodeString(textPayload); err == nil {
		meta.ParsedText = text
	}
	return nil
}

func ParseDescriptorTextValue(data []byte, targetKeys map[string]struct{}) (string, int, error) {
	reader := bytes.NewReader(data)
	if _, err := parseUnicodeStringFromReader(reader); err != nil {
		return "", 0, err
	}
	if _, err := parseDescriptorID(reader); err != nil {
		return "", 0, err
	}
	itemCount, err := readUint32From(reader)
	if err != nil {
		return "", 0, err
	}
	for i := uint32(0); i < itemCount; i++ {
		key, err := parseDescriptorID(reader)
		if err != nil {
			return "", 0, err
		}
		valueType, err := readStringFrom(reader, 4)
		if err != nil {
			return "", 0, err
		}
		switch valueType {
		case "TEXT":
			text, err := parseUnicodeStringFromReader(reader)
			if err != nil {
				return "", 0, err
			}
			if _, ok := targetKeys[key]; ok {
				return text, len(data) - reader.Len(), nil
			}
		case "bool":
			if _, err := reader.ReadByte(); err != nil {
				return "", 0, err
			}
		case "doub":
			if _, err := readBytesFrom(reader, 8); err != nil {
				return "", 0, err
			}
		case "long":
			if _, err := readBytesFrom(reader, 4); err != nil {
				return "", 0, err
			}
		default:
			return "", 0, fmt.Errorf("unsupported descriptor value type %q", valueType)
		}
	}
	return "", len(data) - reader.Len(), nil
}
