package psd

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"unicode/utf16"

	"github.com/cwbudde/agogo-web/packages/engine-wasm/internal/model"
)

func NewParser(data []byte) *Parser {
	return &Parser{r: bytes.NewReader(data)}
}

func NewParserFromReader(r *bytes.Reader) *Parser {
	return &Parser{r: r}
}

func (p *Parser) Warnings() []string {
	return append([]string(nil), p.warnings...)
}

func (p *Parser) warnf(format string, args ...any) {
	p.warnings = append(p.warnings, fmt.Sprintf(format, args...))
}

func Parse(data []byte) (result ParseResult, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			result = ParseResult{}
			err = fmt.Errorf("invalid PSD data: %v", recovered)
		}
	}()
	parser := NewParser(data)
	header, err := parser.ParseHeader()
	if err != nil {
		return ParseResult{}, err
	}
	if err := validateSupportedDepth(header.Depth); err != nil {
		return ParseResult{Header: header}, err
	}
	if err := parser.SkipColorModeData(); err != nil {
		return ParseResult{}, err
	}
	resources, err := parser.ParseImageResources()
	if err != nil {
		return ParseResult{}, err
	}
	layers, err := parser.ParseLayerAndMaskInfo(header)
	if err != nil {
		return ParseResult{
			Header:    header,
			Resources: resources,
			Layers:    layers,
			Warnings:  parser.Warnings(),
		}, err
	}
	compositeRGBA, err := parser.ParseCompositeImageData(header)
	if err != nil {
		return ParseResult{
			Header:    header,
			Resources: resources,
			Layers:    layers,
			Warnings:  parser.Warnings(),
		}, err
	}
	return ParseResult{
		Header:        header,
		Resources:     resources,
		Layers:        layers,
		CompositeRGBA: compositeRGBA,
		Warnings:      parser.Warnings(),
	}, nil
}

func RequiresPSB(width, height int) bool {
	return width > PSDMaxDimension || height > PSDMaxDimension
}

func ParseUnicodeString(data []byte) (string, error) {
	reader := bytes.NewReader(data)
	return parseUnicodeStringFromReader(reader)
}

func DecodePackBits(data []byte, expectedLen int) ([]byte, error) {
	if expectedLen < 0 {
		return nil, fmt.Errorf("invalid PackBits decoded length %d", expectedLen)
	}
	out := make([]byte, 0, expectedLen)
	for i := 0; i < len(data); {
		control := int(int8(data[i]))
		i++
		switch {
		case control >= 0:
			count := control + 1
			if i+count > len(data) {
				return nil, fmt.Errorf("packbits literal overruns row")
			}
			if len(out)+count > expectedLen {
				return nil, fmt.Errorf("packbits literal exceeds decoded row length")
			}
			out = append(out, data[i:i+count]...)
			i += count
		case control >= -127:
			count := 1 - control
			if i >= len(data) {
				return nil, fmt.Errorf("packbits repeat overruns row")
			}
			if len(out)+count > expectedLen {
				return nil, fmt.Errorf("packbits repeat exceeds decoded row length")
			}
			value := data[i]
			i++
			for range count {
				out = append(out, value)
			}
		default:
		}
	}
	if len(out) != expectedLen {
		return nil, fmt.Errorf("packbits decoded %d bytes, want %d", len(out), expectedLen)
	}
	return out, nil
}

func DocumentColorMode(colorMode int) string {
	switch colorMode {
	case ColorModeGrayscale:
		return "gray"
	default:
		return "rgb"
	}
}

type psdBlendModeMapping struct {
	mode      model.BlendMode
	key       string
	canonical bool
}

