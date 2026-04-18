package engine

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf16"
)

func parsePSDLayerObjectEffectsPayload(payload []byte, record *psdLayerRecord) error {
	if record.Effects == nil {
		record.Effects = &psdLayerEffectsMeta{}
	}
	meta := &psdObjectLayerEffectsMeta{}
	record.Effects.Object = meta
	if len(payload) < 8 {
		meta.Malformed = true
		return fmt.Errorf("lfx2 payload too short")
	}
	meta.ObjectVersion = binary.BigEndian.Uint32(payload[0:4])
	meta.DescriptorVersion = binary.BigEndian.Uint32(payload[4:8])
	meta.HasDescriptor = len(payload) > 8
	if len(payload) > 8 {
		meta.EffectKeys = scanObjectEffectStyleKeys(payload[8:])
	}
	return nil
}

func parsePSDLayerLegacyEffectsPayload(payload []byte, record *psdLayerRecord) error {
	reader := bytes.NewReader(payload)
	if record.Effects == nil {
		record.Effects = &psdLayerEffectsMeta{}
	}
	if record.Effects.Legacy == nil {
		record.Effects.Legacy = &psdLegacyLayerEffectsMeta{}
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

func (meta *psdLayerEffectsMeta) GetLegacyStyleStack() []LayerStyle {
	if meta == nil || meta.Legacy == nil {
		return nil
	}
	return mapLegacyEffectKeysToLayerStyles(meta.Legacy.EffectKeys)
}

func (meta *psdLayerEffectsMeta) GetStyleStack() []LayerStyle {
	if meta == nil {
		return nil
	}
	styles := make([]LayerStyle, 0)
	seen := make(map[LayerStyleKind]struct{})
	appendUniqueStyles := func(layerStyles []LayerStyle) {
		for _, style := range layerStyles {
			kind := LayerStyleKind(style.Kind)
			if _, ok := seen[kind]; ok {
				continue
			}
			if kind == "" {
				continue
			}
			styles = append(styles, style)
			seen[kind] = struct{}{}
		}
	}
	appendUniqueStyles(meta.GetLegacyStyleStack())
	if meta.Object != nil {
		appendUniqueStyles(mapObjectEffectKeysToLayerStyles(meta.Object.EffectKeys))
	}
	return styles
}

func mapLegacyEffectKeysToLayerStyles(keys []string) []LayerStyle {
	if len(keys) == 0 {
		return nil
	}
	styles := make([]LayerStyle, 0, len(keys))
	for _, key := range keys {
		kind, ok := legacyEffectStyleKind(key)
		if !ok {
			continue
		}
		styles = append(styles, LayerStyle{
			Kind:    string(kind),
			Enabled: false,
		})
	}
	return styles
}

func mapObjectEffectKeysToLayerStyles(keys []string) []LayerStyle {
	if len(keys) == 0 {
		return nil
	}
	styles := make([]LayerStyle, 0, len(keys))
	for _, key := range keys {
		kind, ok := objectEffectStyleKind(key)
		if !ok {
			continue
		}
		styles = append(styles, LayerStyle{
			Kind:    string(kind),
			Enabled: false,
		})
	}
	return styles
}

func legacyEffectStyleKind(key string) (LayerStyleKind, bool) {
	switch key {
	case "drSh":
		return LayerStyleKindDropShadow, true
	case "dsSh":
		return LayerStyleKindInnerShadow, true
	case "eglw":
		return LayerStyleKindOuterGlow, true
	case "iglw":
		return LayerStyleKindInnerGlow, true
	case "ebbl":
		return LayerStyleKindBevelEmboss, true
	default:
		return "", false
	}
}

func objectEffectStyleKind(key string) (LayerStyleKind, bool) {
	switch key {
	case "drsh", "dropshadow":
		return LayerStyleKindDropShadow, true
	case "dssh", "innershadow":
		return LayerStyleKindInnerShadow, true
	case "eglw", "outerglow":
		return LayerStyleKindOuterGlow, true
	case "iglw", "innerglow":
		return LayerStyleKindInnerGlow, true
	case "ebbl", "bevelemboss":
		return LayerStyleKindBevelEmboss, true
	case "stroke", "strokestyle":
		return LayerStyleKindStroke, true
	case "coloroverlay":
		return LayerStyleKindColorOverlay, true
	case "gradientoverlay":
		return LayerStyleKindGradientOverlay, true
	case "patternoverlay":
		return LayerStyleKindPatternOverlay, true
	case "satin":
		return LayerStyleKindSatin, true
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

func parsePSDLayerAdjustmentMetadata(key string, payload []byte, record *psdLayerRecord) error {
	adjustment := psdAdjustmentMeta{
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

func parsePSDLayerSmartObjectMetadata(key string, payload []byte, record *psdLayerRecord) error {
	meta := &psdSmartObjectMeta{
		Key:        key,
		PayloadLen: len(payload),
	}
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

func parsePSDLayerVectorMaskMetadata(key string, payload []byte, record *psdLayerRecord) error {
	meta := &psdVectorMaskMeta{
		Key:        key,
		PayloadLen: len(payload),
	}
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
	meta.Bounds = LayerBounds{
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

func parsePSDTextLayerMetadata(key string, payload []byte, record *psdLayerRecord) error {
	meta := &psdTextLayerMeta{
		Key:        key,
		PayloadLen: len(payload),
	}
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
						if text, _, err := parsePSDDescriptorTextValue(payload[len(payload)-reader.Len():], map[string]struct{}{
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
	if text, err := parsePSDUnicodeString(textPayload); err == nil {
		meta.ParsedText = text
	}
	return nil
}

func parsePSDDescriptorTextValue(data []byte, targetKeys map[string]struct{}) (string, int, error) {
	reader := bytes.NewReader(data)
	if _, err := parsePSDUnicodeStringFromReader(reader); err != nil {
		return "", 0, err
	}
	if _, err := parsePSDDescriptorID(reader); err != nil {
		return "", 0, err
	}
	itemCount, err := readUint32From(reader)
	if err != nil {
		return "", 0, err
	}
	for i := uint32(0); i < itemCount; i++ {
		key, err := parsePSDDescriptorID(reader)
		if err != nil {
			return "", 0, err
		}
		valueType, err := readStringFrom(reader, 4)
		if err != nil {
			return "", 0, err
		}
		switch valueType {
		case "TEXT":
			text, err := parsePSDUnicodeStringFromReader(reader)
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

func parsePSDUnicodeStringFromReader(reader *bytes.Reader) (string, error) {
	length, err := readUint32From(reader)
	if err != nil {
		return "", err
	}
	if int(length)*2 > reader.Len() {
		return "", fmt.Errorf("invalid PSD unicode string length %d", length)
	}
	chars := make([]uint16, length)
	for i := range chars {
		value, err := readUint16From(reader)
		if err != nil {
			return "", err
		}
		chars[i] = value
	}
	return string(utf16.Decode(chars)), nil
}

func parsePSDDescriptorID(reader *bytes.Reader) (string, error) {
	length, err := readUint32From(reader)
	if err != nil {
		return "", err
	}
	if length == 0 {
		return readStringFrom(reader, 4)
	}
	if int(length) > reader.Len() {
		return "", fmt.Errorf("invalid descriptor id length %d", length)
	}
	return readStringFrom(reader, int(length))
}
