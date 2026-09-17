package abr

import (
	"bytes"
	"errors"
	"testing"

	"github.com/cwbudde/agogo-web/packages/engine-wasm/internal/io/descriptor"
)

// A "type" value is a unicode name followed by a class ID. The parser used to
// read only the class ID, which left the name's bytes in the stream and made
// every value after it garbage. The trailing bool is the detector: it is only
// reachable if the class value consumed exactly its own bytes.
func TestParseDescriptorSectionReadsClassValues(t *testing.T) {
	var b bytes.Buffer
	writeU32(&b, 16) // descriptor version
	writeDescriptorHeader(&b, "", "null", 2)
	writeClassID(&b, "Clss")
	b.WriteString("type")
	writeUnicode(&b, "Brush")
	writeClassID(&b, "Brsh")
	writeClassID(&b, "after")
	b.WriteString("bool")
	b.WriteByte(1)

	d, err := parseDescriptorSection(b.Bytes(), DefaultLimits())
	if err != nil {
		t.Fatalf("parseDescriptorSection: %v", err)
	}
	v, ok := d.Get("Clss")
	if !ok {
		t.Fatal("Clss item missing")
	}
	if v.Name != "Brush" || v.ClassID != "Brsh" {
		t.Errorf("class value = {Name:%q ClassID:%q}, want {Brush Brsh}", v.Name, v.ClassID)
	}
	if after, ok := d.Get("after"); !ok || !after.Bool {
		t.Error("the value after a class value was not read; the stream desynchronised")
	}
}

// The shared parser has its own error vocabulary. ABR callers must keep seeing
// ABR errors, so the translation is asserted rather than assumed.
func TestParseDescriptorSectionReportsABRErrors(t *testing.T) {
	var overBudget bytes.Buffer
	writeU32(&overBudget, 16)
	writeDescriptorHeader(&overBudget, "", "null", 3)
	for _, id := range []string{"a", "b", "c"} {
		writeClassID(&overBudget, id)
		overBudget.WriteString("bool")
		overBudget.WriteByte(1)
	}
	limits := DefaultLimits()
	limits.MaxDescriptorItems = 2
	if _, err := parseDescriptorSection(overBudget.Bytes(), limits); !errors.Is(err, ErrLimit) {
		t.Fatalf("err = %v, want abr.ErrLimit", err)
	}

	var unsupported bytes.Buffer
	writeU32(&unsupported, 16)
	writeDescriptorHeader(&unsupported, "", "null", 1)
	writeClassID(&unsupported, "ObAr")
	unsupported.WriteString("ObAr")
	if _, err := parseDescriptorSection(unsupported.Bytes(), DefaultLimits()); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("err = %v, want abr.ErrUnsupported", err)
	}

	var truncated bytes.Buffer
	writeU32(&truncated, 16)
	writeDescriptorHeader(&truncated, "", "null", 1)
	if _, err := parseDescriptorSection(truncated.Bytes(), DefaultLimits()); !errors.Is(err, ErrMalformed) {
		t.Fatalf("err = %v, want abr.ErrMalformed", err)
	}
}

// requireSharedTypes only compiles when the ABR names are aliases of the shared
// ones rather than distinct types that merely look the same. The alias is what
// kept every existing ABR consumer compiling when the parser moved out.
func requireSharedTypes(descriptor.Descriptor, descriptor.Item, descriptor.Value, descriptor.EnumValue) {
}

func TestDescriptorTypesAreTheSharedOnes(t *testing.T) {
	requireSharedTypes(Descriptor{}, Item{}, Value{}, EnumValue{})
}
