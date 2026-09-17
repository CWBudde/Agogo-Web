package descriptor

import (
	"bytes"
	"encoding/binary"
	"math"
	"unicode/utf16"
)

// The helpers below build descriptor bytes by hand from the format
// description, deliberately without using this package's own writer, so that a
// reader test cannot be satisfied by a writer that shares its mistake.

func putU32(b *bytes.Buffer, v uint32) {
	var raw [4]byte
	binary.BigEndian.PutUint32(raw[:], v)
	b.Write(raw[:])
}

func putUnicode(b *bytes.Buffer, s string) {
	units := utf16.Encode([]rune(s))
	putU32(b, uint32(len(units)))
	for _, u := range units {
		var raw [2]byte
		binary.BigEndian.PutUint16(raw[:], u)
		b.Write(raw[:])
	}
}

// putKey writes the length-and-key form: a length, then four raw bytes when the
// length is zero, otherwise that many bytes.
func putKey(b *bytes.Buffer, key string) {
	if len(key) == 4 {
		putU32(b, 0)
		b.WriteString(key)
		return
	}
	putU32(b, uint32(len(key)))
	b.WriteString(key)
}

func putF64(b *bytes.Buffer, v float64) {
	var raw [8]byte
	binary.BigEndian.PutUint64(raw[:], math.Float64bits(v))
	b.Write(raw[:])
}

type rawItem struct {
	key  string
	body func(*bytes.Buffer)
}

func item(key, typ string, write func(*bytes.Buffer)) rawItem {
	return rawItem{key: key, body: func(b *bytes.Buffer) {
		b.WriteString(typ)
		if write != nil {
			write(b)
		}
	}}
}

func buildDescriptor(name, classID string, items ...rawItem) []byte {
	var b bytes.Buffer
	putUnicode(&b, name)
	putKey(&b, classID)
	putU32(&b, uint32(len(items)))
	for _, it := range items {
		putKey(&b, it.key)
		it.body(&b)
	}
	return b.Bytes()
}
