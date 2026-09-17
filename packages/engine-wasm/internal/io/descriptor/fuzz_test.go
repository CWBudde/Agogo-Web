package descriptor

import (
	"bytes"
	"testing"
)

func FuzzParse(f *testing.F) {
	f.Add([]byte{0, 0, 0, 0})
	f.Add(buildDescriptor("", "null"))
	f.Add(buildDescriptor(
		"name", "null",
		item("bool", TypeBool, func(b *bytes.Buffer) { b.WriteByte(1) }),
		item("Txt ", TypeText, func(b *bytes.Buffer) { putUnicode(b, "hi\x00") }),
	))
	f.Add(buildDescriptor(
		"", "null",
		item("Clr ", TypeObject, func(b *bytes.Buffer) {
			b.Write(buildDescriptor("", "RGBC", item("Rd  ", TypeDouble, func(b *bytes.Buffer) { putF64(b, 255) })))
		}),
	))

	f.Fuzz(func(t *testing.T, data []byte) {
		d, n, err := Parse(data, Limits{})
		if err != nil {
			return
		}
		if n < 0 || n > len(data) {
			t.Fatalf("Parse consumed %d of %d bytes", n, len(data))
		}
		// A successful parse must be self-consistent: every value carries the
		// type it says it does, so a later writer can encode it back.
		var check func(v Value, depth int)
		check = func(v Value, depth int) {
			if depth > 64 {
				t.Fatal("value nesting deeper than the depth limit allows")
			}
			switch v.Type {
			case TypeObject, TypeGlobalObject:
				if v.Object == nil {
					t.Fatalf("value of type %q has no object", v.Type)
				}
				for _, it := range v.Object.Items {
					check(it.Value, depth+1)
				}
			case TypeList:
				for _, item := range v.List {
					check(item, depth+1)
				}
			case "":
				t.Fatal("value parsed with an empty type")
			}
		}
		for _, it := range d.Items {
			check(it.Value, 0)
		}
	})
}
