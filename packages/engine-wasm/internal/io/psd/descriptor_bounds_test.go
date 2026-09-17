package psd

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// descriptorWithShortKeyBools builds a descriptor whose items use one-character
// keys. parseDescriptorID accepts a 4-byte length followed by that many bytes,
// so each item is 4+1 key + 4 value type + 1 bool payload = 10 bytes — below the
// 12 a "every key is 4 characters" guard would assume.
func descriptorWithShortKeyBools(t *testing.T, keys ...string) []byte {
	t.Helper()
	var buf bytes.Buffer
	// Unicode name: 4-byte length 0, no payload.
	_ = binary.Write(&buf, binary.BigEndian, uint32(0))
	// Class ID: length 0 => 4-byte classID.
	_ = binary.Write(&buf, binary.BigEndian, uint32(0))
	buf.WriteString("null")
	_ = binary.Write(&buf, binary.BigEndian, uint32(len(keys)))
	for _, k := range keys {
		_ = binary.Write(&buf, binary.BigEndian, uint32(len(k)))
		buf.WriteString(k)
		buf.WriteString("bool")
		buf.WriteByte(1)
	}
	return buf.Bytes()
}

func TestParseDescriptorTextValueAcceptsShortDescriptorIDs(t *testing.T) {
	data := descriptorWithShortKeyBools(t, "a", "b")
	if _, _, err := ParseDescriptorTextValue(data, map[string]struct{}{"Txt ": {}}); err != nil {
		t.Fatalf("valid descriptor with one-character keys rejected: %v", err)
	}
}

func TestParseDescriptorTextValueStillRejectsImpossibleCounts(t *testing.T) {
	data := descriptorWithShortKeyBools(t, "a", "b")
	// Overwrite the item count with something no remaining input can satisfy.
	binary.BigEndian.PutUint32(data[12:16], 0xFFFFFFFF)
	if _, _, err := ParseDescriptorTextValue(data, map[string]struct{}{"Txt ": {}}); err == nil {
		t.Fatal("expected an error for an impossible descriptor item count")
	}
}
