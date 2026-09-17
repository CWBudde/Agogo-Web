package descriptor

import (
	"bytes"
	"errors"
	"testing"
)

// A four-byte count is attacker-controlled and precedes every item's bytes, so
// it must be bounded against the input that is actually left before anything is
// allocated from it. Without that bound the cumulative MaxItems budget is the
// only guard, and a twelve-byte payload reserves the whole budget.
func TestParseRejectsItemCountLargerThanRemainingInput(t *testing.T) {
	var b bytes.Buffer
	putUnicode(&b, "")
	putKey(&b, "null")
	putU32(&b, 0xFFFFFFFF)

	if _, _, err := Parse(b.Bytes(), Limits{}); !errors.Is(err, ErrMalformed) {
		// Without the remaining-input bound this is reported as ErrLimit by the
		// policy cap instead, which is the wrong diagnosis of a hostile count.
		t.Fatalf("err = %v, want ErrMalformed", err)
	}
}

// The sharp case for the remaining-input bound is a count that sits inside the
// policy budget but nowhere near the input: MaxItems waves it through, and the
// reservation happens anyway. A twelve-byte payload must not reserve 100k items.
func TestParseRejectsItemCountWithinBudgetButNotWithinInput(t *testing.T) {
	var b bytes.Buffer
	putUnicode(&b, "")
	putKey(&b, "null")
	putU32(&b, uint32(DefaultLimits().MaxItems))

	var err error
	bytes := allocatedBytes(t, func() { _, _, err = Parse(b.Bytes(), Limits{}) })
	if !errors.Is(err, ErrMalformed) {
		t.Fatalf("err = %v, want ErrMalformed", err)
	}
	// One Item is well over 100 bytes, so believing the count costs megabytes.
	// 64 KiB is far above what rejecting it needs and far below what it costs.
	if bytes > 64<<10 {
		t.Errorf("rejecting an unreachable item count allocated %d bytes; it reserved the slice before checking", bytes)
	}
}

func TestParseRejectsListCountLargerThanRemainingInput(t *testing.T) {
	payload := buildDescriptor(
		"", "null",
		item("Clrs", TypeList, func(b *bytes.Buffer) { putU32(b, 0xFFFFFFFF) }),
	)
	if _, _, err := Parse(payload, Limits{}); !errors.Is(err, ErrMalformed) {
		t.Fatalf("err = %v, want ErrMalformed", err)
	}
}

func TestParseRejectsItemCountOverBudget(t *testing.T) {
	payload := buildDescriptor(
		"", "null",
		item("a", TypeBool, func(b *bytes.Buffer) { b.WriteByte(1) }),
		item("b", TypeBool, func(b *bytes.Buffer) { b.WriteByte(1) }),
		item("c", TypeBool, func(b *bytes.Buffer) { b.WriteByte(1) }),
	)
	if _, _, err := Parse(payload, Limits{MaxItems: 2}); !errors.Is(err, ErrLimit) {
		t.Fatalf("err = %v, want ErrLimit", err)
	}
}

func TestParseRejectsNestingOverBudget(t *testing.T) {
	payload := buildDescriptor("", "null")
	for range 4 {
		inner := payload
		payload = buildDescriptor(
			"", "null",
			item("Objc", TypeObject, func(b *bytes.Buffer) { b.Write(inner) }),
		)
	}
	if _, _, err := Parse(payload, Limits{MaxDepth: 3}); !errors.Is(err, ErrLimit) {
		t.Fatalf("err = %v, want ErrLimit", err)
	}
	if _, _, err := Parse(payload, Limits{}); err != nil {
		t.Fatalf("within the default depth budget: %v", err)
	}
}

func TestParseRejectsOversizedStringAndData(t *testing.T) {
	text := buildDescriptor(
		"", "null",
		item("Txt ", TypeText, func(b *bytes.Buffer) { putU32(b, 0xFFFF); b.WriteString("xx") }),
	)
	if _, _, err := Parse(text, Limits{MaxStringBytes: 8}); !errors.Is(err, ErrLimit) {
		t.Fatalf("text err = %v, want ErrLimit", err)
	}

	data := buildDescriptor(
		"", "null",
		item("tdta", TypeRawData, func(b *bytes.Buffer) { putU32(b, 0xFFFF); b.WriteString("xx") }),
	)
	if _, _, err := Parse(data, Limits{MaxDataBytes: 8}); !errors.Is(err, ErrLimit) {
		t.Fatalf("data err = %v, want ErrLimit", err)
	}
}

func TestParseRejectsNonBooleanBooleanByte(t *testing.T) {
	payload := buildDescriptor(
		"", "null",
		item("bool", TypeBool, func(b *bytes.Buffer) { b.WriteByte(2) }),
	)
	if _, _, err := Parse(payload, Limits{}); !errors.Is(err, ErrMalformed) {
		t.Fatalf("err = %v, want ErrMalformed", err)
	}
}

func TestParseRejectsTruncationAtEveryOffset(t *testing.T) {
	payload := buildDescriptor(
		"", "null",
		item("Clr ", TypeObject, func(b *bytes.Buffer) {
			b.Write(buildDescriptor("", "RGBC", item("Rd  ", TypeDouble, func(b *bytes.Buffer) { putF64(b, 255) })))
		}),
		item("Txt ", TypeText, func(b *bytes.Buffer) { putUnicode(b, "hi") }),
	)
	for cut := range len(payload) {
		if _, _, err := Parse(payload[:cut], Limits{}); err == nil {
			t.Fatalf("truncation at %d parsed cleanly; every proper prefix must error", cut)
		}
	}
}
