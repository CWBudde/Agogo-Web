package text

import (
	"testing"

	"codeberg.org/go-fonts/dejavu/dejavusans"
)

func TestResolveFallbackMatrix(t *testing.T) {
	reg := DefaultRegistry()

	regular := reg.Resolve("DejaVu Sans", false, false)
	bold := reg.Resolve("DejaVu Sans", true, false)
	italic := reg.Resolve("DejaVu Sans", false, true)
	boldItalic := reg.Resolve("DejaVu Sans", true, true)
	for name, f := range map[string]*Face{
		"regular": regular, "bold": bold, "italic": italic, "bold italic": boldItalic,
	} {
		if f == nil {
			t.Fatalf("DefaultRegistry missing DejaVu Sans %s", name)
		}
	}
	if regular == bold || regular == italic || bold == boldItalic {
		t.Fatal("distinct styles must resolve to distinct faces")
	}

	tests := []struct {
		name         string
		family       string
		bold, italic bool
		want         *Face
	}{
		{"exact regular", "DejaVu Sans", false, false, regular},
		{"exact bold", "DejaVu Sans", true, false, bold},
		{"exact italic", "DejaVu Sans", false, true, italic},
		{"exact bold italic", "DejaVu Sans", true, true, boldItalic},
		{"case and space normalization", "  dejavu sans ", true, false, bold},
		{"empty family", "", false, false, regular},
		{"empty family keeps style", "", true, false, bold},
		{"system-ui", "system-ui", false, false, regular},
		{"system-ui keeps style", "system-ui", false, true, italic},
		{"unknown family", "Comic Sans MS", false, false, regular},
		{"unknown family keeps bold", "Comic Sans MS", true, false, bold},
		{"unknown family keeps bold italic", "Comic Sans MS", true, true, boldItalic},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := reg.Resolve(tt.family, tt.bold, tt.italic); got != tt.want {
				t.Errorf("Resolve(%q, %v, %v) = %p, want %p", tt.family, tt.bold, tt.italic, got, tt.want)
			}
		})
	}
}

// TestResolveStyleFallsBackToFamilyRegular pins the fallback order: a missing
// style of a known family resolves to that family's regular face before
// falling back to DejaVu Sans in the requested style.
func TestResolveStyleFallsBackToFamilyRegular(t *testing.T) {
	reg := NewRegistry()
	if err := reg.Register("TestFam", false, false, dejavusans.TTF); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if err := reg.Register("DejaVu Sans", false, false, dejavusans.TTF); err != nil {
		t.Fatalf("Register: %v", err)
	}

	famRegular := reg.Resolve("TestFam", false, false)
	if famRegular == nil {
		t.Fatal("Resolve(TestFam regular) = nil")
	}
	if got := reg.Resolve("TestFam", true, false); got != famRegular {
		t.Errorf("bold request for family with only regular should fall back to family regular, got %p want %p", got, famRegular)
	}
}

func TestResolveEmptyRegistryReturnsNil(t *testing.T) {
	reg := NewRegistry()
	if got := reg.Resolve("Anything", false, false); got != nil {
		t.Errorf("Resolve on empty registry = %p, want nil", got)
	}
}

func TestRegisterGarbage(t *testing.T) {
	reg := NewRegistry()
	tests := []struct {
		name string
		data []byte
	}{
		{"nil", nil},
		{"empty", []byte{}},
		{"text", []byte("this is definitely not a font")},
		{"truncated ttf magic", []byte{0x00, 0x01, 0x00, 0x00}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := reg.Register("Garbage", false, false, tt.data); err == nil {
				t.Error("Register with garbage bytes succeeded, want error")
			}
		})
	}
	if got := reg.Resolve("Garbage", false, false); got != nil {
		t.Errorf("failed registration must not store a face, Resolve = %p", got)
	}
}

func TestList(t *testing.T) {
	infos := DefaultRegistry().List()
	var dejavu *FamilyInfo
	for i := range infos {
		if infos[i].Family == "DejaVu Sans" {
			dejavu = &infos[i]
			break
		}
	}
	if dejavu == nil {
		t.Fatalf("List() missing DejaVu Sans, got %+v", infos)
	}
	if len(dejavu.Styles) != 4 {
		t.Fatalf("DejaVu Sans styles = %v, want 4 styles", dejavu.Styles)
	}
	want := map[string]bool{"Regular": true, "Bold": true, "Italic": true, "Bold Italic": true}
	for _, s := range dejavu.Styles {
		if !want[s] {
			t.Errorf("unexpected style %q", s)
		}
		delete(want, s)
	}
	for s := range want {
		t.Errorf("missing style %q", s)
	}

	// Families must be sorted.
	for i := 1; i < len(infos); i++ {
		if infos[i-1].Family > infos[i].Family {
			t.Errorf("List() not sorted: %q > %q", infos[i-1].Family, infos[i].Family)
		}
	}
}

func TestListPreservesDisplayName(t *testing.T) {
	reg := NewRegistry()
	if err := reg.Register("  My Fancy Font ", false, false, dejavusans.TTF); err != nil {
		t.Fatalf("Register: %v", err)
	}
	infos := reg.List()
	if len(infos) != 1 || infos[0].Family != "My Fancy Font" {
		t.Errorf("List() = %+v, want trimmed display name \"My Fancy Font\"", infos)
	}
}