// psdBlendModeMappings is the single source of truth for PSD blend keys.
// Keys are byte-exact four-character codes; trailing spaces are significant.
// Pass-through has no distinct model.BlendMode, so it is accepted as Normal
// on import but is never emitted for an ordinary Normal layer.
var psdBlendModeMappings = [...]psdBlendModeMapping{
	{model.BlendModeNormal, "norm", true},
	{model.BlendModeDissolve, "diss", true},
	{model.BlendModeMultiply, "mul ", true},
	{model.BlendModeColorBurn, "idiv", true},
	{model.BlendModeLinearBurn, "lbrn", true},
	{model.BlendModeDarken, "dark", true},
	{model.BlendModeDarkerColor, "dkCl", true},
	{model.BlendModeScreen, "scrn", true},
	{model.BlendModeColorDodge, "div ", true},
	{model.BlendModeLinearDodge, "lddg", true},
	{model.BlendModeLighten, "lite", true},
	{model.BlendModeLighterColor, "lgCl", true},
	{model.BlendModeOverlay, "over", true},
	{model.BlendModeSoftLight, "sLit", true},
	{model.BlendModeHardLight, "hLit", true},
	{model.BlendModeVividLight, "vLit", true},
	{model.BlendModeLinearLight, "lLit", true},
	{model.BlendModePinLight, "pLit", true},
	{model.BlendModeHardMix, "hMix", true},
	{model.BlendModeDifference, "diff", true},
	{model.BlendModeExclusion, "smud", true},
	{model.BlendModeSubtract, "fsub", true},
	{model.BlendModeDivide, "fdiv", true},
	{model.BlendModeHue, "hue ", true},
	{model.BlendModeSaturation, "sat ", true},
	{model.BlendModeColor, "colr", true},
	{model.BlendModeLuminosity, "lum ", true},
	{model.BlendModeNormal, "pass", false},
}

func mapBlendMode(key string) (model.BlendMode, bool) {
	for _, mapping := range psdBlendModeMappings {
		if mapping.key == key {
			return mapping.mode, true
		}
	}
	return model.BlendModeNormal, false
}

func MapBlendMode(key string) model.BlendMode {
	mode, _ := mapBlendMode(key)
	return mode
}

func ParseLayerColorTag(payload []byte) string {
	if len(payload) == 0 {
		return "none"
	}
	switch payload[0] {
	case 1:
		return "red"
	case 2:
		return "orange"
	case 3:
		return "yellow"
	case 4:
		return "green"
	case 5:
		return "blue"
	case 6:
		return "violet"
	case 7:
		return "gray"
	default:
		return fmt.Sprintf("unknown(%d)", payload[0])
	}
}

