package text

import "testing"

func TestMetricsSanity(t *testing.T) {
	const size = 24.0
	reg := DefaultRegistry()

	styles := []struct {
		name         string
		bold, italic bool
	}{
		{"Regular", false, false},
		{"Bold", true, false},
		{"Italic", false, true},
		{"Bold Italic", true, true},
	}
	for _, st := range styles {
		t.Run(st.name, func(t *testing.T) {
			face := reg.Resolve("DejaVu Sans", st.bold, st.italic)
			if face == nil {
				t.Fatal("Resolve returned nil")
			}
			m := face.Metrics(size)
			if m.Ascent <= 0 {
				t.Errorf("Ascent = %f, want > 0", m.Ascent)
			}
			if m.Descent <= 0 {
				t.Errorf("Descent = %f, want > 0 (positive-down)", m.Descent)
			}
			if m.CapHeight <= 0 {
				t.Errorf("CapHeight = %f, want > 0", m.CapHeight)
			}
			if m.XHeight <= 0 {
				t.Errorf("XHeight = %f, want > 0", m.XHeight)
			}
			if m.UnderlineThickness <= 0 {
				t.Errorf("UnderlineThickness = %f, want > 0", m.UnderlineThickness)
			}
			if m.UnderlinePosition <= 0 {
				t.Errorf("UnderlinePosition = %f, want > 0 (below baseline, positive-down)", m.UnderlinePosition)
			}
			if m.LineGap < 0 {
				t.Errorf("LineGap = %f, want >= 0", m.LineGap)
			}
			// Sanity: proportions must be plausible for a text face.
			if m.Ascent >= 2*size || m.Descent >= size {
				t.Errorf("implausible metrics for size %v: %+v", size, m)
			}
			if m.CapHeight >= m.Ascent+1e-9 {
				t.Errorf("CapHeight %f should not exceed Ascent %f", m.CapHeight, m.Ascent)
			}
		})
	}
}

func TestMetricsScaleLinearly(t *testing.T) {
	face := DefaultRegistry().Resolve("DejaVu Sans", false, false)
	m24 := face.Metrics(24)
	m48 := face.Metrics(48)
	// 26.6 quantization allows small deviation from exact doubling.
	const tol = 0.1
	if d := m48.Ascent - 2*m24.Ascent; d > tol || d < -tol {
		t.Errorf("Ascent does not scale: 24->%f 48->%f", m24.Ascent, m48.Ascent)
	}
	if d := m48.UnderlineThickness - 2*m24.UnderlineThickness; d > tol || d < -tol {
		t.Errorf("UnderlineThickness does not scale: 24->%f 48->%f", m24.UnderlineThickness, m48.UnderlineThickness)
	}
}
