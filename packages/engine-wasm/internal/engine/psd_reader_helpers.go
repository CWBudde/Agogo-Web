package engine

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"strings"
	"unicode/utf16"
)

func parsePSDUnicodeString(data []byte) (string, error) {
	reader := bytes.NewReader(data)
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

func decodePackBits(data []byte, expectedLen int) ([]byte, error) {
	out := make([]byte, 0, expectedLen)
	for i := 0; i < len(data) && len(out) < expectedLen; {
		control := int(int8(data[i]))
		i++
		switch {
		case control >= 0:
			count := control + 1
			if i+count > len(data) {
				return nil, fmt.Errorf("packbits literal overruns row")
			}
			out = append(out, data[i:i+count]...)
			i += count
		case control >= -127:
			count := 1 - control
			if i >= len(data) {
				return nil, fmt.Errorf("packbits repeat overruns row")
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

func psdDocumentColorMode(colorMode int) string {
	switch colorMode {
	case psdColorModeGrayscale:
		return "gray"
	default:
		return "rgb"
	}
}

func mapPSDBlendMode(key string) BlendMode {
	switch strings.TrimSpace(key) {
	case "mul":
		return BlendModeMultiply
	case "scrn":
		return BlendModeScreen
	case "over":
		return BlendModeOverlay
	case "diff":
		return BlendModeDifference
	case "smud":
		return BlendModeExclusion
	case "dark":
		return BlendModeDarken
	case "lite":
		return BlendModeLighten
	default:
		return BlendModeNormal
	}
}

func parsePSDLayerColorTag(payload []byte) string {
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

func (p *psdParser) readBytes(n int) ([]byte, error) {
	return readBytesFrom(p.r, n)
}

func (p *psdParser) readString(n int) (string, error) {
	return readStringFrom(p.r, n)
}

func (p *psdParser) readUint16() (uint16, error) {
	return readUint16From(p.r)
}

func (p *psdParser) readUint32() (uint32, error) {
	return readUint32From(p.r)
}

func (p *psdParser) readSectionLength(psb bool) (uint64, error) {
	return readSectionLengthFrom(p.r, psb)
}

func readBytesFrom(r io.Reader, n int) ([]byte, error) {
	if n < 0 {
		return nil, fmt.Errorf("invalid read length %d", n)
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