func readBytesFrom(r io.Reader, n int) ([]byte, error) {
	if n < 0 {
		return nil, fmt.Errorf("invalid read length %d", n)
	}
	if remaining, ok := r.(interface{ Len() int }); ok && n > remaining.Len() {
		return nil, fmt.Errorf("read length %d exceeds remaining input %d", n, remaining.Len())
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

func readStringFrom(r io.Reader, n int) (string, error) {
	buf, err := readBytesFrom(r, n)
	if err != nil {
		return "", err
	}
	return string(buf), nil
}

func readUint8From(r io.Reader) (uint8, error) {
	var value uint8
	err := binary.Read(r, binary.BigEndian, &value)
	return value, err
}

func readUint16From(r io.Reader) (uint16, error) {
	var value uint16
	err := binary.Read(r, binary.BigEndian, &value)
	return value, err
}

func readInt16From(r io.Reader) (int16, error) {
	var value int16
	err := binary.Read(r, binary.BigEndian, &value)
	return value, err
}

func readUint32From(r io.Reader) (uint32, error) {
	var value uint32
	err := binary.Read(r, binary.BigEndian, &value)
	return value, err
}

func readInt32From(r io.Reader) (int32, error) {
	var value int32
	err := binary.Read(r, binary.BigEndian, &value)
	return value, err
}

func readSectionLengthFrom(r io.Reader, psb bool) (uint64, error) {
	if psb {
		var value uint64
		err := binary.Read(r, binary.BigEndian, &value)
		return value, err
	}
	value, err := readUint32From(r)
	return uint64(value), err
}

func readPascalString4(r *bytes.Reader) (string, error) {
	length, err := r.ReadByte()
	if err != nil {
		return "", err
	}
	buf, err := readBytesFrom(r, int(length))
	if err != nil {
		return "", err
	}
	padding := (4 - ((1 + int(length)) % 4)) % 4
	if padding > 0 {
		if _, err := io.CopyN(io.Discard, r, int64(padding)); err != nil {
			return "", err
		}
	}
	return string(buf), nil
}

func parseUnicodeStringFromReader(reader *bytes.Reader) (string, error) {
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

func parseDescriptorID(reader *bytes.Reader) (string, error) {
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

func BuildResolutionInfo(resolution float64) []byte {
	dpi := resolution
	if dpi <= 0 {
		dpi = 72
	}
	fixed := uint32(math.Round(dpi * 65536.0))
	var out bytes.Buffer
	writeUint32(&out, fixed)
	writeUint16(&out, 1)
	writeUint16(&out, 1)
	writeUint32(&out, fixed)
	writeUint16(&out, 1)
	writeUint16(&out, 1)
	return out.Bytes()
}

func RGBAToPlanes(grayscale bool, rgba []byte) [][]byte {
	pixelCount := len(rgba) / 4
	if grayscale {
		gray := make([]byte, pixelCount)
		alpha := make([]byte, pixelCount)
		for i := 0; i < pixelCount; i++ {
			r := float64(rgba[i*4])
			g := float64(rgba[i*4+1])
			b := float64(rgba[i*4+2])
			gray[i] = byte(math.Round((0.299 * r) + (0.587 * g) + (0.114 * b)))
			alpha[i] = rgba[i*4+3]
		}
		return [][]byte{gray, alpha}
	}
	red := make([]byte, pixelCount)
	green := make([]byte, pixelCount)
	blue := make([]byte, pixelCount)
	alpha := make([]byte, pixelCount)
	for i := 0; i < pixelCount; i++ {
		red[i] = rgba[i*4]
		green[i] = rgba[i*4+1]
		blue[i] = rgba[i*4+2]
		alpha[i] = rgba[i*4+3]
	}
	return [][]byte{red, green, blue, alpha}
}

func EncodeChannelData(data []byte, width, height int, psb bool) ([]byte, error) {
	if width <= 0 || height <= 0 {
		return []byte{0, CompressionRLE}, nil
	}
	if len(data) != width*height {
		return nil, fmt.Errorf("channel length %d does not match %dx%d", len(data), width, height)
	}

	rows := make([][]byte, 0, height)
	for row := 0; row < height; row++ {
		start := row * width
		rows = append(rows, EncodePackBitsRow(data[start:start+width]))
	}

	var out bytes.Buffer
	writeUint16(&out, CompressionRLE)
	for _, row := range rows {
		if psb {
			writeUint32(&out, uint32(len(row)))
		} else {
			if len(row) > math.MaxUint16 {
				return nil, fmt.Errorf("RLE row length %d exceeds PSD limit", len(row))
			}
			writeUint16(&out, uint16(len(row)))
		}
	}
	for _, row := range rows {
		out.Write(row)
	}
	return out.Bytes(), nil
}

func EncodePackBitsRow(data []byte) []byte {
	if len(data) == 0 {
		return []byte{}
	}
	out := make([]byte, 0, len(data)+(len(data)/128)+1)
	for i := 0; i < len(data); {
		runLen := 1
		for i+runLen < len(data) && runLen < 128 && data[i+runLen] == data[i] {
			runLen++
		}
		if runLen >= 3 {
			out = append(out, byte(257-runLen), data[i])
			i += runLen
			continue
		}

		literalStart := i
		i += runLen
		for i < len(data) {
			runLen = 1
			for i+runLen < len(data) && runLen < 128 && data[i+runLen] == data[i] {
				runLen++
			}
			if runLen >= 3 || i-literalStart >= 128 {
				break
			}
			i += runLen
		}
		literalLen := i - literalStart
		for literalLen > 0 {
			chunkLen := literalLen
			if chunkLen > 128 {
				chunkLen = 128
			}
			out = append(out, byte(chunkLen-1))
			out = append(out, data[literalStart:literalStart+chunkLen]...)
			literalStart += chunkLen
			literalLen -= chunkLen
		}
	}
	return out
}

func EncodeCompositeImageData(planes [][]byte, width, height int, psb bool) ([]byte, error) {
	if width < 0 || height < 0 {
		return nil, fmt.Errorf("invalid composite dimensions %dx%d", width, height)
	}
	pixelCount := width * height
	rows := make([][]byte, 0, len(planes)*height)
	for planeIndex, plane := range planes {
		if len(plane) != pixelCount {
			return nil, fmt.Errorf("composite plane %d length %d does not match %dx%d", planeIndex, len(plane), width, height)
		}
		for row := 0; row < height; row++ {
			start := row * width
			rows = append(rows, EncodePackBitsRow(plane[start:start+width]))
		}
	}

	var out bytes.Buffer
	writeUint16(&out, CompressionRLE)
	for _, row := range rows {
		if psb {
			writeUint32(&out, uint32(len(row)))
		} else {
			if len(row) > math.MaxUint16 {
				return nil, fmt.Errorf("composite RLE row length %d exceeds PSD limit", len(row))
			}
			writeUint16(&out, uint16(len(row)))
		}
	}
	for _, row := range rows {
		out.Write(row)
	}
	return out.Bytes(), nil
}

func WriteDescriptor(out *bytes.Buffer, name, classID string, items []DescriptorItem) {
	writeUnicodeString(out, name)
	writeDescriptorID(out, classID)
	writeUint32(out, uint32(len(items)))
	for _, item := range items {
		writeDescriptorID(out, item.Key)
		writeString(out, item.Type)
		switch item.Type {
		case "TEXT":
			writeUnicodeString(out, item.Text)
		case "bool":
			if item.Bool {
				out.WriteByte(1)
			} else {
				out.WriteByte(0)
			}
		case "doub":
			writeFloat64(out, item.Float64)
		case "long":
			writeInt32(out, item.Int32)
		default:
			writeUnicodeString(out, item.Text)
		}
	}
}

func WriteImageResource(out *bytes.Buffer, resourceID uint16, name string, payload []byte) {
	writeString(out, "8BIM")
	writeUint16(out, resourceID)
	writePascalString2(out, name)
	writeUint32(out, uint32(len(payload)))
	out.Write(payload)
	if len(payload)%2 != 0 {
		out.WriteByte(0)
	}
}

func WriteAdditionalLayerInfoBlock(out *bytes.Buffer, signature, key string, payload []byte) {
	writeString(out, signature)
	writeString(out, key)
	writeUint32(out, uint32(len(payload)))
	out.Write(payload)
	if len(payload)%2 != 0 {
		out.WriteByte(0)
	}
}

func EncodeUnicodeString(value string) []byte {
	encoded := utf16Encode(value)
	var out bytes.Buffer
	writeUint32(&out, uint32(len(encoded)))
	for _, r := range encoded {
		writeUint16(&out, r)
	}
	return out.Bytes()
}

func utf16Encode(value string) []uint16 {
	runes := []rune(value)
	encoded := make([]uint16, 0, len(runes))
	for _, r := range runes {
		if r <= math.MaxUint16 {
			encoded = append(encoded, uint16(r))
			continue
		}
		r -= 0x10000
		encoded = append(
			encoded,
			uint16(0xd800+((r>>10)&0x3ff)),
			uint16(0xdc00+(r&0x3ff)),
		)
	}
	return encoded
}

func writeUnicodeString(out *bytes.Buffer, value string) {
	encoded := utf16Encode(value)
	writeUint32(out, uint32(len(encoded)))
	for _, r := range encoded {
		writeUint16(out, r)
	}
}

func writeDescriptorID(out *bytes.Buffer, value string) {
	if len(value) == 4 {
		writeUint32(out, 0)
		writeString(out, value)
		return
	}
	writeUint32(out, uint32(len(value)))
	writeString(out, value)
}

func writeFloat64(out *bytes.Buffer, value float64) {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], math.Float64bits(value))
	out.Write(buf[:])
}

func writePascalString2(out *bytes.Buffer, value string) {
	if len(value) > math.MaxUint8 {
		value = value[:math.MaxUint8]
	}
	out.WriteByte(byte(len(value)))
	out.WriteString(value)
	if (1+len(value))%2 != 0 {
		out.WriteByte(0)
	}
}

func writePascalString4(out *bytes.Buffer, value string) {
	if len(value) > math.MaxUint8 {
		value = value[:math.MaxUint8]
	}
	out.WriteByte(byte(len(value)))
	out.WriteString(value)
	for (1+len(value))%4 != 0 {
		out.WriteByte(0)
		value += "\x00"
	}
}

func writeSectionLength(out *bytes.Buffer, psb bool, length uint64) {
	if psb {
		writeUint64(out, length)
		return
	}
	writeUint32(out, uint32(length))
}

func writeString(out *bytes.Buffer, value string) {
	out.WriteString(value)
}

func writeUint16(out *bytes.Buffer, value uint16) {
	var buf [2]byte
	binary.BigEndian.PutUint16(buf[:], value)
	out.Write(buf[:])
}

func writeUint32(out *bytes.Buffer, value uint32) {
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], value)
	out.Write(buf[:])
}

func writeUint64(out *bytes.Buffer, value uint64) {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], value)
	out.Write(buf[:])
}

func writeInt16(out *bytes.Buffer, value int16) {
	writeUint16(out, uint16(value))
}

func writeInt32(out *bytes.Buffer, value int32) {
	writeUint32(out, uint32(value))
}

func UnitOpacity(value float64) uint8 {
	return uint8(math.Round(model.ClampUnit(value) * 255))
}

func BlendKey(mode model.BlendMode) string {
	for _, mapping := range psdBlendModeMappings {
		if mapping.canonical && mapping.mode == mode {
			return mapping.key
		}
	}
	return "norm"
}
