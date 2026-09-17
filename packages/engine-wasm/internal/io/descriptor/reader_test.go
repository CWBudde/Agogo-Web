package descriptor

import (
	"bytes"
	"errors"
	"testing"
)

func TestParseReadsEveryImplementedValueType(t *testing.T) {
	payload := buildDescriptor(
		"", "null",
		item("bool", TypeBool, func(b *bytes.Buffer) { b.WriteByte(1) }),
		item("long", TypeInteger, func(b *bytes.Buffer) { putU32(b, 0xFFFFFFFF) }), // -1
		item("doub", TypeDouble, func(b *bytes.Buffer) { putF64(b, 1.5) }),
		item("Opct", TypeUnitFloat, func(b *bytes.Buffer) { b.WriteString(UnitPercent); putF64(b, 75) }),
		item("Txt ", TypeText, func(b *bytes.Buffer) { putUnicode(b, "hello\x00") }),
		item("Md  ", TypeEnum, func(b *bytes.Buffer) { putKey(b, "BlnM"); putKey(b, "Mltp") }),
		item("tdta", TypeRawData, func(b *bytes.Buffer) { putU32(b, 3); b.WriteString("abc") }),
	)

	d, n, err := Parse(payload, Limits{})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if n != len(payload) {
		t.Fatalf("consumed %d bytes, want %d", n, len(payload))
	}
	if d.ClassID != "null" {
		t.Errorf("classID = %q, want %q", d.ClassID, "null")
	}

	if v, _ := d.Get("bool"); !v.Bool {
		t.Error("bool = false, want true")
	}
	if v, _ := d.Get("long"); v.Integer != -1 {
		t.Errorf("long = %d, want -1", v.Integer)
	}
	if v, _ := d.Get("doub"); v.Float != 1.5 {
		t.Errorf("doub = %v, want 1.5", v.Float)
	}
	if got, ok := d.UnitFloat("Opct", UnitPercent); !ok || got != 75 {
		t.Errorf("UnitFloat(Opct, %%) = %v, %v; want 75, true", got, ok)
	}
	if _, ok := d.UnitFloat("Opct", UnitAngle); ok {
		t.Error("UnitFloat accepted a percentage as an angle")
	}
	v, _ := d.Get("Txt ")
	if v.String != "hello\x00" {
		t.Errorf("String = %q, want the NUL preserved", v.String)
	}
	if v.Text() != "hello" {
		t.Errorf("Text() = %q, want %q", v.Text(), "hello")
	}
	if e, _ := d.Enum("Md  "); e.Type != "BlnM" || e.Value != "Mltp" {
		t.Errorf("enum = %+v, want {BlnM Mltp}", e)
	}
	if v, _ := d.Get("tdta"); string(v.Data) != "abc" {
		t.Errorf("tdta = %q, want %q", v.Data, "abc")
	}
}

// A class value is a unicode name followed by a class ID. Reading only the
// class ID leaves the name's bytes in the stream, so the next value's type is
// read from the middle of this one and everything after it is garbage. The
// trailing bool is what detects that: it is only reachable if the class value
// consumed exactly the right number of bytes.
func TestParseReadsClassValueNameAndClassID(t *testing.T) {
	payload := buildDescriptor(
		"", "null",
		item("Clss", TypeClass, func(b *bytes.Buffer) { putUnicode(b, "Layer"); putKey(b, "Lyr ") }),
		item("after", TypeBool, func(b *bytes.Buffer) { b.WriteByte(1) }),
	)

	d, err := ParseAll(payload, Limits{})
	if err != nil {
		t.Fatalf("ParseAll: %v", err)
	}
	v, ok := d.Get("Clss")
	if !ok {
		t.Fatal("Clss item missing")
	}
	if v.Name != "Layer" {
		t.Errorf("Name = %q, want %q", v.Name, "Layer")
	}
	if v.ClassID != "Lyr " {
		t.Errorf("ClassID = %q, want %q", v.ClassID, "Lyr ")
	}
	if after, ok := d.Get("after"); !ok || !after.Bool {
		t.Error("the value after a class value was not read; the stream desynchronised")
	}
}

func TestParseReadsNestedObjectsAndLists(t *testing.T) {
	inner := buildDescriptor(
		"", "RGBC",
		item("Rd  ", TypeDouble, func(b *bytes.Buffer) { putF64(b, 255) }),
	)
	payload := buildDescriptor(
		"", "null",
		item("Clr ", TypeObject, func(b *bytes.Buffer) { b.Write(inner) }),
		item("Clrs", TypeList, func(b *bytes.Buffer) {
			putU32(b, 2)
			b.WriteString(TypeInteger)
			putU32(b, 7)
			b.WriteString(TypeInteger)
			putU32(b, 9)
		}),
	)

	d, err := ParseAll(payload, Limits{})
	if err != nil {
		t.Fatalf("ParseAll: %v", err)
	}
	obj, ok := d.Object("Clr ")
	if !ok || obj.ClassID != "RGBC" {
		t.Fatalf("nested object = %+v, want classID RGBC", obj)
	}
	if v, _ := obj.Get("Rd  "); v.Float != 255 {
		t.Errorf("nested Rd = %v, want 255", v.Float)
	}
	list, _ := d.Get("Clrs")
	if len(list.List) != 2 || list.List[0].Integer != 7 || list.List[1].Integer != 9 {
		t.Errorf("list = %+v, want two longs 7 and 9", list.List)
	}
}

func TestParseRejectsUnimplementedValueType(t *testing.T) {
	payload := buildDescriptor(
		"", "null",
		item("ObAr", "ObAr", nil),
	)
	_, _, err := Parse(payload, Limits{})
	if !errors.Is(err, ErrUnsupported) {
		t.Fatalf("err = %v, want ErrUnsupported", err)
	}
	// The type must be named, so a reader hitting one in the field knows which
	// value to go and implement.
	if !bytes.Contains([]byte(err.Error()), []byte("ObAr")) {
		t.Errorf("err = %v, want it to name ObAr", err)
	}
}

func TestParseAllRejectsTrailingBytes(t *testing.T) {
	payload := append(buildDescriptor("", "null"), 0x00)
	if _, err := ParseAll(payload, Limits{}); !errors.Is(err, ErrMalformed) {
		t.Fatalf("err = %v, want ErrMalformed", err)
	}
}

func TestParseVersionedRejectsForeignVersion(t *testing.T) {
	payload := append([]byte{0, 0, 0, 15}, buildDescriptor("", "null")...)
	if _, err := ParseVersioned(payload, Limits{}); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("err = %v, want ErrUnsupported", err)
	}
}
